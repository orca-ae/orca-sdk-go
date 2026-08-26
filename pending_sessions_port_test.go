// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript
// tests/api-resources/sessions/{sessions,events,files,resources,threads}.test.ts.
//
// Specifies the typed Sessions resource and its four sub-resources, which this
// SDK does not implement yet — see managed_agents.go, an untyped passthrough.
// Each table is the spec a typed resource must satisfy; the tests skip until
// one exists. See pending_agents_port_test.go for the shared table type.

// pendingSessionsSpec is the wire contract of the typed Sessions resource.
var pendingSessionsSpec = []pendingPortOp{
	{
		Name:   "sessions.create",
		Method: "POST",
		Path:   "/v1/sessions",
		Body:   `{"agent":"agent_abc","environment_id":"env_123"}`,
		Note: "the collection is /v1/sessions — a session is never created under " +
			"/v1/agents/{id}/sessions. `agent` accepts the string shorthand",
	},
	{
		Name:   "sessions.create (all optional fields)",
		Method: "POST",
		Path:   "/v1/sessions",
		Body: `{"agent":{"id":"agent_abc","type":"agent","version":2},` +
			`"environment_id":"env_123","vault_ids":["vault_1","vault_2"],` +
			`"title":"My Session","metadata":{"key":"value"},` +
			`"resources":[{"type":"file","file_id":"file_abc"}]}`,
		Note: "an object agent reference requires the `type` discriminator",
	},
	{
		Name:   "sessions.create (agent_id compatibility form)",
		Method: "POST",
		Path:   "/v1/sessions",
		Body:   `{"agent_id":"agent_abc","environment_id":"env_123"}`,
		Note:   "the agent_id form must be sent as-is; no `agent` field is synthesized",
	},
	{
		Name:   "sessions.create (agent overrides and initial events)",
		Method: "POST",
		Path:   "/v1/sessions",
		Body: `{"agent":{"type":"agent_with_overrides","id":"agent_abc","version":2,` +
			`"system":"Override system prompt"},"environment_id":"env_123",` +
			`"initial_events":[{"type":"user.message","content":[{"type":"text","text":"Start"}]}]}`,
		Note: "overrides and initial events are forwarded verbatim — no backend-specific rewriting",
	},
	{
		Name:     "sessions.retrieve",
		Method:   "GET",
		Path:     "/v1/sessions/{sessionID}",
		Response: `{"outcome_evaluations":[],"resources":[],"archived_at":null}`,
		Note: "outcome_evaluations and resources are required arrays (never " +
			"undefined); archived_at is nullable",
	},
	{
		Name:   "sessions.update",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}",
		Body:   `{"title":"Updated Title"}`,
		Note:   "POST, not PUT",
	},
	{
		Name:   "sessions.update (agent overrides, vaults, metadata)",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}",
		Body: `{"agent":{"tools":[{"type":"custom","name":"lookup_order",` +
			`"description":"Look up an order","input_schema":{"type":"object"}}],` +
			`"mcp_servers":[{"name":"orders","type":"url","url":"https://mcp.example.com"}]},` +
			`"vault_ids":["vault_new"],"metadata":{"updated":"true"}}`,
		Note: "update accepts agent OVERRIDES only; environment_id, resources and " +
			"an `agent` reference were removed from UpdateSessionRequest and must " +
			"not be accepted. metadata:{k:null} removes a key; metadata:null clears all",
	},
	{
		Name:   "sessions.list",
		Method: "GET",
		Path:   "/v1/sessions",
		Query:  []string{"agent_id", "limit", "include_archived"},
		Response: `{"data":[{"id":"session_1"}],"has_more":true,"first_id":"session_1",` +
			`"last_id":"session_1","next_page":"cursor-p2"}`,
		Note: "page-token pagination via next_page. The overlay removes the " +
			"agent_version, created_at[gt|gte|lt|lte], deployment_id, " +
			"memory_store_id, order, statuses and provider filters",
	},
	{
		Name:     "sessions.delete",
		Method:   "DELETE",
		Path:     "/v1/sessions/{sessionID}",
		Response: `{"id":"session_xyz","type":"session_deleted"}`,
	},
	{
		Name:     "sessions.archive",
		Method:   "POST",
		Path:     "/v1/sessions/{sessionID}/archive",
		Response: `{"archived_at":"2026-01-02T00:00:00Z"}`,
		Note:     "an /archive sub-path; returns the archived session",
	},
}

// pendingSessionEventsSpec is the wire contract of the typed session events
// sub-resource, including its SSE stream.
var pendingSessionEventsSpec = []pendingPortOp{
	{
		Name:   "sessions.events.list",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/events",
		Query: []string{
			"limit", "order", "page",
			"created_at[gt]", "created_at[gte]", "created_at[lt]", "created_at[lte]",
			"types (repeated)", "subpath",
		},
		Note: "the bracket keys are literal parameter names — created_at[gte] is " +
			"one parameter called \"created_at[gte]\", not a nested object. " +
			"`types` repeats: types=user.message&types=agent.message",
	},
	{
		Name:     "sessions.events.send",
		Method:   "POST",
		Path:     "/v1/sessions/{sessionID}/events",
		Body:     `{"events":[{"type":"user.message","content":[{"type":"text","text":"Hello!"}]}]}`,
		Response: `{"data":[{"id":"evt_1","type":"user.message"}]}`,
		Note: "several events go in one call: " +
			`{"events":[{"type":"user.message",...},{"type":"user.interrupt"}]}`,
	},
	{
		Name:   "sessions.events.stream",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/events/stream",
		Query:  []string{"from_cursor", "subpath", "event_deltas (repeated)"},
		Note: "text/event-stream of `data: {json}\\n\\n` frames, yielded one event " +
			"at a time and robust to arbitrary chunk boundaries (the TS suite " +
			"feeds the parser one byte at a time). Passing only RequestOptions " +
			"must produce an EMPTY query string",
	},
}

