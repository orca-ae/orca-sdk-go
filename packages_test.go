package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPackagesClientList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/packages/function" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/packages/function")
		}
		_, _ = w.Write([]byte(`[ "pkg-a", "pkg-b" ]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	pkgClient := NewPackagesClient(client)
	packages, err := pkgClient.List(context.Background(), "function")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(packages) != 2 {
		t.Fatalf("packages length = %d, want 2", len(packages))
	}
}

func TestPackagesClientUploadIncludesMetadataAndFile(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/packages/function/pkg-a/v1" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/packages/function/pkg-a/v1")
		}
		fields, files := decodeMultipartRequest(t, r)
		if files["file"] == nil {
			t.Fatalf("missing file part")
		}
		if fields["metadata"] == "" {
			t.Fatal("missing metadata part")
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "pkg.tar")
	if err := os.WriteFile(filePath, []byte("payload"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = NewPackagesClient(client).Upload(context.Background(), "function", "pkg-a", "v1", filePath, PackageMetadata{
		Description: "desc",
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
}

func TestPackagesClientDownloadStreamsPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/packages/function/pkg-a/v1" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/packages/function/pkg-a/v1")
		}
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var downloaded bytes.Buffer
	err = NewPackagesClient(client).Download(context.Background(), "function", "pkg-a", "v1", &downloaded)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if downloaded.String() != "payload" {
		t.Fatalf("payload = %q, want %q", downloaded.String(), "payload")
	}
}

func TestPackagesClientMetadataAndVersionOperations(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/packages/function/pkg%2Fname":
			_, _ = w.Write([]byte(`["v1","v2"]`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/packages/function/pkg%2Fname/v%2F1/metadata":
			_, _ = w.Write([]byte(`{"description":"desc","contact":"owner"}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/packages/function/pkg%2Fname/v%2F1/metadata":
			var metadata PackageMetadata
			if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil || metadata.Description != "updated" {
				t.Fatalf("metadata = %#v, err = %v", metadata, err)
			}
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/packages/function/pkg%2Fname/v%2F1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	base, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := NewPackagesClient(base)
	versions, err := client.ListVersions(context.Background(), "function", "pkg/name")
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions = %#v, err = %v", versions, err)
	}
	metadata, err := client.GetMetadata(context.Background(), "function", "pkg/name", "v/1")
	if err != nil || metadata.Contact != "owner" {
		t.Fatalf("metadata = %#v, err = %v", metadata, err)
	}
	if err := client.UpdateMetadata(context.Background(), "function", "pkg/name", "v/1", PackageMetadata{Description: "updated"}); err != nil {
		t.Fatalf("UpdateMetadata() error = %v", err)
	}
	if err := client.Delete(context.Background(), "function", "pkg/name", "v/1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
