// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// MemoryVersionService reads the audit trail of changes to a memory store.
type MemoryVersionService struct {
	client *Client
}

// MemoryVersionOperation is what a version records.
type MemoryVersionOperation string

// MemoryVersion is one recorded change to a memory.
type MemoryVersion struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	MemoryID  string                 `json:"memory_id,omitempty"`
	Path      string                 `json:"path,omitempty"`
	Operation MemoryVersionOperation `json:"operation,omitempty"`
	Content   string                 `json:"content,omitempty"`
	APIKeyID  string                 `json:"api_key_id,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`

	// RedactedAt is set once the version's content has been redacted.
	RedactedAt *string `json:"redacted_at,omitempty"`
}

// MemoryVersionListParams filters and pages the audit trail.
//
// session_id is deliberately absent: it needs local-to-provider ID translation
// that is not portable across both supported backends, and the overlay removes
// it.
type MemoryVersionListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]

	MemoryID  param.Opt[string]
	APIKeyID  param.Opt[string]
	Operation param.Opt[MemoryVersionOperation]

	// The bracket keys are literal parameter names, not a nested object.
	CreatedAtGTE param.Opt[string]
	CreatedAtLTE param.Opt[string]

	View param.Opt[MemoryView]
}

// MemoryVersionGetParams selects how much of a version to return.
type MemoryVersionGetParams struct {
	View param.Opt[MemoryView]
}

// List returns a page of the audit trail.
func (s MemoryVersionService) List(ctx context.Context, memoryStoreID string, params MemoryVersionListParams, opts ...option.RequestOption) (*pagination.PageCursor[MemoryVersion], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendStringQuery(opts, "memory_id", params.MemoryID)
	opts = appendStringQuery(opts, "api_key_id", params.APIKeyID)
	opts = appendEnumQuery(opts, "operation", params.Operation)
	opts = appendStringQuery(opts, "created_at[gte]", params.CreatedAtGTE)
	opts = appendStringQuery(opts, "created_at[lte]", params.CreatedAtLTE)
	opts = appendEnumQuery(opts, "view", params.View)
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memory_versions"
	return ListPage[MemoryVersion](ctx, s.client, path, opts...)
}

// Get reads one recorded change.
func (s MemoryVersionService) Get(ctx context.Context, memoryStoreID, versionID string, params MemoryVersionGetParams, opts ...option.RequestOption) (*MemoryVersion, error) {
	opts = appendEnumQuery(opts, "view", params.View)
	var version MemoryVersion
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memory_versions/" + url.PathEscape(versionID)
	if err := s.client.GetJSON(ctx, path, &version, opts...); err != nil {
		return nil, err
	}
	return &version, nil
}

// Redact removes a version's content and returns the redacted version.
func (s MemoryVersionService) Redact(ctx context.Context, memoryStoreID, versionID string, opts ...option.RequestOption) (*MemoryVersion, error) {
	var version MemoryVersion
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memory_versions/" + url.PathEscape(versionID) + "/redact"
	if err := s.client.PostJSON(ctx, path, nil, &version, opts...); err != nil {
		return nil, err
	}
	return &version, nil
}
