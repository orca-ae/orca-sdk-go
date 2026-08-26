# Examples

Runnable samples, one directory per example. They live in a separate Go module
that replaces `github.com/orca-ae/orca-sdk-go` with the working tree, so they
always compile against local changes — `./scripts/lint` builds them for exactly
that reason.

Every example reads its deployment from the environment:

| Variable | Meaning |
|---|---|
| `ORCA_BASE_URL` | Deployment host root, e.g. `https://orca.example.com`. No `/v1` suffix. |
| `ORCA_API_KEY` | Workspace API key (`orca_...`), sent as `x-api-key`. |
| `ORCA_ACCESS_TOKEN` | StreamNative Cloud OIDC token, sent as `Authorization: Bearer`. |

Supply exactly one credential. The server reads `x-api-key` first and treats it
as authoritative whenever present.

```bash
cd examples
ORCA_BASE_URL=https://orca.example.com ORCA_API_KEY=orca_... go run ./quickstart
```
