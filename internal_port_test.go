// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Ported from orca-sdk-typescript tests/internal/{headers,path,query,values,
// uploads,to-file}.test.ts.
//
// Those six files unit-test TypeScript's request-plumbing helpers. Go has no
// such layer: the same jobs are done by the standard library (net/url,
// net/http.Header, mime/multipart) driven from client.go. So each test below
// pins the *observable* behaviour of the Go client — the URL it builds, the
// headers it sends, the multipart body it writes — rather than a helper that
// does not exist.
//
// Helpers with no Go counterpart at all are listed at the bottom of this file.

// internalMultipartRequest rebuilds a request from a captured call so the shared
// decodeMultipartRequest helper can parse it.
func internalMultipartRequest(call capturedCall) *http.Request {
	return &http.Request{Header: call.Header, Body: io.NopCloser(bytes.NewReader(call.Body))}
}

// internalPart is one multipart part, including the per-part headers that
// decodeMultipartRequest discards.
type internalPart struct {
	FormName    string
	FileName    string
	ContentType string
	Data        string
}

// internalMultipartParts parses a captured multipart body, keyed by form name.
func internalMultipartParts(t *testing.T, call capturedCall) map[string]internalPart {
	t.Helper()

	_, params, err := mime.ParseMediaType(call.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType(%q) error = %v", call.Header.Get("Content-Type"), err)
	}

	parts := map[string]internalPart{}
	reader := multipart.NewReader(bytes.NewReader(call.Body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		parts[part.FormName()] = internalPart{
			FormName:    part.FormName(),
			FileName:    part.FileName(),
			ContentType: part.Header.Get("Content-Type"),
			Data:        string(data),
		}
		_ = part.Close()
	}
	return parts
}

// internalConfigRequest is the smallest multipart body PostMultipart accepts:
// it insists on a config field.
func internalConfigRequest(file *MultipartFile) MultipartRequest {
	return MultipartRequest{ConfigField: "sinkConfig", Config: map[string]string{"name": "s1"}, File: file}
}

// ---------------------------------------------------------------------------
// headers.test.ts
// ---------------------------------------------------------------------------

func TestDefaultHeadersReachTheRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func(*Client) *Client
		header  string
		want    string
		wantErr bool
	}{
		{
			// TS: 'builds from a plain object'.
			name:   "a default header is sent on every request",
			build:  func(c *Client) *Client { return c.WithDefaultHeader("X-Foo", "bar") },
			header: "X-Foo",
			want:   "bar",
		},
		{
			// TS: 'treats header names case-insensitively'.
			name:   "header names are canonicalised, so lookup is case-insensitive",
			build:  func(c *Client) *Client { return c.WithDefaultHeader("x-foo", "bar") },
			header: "X-FOO",
			want:   "bar",
		},
		{
			// TS: 'object values overwrite (replace) prior entries'.
			name: "a later default replaces an earlier one for the same name",
			build: func(c *Client) *Client {
				return c.WithDefaultHeader("X-Foo", "first").WithDefaultHeader("X-Foo", "second")
			},
			header: "X-Foo",
			want:   "second",
		},
		{
			// TS: 'merges multiple HeadersLike with later entries winning'.
			name: "defaults for different names accumulate",
			build: func(c *Client) *Client {
				return c.WithDefaultHeader("X-One", "1").WithDefaultHeader("X-Two", "2")
			},
			header: "X-Two",
			want:   "2",
		},
		{
			// Unlike the TypeScript merge, a default never overrides a header
			// the request itself already set: doJSON sets Accept first.
			name:   "a default does not override a header the request already set",
			build:  func(c *Client) *Client { return c.WithDefaultHeader("Accept", "text/plain") },
			header: "Accept",
			want:   "application/json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base, transport := newRecordingClient(t, nil)
			var out map[string]any
			if err := tc.build(base).GetJSON(context.Background(), "v1/agents", &out); err != nil {
				t.Fatalf("GetJSON() error = %v", err)
			}
			if got := transport.Only(t).Header.Get(tc.header); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestWithDefaultHeaderReturnsAnIndependentClone(t *testing.T) {
	t.Parallel()

	base, transport := newRecordingClient(t, nil)
	decorated := base.WithDefaultHeader("X-Foo", "bar")

	var out map[string]any
	if err := base.GetJSON(context.Background(), "v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() on the base client error = %v", err)
	}
	if got := transport.Last(t).Header.Get("X-Foo"); got != "" {
		t.Errorf("base client sent X-Foo = %q, want it unset", got)
	}

	if err := decorated.GetJSON(context.Background(), "v1/agents", &out); err != nil {
		t.Fatalf("GetJSON() on the decorated client error = %v", err)
	}
	if got := transport.Last(t).Header.Get("X-Foo"); got != "bar" {
		t.Errorf("decorated client sent X-Foo = %q, want %q", got, "bar")
	}
}

// TestCredentialHeadersAreMutuallyExclusive pins which credential header each
// constructor emits. The two classes are not interchangeable: the server reads
// x-api-key first and only falls back to Bearer when it is absent.
func TestCredentialHeadersAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	t.Run("a bearer client sends Authorization and no api key", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		var out map[string]any
		if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		call := transport.Only(t)
		if got := call.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
		}
		if got := call.Header.Get("x-api-key"); got != "" {
			t.Errorf("x-api-key = %q, want it unset", got)
		}
	})

	t.Run("an api-key client sends x-api-key and no Authorization", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingAPIKeyClient(t, nil)
		var out map[string]any
		if err := client.GetJSON(context.Background(), "v1/agents", &out); err != nil {
			t.Fatalf("GetJSON() error = %v", err)
		}
		call := transport.Only(t)
		if got := call.Header.Get("x-api-key"); got != "orca_test_key" {
			t.Errorf("x-api-key = %q, want %q", got, "orca_test_key")
		}
		if got := call.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want it unset", got)
		}
	})
}

