// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/memory-stores/*.test.ts.

func TestMemoryStores(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.Create(context.Background(), MemoryStoreNewParams{
			Name:        "project-notes",
			Description: param.String("Notes for project foo"),
			Metadata:    map[string]string{"team": "platform"},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/memory_stores" {
			t.Errorf("request = %s %s, want POST /v1/memory_stores", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"name":"project-notes","description":"Notes for project foo",`+
			`"metadata":{"team":"platform"}}`)
	})

	t.Run("retrieve carries the discriminator and escapes the id", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"m1","type":"memory_store"}`), nil
		})

		store, err := client.MemoryStores.Get(context.Background(), "store/slash")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if store.Type != "memory_store" {
			t.Errorf("type = %q, want %q", store.Type, "memory_store")
		}
		if got, want := transport.Only(t).Path(), "/v1/memory_stores/store%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("update uses POST and keeps null metadata entries", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.Update(context.Background(), "m1", MemoryStoreUpdateParams{
			Name: param.String("renamed"),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"name":"renamed"}`)

		client, transport = newRecordingClient(t, nil)
		_, err = client.MemoryStores.Update(context.Background(), "m1", MemoryStoreUpdateParams{
			Metadata: param.New(map[string]*string{"keep": ptr("yes"), "drop": nil}),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"metadata":{"keep":"yes","drop":null}}`)
	})

	t.Run("list sends its filters and omits the ones the overlay removed", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.List(context.Background(), MemoryStoreListParams{
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

		fields := structFieldNames(MemoryStoreListParams{})
		for _, removed := range []string{"CreatedAtGTE", "CreatedAtLTE", "Provider"} {
			if slices.Contains(fields, removed) {
				t.Errorf("MemoryStoreListParams has %s, want the overlay's removals absent", removed)
			}
		}
	})

	t.Run("delete asks for JSON and returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"memstore_abc","type":"memory_store_deleted"}`), nil
		})

		deleted, err := client.MemoryStores.Delete(context.Background(), "memstore_abc")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", call.Method)
		}
		if got := call.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if deleted.Type != "memory_store_deleted" {
			t.Errorf("tombstone type = %q, want memory_store_deleted", deleted.Type)
		}
	})

	t.Run("archive posts to the archive sub-path", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.MemoryStores.Archive(context.Background(), "m1"); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/memory_stores/m1/archive" {
			t.Errorf("request = %s %s, want POST /v1/memory_stores/m1/archive", call.Method, call.Path())
		}
	})
}

