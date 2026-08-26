// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/pagination"
)

// Ported from orca-sdk-typescript tests/core/pagination.test.ts.
//
// Two cursor dialects share one type, and the difference matters:
//
//   - page-token: the response carries next_page, and the next request sends
//     page=<next_page>. Used by agents, sessions, threads, memory stores,
//     memories, memory versions, vaults, credentials, environments, skills and
//     triggers.
//   - ID-cursor: the response carries first_id/last_id, and the next request
//     sends after_id=<last_id> (or, walking backwards, before_id=<first_id>).
//     Used by files and session files ONLY.

// pageServer answers each request with the next scripted body, recording the
// URLs it was asked for so a test can assert the query the cursor built.
type pageServer struct {
	bodies []string
	calls  []*url.URL
}

func (s *pageServer) respond(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, req.URL)
	if len(s.bodies) == 0 {
		return jsonResponse(http.StatusOK, `{"data":[]}`), nil
	}
	body := s.bodies[0]
	s.bodies = s.bodies[1:]
	return jsonResponse(http.StatusOK, body), nil
}

// requests returns the query of each request, in order.
func (s *pageServer) requests() []url.Values {
	queries := make([]url.Values, 0, len(s.calls))
	for _, call := range s.calls {
		queries = append(queries, call.Query())
	}
	return queries
}

// firstPage serves bodies in order and returns the first page.
func firstPage(tb testing.TB, path string, bodies ...string) (*pagination.PageCursor[string], *pageServer) {
	tb.Helper()
	server := &pageServer{bodies: bodies}
	client, _ := newRecordingClient(tb, server.respond)

	page, err := ListPage[string](context.Background(), client, path)
	if err != nil {
		tb.Fatalf("ListPage() error = %v", err)
	}
	return page, server
}

// -----------------------------------------------------------------------
// Construction and next-page detection
// -----------------------------------------------------------------------

func TestPageCursorBasics(t *testing.T) {
	t.Parallel()

	t.Run("items are the data array, in order", func(t *testing.T) {
		t.Parallel()

		page, _ := firstPage(t, "/v1/agents", `{"data":["a1","a2","a3"],"has_more":false,"first_id":"a1","last_id":"a3"}`)
		if got, want := page.Items(), []string{"a1", "a2", "a3"}; !slices.Equal(got, want) {
			t.Errorf("Items() = %v, want %v", got, want)
		}
	})

	// has_more and the cursors interact: the server's explicit "no" always
	// wins, and its "yes" is only actionable when it also supplied something to
	// send in the next request.
	tests := []struct {
		name string
		path string
		body string
		want bool
		why  string
	}{
		{
			name: "has_more false ends the walk",
			path: "/v1/agents",
			body: `{"data":["a1"],"has_more":false,"first_id":"a1","last_id":"a1"}`,
			want: false,
		},
		{
			name: "has_more without any cursor ends the walk",
			path: "/v1/agents",
			body: `{"data":["x"],"has_more":true}`,
			want: false,
			why:  "has_more alone is not enough - there is no cursor to send",
		},
		{
			name: "has_more plus next_page continues",
			path: "/v1/agents",
			body: `{"data":["x"],"has_more":true,"next_page":"cursor-abc"}`,
			want: true,
		},
		{
			name: "an opaque next_page continues even without has_more",
			path: "/v1/agents",
			body: `{"data":["x"],"next_page":"cursor-abc"}`,
			want: true,
			why:  "next_page on its own is sufficient",
		},
		{
			name: "ID-cursor: has_more plus last_id continues",
			path: "/v1/files",
			body: `{"data":["x"],"has_more":true,"last_id":"file_abc"}`,
			want: true,
		},
		{
			name: "ID-cursor: first_id continues a before_id walk",
			path: "/v1/files?before_id=file_anchor",
			body: `{"data":["x"],"has_more":true,"first_id":"file_first"}`,
			want: true,
			why:  "walking backwards, first_id is the cursor that matters, not last_id",
		},
		{
			name: "ID-cursor: first_id alone does not continue a forward walk",
			path: "/v1/files",
			body: `{"data":["x"],"has_more":true,"first_id":"file_first"}`,
			want: false,
			why:  "a forward walk needs last_id; first_id anchors the page it just read",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			page, _ := firstPage(t, tc.path, tc.body)
			if got := page.HasNextPage(); got != tc.want {
				t.Errorf("HasNextPage() = %v, want %v (%s)", got, tc.want, tc.why)
			}
		})
	}

	t.Run("a single page iterates without fetching anything", func(t *testing.T) {
		t.Parallel()

		page, server := firstPage(t, "/v1/agents", `{"data":["a1","a2","a3"],"has_more":false}`)

		var got []string
		for item, err := range page.All(context.Background()) {
			if err != nil {
				t.Fatalf("iteration error = %v", err)
			}
			got = append(got, item)
		}

		if want := []string{"a1", "a2", "a3"}; !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
		if len(server.calls) != 1 {
			t.Errorf("requests = %d, want exactly 1 - a terminal page must not be re-fetched", len(server.calls))
		}
	})
}

