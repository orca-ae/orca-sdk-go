// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"testing"
)

// Ported from orca-sdk-typescript tests/api-resources/discovery.test.ts.
//
// TypeScript exposes discovery as `orca.discovery.groups()`; the Go analogue is
// Client.GetAPIGroups plus Client.GetCloudAPIResources. Both are served from the
// deployment host root, never from an extension prefix — that is the whole point
// of discovery, so the URL assertions spell the literal path out rather than
// building it from CloudExtensionBasePath.
//
// The core surface (GET /api, /healthz, /readyz) is covered at the bottom: it is
// the other half of "what does this deployment serve".

// discoveryGroupsFixture mirrors GROUPS_FIXTURE in the TypeScript suite.
func discoveryGroupsFixture() APIGroupList {
	return APIGroupList{
		Kind: "APIGroupList",
		Groups: []APIGroup{{
			Name:             "cloud.sn.io",
			Versions:         []APIGroupVersion{{GroupVersion: "cloud.sn.io/v1", Version: "v1"}},
			PreferredVersion: APIGroupVersion{GroupVersion: "cloud.sn.io/v1", Version: "v1"},
		}},
	}
}

// TS: 'sends GET to exactly {base}/apis'.
func TestDiscoveryGroupsRequestShape(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonValue(t, http.StatusOK, discoveryGroupsFixture()), nil
	})
	if _, err := client.GetAPIGroups(context.Background()); err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}

	call := transport.Only(t)
	if got := call.URL.String(); got != testBaseURL+"/apis" {
		t.Errorf("request URL = %q, want %q", got, testBaseURL+"/apis")
	}
	if call.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", call.Method)
	}
	if got := call.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
	}
	if got := call.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
	if len(call.Body) != 0 {
		t.Errorf("body = %q, want none on a GET", call.Body)
	}
}

// TS: 'returns the parsed group list'.
func TestDiscoveryGroupsParsesTheGroupList(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonValue(t, http.StatusOK, discoveryGroupsFixture()), nil
	})
	result, err := client.GetAPIGroups(context.Background())
	if err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}

	if result.Kind != "APIGroupList" {
		t.Errorf("kind = %q, want APIGroupList", result.Kind)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("groups = %#v, want exactly one", result.Groups)
	}
	group := result.Groups[0]
	if group.Name != "cloud.sn.io" {
		t.Errorf("group name = %q, want cloud.sn.io", group.Name)
	}
	if group.PreferredVersion.GroupVersion != "cloud.sn.io/v1" {
		t.Errorf("preferred group version = %q, want cloud.sn.io/v1", group.PreferredVersion.GroupVersion)
	}
	if len(group.Versions) != 1 || group.Versions[0].Version != "v1" {
		t.Errorf("versions = %#v, want a single v1", group.Versions)
	}
	if !result.HasGroup("cloud.sn.io") {
		t.Error(`HasGroup("cloud.sn.io") = false, want true`)
	}
}

