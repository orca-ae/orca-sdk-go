// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
)

const (
	// Function triggers may synchronously wait up to 60 seconds for an output
	// message. Keep the default client deadline above that server-side contract.
	defaultHTTPTimeout = 90 * time.Second

	defaultMultipartFieldName    = "data"
	defaultMultipartFileName     = "upload.bin"
	defaultJSONResponseBodyLimit = 10 << 20
	defaultMultipartBodyLimit    = 10 << 20
)

// Client is the base registry HTTP client.
type Client struct {
	baseURL        *url.URL
	httpClient     *http.Client
	token          string
	defaultHeaders http.Header
	pathPrefix     string
}

// MultipartFile represents one file part in a multipart registry request.
type MultipartFile struct {
	FieldName   string
	FileName    string
	ContentType string
	Content     []byte
}

// MultipartRequest represents a registry multipart/form-data request.
type MultipartRequest struct {
	Accept        string
	File          *MultipartFile
	Files         []*MultipartFile
	URL           string
	Fields        map[string]string
	JSONFields    map[string]interface{}
	ConfigField   string
	Config        interface{}
	UpdateOptions interface{}
}

// legacyBaseURLSuffixes are base-URL path suffixes that used to be required and are now stripped
// with a deprecation warning. Order matters: /api/v1 and /v1/registry must be checked before the
// bare /v1, because both of them also end in "/v1" — checking the longer, more specific suffix
// first is what makes the whole suffix get stripped instead of just its last segment. Leaving
// ".../api" behind (by stripping only the trailing "/v1" of ".../api/v1") would produce a base
// where core calls still work via the /api/v1 alias but extension calls under /apis/... 404 on an
// undocumented path - a partial failure that is far more expensive to debug than a clean strip.
var legacyBaseURLSuffixes = []string{"/api/v1", "/v1/registry", "/v1"}

// stripLegacyBaseURLSuffix removes a trailing legacy suffix from a base URL path, if present. The
// match is anchored to the end of the string (via strings.HasSuffix), so a path like
// "/v1/registry-proxy" is left untouched: it does not end in exactly "/v1/registry" or "/v1".
func stripLegacyBaseURLSuffix(path string) (stripped string, matchedSuffix string) {
	trimmed := strings.TrimRight(path, "/")
	for _, suffix := range legacyBaseURLSuffixes {
		if strings.HasSuffix(trimmed, suffix) {
			return strings.TrimSuffix(trimmed, suffix), suffix
		}
	}
	return path, ""
}

// warnLegacyBaseURL writes a deprecation warning to the caller's diagnostic stream - never
// stdout, which this CLI treats as parseable output - when a base URL needed the legacy-suffix
// shim above.
func warnLegacyBaseURL(writer io.Writer, originalBaseURL, matchedSuffix, strippedBaseURL string) {
	fmt.Fprintf(writer,
		"warning: registry base URL %q ends with %q, which is no longer part of the base URL — "+
			"every deployment now serves core at the host root (for example, GET {base}/v1/agents). "+
			"Using %q instead. Update --registry-url / ORCA_REGISTRY_URL to the host root; this "+
			"compatibility shim may be removed in a future version.\n",
		originalBaseURL, matchedSuffix, strippedBaseURL)
}

// NewClient creates a registry client using the provided base URL and bearer token. The base URL
// is the deployment host root: every request path supplied to the client's methods must carry its
// own full prefix (v1/... for core, apis/cloud.sn.io/v1/... for StreamNative Cloud extensions). A
// base URL ending in the legacy /api/v1, /v1/registry, or /v1 suffix is accepted for backward
// compatibility - it is stripped, and a deprecation warning is printed to stderr.
func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	return NewClientWithWarningWriter(baseURL, token, httpClient, os.Stderr)
}

// NewUnauthenticatedClient creates a registry client for endpoints whose OpenAPI security is
// explicitly empty, such as /healthz and /readyz.
func NewUnauthenticatedClient(baseURL string, httpClient *http.Client) (*Client, error) {
	return NewUnauthenticatedClientWithWarningWriter(baseURL, httpClient, os.Stderr)
}

