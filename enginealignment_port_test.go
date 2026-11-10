// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

const engineGroups = `{"groups":[{"name":"policy.runorca.ai"},{"name":"pricing.runorca.ai","versions":[{"version":"v1","group_version":"pricing.runorca.ai/v1"},{"version":"v2","group_version":"pricing.runorca.ai/v2"}]},{"name":"runtime.runorca.ai","versions":[{"version":"v1","group_version":"runtime.runorca.ai/v1"}]}]}`

func TestGuardrailRuleRoundTripAndMalformedKnown(t *testing.T) {
	for _, body := range []string{
		`{"kind":"future","policy":{"nested":[1,null,true]},"large":9007199254740993}`,
		`{"kind":"dogwood","policy":"permit;","applies_to":"mcp_tools","history":"principal","mcp_manifests":[{"server":"tools","url":"https://tools.example","tools":[{"name":"read","inputSchema":{"type":"object"}}]}]}`,
	} {
		var rule GuardrailRule
		if err := json.Unmarshal([]byte(body), &rule); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(rule)
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err := json.Unmarshal([]byte(body), &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("lost rule fields: %s", data)
		}
		if rule.Kind == "future" && rule.Validate() == nil {
			t.Fatal("unknown kind accepted for authoring")
		}
		if rule.Kind == "future" && !strings.Contains(string(data), "9007199254740993") {
			t.Fatalf("unknown numeric field lost precision: %s", data)
		}
	}
	for _, body := range []string{`{"kind":"dogwood"}`, `{"kind":"dogwood","policy":4}`, `{"kind":"builtin"}`, `{"kind":"expression","expression":"true"}`, `{}`, `null`} {
		var rule GuardrailRule
		if err := json.Unmarshal([]byte(body), &rule); err == nil {
			t.Errorf("malformed known rule accepted: %s", body)
		}
	}
}

func TestGuardrailDogwoodAuthoringAndWarnings(t *testing.T) {
	client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/apis" {
			return jsonResponse(200, engineGroups), nil
		}
		return jsonResponse(200, `{"mode":"observe","warnings":["future warning"],"rule":{"kind":"dogwood","policy":"permit;"}}`), nil
	})
	result, err := client.Guardrails.Create(context.Background(), GuardrailNewParams{Name: "policy", Mode: param.New(GuardrailModeObserve), Rule: GuardrailRule{Kind: GuardrailRuleDogwood, Policy: "permit;"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != GuardrailModeObserve || len(result.Warnings) != 1 {
		t.Fatalf("response = %+v", result)
	}
	assertJSONBody(t, transport.Last(t), `{"name":"policy","mode":"observe","rule":{"kind":"dogwood","policy":"permit;"}}`)
}

func TestGuardrailAuthoringRejectsUnknownKinds(t *testing.T) {
	client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) { return jsonResponse(200, engineGroups), nil })
	rule := GuardrailRule{Kind: "future", Raw: json.RawMessage(`{"kind":"future","policy":true}`)}
	if _, err := client.Guardrails.Create(context.Background(), GuardrailNewParams{Name: "future", Rule: rule}); err == nil {
		t.Fatal("Create accepted unknown rule")
	}
	if _, err := client.Guardrails.Update(context.Background(), "grd_a", GuardrailUpdateParams{Rule: param.New(rule)}); err == nil {
		t.Fatal("Update accepted unknown rule")
	}
	if len(transport.Calls()) != 1 || transport.Last(t).Path() != "/apis" {
		t.Fatal("invalid rule reached business endpoint")
	}
}

