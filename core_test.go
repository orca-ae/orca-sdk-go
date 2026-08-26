package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAPIVersionsUsesAuthenticatedHostRootPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Fatalf("path = %q, want /api", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"],"preferred_version":"v1"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	versions, err := client.GetAPIVersions(context.Background())
	if err != nil {
		t.Fatalf("GetAPIVersions() error = %v", err)
	}
	if versions.Kind != "APIVersions" || versions.PreferredVersion != "v1" || len(versions.Versions) != 1 {
		t.Fatalf("versions = %#v", versions)
	}
}

func TestCoreProbesAreUnauthenticated(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want empty", got)
		}
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok","service":"managed-agents"}`))
		case "/readyz":
			_, _ = w.Write([]byte(`{"status":"ready","service":"managed-agents"}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewUnauthenticatedClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewUnauthenticatedClient() error = %v", err)
	}
	health, err := client.GetHealthz(context.Background())
	if err != nil || health.Status != "ok" {
		t.Fatalf("health = %#v, error = %v", health, err)
	}
	ready, err := client.GetReadyz(context.Background())
	if err != nil || ready.Status != "ready" {
		t.Fatalf("ready = %#v, error = %v", ready, err)
	}
}
