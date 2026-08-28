// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

// Ported from orca-sdk-typescript tests/client.test.ts,
// describe('ensureExtensionAvailable'), and from the shared
// assertCloudExtensionUnavailable helper in
// tests/api-resources/cloud/helpers.ts.
//
// Every cloud extension call is gated: before a resource request goes out the
// client resolves GET /apis once, caches the result, and raises
// ExtensionNotAvailableError when the cloud.sn.io group is not advertised. A
// deployment without the extension therefore produces a clear diagnosis
// instead of a bare 404 from a path the caller has no reason to doubt.

// gatingTransport answers /apis with a scripted response and every other path
// with an empty success, counting each so a test can prove how many probes
// happened.
type gatingTransport struct {
	mu        sync.Mutex
	apisCalls int
	other     []string
	respond   func(int) (*http.Response, error)
}

func (g *gatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	g.mu.Lock()
	if req.URL.Path == "/apis" {
		g.apisCalls++
		n := g.apisCalls
		g.mu.Unlock()
		return g.respond(n)
	}
	g.other = append(g.other, req.URL.Path)
	g.mu.Unlock()
	return jsonResponse(http.StatusOK, `[]`), nil
}

func (g *gatingTransport) probes() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.apisCalls
}

func (g *gatingTransport) resourceCalls() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.other...)
}

// newGatedClient builds a client whose /apis probe is scripted by respond, and
// returns the warning writer's buffer so a test can assert on diagnostics.
func newGatedClient(tb testing.TB, respond func(int) (*http.Response, error)) (*Client, *gatingTransport, *bytes.Buffer) {
	tb.Helper()
	transport := &gatingTransport{respond: respond}
	warnings := &bytes.Buffer{}
	client, err := New(
		option.WithBaseURL(testBaseURL),
		option.WithAuthToken("test-key"),
		option.WithHTTPClient(&http.Client{Transport: transport}),
		option.WithWarningWriter(warnings),
		option.WithMaxRetries(0),
	)
	if err != nil {
		tb.Fatalf("New() error = %v", err)
	}
	return client, transport, warnings
}

func apisBody(groups ...string) string {
	entries := make([]string, 0, len(groups))
	for _, group := range groups {
		entries = append(entries, fmt.Sprintf(`{"name":%q,"versions":[]}`, group))
	}
	return fmt.Sprintf(`{"kind":"APIGroupList","groups":[%s]}`, strings.Join(entries, ","))
}

