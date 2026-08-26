package orca

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestClientGetJSONAddsBearerToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/connections")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var out []ConnectionConfig
	if err := client.GetJSON(context.Background(), "/connections", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
}

func TestAPIKeyClientGetJSONAddsAPIKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "orca_test-key" {
			t.Fatalf("x-api-key = %q, want %q", got, "orca_test-key")
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("authorization = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewAPIKeyClient(server.URL, "orca_test-key", server.Client())
	if err != nil {
		t.Fatalf("NewAPIKeyClient() error = %v", err)
	}

	var out []ConnectionConfig
	if err := client.GetJSON(context.Background(), "/connections", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
}

func TestClientWithDefaultHeaderDoesNotOverrideRequestHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Anthropic-Beta"); got != "managed-agents-2026-04-01" {
			t.Fatalf("anthropic beta = %q, want %q", got, "managed-agents-2026-04-01")
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("accept = %q, want %q", got, "application/json")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client = client.WithDefaultHeader("Anthropic-Beta", "managed-agents-2026-04-01").WithDefaultHeader("Accept", "text/plain")

	var out []ConnectionConfig
	if err := client.GetJSON(context.Background(), "/connections", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
}

func TestClientReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Delete(context.Background(), "/connections/test")
	if err == nil {
		t.Fatal("Delete() expected error, got nil")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("Delete() error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("Delete() status = %d, want %d", httpErr.StatusCode, http.StatusBadRequest)
	}
	if httpErr.Body != "boom" {
		t.Fatalf("Delete() body = %q, want %q", httpErr.Body, "boom")
	}
}

func TestClientGetToWriterRejectsNilWriter(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://registry.example.com", "test-token", nil)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.GetToWriter(context.Background(), "/connections/test", nil)
	if err == nil {
		t.Fatal("GetToWriter() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "response writer is required") {
		t.Fatalf("GetToWriter() error = %v, want response writer error", err)
	}
}

func TestClientGetToWriterReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "  boom \n", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var downloaded bytes.Buffer
	err = client.GetToWriter(context.Background(), "/connections/test", &downloaded)
	if err == nil {
		t.Fatal("GetToWriter() expected error, got nil")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("GetToWriter() error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("GetToWriter() status = %d, want %d", httpErr.StatusCode, http.StatusBadRequest)
	}
	if httpErr.Body != "boom" {
		t.Fatalf("GetToWriter() body = %q, want %q", httpErr.Body, "boom")
	}
	if downloaded.Len() != 0 {
		t.Fatalf("GetToWriter() wrote %q on error, want empty buffer", downloaded.String())
	}
}

func TestClientGetStreamUsesAcceptAndHandler(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("accept = %q, want text/event-stream", got)
		}
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var streamed bytes.Buffer
	err = client.GetStream(context.Background(), "/sessions/session-1/events/stream", "text/event-stream", func(reader io.Reader) error {
		_, err := streamed.ReadFrom(reader)
		return err
	})
	if err != nil {
		t.Fatalf("GetStream() error = %v", err)
	}
	if streamed.String() != "data: ok\n\n" {
		t.Fatalf("streamed = %q", streamed.String())
	}
}

func TestClientGetJSONRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	payload := oversizedJSONPayload(defaultJSONResponseBodyLimit)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var out map[string]string
	err = client.GetJSON(context.Background(), "/connections", &out)
	if err == nil {
		t.Fatal("GetJSON() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("GetJSON() error = %v, want response limit error", err)
	}
}

func TestClientPostMultipartWithResponseRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("a"), defaultMultipartBodyLimit+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.PostMultipartWithResponse(context.Background(), "/agents/test:trigger", MultipartRequest{})
	if err == nil {
		t.Fatal("PostMultipartWithResponse() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("PostMultipartWithResponse() error = %v, want response limit error", err)
	}
}

func oversizedJSONPayload(limit int) []byte {
	payload := bytes.NewBuffer(make([]byte, 0, limit+16))
	payload.WriteString(`{"data":"`)
	payload.Write(bytes.Repeat([]byte("a"), limit))
	payload.WriteString(`"}`)
	return payload.Bytes()
}

// captureStderr redirects os.Stderr for the duration of fn and returns everything written to it.
// Not safe to run in parallel with other tests that touch os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	os.Stderr = original

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(captured)
}

func TestStripLegacyBaseURLSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		wantStripped   string
		wantMatchedAny bool
	}{
		{name: "v1 registry", path: "/v1/registry", wantStripped: "", wantMatchedAny: true},
		{name: "bare v1", path: "/v1", wantStripped: "", wantMatchedAny: true},
		{name: "api v1 strips the whole suffix, not just v1", path: "/api/v1", wantStripped: "", wantMatchedAny: true},
		{name: "api v1 with leading path", path: "/deploy/api/v1", wantStripped: "/deploy", wantMatchedAny: true},
		{name: "v1 registry with leading path", path: "/deploy/v1/registry", wantStripped: "/deploy", wantMatchedAny: true},
		{name: "trailing slash tolerated", path: "/v1/registry/", wantStripped: "", wantMatchedAny: true},
		{
			name:           "anchored: v1/registry-proxy is left alone",
			path:           "/v1/registry-proxy",
			wantStripped:   "/v1/registry-proxy",
			wantMatchedAny: false,
		},
		{name: "anchored: v1 mid-path is left alone", path: "/v1/registry/extra", wantStripped: "/v1/registry/extra", wantMatchedAny: false},
		{name: "host root has nothing to strip", path: "/", wantStripped: "/", wantMatchedAny: false},
		{name: "empty path has nothing to strip", path: "", wantStripped: "", wantMatchedAny: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stripped, matched := stripLegacyBaseURLSuffix(tt.path)
			if (matched != "") != tt.wantMatchedAny {
				t.Fatalf("stripLegacyBaseURLSuffix(%q) matched = %q, want a match: %v", tt.path, matched, tt.wantMatchedAny)
			}
			if tt.wantMatchedAny && stripped != tt.wantStripped {
				t.Fatalf("stripLegacyBaseURLSuffix(%q) stripped = %q, want %q", tt.path, stripped, tt.wantStripped)
			}
		})
	}
}

