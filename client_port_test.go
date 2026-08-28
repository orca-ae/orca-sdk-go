// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

// Ported from orca-sdk-typescript tests/client.test.ts — the request-pipeline
// describes: 'Orca constructor', 'auth header', 'default headers',
// 'HTTP method wrappers', 'retries', 'error mapping', 'response parsing',
// 'buildURL', 'idempotency', 'abort', and 'URL construction — core vs
// extension'. ('legacy baseURL shim' is ported in baseurl_port_test.go.)
//
// Two deliberate differences from the TypeScript suite shape these tests:
//
//  1. Credentials. The TS SDK has one mode (Authorization: Bearer). This
//     client has three constructors — NewClient (bearer), NewAPIKeyClient
//     (x-api-key, the Managed Agents workspace key) and
//     NewUnauthenticatedClient (neither, the analogue of the TS `apiKey:
//     null`). Every auth assertion below covers all three.
//
//  2. Missing capabilities are recorded, not faked. Where a TS behaviour has
//     no counterpart here the test is written as the TS suite specifies it and
//     skipped with a "not implemented:" reason naming the exact capability, so
//     the gap is visible in `go test -v` output instead of being silently
//     dropped:
//
//     go test -v ./... 2>&1 | grep "not implemented:"

// errTransportFailure stands in for the TS suite's `throw new Error('socket
// hang up')` fake fetch: a transport-layer failure that never reaches HTTP.
var errTransportFailure = errors.New("socket hang up")

// newUnauthenticatedRecordingClient is newRecordingClient with no credential,
// covering the TS `apiKey: null` case.
func newUnauthenticatedRecordingClient(tb testing.TB) (*Client, *recordingTransport) {
	tb.Helper()
	transport := &recordingTransport{}
	client, err := NewUnauthenticatedClientWithWarningWriter(
		testBaseURL, &http.Client{Transport: transport}, io.Discard,
	)
	if err != nil {
		tb.Fatalf("NewUnauthenticatedClientWithWarningWriter() error = %v", err)
	}
	return client, transport
}

// getJSON issues a GET and fails the test if it errors, for the many cases
// that only care about the request that was sent.
func getJSON(tb testing.TB, client *Client, path string) {
	tb.Helper()
	var out map[string]any
	if err := client.GetJSON(context.Background(), path, &out); err != nil {
		tb.Fatalf("GetJSON(%q) error = %v", path, err)
	}
}

// -----------------------------------------------------------------------
// 1. Constructor: base URL and credential resolution
// -----------------------------------------------------------------------

func TestClientConstructor(t *testing.T) {
	t.Parallel()

	t.Run("rejects a base URL that is missing or blank", func(t *testing.T) {
		t.Parallel()

		for _, baseURL := range []string{"", "   "} {
			if _, err := NewClient(baseURL, "k", nil); err == nil {
				t.Errorf("NewClient(%q) error = nil, want a missing-base-URL error", baseURL)
			}
		}
	})

	t.Run("rejects an unparseable base URL", func(t *testing.T) {
		t.Parallel()

		for _, baseURL := range []string{"://bad", "ht tp://x"} {
			_, err := NewClient(baseURL, "k", nil)
			if err == nil {
				t.Errorf("NewClient(%q) error = nil, want a parse error", baseURL)
				continue
			}
			if !strings.Contains(err.Error(), baseURL) {
				t.Errorf("NewClient(%q) error = %q, want it to name the offending URL", baseURL, err)
			}
		}
	})

	t.Run("rejects a blank credential", func(t *testing.T) {
		t.Parallel()

		// The TS SDK resolves an absent apiKey to null and simply omits the
		// header; this client makes the two cases explicit instead — an
		// authenticated constructor requires a credential, and callers who
		// want no credential ask for NewUnauthenticatedClient by name.
		if _, err := NewClient(testBaseURL, "   ", nil); err == nil {
			t.Error("NewClient() with a blank token: error = nil, want a missing-token error")
		}
		if _, err := NewAPIKeyClient(testBaseURL, "", nil); err == nil {
			t.Error("NewAPIKeyClient() with a blank key: error = nil, want a missing-key error")
		}
		if _, err := NewUnauthenticatedClient(testBaseURL, nil); err != nil {
			t.Errorf("NewUnauthenticatedClient() error = %v, want nil", err)
		}
	})

	t.Run("supplies a default HTTP client with a deadline", func(t *testing.T) {
		t.Parallel()

		// TS defaults to a 10 minute timeout; this client defaults to 90s,
		// chosen to sit just above the 60s a synchronous function trigger may
		// spend waiting for an output message. The assertion is on the
		// documented constant, not on the TS number.
		client, err := NewClient(testBaseURL, "test-key", nil)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		if got := client.cfg.HTTPClient.Timeout; got != defaultHTTPTimeout {
			t.Errorf("default timeout = %v, want %v", got, defaultHTTPTimeout)
		}
	})

	t.Run("strips trailing slashes from the base URL", func(t *testing.T) {
		t.Parallel()

		// Resolution is textual, so a base that keeps its trailing slashes
		// resolves "v1/agents" to a URL with empty path segments in it. The
		// server sees a route that does not exist, and the caller sees a 404
		// with nothing in it to explain why.
		tests := []struct {
			name    string
			baseURL string
		}{
			{name: "no trailing slash", baseURL: "https://api.example.test"},
			{name: "one trailing slash", baseURL: "https://api.example.test/"},
			{name: "several trailing slashes", baseURL: "https://api.example.test///"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				transport := &recordingTransport{}
				client, err := NewClientWithWarningWriter(
					tc.baseURL, "test-key", &http.Client{Transport: transport}, io.Discard,
				)
				if err != nil {
					t.Fatalf("NewClientWithWarningWriter(%q) error = %v", tc.baseURL, err)
				}

				var out map[string]any
				if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
					t.Fatalf("GetJSON() error = %v", err)
				}

				const want = "https://api.example.test/v1/agents"
				if got := transport.Only(t).URL.String(); got != want {
					t.Errorf("resolved URL = %q, want %q", got, want)
				}
			})
		}
	})

}

