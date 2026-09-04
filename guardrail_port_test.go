// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

const guardrailFixtureJSON = `{"id":"grd_example","type":"guardrail","name":"Protect production",` +
	`"description":"Blocks risky shell commands","enabled":true,"phases":["tool_call"],` +
	`"scope":"workspace","rule":{"kind":"builtin","builtin":"block_tools",` +
	`"params":{"tools":["shell"]}},"metadata":{"owner":"platform"},"archived_at":null,` +
	`"created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-01T00:00:00Z"}`

func extensionGroupsJSON(groups ...string) string {
	entries := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		entries = append(entries, map[string]any{"name": group, "versions": []any{}})
	}
	body, _ := json.Marshal(map[string]any{"kind": "APIGroupList", "groups": entries})
	return string(body)
}

func TestGuardrailCreateRequestShape(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/apis" {
			return jsonResponse(http.StatusOK, extensionGroupsJSON(PolicyExtensionGroup)), nil
		}
		return jsonResponse(http.StatusCreated, guardrailFixtureJSON), nil
	})

	guardrail, err := client.Guardrails.Create(context.Background(), GuardrailNewParams{
		Name:        "Protect production",
		Description: param.Null[string](),
		Enabled:     param.Bool(true),
		Phases:      []GuardrailPhase{GuardrailPhaseToolCall},
		Scope:       param.New(GuardrailScopeExplicit),
		Rule: GuardrailRule{
			Kind:       GuardrailRuleExpression,
			Expression: `event.tool.name != "shell"`,
			OnFalse:    GuardrailVerdictAsk,
			Reason:     "Approval required",
		},
		Metadata: map[string]string{"owner": "platform"},
	}, option.WithHeader("X-Test", "create"))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if guardrail.ID != "grd_example" || guardrail.Rule.Builtin != "block_tools" {
		t.Fatalf("guardrail = %#v", guardrail)
	}

	calls := transport.Calls()
	if len(calls) != 2 {
		t.Fatalf("requests = %d, want discovery plus create", len(calls))
	}
	call := calls[1]
	if call.Method != http.MethodPost || call.Path() != "/apis/policy.runorca.ai/v1/guardrails" {
		t.Errorf("request = %s %s, want POST /apis/policy.runorca.ai/v1/guardrails", call.Method, call.Path())
	}
	if got := call.Header.Get("X-Test"); got != "create" {
		t.Errorf("X-Test = %q, want create", got)
	}
	assertJSONBody(t, call, `{"name":"Protect production","description":null,"enabled":true,`+
		`"phases":["tool_call"],"scope":"explicit","rule":{"kind":"expression",`+
		`"expression":"event.tool.name != \"shell\"","on_false":"ask","reason":"Approval required"},`+
		`"metadata":{"owner":"platform"}}`)
}

