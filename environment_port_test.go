// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"slices"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/{environments,files}.test.ts.

func TestEnvironments(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Environments.Create(context.Background(), EnvironmentNewParams{
			Name: "production",
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/environments" {
			t.Errorf("request = %s %s, want POST /v1/environments", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"name":"production"}`)
	})

	t.Run("create with config, metadata and scope", func(t *testing.T) {
		t.Parallel()

		// Packages, networking, image and target live under config. The overlay
		// removes the flattened top-level forms.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.Create(context.Background(), EnvironmentNewParams{
			Name:        "staging",
			Description: param.String("Staging environment"),
			Config: EnvironmentConfig{
				Type:     param.String("cloud"),
				Packages: param.New(EnvironmentPackages{Type: param.String("packages"), Pip: param.New([]string{"requests"})}),
				Networking: param.New(EnvironmentNetworking{
					Type:                 "limited",
					AllowedHosts:         param.New([]string{"api.example.com"}),
					AllowPackageManagers: param.Bool(true),
				}),
			},
			Metadata: map[string]string{"team": "platform", "env": "staging"},
			Scope:    param.New(EnvironmentScopeOrganization),
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"name":"staging","description":"Staging environment",`+
			`"config":{"type":"cloud","packages":{"type":"packages","pip":["requests"]},`+
			`"networking":{"type":"limited","allowed_hosts":["api.example.com"],`+
			`"allow_package_managers":true}},`+
			`"metadata":{"team":"platform","env":"staging"},"scope":"organization"}`)
	})

	t.Run("config needs no discriminator and preserves explicit nulls", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.Create(context.Background(), EnvironmentNewParams{
			Name: "minimal-config",
			Config: EnvironmentConfig{
				Packages:   param.New(EnvironmentPackages{Apt: param.New([]string{"curl"}), Pip: param.Null[[]string]()}),
				Networking: param.Null[EnvironmentNetworking](),
			},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t),
			`{"name":"minimal-config","config":{"packages":{"apt":["curl"],"pip":null},"networking":null}}`)
	})

	t.Run("retrieve escapes the id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"e1","scope":"organization","archived_at":null}`), nil
		})
		environment, err := client.Environments.Get(context.Background(), "env/slash")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if environment.Scope != EnvironmentScopeOrganization {
			t.Errorf("Scope = %q, want organization", environment.Scope)
		}
		if environment.ArchivedAt != nil {
			t.Errorf("ArchivedAt = %v, want nil", environment.ArchivedAt)
		}
		if got, want := transport.Only(t).Path(), "/v1/environments/env%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("update sends only the supplied keys", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.Update(context.Background(), "e1", EnvironmentUpdateParams{
			Name:        param.String("renamed"),
			Description: param.String("Updated description"),
			Config:      EnvironmentConfig{Type: param.String("self_hosted")},
			Metadata:    param.New(map[string]*string{"updated": ptr("true")}),
			Scope:       param.New(EnvironmentScopeAccount),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"name":"renamed","description":"Updated description",`+
			`"config":{"type":"self_hosted"},"metadata":{"updated":"true"},"scope":"account"}`)
	})

	t.Run("an omitted field is absent, not null", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.Update(context.Background(), "e1", EnvironmentUpdateParams{
			Name: param.String("renamed"),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		var body map[string]any
		transport.Only(t).JSONBody(t, &body)
		if _, ok := body["description"]; ok {
			t.Error("body carries description, want an omitted field left out entirely")
		}
	})

	t.Run("limited networking accepts null for every list and flag", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.Update(context.Background(), "e1", EnvironmentUpdateParams{
			Config: EnvironmentConfig{
				Packages: param.Null[EnvironmentPackages](),
				Networking: param.New(EnvironmentNetworking{
					Type:                 "limited",
					AllowedHosts:         param.Null[[]string](),
					AllowMCPServers:      param.Null[bool](),
					AllowPackageManagers: param.Null[bool](),
				}),
			},
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"config":{"packages":null,"networking":{"type":"limited",`+
			`"allowed_hosts":null,"allow_mcp_servers":null,"allow_package_managers":null}}}`)
	})

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Environments.List(context.Background(), EnvironmentListParams{
			Limit:           param.Int(10),
			IncludeArchived: param.Bool(true),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		query := transport.Only(t).Query()
		if query.Get("limit") != "10" || query.Get("include_archived") != "true" {
			t.Errorf("query = %v, want limit=10 and include_archived=true", query)
		}
		if slices.Contains(structFieldNames(EnvironmentListParams{}), "Provider") {
			t.Error("EnvironmentListParams has a Provider field, want the overlay's removal honoured")
		}
	})

	t.Run("delete and archive", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"env_abc123","type":"environment_deleted",`+
				`"archived_at":"2026-01-02T00:00:00Z"}`), nil
		})
		deleted, err := client.Environments.Delete(context.Background(), "env_abc123")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.Type != "environment_deleted" {
			t.Errorf("tombstone type = %q, want environment_deleted", deleted.Type)
		}
		if got := transport.Last(t).Method; got != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", got)
		}

		environment, err := client.Environments.Archive(context.Background(), "env_abc123")
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if got, want := transport.Last(t).Path(), "/v1/environments/env_abc123/archive"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if environment.ArchivedAt == nil {
			t.Error("ArchivedAt = nil, want the archived environment back")
		}
	})
}

// multipartParts decodes a captured multipart body into field name to parts.
func multipartParts(t *testing.T, call capturedCall) map[string][]*multipart.Part {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(call.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parsing Content-Type %q: %v", call.Header.Get("Content-Type"), err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("media type = %q, want multipart/form-data", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(call.Body), params["boundary"])
	parts := map[string][]*multipart.Part{}
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		parts[part.FormName()] = append(parts[part.FormName()], part)
	}
	return parts
}

func TestFiles(t *testing.T) {
	t.Parallel()

	t.Run("upload sends the file under the file field with its own MIME type", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"file_1","filename":"notes.txt"}`), nil
		})

		file, err := client.Files.Upload(context.Background(), FileUploadParams{
			Filename:    "notes.txt",
			ContentType: "text/plain",
			Content:     []byte("hello"),
		})
		if err != nil {
			t.Fatalf("Upload() error = %v", err)
		}
		if file.ID != "file_1" {
			t.Errorf("id = %q, want file_1", file.ID)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/files" {
			t.Errorf("request = %s %s, want POST /v1/files", call.Method, call.Path())
		}

		parts := multipartParts(t, call)
		if len(parts["file"]) != 1 {
			t.Fatalf("file parts = %d, want 1", len(parts["file"]))
		}
		if got := parts["file"][0].Header.Get("Content-Type"); got != "text/plain" {
			t.Errorf("part Content-Type = %q, want text/plain", got)
		}
		// The part already carries the content type; a second declaration is
		// one more thing that can disagree with the bytes.
		if _, ok := parts["content_type"]; ok {
			t.Error("body carries a content_type field, want the part's own type to be the only one")
		}
	})

	t.Run("retrieve escapes the id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Files.Get(context.Background(), "file/with/slash"); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got, want := transport.Only(t).Path(), "/v1/files/file%2Fwith%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("download asks for octet-stream and keeps caller headers", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, "raw-bytes"), nil
		})

		var buf bytes.Buffer
		err := client.Files.Download(context.Background(), "f1", &buf,
			option.WithHeader("X-Trace", "t1"))
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/files/f1/content"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Header.Get("Accept"); got != "application/octet-stream" {
			t.Errorf("Accept = %q, want application/octet-stream", got)
		}
		if got := call.Header.Get("X-Trace"); got != "t1" {
			t.Errorf("X-Trace = %q, want the caller's header preserved", got)
		}
		if buf.String() != "raw-bytes" {
			t.Errorf("body = %q, want raw-bytes", buf.String())
		}
	})

	t.Run("list uses ID cursors and omits the removed filters", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Files.List(context.Background(), FileListParams{
			Limit:   param.Int(10),
			AfterID: param.String("file_1"),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got := transport.Only(t).Query().Get("after_id"); got != "file_1" {
			t.Errorf("after_id = %q, want file_1", got)
		}
		fields := structFieldNames(FileListParams{})
		for _, removed := range []string{"ScopeID", "Provider"} {
			if slices.Contains(fields, removed) {
				t.Errorf("FileListParams has %s, want the overlay's removal honoured", removed)
			}
		}
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"file_abc123","type":"file_deleted"}`), nil
		})
		deleted, err := client.Files.Delete(context.Background(), "file_abc123")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.Type != "file_deleted" {
			t.Errorf("tombstone type = %q, want file_deleted", deleted.Type)
		}
	})
}