// NewClientWithWarningWriter creates a registry client and writes legacy base-URL diagnostics to
// warningWriter. It is intended for command hosts that provide their own stderr stream.
func NewClientWithWarningWriter(baseURL, token string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, apierror.Validationf("registry access token is required")
	}
	if warningWriter == nil {
		warningWriter = os.Stderr
	}
	return newClient(baseURL, token, httpClient, warningWriter)
}

// NewAPIKeyClient creates a registry client that authenticates with a Managed Agents workspace API key.
func NewAPIKeyClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	return NewAPIKeyClientWithWarningWriter(baseURL, apiKey, httpClient, os.Stderr)
}

// NewAPIKeyClientWithWarningWriter creates an API-key client and writes legacy base-URL diagnostics
// to warningWriter.
func NewAPIKeyClientWithWarningWriter(baseURL, apiKey string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, apierror.Validationf("registry API key is required")
	}
	if warningWriter == nil {
		warningWriter = os.Stderr
	}
	client, err := newClient(baseURL, "", httpClient, warningWriter)
	if err != nil {
		return nil, err
	}
	return client.WithDefaultHeader("x-api-key", apiKey), nil
}

// NewUnauthenticatedClientWithWarningWriter creates an unauthenticated registry client and writes
// legacy base-URL diagnostics to warningWriter.
func NewUnauthenticatedClientWithWarningWriter(baseURL string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if warningWriter == nil {
		warningWriter = os.Stderr
	}
	return newClient(baseURL, "", httpClient, warningWriter)
}

func newClient(baseURL, token string, httpClient *http.Client, warningWriter io.Writer) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, apierror.Validationf("registry base URL is required")
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, apierror.Validationf("invalid registry base URL %q: %w", baseURL, err)
	}

	if stripped, matchedSuffix := stripLegacyBaseURLSuffix(parsedBaseURL.Path); matchedSuffix != "" {
		parsedBaseURL.Path = stripped
		warnLegacyBaseURL(warningWriter, baseURL, matchedSuffix, parsedBaseURL.String())
	}
	if !strings.HasSuffix(parsedBaseURL.Path, "/") {
		parsedBaseURL.Path += "/"
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &Client{
		baseURL:    parsedBaseURL,
		httpClient: httpClient,
		token:      token,
	}, nil
}

// WithDefaultHeader returns a client clone that adds a default header unless
// the request already sets that header.
func (c *Client) WithDefaultHeader(name, value string) *Client {
	clone := *c
	clone.defaultHeaders = c.defaultHeaders.Clone()
	if clone.defaultHeaders == nil {
		clone.defaultHeaders = http.Header{}
	}
	clone.defaultHeaders.Set(name, value)
	return &clone
}

// WithPathPrefix returns a client clone that resolves every path relative to prefix instead of the
// base URL directly. Used to scope a client to one API group (for example
// "apis/cloud.sn.io/v1") so its resource methods keep passing short, resource-relative paths
// ("/connections") exactly as they did before the base URL became the host root.
func (c *Client) WithPathPrefix(prefix string) *Client {
	clone := *c
	trimmed := strings.Trim(prefix, "/")
	if trimmed != "" {
		trimmed += "/"
	}
	clone.pathPrefix = trimmed
	return &clone
}

// GetJSON performs a GET request and decodes the JSON response body.
func (c *Client) GetJSON(ctx context.Context, path string, out interface{}) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}

// GetToWriter performs a GET request and streams the raw response body to writer.
func (c *Client) GetToWriter(ctx context.Context, path string, writer io.Writer) error {
	if writer == nil {
		return apierror.Validationf("response writer is required")
	}

	return c.GetStream(ctx, path, "*/*", func(reader io.Reader) error {
		if _, err := io.Copy(writer, reader); err != nil {
			return apierror.Errorf("failed to stream response: %w", err)
		}
		return nil
	})
}

