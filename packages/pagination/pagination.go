// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package pagination walks paginated list responses.
//
// The API uses two cursor dialects, and one type serves both because a caller
// should not have to know which a given resource speaks:
//
//   - Page-token. The response carries next_page and the next request sends
//     page=<next_page>. Used by agents, sessions, threads, memory stores,
//     memories, memory versions, vaults, credentials, environments, skills and
//     triggers.
//   - ID-cursor. The response carries first_id and last_id, and the next
//     request sends after_id=<last_id> walking forwards or before_id=<first_id>
//     walking backwards. Used by files and session files only.
package pagination

import (
	"context"
	"errors"
	"iter"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// ErrNoMorePages is returned by [PageCursor.GetNextPage] when the walk is over.
//
// It is an error rather than an empty page because those mean different things:
// a caller asking for a page that does not exist has a bug, and silently
// handing back an empty result would hide it.
var ErrNoMorePages = errors.New("No more pages to fetch")

// GetFunc performs a GET against an absolute URL and decodes the body into out.
//
// It is how a cursor re-enters the client's request pipeline instead of
// reaching for the transport itself, so retries, credentials, headers and
// per-call options all apply to a next-page fetch exactly as they did to the
// first.
type GetFunc func(ctx context.Context, rawURL string, out any) error

// PageCursor is one page of a list response, plus the state needed to ask for
// the next one.
type PageCursor[T any] struct {
	// Data is the page's items, in the order the server returned them.
	Data []T `json:"data"`

	// HasMore is the server's own statement about whether more pages exist.
	// It is optional rather than a plain bool because absent and false differ:
	// a response carrying next_page but no has_more still continues, while an
	// explicit has_more:false ends the walk whatever cursors are present.
	HasMore param.Opt[bool] `json:"has_more"`

	// FirstID and LastID are the ID-cursor anchors.
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`

	// NextPage is the page-token cursor.
	NextPage string `json:"next_page"`

	requestURL *url.URL
	get        GetFunc
}

// Fetch retrieves the page at rawURL and binds it to get, so the returned
// cursor can fetch its successors through the same pipeline.
func Fetch[T any](ctx context.Context, rawURL string, get GetFunc) (*PageCursor[T], error) {
	var page PageCursor[T]
	if err := get(ctx, rawURL, &page); err != nil {
		return nil, err
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	page.requestURL = parsed
	page.get = get
	return &page, nil
}

// Items returns the page's items.
func (p *PageCursor[T]) Items() []T {
	if p == nil {
		return nil
	}
	return p.Data
}

// HasNextPage reports whether another page can be fetched.
//
// It is false when the server said so, and also when it did not but supplied no
// cursor to send: has_more on its own is not actionable, since there would be
// nothing to put in the next request.
func (p *PageCursor[T]) HasNextPage() bool {
	_, ok := p.nextPageURL()
	return ok
}

// GetNextPage fetches the next page, or returns [ErrNoMorePages].
func (p *PageCursor[T]) GetNextPage(ctx context.Context) (*PageCursor[T], error) {
	next, ok := p.nextPageURL()
	if !ok {
		return nil, ErrNoMorePages
	}
	return Fetch[T](ctx, next, p.get)
}

// nextPageURL builds the URL of the next page, if there is one.
func (p *PageCursor[T]) nextPageURL() (string, bool) {
	if p == nil || p.requestURL == nil || p.get == nil {
		return "", false
	}

	// An explicit has_more:false ends the walk whatever cursors came with it.
	if hasMore, ok := p.HasMore.Value(); ok && !hasMore {
		return "", false
	}

	query := p.requestURL.Query()

	if p.NextPage != "" {
		query.Set("page", p.NextPage)
		return p.urlWithQuery(query), true
	}

	// Direction is decided by the anchor the original request carried, not by
	// which of first_id/last_id the response happens to include. A backwards
	// walk that switched to after_id because last_id was also present would
	// reverse direction mid-iteration and re-read pages it had already seen.
	if query.Has("before_id") {
		if p.FirstID == "" {
			return "", false
		}
		query.Set("before_id", p.FirstID)
		return p.urlWithQuery(query), true
	}

	if p.LastID != "" {
		query.Set("after_id", p.LastID)
		return p.urlWithQuery(query), true
	}

	return "", false
}

// urlWithQuery returns the request URL with query replacing its own, preserving
// every parameter the caller originally sent.
func (p *PageCursor[T]) urlWithQuery(query url.Values) string {
	next := *p.requestURL
	next.RawQuery = query.Encode()
	return next.String()
}

// All returns an iterator over every item across every page, fetching pages as
// it goes.
//
// This is the usual way to consume a list. The error is part of the iteration
// rather than something to remember to check afterwards, so a failure part way
// through cannot be mistaken for the end of the data:
//
//	for agent, err := range page.All(ctx) {
//		if err != nil {
//			return err
//		}
//		...
//	}
func (p *PageCursor[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		for page := p; page != nil; {
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasNextPage() {
				return
			}
			next, err := page.GetNextPage(ctx)
			if err != nil {
				yield(zero, err)
				return
			}
			page = next
		}
	}
}

// AutoPager returns a cursor-style iterator over every item across every page.
//
// [PageCursor.All] is usually clearer. This exists for callers who want the
// running index, or who are on a code path where a range-over-function loop is
// awkward:
//
//	pager := page.AutoPager(ctx)
//	for pager.Next() {
//		item := pager.Current()
//	}
//	if err := pager.Err(); err != nil { ... }
func (p *PageCursor[T]) AutoPager(ctx context.Context) *PageCursorAutoPager[T] {
	return &PageCursorAutoPager[T]{ctx: ctx, page: p, index: -1}
}

// PageCursorAutoPager iterates every item across every page.
type PageCursorAutoPager[T any] struct {
	ctx     context.Context
	page    *PageCursor[T]
	current T
	offset  int
	index   int
	err     error
	done    bool
}

// Next advances to the next item, fetching a page when the current one runs
// out, and reports whether there was one.
func (a *PageCursorAutoPager[T]) Next() bool {
	if a.done || a.err != nil || a.page == nil {
		return false
	}

	for a.offset >= len(a.page.Data) {
		if !a.page.HasNextPage() {
			a.done = true
			return false
		}
		next, err := a.page.GetNextPage(a.ctx)
		if err != nil {
			a.err = err
			a.done = true
			return false
		}
		a.page = next
		a.offset = 0
	}

	a.current = a.page.Data[a.offset]
	a.offset++
	a.index++
	return true
}

// Current returns the item Next advanced to.
func (a *PageCursorAutoPager[T]) Current() T { return a.current }

// Err returns the first error that stopped iteration, if any.
func (a *PageCursorAutoPager[T]) Err() error { return a.err }

// Index returns the zero-based position of the current item across all pages,
// or -1 before the first call to Next.
func (a *PageCursorAutoPager[T]) Index() int { return a.index }
