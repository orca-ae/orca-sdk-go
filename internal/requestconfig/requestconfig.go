// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package requestconfig holds the request pipeline: everything that happens
// between a resource method deciding what to ask for and a response coming
// back.
//
// It exists as its own package so the knobs have one home. Base URL resolution,
// credentials, default headers and query, retries, timeouts and idempotency all
// need to compose - a per-call option has to beat a per-client one, and a
// resource has to be able to scope itself to an API group without re-deriving
// any of the rest.
package requestconfig

import (
	"context"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/orca-ae/orca-sdk-go/internal"
	"github.com/orca-ae/orca-sdk-go/internal/apierror"
)

// RequestOption mutates a [RequestConfig]. It is the single extension point for
// both client-wide and per-call settings: a client stores the options it was
// built with and replays them ahead of each call's own, so a per-call option
// always wins.
type RequestOption interface {
	Apply(*RequestConfig) error
}

// RequestOptionFunc adapts a plain function to [RequestOption].
type RequestOptionFunc func(*RequestConfig) error

// Apply implements [RequestOption].
func (f RequestOptionFunc) Apply(cfg *RequestConfig) error { return f(cfg) }

const (
	// DefaultMaxRetries matches the retry budget the other SDKs for this API
	// use. Retries only ever cover failures that are safe to repeat - see
	// [ShouldRetry].
	DefaultMaxRetries = 2

	// DefaultTimeout sits just above the 60 seconds a synchronous function
	// trigger may spend waiting for an output message, so the client does not
	// give up on a call the server is still allowed to be working on.
	DefaultTimeout = 90 * time.Second

	initialRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 8 * time.Second
)

// Credential is how a request proves who it is.
//
// The two credential classes this API accepts are not interchangeable, and the
// server reads x-api-key first whenever both are present, so a client carries
// exactly one rather than trying to send both and hoping.
type Credential struct {
	// Header is the header name to set, e.g. "Authorization" or "x-api-key".
	// An empty Header means an unauthenticated client.
	Header string

	// Format turns a raw token into the header value, e.g. prefixing "Bearer ".
	Format func(token string) string

	// Token is a fixed credential.
	Token string

	// Fetch supplies a credential per request, for callers whose token rotates
	// or is minted on demand. It takes precedence over Token.
	Fetch func(ctx context.Context) (string, error)
}

// Resolve returns the header name and value to send, or an empty name when the
// client is unauthenticated.
func (c Credential) Resolve(ctx context.Context) (string, string, error) {
	if c.Header == "" {
		return "", "", nil
	}
	token := c.Token
	if c.Fetch != nil {
		fetched, err := c.Fetch(ctx)
		if err != nil {
			return "", "", apierror.Errorf("failed to obtain credential: %w", err)
		}
		token = fetched
	}
	if token == "" {
		return "", "", nil
	}
	if c.Format != nil {
		return c.Header, c.Format(token), nil
	}
	return c.Header, token, nil
}

// RequestConfig is the resolved settings for a request.
type RequestConfig struct {
	// BaseURL is the deployment host root. Paths carry their own full prefix.
	BaseURL *url.URL

	// PathPrefix scopes a client to one API group, so a resource can keep
	// passing short resource-relative paths.
	PathPrefix string

	HTTPClient *http.Client

	// Header and Query are merged into every request. A per-request value
	// replaces the default rather than appending to it.
	Header http.Header
	Query  url.Values

	Credential Credential

	// MaxRetries bounds retry attempts after the first. Zero disables retrying.
	MaxRetries int

	// RequestTimeout bounds a single attempt. It is applied per attempt rather
	// than to the whole call so a retry gets a full budget of its own.
	RequestTimeout time.Duration

	// IdempotencyKey is sent on mutating requests only. Replaying a GET is
	// already safe, and sending the header there would imply the server should
	// deduplicate reads.
	IdempotencyKey string

	// WarningWriter receives diagnostics that are not failures, such as the
	// legacy base-URL deprecation notice. It must never be stdout for a
	// command-line caller, whose stdout is parseable output.
	WarningWriter io.Writer

	// Sleep waits between retry attempts. Tests replace it so a retry policy
	// can be asserted without spending the backoff.
	Sleep func(ctx context.Context, d time.Duration) error

	// pendingWarnings holds diagnostics raised while options were still being
	// applied. They are buffered rather than written immediately because
	// WithWarningWriter may not have run yet - otherwise whether a deprecation
	// notice reached the caller would depend on the order they passed their
	// options in.
	pendingWarnings []string
}

