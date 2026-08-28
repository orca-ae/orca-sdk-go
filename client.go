// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/internal/requestconfig"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/ssestream"
)

const (
	// defaultHTTPTimeout bounds one attempt. Function triggers may
	// synchronously wait up to 60 seconds for an output message, so the client
	// deadline sits above that server-side contract rather than cutting off a
	// call the server is still allowed to be working on.
	defaultHTTPTimeout = requestconfig.DefaultTimeout

	defaultJSONResponseBodyLimit = 10 << 20
	defaultMultipartBodyLimit    = 10 << 20
)

// Environment variables consulted when the corresponding option is not passed.
// They exist so a program can be pointed at a different deployment without a
// recompile, which is what makes the same binary usable across environments.
const (
	envBaseURL     = "ORCA_BASE_URL"
	envAPIKey      = "ORCA_API_KEY"
	envAccessToken = "ORCA_ACCESS_TOKEN"
)

// Client is the registry HTTP client.
type Client struct {
	cfg *requestconfig.RequestConfig

	// discovery is shared with every clone of this client, so scoping a client
	// to an API group or applying per-call options does not start a second
	// cache of the same deployment's capabilities.
	discovery *discoveryCache

	// Agents manages agents and their version history.
	Agents AgentService

	// Sessions manages sessions and everything scoped to one: events, files,
	// resources, and threads.
	Sessions SessionService

	// Cloud is the StreamNative Cloud extension surface. Every call through it
	// is gated on the deployment advertising the cloud.sn.io group.
	Cloud CloudService
}

// newClientFrom builds a client around cfg and wires its resource services.
func newClientFrom(cfg *requestconfig.RequestConfig, discovery *discoveryCache) *Client {
	if discovery == nil {
		discovery = newDiscoveryCache()
	}
	client := &Client{cfg: cfg, discovery: discovery}
	client.Agents = newAgentService(client)
	client.Sessions = newSessionService(client)
	client.Cloud = newCloudService(client)
	return client
}

// New returns a client configured by opts.
//
// The base URL is the deployment host root - no /v1, /v1/registry, or /api/v1
// suffix, since request paths carry their own prefix. A legacy suffix is
// stripped with a deprecation notice on the warning writer.
//
// When no base URL or credential is passed, ORCA_BASE_URL and then
// ORCA_API_KEY or ORCA_ACCESS_TOKEN are consulted. An explicit option always
// wins over the environment.
//
//	client, err := orca.New(
//		option.WithBaseURL("https://orca.example.com"),
//		option.WithAPIKey(os.Getenv("ORCA_API_KEY")),
//	)
func New(opts ...option.RequestOption) (*Client, error) {
	cfg, err := requestconfig.New(append(environmentOptions(), opts...)...)
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL == nil {
		return nil, apierror.Validationf("registry base URL is required")
	}
	return newClientFrom(cfg, nil), nil
}

// environmentOptions returns the options implied by the environment. They are
// applied before the caller's, so anything passed explicitly overrides them.
func environmentOptions() []option.RequestOption {
	var opts []option.RequestOption
	if value := strings.TrimSpace(os.Getenv(envBaseURL)); value != "" {
		opts = append(opts, option.WithBaseURL(value))
	}
	// The server reads x-api-key first and treats it as authoritative whenever
	// present, so an API key in the environment wins over an access token
	// exactly as it would on the wire.
	if value := strings.TrimSpace(os.Getenv(envAPIKey)); value != "" {
		opts = append(opts, option.WithAPIKey(value))
	} else if value := strings.TrimSpace(os.Getenv(envAccessToken)); value != "" {
		opts = append(opts, option.WithAuthToken(value))
	}
	return opts
}

// NewClient creates a registry client using the provided base URL and bearer
// token.
//
// Deprecated: use [New] with [option.WithBaseURL] and [option.WithAuthToken],
// which also accepts per-request options and a rotating credential.
func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	return NewClientWithWarningWriter(baseURL, token, httpClient, os.Stderr)
}

