// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/agents/{agents,versions}.test.ts.
//
// The deployment overlay removes agents.delete: an agent is retired by
// archiving it, and a typed resource must not grow a delete method.

// assertJSONBody decodes a captured request body and compares it to want,
// which is compared as JSON so key order does not matter.
func assertJSONBody(t *testing.T, call capturedCall, want string) {
	t.Helper()

	var got, expected any
	if err := json.Unmarshal(call.Body, &got); err != nil {
		t.Fatalf("decoding request body %q: %v", call.Body, err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatalf("decoding expected body %q: %v", want, err)
	}
	gotNormal, _ := json.Marshal(got)
	wantNormal, _ := json.Marshal(expected)
	if string(gotNormal) != string(wantNormal) {
		t.Errorf("request body =\n  %s\nwant\n  %s", gotNormal, wantNormal)
	}
}

func TestAgentCreate(t *testing.T) {
	t.Parallel()

	t.Run("sends the minimal body with the model shorthand", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Agents.Create(context.Background(), AgentNewParams{
			Model: Model("claude-sonnet-4-6"),
			Name:  "demo",
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", call.Method)
		}
		if call.Path() != "/v1/agents" {
			t.Errorf("path = %q, want /v1/agents", call.Path())
		}
		assertJSONBody(t, call, `{"model":"claude-sonnet-4-6","name":"demo"}`)
	})

	t.Run("sends every optional field in its declared shape", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Create(context.Background(), AgentNewParams{
			Model: AgentModelParam{
				ID:     "claude-opus-4",
				Speed:  param.New(ModelSpeedFast),
				Effort: param.New(ModelEffort{Type: ModelEffortHigh}),
			},
			Name:        "full-agent",
			Description: param.String("A fully-specified agent"),
			System:      param.String("You are a helpful assistant."),
			Metadata:    map[string]string{"team": "a", "project": "b"},
			McpServers: []AgentMcpServerParam{
				{Name: "srv", URL: "https://mcp.example.com"},
			},
			Tools: []AgentTool{
				{
					Type: AgentToolBuiltinToolset,
					Configs: map[string]AgentToolConfig{
						"lookup": {
							Enabled:          param.Bool(true),
							PermissionPolicy: param.New(AgentPermissionPolicy{Type: AgentPermissionAlwaysAsk}),
						},
					},
					DefaultConfig: param.New(AgentToolConfig{
						Enabled: param.Bool(true),
						PermissionPolicy: param.New(AgentPermissionPolicy{
							Type:  AgentPermissionAlwaysAllow,
							Extra: map[string]any{"audit_level": "strict"},
						}),
						Extra: map[string]any{"provider_default": "value"},
					}),
					Extra: map[string]any{"provider_specific": map[string]any{"enabled": true}},
				},
				{
					Type:        AgentToolCustom,
					Name:        "lookup_order",
					Description: "Look up an order",
					InputSchema: &AgentCustomToolInputSchema{Type: "object"},
				},
			},
			Skills: []AgentSkillParam{
				{
					Type:    AgentSkillCustom,
					SkillID: "sk_abc",
					Version: param.String("1"),
					Extra:   map[string]any{"provider_config": map[string]any{"mode": "x"}},
				},
			},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		assertJSONBody(t, transport.Only(t), `{"model":{"id":"claude-opus-4","speed":"fast","effort":{"type":"high"}},`+
			`"name":"full-agent","description":"A fully-specified agent",`+
			`"system":"You are a helpful assistant.","metadata":{"team":"a","project":"b"},`+
			`"mcp_servers":[{"name":"srv","url":"https://mcp.example.com"}],`+
			`"tools":[{"type":"agent_toolset","configs":{"lookup":{"enabled":true,`+
			`"permission_policy":{"type":"always_ask"}}},"default_config":{"enabled":true,`+
			`"permission_policy":{"type":"always_allow","audit_level":"strict"},`+
			`"provider_default":"value"},"provider_specific":{"enabled":true}},`+
			`{"type":"custom","name":"lookup_order","description":"Look up an order",`+
			`"input_schema":{"type":"object"}}],`+
			`"skills":[{"type":"custom","skill_id":"sk_abc","version":"1",`+
			`"provider_config":{"mode":"x"}}]}`)
	})

	t.Run("an mcp_servers entry does not gain a synthesized type", func(t *testing.T) {
		t.Parallel()

		// The contract makes the discriminator optional here. Sending one the
		// caller did not ask for changes the request.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Create(context.Background(), AgentNewParams{
			Model:      Model("m"),
			Name:       "n",
			McpServers: []AgentMcpServerParam{{Name: "srv", URL: "https://mcp.example.com"}},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		var body struct {
			McpServers []map[string]any `json:"mcp_servers"`
		}
		transport.Only(t).JSONBody(t, &body)
		if _, ok := body.McpServers[0]["type"]; ok {
			t.Errorf("mcp_servers[0] = %v, want no synthesized type field", body.McpServers[0])
		}
	})
}

func TestAgentGet(t *testing.T) {
	t.Parallel()

	t.Run("omits version entirely when it is not supplied", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Agents.Get(context.Background(), "agent_1", AgentGetParams{}); err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		call := transport.Only(t)
		if call.Path() != "/v1/agents/agent_1" {
			t.Errorf("path = %q, want /v1/agents/agent_1", call.Path())
		}
		if call.URL.RawQuery != "" {
			t.Errorf("query = %q, want it empty - an omitted version must not appear", call.URL.RawQuery)
		}
	})

	t.Run("sends version when supplied", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Get(context.Background(), "agent_1", AgentGetParams{Version: param.Int(2)})
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got := transport.Only(t).Query().Get("version"); got != "2" {
			t.Errorf("version = %q, want %q", got, "2")
		}
	})

	t.Run("escapes an id that contains a slash", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Agents.Get(context.Background(), "agent/with/slash", AgentGetParams{}); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got, want := transport.Only(t).Path(), "/v1/agents/agent%2Fwith%2Fslash"; got != want {
			t.Errorf("path = %q, want %q - an id must never add path segments", got, want)
		}
	})

	t.Run("decodes the nullable response fields", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"agent_1","type":"agent","name":"n",`+
				`"description":null,"system":null,"model":{"id":"m"},"mcp_servers":[],`+
				`"tools":[],"skills":[],"multiagent":null,"metadata":{},"version":1,`+
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",`+
				`"archived_at":null}`), nil
		})

		agent, err := client.Agents.Get(context.Background(), "agent_1", AgentGetParams{})
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if agent.Tools == nil {
			t.Error("Tools = nil, want an empty slice - it is a required array")
		}
		if agent.Multiagent != nil {
			t.Errorf("Multiagent = %v, want nil", agent.Multiagent)
		}
		if agent.ArchivedAt != nil {
			t.Errorf("ArchivedAt = %v, want nil", agent.ArchivedAt)
		}
		if agent.Description != nil {
			t.Errorf("Description = %v, want nil", agent.Description)
		}
	})
}

