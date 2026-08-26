// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// VaultService manages vaults.
type VaultService struct {
	client *Client

	// Credentials manages the secrets stored in a vault.
	Credentials VaultCredentialService
}

func newVaultService(client *Client) VaultService {
	return VaultService{client: client, Credentials: VaultCredentialService{client: client}}
}

// Vault holds the credentials an agent may use.
type Vault struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`

	// ArchivedAt is nil while the vault is active.
	ArchivedAt *string `json:"archived_at"`
}

// VaultDeleted is the tombstone a delete returns.
type VaultDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// VaultNewParams creates a vault.
type VaultNewParams struct {
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitzero"`
}

// VaultUpdateParams updates a vault.
//
// A null DisplayName clears the name, and a nil metadata value removes that
// key. Neither is dropped on the way out.
type VaultUpdateParams struct {
	DisplayName param.Opt[string]             `json:"display_name,omitzero"`
	Metadata    param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// VaultListParams filters and pages vaults. The overlay removes the provider
// filter, so it is absent here.
type VaultListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// Create creates a vault.
func (s VaultService) Create(ctx context.Context, params VaultNewParams, opts ...option.RequestOption) (*Vault, error) {
	var vault Vault
	if err := s.client.PostJSON(ctx, "v1/vaults", params, &vault, opts...); err != nil {
		return nil, err
	}
	return &vault, nil
}

// Get reads a vault.
func (s VaultService) Get(ctx context.Context, vaultID string, opts ...option.RequestOption) (*Vault, error) {
	var vault Vault
	if err := s.client.GetJSON(ctx, "v1/vaults/"+url.PathEscape(vaultID), &vault, opts...); err != nil {
		return nil, err
	}
	return &vault, nil
}

// Update updates a vault. The verb is POST.
func (s VaultService) Update(ctx context.Context, vaultID string, params VaultUpdateParams, opts ...option.RequestOption) (*Vault, error) {
	var vault Vault
	if err := s.client.PostJSON(ctx, "v1/vaults/"+url.PathEscape(vaultID), params, &vault, opts...); err != nil {
		return nil, err
	}
	return &vault, nil
}

// List returns a page of vaults.
func (s VaultService) List(ctx context.Context, params VaultListParams, opts ...option.RequestOption) (*pagination.PageCursor[Vault], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[Vault](ctx, s.client, "v1/vaults", opts...)
}

// Delete permanently deletes a vault and returns its tombstone.
func (s VaultService) Delete(ctx context.Context, vaultID string, opts ...option.RequestOption) (*VaultDeleted, error) {
	var deleted VaultDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/vaults/"+url.PathEscape(vaultID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Archive archives a vault and returns it.
func (s VaultService) Archive(ctx context.Context, vaultID string, opts ...option.RequestOption) (*Vault, error) {
	var vault Vault
	if err := s.client.PostJSON(ctx, "v1/vaults/"+url.PathEscape(vaultID)+"/archive", nil, &vault, opts...); err != nil {
		return nil, err
	}
	return &vault, nil
}
