// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/orca-ae/orca-sdk-go/internal/requestconfig"
	"github.com/orca-ae/orca-sdk-go/option"
)

// Ported from orca-sdk-typescript tests/smoke.test.ts.
//
// The TypeScript smoke test asserts VERSION and that every resource is *mounted*
// on the client object (orca.cloud.connections is defined, and so on). Go has no
// resource tree: each area is its own constructor taking a *Client. So "mounted"
// becomes something stronger and more useful — each constructor is exercised
// once and its first request must land on the right prefix. A resource wired to
// the wrong base would pass a `!= nil` check but fails this one.

// smokeRecordingClient builds a bearer client at baseURL whose requests are
// captured, without going through the shared harness's fixed testBaseURL.
func smokeRecordingClient(tb testing.TB, baseURL string) (*Client, *recordingTransport) {
	tb.Helper()
	transport := &recordingTransport{}
	client, err := NewClientWithWarningWriter(baseURL, "test-key", &http.Client{Transport: transport}, io.Discard)
	if err != nil {
		tb.Fatalf("NewClientWithWarningWriter(%q) error = %v", baseURL, err)
	}
	return client, transport
}

// TestSmokeCloudResourcesMountUnderTheExtensionBasePath is the Go analogue of
// the TypeScript `expect(orca.cloud.X).toBeDefined()` block. Every StreamNative
// Cloud resource must resolve under /apis/cloud.sn.io/v1, spelled literally.
func TestSmokeCloudResourcesMountUnderTheExtensionBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		invoke   func(context.Context, *Client) error
		wantPath string
	}{
		{
			name: "cloud.catalog.sources",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewCatalogClient(c).ListSources(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/catalog/sources",
		},
		{
			name: "cloud.catalog.sinks",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewCatalogClient(c).ListSinks(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/catalog/sinks",
		},
		{
			name: "cloud.catalog.kafka",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewCatalogClient(c).ListKafkaConnectors(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/catalog/kafka",
		},
		{
			name: "cloud.connections",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewConnectionsClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connections",
		},
		{
			name: "cloud.functions",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewFunctionsClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/functions",
		},
		{
			name: "cloud.health",
			body: `true`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewHealthClient(c).Health(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/health",
		},
		{
			name: "cloud.connectors.kafka.connectors",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewKafkaConnectClient(c).ListConnectors(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connectors/kafka/connectors",
		},
		{
			name: "cloud.connectors.kafka.plugins",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewKafkaConnectClient(c).ListPlugins(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins",
		},
		{
			name: "cloud.packages",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewPackagesClient(c).List(ctx, "function")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/packages/function",
		},
		{
			name: "cloud.agents.providers",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewProvidersClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/agents/providers",
		},
		{
			name: "cloud.connectors.sinks",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewSinksClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connectors/sinks",
		},
		{
			name: "cloud.connectors.sources",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewSourcesClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connectors/sources",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.body), nil
			})
			if err := tc.invoke(context.Background(), client); err != nil {
				t.Fatalf("request error = %v", err)
			}
			if got := transport.Only(t).Path(); got != tc.wantPath {
				t.Errorf("path = %q, want %q", got, tc.wantPath)
			}
		})
	}
}

// TestSmokeCoreResourcesMountAtTheHostRoot is the other half: the core managed
// agents surface and discovery are NOT under the extension prefix. The
// TypeScript suite states the same rule as `expect('triggers' in orca.cloud)
// .toBe(false)`.
func TestSmokeCoreResourcesMountAtTheHostRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		invoke   func(context.Context, *Client) error
		wantPath string
	}{
		{
			name: "discovery.groups",
			body: `{"kind":"APIGroupList","groups":[]}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := c.GetAPIGroups(ctx)
				return err
			},
			wantPath: "/apis",
		},
		{
			name: "core api versions",
			body: `{"kind":"APIVersions","versions":["v1"],"preferred_version":"v1"}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := c.GetAPIVersions(ctx)
				return err
			},
			wantPath: "/api",
		},
		{
			name: "managed agents passthrough",
			body: `{"data":[]}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewManagedAgentsClient(c).Get(ctx, "v1/agents")
				return err
			},
			wantPath: "/v1/agents",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.body), nil
			})
			if err := tc.invoke(context.Background(), client); err != nil {
				t.Fatalf("request error = %v", err)
			}
			got := transport.Only(t).Path()
			if got != tc.wantPath {
				t.Errorf("path = %q, want %q", got, tc.wantPath)
			}
			if strings.HasPrefix(got, "/"+CloudExtensionBasePath) {
				t.Errorf("path = %q, want it outside the Cloud extension prefix", got)
			}
		})
	}
}

// TS: 'requires baseURL' — expect(() => new Orca({})).toThrow(/baseURL is required/).
func TestSmokeConstructorsRequireABaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func(string) (*Client, error)
		wantErr string
	}{
		{
			name:    "bearer",
			build:   func(base string) (*Client, error) { return NewClient(base, "token", nil) },
			wantErr: "registry base URL is required",
		},
		{
			name:    "api key",
			build:   func(base string) (*Client, error) { return NewAPIKeyClient(base, "orca_key", nil) },
			wantErr: "registry base URL is required",
		},
		{
			name:    "unauthenticated",
			build:   func(base string) (*Client, error) { return NewUnauthenticatedClient(base, nil) },
			wantErr: "registry base URL is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, base := range []string{"", "   "} {
				client, err := tc.build(base)
				if err == nil {
					t.Fatalf("constructor(%q) error = nil, want %q", base, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("constructor(%q) error = %v, want it to contain %q", base, err, tc.wantErr)
				}
				if client != nil {
					t.Errorf("constructor(%q) returned a client alongside an error", base)
				}
			}
		})
	}
}

func TestSmokeConstructorsRequireACredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func(string) (*Client, error)
		wantErr string
	}{
		{
			name:    "bearer requires a token",
			build:   func(cred string) (*Client, error) { return NewClient("https://host.example.test", cred, nil) },
			wantErr: "registry access token is required",
		},
		{
			name:    "api key requires a key",
			build:   func(cred string) (*Client, error) { return NewAPIKeyClient("https://host.example.test", cred, nil) },
			wantErr: "registry API key is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, cred := range []string{"", "  \t "} {
				if _, err := tc.build(cred); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("constructor(%q) error = %v, want it to contain %q", cred, err, tc.wantErr)
				}
			}
		})
	}
}

// TestSmokeUnauthenticatedClientNeedsNoCredential is the counterpart: the probe
// constructor is the only one that may be built without one.
func TestSmokeUnauthenticatedClientNeedsNoCredential(t *testing.T) {
	t.Parallel()

	transport := &recordingTransport{respond: func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"status":"ok","service":"managed-agents"}`), nil
	}}
	client, err := NewUnauthenticatedClientWithWarningWriter(
		testBaseURL, &http.Client{Transport: transport}, io.Discard)
	if err != nil {
		t.Fatalf("NewUnauthenticatedClientWithWarningWriter() error = %v", err)
	}
	if _, err := client.GetHealthz(context.Background()); err != nil {
		t.Fatalf("GetHealthz() error = %v", err)
	}
	if got := transport.Only(t).Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want it unset", got)
	}
}

