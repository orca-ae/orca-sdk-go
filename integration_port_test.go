// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ported from orca-sdk-typescript tests/integration/*.int.test.ts (13 files)
// and their tests/integration/setup.ts bootstrap.
//
// These talk to a live deployment, so they are credential-gated exactly like
// the TypeScript suite: setup.ts swaps `describe` for `describe.skip` when
// ORCA_TEST_API_KEY is missing, and the Go equivalent is t.Skip from
// integrationClient. `go test ./...` therefore stays green on a clean
// checkout, and `ORCA_TEST_API_KEY=... go test -run TestIntegration ./...`
// runs the round-trips.
//
// The typed Managed Agents resources do not exist in this SDK yet, so the core
// round-trips go through ManagedAgentsClient, the untyped passthrough in
// managed_agents.go. That still exercises the real thing the TypeScript tests
// exercise — paths, methods, bodies and lifecycle — just without static types
// on the responses. The cloud extension and discovery suites use the typed Go
// clients, which do exist.

const (
	// integrationAPIKeyEnv holds the credential. Absent => the suite skips.
	// The TypeScript client sends this as `Authorization: Bearer <key>`, so the
	// Go port builds a bearer client rather than an x-api-key one.
	integrationAPIKeyEnv = "ORCA_TEST_API_KEY"

	// integrationBaseURLEnv overrides the deployment under test.
	integrationBaseURLEnv = "ORCA_TEST_BASE_URL"

	// integrationDefaultBaseURL is the shared test cluster tests/integration/setup.ts
	// falls back to when ORCA_TEST_BASE_URL is unset.
	integrationDefaultBaseURL = "https://fw-077c87384604.gcp-shared-usc1-test.test.g.sn2.dev"

	// integrationSkipReason is reported when credentials are absent. It is
	// deliberately NOT pendingManagedAgents: these tests are gated on a
	// credential, not on unimplemented typed resources, and must not inflate
	// the pending count.
	integrationSkipReason = "integration: set " + integrationAPIKeyEnv + " to run against a live deployment"

	// integrationTimeout mirrors the 60s jest.setTimeout in setup.ts.
	integrationTimeout = 60 * time.Second

	// integrationMaxPages bounds a list walk so a server that keeps echoing the
	// same next_page cursor fails the test instead of hanging it.
	integrationMaxPages = 100
)

// integrationClient returns a live client plus the untyped Managed Agents
// passthrough, or skips when no credential is configured.
func integrationClient(t *testing.T) (*Client, *ManagedAgentsClient) {
	t.Helper()

	apiKey := strings.TrimSpace(os.Getenv(integrationAPIKeyEnv))
	if apiKey == "" {
		t.Skip(integrationSkipReason)
	}

	baseURL := strings.TrimSpace(os.Getenv(integrationBaseURLEnv))
	if baseURL == "" {
		baseURL = integrationDefaultBaseURL
	}

	client, err := NewClient(baseURL, apiKey, &http.Client{Timeout: integrationTimeout})
	if err != nil {
		t.Fatalf("NewClient(%q) error = %v", baseURL, err)
	}
	return client, NewManagedAgentsClient(client)
}

// integrationContext returns a context bounded by the suite timeout.
func integrationContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	t.Cleanup(cancel)
	return ctx
}

var (
	integrationPrefixOnce  sync.Once
	integrationPrefixValue string
)

// integrationPrefix is the run-scoped namespace getTestPrefix() provides in
// setup.ts, so parallel runs and cleanup can recognize their own resources.
func integrationPrefix() string {
	integrationPrefixOnce.Do(func() {
		integrationPrefixValue = fmt.Sprintf("it-%d-%d", time.Now().UnixMilli(), os.Getpid())
	})
	return integrationPrefixValue
}

// integrationNotImplemented reports whether err is the "this registry does not
// serve that surface yet" signal the memory-store and thread suites treat as a
// soft skip rather than a failure.
func integrationNotImplemented(err error) bool {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.StatusCode == http.StatusNotFound || httpErr.StatusCode == http.StatusNotImplemented
}

// integrationStatus returns the HTTP status carried by err, or 0.
func integrationStatus(err error) int {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}