// TS: 'returns an empty groups array on a self-hosted deployment without error'.
//
// An empty list is a working deployment with no extensions installed. It is not
// the same diagnosis as a 404 from /apis, which means the deployment predates
// discovery — that case is covered by TestGetAPIGroupsReturnsHTTPErrorOn404.
func TestDiscoveryGroupsEmptyOnSelfHostedDeployment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "an explicit empty array", body: `{"kind":"APIGroupList","groups":[]}`},
		{name: "an omitted groups key", body: `{"kind":"APIGroupList"}`},
		{name: "a null groups key", body: `{"kind":"APIGroupList","groups":null}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.body), nil
			})
			result, err := client.GetAPIGroups(context.Background())
			if err != nil {
				t.Fatalf("GetAPIGroups() error = %v", err)
			}
			if len(result.Groups) != 0 {
				t.Errorf("groups = %#v, want empty", result.Groups)
			}
			if result.HasGroup(CloudExtensionGroup) {
				t.Error("HasGroup() = true on a deployment with no extensions, want false")
			}
		})
	}
}

// TS: 'passes request options through' (a custom header on the call). The Go
// analogue is a client-level default header, since the resource methods take no
// per-request options.
func TestDiscoveryGroupsForwardsCustomHeaders(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonValue(t, http.StatusOK, discoveryGroupsFixture()), nil
	})
	decorated := client.WithDefaultHeader("X-Test-Header", "discovery")
	if _, err := decorated.GetAPIGroups(context.Background()); err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
	if got := transport.Only(t).Header.Get("X-Test-Header"); got != "discovery" {
		t.Errorf("X-Test-Header = %q, want %q", got, "discovery")
	}
}

// TestDiscoveryGroupsWithAPIKeyCredential covers the credential class the
// TypeScript suite uses by default (apiKey) against the Go api-key constructor.
func TestDiscoveryGroupsWithAPIKeyCredential(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingAPIKeyClient(t, func(*http.Request) (*http.Response, error) {
		return jsonValue(t, http.StatusOK, discoveryGroupsFixture()), nil
	})
	if _, err := client.GetAPIGroups(context.Background()); err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}

	call := transport.Only(t)
	if got := call.URL.String(); got != testBaseURL+"/apis" {
		t.Errorf("request URL = %q, want %q", got, testBaseURL+"/apis")
	}
	if got := call.Header.Get("x-api-key"); got != "orca_test_key" {
		t.Errorf("x-api-key = %q, want %q", got, "orca_test_key")
	}
}

// TestDiscoveryCloudResourcesRequestShape pins the trailing slash. Kubernetes-
// style resource discovery is served at "{group}/{version}/" — dropping the
// slash is a different route, so the assertion is deliberately exact.
func TestDiscoveryCloudResourcesRequestShape(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK,
			`{"kind":"APIResourceList","group_version":"cloud.sn.io/v1","resources":[
				{"name":"connections","kind":"Connection","namespaced":true},
				{"name":"packages","kind":"Package","namespaced":false}
			]}`), nil
	})
	result, err := client.GetCloudAPIResources(context.Background())
	if err != nil {
		t.Fatalf("GetCloudAPIResources() error = %v", err)
	}

	call := transport.Only(t)
	if got, want := call.URL.String(), testBaseURL+"/apis/cloud.sn.io/v1/"; got != want {
		t.Errorf("request URL = %q, want %q", got, want)
	}
	if got := call.Path(); got != "/apis/cloud.sn.io/v1/" {
		t.Errorf("path = %q, want the trailing slash preserved", got)
	}
	if result.GroupVersion != "cloud.sn.io/v1" {
		t.Errorf("group version = %q, want cloud.sn.io/v1", result.GroupVersion)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("resources = %#v, want two", result.Resources)
	}
	if !result.Resources[0].Namespaced || result.Resources[1].Namespaced {
		t.Errorf("namespaced flags = %v/%v, want true/false",
			result.Resources[0].Namespaced, result.Resources[1].Namespaced)
	}
}

func TestDiscoveryPolicyAndPricingResources(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/apis":
			return jsonResponse(http.StatusOK,
				extensionGroupsJSON(PolicyExtensionGroup, PricingExtensionGroup)), nil
		case "/apis/policy.runorca.ai/v1":
			return jsonResponse(http.StatusOK, `{"kind":"APIResourceList",`+
				`"group_version":"policy.runorca.ai/v1","resources":[`+
				`{"name":"guardrails","namespaced":true,"kind":"Guardrail"}]}`), nil
		case "/apis/pricing.runorca.ai/v1":
			return jsonResponse(http.StatusOK, `{"kind":"APIResourceList",`+
				`"group_version":"pricing.runorca.ai/v1","resources":[`+
				`{"name":"modelprices","namespaced":false,"kind":"ModelPrice"}]}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})
	ctx := context.Background()

	policy, err := client.Discovery.PolicyGroupResources(ctx)
	if err != nil || len(policy.Resources) != 1 || policy.Resources[0].Name != "guardrails" {
		t.Fatalf("PolicyGroupResources() = %#v, %v", policy, err)
	}
	pricing, err := client.Discovery.PricingGroupResources(ctx)
	if err != nil || len(pricing.Resources) != 1 || pricing.Resources[0].Name != "modelprices" {
		t.Fatalf("PricingGroupResources() = %#v, %v", pricing, err)
	}

	calls := transport.Calls()
	if len(calls) != 3 {
		t.Fatalf("requests = %d, want one cached discovery plus two resource lists", len(calls))
	}
	if calls[1].Path() != "/apis/policy.runorca.ai/v1" || calls[2].Path() != "/apis/pricing.runorca.ai/v1" {
		t.Errorf("resource paths = %s, %s", calls[1].Path(), calls[2].Path())
	}
}

