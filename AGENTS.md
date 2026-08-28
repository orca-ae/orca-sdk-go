# orca-sdk-go — contributor guide

Conventions every contribution to this SDK follows. `CLAUDE.md` is a one-line
pointer here.

## 0. Status

The layered structure described below is in place, and the core and cloud
surfaces are typed. `api.md` is the generated index; `docs/surface-status.md`
says what each surface is for and where the contract has a sharp edge.

`go test -v ./... 2>&1 | grep -c -- '--- SKIP'` reports what still skips. What
remains is credential-gated integration tests, which run against a live
deployment, and a handful of TypeScript runtime behaviours with no Go analogue.
Neither is outstanding work.

## 1. Source of truth

Three vendored artifacts define the surface. Every exported method maps 1:1 to an
`operationId` in whichever one governs it. We do not invent operations, rename
endpoints, or add fields no spec declares.

| Artifact | Base-path rule | Governs |
|---|---|---|
| `openapi/managed-agents.yaml` | No `servers` entry; paths carry `/v1`, `/api`, or `/apis` explicitly. | Core: agents (+versions), sessions (+events, files, resources, threads(.events)), environments, files, skills (+versions), vaults (+credentials), memory stores (+memories, memory versions), triggers (+sessions), discovery. |
| `openapi/managed-agents-deployment.overlay.yaml` | Applies to `managed-agents.yaml`. | Deviations from the core contract. Its removals define what is **not** portable across both supported backends. |
| `openapi/cloud-extensions.yaml` | `/apis/cloud.sn.io/v1` | The whole `Cloud` namespace: API-resource discovery, agent providers, catalogs, connections, functions, health, packages, sink/source connectors, Kafka Connect. |

Apply the overlay's portability rules to core APIs:

- `remove: true` operations are not exposed. Agent deletion is removed — retire
  an agent by archiving it.
- `remove_properties` fields are omitted from request types.
- Removed query parameters are omitted from parameter types.
- `x-deployment-query-parameter-extensions` are deployment-only, not part of the
  portable surface. This includes top-level `provider` filters.
- `x-deployment-trigger-schema-extension` deliberately widens the Trigger schemas
  for the managed deployment. Preserve its Kafka/Pulsar sources, session modes,
  and replica range. The SDK does not preflight backend capability; a backend
  implementing only the narrower subset returns its own API error.

When a spec changes, the SDK changes — not the other way around.

## 2. Project layout

```
*.go                 One file per resource, flat at the root: agent.go,
                     session.go, cloudconnection.go, ...
client.go            Client with a field per top-level service
option/              RequestOption — the per-call and per-client knobs
internal/
  apierror/          Typed error hierarchy
  apijson/           Marshal helpers, union decoding
  apiquery/          Struct to query string
  apiform/           multipart/form-data encoding
  requestconfig/     The request pipeline: auth, retries, headers, base URL
packages/
  param/             Opt[T] — absent vs. null vs. value
  pagination/        PageCursor and its auto-pager
  ssestream/         Stream[T] over Server-Sent Events
  respjson/          Response-field presence metadata
lib/                 Conveniences built on resources (SessionHandle)
examples/            Runnable samples; separate module, replaces this one
openapi/             Vendored specs
scripts/             bootstrap, format, lint, test, detect-breaking-changes
```

`internal/` is enforced by the compiler: nothing outside this module can import
it. Put anything consumers must not depend on there.

File naming: lowercase, no underscores or hyphens, matching the resource path —
`memorystorememoryversion.go` for `memory_stores/{id}/memory_versions`. Sub-
resources are separate files, not subdirectories.

## 3. Service pattern

```go
type AgentService struct {
	Options  []option.RequestOption
	Versions AgentVersionService
}

func NewAgentService(opts ...option.RequestOption) (r AgentService) {
	r.Options = opts
	r.Versions = NewAgentVersionService(opts...)
	return
}
```

- Services are values, not pointers. They carry only their options.
- Every method takes `ctx context.Context` first and
  `opts ...option.RequestOption` last.
- Sub-services are fields, constructed in the parent's constructor so they
  inherit its options.
- Methods prepend `r.Options` to the caller's opts, so per-call options win.

## 4. HTTP-method mapping

Match the spec verb. Period.

| Use | Verb |
|---|---|
| Retrieve, list | `GET` |
| Create, and action endpoints (archive, validate, send, restart) | `POST` |
| Full replacement / optimistic-concurrency update carrying `version` | `PUT` |
| Partial update | `PATCH` |
| Permanent delete | `DELETE` |

**Every core resource updates with `POST`, not `PUT`.** If a spec verb looks
wrong, push back upstream rather than "correcting" it here — that just creates
drift.

## 5. Path style

The base URL is the **deployment host root**. Every path literal carries its own
full prefix; nothing is implicit in the client.

- Core paths start with `v1/...`.
- Cloud extension paths start with `apis/cloud.sn.io/v1/...`.
- Archive endpoints are `POST .../{id}/archive` — a sub-path, never `:archive`.
  Where the spec itself uses a colon action (some cloud connector endpoints),
  mirror it literally.
- JSON field names stay snake_case (`created_at`, `agent_id`). Go field names are
  Go-style; the struct tag carries the wire name.

Always percent-escape interpolated segments:

```go
"v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID)
```

An ID containing `/` must produce `%2F`, never an extra path segment.

