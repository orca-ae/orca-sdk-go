// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
	"github.com/orca-ae/orca-sdk-go/packages/ssestream"
)

// SessionThreadService reads the threads within a session.
//
// There is deliberately no create: a session has one primary thread, and the
// coordinator spawns any others as it runs. A create method would suggest the
// caller controls something the server owns.
type SessionThreadService struct {
	client *Client

	// Events reads and streams one thread's events.
	Events SessionThreadEventService
}

// SessionThreadStatus is where a thread is in its lifecycle.
type SessionThreadStatus string

// SessionThreadStats is how long a thread has been running.
type SessionThreadStats struct {
	ActiveSeconds   float64 `json:"active_seconds,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	StartupSeconds  float64 `json:"startup_seconds,omitempty"`
}

// SessionThreadUsage is the token usage a thread has accumulated.
type SessionThreadUsage struct {
	InputTokens          int64               `json:"input_tokens,omitempty"`
	OutputTokens         int64               `json:"output_tokens,omitempty"`
	CacheReadInputTokens int64               `json:"cache_read_input_tokens,omitempty"`
	CacheCreation        *CacheCreationUsage `json:"cache_creation,omitempty"`
}

// SessionThread is one thread of execution within a session.
type SessionThread struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	SessionID string             `json:"session_id"`
	Agent     SessionAgentMember `json:"agent"`

	// ParentThreadID is nil for the primary thread.
	ParentThreadID *string `json:"parent_thread_id"`

	Status SessionThreadStatus `json:"status"`

	// Stats and Usage are nullable on a thread, unlike on a session where they
	// are always objects.
	Stats *SessionThreadStats `json:"stats"`
	Usage *SessionThreadUsage `json:"usage"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// ArchivedAt is nil while the thread is active.
	ArchivedAt *string `json:"archived_at"`
}

// SessionThreadListParams pages a session's threads.
//
// Limit and page only: threads accept neither an archive filter nor an order.
type SessionThreadListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// List returns a page of a session's threads.
func (s SessionThreadService) List(ctx context.Context, sessionID string, params SessionThreadListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionThread], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[SessionThread](ctx, s.client, "v1/sessions/"+url.PathEscape(sessionID)+"/threads", opts...)
}

// Get reads one thread.
func (s SessionThreadService) Get(ctx context.Context, sessionID, threadID string, opts ...option.RequestOption) (*SessionThread, error) {
	var thread SessionThread
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/threads/" + url.PathEscape(threadID)
	if err := s.client.GetJSON(ctx, path, &thread, opts...); err != nil {
		return nil, err
	}
	return &thread, nil
}

// Archive archives a thread and returns it.
func (s SessionThreadService) Archive(ctx context.Context, sessionID, threadID string, opts ...option.RequestOption) (*SessionThread, error) {
	var thread SessionThread
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/threads/" + url.PathEscape(threadID) + "/archive"
	if err := s.client.PostJSON(ctx, path, nil, &thread, opts...); err != nil {
		return nil, err
	}
	return &thread, nil
}

// SessionThreadEventService reads and streams one thread's events.
type SessionThreadEventService struct {
	client *Client
}

// SessionThreadEventListParams pages a thread's events.
//
// Pagination only: a thread's event list accepts no order parameter.
type SessionThreadEventListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// SessionThreadEventStreamParams controls a thread event stream.
//
// Unlike a session stream, a thread stream takes no subpath: the thread already
// identifies the scope.
type SessionThreadEventStreamParams struct {
	FromCursor  param.Opt[string]
	EventDeltas []string
}

// List returns a page of a thread's events.
func (s SessionThreadEventService) List(ctx context.Context, sessionID, threadID string, params SessionThreadEventListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionEvent], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/threads/" + url.PathEscape(threadID) + "/events"
	return ListPage[SessionEvent](ctx, s.client, path, opts...)
}

// Stream opens a Server-Sent Events stream of one thread's events.
//
// The path is /threads/{id}/stream, not /threads/{id}/events/stream - the one
// place the thread and session stream paths diverge.
func (s SessionThreadEventService) Stream(ctx context.Context, sessionID, threadID string, params SessionThreadEventStreamParams, opts ...option.RequestOption) *ssestream.Stream[SessionEvent] {
	opts = appendStringQuery(opts, "from_cursor", params.FromCursor)
	for _, delta := range params.EventDeltas {
		opts = append(opts, option.WithQueryAdd("event_deltas", delta))
	}
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/threads/" + url.PathEscape(threadID) + "/stream"
	return StreamEvents[SessionEvent](ctx, s.client, path, opts...)
}