func TestEngineHistoricalRequests(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name, method, path, body string
		invoke                   func(*Client) error
	}{
		{"rotate", "POST", "/v1/environments/env%2Fa/rotate-key", `{"env_key":"secret","env_key_expires_at":"expiry"}`, func(c *Client) error {
			v, e := c.Environments.RotateKey(ctx, "env/a")
			if e == nil && v.EnvKey != "secret" {
				t.Error(v)
			}
			return e
		}},
		{"revoke", "POST", "/v1/environments/env%2Fa/revoke-key", `{"id":"env_a"}`, func(c *Client) error { _, e := c.Environments.RevokeKey(ctx, "env/a"); return e }},
		{"stats", "GET", "/v1/environments/env%2Fa/work_stats", `{"depth":3,"in_flight":2,"worker_connected":true}`, func(c *Client) error {
			v, e := c.Environments.WorkStats(ctx, "env/a")
			if e == nil && (v.Depth != 3 || !v.WorkerConnected) {
				t.Error(v)
			}
			return e
		}},
		{"interrupt", "POST", "/v1/sessions/ses%2Fa/threads/sth%2Fb/interrupt", `{"id":"sth_a"}`, func(c *Client) error { _, e := c.Sessions.Threads.Interrupt(ctx, "ses/a", "sth/b"); return e }},
		{"runtime", "GET", "/apis/runtime.runorca.ai/v1", `{"required_features":["future"]}`, func(c *Client) error {
			v, e := c.Discovery.RuntimeGroupResources(ctx)
			if e == nil && len(v.RequiredFeatures) != 1 {
				t.Error(v)
			}
			return e
		}},
		{"harnesses", "GET", "/apis/runtime.runorca.ai/v1/harnesses", `{"data":[{"id":"native","capabilities":{"native_resume":true},"models":[{"id":"m","efforts":["high"]}]}]}`, func(c *Client) error {
			v, e := c.Discovery.Harnesses(ctx)
			if e == nil && (len(v.Data) != 1 || !v.Data[0].Capabilities.NativeResume) {
				t.Error(v)
			}
			return e
		}},
		{"pricingv2", "GET", "/apis/pricing.runorca.ai/v2", `{"required_features":["unknown-feature"]}`, func(c *Client) error {
			v, e := c.Discovery.PricingV2GroupResources(ctx)
			if e == nil && v.RequiredFeatures[0] != "unknown-feature" {
				t.Error(v)
			}
			return e
		}},
		{"dogwoodschema", "GET", "/apis/policy.runorca.ai/v1/policyschemas/dogwood", `{"language_version":"1","base_schema":"schema","limits":{"future":4}}`, func(c *Client) error {
			v, e := c.Guardrails.DogwoodSchema(ctx)
			if e == nil && v.LanguageVersion != "1" {
				t.Error(v)
			}
			return e
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/apis" {
					return jsonResponse(200, engineGroups), nil
				}
				return jsonResponse(200, tt.body), nil
			})
			if err := tt.invoke(client); err != nil {
				t.Fatal(err)
			}
			call := transport.Last(t)
			if call.Method != tt.method || call.Path() != tt.path {
				t.Fatalf("request = %+v", call)
			}
			if tt.method == http.MethodPost {
				assertJSONBody(t, call, `{}`)
				if call.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("content type = %q", call.Header.Get("Content-Type"))
				}
			} else if len(call.Body) != 0 {
				t.Fatalf("unexpected request body: %s", call.Body)
			}
			if tt.path[:4] == "/v1/" && len(transport.Calls()) != 1 {
				t.Fatal("core action performed discovery")
			}
		})
	}
}

func catalogPage(next string, index int) string {
	return fmt.Sprintf(`{"catalog_id":"cat","generation":7,"digest":"digest","published_at":"published-%d","expires_at":"expires-%d","required_features":["future-%d"],"conditional_models":[{"provider":"p","model_id":"m%d"}],"data":[{"rule_id":"r%d","rates":{"cache_read_per_million_tokens":null},"conditions":{"request_input_tokens":{"min":0,"max":null}}}],"next_page":%s}`, index, index, index, index, index, next)
}

func TestPricingCatalogPagesAndAutoPager(t *testing.T) {
	client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/apis" {
			return jsonResponse(200, engineGroups), nil
		}
		if r.URL.EscapedPath() != "/apis/pricing.runorca.ai/v2/modelpricerules" {
			t.Errorf("path %s", r.URL.EscapedPath())
		}
		if r.URL.Query().Get("page") == "second" {
			return jsonResponse(200, catalogPage(`null`, 2)), nil
		}
		return jsonResponse(200, catalogPage(`"second"`, 1)), nil
	})
	pager, err := client.ModelPriceRules.ListAutoPaging(context.Background(), ModelPriceRuleListParams{Limit: param.Int(3), Page: param.String("initial")}, option.WithHeader("X-Page", "preserved"), option.WithQuery("extra", "yes"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if !pager.Next() {
			t.Fatalf("page %d: %v", i, pager.Err())
		}
		p := pager.Page()
		if p.CatalogID != "cat" || p.Generation != 7 || p.Digest != "digest" || p.PublishedAt != fmt.Sprintf("published-%d", i) || p.ExpiresAt != fmt.Sprintf("expires-%d", i) || p.RequiredFeatures[0] != fmt.Sprintf("future-%d", i) || p.ConditionalModels[0].ModelID != fmt.Sprintf("m%d", i) {
			t.Fatalf("lost metadata: %+v", p)
		}
		if pager.Current().RuleID != fmt.Sprintf("r%d", i) || pager.Current().Rates.CacheReadPerMillionTokens != nil {
			t.Fatal(pager.Current())
		}
	}
	if pager.Next() || pager.Err() != nil {
		t.Fatal(pager.Err())
	}
	calls := transport.Calls()
	if len(calls) != 3 {
		t.Fatal(calls)
	}
	for _, c := range calls[1:] {
		if c.Query().Get("limit") != "3" || c.Query().Get("extra") != "yes" || c.Header.Get("X-Page") != "preserved" {
			t.Fatalf("lost options: %+v", c)
		}
	}
	if calls[2].Query().Get("page") != "second" {
		t.Fatal(calls[2])
	}
}