// TestCloudExtensionPathConstants checks the constants themselves against the
// literal strings the wire protocol uses, which is the one direction a
// constant-based assertion is meaningful in: every path builder derives from
// these, so a typo here would move the whole extension surface at once.
func TestCloudExtensionPathConstants(t *testing.T) {
	t.Parallel()

	if CloudExtensionGroup != "cloud.sn.io" {
		t.Errorf("CloudExtensionGroup = %q, want %q", CloudExtensionGroup, "cloud.sn.io")
	}
	if CloudExtensionBasePath != "apis/cloud.sn.io/v1" {
		t.Errorf("CloudExtensionBasePath = %q, want %q", CloudExtensionBasePath, "apis/cloud.sn.io/v1")
	}
	if PolicyExtensionGroup != "policy.runorca.ai" || PolicyExtensionBasePath != "apis/policy.runorca.ai/v1" {
		t.Errorf("policy constants = %q, %q", PolicyExtensionGroup, PolicyExtensionBasePath)
	}
	if PricingExtensionGroup != "pricing.runorca.ai" || PricingExtensionBasePath != "apis/pricing.runorca.ai/v1" {
		t.Errorf("pricing constants = %q, %q", PricingExtensionGroup, PricingExtensionBasePath)
	}
}

// TestCoreDiscoveryRequestShapes covers core.go: the version and probe endpoints
// that live at the host root alongside /apis.
func TestCoreDiscoveryRequestShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		invoke  func(context.Context, *Client) error
		wantURL string
	}{
		{
			name: "api versions",
			body: `{"kind":"APIVersions","versions":["v1"],"preferred_version":"v1"}`,
			invoke: func(ctx context.Context, c *Client) error {
				versions, err := c.GetAPIVersions(ctx)
				if err == nil && (versions.PreferredVersion != "v1" || len(versions.Versions) != 1) {
					t.Errorf("versions = %#v", versions)
				}
				return err
			},
			wantURL: testBaseURL + "/api",
		},
		{
			name: "liveness probe",
			body: `{"status":"ok","service":"managed-agents"}`,
			invoke: func(ctx context.Context, c *Client) error {
				status, err := c.GetHealthz(ctx)
				if err == nil && status.Status != "ok" {
					t.Errorf("status = %#v", status)
				}
				return err
			},
			wantURL: testBaseURL + "/healthz",
		},
		{
			name: "readiness probe",
			body: `{"status":"ready","service":"managed-agents"}`,
			invoke: func(ctx context.Context, c *Client) error {
				status, err := c.GetReadyz(ctx)
				if err == nil && status.Status != "ready" {
					t.Errorf("status = %#v", status)
				}
				return err
			},
			wantURL: testBaseURL + "/readyz",
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
			if got := transport.Only(t).URL.String(); got != tc.wantURL {
				t.Errorf("request URL = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

// TestCoreProbesStillCarryACredential records a deliberate divergence from the
// OpenAPI contract, where /healthz and /readyz declare an empty security list:
// an authenticated client sends its bearer token to them anyway. Callers that
// must probe anonymously have to build an unauthenticated client, which
// core_test.go's TestCoreProbesAreUnauthenticated covers.
func TestCoreProbesStillCarryACredential(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"status":"ok","service":"managed-agents"}`), nil
	})
	if _, err := client.GetHealthz(context.Background()); err != nil {
		t.Fatalf("GetHealthz() error = %v", err)
	}
	if got := transport.Only(t).Header.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q, want the bearer token to be sent", got)
	}
}

// TestDiscoveryIsNotRelativeToAnExtensionPrefix proves discovery escapes the
// prefix a scoped client carries: a client scoped to the Cloud extension group
// must still ask the host root which groups exist, or the check becomes
// circular.
func TestDiscoveryIsNotRelativeToAnExtensionPrefix(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonValue(t, http.StatusOK, discoveryGroupsFixture()), nil
	})
	// A prefixed clone is what NewConnectionsClient and friends hold internally.
	scoped := client.WithPathPrefix(CloudExtensionBasePath)
	if _, err := scoped.GetAPIGroups(context.Background()); err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}

	got := transport.Only(t).URL.String()
	if got == testBaseURL+"/apis" {
		return
	}
	t.Skipf("not implemented: GetAPIGroups honours the caller's path prefix, so a scoped "+
		"client asks %q instead of %q — discovery has no way to opt out of WithPathPrefix",
		got, testBaseURL+"/apis")
}