// TS: 'appends multiple values from a single tuple-array source' — buildHeaders
// can produce "x-multi: one, two".
func TestMultiValuedDefaultHeader(t *testing.T) {
	t.Skip("not implemented: WithDefaultHeader uses http.Header.Set, so a default can only " +
		"carry one value — applyDefaultHeaders would forward several, but no public API " +
		"can construct a multi-valued default")
}

// TS: 'null values clear a header and remember the deletion'.
func TestDefaultHeaderDeletion(t *testing.T) {
	t.Skip("no Go analogue: there is no null-header sentinel — Go's http.Header has no " +
		"tri-state (set / unset / explicitly cleared), and WithDefaultHeader cannot remove " +
		"a header the request layer sets")
}

// ---------------------------------------------------------------------------
// path.test.ts
// ---------------------------------------------------------------------------

// TestPathSegmentEncoding ports the `path` template tag tests. The Go analogue is
// url.PathEscape applied at each call site, resolved by Client.resolveURL.
func TestPathSegmentEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		invoke   func(context.Context, *Client) error
		wantPath string
	}{
		{
			// TS: 'encodes a simple substitution'.
			name: "a plain id is substituted verbatim",
			body: `{}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewProvidersClient(c).Get(ctx, "provider-1")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/agents/providers/provider-1",
		},
		{
			// TS: 'encodes special characters' — expects
			// "has%20space%20%26%20slash%2F" from encodeURIComponent.
			// Divergence: url.PathEscape leaves "&" unescaped, because "&" is a
			// legal sub-delimiter inside a path segment. The segment boundary
			// ("/" -> %2F) and the space (-> %20) are still escaped, so the
			// resource identity is preserved; only the byte-for-byte encoding
			// differs from the TypeScript SDK.
			name: "special characters are percent-encoded",
			body: `{}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewProvidersClient(c).Get(ctx, "has space & slash/")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/agents/providers/has%20space%20&%20slash%2F",
		},
		{
			// TS: 'handles multiple substitutions in order'.
			name: "multiple substitutions keep their order",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewPackagesClient(c).ListVersions(ctx, "function", "my pkg")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/packages/function/my%20pkg",
		},
		{
			// TS: 'coerces numeric segments to strings'.
			name: "a numeric segment is rendered as a string",
			body: `{}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewKafkaConnectClient(c).GetTaskStatus(ctx, "conn-1", 42)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connectors/kafka/connectors/conn-1/tasks/42/status",
		},
		{
			// TS: 'returns the static prefix verbatim when there are no
			// substitutions'.
			name: "a static path is sent verbatim",
			body: `[]`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewConnectionsClient(c).List(ctx)
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/connections",
		},
		{
			// Not in the TypeScript suite, but the property that makes the
			// escaping worth testing: a traversal attempt stays one segment.
			name: "a traversal attempt is escaped, not resolved",
			body: `{}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewProvidersClient(c).Get(ctx, "../../etc/passwd")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/agents/providers/..%2F..%2Fetc%2Fpasswd",
		},
		{
			// A "?" in a name must not open a query string.
			name: "a question mark in a name cannot start a query string",
			body: `{}`,
			invoke: func(ctx context.Context, c *Client) error {
				_, err := NewProvidersClient(c).Get(ctx, "a?b")
				return err
			},
			wantPath: "/apis/cloud.sn.io/v1/agents/providers/a%3Fb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.body), nil
			})
			if err := tc.invoke(context.Background(), client); err != nil {
				t.Fatalf("request error = %v", err)
			}
			call := transport.Only(t)
			if got := call.Path(); got != tc.wantPath {
				t.Errorf("path = %q, want %q", got, tc.wantPath)
			}
			if got := call.URL.RawQuery; got != "" {
				t.Errorf("query = %q, want it empty", got)
			}
		})
	}
}