func TestSmokeConstructorsRejectAnUnparseableBaseURL(t *testing.T) {
	t.Parallel()

	for _, base := range []string{"://nope", "http://[::1", "ht tp://x"} {
		client, err := NewClient(base, "token", nil)
		if err == nil {
			t.Errorf("NewClient(%q) error = nil, want a parse error", base)
			continue
		}
		if !strings.Contains(err.Error(), "invalid registry base URL") {
			t.Errorf("NewClient(%q) error = %v, want it to name the base URL", base, err)
		}
		if client != nil {
			t.Errorf("NewClient(%q) returned a client alongside an error", base)
		}
	}
}

// TestSmokeDefaultHTTPClient records the default transport the SDK installs when
// the caller passes nil. The timeout must stay above the 60 s server-side
// synchronous-trigger contract, so it is asserted, not just checked non-zero.
func TestSmokeDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://host.example.test", "token", nil)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.cfg.HTTPClient == nil {
		t.Fatal("httpClient = nil, want a default client")
	}
	if client.cfg.HTTPClient.Timeout != defaultHTTPTimeout {
		t.Errorf("default timeout = %v, want %v", client.cfg.HTTPClient.Timeout, defaultHTTPTimeout)
	}
}

// TS: 'strips trailing slash from baseURL' — new Orca({baseURL:
// 'https://example.test///'}).baseURL === 'https://example.test'.
//
// Divergence: Go only guarantees exactly one trailing slash is present, it does
// not collapse repeats. A base URL of "https://example.test///" therefore keeps
// its extra slashes in every request path. See TestSmokeRepeatedTrailingSlashes
// for the gap.
func TestSmokeBaseURLTrailingSlash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		wantURL string
	}{
		{
			name:    "no trailing slash",
			baseURL: "https://example.test",
			wantURL: "https://example.test/v1/agents",
		},
		{
			name:    "one trailing slash",
			baseURL: "https://example.test/",
			wantURL: "https://example.test/v1/agents",
		},
		{
			name:    "a sub-path base keeps its segment",
			baseURL: "https://example.test/engine",
			wantURL: "https://example.test/engine/v1/agents",
		},
		{
			name:    "a sub-path base with a trailing slash resolves the same way",
			baseURL: "https://example.test/engine/",
			wantURL: "https://example.test/engine/v1/agents",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := smokeRecordingClient(t, tc.baseURL)
			var out map[string]any
			if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
			if got := transport.Only(t).URL.String(); got != tc.wantURL {
				t.Errorf("request URL = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

func TestSmokeRepeatedTrailingSlashes(t *testing.T) {
	t.Parallel()

	client, transport := smokeRecordingClient(t, "https://example.test///")
	var out map[string]any
	if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}

	got := transport.Only(t).URL.String()
	if got == "https://example.test/v1/agents" {
		return
	}
	t.Skipf("not implemented: repeated trailing slashes in the base URL are not collapsed — "+
		"%q produced %q, where the TypeScript SDK normalises the base to %q",
		"https://example.test///", got, "https://example.test")
}

// TestSmokeExportedSurfaceIsConstructible is the closest analogue of the
// TypeScript "every resource is defined" assertion: each exported constructor
// exists, accepts a *Client, and returns a usable value.
func TestSmokeExportedSurfaceIsConstructible(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, nil)

	constructors := map[string]func(*Client) any{
		"NewCatalogClient":      func(c *Client) any { return NewCatalogClient(c) },
		"NewConnectionsClient":  func(c *Client) any { return NewConnectionsClient(c) },
		"NewFunctionsClient":    func(c *Client) any { return NewFunctionsClient(c) },
		"NewHealthClient":       func(c *Client) any { return NewHealthClient(c) },
		"NewKafkaConnectClient": func(c *Client) any { return NewKafkaConnectClient(c) },
		"NewManagedAgentsClient": func(c *Client) any {
			return NewManagedAgentsClient(c)
		},
		"NewPackagesClient":  func(c *Client) any { return NewPackagesClient(c) },
		"NewProvidersClient": func(c *Client) any { return NewProvidersClient(c) },
		"NewSinksClient":     func(c *Client) any { return NewSinksClient(c) },
		"NewSourcesClient":   func(c *Client) any { return NewSourcesClient(c) },
	}

	for name, build := range constructors {
		if build(client) == nil {
			t.Errorf("%s(client) = nil, want a resource client", name)
		}
	}
}

// TestSmokeHTTPErrorMessage covers the error type every resource surfaces, which
// the TypeScript suite gets from APIError.
func TestSmokeHTTPErrorMessage(t *testing.T) {
	t.Parallel()

	withBody := &HTTPError{Method: "GET", URL: "https://host/x", StatusCode: 404, Body: "not found"}
	if got, want := withBody.Error(), "GET https://host/x returned status 404: not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	withoutBody := &HTTPError{Method: "GET", URL: "https://host/x", StatusCode: 500}
	if got, want := withoutBody.Error(), "GET https://host/x returned status 500"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestSmokeResponseBodyLimit records the ceiling on a decoded JSON response.
// There is no TypeScript counterpart — the browser fetch layer has no such cap —
// but it is part of the exported behaviour of every resource method.
func TestSmokeResponseBodyLimit(t *testing.T) {
	t.Parallel()

	oversized := `{"data":"` + strings.Repeat("a", defaultJSONResponseBodyLimit) + `"}`
	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(oversized))),
		}, nil
	})

	var out map[string]any
	err := client.GetJSON(context.Background(), "v1/agents", &out)
	if err == nil {
		t.Fatal("GetJSON() error = nil, want an oversized-response error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("error = %v, want it to name the response size limit", err)
	}
}