**Tests proving a path builder's output must spell the literal path out**, not
reference `CloudExtensionBasePath`. Asserting against the same constant the
implementation uses would still pass if the constant were wrong.

The base-URL handling in `client.go` strips a trailing `/v1`, `/v1/registry`, or
`/api/v1` with a deprecation warning. `/api/v1` strips whole: leaving `/api`
would still resolve core paths through the alias while silently breaking every
`/apis/...` extension call. That shim exists for pre-existing callers — do not
write new code that depends on it.

## 6. Type style

- Co-locate request, response, and shared types with the service in one file.
- Naming: `Agent` for the entity, `AgentNewParams` / `AgentUpdateParams` /
  `AgentListParams` for requests, `AgentDeleted` for tombstones.
- **Optional request fields use `param.Opt[T]` with `json:",omitzero"`.** This is
  what distinguishes absent from `null` from a value. A versionless update must
  not synthesize a `version` key; an explicit `null` must clear the field.
- Nullable response fields use pointers, or `Opt[T]` where a test needs to tell
  null from absent.
- Discriminated unions get an explicit `UnmarshalJSON` switching on the
  discriminator. No reflective union registry.
- **Never add a `beta_version` request-body field.** No spec declares one, and
  sending it fails against the cloud's skill provider dialect. Beta opt-in is a
  header concern: `option.WithHeader("orca-beta", ...)`.

## 7. Pagination

Two cursor dialects share one `PageCursor[T]`:

- **Page-token** — response carries `next_page`, next request sends
  `page=<next_page>`. Used by agents, sessions, threads, memory stores, memories,
  memory versions, vaults, credentials, environments, skills, triggers.
- **ID-cursor** — response carries `first_id`/`last_id`, next request sends
  `after_id=<last_id>` or, walking backwards, `before_id=<first_id>`. Used by
  files and session files **only**.

A `before_id` walk keeps its direction: it replaces the `before_id` anchor with
`first_id` and adds no `after_id`, even when `last_id` is present.

Every list method comes in two forms: `List` returning one page, and
`ListAutoPaging` returning an iterator that walks them all.

## 8. Streaming

The server speaks Server-Sent Events. `Stream[T]` decodes it into typed events;
`DecodeSSE` transcodes it to NDJSON for command-line consumers.

## 9. Multipart uploads

File, skill, and cloud function/package endpoints use `multipart/form-data`.
`internal/apiform` encodes it. Field and file names are sanitised before going
into a `Content-Disposition` header — a resource name is user input, and an
unescaped quote or newline there is header injection.

## 10. Errors

`internal/apierror.Error` is the root, with a type per status. Errors carry the
status code, request ID, and response headers, so a caller can read `Retry-After`
or correlate with server logs.

Cloud methods gate on discovery: calling one against a deployment that does not
advertise `cloud.sn.io` returns `ExtensionNotAvailableError`, not a bare 404. An
**empty** group list is a normal deployment with no extensions installed — not
an error, and not the same as a 404 from `/apis` itself, which means the
deployment predates discovery entirely.

## 11. Branding & naming

**This SDK does not reference any upstream vendor — codename, company, or
third-party SDK — anywhere in code, comments, docs, examples, commit messages, or
tests.** To describe lineage, say "this SDK" or name a specific local file.

## 12. Doc comments

Every exported identifier gets a doc comment starting with its name. Explain
*why* where the reason is not obvious from the code — a wire-format quirk, a
spec deviation, a deliberate deviation from the obvious implementation. Do not
narrate what the next line plainly does.

## 13. Tests

Two suites cover the same code from different angles. Keep both.

- `*_test.go` — `httptest.NewServer`, asserting literal request paths.
- `*_port_test.go` — ported from the TypeScript suite; a recording
  `http.RoundTripper` capturing each request for assertion. `porttest_test.go`
  holds the shared harness.

`pending_*_port_test.go` files carry spec tables — exact method, path, query,
body, and response for operations not yet implemented. **Implementing a resource
means converting its table into real tests**, not deleting it.

Integration tests (`integration_port_test.go`) self-skip without
`ORCA_TEST_API_KEY`. E2E tests (`e2e_port_test.go`) need `-tags e2e`. Neither
runs on a clean `go test ./...`, which must stay green offline.

## 14. Consumer compatibility

`orca-cli` imports this module. Before opening a PR:

```bash
cd ../orca-cli
go mod edit -replace github.com/orca-ae/orca-sdk-go=../orca-sdk-go
go build ./... && go test ./...
go mod edit -dropreplace github.com/orca-ae/orca-sdk-go
```

If that fails, either keep a deprecated shim or land the migration in the same
change. `./scripts/detect-breaking-changes "$(git merge-base HEAD origin/main)"`
reports exported-API breaks explicitly.

## 15. Before opening a PR

```bash
./scripts/format
./scripts/lint
./scripts/test
./scripts/detect-breaking-changes "$(git merge-base HEAD origin/main)"
```

Then the consumer check in §14. Update `api.md` when the public surface changes.

## 16. Adding a resource — checklist

1. Find the `operationId` in the governing spec; apply the overlay's rules.
2. Create `<resource>.go` with the service, its params, and its response types.
3. Mount it on `Client` (or its parent service).
4. Convert the matching `pending_*` spec table into real tests.
5. Add an entry to `api.md`.
6. Run §15.
