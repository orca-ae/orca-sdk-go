//go:build e2e

// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/e2e/sdk.test.cjs.
//
// This drives a real deployment end to end, so it is gated twice: the `e2e`
// build tag keeps it out of `go test ./...` entirely, and even under that tag
// it skips unless the environment is configured. Run it with:
//
//	ORCA_BASE_URL=... ORCA_E2E_API_KEY=... \
//	  go test -tags e2e -timeout 10m -run TestE2E ./...
//
// The environment contract is the TypeScript one, unchanged:
//
//	ORCA_BASE_URL             required
//	ORCA_E2E_API_KEY          workspace API key   -- exactly one of these two;
//	ORCA_E2E_ACCESS_TOKEN     bearer access token    both or neither is an error
//	ORCA_E2E_EXPECT_CLOUD     "true" => the deployment must advertise cloud.sn.io
//	ORCA_E2E_EXPECT_EXECUTION "true" => the deployment must actually run the agent
//
// The scenarios preserve the TypeScript suite's failure-aggregating behaviour:
// a failing scenario is recorded and the run continues, so one bad surface
// does not hide the others, and cleanup always runs.
// That is why the scenario bodies return errors instead of calling t.Fatalf —
// a Fatalf would abort the goroutine and skip both the remaining scenarios and
// the resource cleanup.
//
// Policy, pricing, agent, session, discovery, and cloud scenarios use the typed
// services. The remaining core scenarios keep the untyped compatibility client
// so this suite continues to exercise that public surface too.

const (
	e2eBaseURLEnv         = "ORCA_BASE_URL"
	e2eAPIKeyEnv          = "ORCA_E2E_API_KEY"
	e2eAccessTokenEnv     = "ORCA_E2E_ACCESS_TOKEN"
	e2eExpectCloudEnv     = "ORCA_E2E_EXPECT_CLOUD"
	e2eExpectExecutionEnv = "ORCA_E2E_EXPECT_EXECUTION"

	// e2eSkipReason is reported when nothing is configured. It is deliberately
	// NOT pendingManagedAgents: this suite is gated on credentials, not on
	// unimplemented typed resources, and must not inflate the pending count.
	e2eSkipReason = "e2e: set " + e2eBaseURLEnv + " and exactly one of " +
		e2eAPIKeyEnv + " / " + e2eAccessTokenEnv + " to run against a deployment"

	e2eHTTPTimeout   = 120 * time.Second
	e2eReplyDeadline = 120 * time.Second
	e2eReplyPoll     = time.Second
	e2eReplayTimeout = 10 * time.Second
)

// e2eRun is one end-to-end run: the client under test, the resources it
// created, and the scenario failures collected so far.
type e2eRun struct {
	t                *testing.T
	client           *Client
	agents           *ManagedAgentsClient
	prefix           string
	suffix           string
	expectCloud      bool
	expectExecution  bool
	agentID          string
	agentVersion     interface{}
	guardrailID      string
	environmentID    string
	fileID           string
	sessionID        string
	triggerID        string
	failures         []error
	cleanupSucceeded bool
}

// e2eEnvironment builds the run from the environment, skipping when nothing is
// configured and failing when the configuration is contradictory.
func e2eEnvironment(t *testing.T) *e2eRun {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv(e2eBaseURLEnv))
	apiKey := strings.TrimSpace(os.Getenv(e2eAPIKeyEnv))
	accessToken := strings.TrimSpace(os.Getenv(e2eAccessTokenEnv))

	if baseURL == "" && apiKey == "" && accessToken == "" {
		t.Skip(e2eSkipReason)
	}

	// Exactly one credential: both or neither is a configuration error, not a
	// skip. The TypeScript suite throws here for the same reason — a run that
	// silently picked one of two credentials would test the wrong identity.
	if (apiKey != "") == (accessToken != "") {
		t.Fatalf("exactly one of %s or %s is required", e2eAPIKeyEnv, e2eAccessTokenEnv)
	}
	if baseURL == "" {
		t.Fatalf("%s is required", e2eBaseURLEnv)
	}

	httpClient := &http.Client{Timeout: e2eHTTPTimeout}

	var (
		client *Client
		err    error
	)
	if accessToken != "" {
		client, err = NewClient(baseURL, accessToken, httpClient)
	} else {
		client, err = NewAPIKeyClient(baseURL, apiKey, httpClient)
	}
	if err != nil {
		t.Fatalf("building the e2e client for %q: %v", baseURL, err)
	}

	suffix := strings.Join([]string{
		e2eEnvOr("GITHUB_RUN_ID", "local"),
		e2eEnvOr("GITHUB_RUN_ATTEMPT", "0"),
		fmt.Sprint(os.Getpid()),
	}, "-")

	return &e2eRun{
		t:               t,
		client:          client,
		agents:          NewManagedAgentsClient(client),
		prefix:          "sdk-e2e-" + suffix,
		suffix:          suffix,
		expectCloud:     os.Getenv(e2eExpectCloudEnv) == "true",
		expectExecution: os.Getenv(e2eExpectExecutionEnv) == "true",
	}
}