// integrationObject asserts that an untyped response is a JSON object.
func integrationObject(t *testing.T, what string, value interface{}) map[string]interface{} {
	t.Helper()

	object, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("%s: response type = %T, want a JSON object", what, value)
	}
	return object
}

// integrationField returns a required string field of an untyped response.
func integrationField(t *testing.T, what string, object map[string]interface{}, key string) string {
	t.Helper()

	value, ok := object[key].(string)
	if !ok || value == "" {
		t.Fatalf("%s: %s = %v, want a non-empty string", what, key, object[key])
	}
	return value
}

// integrationPath joins a collection path and its query parameters.
func integrationPath(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// integrationWalk pages through a page-token list endpoint, calling visit for
// each item until it returns false or the pages run out. It is the Go
// equivalent of `for await (const item of client.x.list())`.
func integrationWalk(
	ctx context.Context,
	agents *ManagedAgentsClient,
	path string,
	query url.Values,
	visit func(item map[string]interface{}) bool,
) error {
	cursor := url.Values{}
	for key, values := range query {
		cursor[key] = append([]string(nil), values...)
	}

	for page := 0; page < integrationMaxPages; page++ {
		raw, err := agents.Get(ctx, integrationPath(path, cursor))
		if err != nil {
			return err
		}
		body, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("GET %s: response type = %T, want a JSON object", path, raw)
		}
		items, _ := body["data"].([]interface{})
		for _, entry := range items {
			item, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if !visit(item) {
				return nil
			}
		}

		next, _ := body["next_page"].(string)
		if next == "" || next == cursor.Get("page") {
			return nil
		}
		cursor.Set("page", next)
	}
	return fmt.Errorf("GET %s: pagination did not terminate within %d pages", path, integrationMaxPages)
}

// integrationCount returns how many items in a listing satisfy match.
func integrationCount(
	ctx context.Context,
	t *testing.T,
	agents *ManagedAgentsClient,
	path string,
	query url.Values,
	match func(item map[string]interface{}) bool,
) int {
	t.Helper()

	found := 0
	if err := integrationWalk(ctx, agents, path, query, func(item map[string]interface{}) bool {
		if match(item) {
			found++
		}
		return true
	}); err != nil {
		t.Fatalf("listing %s: %v", path, err)
	}
	return found
}

// integrationArchiveByPrefix archives every item in a collection whose field
// starts with the run prefix. Best-effort, exactly like the cleanup helpers in
// setup.ts: a partial failure must not fail the suite.
func integrationArchiveByPrefix(
	t *testing.T,
	agents *ManagedAgentsClient,
	path string,
	field string,
	prefix string,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	var ids []string
	if err := integrationWalk(ctx, agents, path, nil, func(item map[string]interface{}) bool {
		name, _ := item[field].(string)
		id, _ := item["id"].(string)
		if id != "" && strings.HasPrefix(name, prefix) {
			ids = append(ids, id)
		}
		return true
	}); err != nil {
		t.Logf("cleanup: listing %s failed: %v", path, err)
		return
	}

	for _, id := range ids {
		if _, err := agents.Archive(ctx, path+"/"+url.PathEscape(id)+"/archive"); err != nil {
			t.Logf("cleanup: archiving %s/%s failed: %v", path, id, err)
		}
	}
}

