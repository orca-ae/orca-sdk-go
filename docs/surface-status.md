# Surface status

Which Orca API surfaces this SDK covers, and what is deliberately absent. It is
updated whenever an operation lands or is removed.

Three vendored artifacts govern the surface (see `AGENTS.md` §1):
`openapi/managed-agents.yaml` defines core operations,
`openapi/managed-agents-deployment.overlay.yaml` removes what is not portable
across both supported backends, and `openapi/cloud-extensions.yaml` governs
`client.Cloud.*`.

`api.md` is the generated index of every exported symbol. This document is the
narrative counterpart: what each surface is for, and where the contract has a
sharp edge.

## Core

Core resources are served under `/v1` by the engine itself and are available on
every deployment.

### Agents — `client.Agents`

| Method | Description |
|---|---|
| `Create` | Create an agent. The model is a bare id or an object. |
| `Get` | Retrieve an agent, optionally at a historical `Version`. |
| `Update` | Partial update (POST). Omitted fields are left alone; null clears. |
| `List` | Page-token list, optionally including archived agents. |
| `Archive` | Retire an agent — `POST .../archive`. |
| `Versions.List` | Page an agent's historical snapshots. |

There is no `Delete`: the overlay removes agent deletion, so archiving is the
only retirement path.

### Sessions — `client.Sessions`

| Method | Description |
|---|---|
| `Create` | Create a session at `/v1/sessions`, never under an agent. |
| `Get` / `Update` / `List` / `Delete` / `Archive` | Lifecycle. Update is POST and accepts agent *overrides* only. |
| `Events.List` / `Send` / `Stream` | Read, append, and follow a session's events. |
| `Files.List` / `Get` / `Download` / `Delete` | Files attached to a session. ID-cursor pagination. |
| `Resources.List` / `Add` / `Get` / `Update` / `Delete` | Files, repositories, and memory stores the session can reach. |
| `Threads.List` / `Get` / `Archive` | Threads the coordinator spawned. No create. |
| `Threads.Events.List` / `Stream` | One thread's events. The stream is `/threads/{id}/stream`. |

`client.Session(id)` returns a handle that binds the session ID once, so the
sub-resource calls that follow do not repeat it.

### Memory stores — `client.MemoryStores`

| Method | Description |
|---|---|
| `Create` / `Get` / `Update` / `List` / `Delete` / `Archive` | Store lifecycle. |
| `Memories.List` / `Create` / `Get` / `Update` / `Delete` | Entries. `View` is a query parameter, never a body field. |
| `MemoryVersions.List` / `Get` / `Redact` | The audit trail of changes. |

A memory listing mixes entries with directory markers; `Memory.IsPrefix` tells
them apart. `MemoryDeleteParams.ExpectedContentSHA256` makes a delete
conditional on the content not having changed since it was read.

### Vaults — `client.Vaults`

| Method | Description |
|---|---|
| `Create` / `Get` / `Update` / `List` / `Delete` / `Archive` | Vault lifecycle. |
| `Credentials.*` | Credential lifecycle, plus `Validate`. |

`Credentials.Validate` posts to `mcp_oauth_validate` and returns a typed
`CredentialValidation` rather than a boolean: the MCP probe and the token
refresh fail for different reasons.

### Environments — `client.Environments`

`Create` / `Get` / `Update` / `List` / `Delete` / `Archive`. Packages,
networking, image and target live under `Config`; the overlay removes the
flattened top-level forms.

### Files — `client.Files`

`Upload` / `Get` / `Download` / `List` / `Delete`. ID-cursor pagination. The
uploaded part carries its own content type; the SDK sends no separate
`content_type` field.

### Skills — `client.Skills`

`Create` / `Get` / `List` / `Delete`, and `Versions.Create` / `List` / `Get` /
`Delete`. Files upload under the literal field name `files[]`. `Limit` is the
only portable list filter.

### Triggers — `client.Triggers`

`Create` / `List` / `Get` / `Update` / `Delete` / `Pause` / `Unpause`, and
`Sessions.List`. Triggers are **core**, not a cloud extension: the overlay
promoted them out of the cloud group, so `client.Cloud.Triggers` does not
exist. The managed-deployment Kafka and Pulsar sources, the four session modes,
and positive replica counts are preserved without client-side backend gating —
a deployment implementing only the narrower cron subset returns its own error.

### Discovery — `client.Discovery`

`Groups` lists the extension API groups a deployment serves. An empty list is a
normal deployment with no extensions installed, not an error.

## Cloud extensions — `client.Cloud.*`

Served under `/apis/cloud.sn.io/v1` by the cloud extension service rather than
the core engine. **Every call is gated**: against a deployment that does not
advertise `cloud.sn.io`, it returns `ExtensionNotAvailableError` rather than a
404 from a path the caller has no reason to doubt. Discovery is resolved once
per deployment and cached.

| Namespace | Operations |
|---|---|
| `Cloud.Agents.Providers` | `List`, `Get` |
| `Cloud.APIResources` | `List` |
| `Cloud.Catalog.{Kafka,Sinks,Sources}` | `List`, `Get` |
| `Cloud.Connections` | `List`, `Get`, `Create`, `Update`, `Delete`, `Test`, `Validate` |
| `Cloud.Functions` | Lifecycle, stats, status, state, and start/stop/restart per instance |
| `Cloud.Health` | `Health`, `Ready`, `Live` |
| `Cloud.Packages` | `List`, `ListVersions`, `GetMetadata`, `UpdateMetadata`, `Upload`, `Download`, `Delete` |
| `Cloud.Connectors.{Sinks,Sources}` | Lifecycle plus status and start/stop/restart |
| `Cloud.Connectors.Kafka` | Worker health and info, plugins, and the connector surface |

The pre-existing `NewCatalogClient`-style constructors reach the same
operations **without** the gate, and remain for callers that gate for
themselves.

## Deliberately absent

- **`agents.Delete`** — removed by the overlay; archive instead.
- **`Cloud.Triggers`** — triggers are core.
- **`SessionUpdateParams.EnvironmentID` / `.Resources`** — removed from the
  update request, so the type cannot express them.
- **`MemoryVersionListParams.SessionID`** — needs local-to-provider ID
  translation that is not portable.
- **Kafka Connect `ValidateConfig`** — the spec declares no successful
  response; its only response is a 400 saying validation is unsupported. The
  method exists and returns that error rather than pretending otherwise.
- **`beta_version` request fields** — no spec declares one, and sending it
  fails against the cloud's skill provider dialect. Beta opt-in is a header.