func TestNewClientStripsLegacyV1RegistrySuffixAndReachesHostRootPaths(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var client *Client
	stderr := captureStderr(t, func() {
		var err error
		client, err = NewClient(server.URL+"/v1/registry", "test-token", server.Client())
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
	})

	if !strings.Contains(stderr, "/v1/registry") {
		t.Fatalf("stderr = %q, want a warning mentioning the stripped /v1/registry suffix", stderr)
	}

	var out []string
	if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if gotPath != "/v1/agents" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/agents")
	}
}

func TestNewClientStripsWholeApiV1SuffixNotJustTrailingV1(t *testing.T) {
	// A base URL ending in /api/v1 must have the whole suffix stripped, not just the trailing
	// "/v1" segment. Stripping only "/v1" would leave a base ending in "/api", where core calls
	// still resolve via the /api/v1 alias but a request built as "apis/cloud.sn.io/v1/..." would
	// land on ".../api/apis/cloud.sn.io/v1/...", a path in no documentation - a partial failure.
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	stderr := captureStderr(t, func() {
		client, err := NewClient(server.URL+"/api/v1", "test-token", server.Client())
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		var out []string
		if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
	})

	if gotPath != "/v1/agents" {
		t.Fatalf("path = %q, want %q (the whole /api/v1 suffix should be stripped)", gotPath, "/v1/agents")
	}
	if !strings.Contains(stderr, "/api/v1") {
		t.Fatalf("stderr = %q, want a warning mentioning the stripped /api/v1 suffix", stderr)
	}
}

func TestNewClientLeavesLookalikeSuffixAlone(t *testing.T) {
	// /v1/registry-proxy must not be treated as ending in /v1/registry or /v1: the match is
	// anchored to the end of the string, and this path ends in "-proxy".
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	stderr := captureStderr(t, func() {
		client, err := NewClient(server.URL+"/v1/registry-proxy", "test-token", server.Client())
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		var out []string
		if err := client.GetJSON(context.Background(), "agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
	})

	if gotPath != "/v1/registry-proxy/agents" {
		t.Fatalf("path = %q, want %q (the /v1/registry-proxy prefix must not be stripped)", gotPath, "/v1/registry-proxy/agents")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning for a base URL with no legacy suffix", stderr)
	}
}

func TestNewClientHostRootBaseURLWarnsNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	stderr := captureStderr(t, func() {
		client, err := NewClient(server.URL, "test-token", server.Client())
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		var out []string
		if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want no deprecation warning for an already-host-root base URL", stderr)
	}
}

// TestClientPreservesBasePathPrefixAcrossAllTrailingSlashShapes covers a deployment fronted behind
// a path prefix (a reverse proxy mounting the registry at, say, https://host/foo instead of the
// host root - not a documented configuration, but not a rejected one either). Whether that base
// URL is given with or without a trailing slash must not change where requests land.
//
// RFC 3986 relative-reference merging (net/url's ResolveReference, which resolveURL uses) silently
// drops a base URL's last path segment when the base path lacks a trailing slash and the reference
// being resolved has none either - "https://host/foo" merged with "v1/agents" yields
// "https://host/v1/agents", quietly discarding "/foo". newClient's unconditional
// `if !strings.HasSuffix(parsedBaseURL.Path, "/") { parsedBaseURL.Path += "/" }` is what prevents
// that by normalizing every base path to a trailing slash before it is ever resolved against. This
// test exists so that removing or narrowing that normalization fails loudly here instead of
// silently dropping a path prefix for exactly the base shape nobody tests by hand.
func TestClientPreservesBasePathPrefixAcrossAllTrailingSlashShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		makeBase   func(serverURL string) string
		wantSuffix string
	}{
		{name: "host root, no trailing slash", makeBase: func(u string) string { return u }, wantSuffix: "/v1/agents"},
		{name: "host root, trailing slash", makeBase: func(u string) string { return u + "/" }, wantSuffix: "/v1/agents"},
		{name: "path prefix, no trailing slash", makeBase: func(u string) string { return u + "/foo" }, wantSuffix: "/foo/v1/agents"},
		{name: "path prefix, trailing slash", makeBase: func(u string) string { return u + "/foo/" }, wantSuffix: "/foo/v1/agents"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()

			client, err := NewClient(tt.makeBase(server.URL), "test-token", server.Client())
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			var out []string
			if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
			if gotPath != tt.wantSuffix {
				t.Fatalf("path = %q, want %q", gotPath, tt.wantSuffix)
			}
		})
	}
}

func TestClientWithPathPrefixPrependsToEveryPath(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client = client.WithPathPrefix("apis/cloud.sn.io/v1")

	var out []string
	if err := client.GetJSON(context.Background(), "/connections", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if gotPath != "/apis/cloud.sn.io/v1/connections" {
		t.Fatalf("path = %q, want %q", gotPath, "/apis/cloud.sn.io/v1/connections")
	}
}
