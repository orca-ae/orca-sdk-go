// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAPIGroupsParsesAdvertisedGroups(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind": "APIGroupList",
			"groups": [
				{
					"name": "cloud.sn.io",
					"versions": [{"group_version": "cloud.sn.io/v1", "version": "v1"}],
					"preferred_version": {"group_version": "cloud.sn.io/v1", "version": "v1"}
				}
			]
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	groups, err := client.GetAPIGroups(context.Background())
	if err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
	if gotPath != "/apis" {
		t.Fatalf("path = %q, want %q", gotPath, "/apis")
	}
	if len(groups.Groups) != 1 || groups.Groups[0].Name != CloudExtensionGroup {
		t.Fatalf("groups = %#v, want one %q group", groups.Groups, CloudExtensionGroup)
	}
	if groups.Groups[0].PreferredVersion.GroupVersion != "cloud.sn.io/v1" {
		t.Fatalf("preferred group version = %q, want %q", groups.Groups[0].PreferredVersion.GroupVersion, "cloud.sn.io/v1")
	}
	if !groups.HasGroup(CloudExtensionGroup) {
		t.Fatalf("HasGroup(%q) = false, want true", CloudExtensionGroup)
	}
	if groups.HasGroup("unknown.group") {
		t.Fatal("HasGroup(\"unknown.group\") = true, want false")
	}
}

func TestGetAPIGroupsEmptyListIsNotAnError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	groups, err := client.GetAPIGroups(context.Background())
	if err != nil {
		t.Fatalf("GetAPIGroups() error = %v", err)
	}
	if len(groups.Groups) != 0 {
		t.Fatalf("groups = %#v, want empty", groups.Groups)
	}
	if groups.HasGroup(CloudExtensionGroup) {
		t.Fatal("HasGroup() = true for an empty list, want false")
	}
}

func TestGetAPIGroupsReturnsHTTPErrorOn404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.GetAPIGroups(context.Background())
	if err == nil {
		t.Fatal("GetAPIGroups() expected error, got nil")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("GetAPIGroups() error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("GetAPIGroups() status = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
	}
}

func TestAPIGroupListHasGroupIsNilSafe(t *testing.T) {
	t.Parallel()

	var list *APIGroupList
	if list.HasGroup(CloudExtensionGroup) {
		t.Fatal("HasGroup() on a nil *APIGroupList = true, want false")
	}
}

func TestGetCloudAPIResources(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIResourceList","group_version":"cloud.sn.io/v1","resources":[{"name":"connections","kind":"Connection","namespaced":true}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resources, err := client.GetCloudAPIResources(context.Background())
	if err != nil {
		t.Fatalf("GetCloudAPIResources() error = %v", err)
	}
	if gotPath != "/apis/cloud.sn.io/v1/" {
		t.Fatalf("path = %q, want %q", gotPath, "/apis/cloud.sn.io/v1/")
	}
	if resources.GroupVersion != "cloud.sn.io/v1" || len(resources.Resources) != 1 {
		t.Fatalf("resources = %#v", resources)
	}
	if resources.Resources[0].Name != "connections" || !resources.Resources[0].Namespaced {
		t.Fatalf("resource = %#v", resources.Resources[0])
	}
}