func TestMemories(t *testing.T) {
	t.Parallel()

	t.Run("list escapes path_prefix and decodes prefix markers", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK,
				`{"data":[{"path":"notes/","type":"memory_prefix"}],"next_page":null}`), nil
		})

		page, err := client.MemoryStores.Memories.List(context.Background(), "m1", MemoryListParams{
			Limit:      param.Int(10),
			Page:       param.String("p2"),
			Depth:      param.Int(2),
			PathPrefix: param.String("notes/"),
			View:       param.New(MemoryView("full")),
		})
		if err != nil {
			t.Fatalf("Memories.List() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/memory_stores/m1/memories"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		// The escaping is what keeps a directory prefix from splitting the path.
		if got := call.Query().Get("path_prefix"); got != "notes/" {
			t.Errorf("path_prefix = %q, want it to decode back to notes/", got)
		}
		if got := call.URL.RawQuery; !strings.Contains(got, "path_prefix=notes%2F") {
			t.Errorf("raw query = %q, want it to carry path_prefix=notes%%2F", got)
		}
		for key, want := range map[string]string{"limit": "10", "page": "p2", "depth": "2", "view": "full"} {
			if got := call.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}

		items := page.Items()
		if len(items) != 1 || !items[0].IsPrefix() || items[0].Path != "notes/" {
			t.Errorf("items = %+v, want a single memory_prefix marker", items)
		}
	})

	t.Run("create keeps view in the query, never in the body", func(t *testing.T) {
		t.Parallel()

		// The create schema does not declare a view field. Merging it into the
		// body would send something the server never asked for.
		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.Memories.Create(context.Background(), "m1", MemoryNewParams{
			Body: MemoryNewBody{Path: "notes/todo.md", Content: "Remember this"},
			View: param.New(MemoryView("full")),
		})
		if err != nil {
			t.Fatalf("Memories.Create() error = %v", err)
		}

		call := transport.Only(t)
		assertJSONBody(t, call, `{"path":"notes/todo.md","content":"Remember this"}`)
		if got := call.Query().Get("view"); got != "full" {
			t.Errorf("view = %q, want it in the query", got)
		}

		var body map[string]any
		call.JSONBody(t, &body)
		if _, ok := body["view"]; ok {
			t.Error("request body carries a view field, want it query-only")
		}
	})

	t.Run("retrieve escapes both path parameters", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.Memories.Get(context.Background(), "store/slash", "memory/slash",
			MemoryGetParams{})
		if err != nil {
			t.Fatalf("Memories.Get() error = %v", err)
		}
		want := "/v1/memory_stores/store%2Fslash/memories/memory%2Fslash"
		if got := transport.Only(t).Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("update uses POST with the same body/view split", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.Memories.Update(context.Background(), "m1", "mem1", MemoryUpdateParams{
			Body: MemoryUpdateBody{Content: param.String("Updated")},
			View: param.New(MemoryView("full")),
		})
		if err != nil {
			t.Fatalf("Memories.Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", call.Method)
		}
		assertJSONBody(t, call, `{"content":"Updated"}`)
		if got := call.Query().Get("view"); got != "full" {
			t.Errorf("view = %q, want it in the query", got)
		}
	})

	t.Run("delete escapes the concurrency guard", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"memory_abc","type":"memory_deleted"}`), nil
		})

		deleted, err := client.MemoryStores.Memories.Delete(context.Background(), "m1", "memory_abc",
			MemoryDeleteParams{ExpectedContentSHA256: param.String("sha256:abc")})
		if err != nil {
			t.Fatalf("Memories.Delete() error = %v", err)
		}

		call := transport.Only(t)
		if got := call.Query().Get("expected_content_sha256"); got != "sha256:abc" {
			t.Errorf("expected_content_sha256 = %q, want it to decode back", got)
		}
		if got := call.URL.RawQuery; !strings.Contains(got, "sha256%3Aabc") {
			t.Errorf("raw query = %q, want the colon percent-escaped", got)
		}
		if got := call.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if deleted.Type != "memory_deleted" {
			t.Errorf("tombstone type = %q, want memory_deleted", deleted.Type)
		}
	})
}

func TestMemoryVersions(t *testing.T) {
	t.Parallel()

	t.Run("list sends every supported filter", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.MemoryVersions.List(context.Background(), "m1", MemoryVersionListParams{
			Limit:        param.Int(10),
			Page:         param.String("p2"),
			MemoryID:     param.String("mem1"),
			APIKeyID:     param.String("key1"),
			Operation:    param.New(MemoryVersionOperation("create")),
			CreatedAtGTE: param.String("2026-01-01T00:00:00Z"),
			CreatedAtLTE: param.String("2026-02-01T00:00:00Z"),
			View:         param.New(MemoryView("full")),
		})
		if err != nil {
			t.Fatalf("MemoryVersions.List() error = %v", err)
		}

		call := transport.Only(t)
		// The collection segment is memory_versions, not versions.
		if got, want := call.Path(), "/v1/memory_stores/m1/memory_versions"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		query := call.Query()
		for key, want := range map[string]string{
			"limit": "10", "page": "p2", "memory_id": "mem1", "api_key_id": "key1",
			"operation": "create", "created_at[gte]": "2026-01-01T00:00:00Z",
			"created_at[lte]": "2026-02-01T00:00:00Z", "view": "full",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("session_id is not a supported filter", func(t *testing.T) {
		t.Parallel()

		// It needs local-to-provider ID translation that is not portable across
		// both supported backends, so the overlay removes it.
		if slices.Contains(structFieldNames(MemoryVersionListParams{}), "SessionID") {
			t.Error("MemoryVersionListParams has a SessionID field, want it unsupported")
		}
	})

	t.Run("retrieve escapes both path parameters and keeps view", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.MemoryStores.MemoryVersions.Get(context.Background(), "store/slash", "ver/slash",
			MemoryVersionGetParams{View: param.New(MemoryView("full"))},
			option.WithHeader("X-Trace", "t1"))
		if err != nil {
			t.Fatalf("MemoryVersions.Get() error = %v", err)
		}

		call := transport.Only(t)
		want := "/v1/memory_stores/store%2Fslash/memory_versions/ver%2Fslash"
		if got := call.Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Query().Get("view"); got != "full" {
			t.Errorf("view = %q, want it alongside the caller's own options", got)
		}
		if got := call.Header.Get("X-Trace"); got != "t1" {
			t.Errorf("X-Trace = %q, want the caller's header preserved", got)
		}
	})

	t.Run("redact returns the redacted version", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"v1","redacted_at":"2026-01-01T00:00:00Z"}`), nil
		})

		version, err := client.MemoryStores.MemoryVersions.Redact(context.Background(), "m1", "v1")
		if err != nil {
			t.Fatalf("MemoryVersions.Redact() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/memory_stores/m1/memory_versions/v1/redact" {
			t.Errorf("request = %s %s, want POST .../redact", call.Method, call.Path())
		}
		if version.RedactedAt == nil {
			t.Error("RedactedAt = nil, want the redacted version back")
		}
	})
}
