// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuardrailServiceUsesPolicyExtensionOverHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/apis":
			_, _ = w.Write([]byte(extensionGroupsJSON(PolicyExtensionGroup)))
		case "/apis/policy.runorca.ai/v1/guardrails/grd%2Fhttp":
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			_, _ = w.Write([]byte(guardrailFixtureJSON))
		default:
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	guardrail, err := client.Guardrails.Get(context.Background(), "grd/http")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if guardrail.ID != "grd_example" {
		t.Fatalf("guardrail ID = %q, want grd_example", guardrail.ID)
	}
}
