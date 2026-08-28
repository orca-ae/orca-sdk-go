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

// EnvironmentService manages the environments sessions run in.
type EnvironmentService struct {
	client *Client
}

// EnvironmentScope is how widely an environment is shared.
type EnvironmentScope string

const (
	EnvironmentScopeOrganization EnvironmentScope = "organization"
	EnvironmentScopeAccount      EnvironmentScope = "account"
)

// EnvironmentPackages lists the packages to install in an environment.
//
// Every list is optional and separately nullable: sending pip as null clears
// the pip packages while leaving apt alone, which omitting it would not do.
type EnvironmentPackages struct {
	Type param.Opt[string]   `json:"type,omitzero"`
	Pip  param.Opt[[]string] `json:"pip,omitzero"`
	Apt  param.Opt[[]string] `json:"apt,omitzero"`
	Npm  param.Opt[[]string] `json:"npm,omitzero"`

	Extra map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (p EnvironmentPackages) MarshalJSON() ([]byte, error) {
	type shape EnvironmentPackages
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (p *EnvironmentPackages) UnmarshalJSON(data []byte) error {
	type shape EnvironmentPackages
	return apijson.UnmarshalWithExtra(data, (*shape)(p), []string{"type", "pip", "apt", "npm"}, &p.Extra)
}

// EnvironmentNetworking bounds what an environment may reach.
//
// The flags are individually nullable so an update can clear one without
// restating the rest.
type EnvironmentNetworking struct {
	// Type is "limited" or "unrestricted".
	Type string `json:"type,omitzero"`

	AllowedHosts         param.Opt[[]string] `json:"allowed_hosts,omitzero"`
	AllowMCPServers      param.Opt[bool]     `json:"allow_mcp_servers,omitzero"`
	AllowPackageManagers param.Opt[bool]     `json:"allow_package_managers,omitzero"`

	Extra map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (n EnvironmentNetworking) MarshalJSON() ([]byte, error) {
	type shape EnvironmentNetworking
	return apijson.MarshalWithExtra(shape(n), n.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (n *EnvironmentNetworking) UnmarshalJSON(data []byte) error {
	type shape EnvironmentNetworking
	return apijson.UnmarshalWithExtra(data, (*shape)(n),
		[]string{"type", "allowed_hosts", "allow_mcp_servers", "allow_package_managers"}, &n.Extra)
}

// EnvironmentConfig describes how an environment is built and what it may
// reach.
//
// Packages, networking, image and target live here rather than at the top level
// of a request: the overlay removes the flattened forms, because they are not
// portable across both supported backends. The discriminator is optional, so a
// config that only adjusts packages need not claim a deployment kind.
type EnvironmentConfig struct {
	Type param.Opt[string] `json:"type,omitzero"`

	Packages   param.Opt[EnvironmentPackages]   `json:"packages,omitzero"`
	Networking param.Opt[EnvironmentNetworking] `json:"networking,omitzero"`
	Image      param.Opt[string]                `json:"image,omitzero"`
	Target     param.Opt[string]                `json:"target,omitzero"`

	Extra map[string]any `json:"-"`
}

// IsZero reports whether no config was supplied.
func (c EnvironmentConfig) IsZero() bool {
	return !c.Type.Valid() && c.Packages.IsZero() && c.Networking.IsZero() &&
		!c.Image.Valid() && !c.Target.Valid() && c.Extra == nil
}

// MarshalJSON implements [json.Marshaler].
func (c EnvironmentConfig) MarshalJSON() ([]byte, error) {
	type shape EnvironmentConfig
	return apijson.MarshalWithExtra(shape(c), c.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (c *EnvironmentConfig) UnmarshalJSON(data []byte) error {
	type shape EnvironmentConfig
	return apijson.UnmarshalWithExtra(data, (*shape)(c),
		[]string{"type", "packages", "networking", "image", "target"}, &c.Extra)
}

// Environment is a place sessions run.
type Environment struct {
	ID          string            `json:"id"`
	Type        string            `json:"type,omitzero"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Config      EnvironmentConfig `json:"config"`
	Metadata    map[string]string `json:"metadata"`
	Scope       EnvironmentScope  `json:"scope"`
	CreatedAt   string            `json:"created_at,omitzero"`
	UpdatedAt   string            `json:"updated_at,omitzero"`
	ArchivedAt  *string           `json:"archived_at"`
}

// EnvironmentDeleted is the tombstone a delete returns.
type EnvironmentDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// EnvironmentNewParams creates an environment.
type EnvironmentNewParams struct {
	Name        string                      `json:"name"`
	Description param.Opt[string]           `json:"description,omitzero"`
	Config      EnvironmentConfig           `json:"config,omitzero"`
	Metadata    map[string]string           `json:"metadata,omitzero"`
	Scope       param.Opt[EnvironmentScope] `json:"scope,omitzero"`
}

// EnvironmentUpdateParams updates an environment.
//
// Only the keys the caller set are sent: an omitted description is absent from
// the body, not null, because those mean different things.
type EnvironmentUpdateParams struct {
	Name        param.Opt[string]             `json:"name,omitzero"`
	Description param.Opt[string]             `json:"description,omitzero"`
	Config      EnvironmentConfig             `json:"config,omitzero"`
	Metadata    param.Opt[map[string]*string] `json:"metadata,omitzero"`
	Scope       param.Opt[EnvironmentScope]   `json:"scope,omitzero"`
}

// EnvironmentListParams filters and pages environments. The overlay removes the
// provider filter.
type EnvironmentListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// Create creates an environment.
func (s EnvironmentService) Create(ctx context.Context, params EnvironmentNewParams, opts ...option.RequestOption) (*Environment, error) {
	var environment Environment
	if err := s.client.PostJSON(ctx, "v1/environments", params, &environment, opts...); err != nil {
		return nil, err
	}
	return &environment, nil
}

// Get reads an environment.
func (s EnvironmentService) Get(ctx context.Context, environmentID string, opts ...option.RequestOption) (*Environment, error) {
	var environment Environment
	if err := s.client.GetJSON(ctx, "v1/environments/"+url.PathEscape(environmentID), &environment, opts...); err != nil {
		return nil, err
	}
	return &environment, nil
}

// Update updates an environment. The verb is POST.
func (s EnvironmentService) Update(ctx context.Context, environmentID string, params EnvironmentUpdateParams, opts ...option.RequestOption) (*Environment, error) {
	var environment Environment
	if err := s.client.PostJSON(ctx, "v1/environments/"+url.PathEscape(environmentID), params, &environment, opts...); err != nil {
		return nil, err
	}
	return &environment, nil
}

// List returns a page of environments.
func (s EnvironmentService) List(ctx context.Context, params EnvironmentListParams, opts ...option.RequestOption) (*pagination.PageCursor[Environment], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[Environment](ctx, s.client, "v1/environments", opts...)
}

// Delete permanently deletes an environment and returns its tombstone.
func (s EnvironmentService) Delete(ctx context.Context, environmentID string, opts ...option.RequestOption) (*EnvironmentDeleted, error) {
	var deleted EnvironmentDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/environments/"+url.PathEscape(environmentID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Archive archives an environment and returns it.
func (s EnvironmentService) Archive(ctx context.Context, environmentID string, opts ...option.RequestOption) (*Environment, error) {
	var environment Environment
	path := "v1/environments/" + url.PathEscape(environmentID) + "/archive"
	if err := s.client.PostJSON(ctx, path, nil, &environment, opts...); err != nil {
		return nil, err
	}
	return &environment, nil
}
