// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// MemoryService manages the entries inside a memory store.
type MemoryService struct {
	client *Client
}

// MemoryView selects how much of a memory the server returns.
type MemoryView string

// Memory is one entry in a memory store.
//
// A list can also contain directory markers, which carry a path and the type
// "memory_prefix" and nothing else. Type is what tells them apart.
type Memory struct {
	ID   string `json:"id,omitzero"`
	Type string `json:"type,omitzero"`
	Path string `json:"path"`

	Content       string `json:"content,omitzero"`
	ContentSHA256 string `json:"content_sha256,omitzero"`
	SizeBytes     int64  `json:"size_bytes,omitzero"`

	CreatedAt string `json:"created_at,omitzero"`
	UpdatedAt string `json:"updated_at,omitzero"`
}

// IsPrefix reports whether the entry is a directory marker rather than a
// memory.
func (m Memory) IsPrefix() bool { return m.Type == "memory_prefix" }

// MemoryDeleted is the tombstone a delete returns.
type MemoryDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// MemoryListParams filters and pages the entries in a store.
type MemoryListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]

	// Depth bounds how far below PathPrefix to descend.
	Depth param.Opt[int64]

	// PathPrefix scopes the listing to one directory. It is percent-escaped in
	// the query, so "notes/" is sent as "notes%2F".
	PathPrefix param.Opt[string]

	View param.Opt[MemoryView]
}

// MemoryNewParams creates a memory.
//
// View is a query parameter, not part of the request body. Merging it in would
// send a field the create schema does not declare.
type MemoryNewParams struct {
	Body MemoryNewBody
	View param.Opt[MemoryView]
}

// MemoryNewBody is the JSON payload of a create.
type MemoryNewBody struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// MemoryUpdateParams updates a memory. View is a query parameter - see
// [MemoryNewParams].
type MemoryUpdateParams struct {
	Body MemoryUpdateBody
	View param.Opt[MemoryView]
}

// MemoryUpdateBody is the JSON payload of an update.
type MemoryUpdateBody struct {
	Content param.Opt[string] `json:"content,omitzero"`
	Path    param.Opt[string] `json:"path,omitzero"`
}

// MemoryGetParams selects how much of a memory to return.
type MemoryGetParams struct {
	View param.Opt[MemoryView]
}

// MemoryDeleteParams guards a delete against a concurrent write.
type MemoryDeleteParams struct {
	// ExpectedContentSHA256 makes the delete conditional on the content not
	// having changed since it was read, so a delete cannot silently discard
	// someone else's edit. It is percent-escaped in the query.
	ExpectedContentSHA256 param.Opt[string]
}

// List returns a page of the entries in a store.
func (s MemoryService) List(ctx context.Context, memoryStoreID string, params MemoryListParams, opts ...option.RequestOption) (*pagination.PageCursor[Memory], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendIntQuery(opts, "depth", params.Depth)
	opts = appendStringQuery(opts, "path_prefix", params.PathPrefix)
	opts = appendEnumQuery(opts, "view", params.View)
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memories"
	return ListPage[Memory](ctx, s.client, path, opts...)
}

// Create writes a new memory.
func (s MemoryService) Create(ctx context.Context, memoryStoreID string, params MemoryNewParams, opts ...option.RequestOption) (*Memory, error) {
	opts = appendEnumQuery(opts, "view", params.View)
	var memory Memory
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memories"
	if err := s.client.PostJSON(ctx, path, params.Body, &memory, opts...); err != nil {
		return nil, err
	}
	return &memory, nil
}

// Get reads one memory.
func (s MemoryService) Get(ctx context.Context, memoryStoreID, memoryID string, params MemoryGetParams, opts ...option.RequestOption) (*Memory, error) {
	opts = appendEnumQuery(opts, "view", params.View)
	var memory Memory
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memories/" + url.PathEscape(memoryID)
	if err := s.client.GetJSON(ctx, path, &memory, opts...); err != nil {
		return nil, err
	}
	return &memory, nil
}

// Update rewrites a memory. The verb is POST.
func (s MemoryService) Update(ctx context.Context, memoryStoreID, memoryID string, params MemoryUpdateParams, opts ...option.RequestOption) (*Memory, error) {
	opts = appendEnumQuery(opts, "view", params.View)
	var memory Memory
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memories/" + url.PathEscape(memoryID)
	if err := s.client.PostJSON(ctx, path, params.Body, &memory, opts...); err != nil {
		return nil, err
	}
	return &memory, nil
}

// Delete removes a memory and returns its tombstone.
func (s MemoryService) Delete(ctx context.Context, memoryStoreID, memoryID string, params MemoryDeleteParams, opts ...option.RequestOption) (*MemoryDeleted, error) {
	opts = appendStringQuery(opts, "expected_content_sha256", params.ExpectedContentSHA256)
	var deleted MemoryDeleted
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/memories/" + url.PathEscape(memoryID)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
