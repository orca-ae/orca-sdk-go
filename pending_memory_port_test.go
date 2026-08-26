// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript
// tests/api-resources/memory-stores/{memory-stores,memories,memory-versions}.test.ts.
//
// Specifies the typed MemoryStores resource and its two sub-resources, which
// this SDK does not implement yet — see managed_agents.go, an untyped
// passthrough. See pending_agents_port_test.go for the shared table type.

// pendingMemoryStoresSpec is the wire contract of the typed MemoryStores
// resource. Note the collection segment is memory_stores, with an underscore.
var pendingMemoryStoresSpec = []pendingPortOp{
	{
		Name:   "memoryStores.create",
		Method: "POST",
		Path:   "/v1/memory_stores",
		Body: `{"name":"project-notes","description":"Notes for project foo",` +
			`"metadata":{"team":"platform"}}`,
	},
	{
		Name:     "memoryStores.retrieve",
		Method:   "GET",
		Path:     "/v1/memory_stores/{memoryStoreID}",
		Response: `{"type":"memory_store"}`,
		Note:     "the response discriminator is exactly \"memory_store\"; the ID is escaped",
	},
	{
		Name:   "memoryStores.update",
		Method: "POST",
		Path:   "/v1/memory_stores/{memoryStoreID}",
		Body:   `{"name":"renamed"}`,
		Note: "POST, not PUT. A metadata patch keeps null entries: " +
			`{"metadata":{"keep":"yes","drop":null}}`,
	},
	{
		Name:   "memoryStores.list",
		Method: "GET",
		Path:   "/v1/memory_stores",
		Query:  []string{"limit", "include_archived"},
		Note: "page-token pagination. The overlay removes the " +
			"created_at[gte]/created_at[lte] and provider filters",
	},
	{
		Name:     "memoryStores.delete",
		Method:   "DELETE",
		Path:     "/v1/memory_stores/{memoryStoreID}",
		Response: `{"id":"memstore_abc","type":"memory_store_deleted"}`,
		Note:     "sends Accept: application/json and still honours caller headers",
	},
	{
		Name:   "memoryStores.archive",
		Method: "POST",
		Path:   "/v1/memory_stores/{memoryStoreID}/archive",
	},
}

// pendingMemoriesSpec is the wire contract of the typed memories sub-resource.
//
// Its create and update params split into {body, view}: `view` is a QUERY
// parameter and must NOT be merged into the JSON request body.
var pendingMemoriesSpec = []pendingPortOp{
	{
		Name:     "memoryStores.memories.list",
		Method:   "GET",
		Path:     "/v1/memory_stores/{memoryStoreID}/memories",
		Query:    []string{"limit", "page", "depth", "path_prefix", "view"},
		Response: `{"data":[{"path":"notes/","type":"memory_prefix"}],"next_page":null}`,
		Note: "path_prefix is percent-escaped in the query (notes/ => notes%2F). " +
			"List items are a discriminated union: an entry may be a memory or a " +
			"{type:\"memory_prefix\", path} directory marker",
	},
	{
		Name:   "memoryStores.memories.create",
		Method: "POST",
		Path:   "/v1/memory_stores/{memoryStoreID}/memories",
		Query:  []string{"view"},
		Body:   `{"path":"notes/todo.md","content":"Remember this"}`,
		Note: "params are {body, view}: the body is sent verbatim as the JSON " +
			"payload and `view` goes in the query string. `view` must never " +
			"appear inside the request body",
	},
	{
		Name:   "memoryStores.memories.retrieve",
		Method: "GET",
		Path:   "/v1/memory_stores/{memoryStoreID}/memories/{memoryID}",
		Query:  []string{"view"},
		Note:   "both path parameters are escaped: /v1/memory_stores/store%2Fslash/memories/memory%2Fslash",
	},
	{
		Name:   "memoryStores.memories.update",
		Method: "POST",
		Path:   "/v1/memory_stores/{memoryStoreID}/memories/{memoryID}",
		Query:  []string{"view"},
		Body:   `{"content":"Updated"}`,
		Note:   "POST, not PUT; {body, view} split as for create",
	},
	{
		Name:     "memoryStores.memories.delete",
		Method:   "DELETE",
		Path:     "/v1/memory_stores/{memoryStoreID}/memories/{memoryID}",
		Query:    []string{"expected_content_sha256"},
		Response: `{"id":"memory_abc","type":"memory_deleted"}`,
		Note: "expected_content_sha256 is an optimistic-concurrency guard and is " +
			"percent-escaped (sha256:abc => sha256%3Aabc); sends Accept: application/json",
	},
}

// pendingMemoryVersionsSpec is the wire contract of the typed memory versions
// sub-resource. Note the collection segment is memory_versions.
var pendingMemoryVersionsSpec = []pendingPortOp{
	{
		Name:   "memoryStores.memoryVersions.list",
		Method: "GET",
		Path:   "/v1/memory_stores/{memoryStoreID}/memory_versions",
		Query: []string{
			"limit", "page", "memory_id", "api_key_id", "operation",
			"created_at[gte]", "created_at[lte]", "view",
		},
		Note: "the bracket keys are literal parameter names. session_id is NOT " +
			"portable — it needs local-to-provider ID translation and the " +
			"deployment overlay removes it from the list params",
	},
	{
		Name:   "memoryStores.memoryVersions.retrieve",
		Method: "GET",
		Path:   "/v1/memory_stores/{memoryStoreID}/memory_versions/{versionID}",
		Query:  []string{"view"},
		Note:   "both path parameters are escaped; `view` survives alongside caller RequestOptions",
	},
	{
		Name:     "memoryStores.memoryVersions.redact",
		Method:   "POST",
		Path:     "/v1/memory_stores/{memoryStoreID}/memory_versions/{versionID}/redact",
		Response: `{"redacted_at":"2026-01-01T00:00:00Z"}`,
		Note:     "returns the redacted version",
	},
}

func TestPendingMemoryStores(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingMemoryStoresSpec)
}

func TestPendingMemories(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingMemoriesSpec)
}

func TestPendingMemoryVersions(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingMemoryVersionsSpec)
}