// -----------------------------------------------------------------------
// 2. Auth header behaviour, across all three credential modes
// -----------------------------------------------------------------------

func TestClientAuthHeader(t *testing.T) {
	t.Parallel()

	// Each mode is asserted on both credential headers: the one it must send
	// and the one it must not, so a constructor cannot start emitting the
	// other SDK's credential unnoticed.
	tests := []struct {
		name       string
		newClient  func(testing.TB) (*Client, *recordingTransport)
		wantAuthz  string
		wantAPIKey string
	}{
		{
			name:      "bearer client sends Authorization: Bearer <token>",
			newClient: func(tb testing.TB) (*Client, *recordingTransport) { return newRecordingClient(tb, nil) },
			wantAuthz: "Bearer test-key",
		},
		{
			name: "API key client sends x-api-key and no Authorization",
			newClient: func(tb testing.TB) (*Client, *recordingTransport) {
				return newRecordingAPIKeyClient(tb, nil)
			},
			wantAPIKey: "orca_test_key",
		},
		{
			name:      "unauthenticated client sends no credential at all",
			newClient: func(tb testing.TB) (*Client, *recordingTransport) { return newUnauthenticatedRecordingClient(tb) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := tc.newClient(t)
			getJSON(t, client, "/v1/agents")

			call := transport.Only(t)
			if got := call.Header.Get("Authorization"); got != tc.wantAuthz {
				t.Errorf("Authorization = %q, want %q", got, tc.wantAuthz)
			}
			if got := call.Header.Get("x-api-key"); got != tc.wantAPIKey {
				t.Errorf("x-api-key = %q, want %q", got, tc.wantAPIKey)
			}
		})
	}

	t.Run("credential is attached to every verb, not just GET", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		ctx := context.Background()
		var out map[string]any
		if err := client.PostJSON(ctx, "/v1/agents", map[string]string{"name": "a"}, &out); err != nil {
			t.Fatalf("PostJSON() error = %v", err)
		}
		if err := client.Delete(ctx, "/v1/agents/a1"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if err := client.PostMultipart(ctx, "/v1/agents", MultipartRequest{
			ConfigField: "config",
			Config:      map[string]string{"name": "a"},
		}); err != nil {
			t.Fatalf("PostMultipart() error = %v", err)
		}

		for i, call := range transport.Calls() {
			if got := call.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("call %d (%s) Authorization = %q, want %q", i, call.Method, got, "Bearer test-key")
			}
		}
	})

	t.Run("invokes a credential callback per request", func(t *testing.T) {
		t.Parallel()

		// A credential fixed at construction expires while the client is still
		// alive, and the failure only shows up once the token times out. A
		// provider is consulted per attempt, so a rotating token is picked up
		// without rebuilding anything.
		var calls atomic.Int64
		transport := &recordingTransport{}
		client, err := New(
			option.WithBaseURL(testBaseURL),
			option.WithHTTPClient(&http.Client{Transport: transport}),
			option.WithAuthTokenProvider(func(context.Context) (string, error) {
				return fmt.Sprintf("token-%d", calls.Add(1)), nil
			}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx := context.Background()
		var out map[string]any
		for range 3 {
			if err := client.GetJSON(ctx, "/v1/agents", &out); err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
		}

		for i, call := range transport.Calls() {
			want := fmt.Sprintf("Bearer token-%d", i+1)
			if got := call.Header.Get("Authorization"); got != want {
				t.Errorf("call %d Authorization = %q, want %q", i, got, want)
			}
		}

		t.Run("a provider failure surfaces instead of sending an unauthenticated request", func(t *testing.T) {
			t.Parallel()

			transport := &recordingTransport{}
			client, err := New(
				option.WithBaseURL(testBaseURL),
				option.WithHTTPClient(&http.Client{Transport: transport}),
				option.WithAuthTokenProvider(func(context.Context) (string, error) {
					return "", errors.New("vault unreachable")
				}),
			)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			var out map[string]any
			err = client.GetJSON(context.Background(), "/v1/agents", &out)
			if err == nil {
				t.Fatal("GetJSON() error = nil, want the provider failure")
			}
			if !strings.Contains(err.Error(), "vault unreachable") {
				t.Errorf("error = %v, want it to carry the provider failure", err)
			}
			if got := len(transport.Calls()); got != 0 {
				t.Errorf("requests sent = %d, want 0 - an unauthenticated request must not go out", got)
			}
		})
	})
}

// -----------------------------------------------------------------------
// 3. Default headers
// -----------------------------------------------------------------------

func TestClientDefaultHeaders(t *testing.T) {
	t.Parallel()

	t.Run("sets Accept: application/json on JSON requests", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		getJSON(t, client, "/v1/agents")

		if got := transport.Only(t).Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
	})

	t.Run("sets Accept: text/event-stream for a streaming request", func(t *testing.T) {
		t.Parallel()

		// SSE endpoints content-negotiate strictly and 406 on
		// application/json. Where the TS SDK flips Accept from a `stream:
		// true` flag, this client takes the media type as an argument.
		client, transport := newRecordingClient(t, nil)
		err := client.GetStream(context.Background(), "/v1/sessions/s1/events/stream", "text/event-stream",
			func(body io.Reader) error {
				_, err := io.ReadAll(body)
				return err
			})
		if err != nil {
			t.Fatalf("GetStream() error = %v", err)
		}

		if got := transport.Only(t).Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q, want %q", got, "text/event-stream")
		}
	})

	t.Run("falls back to Accept: */* when the stream media type is blank", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		err := client.GetStream(context.Background(), "/v1/packages/p1/download", "  ",
			func(body io.Reader) error {
				_, err := io.ReadAll(body)
				return err
			})
		if err != nil {
			t.Fatalf("GetStream() error = %v", err)
		}

		if got := transport.Only(t).Header.Get("Accept"); got != "*/*" {
			t.Errorf("Accept = %q, want %q", got, "*/*")
		}
	})

	t.Run("merges client-level default headers into every request", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		client = client.WithDefaultHeader("X-Tenant-Id", "tenant-42")

		getJSON(t, client, "/v1/agents")
		getJSON(t, client, "/v1/sessions")

		for i, call := range transport.Calls() {
			if got := call.Header.Get("X-Tenant-Id"); got != "tenant-42" {
				t.Errorf("call %d X-Tenant-Id = %q, want %q", i, got, "tenant-42")
			}
		}
	})

	t.Run("a header the request already sets wins over the default", func(t *testing.T) {
		t.Parallel()

		// The TS analogue is a per-request `headers` option beating
		// defaultHeaders. Here the request-level headers are the ones the
		// client sets itself (Accept, Content-Type, Authorization); a default
		// header must not clobber them.
		client, transport := newRecordingClient(t, nil)
		client = client.WithDefaultHeader("Accept", "text/plain")

		getJSON(t, client, "/v1/agents")

		if got := transport.Only(t).Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want the request-level %q", got, "application/json")
		}
	})

	t.Run("default headers on a clone do not leak back to the original", func(t *testing.T) {
		t.Parallel()

		base, transport := newRecordingClient(t, nil)
		scoped := base.WithDefaultHeader("X-Tenant-Id", "tenant-42")

		getJSON(t, base, "/v1/agents")
		if got := transport.Last(t).Header.Get("X-Tenant-Id"); got != "" {
			t.Errorf("original client X-Tenant-Id = %q, want it unset", got)
		}
		getJSON(t, scoped, "/v1/agents")
		if got := transport.Last(t).Header.Get("X-Tenant-Id"); got != "tenant-42" {
			t.Errorf("clone X-Tenant-Id = %q, want %q", got, "tenant-42")
		}
	})

	t.Run("sets an SDK identification header on every request", func(t *testing.T) {
		t.Parallel()

		// Identification is deliberately just the SDK and its version, with the
		// Go runtime on the User-Agent. No OS, architecture or hostname: naming
		// the client is enough to pick these requests out of a server log, and
		// anything more would tell the deployment about the caller's machine
		// for no benefit to either side.
		client, transport := newRecordingClient(t, nil)

		var out map[string]any
		if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}

		call := transport.Only(t)

		clientHeader := call.Header.Get("X-Orca-Client")
		if !strings.HasPrefix(clientHeader, "orca-sdk-go/") {
			t.Errorf("X-Orca-Client = %q, want it to start with %q", clientHeader, "orca-sdk-go/")
		}
		userAgent := call.Header.Get("User-Agent")
		if !strings.HasPrefix(userAgent, "orca-sdk-go/") {
			t.Errorf("User-Agent = %q, want it to start with %q", userAgent, "orca-sdk-go/")
		}
		if !strings.Contains(userAgent, "go1") {
			t.Errorf("User-Agent = %q, want it to name the Go runtime", userAgent)
		}

		for _, header := range []string{"X-Orca-Client", "User-Agent"} {
			value := call.Header.Get(header)
			for _, leak := range []string{runtime.GOOS, runtime.GOARCH} {
				if strings.Contains(value, leak) {
					t.Errorf("%s = %q, want it not to disclose %q", header, value, leak)
				}
			}
		}

		t.Run("a caller can override the User-Agent", func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, nil)
			var out map[string]any
			err := client.GetJSON(context.Background(), "/v1/agents", &out,
				option.WithHeader("User-Agent", "my-app/1.0"))
			if err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
			if got := transport.Only(t).Header.Get("User-Agent"); got != "my-app/1.0" {
				t.Errorf("User-Agent = %q, want %q", got, "my-app/1.0")
			}
		})
	})
}

