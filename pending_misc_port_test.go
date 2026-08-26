// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript
// tests/api-resources/{environments,files}.test.ts,
// tests/api-resources/skills/{skills,versions}.test.ts,
// tests/api-resources/triggers/triggers.test.ts,
// tests/managed-agents-trigger-contract.test.ts and tests/lib/session.test.ts.
//
// Specifies the remaining typed core resources plus the session() ergonomic
// handle, none of which this SDK implements yet — see managed_agents.go, an
// untyped passthrough. See pending_agents_port_test.go for the shared table type.

// pendingEnvironmentsSpec is the wire contract of the typed Environments
// resource.
var pendingEnvironmentsSpec = []pendingPortOp{
	{
		Name:   "environments.create",
		Method: "POST",
		Path:   "/v1/environments",
		Body:   `{"name":"production"}`,
	},
	{
		Name:   "environments.create (all optional fields)",
		Method: "POST",
		Path:   "/v1/environments",
		Body: `{"name":"staging","description":"Staging environment",` +
			`"config":{"type":"cloud","packages":{"type":"packages","pip":["requests"]},` +
			`"networking":{"type":"limited","allowed_hosts":["api.example.com"],` +
			`"allow_package_managers":true}},` +
			`"metadata":{"team":"platform","env":"staging"},"scope":"organization"}`,
		Note: "packages/networking/image/target live under `config`; the overlay " +
			"removes the flattened top-level forms",
	},
	{
		Name:   "environments.create (config without discriminators, nullable values)",
		Method: "POST",
		Path:   "/v1/environments",
		Body:   `{"name":"minimal-config","config":{"packages":{"apt":["curl"],"pip":null},"networking":null}}`,
		Note:   "config sub-objects need no `type` discriminator and preserve explicit nulls",
	},
	{
		Name:     "environments.retrieve",
		Method:   "GET",
		Path:     "/v1/environments/{environmentID}",
		Response: `{"scope":"organization","archived_at":null}`,
		Note:     "the ID is escaped",
	},
	{
		Name:   "environments.update",
		Method: "POST",
		Path:   "/v1/environments/{environmentID}",
		Body: `{"name":"renamed","description":"Updated description",` +
			`"config":{"type":"self_hosted"},"metadata":{"updated":"true"},"scope":"account"}`,
		Note: "POST, not PUT. Partial updates send only the supplied keys — an " +
			"omitted description must be absent from the body, not null",
	},
	{
		Name:   "environments.update (nullable config and metadata patch)",
		Method: "POST",
		Path:   "/v1/environments/{environmentID}",
		Body: `{"config":{"packages":null,"networking":{"type":"limited",` +
			`"allowed_hosts":null,"allow_mcp_servers":null,"allow_package_managers":null}}}`,
		Note: "limited networking accepts null for every list/flag. Separately, " +
			`{"metadata":{"keep":"yes","obsolete":null}} removes the obsolete key`,
	},
	{
		Name:   "environments.list",
		Method: "GET",
		Path:   "/v1/environments",
		Query:  []string{"limit", "include_archived"},
		Note:   "page-token pagination; the overlay removes the provider filter",
	},
	{
		Name:     "environments.delete",
		Method:   "DELETE",
		Path:     "/v1/environments/{environmentID}",
		Response: `{"id":"env_abc123","type":"environment_deleted"}`,
	},
	{
		Name:     "environments.archive",
		Method:   "POST",
		Path:     "/v1/environments/{environmentID}/archive",
		Response: `{"archived_at":"2026-01-02T00:00:00Z"}`,
	},
}

// pendingFilesSpec is the wire contract of the typed Files resource. Files use
// ID-cursor pagination, not page tokens.
var pendingFilesSpec = []pendingPortOp{
	{
		Name:   "files.upload",
		Method: "POST",
		Path:   "/v1/files",
		Note: "multipart/form-data with the file under the field name \"file\". The " +
			"uploaded file's own MIME type carries the content type — the SDK must " +
			"NOT add a separate `content_type` form field",
	},
	{
		Name:   "files.retrieve",
		Method: "GET",
		Path:   "/v1/files/{fileID}",
		Note:   "the ID is escaped (file/with/slash => file%2Fwith%2Fslash)",
	},
	{
		Name:   "files.download",
		Method: "GET",
		Path:   "/v1/files/{fileID}/content",
		Note: "sends Accept: application/octet-stream while preserving any caller " +
			"headers, and returns the raw response body",
	},
	{
		Name:     "files.list",
		Method:   "GET",
		Path:     "/v1/files",
		Query:    []string{"limit", "after_id", "before_id"},
		Response: `{"data":[{"id":"file_1"}],"has_more":true,"first_id":"file_1","last_id":"file_1"}`,
		Note: "ID-cursor pagination: the next page sends after_id=<last_id>; a " +
			"before_id-anchored walk keeps its direction, sending " +
			"before_id=<first_id> and NO after_id. The overlay removes the " +
			"scope_id and provider filters",
	},
	{
		Name:     "files.delete",
		Method:   "DELETE",
		Path:     "/v1/files/{fileID}",
		Response: `{"id":"file_abc123","type":"file_deleted"}`,
	},
}

