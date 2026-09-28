# End-to-end harness

Stands up a real Managed Agents deployment in a kind cluster with Helm and runs
the SDK directly against it. `run-managed-agents-direct.sh` connects with a
workspace API key. The proprietary hosted provider topology is excluded; the
engine's internal registry remains part of the stack.

Session cleanup uses `POST /v1/sessions/{id}/archive` and verifies the returned
session ID and nonempty `archived_at`.

The stack stores objects in `object-store.yaml`, an S3-compatible RustFS
server plus a job that creates the bucket, because MinIO's images no longer
allow anonymous pulls. Its objects keep their `orca-managed-agents-minio`
names, which the Helm values address.

The runner executes:

```bash
go test -tags e2e -timeout 15m -run TestE2E ./...
```

The suite lives in `e2e_port_test.go` in the SDK module and is excluded from the
default build by the `e2e` tag, so `go test ./...` stays offline and green on a
clean checkout. It reads its deployment from the environment, which is what the
scripts set up:

| Variable | Meaning |
|---|---|
| `ORCA_BASE_URL` | Deployment host root, port-forwarded from the cluster |
| `ORCA_E2E_API_KEY` | Workspace API key |
| `ORCA_E2E_EXPECT_EXECUTION` | Whether sessions should actually run |

Set both `ORCA_BASE_URL` and `ORCA_E2E_API_KEY` to run the suite, or neither
to skip it. Setting only one fails before making requests. The runner sets
`ORCA_E2E_EXPECT_EXECUTION=true` to require deterministic execution and SSE replay.

The scenarios cover environment, agent, file, session, and trigger lifecycle,
pagination, event polling, Guardrail lifecycle and Agent/Session attachment,
seeded Model Price reads, and cleanup. Policy/pricing discovery is required.
The suite also asserts that the deployment does not advertise `cloud.sn.io`
and that hosted-extension calls fail with `ExtensionNotAvailableError`, rather
than quietly skipping those assertions. Hosted-extension API behavior remains
covered by mocked tests.

## Running locally

Needs `kind`, `kubectl`, `helm`, `yq`, and read access to the engine source
revision the images were built from, which carries the Helm chart.
`dependencies.env` pins the paired backend release; the workflow
resolves the pinned tags to immutable digests and verifies that the registry and
harness images came from the same commit, because a mismatched pair fails in
ways that look like SDK bugs. Their shared OCI source revision selects the
matching Helm chart and deterministic fixture.

```bash
export KIND_HELM_NAMESPACE=orca-sdk-e2e
tests/e2e/deploy-managed-agents-helm.sh
tests/e2e/run-managed-agents-direct.sh
```

## In CI

`.github/workflows/e2e-managed-agents.yml` runs on same-repository pull requests,
pushes to `main`, nightly, and on demand. It requires `SNBOT_GITHUB_TOKEN` for
engine source access and begins with a gate that skips the job when that secret
is not configured or the pull request is from a fork. A repository that never
configured it is not misconfigured, and a permanently red workflow teaches
everyone to ignore it.

The workflow uploads no artifacts, because GitHub does not mask those.