// Warn queues a diagnostic for the warning writer. It is flushed by
// [RequestConfig.FlushWarnings] once every option has been applied.
func (c *RequestConfig) Warn(message string) {
	c.pendingWarnings = append(c.pendingWarnings, message)
}

// FlushWarnings writes and clears any queued diagnostics.
func (c *RequestConfig) FlushWarnings() {
	if len(c.pendingWarnings) == 0 || c.WarningWriter == nil {
		c.pendingWarnings = nil
		return
	}
	for _, message := range c.pendingWarnings {
		_, _ = io.WriteString(c.WarningWriter, message)
	}
	c.pendingWarnings = nil
}

// New returns a config with defaults applied, then opts in order.
func New(opts ...RequestOption) (*RequestConfig, error) {
	cfg := &RequestConfig{
		HTTPClient:     &http.Client{Timeout: DefaultTimeout},
		Header:         http.Header{},
		Query:          url.Values{},
		MaxRetries:     DefaultMaxRetries,
		RequestTimeout: DefaultTimeout,
		WarningWriter:  io.Discard,
		Sleep:          sleep,
	}
	if err := cfg.Apply(opts...); err != nil {
		return nil, err
	}
	cfg.FlushWarnings()
	return cfg, nil
}

// Apply runs opts against the config in order, so a later option wins.
func (c *RequestConfig) Apply(opts ...RequestOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.Apply(c); err != nil {
			return err
		}
	}
	return nil
}

// Clone returns a deep-enough copy that mutating the result cannot be observed
// through the original. Header and Query are copied because options mutate them
// in place; a shallow copy would let a per-call header leak onto the client.
func (c *RequestConfig) Clone() *RequestConfig {
	if c == nil {
		return nil
	}
	clone := *c
	clone.pendingWarnings = nil
	clone.Header = c.Header.Clone()
	if clone.Header == nil {
		clone.Header = http.Header{}
	}
	clone.Query = url.Values{}
	for key, values := range c.Query {
		clone.Query[key] = append([]string(nil), values...)
	}
	if c.BaseURL != nil {
		baseURL := *c.BaseURL
		clone.BaseURL = &baseURL
	}
	return &clone
}

// With returns a copy of the config with opts applied, leaving the receiver
// untouched. This is what makes per-call options safe on a shared client.
func (c *RequestConfig) With(opts ...RequestOption) (*RequestConfig, error) {
	if len(opts) == 0 {
		return c, nil
	}
	clone := c.Clone()
	if err := clone.Apply(opts...); err != nil {
		return nil, err
	}
	clone.FlushWarnings()
	return clone, nil
}

// ResolveURL turns a resource-relative path into an absolute URL, applying the
// path prefix and merging the default query.
func (c *RequestConfig) ResolveURL(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", apierror.Validationf("registry path is required")
	}
	if c.BaseURL == nil {
		return "", apierror.Validationf("registry base URL is required")
	}

	relative, err := url.Parse(c.PathPrefix + strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", apierror.Validationf("invalid registry path %q: %w", path, err)
	}

	resolved := c.BaseURL.ResolveReference(relative)

	// The caller's own query string wins over a client-wide default, so a
	// per-call ?limit=10 is not silently overridden by a default.
	if len(c.Query) > 0 {
		query := resolved.Query()
		for key, values := range c.Query {
			if query.Has(key) {
				continue
			}
			for _, value := range values {
				query.Add(key, value)
			}
		}
		resolved.RawQuery = query.Encode()
	}

	return resolved.String(), nil
}