// pendingSkillsSpec is the wire contract of the typed Skills resource.
var pendingSkillsSpec = []pendingPortOp{
	{
		Name:   "skills.create",
		Method: "POST",
		Path:   "/v1/skills",
		Note: "multipart/form-data. Every uploaded file repeats under the field " +
			"name \"files[]\" (literally, with brackets); the optional display " +
			"title is a plain form field named display_title",
	},
	{
		Name:   "skills.retrieve",
		Method: "GET",
		Path:   "/v1/skills/{skillID}",
		Note:   "the ID is escaped",
	},
	{
		Name:   "skills.list",
		Method: "GET",
		Path:   "/v1/skills",
		Query:  []string{"limit"},
		Note: "page-token pagination. The overlay removes the source, " +
			"include_archived and provider filters — limit is the only portable one",
	},
	{
		Name:     "skills.delete",
		Method:   "DELETE",
		Path:     "/v1/skills/{skillID}",
		Response: `{"id":"skill_abc123","type":"skill_deleted"}`,
	},
}

// pendingSkillVersionsSpec is the wire contract of the typed skill versions
// sub-resource. A version is addressed by its version string, not an opaque ID.
var pendingSkillVersionsSpec = []pendingPortOp{
	{
		Name:   "skills.versions.create",
		Method: "POST",
		Path:   "/v1/skills/{skillID}/versions",
		Note:   "multipart/form-data with files under the \"files[]\" field; the skill ID is escaped",
	},
	{
		Name:   "skills.versions.list",
		Method: "GET",
		Path:   "/v1/skills/{skillID}/versions",
		Query:  []string{"limit", "page"},
	},
	{
		Name:   "skills.versions.retrieve",
		Method: "GET",
		Path:   "/v1/skills/{skillID}/versions/{version}",
		Note: "the version string is a path segment and is escaped like any other " +
			"(v1/v2 => v1%2Fv2)",
	},
	{
		Name:     "skills.versions.delete",
		Method:   "DELETE",
		Path:     "/v1/skills/{skillID}/versions/{version}",
		Response: `{"id":"version_abc123","type":"skill_version_deleted"}`,
	},
}

// pendingTriggersSpec is the wire contract of the typed Triggers resource.
//
// Triggers are CORE (orca.triggers), not a cloud extension: the deployment
// overlay promoted them out of the cloud group, and `cloud.triggers` must not
// exist. A typed Go resource must mount them under /v1/triggers, never under
// /apis/cloud.sn.io/v1.
var pendingTriggersSpec = []pendingPortOp{
	{
		Name:   "triggers.create",
		Method: "POST",
		Path:   "/v1/triggers",
		Body: `{"name":"orders","agent":{"type":"agent","id":"agt_abc123","version":2},` +
			`"session_mode":"SESSION_PER_KEY","source":{"type":"kafka","connection":"orders",` +
			`"topics":["orders.created"],"subscription_name":"orca-sdk",` +
			`"consumer_additional_config":{"auto.offset.reset":"earliest"},` +
			`"input_schema_configs":{"value":{"subject":"orders-value","type":"AVRO","version":1}}},` +
			`"session":{"environment_id":"env_abc123","vault_ids":[]},"replicas":3}`,
		Note: "managed-deployment messaging fields go to the core path unchanged. " +
			"source.type is cron|kafka|pulsar; session_mode is SESSION_PER_EVENT|" +
			"SESSION_PER_TOPIC|SESSION_PER_KEY|SHARED. A kafka source requires " +
			"exactly one of topics or topic_pattern, and a cron source does not " +
			"support SESSION_PER_TOPIC",
	},
	{
		Name:   "triggers.list",
		Method: "GET",
		Path:   "/v1/triggers",
		Query:  []string{"agent_id", "limit", "page"},
		Note:   "the overlay removes include_archived from the trigger list params",
	},
	{
		Name:   "triggers.retrieve",
		Method: "GET",
		Path:   "/v1/triggers/{triggerID}",
		Note:   "the ID is escaped: trigger/with/slash => /v1/triggers/trigger%2Fwith%2Fslash",
	},
	{
		Name:   "triggers.update",
		Method: "POST",
		Path:   "/v1/triggers/{triggerID}",
		Body: `{"session_mode":"SHARED","source":{"type":"pulsar","connection":"events",` +
			`"topic_pattern":"persistent://public/default/orders-.*"},` +
			`"session":{"metadata":{"keep":"yes","remove":null}},"replicas":2}`,
		Note: "POST, not PUT. The body is exactly this — a null session metadata " +
			"value removes that key",
	},
	{
		Name:     "triggers.delete",
		Method:   "DELETE",
		Path:     "/v1/triggers/{triggerID}",
		Response: `{"id":"trg_abc123","type":"trigger_deleted"}`,
	},
	{
		Name:     "triggers.pause",
		Method:   "POST",
		Path:     "/v1/triggers/{triggerID}/pause",
		Response: `{"status":"paused"}`,
	},
	{
		Name:     "triggers.unpause",
		Method:   "POST",
		Path:     "/v1/triggers/{triggerID}/unpause",
		Response: `{"status":"active"}`,
	},
	{
		Name:   "triggers.sessions.list",
		Method: "GET",
		Path:   "/v1/triggers/{triggerID}/sessions",
		Query:  []string{"limit", "page", "include_archived"},
		Note: "returns core Session entities, not a trigger-specific type; the " +
			"trigger ID is escaped",
	},
}

