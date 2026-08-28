// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// Ported from orca-sdk-typescript tests/backend-compatibility.test.ts.
//
// One backend, two ways of addressing it: a deployment reached at its host
// root and a deployment still configured with the legacy union base URL
// (.../v1/registry). Both must produce byte-identical core request paths and
// carry the caller's credential, so a single backend can serve both without
// knowing which shape the client was configured with.
//
// The TypeScript suite drives this through typed resource accessors
// (client.agents.retrieve, client.sessions.files.retrieve,
// client.triggers.retrieve). This SDK has no typed Managed Agents resources
// yet, so the paths are exercised through ManagedAgentsClient, the raw-path
// resource client it does ship; the typed-accessor tests are recorded below as
// pending.

// backendCompatCorePaths are the canonical core paths the TS suite asserts,
// spelled out literally rather than built from a constant.
var backendCompatCorePaths = []string{
	"/v1/agents/agent_abc",
	"/v1/sessions/session_abc/files/file_abc",
	"/v1/triggers/trigger_abc",
}

// backendCompatBodies is the stub backend: a payload per canonical path. Any
// other path is answered with a 404 naming what was asked for, so a path that
// drifts fails at the request rather than silently returning a plausible body.
var backendCompatBodies = map[string]string{
	"/v1/agents/agent_abc": `{"id":"agent_abc","type":"agent","name":"Agent",` +
		`"model":{"id":"model"},"version":1,` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
	"/v1/sessions/session_abc/files/file_abc": `{"id":"file_abc","filename":"result.txt",` +
		`"mime_type":"text/plain","size_bytes":6,"created_at":"2026-01-01T00:00:00Z"}`,
	"/v1/triggers/trigger_abc": `{"id":"trigger_abc","type":"trigger","name":"Trigger",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
}

func backendCompatResponder(req *http.Request) (*http.Response, error) {
	if body, ok := backendCompatBodies[req.URL.Path]; ok {
		return jsonResponse(http.StatusOK, body), nil
	}
	return jsonResponse(http.StatusNotFound,
		fmt.Sprintf(`{"error":"no route for %s"}`, req.URL.Path)), nil
}

// newBackendCompatClient wires a Managed Agents client to the stub backend at
// baseURL. credential selects the constructor under test.
func newBackendCompatClient(tb testing.TB, baseURL, credential string) (*ManagedAgentsClient, *recordingTransport) {
	tb.Helper()

	transport := &recordingTransport{respond: backendCompatResponder}
	httpClient := &http.Client{Transport: transport}

	var client *Client
	var err error
	switch credential {
	case "bearer":
		client, err = NewClientWithWarningWriter(baseURL, "test-key", httpClient, io.Discard)
	case "api-key":
		client, err = NewAPIKeyClientWithWarningWriter(baseURL, "orca_test_key", httpClient, io.Discard)
	default:
		tb.Fatalf("unknown credential mode %q", credential)
	}
	if err != nil {
		tb.Fatalf("client for %q (%s) error = %v", baseURL, credential, err)
	}

	return NewManagedAgentsClient(client), transport
}

// exerciseBackendCompatCore retrieves each canonical core resource and returns
// the ids the backend echoed back, proving the response round-tripped and not
// merely that a request was sent.
func exerciseBackendCompatCore(t *testing.T, client *ManagedAgentsClient) []string {
	t.Helper()

	ids := make([]string, 0, len(backendCompatCorePaths))
	for _, path := range backendCompatCorePaths {
		result, err := client.Get(context.Background(), path)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", path, err)
		}
		object, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Get(%q) result type = %T, want a JSON object", path, result)
		}
		id, _ := object["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func TestSharedBackendCompatibilityCorePaths(t *testing.T) {
	t.Parallel()

	wantIDs := []string{"agent_abc", "file_abc", "trigger_abc"}

	tests := []struct {
		name       string
		baseURL    string
		credential string
		wantHeader string
		wantValue  string
		otherAuth  string // header that must stay unset for this credential mode
	}{
		{
			name:       "a host-root base URL",
			baseURL:    "https://engine.example.test",
			credential: "bearer",
			wantHeader: "Authorization",
			wantValue:  "Bearer test-key",
			otherAuth:  "x-api-key",
		},
		{
			name:       "a legacy union base URL",
			baseURL:    "https://distribution.example.test/v1/registry",
			credential: "bearer",
			wantHeader: "Authorization",
			wantValue:  "Bearer test-key",
			otherAuth:  "x-api-key",
		},
		{
			// The TS SDK has no API-key mode; this client does, and the same
			// backend has to serve it over the same paths.
			name:       "a host-root base URL with an API key credential",
			baseURL:    "https://engine.example.test",
			credential: "api-key",
			wantHeader: "x-api-key",
			wantValue:  "orca_test_key",
			otherAuth:  "Authorization",
		},
		{
			name:       "a legacy union base URL with an API key credential",
			baseURL:    "https://distribution.example.test/v1/registry",
			credential: "api-key",
			wantHeader: "x-api-key",
			wantValue:  "orca_test_key",
			otherAuth:  "Authorization",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newBackendCompatClient(t, tc.baseURL, tc.credential)

			ids := exerciseBackendCompatCore(t, client)
			for i, want := range wantIDs {
				if ids[i] != want {
					t.Errorf("resource %d id = %q, want %q", i, ids[i], want)
				}
			}

			calls := transport.Calls()
			if len(calls) != len(backendCompatCorePaths) {
				t.Fatalf("captured %d requests, want %d", len(calls), len(backendCompatCorePaths))
			}
			for i, call := range calls {
				if got := call.Path(); got != backendCompatCorePaths[i] {
					t.Errorf("request %d path = %q, want %q", i, got, backendCompatCorePaths[i])
				}
				if got := call.Header.Get(tc.wantHeader); got != tc.wantValue {
					t.Errorf("request %d %s = %q, want %q", i, tc.wantHeader, got, tc.wantValue)
				}
				if got := call.Header.Get(tc.otherAuth); got != "" {
					t.Errorf("request %d %s = %q, want it unset", i, tc.otherAuth, got)
				}
			}
		})
	}
}

// TestSharedBackendCompatibilityTypedResources records the surface the TS
// suite drives that this SDK does not expose yet. Each subtest is one typed
// accessor a caller would reach for instead of ManagedAgentsClient's raw
// paths; together they are the outstanding work behind the raw-path test
// above.
func TestSharedBackendCompatibilityTypedResources(t *testing.T) {
	t.Parallel()

	// The same claim as the raw-path test above, made through the typed
	// accessors: whichever base-URL shape a deployment was configured with, a
	// typed resource has to produce the identical core path. The typed layer is
	// where a stray prefix would be easiest to introduce and hardest to notice.
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "a host-root base URL", baseURL: "https://engine.example.test"},
		{name: "a legacy union base URL", baseURL: "https://engine.example.test/v1/registry"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transport := &recordingTransport{respond: backendCompatResponder}
			client, err := NewClientWithWarningWriter(
				tc.baseURL, "test-key", &http.Client{Transport: transport}, io.Discard,
			)
			if err != nil {
				t.Fatalf("client for %q error = %v", tc.baseURL, err)
			}

			t.Run("agents.retrieve", func(t *testing.T) {
				agent, err := client.Agents.Get(context.Background(), "agent_abc", AgentGetParams{})
				if err != nil {
					t.Fatalf("Agents.Get() error = %v", err)
				}
				if agent.ID != "agent_abc" {
					t.Errorf("id = %q, want %q", agent.ID, "agent_abc")
				}
				if got, want := transport.Last(t).Path(), "/v1/agents/agent_abc"; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
			})

			t.Run("sessions.files.retrieve", func(t *testing.T) {
				file, err := client.Sessions.Files.Get(context.Background(), "session_abc", "file_abc")
				if err != nil {
					t.Fatalf("Sessions.Files.Get() error = %v", err)
				}
				if file.ID != "file_abc" {
					t.Errorf("id = %q, want %q", file.ID, "file_abc")
				}
				if got, want := transport.Last(t).Path(), "/v1/sessions/session_abc/files/file_abc"; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
			})

			t.Run("triggers.retrieve", func(t *testing.T) {
				t.Skip(pendingManagedAgents)
			})
		})
	}
}