// -----------------------------------------------------------------------
// Iteration and the query each next-page request builds
// -----------------------------------------------------------------------

func TestPageCursorIteration(t *testing.T) {
	t.Parallel()

	t.Run("iteration spans pages then terminates", func(t *testing.T) {
		t.Parallel()

		page, server := firstPage(t, "/v1/agents",
			`{"data":["item-1","item-2"],"has_more":true,"first_id":"item-1","last_id":"item-2","next_page":"cursor-p2"}`,
			`{"data":["item-3","item-4"],"has_more":false,"first_id":"item-3","last_id":"item-4"}`,
		)

		var got []string
		for item, err := range page.All(context.Background()) {
			if err != nil {
				t.Fatalf("iteration error = %v", err)
			}
			got = append(got, item)
		}

		want := []string{"item-1", "item-2", "item-3", "item-4"}
		if !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
		// The terminal page must not trigger a third fetch.
		if len(server.calls) != 2 {
			t.Errorf("requests = %d, want exactly 2", len(server.calls))
		}
	})

	// What each dialect must put in the next request. Getting this wrong does
	// not fail loudly: it re-reads a page, skips one, or walks backwards
	// forever.
	tests := []struct {
		name string
		path string
		body string
		want url.Values
		why  string
	}{
		{
			name: "page-token: the cursor goes in page",
			path: "/v1/agents",
			body: `{"data":["item-1"],"has_more":true,"next_page":"cursor-xyz"}`,
			want: url.Values{"page": {"cursor-xyz"}},
		},
		{
			name: "ID-cursor: last_id becomes after_id, merged into the existing query",
			path: "/v1/files?limit=10",
			body: `{"data":["file_1"],"has_more":true,"last_id":"file_1"}`,
			want: url.Values{"limit": {"10"}, "after_id": {"file_1"}},
			why:  "the caller's limit must survive",
		},
		{
			name: "ID-cursor: a before_id walk keeps its direction",
			path: "/v1/files?limit=10&before_id=file_1",
			body: `{"data":["file_3","file_2"],"has_more":true,"first_id":"file_3","last_id":"file_2"}`,
			want: url.Values{"limit": {"10"}, "before_id": {"file_3"}},
			why: "first_id replaces the before_id anchor and NO after_id is added, " +
				"even though last_id is present - otherwise the walk reverses and re-reads",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			page, server := firstPage(t, tc.path, tc.body, `{"data":[],"has_more":false}`)
			if _, err := page.GetNextPage(context.Background()); err != nil {
				t.Fatalf("GetNextPage() error = %v", err)
			}

			queries := server.requests()
			if len(queries) != 2 {
				t.Fatalf("requests = %d, want 2", len(queries))
			}
			if got := queries[1]; !slices.Equal(sortedQuery(got), sortedQuery(tc.want)) {
				t.Errorf("next-page query = %v, want %v (%s)", got, tc.want, tc.why)
			}
		})
	}
}

// sortedQuery renders a query as sorted key=value pairs so two Values compare
// independently of map order.
func sortedQuery(values url.Values) []string {
	var pairs []string
	for key, vs := range values {
		for _, v := range vs {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, v))
		}
	}
	slices.Sort(pairs)
	return pairs
}

// -----------------------------------------------------------------------
// Failure behaviour
// -----------------------------------------------------------------------

