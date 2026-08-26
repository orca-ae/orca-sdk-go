// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript tests/core/pagination.test.ts.
//
// Specifies the page cursor that every typed Managed Agents list operation
// returns. This SDK has no cursor type at all: managed_agents.go returns
// interface{} and leaves paging to the caller, so there is nothing to assert
// against yet. These tables are the spec a Go cursor must satisfy.
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

// pendingPaginationRule is one behavioural rule of the cursor: the response
// body and request query it is constructed from, and what must then be true.
type pendingPaginationRule struct {
	// Name identifies the rule.
	Name string

	// Given is the JSON page body the cursor is built from.
	Given string

	// Query is the query the originating request carried, where it changes the
	// outcome (an ID-cursor walk anchored with before_id, for example).
	Query string

	// Then is the required behaviour.
	Then string
}

// pendingPaginationSurface reports every rule in spec as unimplemented. Tests
// call it after t.Skip(pendingManagedAgents), so it runs only once someone
// lifts the skip.
func pendingPaginationSurface(t *testing.T, spec []pendingPaginationRule) {
	t.Helper()
	for _, rule := range spec {
		t.Errorf("no page cursor implements %q: %s", rule.Name, rule.Then)
	}
}

// pendingPageCursorBasicSpec covers construction and next-page detection.
var pendingPageCursorBasicSpec = []pendingPaginationRule{
	{
		Name:  "items are the data array",
		Given: `{"data":[{"id":"a1"},{"id":"a2"},{"id":"a3"}],"has_more":false,"first_id":"a1","last_id":"a3"}`,
		Then:  "the paginated items are exactly the data array, in order",
	},
	{
		Name:  "has_more false ends the walk",
		Given: `{"data":[{"id":"a1"}],"has_more":false,"first_id":"a1","last_id":"a1"}`,
		Then:  "hasNextPage is false",
	},
	{
		Name:  "has_more without any cursor ends the walk",
		Given: `{"data":["x"],"has_more":true}`,
		Then: "hasNextPage is false — has_more alone is not enough, because there " +
			"is no cursor to send",
	},
	{
		Name:  "has_more plus next_page continues",
		Given: `{"data":["x"],"has_more":true,"next_page":"cursor-abc"}`,
		Then:  "hasNextPage is true",
	},
	{
		Name:  "an opaque next_page continues even without has_more",
		Given: `{"data":["x"],"next_page":"cursor-abc"}`,
		Then:  "hasNextPage is true — next_page on its own is sufficient",
	},
	{
		Name:  "ID-cursor: has_more plus last_id continues",
		Given: `{"data":["x"],"has_more":true,"last_id":"file_abc"}`,
		Then:  "hasNextPage is true",
	},
	{
		Name:  "ID-cursor: first_id continues a before_id walk",
		Given: `{"data":["x"],"has_more":true,"first_id":"file_first"}`,
		Query: `{"before_id":"file_anchor"}`,
		Then: "hasNextPage is true — walking backwards, first_id is the cursor " +
			"that matters, not last_id",
	},
	{
		Name:  "single-page iteration fetches nothing",
		Given: `{"data":[{"id":"a1"},{"id":"a2"},{"id":"a3"}],"has_more":false}`,
		Then:  "iterating yields all three items and makes no next-page request",
	},
}

// pendingPageCursorIterationSpec covers multi-page iteration and the exact
// query getNextPage must build.
var pendingPageCursorIterationSpec = []pendingPaginationRule{
	{
		Name: "iteration spans pages then terminates",
		Given: `page1 {"data":["item-1","item-2"],"has_more":true,"first_id":"item-1",` +
			`"last_id":"item-2","next_page":"cursor-p2"}; ` +
			`page2 {"data":["item-3","item-4"],"has_more":false,"first_id":"item-3","last_id":"item-4"}`,
		Then: "iterating yields item-1..item-4 and makes exactly ONE next-page " +
			"request — the terminal page must not trigger a third fetch",
	},
	{
		Name:  "page-token: the cursor goes in `page`",
		Given: `{"data":["item-1"],"has_more":true,"next_page":"cursor-xyz"}`,
		Then:  "the next request's query carries page=cursor-xyz",
	},
	{
		Name:  "ID-cursor: last_id becomes after_id, merged into the existing query",
		Given: `{"data":["file_1"],"has_more":true,"last_id":"file_1"}`,
		Query: `{"limit":10}`,
		Then: `the next request's query is exactly {"limit":10,"after_id":"file_1"} ` +
			"— the caller's limit survives and after_id is added",
	},
	{
		Name:  "ID-cursor: a before_id walk keeps its direction",
		Given: `{"data":["file_3","file_2"],"has_more":true,"first_id":"file_3","last_id":"file_2"}`,
		Query: `{"limit":10,"before_id":"file_1"}`,
		Then: `the next request's query is exactly {"limit":10,"before_id":"file_3"} ` +
			"— first_id replaces the before_id anchor and NO after_id is added, " +
			"even though last_id is present",
	},
}

// pendingPageCursorErrorSpec covers getNextPage's failure and delegation
// behaviour.
var pendingPageCursorErrorSpec = []pendingPaginationRule{
	{
		Name:  "no more pages is an error, not an empty page",
		Given: `{"data":[],"has_more":false}`,
		Then:  `fetching the next page fails with "No more pages to fetch"`,
	},
	{
		Name:  "next-page fetches go through the client request pipeline",
		Given: `{"data":["x"],"has_more":true,"next_page":"cursor-abc"}`,
		Then: "the cursor does not issue its own transport call: it re-enters the " +
			"client's list-request path exactly once, and the resulting page's " +
			"items are the second page's data",
	},
}

// pendingPagePromiseSpec covers the awaitable-and-iterable list result: a list
// call is usable either as a promise of one page or as a stream of items.
var pendingPagePromiseSpec = []pendingPaginationRule{
	{
		Name:  "awaits to a cursor",
		Given: `{"data":["alpha","beta"],"has_more":false}`,
		Then:  "awaiting the list call yields a page cursor whose items are [alpha beta]",
	},
	{
		Name:  "iterates directly, without awaiting first",
		Given: `{"data":[10,20,30],"has_more":false}`,
		Then:  "iterating the list call itself yields 10, 20, 30",
	},
	{
		Name:  "next-page detection survives the await",
		Given: `{"data":["only-item"],"has_more":false}`,
		Then:  "the resolved page reports hasNextPage false",
	},
	{
		Name:  "request failures propagate",
		Given: "the underlying request fails",
		Then: "awaiting (or iterating) the list call surfaces that error rather " +
			"than an empty page",
	},
}

func TestPendingPageCursorBasics(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPaginationSurface(t, pendingPageCursorBasicSpec)
}

func TestPendingPageCursorIteration(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPaginationSurface(t, pendingPageCursorIterationSpec)
}

func TestPendingPageCursorErrors(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPaginationSurface(t, pendingPageCursorErrorSpec)
}

func TestPendingPagePromise(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPaginationSurface(t, pendingPagePromiseSpec)
}