// TestAbsolutePathReplacesTheBaseURL is the Go analogue of values.test.ts'
// isAbsoluteURL cases. TypeScript checks the scheme and refuses to join;
// Go's url.ResolveReference silently lets an absolute path win over the base.
// Call sites must therefore never pass user input as a whole path.
func TestAbsolutePathReplacesTheBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantURL string
	}{
		{
			name:    "an absolute https path replaces the configured host",
			path:    "https://elsewhere.example/v1/agents",
			wantURL: "https://elsewhere.example/v1/agents",
		},
		{
			// Unlike an absolute URL, a scheme-relative one is defused: the
			// leading-slash trim in resolveURL turns "//host/x" into "/host/x",
			// which resolves against the configured host as a path.
			name:    "a scheme-relative path stays on the configured host",
			path:    "//elsewhere.example/v1/agents",
			wantURL: testBaseURL + "/elsewhere.example/v1/agents",
		},
		{
			name:    "a relative path is joined to the base",
			path:    "v1/agents",
			wantURL: testBaseURL + "/v1/agents",
		},
		{
			name:    "a leading slash is trimmed before joining, so the base host root is used",
			path:    "/v1/agents",
			wantURL: testBaseURL + "/v1/agents",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, nil)
			var out map[string]any
			if err := client.GetJSON(context.Background(), tc.path, &out); err != nil {
				t.Fatalf("GetJSON(%q) error = %v", tc.path, err)
			}
			if got := transport.Only(t).URL.String(); got != tc.wantURL {
				t.Errorf("request URL = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

func TestResolveURLRejectsAnEmptyPath(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	for _, path := range []string{"", "   "} {
		var out map[string]any
		if err := client.GetJSON(context.Background(), path, &out); err == nil {
			t.Errorf("GetJSON(%q) error = nil, want an error", path)
		}
	}
	if len(transport.Calls()) != 0 {
		t.Errorf("captured %d requests for rejected paths, want 0", len(transport.Calls()))
	}
}

// ---------------------------------------------------------------------------
// query.test.ts
// ---------------------------------------------------------------------------

// TestQueryStringEncoding ports stringifyQuery. Go builds query strings with
// url.Values.Encode() at the call site and the client passes RawQuery through
// untouched, so the tests assert on what arrives at the transport.
func TestQueryStringEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		values       url.Values
		wantRawQuery string
		wantParsed   map[string][]string
	}{
		{
			// TS: 'returns an empty string for an empty object'.
			name:         "no parameters produce no query string",
			values:       url.Values{},
			wantRawQuery: "",
		},
		{
			// TS: 'encodes a single key'.
			name:         "a single key",
			values:       url.Values{"status": {"active"}},
			wantRawQuery: "status=active",
			wantParsed:   map[string][]string{"status": {"active"}},
		},
		{
			// TS: 'repeats keys for array values'.
			name:         "an array value repeats the key",
			values:       url.Values{"tags": {"a", "b"}},
			wantRawQuery: "tags=a&tags=b",
			wantParsed:   map[string][]string{"tags": {"a", "b"}},
		},
		{
			// TS: 'encodes special characters' — same output, "+" for space.
			name:         "special characters are escaped",
			values:       url.Values{"q": {"hello world & co"}},
			wantRawQuery: "q=hello+world+%26+co",
			wantParsed:   map[string][]string{"q": {"hello world & co"}},
		},
		{
			// TS: 'coerces numbers and booleans to strings'. Go has no coercion:
			// the call site formats them, and Encode() sorts keys.
			name:         "numbers and booleans are formatted by the caller",
			values:       url.Values{"limit": {"10"}, "archived": {"false"}},
			wantRawQuery: "archived=false&limit=10",
			wantParsed:   map[string][]string{"archived": {"false"}, "limit": {"10"}},
		},
		{
			// TS: 'skips null and undefined values entirely'. The Go analogue is
			// an empty value, which is kept, not dropped — url.Values has no null.
			name:         "an empty value is kept, not dropped",
			values:       url.Values{"a": {"x"}, "b": {""}},
			wantRawQuery: "a=x&b=",
			wantParsed:   map[string][]string{"a": {"x"}, "b": {""}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := "v1/agents"
			if encoded := tc.values.Encode(); encoded != "" {
				path += "?" + encoded
			}

			client, transport := newRecordingClient(t, nil)
			var out map[string]any
			if err := client.GetJSON(context.Background(), path, &out); err != nil {
				t.Fatalf("GetJSON(%q) error = %v", path, err)
			}

			call := transport.Only(t)
			if got := call.URL.RawQuery; got != tc.wantRawQuery {
				t.Errorf("RawQuery = %q, want %q", got, tc.wantRawQuery)
			}
			if got := call.Path(); got != "/v1/agents" {
				t.Errorf("path = %q, want /v1/agents", got)
			}
			for key, want := range tc.wantParsed {
				if got := call.Query()[key]; !slices.Equal(got, want) {
					t.Errorf("query[%q] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

// TestOptionalQueryParameterIsOmitted pins the one query string this SDK builds
// itself: omitting the flag must leave the server default in force, which means
// sending no query string at all rather than an explicit default.
func TestOptionalQueryParameterIsOmitted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		connectors   []bool
		wantRawQuery string
	}{
		{name: "omitted flag sends no query string", connectors: nil, wantRawQuery: ""},
		{name: "true is sent explicitly", connectors: []bool{true}, wantRawQuery: "connectorsOnly=true"},
		{name: "false is sent explicitly", connectors: []bool{false}, wantRawQuery: "connectorsOnly=false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `[]`), nil
			})
			if _, err := NewKafkaConnectClient(client).ListPlugins(context.Background(), tc.connectors...); err != nil {
				t.Fatalf("ListPlugins() error = %v", err)
			}

			call := transport.Only(t)
			if got := call.URL.RawQuery; got != tc.wantRawQuery {
				t.Errorf("RawQuery = %q, want %q", got, tc.wantRawQuery)
			}
			if got := call.Path(); got != "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins" {
				t.Errorf("path = %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// values.test.ts
// ---------------------------------------------------------------------------

// TestIsNilMultipartValue covers the one helper in this SDK that plays the role
// of TypeScript's isObj/ensurePresent family: deciding whether an interface
// value is "absent" and should be left out of a multipart body. A typed nil
// pointer is not == nil, so the reflect check is what makes this correct.
func TestIsNilMultipartValue(t *testing.T) {
	t.Parallel()

	type options struct{ Force bool }
	var nilPointer *options
	var nilMap map[string]string
	var nilSlice []string
	var nilFunc func()
	var nilChan chan int
	var nilIface any

	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "an untyped nil is absent", value: nil, want: true},
		{name: "a nil interface is absent", value: nilIface, want: true},
		{name: "a typed nil pointer is absent", value: nilPointer, want: true},
		{name: "a nil map is absent", value: nilMap, want: true},
		{name: "a nil slice is absent", value: nilSlice, want: true},
		{name: "a nil func is absent", value: nilFunc, want: true},
		{name: "a nil channel is absent", value: nilChan, want: true},
		{name: "a non-nil pointer is present", value: &options{}, want: false},
		{name: "a zero struct is present", value: options{}, want: false},
		{name: "a zero int is present", value: 0, want: false},
		{name: "an empty string is present", value: "", want: false},
		{name: "false is present", value: false, want: false},
		{name: "an empty map is present", value: map[string]string{}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isNilMultipartValue(tc.value); got != tc.want {
				t.Errorf("isNilMultipartValue(%#v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// TS: safeJSON — parse if it is JSON, otherwise fall back rather than throw.
// The Go analogue lives in writeManagedAgentSSEEvent and is covered by
// TestManagedAgentSSEDecodesWireFormat's "non-JSON data" case.

// TS: 'validatePositiveInteger'.
func TestValidatePositiveInteger(t *testing.T) {
	t.Skip("no Go analogue: there is no numeric-argument validator — the SDK takes typed " +
		"parameters (for example taskID int) and forwards them without range checks, so " +
		"a negative value would be sent to the server rather than rejected locally")
}

// TS: 'isObj' / 'isEmptyObj' / 'hasOwn' / 'pop' / 'ensurePresent'.
func TestObjectShapeHelpers(t *testing.T) {
	t.Skip("no Go analogue: these guard against untyped JavaScript values at runtime; " +
		"Go's type system decides the same questions at compile time")
}

// ---------------------------------------------------------------------------
// uploads.test.ts
// ---------------------------------------------------------------------------

func TestMultipartFileFieldNamesAndContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		file         *MultipartFile
		wantField    string
		wantFileName string
	}{
		{
			// TS: 'returns a File with the correct name and size'.
			name:         "an explicit field name and file name are used as given",
			file:         &MultipartFile{FieldName: "attachment", FileName: "hello.txt", Content: []byte("hello")},
			wantField:    "attachment",
			wantFileName: "hello.txt",
		},
		{
			// TS: 'uses unknown_file when name is undefined'. Go's fallbacks are
			// the field name "data" and the file name "upload.bin".
			name:         "an empty field name and file name fall back to defaults",
			file:         &MultipartFile{Content: []byte("hello")},
			wantField:    "data",
			wantFileName: "upload.bin",
		},
		{
			name:         "whitespace-only names also fall back to defaults",
			file:         &MultipartFile{FieldName: "  ", FileName: "\t", Content: []byte("hello")},
			wantField:    "data",
			wantFileName: "upload.bin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, nil)
			err := client.PostMultipart(context.Background(), "v1/files", internalConfigRequest(tc.file))
			if err != nil {
				t.Fatalf("PostMultipart() error = %v", err)
			}

			_, files := decodeMultipartRequest(t, internalMultipartRequest(transport.Only(t)))
			content, ok := files[tc.wantField]
			if !ok {
				t.Fatalf("file parts = %v, want one named %q", files, tc.wantField)
			}
			if string(content) != "hello" {
				t.Errorf("content = %q, want %q", content, "hello")
			}

			parts := internalMultipartParts(t, transport.Only(t))
			if got := parts[tc.wantField].FileName; got != tc.wantFileName {
				t.Errorf("filename = %q, want %q", got, tc.wantFileName)
			}
		})
	}
}

// TestMultipartContentTypePerPart ports 'passes options through (type)'.
func TestMultipartContentTypePerPart(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	body := MultipartRequest{
		ConfigField: "sinkConfig",
		Config:      map[string]any{"name": "s1", "parallelism": 2},
		File: &MultipartFile{
			FieldName:   "data",
			FileName:    "data.json",
			ContentType: "application/json",
			Content:     []byte(`{"k":"v"}`),
		},
	}
	if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	parts := internalMultipartParts(t, transport.Only(t))
	if got := parts["data"].ContentType; got != "application/json" {
		t.Errorf("file part Content-Type = %q, want application/json", got)
	}
	if got := parts["data"].FileName; got != "data.json" {
		t.Errorf("file part filename = %q, want data.json", got)
	}
	// TS: 'builds FormData with File and string entries' — the JSON side of a
	// mixed body is sent as an application/json part, not a bare string.
	if got := parts["sinkConfig"].ContentType; got != "application/json" {
		t.Errorf("config part Content-Type = %q, want application/json", got)
	}
	if got := parts["sinkConfig"].Data; got != `{"name":"s1","parallelism":2}` {
		t.Errorf("config part = %q", got)
	}
	if got := parts["sinkConfig"].FileName; got != "" {
		t.Errorf("config part filename = %q, want none", got)
	}
}

// TestMultipartFileWithoutContentTypeOmitsIt records the other branch of
// writeMultipartFileField: with no explicit content type, Go's CreateFormFile
// labels the part application/octet-stream.
func TestMultipartFileWithoutContentTypeOmitsIt(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	err := client.PostMultipart(context.Background(), "v1/files",
		internalConfigRequest(&MultipartFile{FieldName: "data", FileName: "a.bin", Content: []byte("x")}))
	if err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	if got := internalMultipartParts(t, transport.Only(t))["data"].ContentType; got != "application/octet-stream" {
		t.Errorf("file part Content-Type = %q, want application/octet-stream", got)
	}
}

// TestMultipartSanitizesHeaderValues has no TypeScript counterpart — the browser
// FormData API escapes for you. It is the reason sanitizeHeaderValue exists: a
// crafted field or file name must not be able to forge a second part header.
func TestMultipartSanitizesHeaderValues(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	body := internalConfigRequest(&MultipartFile{
		FieldName:   `evil"; name="injected`,
		FileName:    "a\"b\\c\r\nX-Injected: 1",
		ContentType: "application/octet-stream",
		Content:     []byte("payload"),
	})
	if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	call := transport.Only(t)
	parts := internalMultipartParts(t, call)
	if _, forged := parts["injected"]; forged {
		t.Fatalf("a forged part name survived sanitising: %v", parts)
	}
	part, ok := parts[`evil; name=injected`]
	if !ok {
		t.Fatalf("parts = %v, want the quote-stripped field name", parts)
	}
	if got := part.FileName; got != "abcX-Injected: 1" {
		t.Errorf("filename = %q, want quotes, backslashes and CRLF removed", got)
	}
	if bytes.Contains(call.Body, []byte("X-Injected: 1\r\n")) {
		t.Error("the sanitised file name still produced a standalone header line")
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "a plain value is unchanged", input: "report.pdf", want: "report.pdf"},
		{name: "double quotes are removed", input: `a"b`, want: "ab"},
		{name: "backslashes are removed", input: `a\b`, want: "ab"},
		{name: "carriage returns are removed", input: "a\rb", want: "ab"},
		{name: "newlines are removed", input: "a\nb", want: "ab"},
		{name: "a CRLF header injection is flattened", input: "x\r\nX-Evil: 1", want: "xX-Evil: 1"},
		{name: "an empty value stays empty", input: "", want: ""},
		{name: "unicode is preserved", input: "naïve-café.txt", want: "naïve-café.txt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeHeaderValue(tc.input); got != tc.want {
				t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestMultipartAcceptsMultipleFiles ports 'accepts an array of file values for a
// single key'. Go keeps each file under its own field name instead of the
// TypeScript "attachments[]" convention.
func TestMultipartAcceptsMultipleFiles(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	body := MultipartRequest{
		ConfigField: "config",
		Config:      map[string]string{"name": "n"},
		File:        &MultipartFile{FieldName: "primary", FileName: "a.txt", Content: []byte("a")},
		Files: []*MultipartFile{
			{FieldName: "secondary", FileName: "b.txt", Content: []byte("b")},
			nil, // a nil entry is skipped rather than panicking
			{FieldName: "tertiary", FileName: "c.txt", Content: []byte("c")},
		},
	}
	if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	_, files := decodeMultipartRequest(t, internalMultipartRequest(transport.Only(t)))
	want := map[string]string{"primary": "a", "secondary": "b", "tertiary": "c"}
	if len(files) != len(want) {
		t.Fatalf("file parts = %v, want %d of them", files, len(want))
	}
	for field, content := range want {
		if got := string(files[field]); got != content {
			t.Errorf("part %q = %q, want %q", field, got, content)
		}
	}
}

// TestMultipartScalarFields ports 'appends numeric values as strings' and
// 'appends boolean values as strings'. Go's Fields map is already string-typed,
// so the testable behaviour is which entries survive.
func TestMultipartScalarFields(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	body := MultipartRequest{
		ConfigField: "config",
		Config:      map[string]string{"name": "n"},
		URL:         "https://packages.example/pkg.jar",
		Fields: map[string]string{
			"count":   "42",
			"flag":    "true",
			"blank":   "",
			"spaces":  "   ",
			"present": "value",
		},
	}
	if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}

	fields, files := decodeMultipartRequest(t, internalMultipartRequest(transport.Only(t)))
	if len(files) != 0 {
		t.Errorf("file parts = %v, want none", files)
	}
	for name, want := range map[string]string{
		"count":   "42",
		"flag":    "true",
		"present": "value",
		"url":     "https://packages.example/pkg.jar",
	} {
		if got := fields[name]; got != want {
			t.Errorf("field %q = %q, want %q", name, got, want)
		}
	}
	// Blank and whitespace-only fields are dropped, so an unset optional never
	// reaches the server as an empty string.
	for _, name := range []string{"blank", "spaces"} {
		if _, present := fields[name]; present {
			t.Errorf("field %q was sent, want it dropped", name)
		}
	}
}

// TestMultipartJSONFieldsAndUpdateOptions ports 'flattens nested objects' — Go
// does not flatten, it sends one application/json part per structured field.
func TestMultipartJSONFieldsAndUpdateOptions(t *testing.T) {
	t.Parallel()

	type updateOptions struct {
		UpdateAuthData bool `json:"updateAuthData"`
	}

	tests := []struct {
		name          string
		jsonFields    map[string]any
		updateOptions any
		wantParts     map[string]string
		wantAbsent    []string
	}{
		{
			name:       "a nested object is sent as one JSON part",
			jsonFields: map[string]any{"a": map[string]string{"b": "c"}},
			wantParts:  map[string]string{"a": `{"b":"c"}`},
		},
		{
			name:       "a nil JSON field is dropped",
			jsonFields: map[string]any{"present": map[string]string{"b": "c"}, "absent": nil},
			wantParts:  map[string]string{"present": `{"b":"c"}`},
			wantAbsent: []string{"absent"},
		},
		{
			name:          "update options are sent under a fixed field name",
			updateOptions: updateOptions{UpdateAuthData: true},
			wantParts:     map[string]string{"updateOptions": `{"updateAuthData":true}`},
		},
		{
			name:          "a typed nil pointer for update options is dropped",
			updateOptions: (*updateOptions)(nil),
			wantAbsent:    []string{"updateOptions"},
		},
		{
			name:       "an untyped nil for update options is dropped",
			wantAbsent: []string{"updateOptions"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, nil)
			body := MultipartRequest{
				ConfigField:   "config",
				Config:        map[string]string{"name": "n"},
				JSONFields:    tc.jsonFields,
				UpdateOptions: tc.updateOptions,
			}
			if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
				t.Fatalf("PostMultipart() error = %v", err)
			}

			parts := internalMultipartParts(t, transport.Only(t))
			for name, want := range tc.wantParts {
				if got := parts[name].Data; got != want {
					t.Errorf("part %q = %q, want %q", name, got, want)
				}
				if got := parts[name].ContentType; got != "application/json" {
					t.Errorf("part %q Content-Type = %q, want application/json", name, got)
				}
			}
			for _, name := range tc.wantAbsent {
				if _, present := parts[name]; present {
					t.Errorf("part %q was sent, want it dropped", name)
				}
			}
		})
	}
}

// TestMultipartConfigIsRequired ports 'returns opts unchanged when body has no
// uploadables' from the other direction: PostMultipart refuses a body with no
// config rather than silently degrading to an empty form.
func TestMultipartConfigIsRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    MultipartRequest
		wantErr string
	}{
		{
			name:    "no config at all",
			body:    MultipartRequest{File: &MultipartFile{Content: []byte("x")}},
			wantErr: "multipart config field is required",
		},
		{
			name:    "a config payload with no field name",
			body:    MultipartRequest{Config: map[string]string{"a": "b"}},
			wantErr: "multipart config field is required",
		},
		{
			name:    "a field name with no config payload",
			body:    MultipartRequest{ConfigField: "config"},
			wantErr: "multipart config payload is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := newRecordingClient(t, nil)
			err := client.PostMultipart(context.Background(), "v1/files", tc.body)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("PostMultipart() error = %v, want it to contain %q", err, tc.wantErr)
			}
			if len(transport.Calls()) != 0 {
				t.Errorf("captured %d requests for a rejected body, want 0", len(transport.Calls()))
			}
		})
	}
}

// TestMultipartWithResponseAllowsAnEmptyConfig records the one entry point that
// does not require a config field, and that it returns the raw response.
func TestMultipartWithResponseAllowsAnEmptyConfig(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"file-1"}`), nil
	})
	response, err := client.PostMultipartWithResponse(context.Background(), "v1/files", MultipartRequest{
		File: &MultipartFile{FieldName: "file", FileName: "a.txt", Content: []byte("a")},
	})
	if err != nil {
		t.Fatalf("PostMultipartWithResponse() error = %v", err)
	}
	if string(response) != `{"id":"file-1"}` {
		t.Errorf("response = %q", response)
	}

	call := transport.Only(t)
	if !strings.HasPrefix(call.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %q, want a multipart/form-data boundary", call.Header.Get("Content-Type"))
	}
	if got := call.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want the application/json default", got)
	}
}

func TestMultipartAcceptOverride(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	body := internalConfigRequest(&MultipartFile{FieldName: "data", Content: []byte("x")})
	body.Accept = "application/octet-stream"
	if err := client.PostMultipart(context.Background(), "v1/files", body); err != nil {
		t.Fatalf("PostMultipart() error = %v", err)
	}
	if got := transport.Only(t).Header.Get("Accept"); got != "application/octet-stream" {
		t.Errorf("Accept = %q, want the override", got)
	}
}

// ---------------------------------------------------------------------------
// to-file.test.ts
// ---------------------------------------------------------------------------

// TestMultipartFileFromPath is the Go analogue of toFile(). TypeScript accepts
// Blobs, Responses, async iterables and promises; Go's only source is a path on
// disk, read eagerly into memory.
func TestMultipartFileFromPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "original.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Run("reads the content and names the part after the file", func(t *testing.T) {
		t.Parallel()

		file, err := multipartFileFromPath(path)
		if err != nil {
			t.Fatalf("multipartFileFromPath() error = %v", err)
		}
		if file.FieldName != "data" {
			t.Errorf("FieldName = %q, want the %q default", file.FieldName, "data")
		}
		// TS: 'returns a File with the same content when no overrides given'.
		if file.FileName != "original.txt" {
			t.Errorf("FileName = %q, want the base name", file.FileName)
		}
		if string(file.Content) != "data" {
			t.Errorf("Content = %q, want %q", file.Content, "data")
		}
		if file.ContentType != "" {
			// TS: 'infers MIME type from Blob parts'. Go infers nothing.
			t.Errorf("ContentType = %q, want it unset — Go does not sniff a type", file.ContentType)
		}
	})

	t.Run("honours a field name override", func(t *testing.T) {
		t.Parallel()

		// TS: 'honors the name override' — Go overrides the *field* name only;
		// the file name always comes from the path.
		file, err := multipartFileFromPathWithField(path, "attachment")
		if err != nil {
			t.Fatalf("multipartFileFromPathWithField() error = %v", err)
		}
		if file.FieldName != "attachment" {
			t.Errorf("FieldName = %q, want %q", file.FieldName, "attachment")
		}
		if file.FileName != "original.txt" {
			t.Errorf("FileName = %q, want the base name", file.FileName)
		}
	})

	t.Run("an empty path yields no file and no error", func(t *testing.T) {
		t.Parallel()

		file, err := multipartFileFromPath("")
		if err != nil {
			t.Fatalf("multipartFileFromPath(\"\") error = %v", err)
		}
		if file != nil {
			t.Errorf("file = %#v, want nil for an omitted upload", file)
		}
	})

	t.Run("a missing path is an error", func(t *testing.T) {
		t.Parallel()

		// TS: 'throws a clear error for an unsupported plain-object input'.
		if _, err := multipartFileFromPath(filepath.Join(dir, "absent.txt")); err == nil {
			t.Error("multipartFileFromPath() error = nil, want a read error")
		}
	})

	t.Run("an empty file still produces a part", func(t *testing.T) {
		t.Parallel()

		emptyPath := filepath.Join(dir, "empty.bin")
		if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		file, err := multipartFileFromPath(emptyPath)
		if err != nil {
			t.Fatalf("multipartFileFromPath() error = %v", err)
		}
		if file == nil || len(file.Content) != 0 || file.FileName != "empty.bin" {
			t.Errorf("file = %#v, want a named part with empty content", file)
		}
	})
}

// TS: toFile() accepting a Blob, a Response, an async iterable or a PromiseLike.
func TestToFileFromStreamingSources(t *testing.T) {
	t.Skip("no Go analogue: uploads are []byte held in MultipartFile.Content, so there is " +
		"nothing to convert from a Blob/Response/async iterable — and no streaming upload " +
		"path either, since doMultipart buffers the whole body before sending")
}
