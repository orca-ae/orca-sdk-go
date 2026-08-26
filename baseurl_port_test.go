// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
)

// Ported from orca-sdk-typescript tests/client.test.ts, describe('legacy baseURL shim').
//
// The base URL is the deployment host root. A legacy /api/v1, /v1/registry, or
// bare /v1 suffix is accepted and stripped, with a deprecation warning written
// to the caller's diagnostic stream.

// newShimClient builds a bearer client at baseURL, capturing both the requests
// it makes and anything written to its warning stream.
func newShimClient(tb testing.TB, baseURL string) (*Client, *recordingTransport, *bytes.Buffer) {
	tb.Helper()
	transport := &recordingTransport{}
	var warnings bytes.Buffer
	client, err := NewClientWithWarningWriter(
		baseURL, "test-key", &http.Client{Transport: transport}, &warnings,
	)
	if err != nil {
		tb.Fatalf("NewClientWithWarningWriter(%q) error = %v", baseURL, err)
	}
	return client, transport, &warnings
}

// requestURL issues a GET and returns the absolute URL that was actually sent.
func requestURL(tb testing.TB, client *Client, transport *recordingTransport, path string) string {
	tb.Helper()
	var out map[string]any
	if err := client.GetJSON(context.Background(), path, &out); err != nil {
		tb.Fatalf("GetJSON(%q) error = %v", path, err)
	}
	return transport.Last(tb).URL.String()
}

func TestLegacyBaseURLShim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		path     string
		wantURL  string
		wantWarn string // substring the deprecation warning must contain; "" means no warning
	}{
		{
			name:     "strips a /v1/registry suffix and still reaches {host}/v1/agents",
			baseURL:  "https://host.example.test/v1/registry",
			path:     "/v1/agents",
			wantURL:  "https://host.example.test/v1/agents",
			wantWarn: "/v1/registry",
		},
		{
			name:     "strips a bare /v1 suffix and still reaches {host}/v1/agents",
			baseURL:  "https://host.example.test/v1",
			path:     "/v1/agents",
			wantURL:  "https://host.example.test/v1/agents",
			wantWarn: "/v1",
		},
		{
			// Stripping only "/v1" would leave ".../api", where core paths still
			// resolve via the documented alias but extension paths do not.
			name:     `strips the whole "/api/v1" suffix, not just the trailing /v1`,
			baseURL:  "https://host.example.test/api/v1",
			path:     "/v1/agents",
			wantURL:  "https://host.example.test/v1/agents",
			wantWarn: "/api/v1",
		},
		{
			name:     "from a stripped /api/v1 base, an extension path also resolves",
			baseURL:  "https://host.example.test/api/v1",
			path:     "/apis/cloud.sn.io/v1/connections",
			wantURL:  "https://host.example.test/apis/cloud.sn.io/v1/connections",
			wantWarn: "/api/v1",
		},
		{
			name:     "does not warn or alter a baseURL with no legacy suffix",
			baseURL:  "https://host.example.test",
			path:     "/v1/agents",
			wantURL:  "https://host.example.test/v1/agents",
			wantWarn: "",
		},
		{
			name:     `does not strip a baseURL that merely contains "v1" mid-path`,
			baseURL:  "https://host.example.test/v1/extra",
			path:     "/v1/agents",
			wantURL:  "https://host.example.test/v1/extra/v1/agents",
			wantWarn: "",
		},
		{
			name:     "does not mistake a version-named hostname for a legacy path",
			baseURL:  "http://v1",
			path:     "/v1/agents",
			wantURL:  "http://v1/v1/agents",
			wantWarn: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport, warnings := newShimClient(t, tc.baseURL)
			if got := requestURL(t, client, transport, tc.path); got != tc.wantURL {
				t.Errorf("request URL = %q, want %q", got, tc.wantURL)
			}

			warning := warnings.String()
			switch {
			case tc.wantWarn == "" && warning != "":
				t.Errorf("unexpected deprecation warning: %q", warning)
			case tc.wantWarn != "" && !strings.Contains(warning, tc.wantWarn):
				t.Errorf("warning = %q, want it to name %q", warning, tc.wantWarn)
			}
		})
	}
}

func TestLegacyBaseURLShimWarnsOnce(t *testing.T) {
	t.Parallel()

	client, transport, warnings := newShimClient(t, "https://host.example.test/v1/registry")
	requestURL(t, client, transport, "/v1/agents")
	requestURL(t, client, transport, "/v1/sessions")

	if got := strings.Count(warnings.String(), "warning:"); got != 1 {
		t.Errorf("deprecation warnings = %d, want exactly 1", got)
	}
}
