// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package option configures requests.
//
// The same options work in two places, which is the point: pass one to a
// constructor and it applies to every call that client makes, or pass it to a
// single call and it applies just there. A per-call option always wins, because
// the client replays its own options first.
//
//	client, err := orca.New(
//		option.WithBaseURL("https://orca.example.com"),
//		option.WithAPIKey(key),
//	)
//	agent, err := client.Agents.Retrieve(ctx, id, option.WithMaxRetries(0))
package option

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/internal/requestconfig"
)

// RequestOption configures a client or a single request.
type RequestOption = requestconfig.RequestOption

func newOption(fn func(*requestconfig.RequestConfig) error) RequestOption {
	return requestconfig.RequestOptionFunc(fn)
}

// WithBaseURL sets the deployment host root.
//
// This is the host root, not an API path: request paths carry their own /v1 or
// /apis prefix. A URL ending in the legacy /v1, /v1/registry, or /api/v1 suffix
// is accepted and stripped, with a deprecation notice on the warning writer.
func WithBaseURL(baseURL string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if strings.TrimSpace(baseURL) == "" {
			return apierror.Validationf("registry base URL is required")
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return apierror.Validationf("invalid registry base URL %q: %w", baseURL, err)
		}
		requestconfig.NormalizeBaseURL(cfg, parsed, baseURL)
		return nil
	})
}

// WithAuthToken authenticates with a StreamNative Cloud OIDC access token, sent
// as an Authorization: Bearer header.
//
// This and [WithAPIKey] are mutually exclusive: the server reads x-api-key
// first and treats it as authoritative whenever present, so setting both would
// silently ignore the token. The last one applied wins.
func WithAuthToken(token string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if strings.TrimSpace(token) == "" {
			return apierror.Validationf("registry access token is required")
		}
		cfg.Credential = requestconfig.Credential{
			Header: "Authorization",
			Format: func(t string) string { return "Bearer " + t },
			Token:  token,
		}
		return nil
	})
}

// WithAPIKey authenticates with an Orca workspace API key (orca_...), sent as an
// x-api-key header. See [WithAuthToken] for why the two are exclusive.
func WithAPIKey(apiKey string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if strings.TrimSpace(apiKey) == "" {
			return apierror.Validationf("registry API key is required")
		}
		cfg.Credential = requestconfig.Credential{Header: "x-api-key", Token: apiKey}
		return nil
	})
}

// WithAuthTokenProvider authenticates with a token minted per request.
//
// Use it when the credential rotates - a short-lived OIDC token, or one fetched
// from a secret manager. Without it a rotating credential means rebuilding the
// client, which is easy to forget and fails only once the old token expires.
// The function is called on every attempt, including retries, so a token that
// expired mid-call is refreshed rather than replayed.
func WithAuthTokenProvider(fetch func(ctx context.Context) (string, error)) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if fetch == nil {
			return apierror.Validationf("credential provider is required")
		}
		cfg.Credential = requestconfig.Credential{
			Header: "Authorization",
			Format: func(t string) string { return "Bearer " + t },
			Fetch:  fetch,
		}
		return nil
	})
}

// WithAPIKeyProvider authenticates with an API key minted per request. See
// [WithAuthTokenProvider].
func WithAPIKeyProvider(fetch func(ctx context.Context) (string, error)) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if fetch == nil {
			return apierror.Validationf("credential provider is required")
		}
		cfg.Credential = requestconfig.Credential{Header: "x-api-key", Fetch: fetch}
		return nil
	})
}

// WithoutAuthentication removes any credential.
//
// Use it for the endpoints whose OpenAPI security is explicitly empty, such as
// /healthz and /readyz, or when the deployment sits behind a proxy that
// authenticates on the caller's behalf.
func WithoutAuthentication() RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Credential = requestconfig.Credential{}
		return nil
	})
}

// WithHTTPClient replaces the underlying client, for callers who need their own
// transport, proxy, or TLS configuration.
func WithHTTPClient(client *http.Client) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if client == nil {
			return apierror.Validationf("HTTP client is required")
		}
		cfg.HTTPClient = client
		return nil
	})
}