func TestPageCursorErrors(t *testing.T) {
	t.Parallel()

	t.Run("no more pages is an error, not an empty page", func(t *testing.T) {
		t.Parallel()

		// Asking for a page that does not exist is a caller bug. Returning an
		// empty page instead would hide it behind a loop that silently does
		// nothing.
		page, _ := firstPage(t, "/v1/agents", `{"data":[],"has_more":false}`)

		next, err := page.GetNextPage(context.Background())
		if !errors.Is(err, pagination.ErrNoMorePages) {
			t.Errorf("GetNextPage() error = %v, want ErrNoMorePages", err)
		}
		if next != nil {
			t.Errorf("GetNextPage() page = %v, want nil", next)
		}
	})

	t.Run("next-page fetches go through the client request pipeline", func(t *testing.T) {
		t.Parallel()

		// The cursor must not reach for the transport itself, or a next-page
		// fetch would lose the credential, headers, retries and per-call
		// options the first page had.
		page, server := firstPage(t, "/v1/agents",
			`{"data":["x"],"has_more":true,"next_page":"cursor-abc"}`,
			`{"data":["second-page-item"],"has_more":false}`,
		)

		next, err := page.GetNextPage(context.Background())
		if err != nil {
			t.Fatalf("GetNextPage() error = %v", err)
		}
		if got, want := next.Items(), []string{"second-page-item"}; !slices.Equal(got, want) {
			t.Errorf("next page items = %v, want %v", got, want)
		}
		if len(server.calls) != 2 {
			t.Fatalf("requests = %d, want exactly 2", len(server.calls))
		}
		if got := server.calls[1].Query().Get("page"); got != "cursor-abc" {
			t.Errorf("next-page cursor = %q, want %q", got, "cursor-abc")
		}
	})

	t.Run("a mid-iteration failure surfaces instead of ending the walk", func(t *testing.T) {
		t.Parallel()

		// A failure that looked like the end of the data would silently
		// truncate the caller's results.
		var calls int
		client, _ := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(http.StatusOK, `{"data":["a1"],"has_more":true,"next_page":"p2"}`), nil
			}
			return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
		})

		page, err := ListPage[string](context.Background(), client, "/v1/agents")
		if err != nil {
			t.Fatalf("ListPage() error = %v", err)
		}

		var items []string
		var iterErr error
		for item, err := range page.All(context.Background()) {
			if err != nil {
				iterErr = err
				break
			}
			items = append(items, item)
		}

		if want := []string{"a1"}; !slices.Equal(items, want) {
			t.Errorf("items = %v, want %v", items, want)
		}
		var serverErr *InternalServerError
		if !errors.As(iterErr, &serverErr) {
			t.Errorf("iteration error = %T (%v), want *InternalServerError", iterErr, iterErr)
		}
	})

	t.Run("a failed first request never yields a cursor", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, `{"error":"nope"}`), nil
		})

		page, err := ListPage[string](context.Background(), client, "/v1/agents")
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("ListPage() error = %T (%v), want *NotFoundError", err, err)
		}
		if page != nil {
			t.Errorf("ListPage() page = %v, want nil", page)
		}
	})
}

// -----------------------------------------------------------------------
// The auto-pager
// -----------------------------------------------------------------------

func TestPageCursorAutoPager(t *testing.T) {
	t.Parallel()

	t.Run("walks every item across every page and reports its index", func(t *testing.T) {
		t.Parallel()

		page, _ := firstPage(t, "/v1/agents",
			`{"data":["a","b"],"has_more":true,"next_page":"p2"}`,
			`{"data":["c"],"has_more":false}`,
		)

		pager := page.AutoPager(context.Background())
		if got := pager.Index(); got != -1 {
			t.Errorf("Index() before Next = %d, want -1", got)
		}

		var got []string
		var indexes []int
		for pager.Next() {
			got = append(got, pager.Current())
			indexes = append(indexes, pager.Index())
		}
		if err := pager.Err(); err != nil {
			t.Fatalf("Err() = %v", err)
		}
		if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
		if want := []int{0, 1, 2}; !slices.Equal(indexes, want) {
			t.Errorf("indexes = %v, want %v - the index spans pages", indexes, want)
		}
	})

	t.Run("reports a fetch failure through Err", func(t *testing.T) {
		t.Parallel()

		var calls int
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(http.StatusOK, `{"data":["a1"],"has_more":true,"next_page":"p2"}`), nil
			}
			return jsonResponse(http.StatusInternalServerError, `{}`), nil
		})

		page, err := ListPage[string](context.Background(), client, "/v1/agents")
		if err != nil {
			t.Fatalf("ListPage() error = %v", err)
		}

		pager := page.AutoPager(context.Background())
		var got []string
		for pager.Next() {
			got = append(got, pager.Current())
		}
		if want := []string{"a1"}; !slices.Equal(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
		var serverErr *InternalServerError
		if !errors.As(pager.Err(), &serverErr) {
			t.Errorf("Err() = %T (%v), want *InternalServerError", pager.Err(), pager.Err())
		}
	})

	t.Run("breaking out of All stops fetching", func(t *testing.T) {
		t.Parallel()

		page, server := firstPage(t, "/v1/agents",
			`{"data":["a","b"],"has_more":true,"next_page":"p2"}`,
			`{"data":["c"],"has_more":false}`,
		)

		for item, err := range page.All(context.Background()) {
			if err != nil {
				t.Fatalf("iteration error = %v", err)
			}
			if item == "a" {
				break
			}
		}

		if len(server.calls) != 1 {
			t.Errorf("requests = %d, want 1 - abandoning the loop must not fetch more pages", len(server.calls))
		}
	})
}