// ApplyHeaders sets the credential, the SDK identification headers, the caller's
// default headers, and the idempotency key on req.
//
// A header already present on the request wins: a resource that set Content-Type
// for a multipart body must not have it replaced by a client-wide default.
func (c *RequestConfig) ApplyHeaders(req *http.Request) error {
	name, value, err := c.Credential.Resolve(req.Context())
	if err != nil {
		return err
	}
	if name != "" && req.Header.Get(name) == "" {
		req.Header.Set(name, value)
	}

	for header, values := range c.Header {
		if len(req.Header[http.CanonicalHeaderKey(header)]) > 0 {
			continue
		}
		// A nil value means "delete this default", so a per-call option can
		// suppress a client-wide header rather than only override it.
		if values == nil {
			req.Header.Del(header)
			continue
		}
		for _, v := range values {
			req.Header.Add(header, v)
		}
	}

	setIdentificationHeaders(req)

	if c.IdempotencyKey != "" && isMutating(req.Method) {
		if req.Header.Get("Idempotency-Key") == "" {
			req.Header.Set("Idempotency-Key", c.IdempotencyKey)
		}
	}

	return nil
}

// isMutating reports whether replaying the method could create or change
// something, which is what makes an idempotency key meaningful.
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Do sends the request, retrying failures that are safe to repeat.
//
// newBody is called once per attempt rather than being handed a reader, because
// a retry has to send the body again and a reader is consumed by the first
// attempt. It may be nil for bodyless requests.
//
// The returned response's body must be closed by the caller, as usual: closing
// it also releases the attempt's timeout.
func (c *RequestConfig) Do(
	ctx context.Context,
	method, endpoint string,
	newBody func() (io.Reader, error),
	header http.Header,
) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		resp, err := c.attempt(ctx, method, endpoint, newBody, header, attempt)

		var retryable bool
		switch {
		case err != nil:
			retryable = isRetryableTransportError(err)
		default:
			retryable = ShouldRetry(resp)
		}
		if !retryable || attempt >= c.MaxRetries {
			return resp, err
		}

		delay := retryDelay(resp, attempt)
		drainAndClose(resp)

		if sleepErr := c.sleepFor(ctx, delay); sleepErr != nil {
			return nil, apierror.FromTransport(ctx, method, endpoint, sleepErr)
		}
	}
}

// attempt performs one try. The per-attempt timeout is released when the
// response body is closed, which is why the deadline is not simply deferred:
// the caller has yet to read the body when this returns.
func (c *RequestConfig) attempt(
	ctx context.Context,
	method, endpoint string,
	newBody func() (io.Reader, error),
	header http.Header,
	attempt int,
) (*http.Response, error) {
	var body io.Reader
	if newBody != nil {
		var err error
		if body, err = newBody(); err != nil {
			return nil, err
		}
	}

	attemptCtx, cancel := c.attemptContext(ctx)

	req, err := http.NewRequestWithContext(attemptCtx, method, endpoint, body)
	if err != nil {
		cancel()
		return nil, apierror.Errorf("failed to create %s request: %w", method, err)
	}
	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if err := c.ApplyHeaders(req); err != nil {
		cancel()
		return nil, err
	}
	c.setAttemptHeaders(req, attempt)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		cancel()
		return nil, apierror.FromTransport(ctx, method, endpoint, err)
	}

	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnClose ties an attempt's context to the lifetime of its response body,
// so the deadline is released exactly when the caller is done reading rather
// than being left to expire on its own.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// drainAndClose reads a bounded amount of a discarded response before closing
// it, so the underlying connection goes back to the pool instead of being torn
// down and redialled on every retry.
func drainAndClose(resp *http.Response) {
	if resp == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
}

func (c *RequestConfig) attemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.RequestTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.RequestTimeout)
}