func TestPricingCatalogExpiryAndIdentity(t *testing.T) {
	for _, status := range []int{410, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := 0
			client, _ := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/apis" {
					return jsonResponse(200, engineGroups), nil
				}
				requests++
				if requests == 1 {
					return jsonResponse(200, catalogPage(`"next"`, 1)), nil
				}
				if status == 410 {
					return jsonResponse(410, `{"error":{"message":"expired"}}`), nil
				}
				return jsonResponse(200, `{"catalog_id":"other","data":[]}`), nil
			})
			page, err := client.ModelPriceRules.List(context.Background(), ModelPriceRuleListParams{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := page.GetNextPage(context.Background()); err == nil {
				t.Fatal("expected terminal error")
			}
			if page.HasNextPage() {
				t.Fatal("terminal page still fetchable")
			}
			_, _ = page.GetNextPage(context.Background())
			if requests != 2 {
				t.Fatalf("restarted catalog: %d requests", requests)
			}
		})
	}
}

func TestPricingV2RequiresAdvertisedVersion(t *testing.T) {
	for _, groups := range []string{extensionGroupsJSON(PricingExtensionGroup), `{"groups":[{"name":"pricing.runorca.ai","versions":[{"version":"v1","group_version":"pricing.runorca.ai/v1"}],"preferred_version":{"version":"v2","group_version":"pricing.runorca.ai/v2"}}]}`} {
		client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) { return jsonResponse(200, groups), nil })
		_, err := client.ModelPriceRules.List(context.Background(), ModelPriceRuleListParams{})
		var unavailable *ExtensionNotAvailableError
		if !errors.As(err, &unavailable) || len(transport.Calls()) != 1 {
			t.Fatalf("gate: %v, calls=%v", err, transport.Calls())
		}
	}
}

type trackedEngineBody struct {
	io.Reader
	closed *int
}

func (b trackedEngineBody) Close() error { *b.closed++; return nil }

func TestEffectivePortRetryTimeoutAndBodyClose(t *testing.T) {
	calls, closed := 0, 0
	var attemptContexts []context.Context
	client, transport := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/apis" {
			return jsonResponse(200, engineGroups), nil
		}
		calls++
		attemptContexts = append(attemptContexts, r.Context())
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("request timeout was lost")
		}
		status := 304
		if calls == 1 {
			status = 503
		}
		response := jsonResponse(status, "")
		response.Body = trackedEngineBody{Reader: strings.NewReader(""), closed: &closed}
		response.Header.Set("retry-after-ms", "0")
		response.Header.Set("ETag", `"lease"`)
		return response, nil
	})
	result, err := client.Guardrails.Effective(context.Background(), GuardrailEffectiveParams{IfNoneMatch: param.String(`"lease"`), RuleKinds: param.String("dogwood"), GuardrailCapabilities: param.String("future")}, option.WithRequestTimeout(time.Second), option.WithMaxRetries(1))
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != 304 || result.Bundle != nil || result.Header.Get("ETag") != `"lease"` || calls != 2 || closed != 2 {
		t.Fatalf("result=%+v calls=%d closed=%d", result, calls, closed)
	}
	for _, ctx := range attemptContexts {
		if ctx.Err() != context.Canceled {
			t.Errorf("attempt not cancelled on close: %v", ctx.Err())
		}
	}
	last := transport.Last(t)
	if last.Method != "GET" || last.Path() != "/apis/policy.runorca.ai/v1/effectiveguardrails" || last.Header.Get("If-None-Match") != `"lease"` || last.Query().Get("rule_kinds") != "dogwood" || last.Query().Get("guardrail_capabilities") != "future" {
		t.Fatal(last)
	}
	if err := client.GetJSON(context.Background(), "v1/agents", nil); err == nil {
		t.Fatal("304 was accepted by the global JSON pipeline")
	}
}

func TestEffectiveRejectsOtherStatusesAndMalformedBundle(t *testing.T) {
	for _, status := range []int{200, 302, 400} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client, _ := newRecordingClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/apis" {
					return jsonResponse(200, engineGroups), nil
				}
				return jsonResponse(status, `{"generation":"invalid"}`), nil
			})
			if _, err := client.Guardrails.Effective(context.Background(), GuardrailEffectiveParams{}); err == nil {
				t.Fatalf("accepted status %d or invalid bundle", status)
			}
		})
	}
}
