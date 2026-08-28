// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/{skills,triggers}/*.test.ts.

func TestSkills(t *testing.T) {
	t.Parallel()

	t.Run("create uploads every file under files[]", func(t *testing.T) {
		t.Parallel()

		// The brackets are literal: the server reads a repeated "files[]"
		// field, not "files".
		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"skill_1"}`), nil
		})

		_, err := client.Skills.Create(context.Background(), SkillNewParams{
			Files: []SkillFile{
				{Filename: "SKILL.md", ContentType: "text/markdown", Content: []byte("# skill")},
				{Filename: "helper.py", ContentType: "text/x-python", Content: []byte("print(1)")},
			},
			DisplayTitle: param.String("My Skill"),
		})
		if err != nil {
			t.Fatalf("Skills.Create() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/skills" {
			t.Errorf("request = %s %s, want POST /v1/skills", call.Method, call.Path())
		}

		parts := multipartParts(t, call)
		if got := len(parts["files[]"]); got != 2 {
			t.Errorf("files[] parts = %d, want 2", got)
		}
		if got := len(parts["display_title"]); got != 1 {
			t.Errorf("display_title parts = %d, want 1 plain form field", got)
		}
	})

	t.Run("retrieve escapes the id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Skills.Get(context.Background(), "skill/slash"); err != nil {
			t.Fatalf("Skills.Get() error = %v", err)
		}
		if got, want := transport.Only(t).Path(), "/v1/skills/skill%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("list accepts only the portable filters", func(t *testing.T) {
		t.Parallel()

		// The overlay removes source, include_archived and provider.
		fields := structFieldNames(SkillListParams{})
		for _, removed := range []string{"Source", "IncludeArchived", "Provider"} {
			if slices.Contains(fields, removed) {
				t.Errorf("SkillListParams has %s, want the overlay's removal honoured", removed)
			}
		}

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Skills.List(context.Background(), SkillListParams{Limit: param.Int(10)}); err != nil {
			t.Fatalf("Skills.List() error = %v", err)
		}
		if got := transport.Only(t).Query().Get("limit"); got != "10" {
			t.Errorf("limit = %q, want 10", got)
		}
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"skill_abc123","type":"skill_deleted"}`), nil
		})
		deleted, err := client.Skills.Delete(context.Background(), "skill_abc123")
		if err != nil {
			t.Fatalf("Skills.Delete() error = %v", err)
		}
		if deleted.Type != "skill_deleted" {
			t.Errorf("tombstone type = %q, want skill_deleted", deleted.Type)
		}
	})
}