// NewUnauthenticatedClient creates a registry client for endpoints whose
// OpenAPI security is explicitly empty, such as /healthz and /readyz.
//
// Deprecated: use [New] with [option.WithoutAuthentication].
func NewUnauthenticatedClient(baseURL string, httpClient *http.Client) (*Client, error) {
	return NewUnauthenticatedClientWithWarningWriter(baseURL, httpClient, os.Stderr)
}

// NewClientWithWarningWriter creates a registry client and writes legacy
// base-URL diagnostics to warningWriter.
//
// Deprecated: use [New] with [option.WithWarningWriter].
func NewClientWithWarningWriter(baseURL, token string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, apierror.Validationf("registry access token is required")
	}
	return newLegacyClient(baseURL, httpClient, warningWriter, option.WithAuthToken(token))
}

// NewAPIKeyClient creates a registry client that authenticates with a Managed
// Agents workspace API key.
//
// Deprecated: use [New] with [option.WithAPIKey].
func NewAPIKeyClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	return NewAPIKeyClientWithWarningWriter(baseURL, apiKey, httpClient, os.Stderr)
}

// NewAPIKeyClientWithWarningWriter creates an API-key client and writes legacy
// base-URL diagnostics to warningWriter.
//
// Deprecated: use [New] with [option.WithAPIKey] and [option.WithWarningWriter].
func NewAPIKeyClientWithWarningWriter(baseURL, apiKey string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, apierror.Validationf("registry API key is required")
	}
	return newLegacyClient(baseURL, httpClient, warningWriter, option.WithAPIKey(apiKey))
}

// NewUnauthenticatedClientWithWarningWriter creates an unauthenticated registry
// client and writes legacy base-URL diagnostics to warningWriter.
//
// Deprecated: use [New] with [option.WithoutAuthentication] and
// [option.WithWarningWriter].
func NewUnauthenticatedClientWithWarningWriter(baseURL string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	return newLegacyClient(baseURL, httpClient, warningWriter, option.WithoutAuthentication())
}

// newLegacyClient backs the pre-option constructors. They are kept because
// callers depend on them; behind the façade they build the same config New does.
//
// Retries are off: these constructors predate a retry policy, and silently
// beginning to repeat a caller's mutations would change what their existing
// code does to the server.
func newLegacyClient(
	baseURL string,
	httpClient *http.Client,
	warningWriter io.Writer,
	credential option.RequestOption,
) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, apierror.Validationf("registry base URL is required")
	}
	if warningWriter == nil {
		warningWriter = os.Stderr
	}

	opts := []option.RequestOption{
		option.WithWarningWriter(warningWriter),
		option.WithBaseURL(baseURL),
		credential,
		option.WithMaxRetries(0),
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	cfg, err := requestconfig.New(opts...)
	if err != nil {
		return nil, err
	}
	return newClientFrom(cfg, nil), nil
}

// stripLegacyBaseURLSuffix removes a trailing legacy suffix from a base-URL
// path, if present.
func stripLegacyBaseURLSuffix(path string) (stripped string, matchedSuffix string) {
	return requestconfig.StripLegacyBaseURLSuffix(path)
}

// With returns a clone of the client with opts applied, leaving the receiver
// unchanged.
func (c *Client) With(opts ...option.RequestOption) (*Client, error) {
	cfg, err := c.cfg.With(opts...)
	if err != nil {
		return nil, err
	}
	return newClientFrom(cfg, c.discovery), nil
}

// WithDefaultHeader returns a client clone that adds a default header unless
// the request already sets that header.
func (c *Client) WithDefaultHeader(name, value string) *Client {
	cfg := c.cfg.Clone()
	cfg.Header.Set(name, value)
	return newClientFrom(cfg, c.discovery)
}

