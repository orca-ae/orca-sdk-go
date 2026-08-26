# orca-sdk-go

Go client for the [Orca Agent Engine](https://github.com/orca-ae/orca-managed-agents)
(OMA) Registry API, plus the StreamNative Cloud extension surface.

```bash
go get github.com/orca-ae/orca-sdk-go
```

## Usage

The base URL is the **deployment host root** — no `/v1`, `/v1/registry`, or
`/api/v1` suffix. A legacy suffix is stripped with a deprecation warning on the
supplied warning writer.

OMA accepts two credential classes, and they are **not interchangeable**:

```go
// StreamNative Cloud OIDC / access token -> Authorization: Bearer
client, err := orca.NewClient("https://host.example.com", token, nil)

// OMA workspace API key (orca_...) -> x-api-key
client, err := orca.NewAPIKeyClient("https://host.example.com", apiKey, nil)
```

The server reads `x-api-key` first and treats it as authoritative whenever
present; Bearer is only consulted when no API key was supplied. Supply exactly
one.

Cloud extension resources live under `/apis/cloud.sn.io/v1` and exist only on
deployments that advertise the `cloud.sn.io` group. Check discovery before
calling them:

```go
groups, err := client.GetAPIGroups(ctx)
if err != nil {
	return err
}
if !groups.HasGroup(orca.CloudExtensionGroup) {
	return fmt.Errorf("deployment does not serve %s", orca.CloudExtensionGroup)
}
connections, err := orca.NewConnectionsClient(client).List(ctx)
```

An **empty** group list is a normal deployment with no extensions installed — it
is not an error, and it is not the same as a 404 from `/apis` itself, which means
the deployment predates discovery entirely.

See [`api.md`](api.md) for the full surface and [`examples/`](examples) for
runnable samples.

## Development

```bash
./scripts/bootstrap   # dependencies
./scripts/format      # gofmt -s -w .
./scripts/lint        # gofmt check, vet, build, tests compile, examples build
./scripts/test        # go test ./...
```

`go test ./...` runs offline. Tests needing a live deployment gate themselves —
see [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Status

This SDK is being restructured from the flat client extracted from `orca-cli`
into a layered one: per-resource services, request options, typed errors,
pagination, and typed streaming. The Managed Agents surface is currently a
generic untyped passthrough (`ManagedAgentsClient`); the Cloud extension surface
is typed.

The test suite ported from
[`orca-sdk-typescript`](https://github.com/orca-ae/orca-sdk-typescript) already
specifies the typed surface and skips pending its implementation, so the skip
count is the remaining work:

```bash
go test -v ./... 2>&1 | grep -c SKIP
```

`AGENTS.md` describes the destination and the conventions to follow.