// integrationCloudAvailable reports whether the deployment advertises the
// cloud.sn.io extension group — the Go equivalent of supportsExtension().
func integrationCloudAvailable(ctx context.Context, t *testing.T, client *Client) bool {
	t.Helper()

	groups, err := client.GetAPIGroups(ctx)
	if err != nil {
		if integrationNotImplemented(err) {
			return false
		}
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
	return groups.HasGroup(CloudExtensionGroup)
}

// ---------------------------------------------------------------------------
// agents.int.test.ts — create, list, retrieve, update, archive, verify absence
// ---------------------------------------------------------------------------

func TestIntegrationAgents(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)
	prefix := integrationPrefix()
	name := prefix + "-agent"

	t.Cleanup(func() { integrationArchiveByPrefix(t, agents, "/v1/agents", "name", prefix) })

	created, err := agents.Create(ctx, "/v1/agents", map[string]interface{}{
		"name":        name,
		"model":       "claude-sonnet-4-6",
		"description": "Integration test agent",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	agent := integrationObject(t, "create agent", created)
	agentID := integrationField(t, "create agent", agent, "id")
	if got := agent["name"]; got != name {
		t.Errorf("created name = %v, want %q", got, name)
	}
	if got := agent["type"]; got != "agent" {
		t.Errorf("created type = %v, want \"agent\"", got)
	}
	if got, ok := agent["version"].(float64); !ok || got != 1 {
		t.Errorf("created version = %v, want 1", agent["version"])
	}

	t.Run("list", func(t *testing.T) {
		found := integrationCount(ctx, t, agents, "/v1/agents", nil, func(item map[string]interface{}) bool {
			return item["id"] == agentID
		})
		if found < 1 {
			t.Errorf("created agent %s not found in listing", agentID)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/agents/"+url.PathEscape(agentID))
		if err != nil {
			t.Fatalf("retrieve agent: %v", err)
		}
		fetched := integrationObject(t, "retrieve agent", raw)
		if fetched["id"] != agentID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], agentID)
		}
		if fetched["name"] != name {
			t.Errorf("retrieved name = %v, want %q", fetched["name"], name)
		}
	})

	t.Run("update", func(t *testing.T) {
		// Update is POST, not PUT, for every core resource.
		raw, err := agents.Update(ctx, http.MethodPost, "/v1/agents/"+url.PathEscape(agentID),
			map[string]interface{}{"version": agent["version"], "description": "Updated by integration test"})
		if err != nil {
			t.Fatalf("update agent: %v", err)
		}
		updated := integrationObject(t, "update agent", raw)
		if updated["description"] != "Updated by integration test" {
			t.Errorf("updated description = %v", updated["description"])
		}
		before, _ := agent["version"].(float64)
		after, _ := updated["version"].(float64)
		if after <= before {
			t.Errorf("updated version = %v, want greater than %v", after, before)
		}
	})

	t.Run("archive then absent from the default listing", func(t *testing.T) {
		if _, err := agents.Archive(ctx, "/v1/agents/"+url.PathEscape(agentID)+"/archive"); err != nil {
			t.Fatalf("archive agent: %v", err)
		}
		found := integrationCount(ctx, t, agents, "/v1/agents", nil, func(item map[string]interface{}) bool {
			return item["id"] == agentID
		})
		if found != 0 {
			t.Errorf("archived agent %s still visible without include_archived", agentID)
		}
	})
}

// ---------------------------------------------------------------------------
// environments.int.test.ts
// ---------------------------------------------------------------------------

func TestIntegrationEnvironments(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)
	prefix := integrationPrefix()
	name := prefix + "-env"

	t.Cleanup(func() { integrationArchiveByPrefix(t, agents, "/v1/environments", "name", prefix) })

	created, err := agents.Create(ctx, "/v1/environments", map[string]interface{}{
		"name":        name,
		"description": "Integration test environment",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	env := integrationObject(t, "create environment", created)
	envID := integrationField(t, "create environment", env, "id")
	if env["name"] != name {
		t.Errorf("created name = %v, want %q", env["name"], name)
	}
	if env["type"] != "environment" {
		t.Errorf("created type = %v, want \"environment\"", env["type"])
	}

	t.Run("list", func(t *testing.T) {
		found := integrationCount(ctx, t, agents, "/v1/environments", nil, func(item map[string]interface{}) bool {
			return item["id"] == envID
		})
		if found < 1 {
			t.Errorf("created environment %s not found in listing", envID)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/environments/"+url.PathEscape(envID))
		if err != nil {
			t.Fatalf("retrieve environment: %v", err)
		}
		if fetched := integrationObject(t, "retrieve environment", raw); fetched["id"] != envID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], envID)
		}
	})

	t.Run("update", func(t *testing.T) {
		raw, err := agents.Update(ctx, http.MethodPost, "/v1/environments/"+url.PathEscape(envID),
			map[string]interface{}{"description": "Updated by integration test"})
		if err != nil {
			t.Fatalf("update environment: %v", err)
		}
		if updated := integrationObject(t, "update environment", raw); updated["description"] != "Updated by integration test" {
			t.Errorf("updated description = %v", updated["description"])
		}
	})

	t.Run("archive then absent from the default listing", func(t *testing.T) {
		if _, err := agents.Archive(ctx, "/v1/environments/"+url.PathEscape(envID)+"/archive"); err != nil {
			t.Fatalf("archive environment: %v", err)
		}
		found := integrationCount(ctx, t, agents, "/v1/environments", nil, func(item map[string]interface{}) bool {
			return item["id"] == envID
		})
		if found != 0 {
			t.Errorf("archived environment %s still visible without include_archived", envID)
		}
	})
}