// WithPathPrefix returns a client clone that resolves every path relative to
// prefix instead of the base URL directly.
//
// Used to scope a client to one API group (for example "apis/cloud.sn.io/v1")
// so its resource methods keep passing short, resource-relative paths.
func (c *Client) WithPathPrefix(prefix string) *Client {
	cfg := c.cfg.Clone()
	trimmed := strings.Trim(prefix, "/")
	if trimmed != "" {
		trimmed += "/"
	}
	cfg.PathPrefix = trimmed
	return newClientFrom(cfg, c.discovery)
}

// scoped is WithPathPrefix without the resource services.
//
// The cloud services are built from clients scoped to their API group, so
// building those services from a scoped client would recurse forever. Nothing
// reached through a scoped client needs the tree, because the gate has already
// run by the time one is used.
func (c *Client) scoped(prefix string) *Client {
	cfg := c.cfg.Clone()
	trimmed := strings.Trim(prefix, "/")
	if trimmed != "" {
		trimmed += "/"
	}
	cfg.PathPrefix = trimmed
	return &Client{cfg: cfg, discovery: c.discovery}
}

// Options returns the client's request configuration. It is unexported state
// made reachable for resources in this package; callers configure a client
// through [option] instead.
func (c *Client) config() *requestconfig.RequestConfig { return c.cfg }

// getRootJSON performs a GET against the deployment host root, ignoring any
// path prefix the client is scoped to.
//
// Discovery and the core probes live at the root whatever a client is scoped
// to. Resolving them relative to a prefix asks the wrong deployment - a client
// scoped to an API group would probe {prefix}/apis, which does not exist, and
// report the extension as missing on a deployment that serves it.
func (c *Client) getRootJSON(ctx context.Context, path string, out interface{}, opts ...option.RequestOption) error {
	root := c
	if c.cfg.PathPrefix != "" {
		root = c.scoped("")
	}
	return root.doJSON(ctx, http.MethodGet, path, nil, out, opts...)
}

// GetJSON performs a GET request and decodes the JSON response body.
func (c *Client) GetJSON(ctx context.Context, path string, out interface{}, opts ...option.RequestOption) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out, opts...)
}

// PostJSON performs a POST request with a JSON request body.
func (c *Client) PostJSON(ctx context.Context, path string, body interface{}, out interface{}, opts ...option.RequestOption) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out, opts...)
}

// PutJSON performs a PUT request with a JSON request body.
func (c *Client) PutJSON(ctx context.Context, path string, body interface{}, out interface{}, opts ...option.RequestOption) error {
	return c.doJSON(ctx, http.MethodPut, path, body, out, opts...)
}

// PatchJSON performs a PATCH request with a JSON request body.
func (c *Client) PatchJSON(ctx context.Context, path string, body interface{}, out interface{}, opts ...option.RequestOption) error {
	return c.doJSON(ctx, http.MethodPatch, path, body, out, opts...)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, opts ...option.RequestOption) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, opts...)
}

// GetToWriter performs a GET request and streams the raw response body to writer.
func (c *Client) GetToWriter(ctx context.Context, path string, writer io.Writer, opts ...option.RequestOption) error {
	if writer == nil {
		return apierror.Validationf("response writer is required")
	}

	return c.GetStream(ctx, path, "*/*", func(reader io.Reader) error {
		if _, err := io.Copy(writer, reader); err != nil {
			return apierror.Errorf("failed to stream response: %w", err)
		}
		return nil
	}, opts...)
}

// GetStream performs a GET request and lets handle consume the raw response body.
//
// The body is handed over unread, so a long-lived stream - Server-Sent Events,
// a large download - is processed as it arrives instead of being buffered.
func (c *Client) GetStream(
	ctx context.Context,
	path string,
	accept string,
	handle func(io.Reader) error,
	opts ...option.RequestOption,
) error {
	if handle == nil {
		return apierror.Validationf("response handler is required")
	}

	body, endpoint, err := c.openStream(ctx, path, accept, opts...)
	if err != nil {
		return err
	}
	defer body.Close()

	if err := handle(body); err != nil {
		return apierror.Errorf("failed to stream %s %s response: %w", http.MethodGet, endpoint, err)
	}

	return nil
}