// WithHeader sets a header on every request, replacing any previous value.
func WithHeader(name, value string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Header.Set(name, value)
		return nil
	})
}

// WithHeaderAdd appends a header value, keeping any already set. Use it for the
// headers that are genuinely multi-valued; [WithHeader] is right for the rest.
func WithHeaderAdd(name, value string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Header.Add(name, value)
		return nil
	})
}

// WithHeaderDel removes a header the client would otherwise send.
//
// Passed per call, it also suppresses a header the client was built with, which
// is the only way to make one call opt out of a client-wide default.
func WithHeaderDel(name string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Header[http.CanonicalHeaderKey(name)] = nil
		return nil
	})
}

// WithQuery sets a query parameter on every request, replacing any previous
// value. A parameter the call supplies in its own path or params wins over one
// set here.
func WithQuery(key, value string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Query.Set(key, value)
		return nil
	})
}

// WithQueryAdd appends a query parameter value, keeping any already set.
func WithQueryAdd(key, value string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Query.Add(key, value)
		return nil
	})
}

// WithQueryDel removes a query parameter the client would otherwise send.
func WithQueryDel(key string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.Query.Del(key)
		return nil
	})
}

// WithMaxRetries bounds how many times a failed request is retried after the
// first attempt. Zero disables retrying.
//
// Only failures that are safe to repeat are retried at all - a timeout, a
// conflict, a rate limit, a 5xx, or a transport failure. A rejected credential
// or a malformed request is returned immediately, because another identical
// attempt cannot do better.
func WithMaxRetries(retries int) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if retries < 0 {
			return apierror.Validationf("max retries must not be negative, got %d", retries)
		}
		cfg.MaxRetries = retries
		return nil
	})
}

// WithRequestTimeout bounds a single attempt.
//
// The budget is per attempt rather than per call, so a retry starts with a full
// deadline instead of inheriting whatever the previous attempt left behind.
// Zero removes the deadline, leaving only the context's.
func WithRequestTimeout(timeout time.Duration) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if timeout < 0 {
			return apierror.Validationf("request timeout must not be negative, got %s", timeout)
		}
		cfg.RequestTimeout = timeout
		return nil
	})
}

// WithIdempotencyKey sets an Idempotency-Key header so a retried mutation is
// applied once rather than repeated.
//
// It is sent on mutating methods only. A GET is already safe to replay, and
// sending the header there would imply the server should deduplicate reads.
// Because a key identifies one logical operation, this belongs on a call rather
// than on a client - a client-wide key would make every mutation look like the
// same one.
func WithIdempotencyKey(key string) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		cfg.IdempotencyKey = key
		return nil
	})
}

// WithRawJSON captures the response body exactly as the server sent it.
//
// A decoded value is shaped by whatever this SDK models: a field the types do
// not declare is dropped, and an empty one may vanish. That is fine for reading
// a value, and wrong for reproducing a response - rendering it to a user,
// forwarding it, or storing it. Use this to get both, the typed value to work
// with and the bytes to reproduce:
//
//	var raw []byte
//	agent, err := client.Agents.Get(ctx, id, params, option.WithRawJSON(&raw))
//
// It applies to operations with a JSON response body. Streaming and download
// operations hand their body to the caller already, and leave raw untouched.
func WithRawJSON(raw *[]byte) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if raw == nil {
			return apierror.Validationf("raw JSON destination is required")
		}
		cfg.RawBody = raw
		return nil
	})
}

// WithWarningWriter routes non-fatal diagnostics, such as the legacy base-URL
// deprecation notice, to w. It defaults to discarding them.
//
// Command-line callers should point this at stderr, never stdout: stdout is
// parseable output, and a warning in the middle of it corrupts the result.
func WithWarningWriter(w io.Writer) RequestOption {
	return newOption(func(cfg *requestconfig.RequestConfig) error {
		if w == nil {
			w = io.Discard
		}
		cfg.WarningWriter = w
		return nil
	})
}