// ---------------------------------------------------------------------------
// vaults.int.test.ts
// ---------------------------------------------------------------------------

func TestIntegrationVaults(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)
	prefix := integrationPrefix()
	displayName := prefix + "-vault"

	t.Cleanup(func() { integrationArchiveByPrefix(t, agents, "/v1/vaults", "display_name", prefix) })

	created, err := agents.Create(ctx, "/v1/vaults", map[string]interface{}{"display_name": displayName})
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	vault := integrationObject(t, "create vault", created)
	vaultID := integrationField(t, "create vault", vault, "id")
	if vault["type"] != "vault" {
		t.Errorf("created type = %v, want \"vault\"", vault["type"])
	}
	if vault["display_name"] != displayName {
		t.Errorf("created display_name = %v, want %q", vault["display_name"], displayName)
	}

	t.Run("list", func(t *testing.T) {
		found := integrationCount(ctx, t, agents, "/v1/vaults", nil, func(item map[string]interface{}) bool {
			return item["id"] == vaultID
		})
		if found < 1 {
			t.Errorf("created vault %s not found in listing", vaultID)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/vaults/"+url.PathEscape(vaultID))
		if err != nil {
			t.Fatalf("retrieve vault: %v", err)
		}
		fetched := integrationObject(t, "retrieve vault", raw)
		if fetched["id"] != vaultID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], vaultID)
		}
		if fetched["display_name"] != displayName {
			t.Errorf("retrieved display_name = %v, want %q", fetched["display_name"], displayName)
		}
	})

	t.Run("archive then absent from the default listing", func(t *testing.T) {
		if _, err := agents.Archive(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/archive"); err != nil {
			t.Fatalf("archive vault: %v", err)
		}
		found := integrationCount(ctx, t, agents, "/v1/vaults", nil, func(item map[string]interface{}) bool {
			return item["id"] == vaultID
		})
		if found != 0 {
			t.Errorf("archived vault %s still visible without include_archived", vaultID)
		}
	})
}

// ---------------------------------------------------------------------------
// files.int.test.ts — upload, list, retrieve, download, delete
// ---------------------------------------------------------------------------

func TestIntegrationFiles(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)
	prefix := integrationPrefix()
	const content = "hello integration test"

	uploaded, err := agents.DoMultipart(ctx, http.MethodPost, "/v1/files", MultipartRequest{
		File: &MultipartFile{
			FieldName:   "file",
			FileName:    prefix + "-hi.txt",
			ContentType: "text/plain",
			Content:     []byte(content),
		},
	})
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	meta := integrationObject(t, "upload file", uploaded)
	fileID := integrationField(t, "upload file", meta, "id")

	deleted := false
	t.Cleanup(func() {
		if deleted {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := agents.Delete(cleanupCtx, "/v1/files/"+url.PathEscape(fileID)); err != nil {
			t.Logf("cleanup: deleting file %s failed: %v", fileID, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		// Files use ID-cursor pagination (limit/after_id/before_id -> has_more/
		// first_id/last_id), not the page token the other collections use, so
		// integrationWalk's next_page follow does not apply. One page is enough
		// to prove the upload is listed.
		raw, err := agents.Get(ctx, integrationPath("/v1/files", url.Values{"limit": {"100"}}))
		if err != nil {
			t.Fatalf("list files: %v", err)
		}
		page := integrationObject(t, "list files", raw)
		items, _ := page["data"].([]interface{})
		found := false
		for _, entry := range items {
			if item, ok := entry.(map[string]interface{}); ok && item["id"] == fileID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("uploaded file %s not found in the first page of the listing", fileID)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/files/"+url.PathEscape(fileID))
		if err != nil {
			t.Fatalf("retrieve file: %v", err)
		}
		if fetched := integrationObject(t, "retrieve file", raw); fetched["id"] != fileID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], fileID)
		}
	})

	t.Run("download", func(t *testing.T) {
		var body bytes.Buffer
		if err := agents.GetToWriter(ctx, "/v1/files/"+url.PathEscape(fileID)+"/content", &body); err != nil {
			t.Fatalf("download file: %v", err)
		}
		if body.String() != content {
			t.Errorf("downloaded content = %q, want %q", body.String(), content)
		}
	})

	t.Run("delete", func(t *testing.T) {
		raw, err := agents.Delete(ctx, "/v1/files/"+url.PathEscape(fileID))
		if err != nil {
			t.Fatalf("delete file: %v", err)
		}
		tombstone := integrationObject(t, "delete file", raw)
		if tombstone["id"] != fileID || tombstone["type"] != "file_deleted" {
			t.Errorf("tombstone = %v, want {id:%q, type:\"file_deleted\"}", tombstone, fileID)
		}
		deleted = true
	})
}