func (c *RequestConfig) sleepFor(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	return sleep(ctx, d)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ShouldRetry reports whether a response should be retried.
//
// The server can override the decision either way with x-should-retry, which is
// how a deployment marks a 400 as transient or a 503 as terminal - it knows
// things about its own failure modes that a status code cannot express.
func ShouldRetry(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("x-should-retry"))) {
	case "true":
		return true
	case "false":
		return false
	}

	switch {
	case resp.StatusCode == http.StatusRequestTimeout:
		return true
	case resp.StatusCode == http.StatusConflict:
		// A conflict is usually a lost optimistic-concurrency race, which the
		// next attempt can win.
		return true
	case resp.StatusCode == http.StatusTooManyRequests:
		return true
	case resp.StatusCode >= 500:
		return true
	default:
		return false
	}
}

// isRetryableTransportError reports whether a failed attempt is worth another.
//
// Only failures of the network itself qualify. Everything else that can go
// wrong before a response arrives is deterministic - a rejected argument, a
// credential provider that returned an error, a request that would not build -
// and repeating it just burns the retry budget and the backoff delay while
// producing the identical failure. A caller-cancelled request is excluded for a
// different reason: the caller has already said to stop.
func isRetryableTransportError(err error) bool {
	var connErr *apierror.ConnectionError
	if errors.As(err, &connErr) {
		return true
	}
	var timeoutErr *apierror.TimeoutError
	return errors.As(err, &timeoutErr)
}

// retryDelay is how long to wait before the next attempt.
//
// A server that says how long to wait is obeyed; guessing a backoff when the
// answer was in the response is how a client turns rate limiting into an
// outage.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ms := resp.Header.Get("retry-after-ms"); ms != "" {
			if parsed, err := strconv.ParseFloat(ms, 64); err == nil && parsed >= 0 {
				return capDelay(time.Duration(parsed) * time.Millisecond)
			}
		}
		if after := resp.Header.Get("Retry-After"); after != "" {
			if seconds, err := strconv.ParseFloat(after, 64); err == nil && seconds >= 0 {
				return capDelay(time.Duration(seconds * float64(time.Second)))
			}
			if when, err := http.ParseTime(after); err == nil {
				if d := time.Until(when); d > 0 {
					return capDelay(d)
				}
				return 0
			}
		}
	}

	// Exponential backoff with jitter, so a fleet of clients retrying the same
	// failed deployment does not resynchronise into a thundering herd.
	backoff := float64(initialRetryDelay) * math.Pow(2, float64(attempt))
	jitter := 1 - 0.25*rand.Float64()
	return capDelay(time.Duration(backoff * jitter))
}

func capDelay(d time.Duration) time.Duration {
	if d > maxRetryDelay {
		return maxRetryDelay
	}
	if d < 0 {
		return 0
	}
	return d
}

// clientIdentifier names this SDK and version on every outgoing request.
//
// It is deliberately the whole story: no OS, architecture, or hostname. A
// single identifier is enough to tell an SDK request from any other Go HTTP
// client when reading server logs, and anything more granular would send the
// deployment details about the caller's machine that it has no use for.
var clientIdentifier = "orca-sdk-go/" + internal.Version

// userAgent adds the language runtime, which is the one extra detail that
// changes how a request behaves - TLS defaults, HTTP/2 support, and header
// canonicalisation all move between Go releases.
var userAgent = clientIdentifier + " " + runtime.Version()

func setIdentificationHeaders(req *http.Request) {
	if req.Header.Get("X-Orca-Client") == "" {
		req.Header.Set("X-Orca-Client", clientIdentifier)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
}

// setAttemptHeaders records which attempt this is and how long the caller is
// prepared to wait, so a deployment can tell a retry storm from genuine load
// and can stop working on a request whose caller has already given up.
func (c *RequestConfig) setAttemptHeaders(req *http.Request, attempt int) {
	req.Header.Set("X-Orca-Retry-Count", strconv.Itoa(attempt))
	if c.RequestTimeout > 0 {
		req.Header.Set("X-Orca-Timeout", strconv.Itoa(int(c.RequestTimeout.Seconds())))
	}
}