func TestCloudExtensionGating(t *testing.T) {
	t.Parallel()

	// The three ways a deployment can decline to serve the extension. All of
	// them have to arrive as the same distinct error, or a caller cannot tell
	// "this deployment cannot do that" from "that resource does not exist".
	unavailable := []struct {
		name       string
		respond    func(int) (*http.Response, error)
		wantReason string
	}{
		{
			name: "groups is empty",
			respond: func(int) (*http.Response, error) {
				return jsonResponse(http.StatusOK, apisBody()), nil
			},
			wantReason: "advertises no extension groups",
		},
		{
			name: "groups exist but do not include the target",
			respond: func(int) (*http.Response, error) {
				return jsonResponse(http.StatusOK, apisBody("some.other.group")), nil
			},
			wantReason: "some.other.group",
		},
		{
			name: "/apis itself 404s",
			respond: func(int) (*http.Response, error) {
				return jsonResponse(http.StatusNotFound, `{}`), nil
			},
			wantReason: "does not serve an extension discovery endpoint",
		},
	}

	for _, tc := range unavailable {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport, _ := newGatedClient(t, tc.respond)

			_, err := client.Cloud.Connections.List(context.Background())

			var unavailableErr *ExtensionNotAvailableError
			if !errors.As(err, &unavailableErr) {
				t.Fatalf("error = %v (%T), want *ExtensionNotAvailableError", err, err)
			}
			if unavailableErr.Group != CloudExtensionGroup {
				t.Errorf("Group = %q, want %q", unavailableErr.Group, CloudExtensionGroup)
			}
			if !strings.Contains(unavailableErr.Reason, tc.wantReason) {
				t.Errorf("Reason = %q, want it to mention %q", unavailableErr.Reason, tc.wantReason)
			}

			// A 404 from /apis must not reach the caller as a NotFoundError:
			// that reads as a missing resource, which is a different problem
			// with a different fix.
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				t.Error("error is also a *NotFoundError, want the extension error to replace it")
			}

			// The resource request must never have gone out.
			if got := transport.resourceCalls(); len(got) != 0 {
				t.Errorf("resource requests = %v, want none - the gate runs first", got)
			}
		})
	}

	t.Run("a 404 from /apis warns that the deployment predates discovery", func(t *testing.T) {
		t.Parallel()

		client, _, warnings := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, `{}`), nil
		})

		_, _ = client.Cloud.Connections.List(context.Background())

		if !strings.Contains(warnings.String(), "predates extension discovery") {
			t.Errorf("warnings = %q, want a version hint", warnings.String())
		}
	})

	t.Run("an empty groups array does not warn", func(t *testing.T) {
		t.Parallel()

		// A deployment with no extensions installed is normal and fully
		// functional. Warning about it would train callers to ignore warnings.
		client, _, warnings := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody()), nil
		})

		_, _ = client.Cloud.Connections.List(context.Background())

		if warnings.Len() != 0 {
			t.Errorf("warnings = %q, want none", warnings.String())
		}
	})

	t.Run("a successful discovery is cached across calls", func(t *testing.T) {
		t.Parallel()

		client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		ctx := context.Background()
		for range 3 {
			if _, err := client.Cloud.Connections.List(ctx); err != nil {
				t.Fatalf("List() error = %v", err)
			}
		}

		if got := transport.probes(); got != 1 {
			t.Errorf("GET /apis requests = %d, want 1 - the result is cached", got)
		}
		if got := len(transport.resourceCalls()); got != 3 {
			t.Errorf("resource requests = %d, want 3", got)
		}
	})

	t.Run("the cache is shared across scoped clones", func(t *testing.T) {
		t.Parallel()

		// Cloning a client to add a header or scope a path is talking to the
		// same deployment, so it must not restart the probe.
		client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		ctx := context.Background()
		if _, err := client.Cloud.Connections.List(ctx); err != nil {
			t.Fatalf("List() error = %v", err)
		}
		scoped := client.WithDefaultHeader("X-Tenant-Id", "t1")
		if _, err := scoped.Cloud.Connections.List(ctx); err != nil {
			t.Fatalf("List() on clone error = %v", err)
		}

		if got := transport.probes(); got != 1 {
			t.Errorf("GET /apis requests = %d, want 1", got)
		}
	})

	t.Run("concurrent callers share one in-flight probe", func(t *testing.T) {
		t.Parallel()

		// Without single-flighting, a program that fans out cloud calls at
		// startup sends one discovery request per goroutine.
		release := make(chan struct{})
		client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
			<-release
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		ctx := context.Background()
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = client.Cloud.Connections.List(ctx)
			}()
		}
		// Give every goroutine time to reach the gate before the probe answers.
		time.Sleep(20 * time.Millisecond)
		close(release)
		wg.Wait()

		if got := transport.probes(); got != 1 {
			t.Errorf("GET /apis requests = %d, want 1 for 8 concurrent callers", got)
		}
	})

	t.Run("a failed discovery is not cached", func(t *testing.T) {
		t.Parallel()

		// A probe that failed says nothing about the deployment. Remembering it
		// would turn one bad moment into a permanent refusal to make any cloud
		// call for the life of the process.
		client, transport, _ := newGatedClient(t, func(n int) (*http.Response, error) {
			if n == 1 {
				return nil, errors.New("dial tcp: connection refused")
			}
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		ctx := context.Background()
		if _, err := client.Cloud.Connections.List(ctx); err == nil {
			t.Fatal("first List() error = nil, want the transport failure")
		}
		if _, err := client.Cloud.Connections.List(ctx); err != nil {
			t.Fatalf("second List() error = %v, want the retry to succeed", err)
		}
		if got := transport.probes(); got != 2 {
			t.Errorf("GET /apis requests = %d, want 2 - the failure must not be cached", got)
		}
	})

	t.Run("every cloud resource is gated", func(t *testing.T) {
		t.Parallel()
		assertServiceGated(t, "Cloud", func(c *Client) any { return c.Cloud })
	})
}

// TestCloudExtensionDiscoveryPrimitives covers the part of the specification
// this SDK does implement: the GET /apis probe a caller-side gate is built out
// of. These are the three ensureExtensionAvailable cases that survive the
// translation, expressed against the primitives rather than against a gate.
func TestCloudExtensionDiscoveryPrimitives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		status    int
		available bool
		wantErr   bool
	}{
		{
			name:      "reports available when GET /apis advertises the group",
			body:      `{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io","versions":[]}]}`,
			available: true,
		},
		{
			name:      "reports unavailable, and not an error, when groups is empty",
			body:      `{"kind":"APIGroupList","groups":[]}`,
			available: false,
		},
		{
			name:      "reports unavailable when groups exist but do not include the target",
			body:      `{"kind":"APIGroupList","groups":[{"name":"some.other.group","versions":[]}]}`,
			available: false,
		},
		{
			name:    "surfaces the transport error when /apis itself 404s",
			body:    `{}`,
			status:  http.StatusNotFound,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(status, tc.body), nil
			})

			groups, err := client.GetAPIGroups(context.Background())
			if tc.wantErr {
				var httpErr *HTTPError
				if !errors.As(err, &httpErr) {
					t.Fatalf("error = %v (%T), want an *HTTPError", err, err)
				}
				// Recorded, not endorsed: the TypeScript contract says this
				// must not reach the caller as a 404. See the skipped case above.
				if httpErr.StatusCode != http.StatusNotFound {
					t.Errorf("status = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetAPIGroups() error = %v", err)
			}
			if got := groups.HasGroup("cloud.sn.io"); got != tc.available {
				t.Errorf("HasGroup(%q) = %t, want %t", "cloud.sn.io", got, tc.available)
			}
			if got := transport.Only(t).Path(); got != "/apis" {
				t.Errorf("path = %q, want /apis at the host root", got)
			}
		})
	}
}

