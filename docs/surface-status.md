# Surface status

Which Orca API surfaces this SDK covers, and what is deliberately absent. It is
updated whenever an operation lands or is removed.

Four vendored artifacts govern the surface (see `AGENTS.md` §1):
`openapi/managed-agents.yaml` defines core operations,
`openapi/managed-agents-deployment.overlay.yaml` removes what is not portable
across both supported backends, `openapi/managed-agents-engine.yaml`
defines engine policy, pricing and runtime extensions plus explicit engine-only core additions, and `openapi/cloud-extensions.yaml`
governs `client.Cloud.*`.

The engine artifact is copied byte-for-byte from the public generated contract
at `80d1613ae47b42534307a854c1e7a6eb51fc996a` (SHA-256
`6d1ccc257d0f636bdf92a45e2b248607afefbe32eef12f1106896a98c2aabe1b`).
Its overlapping core definitions are informational: the portable core and
deployment overlay remain unchanged and govern existing core types. Only
explicitly documented engine additions below use the engine contract. Cloud
continues to use its own contract. `enginecontract_test.go` classifies all 114
engine operations as 89 portable, 21 engine extension/addition operations, and
four exclusions (agent deletion, Git credentials, and two Git proxy operations).

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
| `Threads.Interrupt` | Engine-only POST action on one thread; sends `{}`. Defaults to zero retries to avoid duplicate terminal events; explicit per-call retry options override this. |
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

Engine-only `RotateKey`, `RevokeKey`, and `WorkStats` use the literal
`/v1/environments/{id}/rotate-key`, `/revoke-key`, and `/work_stats` paths.
They do not probe an invented core capability endpoint: unsupported deployments
return their normal API error. `RotateKey` returns the new environment key and
its expiry; callers are responsible for keeping the key private.

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

`PolicyGroupResources` and `PricingGroupResources` list the resources advertised
by those extension groups. Both calls are gated on discovery.

Engine-only `RuntimeGroupResources` and `Harnesses` require the runtime v1
group version. `PricingV2GroupResources` requires pricing v2, not just the
pricing group. Resource discovery preserves unknown `RequiredFeatures` strings.

## Policy and pricing extensions

### Guardrails — `client.Guardrails`

Served under `/apis/policy.runorca.ai/v1`. `Create`, `List`, `Get`, `Update`,
`Archive`, `Delete`, and `ListTypes` are all gated on the policy extension.
Agent attachments and session-local overrides use `guardrail_ids`, require the
same gate, and leave beta opt-in to the caller's `orca-beta` header.

Rules support builtin, expression, and Dogwood (`Policy`, `AppliesTo`, `History`,
`MCPManifests`). `Mode` is `enforce` or `observe`; responses expose `Warnings`.
Builtin state scopes include `workspace_window`. Existing exported rule fields
remain available. Create and Update require known valid rule shapes; unknown
response kinds retain their entire JSON object in `GuardrailRule.Raw` and
round-trip without losing fields. Malformed known kinds return a decode error.

`DogwoodSchema` retrieves the language schema. `Effective` negotiates
`rule_kinds` and `guardrail_capabilities` and accepts `IfNoneMatch`. Its result
exposes `StatusCode`, `Header`, and an optional `Bundle`: 200 has the bundle;
304 has no body. Both preserve ETag, Cache-Control and
Orca-Guardrail-Issued-At/Expires-At/Generation headers. A 304 does not extend any
SDK cache: callers own bundle storage and lease interpretation. Other JSON
operations continue to reject 304.

### Model prices — `client.ModelPrices`

Served under `/apis/pricing.runorca.ai/v1`. `List` reads the effective price
catalog and `Get` retrieves one model, optionally qualified by provider. Both
calls are gated on the pricing extension.

`client.ModelPriceRules.List` and `ListAutoPaging` use pricing v2 and require
that exact group version in discovery. Each `ModelPriceRulePage` retains
`CatalogID`, `Generation`, `Digest`, `PublishedAt`, `ExpiresAt`,
`RequiredFeatures`, and `ConditionalModels`. The auto-pager's `Page()` exposes
the current page metadata. Unknown required feature names remain strings;
callers decide whether they support them. Subsequent pages preserve request
options and cursors. A 410 terminates the walk with its API error; the SDK never
restarts or mixes catalogs, and rejects changed catalog identities. It does
not evaluate billing or automatically cache catalogs.

## Cloud extensions — `client.Cloud.*`

Served under `/apis/cloud.sn.io/v1` by the cloud extension service rather than
the core engine. **Every call is gated**: against a deployment that does not
advertise `cloud.sn.io`, it returns `ExtensionNotAvailableError` rather than a
404 from a path the caller has no reason to doubt. Discovery snapshots are cached
per deployment base URL. Supported groups and versions reuse the snapshot
regardless of age. Missing groups or versions refresh after 15 seconds from the
last successful fetch; concurrent missing probes share the refresh. Failed
refreshes return the discovery error and retain the last successful snapshot
and timestamp, so supported capabilities remain usable and the next missing
probe can retry immediately. This also applies to policy, pricing and runtime
extension gates.

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
- **Git credential resolution** — removed by the portable deployment overlay.
- **Git proxy advertise/upload-pack** — special JWT authentication and binary
  Git smart HTTP are outside this SDK's resource API.
- **Admin and internal operations** — outside the public generated contract.
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