func e2eEnvOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// scenario runs one named scenario, recording rather than raising its failure
// so the remaining scenarios and the cleanup still run.
func (r *e2eRun) scenario(name string, operation func() error) {
	r.t.Logf("[RUN] %s", name)
	if err := operation(); err != nil {
		r.failures = append(r.failures, fmt.Errorf("%s: %w", name, err))
		r.t.Logf("[FAIL] %s: %v", name, err)
		return
	}
	r.t.Logf("[PASS] %s", name)
}

// e2ePath joins a collection path and its query parameters.
func e2ePath(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// e2eObject asserts that an untyped response is a JSON object.
func e2eObject(what string, value interface{}) (map[string]interface{}, error) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: response type = %T, want a JSON object", what, value)
	}
	return object, nil
}

// e2eString reads a required string field out of an untyped response.
func e2eString(what string, object map[string]interface{}, key string) (string, error) {
	value, ok := object[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s: %s = %v, want a non-empty string", what, key, object[key])
	}
	return value, nil
}

// e2eEqual compares an untyped field against an expected value.
func e2eEqual(what string, got interface{}, want interface{}) error {
	if got != want {
		return fmt.Errorf("%s = %v, want %v", what, got, want)
	}
	return nil
}

// e2eListContains reports whether a single list page contains the given ID.
func (r *e2eRun) e2eListContains(ctx context.Context, path string, query url.Values, id string) error {
	raw, err := r.agents.Get(ctx, e2ePath(path, query))
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	page, err := e2eObject("GET "+path, raw)
	if err != nil {
		return err
	}
	items, _ := page["data"].([]interface{})
	for _, entry := range items {
		if item, ok := entry.(map[string]interface{}); ok && item["id"] == id {
			return nil
		}
	}
	return fmt.Errorf("GET %s: %s is not in the listing", path, id)
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

// e2eDiscovery checks that the advertised topology matches what the caller
// declared through ORCA_E2E_EXPECT_CLOUD.
func (r *e2eRun) e2eDiscovery(ctx context.Context) error {
	groups, err := r.client.GetAPIGroups(ctx)
	if err != nil {
		return fmt.Errorf("discovery groups: %w", err)
	}
	if err := e2eEqual("discovery kind", groups.Kind, "APIGroupList"); err != nil {
		return err
	}
	if got := groups.HasGroup(CloudExtensionGroup); got != r.expectCloud {
		return fmt.Errorf("advertises %s = %t, want %t", CloudExtensionGroup, got, r.expectCloud)
	}

	if r.expectCloud {
		resources, err := r.client.Cloud.APIResources.List(ctx)
		if err != nil {
			return fmt.Errorf("cloud API resources: %w", err)
		}
		if err := e2eEqual("cloud group_version", resources.GroupVersion, CloudExtensionGroup+"/v1"); err != nil {
			return err
		}
		if len(resources.Resources) == 0 {
			return errors.New("cloud API resource list is empty")
		}
		return nil
	}

	if _, err := r.client.Cloud.APIResources.List(ctx); err == nil {
		return fmt.Errorf("cloud API resources succeeded although %s is not advertised", CloudExtensionGroup)
	} else {
		var unavailable *ExtensionNotAvailableError
		if !errors.As(err, &unavailable) || unavailable.Group != CloudExtensionGroup {
			return fmt.Errorf("cloud API resources error = %v, want %s ExtensionNotAvailableError", err, CloudExtensionGroup)
		}
	}

	policy, err := r.client.Discovery.PolicyGroupResources(ctx)
	if err != nil {
		return fmt.Errorf("policy API resources: %w", err)
	}
	if policy.GroupVersion != PolicyExtensionGroup+"/v1" || !e2eHasResource(policy, "guardrails") {
		return fmt.Errorf("policy API resources = %#v, want guardrails", policy)
	}
	pricing, err := r.client.Discovery.PricingGroupResources(ctx)
	if err != nil {
		return fmt.Errorf("pricing API resources: %w", err)
	}
	if pricing.GroupVersion != PricingExtensionGroup+"/v1" || !e2eHasResource(pricing, "modelprices") {
		return fmt.Errorf("pricing API resources = %#v, want modelprices", pricing)
	}
	return nil
}

func e2eHasResource(resources *APIResourceList, name string) bool {
	for _, resource := range resources.Resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

func (r *e2eRun) e2eGuardrailScenario(ctx context.Context) error {
	types, err := r.client.Guardrails.ListTypes(ctx)
	if err != nil {
		return fmt.Errorf("list types: %w", err)
	}
	foundBuiltin := false
	for _, guardrailType := range types.Data {
		if guardrailType.Name == "block_tools" {
			foundBuiltin = true
			break
		}
	}
	if !foundBuiltin {
		return errors.New("guardrail type list does not include block_tools")
	}

	created, err := r.client.Guardrails.Create(ctx, GuardrailNewParams{
		Name:        r.prefix + "-guardrail",
		Description: param.String("created by SDK E2E"),
		Phases:      []GuardrailPhase{GuardrailPhaseRequest},
		Scope:       param.New(GuardrailScopeExplicit),
		Rule: GuardrailRule{
			Kind:       GuardrailRuleExpression,
			Expression: "true",
			OnFalse:    GuardrailVerdictDeny,
		},
		Metadata: map[string]string{"suite": "orca-sdk-e2e"},
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	r.guardrailID = created.ID

	updated, err := r.client.Guardrails.Update(ctx, created.ID, GuardrailUpdateParams{
		Description: param.String("updated by SDK E2E"),
	})
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if updated.Description != "updated by SDK E2E" {
		return fmt.Errorf("updated description = %q", updated.Description)
	}
	retrieved, err := r.client.Guardrails.Get(ctx, updated.ID)
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	if retrieved.ID != updated.ID {
		return fmt.Errorf("retrieved id = %q, want %q", retrieved.ID, updated.ID)
	}
	page, err := r.client.Guardrails.List(ctx, GuardrailListParams{Limit: param.Int(100)})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, guardrail := range page.Data {
		if guardrail.ID == updated.ID {
			return nil
		}
	}
	return fmt.Errorf("guardrail %s is not in the listing", updated.ID)
}

func (r *e2eRun) e2eModelPriceScenario(ctx context.Context) error {
	page, err := r.client.ModelPrices.List(ctx, ModelPriceListParams{Limit: param.Int(10)})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	if len(page.Data) == 0 {
		return errors.New("seeded model price catalog is empty")
	}
	first := page.Data[0]
	retrieved, err := r.client.ModelPrices.Get(ctx, first.ModelID, ModelPriceGetParams{
		Provider: param.String(first.Provider),
	})
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	if *retrieved != first {
		return fmt.Errorf("retrieved price = %#v, want %#v", retrieved, first)
	}
	return nil
}

// e2eEnvironmentScenario exercises the environment lifecycle.
func (r *e2eRun) e2eEnvironmentScenario(ctx context.Context) error {
	created, err := r.agents.Create(ctx, "/v1/environments", map[string]interface{}{
		"name":        r.prefix + "-environment",
		"description": "created by SDK E2E",
		"config": map[string]interface{}{
			"type":       "cloud",
			"networking": map[string]interface{}{"type": "unrestricted"},
		},
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	environment, err := e2eObject("create environment", created)
	if err != nil {
		return err
	}
	if r.environmentID, err = e2eString("create environment", environment, "id"); err != nil {
		return err
	}

	raw, err := r.agents.Update(ctx, http.MethodPost,
		"/v1/environments/"+url.PathEscape(r.environmentID),
		map[string]interface{}{"description": "updated by SDK E2E"})
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	updated, err := e2eObject("update environment", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("updated description", updated["description"], "updated by SDK E2E"); err != nil {
		return err
	}

	raw, err = r.agents.Get(ctx, "/v1/environments/"+url.PathEscape(r.environmentID))
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	retrieved, err := e2eObject("retrieve environment", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("retrieved id", retrieved["id"], r.environmentID); err != nil {
		return err
	}

	return r.e2eListContains(ctx, "/v1/environments", url.Values{"limit": {"100"}}, r.environmentID)
}

func (r *e2eRun) e2eAgentScenario(ctx context.Context) error {
	params := AgentNewParams{
		Name: r.prefix + "-agent",
		Model: AgentModelParam{
			ID:       "claude-sonnet-4-5-20250929",
			Provider: param.String("anthropic"),
		},
		System:   param.String("Return concise answers."),
		Metadata: map[string]string{"suite": "orca-sdk-e2e"},
	}
	var opts []option.RequestOption
	if r.guardrailID != "" {
		params.GuardrailIDs = []string{r.guardrailID}
		opts = append(opts, option.WithHeader("orca-beta", "managed-agents-2026-04-01"))
	}
	created, err := r.client.Agents.Create(ctx, params, opts...)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if created.ID == "" {
		return errors.New("create agent returned an empty id")
	}
	r.agentID = created.ID
	r.agentVersion = created.Version

	updated, err := r.client.Agents.Update(ctx, r.agentID, AgentUpdateParams{
		Description: param.String("updated by SDK E2E"),
	}, opts...)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if updated.Description == nil || *updated.Description != "updated by SDK E2E" {
		return fmt.Errorf("updated description = %v", updated.Description)
	}
	if r.guardrailID != "" && (len(updated.GuardrailIDs) != 1 || updated.GuardrailIDs[0] != r.guardrailID) {
		return fmt.Errorf("updated guardrail_ids = %v, want %s", updated.GuardrailIDs, r.guardrailID)
	}
	r.agentVersion = updated.Version

	retrieved, err := r.client.Agents.Get(ctx, r.agentID, AgentGetParams{}, opts...)
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	if retrieved.ID != r.agentID {
		return fmt.Errorf("retrieved id = %q, want %q", retrieved.ID, r.agentID)
	}
	if r.guardrailID != "" && (len(retrieved.GuardrailIDs) != 1 || retrieved.GuardrailIDs[0] != r.guardrailID) {
		return fmt.Errorf("retrieved guardrail_ids = %v, want %s", retrieved.GuardrailIDs, r.guardrailID)
	}

	page, err := r.client.Agents.List(ctx, AgentListParams{Limit: param.Int(100)}, opts...)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, agent := range page.Data {
		if agent.ID == r.agentID {
			return nil
		}
	}
	return fmt.Errorf("agent %s is not in the listing", r.agentID)
}

// e2eTriggerScenario exercises create, retrieve, update, list, session history,
// and the pause/unpause actions.
func (r *e2eRun) e2eTriggerScenario(ctx context.Context) error {
	if r.agentID == "" {
		return errors.New("agent scenario did not create an agent")
	}
	if r.environmentID == "" {
		return errors.New("environment scenario did not create an environment")
	}

	created, err := r.agents.Create(ctx, "/v1/triggers", map[string]interface{}{
		"name": r.prefix + "-trigger",
		"agent": map[string]interface{}{
			"type":    "agent",
			"id":      r.agentID,
			"version": r.agentVersion,
		},
		"session_mode": "SESSION_PER_EVENT",
		"source": map[string]interface{}{
			"type":     "cron",
			"schedule": "0 0 1 1 *",
			"timezone": "Etc/UTC",
			"payload":  "SDK trigger " + r.suffix,
		},
		"session": map[string]interface{}{
			"environment_id": r.environmentID,
			"title_template": "${trigger.name}",
			"metadata":       map[string]interface{}{"suite": "orca-sdk-e2e"},
			"vault_ids":      []interface{}{},
		},
		"replicas": 1,
		"paused":   true,
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	trigger, err := e2eObject("create trigger", created)
	if err != nil {
		return err
	}
	if r.triggerID, err = e2eString("create trigger", trigger, "id"); err != nil {
		return err
	}
	if err := e2eEqual("created status", trigger["status"], "paused"); err != nil {
		return err
	}

	triggerPath := "/v1/triggers/" + url.PathEscape(r.triggerID)

	raw, err := r.agents.Get(ctx, triggerPath)
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	retrieved, err := e2eObject("retrieve trigger", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("retrieved id", retrieved["id"], r.triggerID); err != nil {
		return err
	}

	wantName := r.prefix + "-trigger-updated"
	wantPayload := "Updated SDK trigger " + r.suffix
	raw, err = r.agents.Update(ctx, http.MethodPost, triggerPath, map[string]interface{}{
		"name":   wantName,
		"source": map[string]interface{}{"type": "cron", "payload": wantPayload},
	})
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	updated, err := e2eObject("update trigger", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("updated name", updated["name"], wantName); err != nil {
		return err
	}
	source, err := e2eObject("updated trigger source", updated["source"])
	if err != nil {
		return err
	}
	if err := e2eEqual("updated source payload", source["payload"], wantPayload); err != nil {
		return err
	}

	if err := r.e2eListContains(ctx, "/v1/triggers",
		url.Values{"agent_id": {r.agentID}, "limit": {"100"}}, r.triggerID); err != nil {
		return err
	}

	raw, err = r.agents.Get(ctx, e2ePath(triggerPath+"/sessions", url.Values{"limit": {"10"}}))
	if err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	sessions, err := e2eObject("trigger sessions", raw)
	if err != nil {
		return err
	}
	if items, _ := sessions["data"].([]interface{}); len(items) != 0 {
		return fmt.Errorf("trigger sessions = %d, want 0 for a never-fired trigger", len(items))
	}

	// A nil payload makes Create a plain POST with no request body, which is
	// what the pause/unpause actions take.
	raw, err = r.agents.Create(ctx, triggerPath+"/unpause", nil)
	if err != nil {
		return fmt.Errorf("unpause: %w", err)
	}
	active, err := e2eObject("unpause trigger", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("unpaused status", active["status"], "active"); err != nil {
		return err
	}

	raw, err = r.agents.Create(ctx, triggerPath+"/pause", nil)
	if err != nil {
		return fmt.Errorf("pause: %w", err)
	}
	paused, err := e2eObject("pause trigger", raw)
	if err != nil {
		return err
	}
	return e2eEqual("paused status", paused["status"], "paused")
}

// e2eFileScenario exercises file operations. The deployment deliberately denies content
// download, so a 403 is the expected outcome, not a failure.
func (r *e2eRun) e2eFileScenario(ctx context.Context) error {
	uploaded, err := r.agents.DoMultipart(ctx, http.MethodPost, "/v1/files", MultipartRequest{
		File: &MultipartFile{
			FieldName:   "file",
			FileName:    r.prefix + "-upload.txt",
			ContentType: "text/plain",
			Content:     []byte(fmt.Sprintf("orca-sdk e2e file %s\n", r.suffix)),
		},
	})
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	file, err := e2eObject("upload file", uploaded)
	if err != nil {
		return err
	}
	if r.fileID, err = e2eString("upload file", file, "id"); err != nil {
		return err
	}

	raw, err := r.agents.Get(ctx, "/v1/files/"+url.PathEscape(r.fileID))
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	retrieved, err := e2eObject("retrieve file", raw)
	if err != nil {
		return err
	}
	if err := e2eEqual("retrieved id", retrieved["id"], r.fileID); err != nil {
		return err
	}

	if err := r.e2eListContains(ctx, "/v1/files", url.Values{"limit": {"100"}}, r.fileID); err != nil {
		return err
	}

	var discard bytes.Buffer
	err = r.agents.GetToWriter(ctx, "/v1/files/"+url.PathEscape(r.fileID)+"/content", &discard)
	if err == nil {
		return errors.New("download succeeded, want it to be denied with 403")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusForbidden {
		return fmt.Errorf("download error = %v, want HTTP 403", err)
	}
	return nil
}

func (r *e2eRun) e2eSessionScenario(ctx context.Context) error {
	if r.agentID == "" {
		return errors.New("agent scenario did not create an agent")
	}
	if r.environmentID == "" {
		return errors.New("environment scenario did not create an environment")
	}

	params := SessionNewParams{
		Agent:         AgentRef(r.agentID),
		EnvironmentID: r.environmentID,
		Title:         param.String(r.prefix + "-session"),
	}
	var opts []option.RequestOption
	if r.guardrailID != "" {
		params.Agent = SessionAgentParam{
			Type:         SessionAgentRefWithOverrides,
			ID:           r.agentID,
			GuardrailIDs: []string{r.guardrailID},
		}
		opts = append(opts, option.WithHeader("orca-beta", "managed-agents-2026-04-01"))
	}
	created, err := r.client.Sessions.Create(ctx, params, opts...)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if created.ID == "" {
		return errors.New("create session returned an empty id")
	}
	r.sessionID = created.ID

	retrieved, err := r.client.Sessions.Get(ctx, r.sessionID)
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}
	if retrieved.ID != r.sessionID {
		return fmt.Errorf("retrieved id = %q, want %q", retrieved.ID, r.sessionID)
	}

	page, err := r.client.Sessions.List(ctx, SessionListParams{
		AgentID: param.String(r.agentID),
		Limit:   param.Int(100),
	})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	for _, session := range page.Data {
		if session.ID == r.sessionID {
			return nil
		}
	}
	return fmt.Errorf("session %s is not in the listing", r.sessionID)
}

// e2eExecutionScenario sends a prompt carrying a unique marker,
// poll the event log until the agent echoes it, then prove the SSE stream
// replays the same reply from the beginning of the log.
func (r *e2eRun) e2eExecutionScenario(ctx context.Context) error {
	if r.sessionID == "" {
		return errors.New("session scenario did not create a session")
	}

	marker := "KIND_HELM_SDK_" + r.suffix
	expectedReply := "MISSING_ECHO_TOOL " + marker

	sent, err := r.agents.Create(ctx, "/v1/sessions/"+url.PathEscape(r.sessionID)+"/events",
		map[string]interface{}{
			"events": []interface{}{
				map[string]interface{}{
					"type": "user.message",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Return the deterministic marker " + marker + ".",
						},
					},
				},
			},
		})
	if err != nil {
		return fmt.Errorf("send user.message: %w", err)
	}
	appended, err := e2eObject("send user.message", sent)
	if err != nil {
		return err
	}
	if items, _ := appended["data"].([]interface{}); len(items) != 1 {
		return fmt.Errorf("appended events = %d, want 1", len(items))
	}

	if err := r.e2eWaitForAgentReply(ctx, expectedReply); err != nil {
		return err
	}
	return r.e2eAssertSSEReplay(ctx, expectedReply)
}

// e2eWaitForAgentReply polls the event log until an agent.message carries the
// expected reply, a terminal session error appears, or the deadline passes.
func (r *e2eRun) e2eWaitForAgentReply(ctx context.Context, expectedReply string) error {
	terminal := map[string]bool{
		"session.error":        true,
		"session.status_error": true,
		"session.setup_failed": true,
	}
	query := url.Values{"limit": {"1000"}, "order": {"asc"}}
	path := e2ePath("/v1/sessions/"+url.PathEscape(r.sessionID)+"/events", query)

	deadline := time.Now().Add(e2eReplyDeadline)
	for time.Now().Before(deadline) {
		raw, err := r.agents.Get(ctx, path)
		if err != nil {
			return fmt.Errorf("list events: %w", err)
		}
		page, err := e2eObject("list events", raw)
		if err != nil {
			return err
		}
		items, _ := page["data"].([]interface{})
		for _, entry := range items {
			event, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			eventType, _ := event["type"].(string)
			encoded, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("re-encoding event: %w", err)
			}
			if eventType == "agent.message" && strings.Contains(string(encoded), expectedReply) {
				return nil
			}
			if terminal[eventType] {
				return fmt.Errorf("Managed Agents execution emitted %s: %s", eventType, encoded)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(e2eReplyPoll):
		}
	}
	return fmt.Errorf("timed out waiting for deterministic reply: %s", expectedReply)
}

// errE2EReplayFound stops the SSE read once the expected reply has arrived.
var errE2EReplayFound = errors.New("e2e: SSE replay found the deterministic reply")

// e2eAssertSSEReplay replays the session stream from the start of the log and
// requires the deterministic reply to appear in it.
func (r *e2eRun) e2eAssertSSEReplay(ctx context.Context, expectedReply string) error {
	streamCtx, cancel := context.WithTimeout(ctx, e2eReplayTimeout)
	defer cancel()

	path := e2ePath("/v1/sessions/"+url.PathEscape(r.sessionID)+"/events/stream",
		url.Values{"from_cursor": {"0"}})

	found := false
	err := r.agents.GetStream(streamCtx, path, "text/event-stream", func(body io.Reader) error {
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			if strings.Contains(line, expectedReply) {
				found = true
				return errE2EReplayFound
			}
		}
		return scanner.Err()
	})
	if found {
		return nil
	}
	if err != nil && !errors.Is(err, errE2EReplayFound) {
		return fmt.Errorf("SSE replay: %w", err)
	}
	return errors.New("SSE replay did not include the deterministic reply")
}

// e2eCloudScenario exercises cloud provider and connection discovery.
func (r *e2eRun) e2eCloudScenario(ctx context.Context) error {
	providers := NewProvidersClient(r.client)

	listed, err := providers.List(ctx)
	if err != nil {
		return fmt.Errorf("providers list: %w", err)
	}
	found := false
	for _, provider := range listed {
		if provider.Name == "orca-managed-agents" {
			found = true
			break
		}
	}
	if !found {
		return errors.New("provider list does not include orca-managed-agents")
	}

	provider, err := providers.Get(ctx, "orca-managed-agents")
	if err != nil {
		return fmt.Errorf("provider retrieve: %w", err)
	}
	if err := e2eEqual("provider name", provider.Name, "orca-managed-agents"); err != nil {
		return err
	}
	if !provider.APIKeyConfigured {
		return errors.New("provider api_key_configured = false, want true")
	}

	if _, err := NewConnectionsClient(r.client).List(ctx); err != nil {
		return fmt.Errorf("connections list: %w", err)
	}
	return nil
}

// e2eCleanup is the final scenario when strict, and the always-runs safety net
// otherwise. It tears resources down in dependency order — trigger, session,
// agent, environment, file — and aggregates its own failures so one stuck
// resource does not strand the rest.
func (r *e2eRun) e2eCleanup(ctx context.Context, strict bool) error {
	var failures []error
	step := func(name string, operation func() error) {
		if err := operation(); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
		}
	}

	if r.triggerID != "" {
		id := r.triggerID
		step("delete trigger", func() error {
			raw, err := r.agents.Delete(ctx, "/v1/triggers/"+url.PathEscape(id))
			if err != nil {
				return err
			}
			tombstone, err := e2eObject("delete trigger", raw)
			if err != nil {
				return err
			}
			if tombstone["id"] != id || tombstone["type"] != "trigger_deleted" {
				return fmt.Errorf("tombstone = %v, want {id:%q, type:\"trigger_deleted\"}", tombstone, id)
			}
			r.triggerID = ""
			return nil
		})
	}
	if r.sessionID != "" {
		id := r.sessionID
		step("archive session", func() error {
			return r.e2eArchive(ctx, "/v1/sessions/"+url.PathEscape(id)+"/archive", id, func() { r.sessionID = "" })
		})
	}
	if r.agentID != "" {
		id := r.agentID
		if r.guardrailID != "" {
			step("clear agent guardrails", func() error {
				updated, err := r.client.Agents.Update(ctx, id, AgentUpdateParams{
					GuardrailIDs: param.Null[[]string](),
				}, option.WithHeader("orca-beta", "managed-agents-2026-04-01"))
				if err != nil {
					return err
				}
				if updated.GuardrailIDs == nil || len(updated.GuardrailIDs) != 0 {
					return fmt.Errorf("guardrail_ids = %v, want []", updated.GuardrailIDs)
				}
				return nil
			})
		}
		step("archive agent", func() error {
			return r.e2eArchive(ctx, "/v1/agents/"+url.PathEscape(id)+"/archive", id, func() { r.agentID = "" })
		})
	}
	if r.guardrailID != "" {
		id := r.guardrailID
		step("archive guardrail", func() error {
			archived, err := r.client.Guardrails.Archive(ctx, id)
			if err != nil {
				return err
			}
			if archived.ArchivedAt == nil {
				return errors.New("archived_at is null after archiving the guardrail")
			}
			return nil
		})
		step("delete guardrail", func() error {
			deleted, err := r.client.Guardrails.Delete(ctx, id)
			if err != nil {
				return err
			}
			if deleted.ID != id || deleted.Type != "guardrail_deleted" {
				return fmt.Errorf("tombstone = %#v, want guardrail_deleted for %s", deleted, id)
			}
			r.guardrailID = ""
			return nil
		})
	}
	if r.environmentID != "" {
		id := r.environmentID
		step("archive environment", func() error {
			return r.e2eArchive(ctx, "/v1/environments/"+url.PathEscape(id)+"/archive", id, func() { r.environmentID = "" })
		})
	}
	if r.fileID != "" {
		id := r.fileID
		step("delete file", func() error {
			raw, err := r.agents.Delete(ctx, "/v1/files/"+url.PathEscape(id))
			if err != nil {
				return err
			}
			tombstone, err := e2eObject("delete file", raw)
			if err != nil {
				return err
			}
			if tombstone["id"] != id || tombstone["type"] != "file_deleted" {
				return fmt.Errorf("tombstone = %v, want {id:%q, type:\"file_deleted\"}", tombstone, id)
			}
			r.fileID = ""
			return nil
		})
	}

	if len(failures) == 0 {
		r.cleanupSucceeded = true
		return nil
	}
	if strict {
		return errors.Join(failures...)
	}
	for _, failure := range failures {
		r.t.Logf("[CLEANUP FAIL] %v", failure)
	}
	return nil
}

// e2eArchive archives one resource and checks the response echoes its ID.
func (r *e2eRun) e2eArchive(ctx context.Context, path, id string, clear func()) error {
	raw, err := r.agents.Archive(ctx, path)
	if err != nil {
		return err
	}
	archived, err := e2eObject("archive "+id, raw)
	if err != nil {
		return err
	}
	if archived["id"] != id {
		return fmt.Errorf("archived id = %v, want %q", archived["id"], id)
	}
	clear()
	return nil
}

// ---------------------------------------------------------------------------
// The run
// ---------------------------------------------------------------------------

func TestE2ESDKExercisesDeployedTopology(t *testing.T) {
	run := e2eEnvironment(t)

	// The TypeScript test carries its own 300s budget; here the budget is the
	// `go test -timeout` flag, and this context bounds every request under it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// The safety-net cleanup runs whatever happened above, and is a no-op once
	// the final scenario has already torn everything down.
	defer func() {
		if run.cleanupSucceeded {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
		defer cleanupCancel()
		_ = run.e2eCleanup(cleanupCtx, false)
	}()

	run.scenario("extension discovery matches the topology", func() error {
		return run.e2eDiscovery(ctx)
	})
	if !run.expectCloud {
		run.scenario("guardrail create, list types, list, update, and retrieve", func() error {
			return run.e2eGuardrailScenario(ctx)
		})
		run.scenario("model price list and retrieve", func() error {
			return run.e2eModelPriceScenario(ctx)
		})
	}
	run.scenario("environment create, list, update, and retrieve", func() error {
		return run.e2eEnvironmentScenario(ctx)
	})
	run.scenario("agent create, list, update, and retrieve", func() error {
		return run.e2eAgentScenario(ctx)
	})
	run.scenario("trigger create, list, update, actions, and session history", func() error {
		return run.e2eTriggerScenario(ctx)
	})
	run.scenario("file upload, list, retrieve, and denied download", func() error {
		return run.e2eFileScenario(ctx)
	})
	run.scenario("session create, list, and retrieve", func() error {
		return run.e2eSessionScenario(ctx)
	})
	if run.expectExecution {
		run.scenario("deterministic execution and SSE replay", func() error {
			return run.e2eExecutionScenario(ctx)
		})
	}
	if run.expectCloud {
		run.scenario("cloud provider and connection discovery", func() error {
			return run.e2eCloudScenario(ctx)
		})
	}
	run.scenario("resource archival and deletion", func() error {
		return run.e2eCleanup(ctx, true)
	})

	// Report every scenario failure at once: the point of aggregating is that a
	// single run tells you the state of the whole topology, not just the first
	// thing that broke.
	if len(run.failures) > 0 {
		for _, failure := range run.failures {
			t.Error(failure)
		}
		t.Fatalf("%d SDK E2E scenario(s) failed", len(run.failures))
	}
}
