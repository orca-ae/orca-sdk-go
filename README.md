# orca-sdk-go

Go client for the [Orca Agent Engine](https://github.com/orca-ae/orca-managed-agents)
(OMA), including its policy and pricing extensions, plus the StreamNative Cloud
extension surface.

<!-- x-release-please-start-version -->
```bash
go get -u 'github.com/orca-ae/orca-sdk-go@v0.2.0'
```
<!-- x-release-please-end -->

The version above is rewritten by the release pull request, so it always names
the current release. See [`RELEASING.md`](RELEASING.md).

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	orca "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/option"
)

func main() {
	client, err := orca.New(
		option.WithBaseURL("https://orca.example.com"),
		option.WithAPIKey(os.Getenv("ORCA_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	agent, err := client.Agents.Create(context.Background(), orca.AgentNewParams{
		Model: orca.Model("claude-sonnet-4-6"),
		Name:  "my-first-agent",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(agent.ID)
}
```

`ORCA_BASE_URL` and `ORCA_API_KEY` (or `ORCA_ACCESS_TOKEN`) are read from the
environment when the matching option is absent, so the same binary can be
pointed at another deployment without a recompile. An explicit option always
wins.

### The base URL is the host root

No `/v1`, `/v1/registry`, or `/api/v1` suffix — request paths carry their own
prefix. A legacy suffix is accepted and stripped, with a deprecation notice on
the warning writer.

### Credentials

Two credential classes, and they are **not** interchangeable:

```go
option.WithAPIKey(key)        // OMA workspace API key (orca_...) -> x-api-key
option.WithAuthToken(token)   // StreamNative Cloud OIDC token    -> Authorization: Bearer
```

The server reads `x-api-key` first and treats it as authoritative whenever
present, so supply exactly one. For a credential that rotates, use
`option.WithAuthTokenProvider` — it is consulted on every attempt, so a token
that expired mid-call is refreshed rather than replayed.

### Options

The same options work per-client and per-call, and a per-call option always
wins without leaking back onto the shared client:

```go
agent, err := client.Agents.Get(ctx, agentID, orca.AgentGetParams{},
	option.WithHeader("X-Trace-Id", traceID),
	option.WithMaxRetries(0),
)
```

Requests are retried only where repeating can help — a timeout, conflict, rate
limit, 5xx, or transport failure — and `Retry-After` is obeyed.

### Pagination

Lists return a cursor. `All` iterates every item across every page, yielding
the error as part of the iteration so a mid-walk failure cannot be mistaken for
the end of the data:

```go
page, err := client.Agents.List(ctx, orca.AgentListParams{})
if err != nil {
	return err
}
for agent, err := range page.All(ctx) {
	if err != nil {
		return err
	}
	fmt.Println(agent.ID)
}
```

### Streaming

```go
stream := client.Sessions.Events.Stream(ctx, sessionID, orca.SessionEventStreamParams{})
defer stream.Close()

for stream.Next() {
	event := stream.Current()
	fmt.Println(event.Type)
}
if err := stream.Err(); err != nil {
	return err
}
```

Breaking out of the loop is safe: `Close` aborts the request rather than
leaving it running.

### Errors

Every failure satisfies `orca.Error`, and each meaningful status has its own
type wrapping a common `orca.APIError`:

```go
var notFound *orca.NotFoundError
if errors.As(err, &notFound) {
	// the agent is gone
}

var apiErr *orca.APIError
if errors.As(err, &apiErr) {
	log.Printf("status %d, request %s", apiErr.StatusCode, apiErr.RequestID)
}
```

### Optional request fields

An optional field means three different things on the wire, and the API acts on
all three. `param.Opt` carries which one you meant:

```go
client.Agents.Update(ctx, agentID, orca.AgentUpdateParams{
	Name:        param.String("new name"),      // set it
	Description: param.Null[string](),          // clear it
	// System is absent, so the server leaves it alone
})
```

### Cloud extensions

`client.Cloud.*` is served under `/apis/cloud.sn.io/v1` and exists only on
deployments that advertise the `cloud.sn.io` group. Every call checks first:

```go
connections, err := client.Cloud.Connections.List(ctx)

var unavailable *orca.ExtensionNotAvailableError
if errors.As(err, &unavailable) {
	// this deployment has no connections at all, as opposed to none matching
}
```

### Policy and pricing extensions

Guardrails and model prices are served by separately discoverable extension
groups. Their typed services run the same cached capability check before the
business request:

```go
guardrail, err := client.Guardrails.Create(ctx, orca.GuardrailNewParams{
	Name: "protect-production",
	Rule: orca.GuardrailRule{
		Kind:    orca.GuardrailRuleBuiltin,
		Builtin: "block_tools",
		Params:  map[string]any{"tools": []string{"shell"}},
	},
})

prices, err := client.ModelPrices.List(ctx, orca.ModelPriceListParams{})
```

Attaching guardrails to an agent or a session-local override also requires an
explicit beta header; the SDK never synthesizes it:

```go
agent, err := client.Agents.Create(ctx, orca.AgentNewParams{
	Model:        orca.Model("claude-sonnet-4-6"),
	Name:         "guarded-agent",
	GuardrailIDs: []string{guardrail.ID},
}, option.WithHeader("orca-beta", "managed-agents-2026-04-01"))
```

## Documentation

- [`api.md`](api.md) — every exported symbol, generated from the source.
- [`docs/surface-status.md`](docs/surface-status.md) — what each surface is for
  and where the contract has a sharp edge.
- [`examples/`](examples) — runnable samples.
- [`AGENTS.md`](AGENTS.md) — conventions for contributing.

## Development

```bash
./scripts/bootstrap   # dependencies
./scripts/format      # gofmt -s -w .
./scripts/lint        # gofmt, vet, build, tests compile, examples, shell, e2e
./scripts/test        # go test ./...
```

`go test ./...` runs offline. Tests needing a live deployment gate themselves —
see [`CONTRIBUTING.md`](CONTRIBUTING.md).