// -----------------------------------------------------------------------
// 4. HTTP verb wrappers
// -----------------------------------------------------------------------

func TestClientHTTPMethodWrappers(t *testing.T) {
	t.Parallel()

	t.Run("each wrapper uses its own method", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			call       func(*Client) error
			wantMethod string
			wantBody   map[string]string
		}{
			{
				name: "GetJSON",
				call: func(c *Client) error {
					var out map[string]any
					return c.GetJSON(context.Background(), "/v1/agents", &out)
				},
				wantMethod: http.MethodGet,
			},
			{
				name: "PostJSON",
				call: func(c *Client) error {
					var out map[string]any
					return c.PostJSON(context.Background(), "/v1/agents", map[string]string{"name": "agent-1"}, &out)
				},
				wantMethod: http.MethodPost,
				wantBody:   map[string]string{"name": "agent-1"},
			},
			{
				name: "PutJSON",
				call: func(c *Client) error {
					var out map[string]any
					return c.PutJSON(context.Background(), "/v1/agents/a1", map[string]string{"description": "x"}, &out)
				},
				wantMethod: http.MethodPut,
				wantBody:   map[string]string{"description": "x"},
			},
			{
				name: "PatchJSON",
				call: func(c *Client) error {
					var out map[string]any
					return c.PatchJSON(context.Background(), "/v1/agents/a1", map[string]string{"description": "y"}, &out)
				},
				wantMethod: http.MethodPatch,
				wantBody:   map[string]string{"description": "y"},
			},
			{
				name:       "Delete",
				call:       func(c *Client) error { return c.Delete(context.Background(), "/v1/agents/a1") },
				wantMethod: http.MethodDelete,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, transport := newRecordingClient(t, nil)
				if err := tc.call(client); err != nil {
					t.Fatalf("%s() error = %v", tc.name, err)
				}

				call := transport.Only(t)
				if call.Method != tc.wantMethod {
					t.Errorf("method = %q, want %q", call.Method, tc.wantMethod)
				}

				if tc.wantBody == nil {
					if len(call.Body) != 0 {
						t.Errorf("body = %q, want it empty", call.Body)
					}
					if got := call.Header.Get("Content-Type"); got != "" {
						t.Errorf("Content-Type = %q, want it unset for a body-less request", got)
					}
					return
				}

				if got := call.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want %q", got, "application/json")
				}
				var body map[string]string
				call.JSONBody(t, &body)
				if len(body) != len(tc.wantBody) {
					t.Fatalf("body = %v, want %v", body, tc.wantBody)
				}
				for key, want := range tc.wantBody {
					if body[key] != want {
						t.Errorf("body[%q] = %q, want %q", key, body[key], want)
					}
				}
			})
		}
	})

	t.Run("query parameters supplied in the path reach the wire", func(t *testing.T) {
		t.Parallel()

		// The TS SDK builds the query string from a structured `query`
		// option (see TestClientBuildURL for that gap). Here the caller owns
		// the query string, so what this proves is that path resolution
		// preserves it rather than dropping or re-escaping it.
		client, transport := newRecordingClient(t, nil)
		getJSON(t, client, "/v1/agents?limit=25&archived=false")

		query := transport.Only(t).Query()
		if got := query.Get("limit"); got != "25" {
			t.Errorf("limit = %q, want %q", got, "25")
		}
		if got := query.Get("archived"); got != "false" {
			t.Errorf("archived = %q, want %q", got, "false")
		}
	})

	t.Run("a blank path is rejected before any request is sent", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		var out map[string]any
		if err := client.GetJSON(context.Background(), "   ", &out); err == nil {
			t.Error("GetJSON() with a blank path: error = nil, want a missing-path error")
		}
		if got := len(transport.Calls()); got != 0 {
			t.Errorf("requests sent = %d, want 0", got)
		}
	})
}