// openStream performs a GET and returns the response body, unread.
//
// The caller owns the body and must close it: closing is also what releases the
// request's deadline, so a stream abandoned without a Close leaves the attempt
// running until its timeout expires.
func (c *Client) openStream(
	ctx context.Context,
	path string,
	accept string,
	opts ...option.RequestOption,
) (io.ReadCloser, string, error) {
	cfg, err := c.cfg.With(opts...)
	if err != nil {
		return nil, "", err
	}
	endpoint, err := cfg.ResolveURL(path)
	if err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(accept) == "" {
		accept = "*/*"
	}
	header := http.Header{"Accept": []string{accept}}

	resp, err := cfg.Do(ctx, http.MethodGet, endpoint, nil, header)
	if err != nil {
		return nil, endpoint, err
	}

	if !isSuccess(resp.StatusCode) {
		defer resp.Body.Close()
		responseBytes, readErr := readResponseBodyLimited(resp.Body, defaultJSONResponseBodyLimit)
		if readErr != nil {
			return nil, endpoint, apierror.Errorf("failed to read %s %s response: %w", http.MethodGet, endpoint, readErr)
		}
		return nil, endpoint, apierror.FromResponse(http.MethodGet, endpoint, resp.StatusCode, string(responseBytes), resp.Header)
	}

	return resp.Body, endpoint, nil
}

// ListPage fetches one page of a paginated list.
//
// Like [StreamEvents] it is a function rather than a method, because Go does
// not allow methods to introduce type parameters. The returned cursor fetches
// its successors through this same client, so retries, credentials and
// per-call options apply to every page and not just the first:
//
//	page, err := orca.ListPage[Agent](ctx, client, "v1/agents")
//	if err != nil {
//		return err
//	}
//	for agent, err := range page.All(ctx) {
//		if err != nil {
//			return err
//		}
//		...
//	}
func ListPage[T any](
	ctx context.Context,
	client *Client,
	path string,
	opts ...option.RequestOption,
) (*pagination.PageCursor[T], error) {
	cfg, err := client.cfg.With(opts...)
	if err != nil {
		return nil, err
	}
	// Resolved up front so the cursor holds an absolute URL to derive the next
	// page's query from. Re-resolving an absolute URL is a no-op, so the
	// per-page fetch below stays a normal request.
	endpoint, err := cfg.ResolveURL(path)
	if err != nil {
		return nil, err
	}

	get := func(ctx context.Context, rawURL string, out any) error {
		return client.GetJSON(ctx, rawURL, out, opts...)
	}
	return pagination.Fetch[T](ctx, endpoint, get)
}

// StreamEvents opens a Server-Sent Events stream at path and decodes each
// event's payload as T.
//
// It is a function rather than a method because Go does not allow methods to
// introduce their own type parameters.
//
// The returned stream must be closed. A failure opening the stream is carried
// on the stream itself rather than returned separately, so the calling shape is
// the same whether or not the request succeeded:
//
//	stream := orca.StreamEvents[SessionEvent](ctx, client, path)
//	defer stream.Close()
//	for stream.Next() {
//		event := stream.Current()
//	}
//	if err := stream.Err(); err != nil { ... }
func StreamEvents[T any](
	ctx context.Context,
	client *Client,
	path string,
	opts ...option.RequestOption,
) *ssestream.Stream[T] {
	body, _, err := client.openStream(ctx, path, "text/event-stream", opts...)
	return ssestream.NewStream[T](ctx, body, err)
}

// PostMultipart performs a POST request with a multipart/form-data request body.
func (c *Client) PostMultipart(ctx context.Context, path string, body MultipartRequest, opts ...option.RequestOption) error {
	_, err := c.doMultipart(ctx, http.MethodPost, path, body, false, opts...)
	return err
}

// PutMultipart performs a PUT request with a multipart/form-data request body.
func (c *Client) PutMultipart(ctx context.Context, path string, body MultipartRequest, opts ...option.RequestOption) error {
	_, err := c.doMultipart(ctx, http.MethodPut, path, body, false, opts...)
	return err
}

