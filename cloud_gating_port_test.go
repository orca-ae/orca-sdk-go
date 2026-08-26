// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// Ported from orca-sdk-typescript tests/client.test.ts,
// describe('ensureExtensionAvailable'), and from the shared
// assertCloudExtensionUnavailable helper in
// tests/api-resources/cloud/helpers.ts.
//
// The TypeScript client gates every cloud extension call: before a resource
// request goes out it resolves GET /apis once, caches the result, and raises a
// distinct ExtensionNotAvailableError when the cloud.sn.io group is not
// advertised - so a deployment without the extension produces a clear
// diagnosis instead of a bare 404 from an unrelated path.
//
// This SDK ships the discovery half of that (Client.GetAPIGroups and
// APIGroupList.HasGroup) but not the gate: no cached probe, no distinct error
// type, and no automatic check inside the resource clients. The gate lives one
// layer up, in orca-cli's requireCloudExtension, which was not copied here.
//
// The specification is kept below so the gap is enumerable rather than
// forgotten: `go test -v -run Gating ./... | grep 'not implemented'` lists it.

// cloudGatingUnimplemented is the single skip reason for the ported gating
// specifications. The sibling cloud_*_port_test.go files share it for their
// per-resource "gates X before its API request" cases.
const cloudGatingUnimplemented = "not implemented: cloud extension gating - " +
	"there is no ensureExtensionAvailable, no ExtensionNotAvailableError, and no automatic " +
	"GET /apis probe before a cloud resource request; a caller wanting the TypeScript SDK's " +
	"behaviour must call Client.GetAPIGroups and APIGroupList.HasGroup itself"

// TestCloudExtensionGatingSpecification is the ported describe block. Each case
// names the capability it needs, so the skip output reads as a work list.
func TestCloudExtensionGatingSpecification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason string
	}{
		{
			name: "throws ExtensionNotAvailableError rather than a surfaced 404 when groups is empty",
			reason: "not implemented: a distinct extension-not-available error; an empty group list is " +
				"reported only as HasGroup() == false, which no resource client consults",
		},
		{
			name: "throws ExtensionNotAvailableError when groups exist but do not include the target",
			reason: "not implemented: a distinct extension-not-available error; a non-matching group list " +
				"is reported only as HasGroup() == false, which no resource client consults",
		},
		{
			name: "throws ExtensionNotAvailableError, not NotFoundError, when /apis itself 404s",
			reason: "not implemented: GetAPIGroups surfaces the raw *HTTPError with StatusCode 404, so a " +
				"deployment predating discovery is indistinguishable from a missing extension",
		},
		{
			name: "logs a version warning when /apis 404s (pre-discovery deployment)",
			reason: "not implemented: there is no diagnostic channel for discovery; the warning writer " +
				"passed at construction only carries the legacy base-URL deprecation notice",
		},
		{
			name:   "does not warn when /apis returns 200 with an empty groups array",
			reason: "not implemented: the discovery warning this case bounds does not exist to be suppressed",
		},
		{
			name: "caches a successful discovery result across repeated calls",
			reason: "not implemented: discovery result caching; every GetAPIGroups call issues a fresh " +
				"GET /apis",
		},
		{
			name:   "shares one in-flight discovery request across concurrent callers",
			reason: "not implemented: discovery single-flight; concurrent GetAPIGroups calls each issue a request",
		},
		{
			name:   "does not cache a failed discovery attempt so the next call retries",
			reason: "not implemented: discovery result caching, so there is no negative-caching rule to assert",
		},
		{
			name: "gates every cloud resource request behind discovery",
			reason: "not implemented: resource clients issue their request immediately; " +
				"nothing probes GET /apis first",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Skip(tc.reason)
		})
	}
}

// TestCloudExtensionDiscoveryPrimitives covers the part of the specification
// this SDK does implement: the GET /apis probe a caller-side gate is built out
// of. These are the three ensureExtensionAvailable cases that survive the
// translation, expressed against the primitives rather than against a gate.
func TestCloudExtensionDiscoveryPrimitives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		status    int
		available bool
		wantErr   bool
	}{
		{
			name:      "reports available when GET /apis advertises the group",
			body:      `{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io","versions":[]}]}`,
			available: true,
		},
		{
			name:      "reports unavailable, and not an error, when groups is empty",
			body:      `{"kind":"APIGroupList","groups":[]}`,
			available: false,
		},
		{
			name:      "reports unavailable when groups exist but do not include the target",
			body:      `{"kind":"APIGroupList","groups":[{"name":"some.other.group","versions":[]}]}`,
			available: false,
		},
		{
			name:    "surfaces the transport error when /apis itself 404s",
			body:    `{}`,
			status:  http.StatusNotFound,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(status, tc.body), nil
			})

			groups, err := client.GetAPIGroups(context.Background())
			if tc.wantErr {
				var httpErr *HTTPError
				if !errors.As(err, &httpErr) {
					t.Fatalf("error = %v (%T), want an *HTTPError", err, err)
				}
				// Recorded, not endorsed: the TypeScript contract says this
				// must not reach the caller as a 404. See the skipped case above.
				if httpErr.StatusCode != http.StatusNotFound {
					t.Errorf("status = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetAPIGroups() error = %v", err)
			}
			if got := groups.HasGroup("cloud.sn.io"); got != tc.available {
				t.Errorf("HasGroup(%q) = %t, want %t", "cloud.sn.io", got, tc.available)
			}
			if got := transport.Only(t).Path(); got != "/apis" {
				t.Errorf("path = %q, want /apis at the host root", got)
			}
		})
	}
}

// TestCloudExtensionDiscoveryIsNotCached records the concrete shape of the
// caching gap: three probes, three requests. The TypeScript client answers the
// second and third from its cache.
func TestCloudExtensionDiscoveryIsNotCached(t *testing.T) {
	t.Parallel()
	t.Skip("not implemented: discovery result caching; each GetAPIGroups call issues its own GET /apis")
}