// -----------------------------------------------------------------------
// 5. Retry behaviour
// -----------------------------------------------------------------------

func TestClientRetryBehaviour(t *testing.T) {
	t.Parallel()

	t.Run("a status that cannot improve on retry is surfaced immediately", func(t *testing.T) {
		t.Parallel()

		// A rejected credential or a malformed request is deterministic:
		// repeating it produces the identical failure, having spent the backoff
		// and hidden the real error behind a delay.
		for _, status := range []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
		} {
			t.Run(http.StatusText(status), func(t *testing.T) {
				t.Parallel()

				client, transport, delays := newRetryingClient(t, 2, func(*http.Request) (*http.Response, error) {
					return jsonResponse(status, `{"error":"boom"}`), nil
				})

				var out map[string]any
				err := client.GetJSON(context.Background(), "/v1/agents", &out)

				var httpErr *HTTPError
				if !errors.As(err, &httpErr) {
					t.Fatalf("GetJSON() error = %v, want *HTTPError", err)
				}
				if httpErr.StatusCode != status {
					t.Errorf("status = %d, want %d", httpErr.StatusCode, status)
				}
				if got := len(transport.Calls()); got != 1 {
					t.Errorf("requests sent = %d, want exactly 1 (no retries)", got)
				}
				if got := len(*delays); got != 0 {
					t.Errorf("backoff waits = %d, want 0", got)
				}
			})
		}
	})

	t.Run("the legacy constructors do not retry", func(t *testing.T) {
		t.Parallel()

		// Callers built against these predate any retry policy. Silently
		// starting to repeat their mutations would change what their existing
		// code does to the server, so retries are opt-in through New.
		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
		})

		var out map[string]any
		if err := client.GetJSON(context.Background(), "/v1/agents", &out); err == nil {
			t.Fatal("GetJSON() error = nil, want a failure")
		}
		if got := len(transport.Calls()); got != 1 {
			t.Errorf("requests sent = %d, want exactly 1", got)
		}
	})

	t.Run("retries a 500 up to maxRetries", func(t *testing.T) {
		t.Parallel()

		t.Run("gives up after maxRetries and returns the last failure", func(t *testing.T) {
			t.Parallel()

			client, transport, delays := newRetryingClient(t, 2, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
			})

			var out map[string]any
			err := client.GetJSON(context.Background(), "/v1/agents", &out)

			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("GetJSON() error = %v, want *HTTPError", err)
			}
			// maxRetries counts attempts after the first, so 2 means 3 total.
			if got := len(transport.Calls()); got != 3 {
				t.Errorf("requests sent = %d, want 3 (the first attempt plus 2 retries)", got)
			}
			if got := len(*delays); got != 2 {
				t.Errorf("backoff waits = %d, want 2", got)
			}
			// Each wait is longer than the last, so a deployment that is down
			// is not hammered at a fixed rate.
			if len(*delays) == 2 && (*delays)[1] <= (*delays)[0] {
				t.Errorf("delays = %v, want the backoff to grow", *delays)
			}
		})

		t.Run("stops as soon as an attempt succeeds", func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Int64
			client, transport, _ := newRetryingClient(t, 5, func(*http.Request) (*http.Response, error) {
				if attempts.Add(1) < 3 {
					return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
				}
				return jsonResponse(http.StatusOK, `{"id":"agent-1"}`), nil
			})

			var out struct {
				ID string `json:"id"`
			}
			if err := client.GetJSON(context.Background(), "/v1/agents/a1", &out); err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
			if out.ID != "agent-1" {
				t.Errorf("id = %q, want %q", out.ID, "agent-1")
			}
			if got := len(transport.Calls()); got != 3 {
				t.Errorf("requests sent = %d, want 3", got)
			}
		})

		t.Run("retries a transport failure", func(t *testing.T) {
			t.Parallel()

			client, transport, _ := newRetryingClient(t, 2, func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial tcp: connection refused")
			})

			var out map[string]any
			err := client.GetJSON(context.Background(), "/v1/agents", &out)

			var connErr *ConnectionError
			if !errors.As(err, &connErr) {
				t.Fatalf("GetJSON() error = %T, want *ConnectionError", err)
			}
			if got := len(transport.Calls()); got != 3 {
				t.Errorf("requests sent = %d, want 3", got)
			}
		})

		t.Run("records the attempt number on each request", func(t *testing.T) {
			t.Parallel()

			client, transport, _ := newRetryingClient(t, 2, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{}`), nil
			})

			var out map[string]any
			_ = client.GetJSON(context.Background(), "/v1/agents", &out)

			for i, call := range transport.Calls() {
				want := strconv.Itoa(i)
				if got := call.Header.Get("X-Orca-Retry-Count"); got != want {
					t.Errorf("call %d X-Orca-Retry-Count = %q, want %q", i, got, want)
				}
			}
		})

		t.Run("a retried body is sent again in full", func(t *testing.T) {
			t.Parallel()

			// The body reader is consumed by the first attempt, so a retry that
			// reused it would send an empty body - a silent, data-losing bug
			// that only shows up under failure.
			client, transport, _ := newRetryingClient(t, 1, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{}`), nil
			})

			var out map[string]any
			_ = client.PostJSON(context.Background(), "/v1/agents", map[string]string{"name": "demo"}, &out)

			calls := transport.Calls()
			if len(calls) != 2 {
				t.Fatalf("captured %d requests, want 2", len(calls))
			}
			for i, call := range calls {
				if got, want := string(call.Body), `{"name":"demo"}`; got != want {
					t.Errorf("call %d body = %q, want %q", i, got, want)
				}
			}
		})
	})

	t.Run("retries a 429 honouring the retry-after-ms header", func(t *testing.T) {
		t.Parallel()

		// A server that says how long to wait is obeyed. Guessing a backoff
		// when the answer was in the response is how a client turns rate
		// limiting into an outage.
		tests := []struct {
			name   string
			header string
			value  string
			want   time.Duration
		}{
			{name: "retry-after-ms", header: "retry-after-ms", value: "1500", want: 1500 * time.Millisecond},
			{name: "Retry-After seconds", header: "Retry-After", value: "2", want: 2 * time.Second},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var attempts atomic.Int64
				client, _, delays := newRetryingClient(t, 1, func(*http.Request) (*http.Response, error) {
					if attempts.Add(1) == 1 {
						res := jsonResponse(http.StatusTooManyRequests, `{}`)
						res.Header.Set(tc.header, tc.value)
						return res, nil
					}
					return jsonResponse(http.StatusOK, `{}`), nil
				})

				var out map[string]any
				if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
					t.Fatalf("GetJSON() error = %v", err)
				}
				if len(*delays) != 1 {
					t.Fatalf("backoff waits = %d, want 1", len(*delays))
				}
				if (*delays)[0] != tc.want {
					t.Errorf("delay = %v, want %v (the value the server asked for)", (*delays)[0], tc.want)
				}
			})
		}
	})

	t.Run("honours x-should-retry response overrides", func(t *testing.T) {
		t.Parallel()

		// A deployment knows things about its own failure modes that a status
		// code cannot express, so it can mark a normally-terminal status as
		// transient, or a normally-transient one as final.
		tests := []struct {
			name        string
			status      int
			shouldRetry string
			wantCalls   int
		}{
			{name: "a 400 marked retryable is retried", status: http.StatusBadRequest, shouldRetry: "true", wantCalls: 2},
			{name: "a 500 marked terminal is not", status: http.StatusInternalServerError, shouldRetry: "false", wantCalls: 1},
			{name: "an unrecognised value falls back to the status", status: http.StatusInternalServerError, shouldRetry: "maybe", wantCalls: 2},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, transport, _ := newRetryingClient(t, 1, func(*http.Request) (*http.Response, error) {
					res := jsonResponse(tc.status, `{}`)
					res.Header.Set("x-should-retry", tc.shouldRetry)
					return res, nil
				})

				var out map[string]any
				_ = client.GetJSON(context.Background(), "/v1/agents", &out)

				if got := len(transport.Calls()); got != tc.wantCalls {
					t.Errorf("requests sent = %d, want %d", got, tc.wantCalls)
				}
			})
		}
	})
}