// TestCloudExtensionDiscoveryPrimitiveIsUncached pins that the raw probe stays
// uncached. GetAPIGroups is the primitive a caller uses to ask the deployment
// directly; the cache belongs to the gate, so a caller polling for an extension
// that is being installed sees it appear.
func TestCloudExtensionDiscoveryPrimitiveIsUncached(t *testing.T) {
	t.Parallel()

	client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
		return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
	})

	ctx := context.Background()
	for range 3 {
		if _, err := client.GetAPIGroups(ctx); err != nil {
			t.Fatalf("GetAPIGroups() error = %v", err)
		}
	}
	if got := transport.probes(); got != 3 {
		t.Errorf("GET /apis requests = %d, want 3 - the primitive asks every time", got)
	}
}

// assertServiceGated calls every operation reachable from a cloud service and
// requires each to refuse with [ExtensionNotAvailableError] before sending a
// request.
//
// It walks the tree by reflection rather than listing the operations, because a
// list goes stale the moment someone adds a method - and an ungated method is a
// hole that only shows up on a deployment without the extension, which is
// exactly where nobody is looking.
func assertServiceGated(t *testing.T, path string, pick func(*Client) any) {
	t.Helper()

	client, _, _ := newGatedClient(t, func(int) (*http.Response, error) {
		return jsonResponse(http.StatusOK, apisBody()), nil
	})
	walkServiceMethods(t, path, reflect.ValueOf(pick(client)))
}

