package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagedAgentsClientSendsJSONRequests(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agents/agent-1" {
			t.Fatalf("path = %q, want /v1/agents/agent-1", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		// Deliberately no anthropic-beta assertion: ManagedAgentsClient must not send it by
		// default (see the doc comment on ManagedAgentsClient). Assert its absence below.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"agent-1"}`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := NewManagedAgentsClient(baseClient)

	result, err := client.Update(context.Background(), http.MethodPost, "/v1/agents/agent-1", map[string]string{
		"name": "updated",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}
	if resultMap["id"] != "agent-1" {
		t.Fatalf("result id = %v, want agent-1", resultMap["id"])
	}
}

func TestManagedAgentsClientDoesNotSendAnthropicBetaHeaderByDefault(t *testing.T) {
	t.Parallel()

	var gotHeader string
	sawHeader := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader, sawHeader = r.Header.Get("anthropic-beta"), r.Header.Get("anthropic-beta") != ""
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := NewManagedAgentsClient(baseClient)

	if _, err := client.Get(context.Background(), "/v1/agents"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if sawHeader {
		t.Fatalf("anthropic-beta header = %q, want it not to be sent at all", gotHeader)
	}
}
