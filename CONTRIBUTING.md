# Contributing

## Setup

```bash
./scripts/bootstrap    # Go dependencies (and Homebrew packages on macOS)
```

Requires Go 1.25 or newer — the SDK relies on `encoding/json`'s `omitzero` tag to
distinguish an absent request field from an explicit `null`.

## Day to day

```bash
./scripts/format   # gofmt -s -w .
./scripts/lint     # gofmt check, vet, build, tests compile, examples build
./scripts/test     # go test ./...
```

`go test ./...` must pass offline on a clean checkout. Tests that need a live
deployment gate themselves:

```bash
# Integration — skips itself when the credential is absent
ORCA_TEST_API_KEY=... ORCA_TEST_BASE_URL=... go test -run TestIntegration -v ./...

# End to end — excluded from the default build entirely
ORCA_BASE_URL=... ORCA_E2E_API_KEY=... go test -tags e2e -timeout 10m -run TestE2E ./...
```

## Skipped tests are the backlog

The suite ported from the TypeScript SDK specifies more than this SDK currently
implements. Each skip names the missing capability:

```bash
go test -v ./... 2>&1 | grep -c SKIP
```

`pending_*_port_test.go` files hold spec tables giving the exact method, path,
query, body, and response each unimplemented operation must produce.
Implementing one means turning its table into real tests.

## Conventions

`AGENTS.md` is the full guide — spec provenance, service patterns, path style,
pagination dialects, error handling, and the branding rule. Read §1 and §5 before
adding a resource; they are where mistakes are most expensive.

## Before opening a PR

```bash
./scripts/lint
./scripts/test
./scripts/detect-breaking-changes "$(git merge-base HEAD origin/main)"
```

`orca-cli` imports this module, so also confirm it still builds:

```bash
cd ../orca-cli
go mod edit -replace github.com/orca-ae/orca-sdk-go=../orca-sdk-go
go build ./... && go test ./...
go mod edit -dropreplace github.com/orca-ae/orca-sdk-go
```

## Commits and releases

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
— `feat:`, `fix:`, `chore:`, `test:`, `docs:`, with `!` or a `BREAKING CHANGE:`
footer for incompatible changes. Release Please reads them to pick the next
version and assemble the changelog, then a maintainer merges its release PR to
publish the tag. Go modules are served from the tag, so there is no separate
publish step.

Never edit the version in `internal/version.go` or the README install line by
hand — the release PR rewrites both. [`RELEASING.md`](RELEASING.md) has the full
process, including how to cut a release candidate.