// PostMultipartWithResponse performs a POST request with a multipart/form-data
// request body and returns the raw response payload.
func (c *Client) PostMultipartWithResponse(ctx context.Context, path string, body MultipartRequest, opts ...option.RequestOption) ([]byte, error) {
	return c.doMultipart(ctx, http.MethodPost, path, body, true, opts...)
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	requestBody interface{},
	responseBody interface{},
	opts ...option.RequestOption,
) error {
	cfg, err := c.cfg.With(opts...)
	if err != nil {
		return err
	}
	endpoint, err := cfg.ResolveURL(path)
	if err != nil {
		return err
	}

	header := http.Header{"Accept": []string{"application/json"}}

	var newBody func() (io.Reader, error)
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return apierror.Errorf("failed to marshal %s payload: %w", method, err)
		}
		header.Set("Content-Type", "application/json")
		// Rebuilt per attempt: a reader is consumed by the first one.
		newBody = func() (io.Reader, error) { return bytes.NewReader(payload), nil }
	}

	resp, err := cfg.Do(ctx, method, endpoint, newBody, header)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBytes, err := readResponseBodyLimited(resp.Body, defaultJSONResponseBodyLimit)
	if err != nil {
		return apierror.Errorf("failed to read %s %s response: %w", method, endpoint, err)
	}

	if !isSuccess(resp.StatusCode) {
		return apierror.FromResponse(method, endpoint, resp.StatusCode, string(responseBytes), resp.Header)
	}

	if responseBody == nil || len(bytes.TrimSpace(responseBytes)) == 0 {
		return nil
	}

	if err := json.Unmarshal(responseBytes, responseBody); err != nil {
		return &apierror.DecodeError{Method: method, URL: endpoint, Err: err}
	}

	return nil
}

func (c *Client) doMultipart(
	ctx context.Context,
	method, path string,
	requestBody MultipartRequest,
	allowEmptyConfig bool,
	opts ...option.RequestOption,
) ([]byte, error) {
	cfg, err := c.cfg.With(opts...)
	if err != nil {
		return nil, err
	}
	endpoint, err := cfg.ResolveURL(path)
	if err != nil {
		return nil, err
	}

	contentType, payload, err := buildMultipartBody(requestBody, allowEmptyConfig)
	if err != nil {
		return nil, err
	}

	accept := strings.TrimSpace(requestBody.Accept)
	if accept == "" {
		accept = "application/json"
	}
	header := http.Header{
		"Accept":       []string{accept},
		"Content-Type": []string{contentType},
	}

	resp, err := cfg.Do(ctx, method, endpoint, func() (io.Reader, error) {
		return bytes.NewReader(payload), nil
	}, header)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBytes, err := readResponseBodyLimited(resp.Body, defaultMultipartBodyLimit)
	if err != nil {
		return nil, apierror.Errorf("failed to read %s %s response: %w", method, endpoint, err)
	}

	if !isSuccess(resp.StatusCode) {
		return nil, apierror.FromResponse(method, endpoint, resp.StatusCode, string(responseBytes), resp.Header)
	}

	return responseBytes, nil
}

// isSuccess reports whether a status is a 2xx.
//
// Only 2xx counts: a 3xx the transport did not follow has no body worth
// decoding, so it surfaces as an error rather than being handed back as if the
// server had answered.
func isSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

// readResponseBodyLimited reads at most limit bytes, failing rather than
// buffering an unbounded response into memory.
func readResponseBodyLimited(body io.Reader, limit int64) ([]byte, error) {
	responseBytes, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(responseBytes)) > limit {
		return nil, apierror.Errorf("response body exceeds limit of %d bytes", limit)
	}

	return responseBytes, nil
}

// resolveURL turns a resource-relative path into an absolute URL.
func (c *Client) resolveURL(path string) (string, error) {
	return c.cfg.ResolveURL(path)
}