// -----------------------------------------------------------------------
// 6. Error mapping
// -----------------------------------------------------------------------

func TestClientErrorMapping(t *testing.T) {
	t.Parallel()

	t.Run("a 404 surfaces an HTTPError carrying status, method, URL and body", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, `{"error":{"type":"not_found"}}`), nil
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents/a1", &out)

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("GetJSON() error = %v, want *HTTPError", err)
		}
		if httpErr.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
		}
		if httpErr.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", httpErr.Method, http.MethodGet)
		}
		if want := testBaseURL + "/v1/agents/a1"; httpErr.URL != want {
			t.Errorf("URL = %q, want %q", httpErr.URL, want)
		}
		if !strings.Contains(httpErr.Body, "not_found") {
			t.Errorf("body = %q, want it to carry the server payload", httpErr.Body)
		}
	})

	t.Run("a transport failure is not an HTTPError", func(t *testing.T) {
		t.Parallel()

		// The TS SDK maps this to APIConnectionError. Here the distinction is
		// carried by the error's type: a protocol-level failure is an
		// *HTTPError with a status, a transport failure is not, so callers
		// switching on *HTTPError cannot mistake "the socket died" for a
		// response the server actually sent.
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return nil, errTransportFailure
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out)
		if err == nil {
			t.Fatal("GetJSON() error = nil, want a transport failure")
		}

		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			t.Errorf("error = %v, want it NOT to be an *HTTPError", err)
		}
		if !errors.Is(err, errTransportFailure) {
			t.Errorf("error = %v, want it to wrap the transport failure", err)
		}
	})

	t.Run("maps each status to a distinct error type", func(t *testing.T) {
		t.Parallel()

		// The exhaustive per-status mapping lives in error_port_test.go. What
		// this pins is the property the request pipeline is responsible for:
		// whichever specific type comes back, it still carries the method, URL
		// and status of the request that produced it, so a caller matching the
		// narrow type loses none of the context the general one carried.
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusTooManyRequests, `{"error":{"type":"rate_limit"}}`), nil
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents/a1", &out)

		var rateLimitErr *RateLimitError
		if !errors.As(err, &rateLimitErr) {
			t.Fatalf("GetJSON() error = %T, want *RateLimitError", err)
		}
		if rateLimitErr.StatusCode != http.StatusTooManyRequests {
			t.Errorf("status = %d, want %d", rateLimitErr.StatusCode, http.StatusTooManyRequests)
		}
		if rateLimitErr.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", rateLimitErr.Method, http.MethodGet)
		}
		if want := testBaseURL + "/v1/agents/a1"; rateLimitErr.URL != want {
			t.Errorf("URL = %q, want %q", rateLimitErr.URL, want)
		}
	})
}

