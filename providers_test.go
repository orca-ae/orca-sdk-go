// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProvidersClientListUsesAgentsProvidersPath(t *testing.T) {
	t.Parallel()

	client := newProvidersTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/agents/providers", `[{"name":"openai","type":"llm","api_key_configured":true}]`)

	items, err := NewProvidersClient(client).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Name != "openai" || items[0].Type != "llm" || !items[0].APIKeyConfigured {
		t.Fatalf("List() = %#v, want configured openai llm provider", items)
	}
}

func TestProvidersClientGetUsesEscapedPath(t *testing.T) {
	t.Parallel()

	client := newProvidersTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/agents/providers/openai%2Fprod", `{"name":"openai/prod","type":"llm"}`)

	item, err := NewProvidersClient(client).Get(context.Background(), "openai/prod")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item == nil || item.Name != "openai/prod" || item.Type != "llm" {
		t.Fatalf("Get() = %#v, want openai/prod llm provider", item)
	}
}

func newProvidersTestClient(t *testing.T, wantMethod, wantPath, response string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != wantMethod {
			t.Fatalf("method = %q, want %q", r.Method, wantMethod)
		}
		if r.URL.EscapedPath() != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.EscapedPath(), wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}
