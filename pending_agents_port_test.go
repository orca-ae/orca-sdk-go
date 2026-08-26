// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript tests/api-resources/agents/{agents,versions}.test.ts.
//
// These tests specify the typed Managed Agents resources. This SDK does not
// implement them yet: managed_agents.go exposes the whole surface as one
// untyped passthrough (Get/Create/Update/Delete/Archive/DoMultipart/
// GetToWriter/GetStream over interface{}), so there is nothing typed to assert
// against. Rather than drop the TypeScript coverage on the floor, each test
// carries the operation table it would assert and then skips, so the repo
// keeps an executable specification of the deferred work.
//
//	go test -v ./... 2>&1 | grep -c "pending: typed Managed Agents"
//
// reports how much of that surface is still outstanding.

// pendingPortOp is one operation in a ported specification table: the exact
// HTTP method, path, query parameters, and request body a future typed Go
// resource must produce. Shared by every pending_*_port_test.go file.
type pendingPortOp struct {
	// Name is the SDK method the TypeScript suite exercises, e.g. "agents.create".
	Name string

	// Method is the HTTP verb. Note that every core resource updates with POST,
	// never PUT, and archives through a POST .../archive sub-path.
	Method string

	// Path is the request path with {placeholders} for path parameters. Every
	// path parameter is percent-escaped, so an ID of "agent/with/slash" is sent
	// as "agent%2Fwith%2Fslash" and never adds path segments.
	Path string

	// Query names the query parameters the operation accepts. Bracket keys such
	// as "created_at[gte]" are literal parameter names, not nested structures.
	// A repeated parameter is marked "name (repeated)".
	Query []string

	// Body is the JSON request body the SDK must send, verbatim, or "" for
	// operations that send no body. Multipart bodies are described in Note.
	Body string

	// Response is the JSON response body the TypeScript suite asserts on, where
	// its exact shape is part of the contract (tombstones, discriminators).
	Response string

	// Note records a contract detail the fields above cannot carry.
	Note string
}

// pendingPortSurface reports every operation in spec as unimplemented. Tests
// call it after t.Skip(pendingManagedAgents), so it runs only once someone
// lifts the skip — at which point it names exactly what is still missing
// instead of failing with a bare compile error.
func pendingPortSurface(t *testing.T, spec []pendingPortOp) {
	t.Helper()
	for _, op := range spec {
		t.Errorf("no typed resource implements %s: %s %s", op.Name, op.Method, op.Path)
	}
}

// pendingAgentsSpec is the wire contract of the typed Agents resource.
//
// The deployment overlay removes agents.delete: an agent is retired by
// archiving it (POST .../archive), and a typed resource must not grow a
// delete method.
var pendingAgentsSpec = []pendingPortOp{
	{
		Name:   "agents.create",
		Method: "POST",
		Path:   "/v1/agents",
		Body:   `{"model":"claude-sonnet-4-6","name":"demo"}`,
		Note:   "model accepts the string shorthand or the object form",
	},
	{
		Name:   "agents.create (all optional fields)",
		Method: "POST",
		Path:   "/v1/agents",
		Body: `{"model":{"id":"claude-opus-4","speed":"fast","effort":{"type":"high"}},` +
			`"name":"full-agent","description":"A fully-specified agent",` +
			`"system":"You are a helpful assistant.","metadata":{"team":"a","project":"b"},` +
			`"mcp_servers":[{"name":"srv","url":"https://mcp.example.com"}],` +
			`"tools":[{"type":"agent_toolset","configs":{"lookup":{"enabled":true,` +
			`"permission_policy":{"type":"always_ask"}}},"default_config":{"enabled":true,` +
			`"permission_policy":{"type":"always_allow","audit_level":"strict"},` +
			`"provider_default":"value"},"provider_specific":{"enabled":true}},` +
			`{"type":"custom","name":"lookup_order","description":"Look up an order",` +
			`"input_schema":{"type":"object"}}],` +
			`"skills":[{"type":"custom","skill_id":"sk_abc","version":"1",` +
			`"provider_config":{"mode":"x"}}]}`,
		Note: "an mcp_servers entry must NOT gain a synthesized `type` field; " +
			"tools are discriminated by `type`, and a custom tool requires " +
			"name, description and input_schema",
	},
	{
		Name:   "agents.retrieve",
		Method: "GET",
		Path:   "/v1/agents/{agentID}",
		Query:  []string{"version"},
		Note: "version is sent only when supplied — an omitted version must not " +
			"appear in the query string at all",
	},
	{
		Name:     "agents.retrieve (response nullability)",
		Method:   "GET",
		Path:     "/v1/agents/{agentID}",
		Response: `{"tools":[],"multiagent":null,"archived_at":null}`,
		Note: "tools is a required array (never undefined); multiagent and " +
			"archived_at are nullable on the wire",
	},
	{
		Name:   "agents.update",
		Method: "POST",
		Path:   "/v1/agents/{agentID}",
		Body:   `{"name":"new name"}`,
		Note: "POST, not PUT. A versionless partial update must not synthesize a " +
			"`version` field into the body",
	},
	{
		Name:   "agents.update (nullable fields and metadata patch)",
		Method: "POST",
		Path:   "/v1/agents/{agentID}",
		Body: `{"version":1,"mcp_servers":null,"tools":null,"skills":null,` +
			`"multiagent":null,"metadata":{"keep":"value","remove":null}}`,
		Note: "explicit nulls clear a field; a null metadata value removes that key. " +
			"description:null is likewise sent as null, not dropped",
	},
	{
		Name:   "agents.list",
		Method: "GET",
		Path:   "/v1/agents",
		Query:  []string{"limit", "page", "include_archived"},
		Response: `{"data":[{"id":"agent_1"}],"has_more":true,"first_id":"agent_1",` +
			`"last_id":"agent_1","next_page":"cursor-p2"}`,
		Note: "page-token pagination: iteration follows next_page until " +
			"has_more is false (two pages => exactly two requests)",
	},
	{
		Name:     "agents.archive",
		Method:   "POST",
		Path:     "/v1/agents/{agentID}/archive",
		Response: `{"archived_at":"2026-01-02T00:00:00Z"}`,
		Note:     "an /archive sub-path, never a `:archive` verb suffix; returns the archived agent",
	},
}

// pendingAgentVersionsSpec is the wire contract of the typed agent versions
// sub-resource. It is read-only: versions are created as a side effect of
// agents.update, never directly.
var pendingAgentVersionsSpec = []pendingPortOp{
	{
		Name:   "agents.versions.list",
		Method: "GET",
		Path:   "/v1/agents/{agentID}/versions",
		Query:  []string{"limit", "page"},
		Note: "page-token pagination over agent snapshots; each item is a full " +
			"agent whose `version` is the snapshot number",
	},
}

func TestPendingAgents(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingAgentsSpec)
}

func TestPendingAgentVersions(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingAgentVersionsSpec)
}
