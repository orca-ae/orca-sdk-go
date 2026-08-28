// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/internal/requestconfig"
	"github.com/orca-ae/orca-sdk-go/option"
)

// This file provides the shared harness for the test suite ported from
// orca-sdk-typescript. Those tests inject a fake `fetch` and assert on the
// captured {url, init} tuple; the Go equivalent is a recording
// http.RoundTripper, so no mock server and no node are needed in CI.
//
// The copied Go tests in this package use httptest.NewServer instead and assert
// literal paths. Both are kept: they cover the same code from different angles.

// pendingManagedAgents is the single skip reason for ported tests that specify
// the typed Managed Agents resources, which this SDK does not implement yet.
// Counting these reports the outstanding surface:
//
//	go test -v ./... 2>&1 | grep -c "pending: typed Managed Agents"
const pendingManagedAgents = "pending: typed Managed Agents resources"

// capturedCall is one request observed by recordingTransport.
type capturedCall struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte
}

// Path returns the request path, still percent-encoded, so tests can assert on
// escaping (e.g. a connection named "events/primary").
func (c capturedCall) Path() string { return c.URL.EscapedPath() }

// Query returns the parsed query string.
func (c capturedCall) Query() url.Values { return c.URL.Query() }

// JSONBody decodes the captured request body into out.
func (c capturedCall) JSONBody(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(c.Body, out); err != nil {
		t.Fatalf("decoding request body %q: %v", string(c.Body), err)
	}
}

// responder produces the response for a request. Returning a nil response means
// "fall through to an empty 200 JSON object".
type responder func(*http.Request) (*http.Response, error)

// recordingTransport captures every request and serves a scripted response.
type recordingTransport struct {
	respond responder
	calls   []capturedCall
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		// Restore the body so a responder can inspect it too.
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	t.calls = append(t.calls, capturedCall{
		Method: req.Method,
		URL:    req.URL,
		Header: req.Header.Clone(),
		Body:   body,
	})

	if t.respond != nil {
		res, err := t.respond(req)
		if err != nil || res != nil {
			return res, err
		}
	}
	return jsonResponse(http.StatusOK, `{}`), nil
}

// Calls returns the requests captured so far.
func (t *recordingTransport) Calls() []capturedCall { return t.calls }

// Reset discards captured requests, e.g. after a discovery pre-warm.
func (t *recordingTransport) Reset() { t.calls = nil }

// Last returns the most recent captured request, failing if there is none.
func (t *recordingTransport) Last(tb testing.TB) capturedCall {
	tb.Helper()
	if len(t.calls) == 0 {
		tb.Fatal("no requests were captured")
	}
	return t.calls[len(t.calls)-1]
}

// Only returns the single captured request, failing if there is not exactly one.
func (t *recordingTransport) Only(tb testing.TB) capturedCall {
	tb.Helper()
	if len(t.calls) != 1 {
		tb.Fatalf("captured %d requests, want exactly 1", len(t.calls))
	}
	return t.calls[0]
}

// jsonResponse builds an application/json response with the given body.
func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// jsonValue marshals v and builds a JSON response from it.
func jsonValue(tb testing.TB, status int, v any) *http.Response {
	tb.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		tb.Fatalf("marshaling response: %v", err)
	}
	return jsonResponse(status, string(encoded))
}

// testBaseURL is the deployment host root used by the ported tests. It matches
// the TypeScript suite's baseURL so path assertions can be compared directly.
const testBaseURL = "https://api.example.test"

// newRecordingClient returns a bearer-authenticated client whose requests are
// captured. Pass nil to have every request answered with an empty 200 `{}`.
func newRecordingClient(tb testing.TB, respond responder) (*Client, *recordingTransport) {
	tb.Helper()
	transport := &recordingTransport{respond: respond}
	client, err := NewClientWithWarningWriter(
		testBaseURL, "test-key", &http.Client{Transport: transport}, io.Discard,
	)
	if err != nil {
		tb.Fatalf("NewClientWithWarningWriter() error = %v", err)
	}
	return client, transport
}

// newRecordingAPIKeyClient is newRecordingClient for the x-api-key credential.
func newRecordingAPIKeyClient(tb testing.TB, respond responder) (*Client, *recordingTransport) {
	tb.Helper()
	transport := &recordingTransport{respond: respond}
	client, err := NewAPIKeyClientWithWarningWriter(
		testBaseURL, "orca_test_key", &http.Client{Transport: transport}, io.Discard,
	)
	if err != nil {
		tb.Fatalf("NewAPIKeyClientWithWarningWriter() error = %v", err)
	}
	return client, transport
}

// newRecordingClientWith is newRecordingClient with extra options applied.
//
// Note that the recording clients above deliberately have retries disabled, via
// the legacy constructors they are built with. That is what lets the assertions
// throughout this suite treat "requests captured" as "calls the SDK made" - a
// retrying client would make a 500 look like three requests and quietly break
// every call-count assertion. Retry behaviour has its own harness below.
func newRecordingClientWith(tb testing.TB, respond responder, opts ...option.RequestOption) (*Client, *recordingTransport) {
	tb.Helper()
	transport := &recordingTransport{respond: respond}
	base := []option.RequestOption{
		option.WithBaseURL(testBaseURL),
		option.WithAuthToken("test-key"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
		option.WithWarningWriter(io.Discard),
		option.WithMaxRetries(0),
	}
	client, err := New(append(base, opts...)...)
	if err != nil {
		tb.Fatalf("New() error = %v", err)
	}
	return client, transport
}

// newRetryingClient returns a client that retries, with the backoff replaced by
// a recorder. Tests can then assert the delay the policy chose without paying
// it: a suite that actually slept through exponential backoff would take
// minutes, and would be the first thing anyone deleted.
func newRetryingClient(
	tb testing.TB,
	maxRetries int,
	respond responder,
) (*Client, *recordingTransport, *[]time.Duration) {
	tb.Helper()
	transport := &recordingTransport{respond: respond}
	delays := &[]time.Duration{}

	client, err := New(
		option.WithBaseURL(testBaseURL),
		option.WithAuthToken("test-key"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
		option.WithWarningWriter(io.Discard),
		option.WithMaxRetries(maxRetries),
		withRecordedSleep(delays),
	)
	if err != nil {
		tb.Fatalf("New() error = %v", err)
	}
	return client, transport, delays
}

// withRecordedSleep replaces the retry backoff with a recorder. It is a
// test-only option, which is why it lives here rather than in the option
// package: nothing outside this suite should be able to disable the backoff.
func withRecordedSleep(into *[]time.Duration) option.RequestOption {
	var mu sync.Mutex
	return requestconfig.RequestOptionFunc(func(cfg *requestconfig.RequestConfig) error {
		cfg.Sleep = func(_ context.Context, d time.Duration) error {
			mu.Lock()
			defer mu.Unlock()
			*into = append(*into, d)
			return nil
		}
		return nil
	})
}
