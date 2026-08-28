// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"io"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/ssestream"
)

// SessionHandle binds a session ID once so the sub-resource calls that follow
// do not repeat it.
//
// Working with one session means passing the same ID to every call, and the
// compiler cannot tell a session ID from a thread ID when both are strings. A
// handle removes the repetition and the chance of transposing them.
//
// It is a binding and nothing more: every call produces byte-for-byte the same
// request as the equivalent call on client.Sessions.
//
// It lives in this package rather than a lib subpackage so it can be reached as
// client.Session(id); a separate package could not be.
type SessionHandle struct {
	client *Client

	// sessionID is the session every call through this handle is scoped to.
	sessionID string

	// Events reads, sends, and streams this session's events.
	Events SessionHandleEvents

	// Files manages the files attached to this session.
	Files SessionHandleFiles

	// Resources manages what this session can reach.
	Resources SessionHandleResources

	// Threads reads this session's threads.
	Threads SessionHandleThreads
}

// Session returns a handle bound to sessionID.
func (c *Client) Session(sessionID string) SessionHandle {
	return SessionHandle{
		client:    c,
		sessionID: sessionID,
		Events:    SessionHandleEvents{client: c, sessionID: sessionID},
		Files:     SessionHandleFiles{client: c, sessionID: sessionID},
		Resources: SessionHandleResources{client: c, sessionID: sessionID},
		Threads: SessionHandleThreads{
			client:    c,
			sessionID: sessionID,
			Events:    SessionHandleThreadEvents{client: c, sessionID: sessionID},
		},
	}
}

// SessionID returns the session this handle is bound to.
func (h SessionHandle) SessionID() string { return h.sessionID }

// Get reads the session.
func (h SessionHandle) Get(ctx context.Context, opts ...option.RequestOption) (*Session, error) {
	return h.client.Sessions.Get(ctx, h.sessionID, opts...)
}

// Update updates the session.
func (h SessionHandle) Update(ctx context.Context, params SessionUpdateParams, opts ...option.RequestOption) (*Session, error) {
	return h.client.Sessions.Update(ctx, h.sessionID, params, opts...)
}

// Delete permanently deletes the session.
func (h SessionHandle) Delete(ctx context.Context, opts ...option.RequestOption) (*SessionDeleted, error) {
	return h.client.Sessions.Delete(ctx, h.sessionID, opts...)
}

// Archive archives the session.
func (h SessionHandle) Archive(ctx context.Context, opts ...option.RequestOption) (*Session, error) {
	return h.client.Sessions.Archive(ctx, h.sessionID, opts...)
}

// SessionHandleEvents is [SessionEventService] bound to one session.
type SessionHandleEvents struct {
	client    *Client
	sessionID string
}

// List returns a page of the session's persisted events.
func (h SessionHandleEvents) List(ctx context.Context, params SessionEventListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionEvent], error) {
	return h.client.Sessions.Events.List(ctx, h.sessionID, params, opts...)
}

// Send appends events to the session.
func (h SessionHandleEvents) Send(ctx context.Context, events []SessionEventParam, opts ...option.RequestOption) ([]SessionEvent, error) {
	return h.client.Sessions.Events.Send(ctx, h.sessionID, events, opts...)
}

// Stream opens a Server-Sent Events stream of the session's events.
func (h SessionHandleEvents) Stream(ctx context.Context, params SessionEventStreamParams, opts ...option.RequestOption) *ssestream.Stream[SessionEvent] {
	return h.client.Sessions.Events.Stream(ctx, h.sessionID, params, opts...)
}

// SessionHandleFiles is [SessionFileService] bound to one session.
type SessionHandleFiles struct {
	client    *Client
	sessionID string
}

// List returns a page of the files attached to the session.
func (h SessionHandleFiles) List(ctx context.Context, params SessionFileListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionFile], error) {
	return h.client.Sessions.Files.List(ctx, h.sessionID, params, opts...)
}

