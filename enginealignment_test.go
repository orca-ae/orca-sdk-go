// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

func TestEngineAlignmentOverHTTP(t *testing.T) {
	expected := map[string]string{
		"/v1/environments/env%2Fa/rotate-key":              "POST",
		"/v1/environments/env%2Fa/revoke-key":              "POST",
		"/v1/environments/env%2Fa/work_stats":              "GET",
		"/v1/sessions/ses%2Fa/threads/sth%2Fb/interrupt":   "POST",
		"/apis/runtime.runorca.ai/v1":                      "GET",
		"/apis/runtime.runorca.ai/v1/harnesses":            "GET",
		"/apis/policy.runorca.ai/v1/policyschemas/dogwood": "GET",
		"/apis/pricing.runorca.ai/v2":                      "GET",
	}
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/apis" {
			_, _ = w.Write([]byte(engineGroups))
			return
		}
		path := r.URL.EscapedPath()
		if method, ok := expected[path]; !ok || r.Method != method {
			t.Errorf("unexpected %s %s", r.Method, path)
		}
		seen[path] = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, a := client.Environments.RotateKey(ctx, "env/a")
	_, b := client.Environments.RevokeKey(ctx, "env/a")
	_, c := client.Environments.WorkStats(ctx, "env/a")
	_, d := client.Sessions.Threads.Interrupt(ctx, "ses/a", "sth/b")
	_, e := client.Discovery.RuntimeGroupResources(ctx)
	_, f := client.Discovery.Harnesses(ctx)
	_, g := client.Guardrails.DogwoodSchema(ctx)
	_, h := client.Discovery.PricingV2GroupResources(ctx)
	for _, err := range []error{a, b, c, d, e, f, g, h} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("seen %v", seen)
	}
}

func TestEffectiveConditionalOverHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apis" {
			_, _ = w.Write([]byte(engineGroups))
			return
		}
		if r.URL.EscapedPath() != "/apis/policy.runorca.ai/v1/effectiveguardrails" || r.Method != "GET" {
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("rule_kinds") != "builtin,dogwood" || r.URL.Query().Get("guardrail_capabilities") != "lease-v1" {
			t.Error(r.URL)
		}
		for key, value := range map[string]string{"ETag": `"registry-abc"`, "Cache-Control": "no-store", "Orca-Guardrail-Issued-At": "issued", "Orca-Guardrail-Expires-At": "expires", "Orca-Guardrail-Generation": "42"} {
			w.Header().Set(key, value)
		}
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(304)
			return
		}
		_, _ = w.Write([]byte(`{"bundle_id":"b","generation":42,"guardrails":[{"id":"grd_a","mode":"observe","rule":["future",{"preserved":true}]}]}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	params := GuardrailEffectiveParams{RuleKinds: param.String("builtin,dogwood"), GuardrailCapabilities: param.String("lease-v1")}
	result, err := client.Guardrails.Effective(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != 200 || result.Bundle == nil || result.Bundle.BundleID != "b" || string(result.Bundle.Guardrails[0].Rule) != `["future",{"preserved":true}]` {
		t.Fatalf("result %+v", result)
	}
	params.IfNoneMatch = param.String(result.Header.Get("ETag"))
	result, err = client.Guardrails.Effective(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != 304 || result.Bundle != nil || result.Header.Get("ETag") != `"registry-abc"` || result.Header.Get("Cache-Control") != "no-store" || result.Header.Get("Orca-Guardrail-Issued-At") != "issued" || result.Header.Get("Orca-Guardrail-Expires-At") != "expires" || result.Header.Get("Orca-Guardrail-Generation") != "42" {
		t.Fatalf("304 result %+v", result)
	}
}

func TestPricingCatalogOverHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apis" {
			_, _ = w.Write([]byte(engineGroups))
			return
		}
		if r.URL.EscapedPath() != "/apis/pricing.runorca.ai/v2/modelpricerules" || r.Method != "GET" || r.Header.Get("X-Test") != "catalog" || r.URL.Query().Get("limit") != "2" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("page") == "next" {
			_, _ = w.Write([]byte(catalogPage(`null`, 2)))
			return
		}
		_, _ = w.Write([]byte(catalogPage(`"next"`, 1)))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ModelPriceRules.List(context.Background(), ModelPriceRuleListParams{Limit: param.Int(2)}, option.WithHeader("X-Test", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	next, err := page.GetNextPage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if next.PublishedAt != "published-2" || next.RequiredFeatures[0] != "future-2" {
		t.Fatalf("lost metadata %+v", next)
	}
}