func TestGuardrailOperations(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/apis":
			return jsonResponse(http.StatusOK, extensionGroupsJSON(PolicyExtensionGroup)), nil
		case req.URL.Path == "/apis/policy.runorca.ai/v1/guardrails" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[`+guardrailFixtureJSON+`],"next_page":null}`), nil
		case req.URL.Path == "/apis/policy.runorca.ai/v1/guardrailtypes":
			return jsonResponse(http.StatusOK, `{"data":[{"name":"block_tools","title":"Block tools",`+
				`"description":"Blocks tools","phases":["tool_call"],"stateful":false,`+
				`"verdicts":["deny"],"paramsSchema":{"type":"object"}}]}`), nil
		case req.Method == http.MethodDelete:
			return jsonResponse(http.StatusOK, `{"id":"grd_example","type":"guardrail_deleted"}`), nil
		default:
			return jsonResponse(http.StatusOK, guardrailFixtureJSON), nil
		}
	})
	ctx := context.Background()

	page, err := client.Guardrails.List(ctx, GuardrailListParams{
		Limit: param.Int(25), Page: param.String("next"), IncludeArchived: param.Bool(true),
	})
	if err != nil || len(page.Data) != 1 {
		t.Fatalf("List() = %#v, %v", page, err)
	}
	listCall := transport.Last(t)
	if got := listCall.Query().Get("limit"); got != "25" {
		t.Errorf("limit = %q, want 25", got)
	}
	if got := listCall.Query().Get("page"); got != "next" {
		t.Errorf("page = %q, want next", got)
	}
	if got := listCall.Query().Get("include_archived"); got != "true" {
		t.Errorf("include_archived = %q, want true", got)
	}

	if _, err := client.Guardrails.Get(ctx, "grd/with slash"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got := transport.Last(t).Path(); got != "/apis/policy.runorca.ai/v1/guardrails/grd%2Fwith%20slash" {
		t.Errorf("Get() path = %q", got)
	}

	if _, err := client.Guardrails.Update(ctx, "grd_example", GuardrailUpdateParams{
		Enabled:  param.Bool(false),
		Metadata: param.New(map[string]*string{"owner": nil}),
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updateCall := transport.Last(t)
	if updateCall.Method != http.MethodPost {
		t.Errorf("Update() method = %s, want POST", updateCall.Method)
	}
	assertJSONBody(t, updateCall, `{"enabled":false,"metadata":{"owner":null}}`)

	if _, err := client.Guardrails.Archive(ctx, "grd_example"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if got := transport.Last(t).Path(); got != "/apis/policy.runorca.ai/v1/guardrails/grd_example/archive" {
		t.Errorf("Archive() path = %q", got)
	}

	deleted, err := client.Guardrails.Delete(ctx, "grd_example")
	if err != nil || deleted.Type != "guardrail_deleted" {
		t.Fatalf("Delete() = %#v, %v", deleted, err)
	}

	types, err := client.Guardrails.ListTypes(ctx)
	if err != nil || len(types.Data) != 1 || types.Data[0].ParamsSchema["type"] != "object" {
		t.Fatalf("ListTypes() = %#v, %v", types, err)
	}
}

func TestGuardrailGateStopsBusinessRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invoke func(context.Context, *Client) error
	}{
		{name: "create", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.Create(ctx, GuardrailNewParams{
				Name: "Rule", Rule: GuardrailRule{Kind: GuardrailRuleBuiltin, Builtin: "block_tools"},
			})
			return err
		}},
		{name: "list", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.List(ctx, GuardrailListParams{})
			return err
		}},
		{name: "get", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.Get(ctx, "grd_example")
			return err
		}},
		{name: "update", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.Update(ctx, "grd_example", GuardrailUpdateParams{})
			return err
		}},
		{name: "archive", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.Archive(ctx, "grd_example")
			return err
		}},
		{name: "delete", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.Delete(ctx, "grd_example")
			return err
		}},
		{name: "list types", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.Guardrails.ListTypes(ctx)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/apis" {
					return jsonResponse(http.StatusOK, extensionGroupsJSON()), nil
				}
				return jsonResponse(http.StatusInternalServerError, `{}`), nil
			})
			err := tc.invoke(context.Background(), client)
			var unavailable *ExtensionNotAvailableError
			if !errors.As(err, &unavailable) || unavailable.Group != PolicyExtensionGroup {
				t.Fatalf("error = %v, want policy ExtensionNotAvailableError", err)
			}
			if calls := transport.Calls(); len(calls) != 1 || calls[0].Path() != "/apis" {
				t.Fatalf("requests = %#v, want only GET /apis", calls)
			}
		})
	}
}

func TestGuardrailRuleUnmarshalSelectsTheDeclaredVariant(t *testing.T) {
	t.Parallel()

	var rule GuardrailRule
	if err := json.Unmarshal([]byte(`{"kind":"expression","expression":"true","on_false":"deny",`+
		`"builtin":"must-not-leak"}`), &rule); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if rule.Kind != GuardrailRuleExpression || rule.Expression != "true" || rule.OnFalse != GuardrailVerdictDeny {
		t.Fatalf("rule = %#v, want expression variant", rule)
	}
	if rule.Builtin != "" {
		t.Fatalf("Builtin = %q, want mismatched variant fields ignored", rule.Builtin)
	}
}
