# orca-sdk-go

Go client for the [Orca Agent Engine](https://github.com/orca-ae/orca-managed-agents) (OMA)
Registry API, plus the StreamNative Cloud extension surface.

```bash
go get github.com/orca-ae/orca-sdk-go
```

## Usage

The base URL is the **deployment host root** — no `/v1`, `/v1/registry`, or `/api/v1` suffix.
A legacy suffix is stripped with a deprecation warning on the supplied warning writer.

OMA accepts two credential classes, and they are **not interchangeable**:

```go
// StreamNative Cloud OIDC / access token -> Authorization: Bearer
client, err := orca.NewClient("https://host.example.com", token, nil)

// OMA workspace API key (orca_...) -> x-api-key
client, err := orca.NewAPIKeyClient("https://host.example.com", apiKey, nil)
```

The server reads `x-api-key` first and treats it as authoritative whenever present; Bearer is
only consulted when no API key was supplied. Supply exactly one.

Cloud extension resources live under `/apis/cloud.sn.io/v1` and are only available on
deployments that advertise the `cloud.sn.io` group. Use the discovery helpers to check before
calling them.

## Development

```bash
./scripts/format   # gofmt -s -w .
./scripts/lint     # gofmt check, go vet, build, compile tests
./scripts/test     # go test ./...
```

## Status

This SDK currently exposes the Managed Agents surface through a generic, untyped passthrough
(`ManagedAgentsClient`); the Cloud extension surface is typed. Typed Managed Agents resources
are planned — the test suite ported from
[`orca-sdk-typescript`](https://github.com/orca-ae/orca-sdk-typescript) already specifies them
and is skipped pending implementation.