// integrationSessionFixture creates the agent, environment and session the
// session, thread and streaming suites all need, and registers their cleanup.
func integrationSessionFixture(
	ctx context.Context,
	t *testing.T,
	agents *ManagedAgentsClient,
	kind string,
) (agentID, sessionID string) {
	t.Helper()

	prefix := integrationPrefix()
	t.Cleanup(func() {
		integrationArchiveByPrefix(t, agents, "/v1/agents", "name", prefix)
		integrationArchiveByPrefix(t, agents, "/v1/environments", "name", prefix)
	})

	createdAgent, err := agents.Create(ctx, "/v1/agents", map[string]interface{}{
		"name":  fmt.Sprintf("%s-%s-agent", prefix, kind),
		"model": "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	agentID = integrationField(t, "create agent", integrationObject(t, "create agent", createdAgent), "id")

	createdEnv, err := agents.Create(ctx, "/v1/environments", map[string]interface{}{
		"name": fmt.Sprintf("%s-%s-env", prefix, kind),
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	envID := integrationField(t, "create environment", integrationObject(t, "create environment", createdEnv), "id")

	createdSession, err := agents.Create(ctx, "/v1/sessions", map[string]interface{}{
		"agent":          agentID,
		"environment_id": envID,
		"title":          fmt.Sprintf("%s-%s-session", prefix, kind),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	session := integrationObject(t, "create session", createdSession)
	sessionID = integrationField(t, "create session", session, "id")

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := agents.Archive(cleanupCtx, "/v1/sessions/"+url.PathEscape(sessionID)+"/archive"); err != nil {
			t.Logf("cleanup: archiving session %s failed: %v", sessionID, err)
		}
	})

	return agentID, sessionID
}

// ---------------------------------------------------------------------------
// sessions.int.test.ts
// ---------------------------------------------------------------------------

func TestIntegrationSessions(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)

	agentID, sessionID := integrationSessionFixture(ctx, t, agents, "sessions")
	byAgent := url.Values{"agent_id": {agentID}}

	t.Run("create is visible in the agent-scoped listing", func(t *testing.T) {
		found := integrationCount(ctx, t, agents, "/v1/sessions", byAgent, func(item map[string]interface{}) bool {
			return item["id"] == sessionID
		})
		if found < 1 {
			t.Errorf("created session %s not found in listing", sessionID)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/sessions/"+url.PathEscape(sessionID))
		if err != nil {
			t.Fatalf("retrieve session: %v", err)
		}
		fetched := integrationObject(t, "retrieve session", raw)
		if fetched["id"] != sessionID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], sessionID)
		}
		if fetched["type"] != "session" {
			t.Errorf("retrieved type = %v, want \"session\"", fetched["type"])
		}
	})

	t.Run("files.list returns a page", func(t *testing.T) {
		raw, err := agents.Get(ctx,
			integrationPath("/v1/sessions/"+url.PathEscape(sessionID)+"/files", url.Values{"limit": {"10"}}))
		if err != nil {
			t.Fatalf("list session files: %v", err)
		}
		page := integrationObject(t, "list session files", raw)
		if _, ok := page["data"].([]interface{}); !ok {
			t.Errorf("session files page data = %v, want an array", page["data"])
		}
	})

	t.Run("update", func(t *testing.T) {
		title := integrationPrefix() + "-session-updated"
		raw, err := agents.Update(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(sessionID),
			map[string]interface{}{"title": title})
		if err != nil {
			t.Fatalf("update session: %v", err)
		}
		if updated := integrationObject(t, "update session", raw); updated["title"] != title {
			t.Errorf("updated title = %v, want %q", updated["title"], title)
		}
	})

	t.Run("archive then absent from the default listing", func(t *testing.T) {
		if _, err := agents.Archive(ctx, "/v1/sessions/"+url.PathEscape(sessionID)+"/archive"); err != nil {
			t.Fatalf("archive session: %v", err)
		}
		found := integrationCount(ctx, t, agents, "/v1/sessions", byAgent, func(item map[string]interface{}) bool {
			return item["id"] == sessionID
		})
		if found != 0 {
			t.Errorf("archived session %s still visible without include_archived", sessionID)
		}
	})
}

// ---------------------------------------------------------------------------
// threads.int.test.ts — threads are coordinator-spawned, so there is no create
// ---------------------------------------------------------------------------

func TestIntegrationSessionThreads(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)

	_, sessionID := integrationSessionFixture(ctx, t, agents, "threads")
	threadsPath := "/v1/sessions/" + url.PathEscape(sessionID) + "/threads"

	var first map[string]interface{}
	if err := integrationWalk(ctx, agents, threadsPath, nil, func(item map[string]interface{}) bool {
		first = item
		return false
	}); err != nil {
		if !integrationNotImplemented(err) {
			t.Fatalf("list threads: %v", err)
		}
		t.Skip("integration: registry does not expose thread endpoints yet")
	}

	if first == nil {
		// A server that supports threads may not have produced one yet; an
		// empty listing is still a successful contract check.
		t.Log("integration: no threads produced yet — skipping retrieve/archive")
		return
	}
	threadID := integrationField(t, "list threads", first, "id")

	t.Run("retrieve", func(t *testing.T) {
		raw, err := agents.Get(ctx, threadsPath+"/"+url.PathEscape(threadID))
		if err != nil {
			t.Fatalf("retrieve thread: %v", err)
		}
		fetched := integrationObject(t, "retrieve thread", raw)
		if fetched["id"] != threadID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], threadID)
		}
		if fetched["session_id"] != sessionID {
			t.Errorf("retrieved session_id = %v, want %q", fetched["session_id"], sessionID)
		}
	})

	t.Run("archive is best-effort", func(t *testing.T) {
		// A primary thread may refuse to be archived; 400/404/409/501 are all
		// acceptable answers here.
		if _, err := agents.Archive(ctx, threadsPath+"/"+url.PathEscape(threadID)+"/archive"); err != nil {
			switch integrationStatus(err) {
			case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusNotImplemented:
				t.Logf("integration: thread %s is not archivable: %v", threadID, err)
			default:
				t.Fatalf("archive thread: %v", err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// memory-stores.int.test.ts — 404/501 means "not served yet", not a failure
// ---------------------------------------------------------------------------

func TestIntegrationMemoryStores(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)
	prefix := integrationPrefix()
	name := prefix + "-store"

	created, err := agents.Create(ctx, "/v1/memory_stores", map[string]interface{}{
		"name":        name,
		"description": "integration-test store",
	})
	if err != nil {
		if !integrationNotImplemented(err) {
			t.Fatalf("create memory store: %v", err)
		}
		t.Skip("integration: registry does not expose memory-store endpoints yet")
	}
	store := integrationObject(t, "create memory store", created)
	storeID := integrationField(t, "create memory store", store, "id")
	if store["type"] != "memory_store" {
		t.Errorf("created type = %v, want \"memory_store\"", store["type"])
	}
	if store["name"] != name {
		t.Errorf("created name = %v, want %q", store["name"], name)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := agents.Archive(cleanupCtx, "/v1/memory_stores/"+url.PathEscape(storeID)+"/archive"); err != nil {
			t.Logf("cleanup: archiving memory store %s failed: %v", storeID, err)
		}
	})

	t.Run("retrieve and update round-trip", func(t *testing.T) {
		raw, err := agents.Get(ctx, "/v1/memory_stores/"+url.PathEscape(storeID))
		if err != nil {
			t.Fatalf("retrieve memory store: %v", err)
		}
		if fetched := integrationObject(t, "retrieve memory store", raw); fetched["id"] != storeID {
			t.Errorf("retrieved id = %v, want %q", fetched["id"], storeID)
		}

		raw, err = agents.Update(ctx, http.MethodPost, "/v1/memory_stores/"+url.PathEscape(storeID),
			map[string]interface{}{"description": "updated description"})
		if err != nil {
			t.Fatalf("update memory store: %v", err)
		}
		if updated := integrationObject(t, "update memory store", raw); updated["description"] != "updated description" {
			t.Errorf("updated description = %v", updated["description"])
		}
	})

	t.Run("list", func(t *testing.T) {
		found := integrationCount(ctx, t, agents, "/v1/memory_stores", nil, func(item map[string]interface{}) bool {
			return item["id"] == storeID
		})
		if found < 1 {
			t.Errorf("created memory store %s not found in listing", storeID)
		}
	})
}

// ---------------------------------------------------------------------------
// skills.int.test.ts — the registry may hold no skills; reachability is enough
// ---------------------------------------------------------------------------

func TestIntegrationSkills(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)

	seen := 0
	if err := integrationWalk(ctx, agents, "/v1/skills", nil, func(map[string]interface{}) bool {
		seen++
		// Cap the walk so a large registry does not slow the suite down.
		return seen < 10
	}); err != nil {
		t.Fatalf("list skills: %v", err)
	}
	t.Logf("integration: skills listing returned %d item(s)", seen)
}

// ---------------------------------------------------------------------------
// triggers.int.test.ts — core triggers, not a cloud extension
// ---------------------------------------------------------------------------

func TestIntegrationTriggers(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)

	seen := 0
	if err := integrationWalk(ctx, agents, "/v1/triggers", url.Values{"limit": {"10"}},
		func(map[string]interface{}) bool {
			seen++
			return true
		}); err != nil {
		t.Fatalf("list triggers: %v", err)
	}
	t.Logf("integration: triggers listing returned %d item(s)", seen)
}

// ---------------------------------------------------------------------------
// streaming.int.test.ts — send a user.message, then read the SSE stream
// ---------------------------------------------------------------------------

// errIntegrationStreamDone stops GetStream once the first event has arrived.
var errIntegrationStreamDone = errors.New("integration: first stream event received")

func TestIntegrationStreaming(t *testing.T) {
	_, agents := integrationClient(t)
	ctx := integrationContext(t)

	_, sessionID := integrationSessionFixture(ctx, t, agents, "stream")

	if _, err := agents.Create(ctx, "/v1/sessions/"+url.PathEscape(sessionID)+"/events",
		map[string]interface{}{
			"events": []interface{}{
				map[string]interface{}{
					"type":    "user.message",
					"content": []interface{}{map[string]interface{}{"type": "text", "text": "Hello, integration test!"}},
				},
			},
		}); err != nil {
		t.Fatalf("send user.message: %v", err)
	}

	// Bound the read at 30s like the TypeScript AbortController does.
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	events := 0
	err := agents.GetStream(streamCtx, "/v1/sessions/"+url.PathEscape(sessionID)+"/events/stream",
		"text/event-stream", func(body io.Reader) error {
			scanner := bufio.NewScanner(body)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), "data:") {
					events++
					return errIntegrationStreamDone
				}
			}
			return scanner.Err()
		})
	// Breaking out of the stream early is expected, and so is the deadline
	// firing once we already have what we came for.
	if err != nil && !errors.Is(err, errIntegrationStreamDone) && events == 0 {
		t.Fatalf("stream session events: %v", err)
	}
	if events < 1 {
		t.Error("stream yielded no events within 30s of sending a user.message")
	}
}

