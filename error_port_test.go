// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Ported from orca-sdk-typescript tests/core/error.test.ts.
//
// The TypeScript SDK maps each failing status onto its own error class
// (BadRequestError, AuthenticationError, NotFoundError, RateLimitError, …)
// descending from a shared OrcaError, and asserts the mapping with
// `instanceof`. This client has a single error type — *HTTPError, carrying
// Method, URL, StatusCode and Body — so the mapping the TS suite specifies
// collapses to "the status is reported faithfully and the caller switches on
// it".
//
// What that costs is recorded here rather than papered over: the tests that
// need a class hierarchy, a request ID, or response headers are written as the
// TS suite specifies them and skipped with a "not implemented:" reason.

// errorStatuses are the statuses tests/core/error.test.ts pins to a specific
// error class, plus the two it uses for the fall-through case.
var errorStatuses = []int{
	http.StatusBadRequest,          // 400 → BadRequestError
	http.StatusUnauthorized,        // 401 → AuthenticationError
	http.StatusForbidden,           // 403 → PermissionDeniedError
	http.StatusNotFound,            // 404 → NotFoundError
	http.StatusConflict,            // 409 → ConflictError
	http.StatusUnprocessableEntity, // 422 → UnprocessableEntityError
	http.StatusTooManyRequests,     // 429 → RateLimitError
	http.StatusInternalServerError, // 500 → InternalServerError
	http.StatusServiceUnavailable,  // 503 → InternalServerError
	599,                            // 599 → InternalServerError (any 5xx)
	http.StatusTeapot,              // 418 → plain APIError (unmapped status)
}

// -----------------------------------------------------------------------
// 1. Status classification (the analogue of APIError.generate)
// -----------------------------------------------------------------------

func TestHTTPErrorStatusClassification(t *testing.T) {
	t.Parallel()

	for _, status := range errorStatuses {
		t.Run(fmt.Sprintf("%d", status), func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`{"error":{"status":%d}}`, status)
			client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(status, body), nil
			})

			var out map[string]any
			err := client.GetJSON(context.Background(), "/v1/agents/a1", &out)

			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("GetJSON() error = %v (%T), want *HTTPError", err, err)
			}
			if httpErr.StatusCode != status {
				t.Errorf("StatusCode = %d, want %d", httpErr.StatusCode, status)
			}
			if httpErr.Body != body {
				t.Errorf("Body = %q, want %q", httpErr.Body, body)
			}
		})
	}

	t.Run("a 3xx without a Location header is an error, not a redirect", func(t *testing.T) {
		t.Parallel()

		// Go-specific and worth pinning: only 2xx is success here, so a 302
		// the transport cannot follow surfaces as an *HTTPError rather than
		// being silently returned as a body.
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusFound, `{}`), nil
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out)

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("GetJSON() error = %v, want *HTTPError", err)
		}
		if httpErr.StatusCode != http.StatusFound {
			t.Errorf("StatusCode = %d, want %d", httpErr.StatusCode, http.StatusFound)
		}
	})

	t.Run("maps each status to its own error class", func(t *testing.T) {
		t.Parallel()
		t.Skip("not implemented: per-status error classes (BadRequestError, AuthenticationError, " +
			"PermissionDeniedError, NotFoundError, ConflictError, UnprocessableEntityError, " +
			"RateLimitError, InternalServerError) — every failing status yields the same " +
			"*HTTPError, so callers must switch on StatusCode")
	})

	t.Run("exposes a shared error root the whole SDK descends from", func(t *testing.T) {
		t.Parallel()
		t.Skip("not implemented: shared OrcaError root type — there is no package-level error " +
			"type that argument-validation, decode and HTTP failures all satisfy, so " +
			"`errors.As(err, &orcaErr)` cannot be used to catch every SDK failure")
	})
}

// -----------------------------------------------------------------------
// 2. Error identity across the request surface
// -----------------------------------------------------------------------