func walkServiceMethods(t *testing.T, path string, service reflect.Value) {
	t.Helper()

	// Sub-services are exported struct fields; operations are methods.
	if service.Kind() == reflect.Struct {
		for i := range service.NumField() {
			field := service.Type().Field(i)
			if !field.IsExported() || field.Type.Kind() != reflect.Struct {
				continue
			}
			walkServiceMethods(t, path+"."+field.Name, service.Field(i))
		}
	}

	for i := range service.NumMethod() {
		method := service.Type().Method(i)
		if !method.IsExported() {
			continue
		}
		name := path + "." + method.Name
		t.Run(name, func(t *testing.T) {
			results := callWithZeroArgs(t, service.Method(i))
			if len(results) == 0 {
				t.Fatalf("%s returned nothing, want an error", name)
			}
			last := results[len(results)-1].Interface()
			err, _ := last.(error)
			if err == nil {
				t.Fatalf("%s error = nil, want *ExtensionNotAvailableError", name)
			}
			var unavailable *ExtensionNotAvailableError
			if !errors.As(err, &unavailable) {
				t.Errorf("%s error = %v (%T), want *ExtensionNotAvailableError", name, err, err)
			}
		})
	}
}

// callWithZeroArgs invokes fn with a background context and the zero value of
// every other parameter. The gate runs before any argument is read, so the
// values only have to type-check.
func callWithZeroArgs(t *testing.T, fn reflect.Value) []reflect.Value {
	t.Helper()

	fnType := fn.Type()
	args := make([]reflect.Value, 0, fnType.NumIn())
	for i := range fnType.NumIn() {
		in := fnType.In(i)
		if fnType.IsVariadic() && i == fnType.NumIn()-1 {
			break
		}
		if in == reflect.TypeOf((*context.Context)(nil)).Elem() {
			args = append(args, reflect.ValueOf(context.Background()))
			continue
		}
		if in.Kind() == reflect.Interface && in.NumMethod() > 0 {
			// An io.Writer parameter cannot be a nil interface value.
			if in.Implements(reflect.TypeOf((*io.Writer)(nil)).Elem()) ||
				reflect.TypeOf(io.Discard).Implements(in) {
				args = append(args, reflect.ValueOf(io.Discard))
				continue
			}
		}
		args = append(args, reflect.Zero(in))
	}
	return fn.Call(args)
}

// TestCloudEnsureAvailable covers the exported gate.
//
// Every cloud operation already checks, so this exists only for callers that
// want to fail before starting work rather than partway through - and the point
// is that asking first costs nothing, because it shares the same cached probe.
func TestCloudEnsureAvailable(t *testing.T) {
	t.Parallel()

	t.Run("reports the extension missing without sending a resource request", func(t *testing.T) {
		t.Parallel()

		client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody()), nil
		})

		err := client.Cloud.EnsureAvailable(context.Background())

		var unavailable *ExtensionNotAvailableError
		if !errors.As(err, &unavailable) {
			t.Fatalf("error = %v (%T), want *ExtensionNotAvailableError", err, err)
		}
		if got := transport.resourceCalls(); len(got) != 0 {
			t.Errorf("resource requests = %v, want none", got)
		}
	})

	t.Run("passes when the deployment advertises the group", func(t *testing.T) {
		t.Parallel()

		client, _, _ := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		if err := client.Cloud.EnsureAvailable(context.Background()); err != nil {
			t.Errorf("EnsureAvailable() error = %v, want nil", err)
		}
	})

	t.Run("shares its probe with the operations that follow", func(t *testing.T) {
		t.Parallel()

		// Checking first must not cost an extra round trip, or a short-lived
		// process pays for the reassurance on every command it runs.
		client, transport, _ := newGatedClient(t, func(int) (*http.Response, error) {
			return jsonResponse(http.StatusOK, apisBody(CloudExtensionGroup)), nil
		})

		ctx := context.Background()
		if err := client.Cloud.EnsureAvailable(ctx); err != nil {
			t.Fatalf("EnsureAvailable() error = %v", err)
		}
		if _, err := client.Cloud.Connections.List(ctx); err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if got := transport.probes(); got != 1 {
			t.Errorf("GET /apis requests = %d, want 1 shared between the check and the call", got)
		}
	})
}