// ---------------------------------------------------------------------------
// discovery.int.test.ts
// ---------------------------------------------------------------------------

func TestIntegrationDiscovery(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := integrationContext(t)

	groups, err := client.GetAPIGroups(ctx)
	if err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
	if groups.Kind != "APIGroupList" {
		t.Errorf("kind = %q, want APIGroupList", groups.Kind)
	}
	// An empty group list is a valid answer: a deployment with no extensions.
	t.Logf("integration: discovery advertises %d group(s)", len(groups.Groups))
}

// ---------------------------------------------------------------------------
// cloud-agent-providers.int.test.ts
// ---------------------------------------------------------------------------

func TestIntegrationCloudAgentProviders(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := integrationContext(t)

	if !integrationCloudAvailable(ctx, t, client) {
		t.Skipf("integration: deployment does not advertise the %s extension", CloudExtensionGroup)
	}

	providers, err := NewProvidersClient(client).List(ctx)
	if err != nil {
		t.Fatalf("providers.List() error = %v", err)
	}
	t.Logf("integration: %d agent provider(s) configured", len(providers))
}

// ---------------------------------------------------------------------------
// cloud-extensions.int.test.ts — read-only surface only; mutating lifecycle
// operations are covered hermetically by the unit tests
// ---------------------------------------------------------------------------