// -----------------------------------------------------------------------
// 7. Response parsing
// -----------------------------------------------------------------------

func TestClientResponseParsing(t *testing.T) {
	t.Parallel()

	t.Run("a JSON body is decoded into the output value", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"agent-1","name":"x"}`), nil
		})

		var out struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := client.GetJSON(context.Background(), "/v1/agents/a1", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if out.ID != "agent-1" || out.Name != "x" {
			t.Errorf("decoded = %+v, want {ID:agent-1 Name:x}", out)
		}
	})

	t.Run("an empty success body leaves the output untouched", func(t *testing.T) {
		t.Parallel()

		// The TS SDK resolves a 204 to null. The Go analogue of "no value" is
		// leaving the caller's destination alone and returning no error, for
		// both a 204 and a whitespace-only 200.
		tests := []struct {
			name string
			body *http.Response
		}{
			{name: "204 no content", body: &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}},
			{name: "200 with a blank body", body: jsonResponse(http.StatusOK, "   ")},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
					return tc.body, nil
				})

				out := map[string]any{"untouched": true}
				if err := client.GetJSON(context.Background(), "/v1/agents/a1", &out); err != nil {
					t.Fatalf("GetJSON() error = %v", err)
				}
				if len(out) != 1 || out["untouched"] != true {
					t.Errorf("out = %v, want it untouched", out)
				}
			})
		}
	})

	t.Run("Delete tolerates a 204 with no body", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})

		if err := client.Delete(context.Background(), "/v1/agents/a1"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	})

	t.Run("an undecodable body surfaces a decode error naming the request", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, "not json"), nil
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out)
		if err == nil {
			t.Fatal("GetJSON() error = nil, want a decode error")
		}
		for _, want := range []string{"decode", http.MethodGet, testBaseURL + "/v1/agents"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to mention %q", err, want)
			}
		}
	})
}

// -----------------------------------------------------------------------
// 8. URL building
// -----------------------------------------------------------------------

func TestClientBuildURL(t *testing.T) {
	t.Parallel()

	t.Run("an absolute path bypasses the base URL", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		getJSON(t, client, "https://other.example.test/health")

		if got := transport.Only(t).URL.String(); got != "https://other.example.test/health" {
			t.Errorf("request URL = %q, want %q", got, "https://other.example.test/health")
		}
	})

	t.Run("a leading slash is optional", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		getJSON(t, client, "v1/agents")
		if got := transport.Only(t).URL.String(); got != testBaseURL+"/v1/agents" {
			t.Errorf("request URL = %q, want %q", got, testBaseURL+"/v1/agents")
		}
	})

	t.Run("merges a client-level defaultQuery into every request", func(t *testing.T) {
		t.Parallel()

		// A workspace or tenant selector has to ride on every call. Without a
		// client-level default, every resource method would have to remember to
		// append it, and the one that forgets reads another tenant's data.
		client, transport := newRecordingClientWith(t, nil, option.WithQuery("workspace", "ws-1"))

		var out map[string]any
		if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if got := transport.Only(t).Query().Get("workspace"); got != "ws-1" {
			t.Errorf("workspace = %q, want %q", got, "ws-1")
		}
	})

	t.Run("a default query merges with a query the path already carries", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClientWith(t, nil, option.WithQuery("workspace", "ws-1"))

		var out map[string]any
		if err := client.GetJSON(context.Background(), "/v1/agents?limit=10", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		query := transport.Only(t).Query()
		if got := query.Get("workspace"); got != "ws-1" {
			t.Errorf("workspace = %q, want %q", got, "ws-1")
		}
		if got := query.Get("limit"); got != "10" {
			t.Errorf("limit = %q, want %q", got, "10")
		}
	})

	t.Run("a per-request query overrides the defaultQuery", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClientWith(t, nil, option.WithQuery("workspace", "ws-1"))

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out,
			option.WithQuery("workspace", "ws-override"))
		if err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		query := transport.Only(t).Query()
		if got := query["workspace"]; len(got) != 1 || got[0] != "ws-override" {
			t.Errorf("workspace = %v, want exactly [ws-override] - the default must be replaced, not appended", got)
		}
	})

	t.Run("a per-request option does not leak onto the client", func(t *testing.T) {
		t.Parallel()

		// Per-call options are the reason a client is safe to share. If one
		// call's option mutated the client, a concurrent call would silently
		// inherit it.
		client, transport := newRecordingClientWith(t, nil, option.WithQuery("workspace", "ws-1"))

		ctx := context.Background()
		var out map[string]any
		if err := client.GetJSON(ctx, "/v1/agents", &out, option.WithQuery("workspace", "ws-override")); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if err := client.GetJSON(ctx, "/v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}

		calls := transport.Calls()
		if len(calls) != 2 {
			t.Fatalf("captured %d requests, want 2", len(calls))
		}
		if got := calls[1].Query().Get("workspace"); got != "ws-1" {
			t.Errorf("second call workspace = %q, want the client default %q", got, "ws-1")
		}
	})
}

// -----------------------------------------------------------------------
// 9. Idempotency key
// -----------------------------------------------------------------------

func TestClientIdempotencyKey(t *testing.T) {
	t.Parallel()

	t.Run("sets Idempotency-Key on a mutating request when supplied", func(t *testing.T) {
		t.Parallel()

		// A key identifies one logical operation, so it belongs on the call
		// rather than the client - a client-wide key would make every mutation
		// look like a replay of the same one.
		for _, tc := range []struct {
			method string
			call   func(*Client) error
		}{
			{http.MethodPost, func(c *Client) error {
				var out map[string]any
				return c.PostJSON(context.Background(), "/v1/agents", map[string]string{"name": "a"}, &out,
					option.WithIdempotencyKey("idem-123"))
			}},
			{http.MethodPut, func(c *Client) error {
				var out map[string]any
				return c.PutJSON(context.Background(), "/v1/agents/a1", map[string]string{"name": "a"}, &out,
					option.WithIdempotencyKey("idem-123"))
			}},
			{http.MethodPatch, func(c *Client) error {
				var out map[string]any
				return c.PatchJSON(context.Background(), "/v1/agents/a1", map[string]string{"name": "a"}, &out,
					option.WithIdempotencyKey("idem-123"))
			}},
			{http.MethodDelete, func(c *Client) error {
				return c.Delete(context.Background(), "/v1/agents/a1", option.WithIdempotencyKey("idem-123"))
			}},
		} {
			t.Run(tc.method, func(t *testing.T) {
				t.Parallel()

				client, transport := newRecordingClient(t, nil)
				if err := tc.call(client); err != nil {
					t.Fatalf("%s error = %v", tc.method, err)
				}
				if got := transport.Only(t).Header.Get("Idempotency-Key"); got != "idem-123" {
					t.Errorf("Idempotency-Key = %q, want %q", got, "idem-123")
				}
			})
		}
	})

	t.Run("does not set Idempotency-Key on GET", func(t *testing.T) {
		t.Parallel()

		// Replaying a read is already safe, so a key on a GET would ask the
		// server to deduplicate something that never needed it - and would
		// pin the caller to a cached answer if the server obliged.
		client, transport := newRecordingClient(t, nil)

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out,
			option.WithIdempotencyKey("idem-123"))
		if err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if got := transport.Only(t).Header.Get("Idempotency-Key"); got != "" {
			t.Errorf("Idempotency-Key = %q, want it unset on a read", got)
		}
	})

	t.Run("the default-header workaround also stamps the key onto reads", func(t *testing.T) {
		t.Parallel()

		// Recorded deliberately: WithDefaultHeader is the only way to send an
		// Idempotency-Key today, and because it is client-scoped it reaches
		// GETs too — which is exactly what the TS suite asserts must not
		// happen. Anyone reaching for this workaround should see that first.
		client, transport := newRecordingClient(t, nil)
		client = client.WithDefaultHeader("Idempotency-Key", "idem-123")

		var out map[string]any
		if err := client.PostJSON(context.Background(), "/v1/agents", map[string]string{"name": "a"}, &out); err != nil {
			t.Fatalf("PostJSON() error = %v", err)
		}
		getJSON(t, client, "/v1/agents")

		calls := transport.Calls()
		if len(calls) != 2 {
			t.Fatalf("captured %d requests, want 2", len(calls))
		}
		if got := calls[0].Header.Get("Idempotency-Key"); got != "idem-123" {
			t.Errorf("POST Idempotency-Key = %q, want %q", got, "idem-123")
		}
		if got := calls[1].Header.Get("Idempotency-Key"); got != "idem-123" {
			t.Errorf("GET Idempotency-Key = %q, want %q — the client-scoped header cannot be "+
				"confined to mutating requests", got, "idem-123")
		}
	})
}

// -----------------------------------------------------------------------
// 10. Cancellation
// -----------------------------------------------------------------------

func TestClientRequestAbort(t *testing.T) {
	t.Parallel()

	// These two use a real server and transport rather than the recording
	// transport: cancellation is enforced by http.Transport, so a stub
	// RoundTripper that ignores the request context would prove nothing.

	t.Run("an already-cancelled context fails before the request is sent", func(t *testing.T) {
		t.Parallel()

		var served atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			served.Add(1)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client, err := NewClientWithWarningWriter(server.URL, "test-key", server.Client(), io.Discard)
		if err != nil {
			t.Fatalf("NewClientWithWarningWriter() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		var out map[string]any
		err = client.GetJSON(ctx, "/v1/agents", &out)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("GetJSON() error = %v, want it to wrap context.Canceled", err)
		}
		if got := served.Load(); got != 0 {
			t.Errorf("server saw %d requests, want 0", got)
		}
	})

	t.Run("an expired deadline surfaces context.DeadlineExceeded", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		client, err := NewClientWithWarningWriter(server.URL, "test-key", server.Client(), io.Discard)
		if err != nil {
			t.Fatalf("NewClientWithWarningWriter() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var out map[string]any
		err = client.GetJSON(ctx, "/v1/agents", &out)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("GetJSON() error = %v, want it to wrap context.DeadlineExceeded", err)
		}
	})
}

// -----------------------------------------------------------------------
// 11. URL construction — core vs extension
// -----------------------------------------------------------------------

// From a base URL with no path (the host root), a core path must resolve to
// exactly {base}/v1/... and an extension path to exactly
// {base}/apis/cloud.sn.io/v1/.... Asserted against literal strings, never
// against the constants the implementation uses.
func TestClientURLConstructionCoreVsExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantURL string
	}{
		{
			name:    "a core path resolves to {base}/v1/agents",
			path:    "/v1/agents",
			wantURL: "https://workspace.example.com/v1/agents",
		},
		{
			name:    "an extension path resolves to {base}/apis/cloud.sn.io/v1/connections",
			path:    "/apis/cloud.sn.io/v1/connections",
			wantURL: "https://workspace.example.com/apis/cloud.sn.io/v1/connections",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport, _ := newShimClient(t, "https://workspace.example.com")
			if got := requestURL(t, client, transport, tc.path); got != tc.wantURL {
				t.Errorf("request URL = %q, want %q", got, tc.wantURL)
			}
		})
	}

	t.Run("a path prefix scopes an extension client to its API group", func(t *testing.T) {
		t.Parallel()

		// WithPathPrefix is how a resource client keeps passing short,
		// resource-relative paths now that the base URL is the host root.
		client, transport, _ := newShimClient(t, "https://workspace.example.com")
		scoped := client.WithPathPrefix("apis/cloud.sn.io/v1")

		want := "https://workspace.example.com/apis/cloud.sn.io/v1/connections"
		if got := requestURL(t, scoped, transport, "/connections"); got != want {
			t.Errorf("request URL = %q, want %q", got, want)
		}
	})
}

// TestClientEnvironmentFallback is deliberately a top-level, non-parallel test:
// t.Setenv panics under a parallel ancestor, because the environment is
// process-wide and a sibling could observe a value meant for this test alone.
func TestClientEnvironmentFallback(t *testing.T) {
	// Reading the deployment from the environment is what lets one binary
	// be pointed at dev, staging and production without a recompile.
	t.Setenv("ORCA_BASE_URL", testBaseURL)
	t.Setenv("ORCA_API_KEY", "orca_from_env")

	transport := &recordingTransport{}
	client, err := New(option.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var out map[string]any
	if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}

	call := transport.Only(t)
	if got := call.Header.Get("x-api-key"); got != "orca_from_env" {
		t.Errorf("x-api-key = %q, want %q", got, "orca_from_env")
	}
	if want := testBaseURL + "/v1/agents"; call.URL.String() != want {
		t.Errorf("URL = %q, want %q", call.URL.String(), want)
	}

	t.Run("an explicit option beats the environment", func(t *testing.T) {
		transport := &recordingTransport{}
		client, err := New(
			option.WithHTTPClient(&http.Client{Transport: transport}),
			option.WithAPIKey("orca_explicit"),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		var out map[string]any
		if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		if got := transport.Only(t).Header.Get("x-api-key"); got != "orca_explicit" {
			t.Errorf("x-api-key = %q, want the explicit option to win", got)
		}
	})
}