// The TS class hierarchy is uniform no matter which resource method failed.
// The equivalent claim here is that every entry point produces the same error
// type with Method and URL filled in — including the streaming and multipart
// paths, which build their responses on separate code paths from doJSON.
func TestHTTPErrorCarriesMethodAndURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		call       func(*Client) error
		wantMethod string
		wantPath   string
	}{
		{
			name: "GetJSON",
			call: func(c *Client) error {
				var out map[string]any
				return c.GetJSON(context.Background(), "/v1/agents", &out)
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/agents",
		},
		{
			name: "PostJSON",
			call: func(c *Client) error {
				var out map[string]any
				return c.PostJSON(context.Background(), "/v1/agents", map[string]string{"name": "a"}, &out)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/agents",
		},
		{
			name: "PutJSON",
			call: func(c *Client) error {
				var out map[string]any
				return c.PutJSON(context.Background(), "/v1/agents/a1", map[string]string{"name": "a"}, &out)
			},
			wantMethod: http.MethodPut,
			wantPath:   "/v1/agents/a1",
		},
		{
			name: "PatchJSON",
			call: func(c *Client) error {
				var out map[string]any
				return c.PatchJSON(context.Background(), "/v1/agents/a1", map[string]string{"name": "a"}, &out)
			},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/agents/a1",
		},
		{
			name:       "Delete",
			call:       func(c *Client) error { return c.Delete(context.Background(), "/v1/agents/a1") },
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/agents/a1",
		},
		{
			name: "GetStream",
			call: func(c *Client) error {
				return c.GetStream(context.Background(), "/v1/sessions/s1/events/stream", "text/event-stream",
					func(io.Reader) error { return nil })
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/sessions/s1/events/stream",
		},
		{
			name: "PostMultipart",
			call: func(c *Client) error {
				return c.PostMultipart(context.Background(), "/v1/files", MultipartRequest{
					ConfigField: "config",
					Config:      map[string]string{"name": "a"},
				})
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusForbidden, `{"error":"forbidden"}`), nil
			})

			err := tc.call(client)

			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("%s() error = %v (%T), want *HTTPError", tc.name, err, err)
			}
			if httpErr.StatusCode != http.StatusForbidden {
				t.Errorf("StatusCode = %d, want %d", httpErr.StatusCode, http.StatusForbidden)
			}
			if httpErr.Method != tc.wantMethod {
				t.Errorf("Method = %q, want %q", httpErr.Method, tc.wantMethod)
			}
			if want := testBaseURL + tc.wantPath; httpErr.URL != want {
				t.Errorf("URL = %q, want %q", httpErr.URL, want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// 3. Error message rendering
// -----------------------------------------------------------------------

func TestHTTPErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *HTTPError
		want string
	}{
		{
			name: "includes the server payload when there is one",
			err: &HTTPError{
				Method:     http.MethodGet,
				URL:        testBaseURL + "/v1/agents/a1",
				StatusCode: http.StatusNotFound,
				Body:       `{"error":"not found"}`,
			},
			want: `GET https://api.example.test/v1/agents/a1 returned status 404: {"error":"not found"}`,
		},
		{
			name: "omits the trailing separator when the body is empty",
			err: &HTTPError{
				Method:     http.MethodDelete,
				URL:        testBaseURL + "/v1/agents/a1",
				StatusCode: http.StatusNotFound,
			},
			want: "DELETE https://api.example.test/v1/agents/a1 returned status 404",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("surrounding whitespace is trimmed from the captured body", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadRequest, "\n  bad request  \n"), nil
		})

		var out map[string]any
		err := client.GetJSON(context.Background(), "/v1/agents", &out)

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("GetJSON() error = %v, want *HTTPError", err)
		}
		if httpErr.Body != "bad request" {
			t.Errorf("Body = %q, want %q", httpErr.Body, "bad request")
		}
	})
}

// -----------------------------------------------------------------------
// 4. Failure taxonomy — the analogue of the TS `instanceof` chain
// -----------------------------------------------------------------------

// A caller has to tell four kinds of failure apart: the server answered with a
// failing status; the request never got a response; the caller cancelled or
// ran out of time; the response arrived but could not be decoded. The TS SDK
// draws those lines with error classes. Here they are drawn by whether the
// error is an *HTTPError and by what errors.Is finds underneath, so that is
// what this pins.
func TestErrorTaxonomy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		respond     responder
		ctx         func() (context.Context, context.CancelFunc)
		wantHTTP    bool
		wantCause   error
		wantMessage string
	}{
		{
			name: "a failing status is an *HTTPError",
			respond: func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
			},
			wantHTTP: true,
		},
		{
			name:        "a transport failure is not, and wraps its cause",
			respond:     func(*http.Request) (*http.Response, error) { return nil, errTransportFailure },
			wantCause:   errTransportFailure,
			wantMessage: "failed to execute",
		},
		{
			name:        "a cancellation wraps context.Canceled",
			respond:     func(*http.Request) (*http.Response, error) { return nil, context.Canceled },
			wantCause:   context.Canceled,
			wantMessage: "failed to execute",
		},
		{
			name:        "a timeout wraps context.DeadlineExceeded",
			respond:     func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded },
			wantCause:   context.DeadlineExceeded,
			wantMessage: "failed to execute",
		},
		{
			name: "an undecodable success body is a decode failure",
			respond: func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, "not json"), nil
			},
			wantMessage: "failed to decode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, tc.respond)

			var out map[string]any
			err := client.GetJSON(context.Background(), "/v1/agents", &out)
			if err == nil {
				t.Fatal("GetJSON() error = nil, want a failure")
			}

			var httpErr *HTTPError
			if got := errors.As(err, &httpErr); got != tc.wantHTTP {
				t.Errorf("errors.As(*HTTPError) = %v, want %v (error was %v)", got, tc.wantHTTP, err)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantCause)
			}
			if tc.wantMessage != "" && !strings.Contains(err.Error(), tc.wantMessage) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantMessage)
			}
		})
	}

	t.Run("distinguishes an aborted request from a timed-out one by type", func(t *testing.T) {
		t.Parallel()
		t.Skip("not implemented: APIUserAbortError / APIConnectionTimeoutError types with " +
			"default messages — cancellation and deadline are only distinguishable via " +
			"errors.Is on the context sentinels, and neither carries an SDK-owned message")
	})
}

// -----------------------------------------------------------------------
// 5. Request correlation
// -----------------------------------------------------------------------

func TestHTTPErrorRequestID(t *testing.T) {
	t.Parallel()

	t.Run("reads requestID from the request-id response header", func(t *testing.T) {
		t.Parallel()
		t.Skip("not implemented: request-ID capture — *HTTPError keeps no response headers, so " +
			"the request-id a deployment returns cannot be surfaced to the caller or logged " +
			"alongside the failure")
	})

	t.Run("exposes the failing response headers", func(t *testing.T) {
		t.Parallel()
		t.Skip("not implemented: response header access on errors — nothing can read Retry-After, " +
			"rate-limit budgets, or deprecation headers off a failed call")
	})
}