// pendingTriggerContractSpec pins the eight Trigger operationIds the core
// openapi/managed-agents.yaml declares to the SDK method that must implement
// each one. The TypeScript contract test greps
// `^\s+operationId:\s+(triggers\.\S+)$` out of the spec and asserts the sets
// match exactly and that every mapped accessor is callable — so a spec
// operation with no SDK method, or an SDK method with no spec operation, is a
// failure in both directions.
var pendingTriggerContractSpec = []pendingPortOp{
	{Name: "triggers.create", Method: "POST", Path: "/v1/triggers", Note: "SDK: triggers.Create"},
	{Name: "triggers.list", Method: "GET", Path: "/v1/triggers", Note: "SDK: triggers.List"},
	{Name: "triggers.get", Method: "GET", Path: "/v1/triggers/{triggerID}", Note: "SDK: triggers.Retrieve — the operationId is `get`, the method is `retrieve`"},
	{Name: "triggers.update", Method: "POST", Path: "/v1/triggers/{triggerID}", Note: "SDK: triggers.Update"},
	{Name: "triggers.delete", Method: "DELETE", Path: "/v1/triggers/{triggerID}", Note: "SDK: triggers.Delete"},
	{Name: "triggers.pause", Method: "POST", Path: "/v1/triggers/{triggerID}/pause", Note: "SDK: triggers.Pause"},
	{Name: "triggers.unpause", Method: "POST", Path: "/v1/triggers/{triggerID}/unpause", Note: "SDK: triggers.Unpause"},
	{Name: "triggers.sessions", Method: "GET", Path: "/v1/triggers/{triggerID}/sessions", Note: "SDK: triggers.Sessions.List"},
}

// pendingSessionHandleSpec is the wire contract of the orca.session(id)
// ergonomic handle: a thin binding that pre-fills the session ID and must
// otherwise produce byte-for-byte the same requests as the underlying
// sub-resources.
var pendingSessionHandleSpec = []pendingPortOp{
	{
		Name:   "session(id).sessionId",
		Method: "-",
		Path:   "-",
		Note:   "the handle exposes the session ID it was built with; no request is made",
	},
	{
		Name:   "session(id).events.send",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}/events",
		Body:   `{"events":[{"type":"user.message","content":[{"type":"text","text":"test"}]}]}`,
	},
	{
		Name:   "session(id).events.stream",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/events/stream",
		Query:  []string{"from_cursor", "subpath", "event_deltas (repeated)"},
		Note:   "passing only RequestOptions must produce an EMPTY query string",
	},
	{
		Name:   "session(id).threads.events.stream",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/threads/{threadID}/stream",
		Query:  []string{"from_cursor", "event_deltas (repeated)"},
		Note: "/threads/{id}/stream, NOT /threads/{id}/events/stream. Passing only " +
			"RequestOptions must produce an EMPTY query string",
	},
	{
		Name:   "session(id).resources.add",
		Method: "POST",
		Path:   "/v1/sessions/{sessionID}/resources",
		Body:   `{"type":"file","file_id":"file_doc"}`,
	},
	{
		Name:   "session(id).files.download",
		Method: "GET",
		Path:   "/v1/sessions/{sessionID}/files/{fileID}/content",
		Note:   "sends Accept: application/octet-stream and returns the raw response body",
	},
}

func TestPendingEnvironments(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingEnvironmentsSpec)
}

func TestPendingFiles(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingFilesSpec)
}

func TestPendingSkills(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSkillsSpec)
}

func TestPendingSkillVersions(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSkillVersionsSpec)
}

func TestPendingTriggers(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingTriggersSpec)
}

func TestPendingTriggerContractCoverage(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingTriggerContractSpec)
}

func TestPendingSessionHandle(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingSessionHandleSpec)
}