func TestSkillVersions(t *testing.T) {
	t.Parallel()

	t.Run("create uploads under files[] and escapes the skill id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"version_1"}`), nil
		})
		_, err := client.Skills.Versions.Create(context.Background(), "skill/slash", SkillVersionNewParams{
			Files: []SkillFile{{Filename: "SKILL.md", Content: []byte("# v2")}},
		})
		if err != nil {
			t.Fatalf("Versions.Create() error = %v", err)
		}
		call := transport.Only(t)
		if got, want := call.Path(), "/v1/skills/skill%2Fslash/versions"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := len(multipartParts(t, call)["files[]"]); got != 1 {
			t.Errorf("files[] parts = %d, want 1", got)
		}
	})

	t.Run("list pages", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Skills.Versions.List(context.Background(), "s1", SkillVersionListParams{
			Limit: param.Int(5), Page: param.String("p2"),
		})
		if err != nil {
			t.Fatalf("Versions.List() error = %v", err)
		}
		query := transport.Only(t).Query()
		if query.Get("limit") != "5" || query.Get("page") != "p2" {
			t.Errorf("query = %v, want limit=5 and page=p2", query)
		}
	})

	t.Run("the version string is an escaped path segment", func(t *testing.T) {
		t.Parallel()

		// A version is addressed by its string, not an opaque ID, so it needs
		// the same escaping as any other segment.
		client, transport := newRecordingClient(t, nil)
		if _, err := client.Skills.Versions.Get(context.Background(), "s1", "v1/v2"); err != nil {
			t.Fatalf("Versions.Get() error = %v", err)
		}
		if got, want := transport.Only(t).Path(), "/v1/skills/s1/versions/v1%2Fv2"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"version_abc123","type":"skill_version_deleted"}`), nil
		})
		deleted, err := client.Skills.Versions.Delete(context.Background(), "s1", "v1")
		if err != nil {
			t.Fatalf("Versions.Delete() error = %v", err)
		}
		if deleted.Type != "skill_version_deleted" {
			t.Errorf("tombstone type = %q, want skill_version_deleted", deleted.Type)
		}
	})
}

func TestTriggers(t *testing.T) {
	t.Parallel()

	t.Run("create sends the managed-deployment messaging fields unchanged", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Triggers.Create(context.Background(), TriggerNewParams{
			Name:        "orders",
			Agent:       SessionAgentParam{Type: SessionAgentRef, ID: "agt_abc123", Version: param.Int(2)},
			SessionMode: TriggerSessionPerKey,
			Source: TriggerSource{
				Type:                     TriggerSourceKafka,
				Connection:               param.String("orders"),
				Topics:                   []string{"orders.created"},
				SubscriptionName:         param.String("orca-sdk"),
				ConsumerAdditionalConfig: map[string]string{"auto.offset.reset": "earliest"},
				InputSchemaConfigs: map[string]TriggerSchemaConfig{
					"value": {Subject: "orders-value", Type: "AVRO", Version: param.Int(1)},
				},
			},
			Session:  TriggerSessionTemplate{EnvironmentID: param.String("env_abc123"), VaultIDs: []string{}},
			Replicas: param.Int(3),
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		call := transport.Only(t)
		// Core, not a cloud extension.
		if call.Method != http.MethodPost || call.Path() != "/v1/triggers" {
			t.Errorf("request = %s %s, want POST /v1/triggers", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"name":"orders","agent":{"type":"agent","id":"agt_abc123","version":2},`+
			`"session_mode":"SESSION_PER_KEY","source":{"type":"kafka","connection":"orders",`+
			`"topics":["orders.created"],"subscription_name":"orca-sdk",`+
			`"consumer_additional_config":{"auto.offset.reset":"earliest"},`+
			`"input_schema_configs":{"value":{"subject":"orders-value","type":"AVRO","version":1}}},`+
			`"session":{"environment_id":"env_abc123","vault_ids":[]},"replicas":3}`)
	})

	t.Run("triggers are never mounted under the cloud extension group", func(t *testing.T) {
		t.Parallel()

		// The overlay promoted triggers out of the cloud group. A cloud.triggers
		// would sit behind the extension gate and on a path the core engine
		// does not serve.
		if slices.Contains(structFieldNames(CloudService{}), "Triggers") {
			t.Error("CloudService has a Triggers field, want triggers to be core only")
		}
	})

	t.Run("list omits include_archived", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Triggers.List(context.Background(), TriggerListParams{
			AgentID: param.String("agt_1"), Limit: param.Int(10), Page: param.String("p2"),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		query := transport.Only(t).Query()
		for key, want := range map[string]string{"agent_id": "agt_1", "limit": "10", "page": "p2"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		if slices.Contains(structFieldNames(TriggerListParams{}), "IncludeArchived") {
			t.Error("TriggerListParams has IncludeArchived, want the overlay's removal honoured")
		}
	})

	t.Run("retrieve escapes the id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Triggers.Get(context.Background(), "trigger/with/slash"); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got, want := transport.Only(t).Path(), "/v1/triggers/trigger%2Fwith%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("update uses POST and removes null session metadata keys", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Triggers.Update(context.Background(), "t1", TriggerUpdateParams{
			SessionMode: TriggerSessionShared,
			Source: TriggerSource{
				Type:         TriggerSourcePulsar,
				Connection:   param.String("events"),
				TopicPattern: param.String("persistent://public/default/orders-.*"),
			},
			Session:  TriggerSessionTemplate{Metadata: param.New(map[string]*string{"keep": ptr("yes"), "remove": nil})},
			Replicas: param.Int(2),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"session_mode":"SHARED","source":{"type":"pulsar","connection":"events",`+
			`"topic_pattern":"persistent://public/default/orders-.*"},`+
			`"session":{"metadata":{"keep":"yes","remove":null}},"replicas":2}`)
	})

	t.Run("delete, pause and unpause", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/pause"):
				return jsonResponse(http.StatusOK, `{"status":"paused"}`), nil
			case strings.HasSuffix(req.URL.Path, "/unpause"):
				return jsonResponse(http.StatusOK, `{"status":"active"}`), nil
			default:
				return jsonResponse(http.StatusOK, `{"id":"trg_abc123","type":"trigger_deleted"}`), nil
			}
		})

		ctx := context.Background()
		deleted, err := client.Triggers.Delete(ctx, "trg_abc123")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.Type != "trigger_deleted" {
			t.Errorf("tombstone type = %q, want trigger_deleted", deleted.Type)
		}

		paused, err := client.Triggers.Pause(ctx, "trg_abc123")
		if err != nil {
			t.Fatalf("Pause() error = %v", err)
		}
		if paused.Status != "paused" {
			t.Errorf("status = %q, want paused", paused.Status)
		}

		active, err := client.Triggers.Unpause(ctx, "trg_abc123")
		if err != nil {
			t.Fatalf("Unpause() error = %v", err)
		}
		if active.Status != "active" {
			t.Errorf("status = %q, want active", active.Status)
		}

		calls := transport.Calls()
		if got, want := calls[1].Path(), "/v1/triggers/trg_abc123/pause"; got != want {
			t.Errorf("pause path = %q, want %q", got, want)
		}
		if got, want := calls[2].Path(), "/v1/triggers/trg_abc123/unpause"; got != want {
			t.Errorf("unpause path = %q, want %q", got, want)
		}
	})

	t.Run("sessions.list returns core sessions", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"data":[{"id":"session_1","type":"session"}],"has_more":false}`), nil
		})

		page, err := client.Triggers.Sessions.List(context.Background(), "trigger/slash",
			TriggerSessionListParams{Limit: param.Int(5), Page: param.String("p2"), IncludeArchived: param.Bool(true)})
		if err != nil {
			t.Fatalf("Sessions.List() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/triggers/trigger%2Fslash/sessions"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		query := call.Query()
		for key, want := range map[string]string{"limit": "5", "page": "p2", "include_archived": "true"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}

		// Core Session values, not a trigger-specific type.
		items := page.Items()
		if len(items) != 1 || items[0].Type != "session" {
			t.Errorf("items = %+v, want a core session", items)
		}
	})
}

// TestTriggerContractCoverage pins the Trigger operationIds the core spec
// declares to the SDK method implementing each.
//
// It reads the vendored spec rather than a hand-written list, so a spec
// operation with no SDK method - or an SDK method with no spec operation -
// fails in both directions the day the spec changes.
func TestTriggerContractCoverage(t *testing.T) {
	t.Parallel()

	spec, err := os.ReadFile("openapi/managed-agents.yaml")
	if err != nil {
		t.Fatalf("reading the vendored core spec: %v", err)
	}

	pattern := regexp.MustCompile(`(?m)^\s+operationId:\s+(triggers\.\S+)\s*$`)
	var operations []string
	for _, match := range pattern.FindAllStringSubmatch(string(spec), -1) {
		operations = append(operations, match[1])
	}
	slices.Sort(operations)
	operations = slices.Compact(operations)

	if len(operations) == 0 {
		t.Fatal("no trigger operationIds found in the spec; the pattern or the spec changed")
	}

	// The operationId is `triggers.get`; the Go method is Get.
	implemented := map[string]func(*Client) any{
		"triggers.create":   func(c *Client) any { return c.Triggers.Create },
		"triggers.list":     func(c *Client) any { return c.Triggers.List },
		"triggers.get":      func(c *Client) any { return c.Triggers.Get },
		"triggers.update":   func(c *Client) any { return c.Triggers.Update },
		"triggers.delete":   func(c *Client) any { return c.Triggers.Delete },
		"triggers.pause":    func(c *Client) any { return c.Triggers.Pause },
		"triggers.unpause":  func(c *Client) any { return c.Triggers.Unpause },
		"triggers.sessions": func(c *Client) any { return c.Triggers.Sessions.List },
	}

	client, _ := newRecordingClient(t, nil)

	for _, operation := range operations {
		accessor, ok := implemented[operation]
		if !ok {
			t.Errorf("the spec declares %s but no SDK method implements it", operation)
			continue
		}
		if accessor(client) == nil {
			t.Errorf("%s maps to a nil accessor", operation)
		}
	}
	for operation := range implemented {
		if !slices.Contains(operations, operation) {
			t.Errorf("the SDK implements %s but the spec does not declare it", operation)
		}
	}
}

func TestSkillVersionDownload(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "PK\x03\x04zip-bytes"), nil
	})

	var buf bytes.Buffer
	err := client.Skills.Versions.Download(context.Background(), "skill/slash", "v1/v2", &buf)
	if err != nil {
		t.Fatalf("Versions.Download() error = %v", err)
	}

	call := transport.Only(t)
	want := "/v1/skills/skill%2Fslash/versions/v1%2Fv2/content"
	if got := call.Path(); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got := call.Header.Get("Accept"); got != "application/zip" {
		t.Errorf("Accept = %q, want application/zip", got)
	}
	if got := buf.String(); got != "PK\x03\x04zip-bytes" {
		t.Errorf("body = %q, want the archive bytes unchanged", got)
	}
}
