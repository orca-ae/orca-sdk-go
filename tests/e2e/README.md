# End-to-end harness

Stands up a real Managed Agents deployment in a kind cluster and runs the SDK
against it. Two topologies:

| Script | Topology | Credential |
|---|---|---|
| `run-managed-agents-direct.sh` | Managed Agents registry on its own | workspace API key |
| `run-registry-provider.sh` | Registry fronting a Managed Agents provider | StreamNative Cloud access token |

Both end by running the same suite:

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
| `ORCA_E2E_API_KEY` | Workspace API key, for the direct topology |
| `ORCA_E2E_ACCESS_TOKEN` | Cloud access token, for the provider topology |
| `ORCA_E2E_EXPECT_CLOUD` | Whether the deployment should advertise `cloud.sn.io` |
| `ORCA_E2E_EXPECT_EXECUTION` | Whether sessions should actually run |

The two expectation flags are what let one suite assert against both
topologies: a deployment without the cloud extension must be *observed* not to
advertise it, rather than the test quietly skipping the cloud assertions.

## Running locally

Needs `kind`, `kubectl`, `helm`, `yq`, and access to the private Managed Agents
chart. `dependencies.env` pins the images and source revision; the workflows
resolve the latest tags and verify that the registry and harness images came
from the same commit, because a mismatched pair fails in ways that look like
SDK bugs.

```bash
export KIND_HELM_NAMESPACE=orca-sdk-e2e
tests/e2e/deploy-managed-agents-helm.sh
tests/e2e/run-managed-agents-direct.sh
```

## In CI

`.github/workflows/e2e-managed-agents.yml` and `e2e-registry-provider.yml` run
these nightly and on demand. Both begin with a gate that skips the job when its
secrets are not configured, rather than failing: a repository that never
configured them is not misconfigured, and a permanently red workflow teaches
everyone to ignore it.