// Get reads a session file's metadata.
func (h SessionHandleFiles) Get(ctx context.Context, fileID string, opts ...option.RequestOption) (*SessionFile, error) {
	return h.client.Sessions.Files.Get(ctx, h.sessionID, fileID, opts...)
}

// Download streams a session file's raw content to writer.
func (h SessionHandleFiles) Download(ctx context.Context, fileID string, writer io.Writer, opts ...option.RequestOption) error {
	return h.client.Sessions.Files.Download(ctx, h.sessionID, fileID, writer, opts...)
}

// Delete removes a session file.
func (h SessionHandleFiles) Delete(ctx context.Context, fileID string, opts ...option.RequestOption) (*SessionFileDeleted, error) {
	return h.client.Sessions.Files.Delete(ctx, h.sessionID, fileID, opts...)
}

// SessionHandleResources is [SessionResourceService] bound to one session.
type SessionHandleResources struct {
	client    *Client
	sessionID string
}

// List returns a page of the session's resources.
func (h SessionHandleResources) List(ctx context.Context, params SessionResourceListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionResource], error) {
	return h.client.Sessions.Resources.List(ctx, h.sessionID, params, opts...)
}

// Add attaches a resource to the session.
func (h SessionHandleResources) Add(ctx context.Context, params SessionResourceParam, opts ...option.RequestOption) (*SessionResource, error) {
	return h.client.Sessions.Resources.Add(ctx, h.sessionID, params, opts...)
}

// Get reads one of the session's resources.
func (h SessionHandleResources) Get(ctx context.Context, resourceID string, opts ...option.RequestOption) (*SessionResource, error) {
	return h.client.Sessions.Resources.Get(ctx, h.sessionID, resourceID, opts...)
}

// Update rotates a repository resource's authorization token.
func (h SessionHandleResources) Update(ctx context.Context, resourceID string, params SessionResourceUpdateParams, opts ...option.RequestOption) (*SessionResource, error) {
	return h.client.Sessions.Resources.Update(ctx, h.sessionID, resourceID, params, opts...)
}

// Delete detaches a resource from the session.
func (h SessionHandleResources) Delete(ctx context.Context, resourceID string, opts ...option.RequestOption) (*SessionResourceDeleted, error) {
	return h.client.Sessions.Resources.Delete(ctx, h.sessionID, resourceID, opts...)
}

// SessionHandleThreads is [SessionThreadService] bound to one session.
type SessionHandleThreads struct {
	client    *Client
	sessionID string

	// Events reads and streams one thread's events.
	Events SessionHandleThreadEvents
}

// List returns a page of the session's threads.
func (h SessionHandleThreads) List(ctx context.Context, params SessionThreadListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionThread], error) {
	return h.client.Sessions.Threads.List(ctx, h.sessionID, params, opts...)
}

// Get reads one thread.
func (h SessionHandleThreads) Get(ctx context.Context, threadID string, opts ...option.RequestOption) (*SessionThread, error) {
	return h.client.Sessions.Threads.Get(ctx, h.sessionID, threadID, opts...)
}

// Archive archives a thread.
func (h SessionHandleThreads) Archive(ctx context.Context, threadID string, opts ...option.RequestOption) (*SessionThread, error) {
	return h.client.Sessions.Threads.Archive(ctx, h.sessionID, threadID, opts...)
}

// SessionHandleThreadEvents is [SessionThreadEventService] bound to one session.
type SessionHandleThreadEvents struct {
	client    *Client
	sessionID string
}

// List returns a page of a thread's events.
func (h SessionHandleThreadEvents) List(ctx context.Context, threadID string, params SessionThreadEventListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionEvent], error) {
	return h.client.Sessions.Threads.Events.List(ctx, h.sessionID, threadID, params, opts...)
}

// Stream opens a Server-Sent Events stream of one thread's events.
func (h SessionHandleThreadEvents) Stream(ctx context.Context, threadID string, params SessionThreadEventStreamParams, opts ...option.RequestOption) *ssestream.Stream[SessionEvent] {
	return h.client.Sessions.Threads.Events.Stream(ctx, h.sessionID, threadID, params, opts...)
}
