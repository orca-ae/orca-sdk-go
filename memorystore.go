// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// MemoryStoreService manages memory stores.
type MemoryStoreService struct {
	client *Client

	// Memories manages the entries inside a store.
	Memories MemoryService

	// MemoryVersions reads the audit trail of changes to a store.
	MemoryVersions MemoryVersionService
}

func newMemoryStoreService(client *Client) MemoryStoreService {
	return MemoryStoreService{
		client:         client,
		Memories:       MemoryService{client: client},
		MemoryVersions: MemoryVersionService{client: client},
	}
}

// MemoryStore is a durable store an agent can read from and write to across
// sessions.
type MemoryStore struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// MemoryStoreDeleted is the tombstone a delete returns.
type MemoryStoreDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// MemoryStoreNewParams creates a memory store.
type MemoryStoreNewParams struct {
	Name        string            `json:"name"`
	Description param.Opt[string] `json:"description,omitzero"`
	Metadata    map[string]string `json:"metadata,omitzero"`
}

// MemoryStoreUpdateParams updates a memory store.
type MemoryStoreUpdateParams struct {
	Name        param.Opt[string] `json:"name,omitzero"`
	Description param.Opt[string] `json:"description,omitzero"`

	// Metadata patches individual keys. A nil value removes its key.
	Metadata param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// MemoryStoreListParams filters and pages memory stores.
//
// The overlay removes the created_at and provider filters, so they are absent
// here rather than sent and ignored.
type MemoryStoreListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// Create creates a memory store.
func (s MemoryStoreService) Create(ctx context.Context, params MemoryStoreNewParams, opts ...option.RequestOption) (*MemoryStore, error) {
	var store MemoryStore
	if err := s.client.PostJSON(ctx, "v1/memory_stores", params, &store, opts...); err != nil {
		return nil, err
	}
	return &store, nil
}

// Get reads a memory store.
func (s MemoryStoreService) Get(ctx context.Context, memoryStoreID string, opts ...option.RequestOption) (*MemoryStore, error) {
	var store MemoryStore
	if err := s.client.GetJSON(ctx, "v1/memory_stores/"+url.PathEscape(memoryStoreID), &store, opts...); err != nil {
		return nil, err
	}
	return &store, nil
}

// Update updates a memory store. The verb is POST.
func (s MemoryStoreService) Update(ctx context.Context, memoryStoreID string, params MemoryStoreUpdateParams, opts ...option.RequestOption) (*MemoryStore, error) {
	var store MemoryStore
	if err := s.client.PostJSON(ctx, "v1/memory_stores/"+url.PathEscape(memoryStoreID), params, &store, opts...); err != nil {
		return nil, err
	}
	return &store, nil
}

// List returns a page of memory stores.
func (s MemoryStoreService) List(ctx context.Context, params MemoryStoreListParams, opts ...option.RequestOption) (*pagination.PageCursor[MemoryStore], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[MemoryStore](ctx, s.client, "v1/memory_stores", opts...)
}

// Delete permanently deletes a memory store and returns its tombstone.
func (s MemoryStoreService) Delete(ctx context.Context, memoryStoreID string, opts ...option.RequestOption) (*MemoryStoreDeleted, error) {
	var deleted MemoryStoreDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/memory_stores/"+url.PathEscape(memoryStoreID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Archive archives a memory store and returns it.
func (s MemoryStoreService) Archive(ctx context.Context, memoryStoreID string, opts ...option.RequestOption) (*MemoryStore, error) {
	var store MemoryStore
	path := "v1/memory_stores/" + url.PathEscape(memoryStoreID) + "/archive"
	if err := s.client.PostJSON(ctx, path, nil, &store, opts...); err != nil {
		return nil, err
	}
	return &store, nil
}
