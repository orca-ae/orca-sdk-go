// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// TriggerSessionService lists the sessions a trigger created.
type TriggerSessionService struct {
	client *Client
}

// TriggerSessionListParams filters and pages the sessions a trigger created.
type TriggerSessionListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// List returns a page of the sessions a trigger created.
//
// The items are core [Session] values, not a trigger-specific type: a session a
// trigger started is the same thing as one a caller started.
func (s TriggerSessionService) List(ctx context.Context, triggerID string, params TriggerSessionListParams, opts ...option.RequestOption) (*pagination.PageCursor[Session], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[Session](ctx, s.client, "v1/triggers/"+url.PathEscape(triggerID)+"/sessions", opts...)
}