func TestIntegrationCloudExtensions(t *testing.T) {
	client, _ := integrationClient(t)
	ctx := integrationContext(t)

	if !integrationCloudAvailable(ctx, t, client) {
		t.Skipf("integration: deployment does not advertise the %s extension", CloudExtensionGroup)
	}

	t.Run("API-resource discovery", func(t *testing.T) {
		resources, err := client.GetCloudAPIResources(ctx)
		if err != nil {
			t.Fatalf("GetCloudAPIResources() error = %v", err)
		}
		if resources.Kind != "APIResourceList" {
			t.Errorf("kind = %q, want APIResourceList", resources.Kind)
		}
	})

	t.Run("catalog lists connector definitions", func(t *testing.T) {
		catalog := NewCatalogClient(client)
		if _, err := catalog.ListKafkaConnectors(ctx); err != nil {
			t.Errorf("catalog.ListKafkaConnectors() error = %v", err)
		}
		if _, err := catalog.ListSinks(ctx); err != nil {
			t.Errorf("catalog.ListSinks() error = %v", err)
		}
		if _, err := catalog.ListSources(ctx); err != nil {
			t.Errorf("catalog.ListSources() error = %v", err)
		}
	})

	t.Run("connections list", func(t *testing.T) {
		if _, err := NewConnectionsClient(client).List(ctx); err != nil {
			t.Errorf("connections.List() error = %v", err)
		}
	})

	t.Run("functions list", func(t *testing.T) {
		if _, err := NewFunctionsClient(client).List(ctx); err != nil {
			t.Errorf("functions.List() error = %v", err)
		}
	})

	t.Run("health probes", func(t *testing.T) {
		health := NewHealthClient(client)
		if _, err := health.Health(ctx); err != nil {
			t.Errorf("health.Health() error = %v", err)
		}
		if _, err := health.Ready(ctx); err != nil {
			t.Errorf("health.Ready() error = %v", err)
		}
		if _, err := health.Live(ctx); err != nil {
			t.Errorf("health.Live() error = %v", err)
		}
	})

	t.Run("packages list", func(t *testing.T) {
		if _, err := NewPackagesClient(client).List(ctx, "function"); err != nil {
			t.Errorf("packages.List() error = %v", err)
		}
	})

	t.Run("sink and source connector lists", func(t *testing.T) {
		if _, err := NewSinksClient(client).List(ctx); err != nil {
			t.Errorf("sinks.List() error = %v", err)
		}
		if _, err := NewSourcesClient(client).List(ctx); err != nil {
			t.Errorf("sources.List() error = %v", err)
		}
	})

	t.Run("Kafka Connect worker, plugins and connectors", func(t *testing.T) {
		kafka := NewKafkaConnectClient(client)
		if _, err := kafka.GetInfo(ctx); err != nil {
			t.Errorf("kafka.GetInfo() error = %v", err)
		}
		if _, err := kafka.ListPlugins(ctx); err != nil {
			t.Errorf("kafka.ListPlugins() error = %v", err)
		}
		if _, err := kafka.ListConnectors(ctx); err != nil {
			t.Errorf("kafka.ListConnectors() error = %v", err)
		}
	})
}
