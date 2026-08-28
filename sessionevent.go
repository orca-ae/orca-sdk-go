// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apijson"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
	"github.com/orca-ae/orca-sdk-go/packages/ssestream"
)

// SessionEventService reads, sends, and streams a session's events.
type SessionEventService struct {
	client *Client
}

// SessionEventContent is one block of an event's content.
//
// Content blocks are open-ended - text, images, tool calls and whatever a
// provider adds next - so everything past the discriminator travels in Extra
// rather than being dropped.
type SessionEventContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitzero"`
	Extra map[string]any `json:"-"`
}

// Text returns a text content block.
func Text(text string) SessionEventContent {
	return SessionEventContent{Type: "text", Text: text}
}

// MarshalJSON implements [json.Marshaler].
func (c SessionEventContent) MarshalJSON() ([]byte, error) {
	type shape SessionEventContent
	return apijson.MarshalWithExtra(shape(c), c.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (c *SessionEventContent) UnmarshalJSON(data []byte) error {
	type shape SessionEventContent
	return apijson.UnmarshalWithExtra(data, (*shape)(c), []string{"type", "text"}, &c.Extra)
}

// SessionEventParam is an event to append to a session.
type SessionEventParam struct {
	Type    string                `json:"type"`
	Content []SessionEventContent `json:"content,omitzero"`
	Extra   map[string]any        `json:"-"`
}

// UserMessage returns a user message event carrying text.
func UserMessage(text string) SessionEventParam {
	return SessionEventParam{Type: "user.message", Content: []SessionEventContent{Text(text)}}
}

// MarshalJSON implements [json.Marshaler].
func (e SessionEventParam) MarshalJSON() ([]byte, error) {
	type shape SessionEventParam
	return apijson.MarshalWithExtra(shape(e), e.Extra)
}

// SessionEvent is one persisted event.
type SessionEvent struct {
	ID        string                `json:"id"`
	Type      string                `json:"type"`
	SessionID string                `json:"session_id,omitzero"`
	ThreadID  string                `json:"thread_id,omitzero"`
	Subpath   string                `json:"subpath,omitzero"`
	Content   []SessionEventContent `json:"content,omitzero"`
	CreatedAt string                `json:"created_at,omitzero"`
	Extra     map[string]any        `json:"-"`
}

// UnmarshalJSON implements [json.Unmarshaler].
func (e *SessionEvent) UnmarshalJSON(data []byte) error {
	type shape SessionEvent
	return apijson.UnmarshalWithExtra(data, (*shape)(e),
		[]string{"id", "type", "session_id", "thread_id", "subpath", "content", "created_at"},
		&e.Extra)
}

// SessionEventOrder is the direction a list of events is returned in.
type SessionEventOrder string

const (
	SessionEventOrderAsc  SessionEventOrder = "asc"
	SessionEventOrderDesc SessionEventOrder = "desc"
)

// SessionEventListParams filters and pages a session's events.
//
// The created_at bounds are literal parameter names with brackets in them -
// "created_at[gte]" is one parameter, not a nested object - which is why they
// are separate fields rather than a struct.
type SessionEventListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
	Order param.Opt[SessionEventOrder]

	CreatedAtGT  param.Opt[string]
	CreatedAtGTE param.Opt[string]
	CreatedAtLT  param.Opt[string]
	CreatedAtLTE param.Opt[string]

	// Types repeats: types=user.message&types=agent.message.
	Types []string

	Subpath param.Opt[string]
}

// SessionEventStreamParams controls a session event stream.
type SessionEventStreamParams struct {
	// FromCursor resumes a stream where a previous one stopped.
	FromCursor param.Opt[string]

	Subpath param.Opt[string]

	// EventDeltas repeats, selecting which incremental updates to receive.
	EventDeltas []string
}

// List returns a page of a session's persisted events.
func (s SessionEventService) List(ctx context.Context, sessionID string, params SessionEventListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionEvent], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendEnumQuery(opts, "order", params.Order)
	opts = appendStringQuery(opts, "created_at[gt]", params.CreatedAtGT)
	opts = appendStringQuery(opts, "created_at[gte]", params.CreatedAtGTE)
	opts = appendStringQuery(opts, "created_at[lt]", params.CreatedAtLT)
	opts = appendStringQuery(opts, "created_at[lte]", params.CreatedAtLTE)
	opts = appendStringQuery(opts, "subpath", params.Subpath)
	for _, eventType := range params.Types {
		opts = append(opts, option.WithQueryAdd("types", eventType))
	}
	return ListPage[SessionEvent](ctx, s.client, "v1/sessions/"+url.PathEscape(sessionID)+"/events", opts...)
}

// Send appends events to a session and returns their persisted forms.
//
// Several events go in one call, which is how a caller sends a message and an
// interrupt without racing them against each other.
func (s SessionEventService) Send(ctx context.Context, sessionID string, events []SessionEventParam, opts ...option.RequestOption) ([]SessionEvent, error) {
	body := struct {
		Events []SessionEventParam `json:"events"`
	}{Events: events}

	var response struct {
		Data []SessionEvent `json:"data"`
	}
	if err := s.client.PostJSON(ctx, "v1/sessions/"+url.PathEscape(sessionID)+"/events", body, &response, opts...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// Stream opens a Server-Sent Events stream of a session's events.
//
// The returned stream must be closed. Parameters that were not set produce no
// query string at all, so a caller passing only request options opens the
// stream the server would have opened by default.
func (s SessionEventService) Stream(ctx context.Context, sessionID string, params SessionEventStreamParams, opts ...option.RequestOption) *ssestream.Stream[SessionEvent] {
	opts = appendStringQuery(opts, "from_cursor", params.FromCursor)
	opts = appendStringQuery(opts, "subpath", params.Subpath)
	for _, delta := range params.EventDeltas {
		opts = append(opts, option.WithQueryAdd("event_deltas", delta))
	}
	return StreamEvents[SessionEvent](ctx, s.client, "v1/sessions/"+url.PathEscape(sessionID)+"/events/stream", opts...)
}
