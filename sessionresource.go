// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apijson"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// SessionResourceService manages what a session can reach: files,
// repositories, and memory stores.
type SessionResourceService struct {
	client *Client
}

// SessionResourceType discriminates a session resource.
type SessionResourceType string

const (
	SessionResourceFile       SessionResourceType = "file"
	SessionResourceRepository SessionResourceType = "github_repository"
	SessionResourceMemory     SessionResourceType = "memory_store"
)

// SessionResourceAccess is whether a session may write to a resource.
type SessionResourceAccess string

const (
	SessionResourceReadOnly  SessionResourceAccess = "read_only"
	SessionResourceReadWrite SessionResourceAccess = "read_write"
)

// SessionResourceCheckout pins a repository resource to a branch or a commit.
type SessionResourceCheckout struct {
	// Type is "branch" or "commit".
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// Branch returns a checkout pinned to a branch.
func Branch(name string) SessionResourceCheckout {
	return SessionResourceCheckout{Type: "branch", Name: name}
}

// Commit returns a checkout pinned to a commit.
func Commit(sha string) SessionResourceCheckout {
	return SessionResourceCheckout{Type: "commit", SHA: sha}
}

// SessionResource is something a session can reach.
//
// The API discriminates three shapes on `type`. They overlap enough that one
// struct carries all of them; which fields are meaningful follows from Type.
type SessionResource struct {
	ID   string              `json:"id,omitempty"`
	Type SessionResourceType `json:"type"`

	// File.
	FileID string `json:"file_id,omitempty"`

	// Repository.
	URL      string                   `json:"url,omitempty"`
	Checkout *SessionResourceCheckout `json:"checkout,omitempty"`

	// Memory store.
	MemoryStoreID string  `json:"memory_store_id,omitempty"`
	Description   string  `json:"description,omitempty"`
	Instructions  *string `json:"instructions,omitempty"`
	Name          *string `json:"name,omitempty"`

	Access    SessionResourceAccess `json:"access,omitempty"`
	MountPath *string               `json:"mount_path,omitempty"`

	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`

	Extra map[string]any `json:"-"`
}

// UnmarshalJSON implements [json.Unmarshaler].
func (r *SessionResource) UnmarshalJSON(data []byte) error {
	type shape SessionResource
	return apijson.UnmarshalWithExtra(data, (*shape)(r), []string{
		"id", "type", "file_id", "url", "checkout", "memory_store_id", "description",
		"instructions", "name", "access", "mount_path", "created_at", "updated_at",
	}, &r.Extra)
}

// SessionResourceParam attaches a resource to a session.
//
// The object is the request body itself rather than being wrapped in a
// "resource" key.
type SessionResourceParam struct {
	Type SessionResourceType `json:"type"`

	// File.
	FileID        param.Opt[string] `json:"file_id,omitzero"`
	MountStrategy param.Opt[string] `json:"mount_strategy,omitzero"`

	// Repository. AuthorizationToken is required for a repository resource and
	// is what Update rotates.
	URL                param.Opt[string]                  `json:"url,omitzero"`
	AuthorizationToken param.Opt[string]                  `json:"authorization_token,omitzero"`
	Checkout           param.Opt[SessionResourceCheckout] `json:"checkout,omitzero"`

	// Memory store.
	MemoryStoreID param.Opt[string] `json:"memory_store_id,omitzero"`

	Access       param.Opt[SessionResourceAccess] `json:"access,omitzero"`
	Instructions param.Opt[string]                `json:"instructions,omitzero"`
	MountPath    param.Opt[string]                `json:"mount_path,omitzero"`

	Extra map[string]any `json:"-"`
}

// FileResource returns a parameter attaching an uploaded file to a session.
func FileResource(fileID string) SessionResourceParam {
	return SessionResourceParam{Type: SessionResourceFile, FileID: param.String(fileID)}
}

// MemoryStoreResource returns a parameter attaching a memory store.
func MemoryStoreResource(memoryStoreID string) SessionResourceParam {
	return SessionResourceParam{Type: SessionResourceMemory, MemoryStoreID: param.String(memoryStoreID)}
}

// MarshalJSON implements [json.Marshaler].
func (p SessionResourceParam) MarshalJSON() ([]byte, error) {
	type shape SessionResourceParam
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// SessionResourceUpdateParams rotates a repository resource's credential.
//
// That is the only mutable field: everything else about a resource is fixed
// once the session can see it.
type SessionResourceUpdateParams struct {
	AuthorizationToken param.Opt[string] `json:"authorization_token,omitzero"`
}

// SessionResourceDeleted is the tombstone a delete returns.
type SessionResourceDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// SessionResourceListParams pages a session's resources.
type SessionResourceListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// List returns a page of the resources attached to a session.
func (s SessionResourceService) List(ctx context.Context, sessionID string, params SessionResourceListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionResource], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[SessionResource](ctx, s.client, "v1/sessions/"+url.PathEscape(sessionID)+"/resources", opts...)
}

// Add attaches a resource to a session.
func (s SessionResourceService) Add(ctx context.Context, sessionID string, params SessionResourceParam, opts ...option.RequestOption) (*SessionResource, error) {
	var resource SessionResource
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/resources"
	if err := s.client.PostJSON(ctx, path, params, &resource, opts...); err != nil {
		return nil, err
	}
	return &resource, nil
}

// Get reads one of a session's resources.
func (s SessionResourceService) Get(ctx context.Context, sessionID, resourceID string, opts ...option.RequestOption) (*SessionResource, error) {
	var resource SessionResource
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/resources/" + url.PathEscape(resourceID)
	if err := s.client.GetJSON(ctx, path, &resource, opts...); err != nil {
		return nil, err
	}
	return &resource, nil
}

// Update rotates a repository resource's authorization token. The verb is POST.
func (s SessionResourceService) Update(ctx context.Context, sessionID, resourceID string, params SessionResourceUpdateParams, opts ...option.RequestOption) (*SessionResource, error) {
	var resource SessionResource
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/resources/" + url.PathEscape(resourceID)
	if err := s.client.PostJSON(ctx, path, params, &resource, opts...); err != nil {
		return nil, err
	}
	return &resource, nil
}

// Delete detaches a resource and returns its tombstone.
func (s SessionResourceService) Delete(ctx context.Context, sessionID, resourceID string, opts ...option.RequestOption) (*SessionResourceDeleted, error) {
	var deleted SessionResourceDeleted
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/resources/" + url.PathEscape(resourceID)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