func TestAgentUpdate(t *testing.T) {
	t.Parallel()

	t.Run("uses POST and does not synthesize a version", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Update(context.Background(), "agent_1", AgentUpdateParams{
			Name: param.String("new name"),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		if call.Path() != "/v1/agents/agent_1" {
			t.Errorf("path = %q, want /v1/agents/agent_1", call.Path())
		}
		assertJSONBody(t, call, `{"name":"new name"}`)
	})

	t.Run("sends explicit nulls to clear fields", func(t *testing.T) {
		t.Parallel()

		// Omitting a field leaves it alone; sending null clears it. A metadata
		// value of null removes that one key rather than replacing the map.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Update(context.Background(), "agent_1", AgentUpdateParams{
			Version:    param.Int(1),
			McpServers: param.Null[[]AgentMcpServerParam](),
			Tools:      param.Null[[]AgentTool](),
			Skills:     param.Null[[]AgentSkillParam](),
			Multiagent: param.Null[AgentMultiagent](),
			Metadata: param.New(map[string]*string{
				"keep":   ptr("value"),
				"remove": nil,
			}),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		assertJSONBody(t, transport.Only(t), `{"version":1,"mcp_servers":null,"tools":null,`+
			`"skills":null,"multiagent":null,"metadata":{"keep":"value","remove":null}}`)
	})

	t.Run("a null description is sent, not dropped", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.Update(context.Background(), "agent_1", AgentUpdateParams{
			Description: param.Null[string](),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"description":null}`)
	})
}

func TestAgentList(t *testing.T) {
	t.Parallel()

	t.Run("sends the list filters it was given", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Agents.List(context.Background(), AgentListParams{
			Limit:           param.Int(10),
			Page:            param.String("cursor-p2"),
			IncludeArchived: param.Bool(true),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		query := transport.Only(t).Query()
		for key, want := range map[string]string{"limit": "10", "page": "cursor-p2", "include_archived": "true"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("omits filters that were not set", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Agents.List(context.Background(), AgentListParams{}); err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got := transport.Only(t).URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}
	})

	t.Run("walks pages until has_more is false", func(t *testing.T) {
		t.Parallel()

		var calls int
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(http.StatusOK, `{"data":[{"id":"agent_1"}],"has_more":true,`+
					`"first_id":"agent_1","last_id":"agent_1","next_page":"cursor-p2"}`), nil
			}
			return jsonResponse(http.StatusOK, `{"data":[{"id":"agent_2"}],"has_more":false}`), nil
		})

		page, err := client.Agents.List(context.Background(), AgentListParams{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		var ids []string
		for agent, err := range page.All(context.Background()) {
			if err != nil {
				t.Fatalf("iteration error = %v", err)
			}
			ids = append(ids, agent.ID)
		}
		if len(ids) != 2 || ids[0] != "agent_1" || ids[1] != "agent_2" {
			t.Errorf("ids = %v, want [agent_1 agent_2]", ids)
		}
		if calls != 2 {
			t.Errorf("requests = %d, want exactly 2", calls)
		}
	})
}

func TestAgentArchive(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"agent_1","archived_at":"2026-01-02T00:00:00Z"}`), nil
	})

	agent, err := client.Agents.Archive(context.Background(), "agent_1")
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	call := transport.Only(t)
	if call.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", call.Method)
	}
	// A sub-path, never a ":archive" verb suffix.
	if got, want := call.Path(), "/v1/agents/agent_1/archive"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if agent.ArchivedAt == nil || *agent.ArchivedAt != "2026-01-02T00:00:00Z" {
		t.Errorf("ArchivedAt = %v, want the archived agent to come back", agent.ArchivedAt)
	}
}

func TestAgentHasNoDelete(t *testing.T) {
	t.Parallel()

	// The deployment overlay removes agent deletion. A Delete method would
	// compile against a server that rejects it at runtime, so its absence is
	// part of the contract.
	if _, ok := any(AgentService{}).(interface {
		Delete(context.Context, string, ...option.RequestOption) error
	}); ok {
		t.Error("AgentService has a Delete method, want archiving to be the only retirement path")
	}
}

func TestAgentVersionsList(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	_, err := client.Agents.Versions.List(context.Background(), "agent_1", AgentVersionListParams{
		Limit: param.Int(5),
		Page:  param.String("p2"),
	})
	if err != nil {
		t.Fatalf("Versions.List() error = %v", err)
	}

	call := transport.Only(t)
	if got, want := call.Path(), "/v1/agents/agent_1/versions"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got := call.Query().Get("limit"); got != "5" {
		t.Errorf("limit = %q, want 5", got)
	}
	if got := call.Query().Get("page"); got != "p2" {
		t.Errorf("page = %q, want p2", got)
	}
}

func ptr[T any](v T) *T { return &v }