// GetStream performs a GET request and lets handle consume the raw response body.
func (c *Client) GetStream(ctx context.Context, path string, accept string, handle func(io.Reader) error) error {
	if handle == nil {
		return apierror.Validationf("response handler is required")
	}

	endpoint, err := c.resolveURL(path)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return apierror.Errorf("failed to create %s request: %w", http.MethodGet, err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if strings.TrimSpace(accept) == "" {
		accept = "*/*"
	}
	req.Header.Set("Accept", accept)
	c.applyDefaultHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apierror.FromTransport(ctx, http.MethodGet, endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBytes, readErr := readResponseBodyLimited(resp.Body, defaultJSONResponseBodyLimit)
		if readErr != nil {
			return apierror.Errorf("failed to read %s %s response: %w", http.MethodGet, endpoint, readErr)
		}
		return apierror.FromResponse(http.MethodGet, endpoint, resp.StatusCode, string(responseBytes), resp.Header)
	}

	if err := handle(resp.Body); err != nil {
		return apierror.Errorf("failed to stream %s %s response: %w", http.MethodGet, endpoint, err)
	}

	return nil
}

// PostJSON performs a POST request with a JSON request body.
func (c *Client) PostJSON(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}

// PutJSON performs a PUT request with a JSON request body.
func (c *Client) PutJSON(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.doJSON(ctx, http.MethodPut, path, body, out)
}

// PatchJSON performs a PATCH request with a JSON request body.
func (c *Client) PatchJSON(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.doJSON(ctx, http.MethodPatch, path, body, out)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// PostMultipart performs a POST request with a multipart/form-data request body.
func (c *Client) PostMultipart(ctx context.Context, path string, body MultipartRequest) error {
	_, err := c.doMultipart(ctx, http.MethodPost, path, body, false)
	return err
}

// PutMultipart performs a PUT request with a multipart/form-data request body.
func (c *Client) PutMultipart(ctx context.Context, path string, body MultipartRequest) error {
	_, err := c.doMultipart(ctx, http.MethodPut, path, body, false)
	return err
}

// PostMultipartWithResponse performs a POST request with a multipart/form-data
// request body and returns the raw response payload.
func (c *Client) PostMultipartWithResponse(ctx context.Context, path string, body MultipartRequest) ([]byte, error) {
	return c.doMultipart(ctx, http.MethodPost, path, body, true)
}

func (c *Client) doJSON(ctx context.Context, method, path string, requestBody interface{}, responseBody interface{}) error {
	endpoint, err := c.resolveURL(path)
	if err != nil {
		return err
	}

	var bodyReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return apierror.Errorf("failed to marshal %s payload: %w", method, err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return apierror.Errorf("failed to create %s request: %w", method, err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyDefaultHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apierror.FromTransport(ctx, method, endpoint, err)
	}
	defer resp.Body.Close()

	responseBytes, err := readResponseBodyLimited(resp.Body, defaultJSONResponseBodyLimit)
	if err != nil {
		return apierror.Errorf("failed to read %s %s response: %w", method, endpoint, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
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

func (c *Client) doMultipart(ctx context.Context, method, path string, requestBody MultipartRequest, allowEmptyConfig bool) ([]byte, error) {
	endpoint, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}

	hasConfig := strings.TrimSpace(requestBody.ConfigField) != "" || requestBody.Config != nil
	if !allowEmptyConfig && !hasConfig {
		return nil, apierror.Validationf("multipart config field is required")
	}
	if hasConfig {
		if strings.TrimSpace(requestBody.ConfigField) == "" {
			return nil, apierror.Validationf("multipart config field is required")
		}
		if requestBody.Config == nil {
			return nil, apierror.Validationf("multipart config payload is required")
		}
	}

	bodyBuf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(bodyBuf)

	files := make([]*MultipartFile, 0, 1+len(requestBody.Files))
	if requestBody.File != nil {
		files = append(files, requestBody.File)
	}
	files = append(files, requestBody.Files...)
	for _, file := range files {
		if file == nil {
			continue
		}
		if err := writeMultipartFileField(writer, file); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(requestBody.URL) != "" {
		if err := writer.WriteField("url", requestBody.URL); err != nil {
			return nil, apierror.Errorf("failed to write multipart url field: %w", err)
		}
	}

	if hasConfig {
		if err := writeMultipartJSONField(writer, requestBody.ConfigField, requestBody.Config); err != nil {
			return nil, err
		}
	}

	for fieldName, fieldValue := range requestBody.Fields {
		if strings.TrimSpace(fieldValue) == "" {
			continue
		}
		if err := writer.WriteField(fieldName, fieldValue); err != nil {
			return nil, apierror.Errorf("failed to write multipart %s field: %w", fieldName, err)
		}
	}

	for fieldName, fieldValue := range requestBody.JSONFields {
		if fieldValue == nil {
			continue
		}
		if err := writeMultipartJSONField(writer, fieldName, fieldValue); err != nil {
			return nil, err
		}
	}

	if !isNilMultipartValue(requestBody.UpdateOptions) {
		if err := writeMultipartJSONField(writer, "updateOptions", requestBody.UpdateOptions); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, apierror.Errorf("failed to finalize multipart request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyBuf)
	if err != nil {
		return nil, apierror.Errorf("failed to create %s request: %w", method, err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	accept := strings.TrimSpace(requestBody.Accept)
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.applyDefaultHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apierror.FromTransport(ctx, method, endpoint, err)
	}
	defer resp.Body.Close()

	responseBytes, err := readResponseBodyLimited(resp.Body, defaultMultipartBodyLimit)
	if err != nil {
		return nil, apierror.Errorf("failed to read %s %s response: %w", method, endpoint, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apierror.FromResponse(method, endpoint, resp.StatusCode, string(responseBytes), resp.Header)
	}

	return responseBytes, nil
}

func (c *Client) applyDefaultHeaders(req *http.Request) {
	for name, values := range c.defaultHeaders {
		if len(req.Header[http.CanonicalHeaderKey(name)]) > 0 {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
}

func isNilMultipartValue(value interface{}) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

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

func writeMultipartFileField(writer *multipart.Writer, file *MultipartFile) error {
	fieldName := file.FieldName
	if strings.TrimSpace(fieldName) == "" {
		fieldName = defaultMultipartFieldName
	}
	fileName := file.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = defaultMultipartFileName
	}

	if strings.TrimSpace(file.ContentType) == "" {
		part, err := writer.CreateFormFile(fieldName, fileName)
		if err != nil {
			return apierror.Errorf("failed to create multipart file part: %w", err)
		}
		if _, err := part.Write(file.Content); err != nil {
			return apierror.Errorf("failed to write multipart file part: %w", err)
		}
		return nil
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			sanitizeHeaderValue(fieldName), sanitizeHeaderValue(fileName)))
	header.Set("Content-Type", file.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return apierror.Errorf("failed to create multipart file part: %w", err)
	}
	if _, err := part.Write(file.Content); err != nil {
		return apierror.Errorf("failed to write multipart file part: %w", err)
	}
	return nil
}

// sanitizeHeaderValue removes characters that could cause header injection
// (double quotes, backslashes, carriage returns, newlines).
func sanitizeHeaderValue(s string) string {
	r := strings.NewReplacer(`"`, "", `\`, "", "\r", "", "\n", "")
	return r.Replace(s)
}

func writeMultipartJSONField(writer *multipart.Writer, fieldName string, value interface{}) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, fieldName))
	header.Set("Content-Type", "application/json")

	part, err := writer.CreatePart(header)
	if err != nil {
		return apierror.Errorf("failed to create multipart %s field: %w", fieldName, err)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return apierror.Errorf("failed to marshal multipart %s payload: %w", fieldName, err)
	}

	if _, err := part.Write(payload); err != nil {
		return apierror.Errorf("failed to write multipart %s payload: %w", fieldName, err)
	}

	return nil
}

func (c *Client) resolveURL(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", apierror.Validationf("registry path is required")
	}

	relativePath := c.pathPrefix + strings.TrimPrefix(path, "/")
	relativeURL, err := url.Parse(relativePath)
	if err != nil {
		return "", apierror.Validationf("invalid registry path %q: %w", path, err)
	}

	return c.baseURL.ResolveReference(relativeURL).String(), nil
}