// pendingSessionFilesSpec is the wire contract of the typed session files
// sub-resource. Session files use ID-cursor pagination, not page tokens.
var pendingSessionFilesSpec = []pendingPortOp{
	{
		Name:     "sessions.files.list",
		Method:   "GET",
		Path:     "/v1/sessions/{sessionID}/files",
		Query:    []string{"limit", "after_id", "before_id"},
		Response: `{"data":[{"id":"file_1"}],"has_more":true,"first_id":"file_1","last_id":"file_1"}`,
		Note: "ID-cursor pagination: the next page sends after_id=<last_id>. When " +
			"the caller started with before_id, iteration keeps that direction — " +
			"the next page sends before_id=<first_id> and NO after_id",
	},
	{
		Name:   "sessions.files.retrieve",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/files/{fileID}",
		Note:   "both path parameters are escaped: /v1/sessions/session%2Fslash/files/file%2Fslash",
	},
	{
		Name:   "sessions.files.download",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/files/{fileID}/content",
		Note:   "sends Accept: application/octet-stream and returns the raw response body",
	},
	{
		Name:     "sessions.files.delete",
		Method:   "DELETE",
		Path:     "/v1/sessions/{sessionID}/files/{fileID}",
		Response: `{"id":"file_abc","type":"file_deleted"}`,
	},
	{
		Name:   "sessions.files portability",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/files",
		Note: "every session-file operation stays under /v1/sessions — no " +
			"/v1/registry prefix and no /apis/ extension prefix on either backend",
	},
}

// pendingSessionResourcesSpec is the wire contract of the typed session
// resources sub-resource.
var pendingSessionResourcesSpec = []pendingPortOp{
	{
		Name:   "sessions.resources.list",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/resources",
		Query:  []string{"limit", "page"},
	},
	{
		Name:   "sessions.resources.add",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}/resources",
		Body:   `{"type":"file","file_id":"file_doc"}`,
		Note:   "the resource object is the body itself, not wrapped in a `resource` key",
	},
	{
		Name:   "sessions.resources.retrieve",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/resources/{resourceID}",
	},
	{
		Name:   "sessions.resources.update",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}/resources/{resourceID}",
		Body:   `{"authorization_token":"updated-token"}`,
		Note:   "POST, not PUT — updateResource is POST per the OpenAPI spec",
	},
	{
		Name:     "sessions.resources.delete",
		Method:   "DELETE",
		Path:     "/v1/sessions/{sessionID}/resources/{resourceID}",
		Response: `{"id":"resource_abc","type":"session_resource_deleted"}`,
	},
}

// pendingSessionThreadsSpec is the wire contract of the typed session threads
// sub-resource and its nested events sub-resource.
//
// Threads are spawned by the coordinator, so there is deliberately no create
// method: a typed resource must not invent one.
var pendingSessionThreadsSpec = []pendingPortOp{
	{
		Name:   "sessions.threads.retrieve",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/threads/{threadID}",
		Response: `{"type":"session_thread","status":"idle","stats":null,"usage":null,` +
			`"parent_thread_id":null,"archived_at":null}`,
		Note: "stats and usage are nullable on a thread (unlike a session, where " +
			"they are objects); both path parameters are escaped",
	},
	{
		Name:   "sessions.threads.list",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/threads",
		Query:  []string{"limit", "page"},
		Note:   "limit and page ONLY — no include_archived and no order",
	},
	{
		Name:   "sessions.threads.archive",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}/threads/{threadID}/archive",
	},
	{
		Name:   "sessions.threads.events.list",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/threads/{threadID}/events",
		Query:  []string{"limit", "page"},
		Note:   "the supported pagination params only — `order` is not accepted here",
	},
	{
		Name:   "sessions.threads.events.stream",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/threads/{threadID}/stream",
		Query:  []string{"from_cursor", "event_deltas (repeated)"},
		Note: "the thread stream hangs off /threads/{id}/stream — NOT " +
			"/threads/{id}/events/stream, which is the one place the thread and " +
			"session stream paths diverge. Passing only RequestOptions must " +
			"produce an EMPTY query string. Unlike the session stream, the thread " +
			"stream takes no `subpath` parameter",
	},
}

func TestPendingSessions(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionsSpec)
}

func TestPendingSessionEvents(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionEventsSpec)
}

func TestPendingSessionFiles(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionFilesSpec)
}

func TestPendingSessionResources(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionResourcesSpec)
}

func TestPendingSessionThreads(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionThreadsSpec)
}