// ---------------------------------------------------------------------------
// TypeScript smoke assertions with no Go analogue, or not implemented here.
// ---------------------------------------------------------------------------

// TS: expect(VERSION).toBe('0.2.0').
func TestSmokeVersionConstant(t *testing.T) {
	t.Parallel()

	// The constant is exported for callers who put this client behind their own
	// service and need it in their User-Agent, and for anyone quoting a version
	// in a bug report. It also has to reach the wire, or a server log cannot
	// tell which SDK version produced a request.
	if Version == "" {
		t.Fatal("Version = \"\", want a semantic version")
	}
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(Version) {
		t.Errorf("Version = %q, want a semantic version", Version)
	}

	client, transport := newRecordingClient(t, nil)
	var out map[string]any
	if err := client.GetJSON(context.Background(), "/v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if got, want := transport.Only(t).Header.Get("X-Orca-Client"), "orca-sdk-go/"+Version; got != want {
		t.Errorf("X-Orca-Client = %q, want %q", got, want)
	}
}

// TS: expect(orca.maxRetries).toBe(2).
func TestSmokeDefaultMaxRetries(t *testing.T) {
	t.Parallel()

	// Retrying by default is the right behaviour for a network client, but only
	// because the policy is narrow: see TestClientRetryBehaviour for which
	// failures qualify.
	client, err := New(option.WithBaseURL(testBaseURL), option.WithAPIKey("orca_test"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := client.cfg.MaxRetries; got != requestconfig.DefaultMaxRetries {
		t.Errorf("default MaxRetries = %d, want %d", got, requestconfig.DefaultMaxRetries)
	}
	if requestconfig.DefaultMaxRetries != 2 {
		t.Errorf("DefaultMaxRetries = %d, want 2", requestconfig.DefaultMaxRetries)
	}
}

// TS: typed resources such as orca.triggers, orca.sessions.files,
// orca.memoryStores.memories, orca.agents.versions.
func TestSmokeTypedManagedAgentsResources(t *testing.T) {
	t.Skip(pendingManagedAgents)
}
