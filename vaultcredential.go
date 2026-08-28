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

// VaultCredentialService manages the secrets stored in a vault.
type VaultCredentialService struct {
	client *Client
}

// CredentialAuthType discriminates how a credential authenticates.
type CredentialAuthType string

const (
	CredentialAuthStaticBearer        CredentialAuthType = "static_bearer"
	CredentialAuthMCPOAuth            CredentialAuthType = "mcp_oauth"
	CredentialAuthEnvironmentVariable CredentialAuthType = "environment_variable"
	CredentialAuthProvider            CredentialAuthType = "provider"
)

// CredentialNetworking bounds which hosts a credential may be sent to.
//
// Limiting it is what stops a secret minted for one service from being usable
// against another, so an unrestricted credential is a deliberate choice rather
// than a default.
type CredentialNetworking struct {
	// Type is "limited" or "unrestricted".
	Type         string   `json:"type"`
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
}

// LimitedNetworking returns networking restricted to the given hosts.
func LimitedNetworking(hosts ...string) CredentialNetworking {
	return CredentialNetworking{Type: "limited", AllowedHosts: hosts}
}

// UnrestrictedNetworking returns networking with no host restriction.
func UnrestrictedNetworking() CredentialNetworking {
	return CredentialNetworking{Type: "unrestricted"}
}

// CredentialInjectionLocation is where a secret is placed in an outgoing
// request.
type CredentialInjectionLocation struct {
	Header param.Opt[bool] `json:"header,omitzero"`
	Body   param.Opt[bool] `json:"body,omitzero"`
}

// CredentialTokenEndpointAuth is how a refresh request authenticates itself.
type CredentialTokenEndpointAuth struct {
	// Type is "none", "client_secret_basic", or "client_secret_post".
	Type         string            `json:"type"`
	ClientSecret param.Opt[string] `json:"client_secret,omitzero"`
}

// CredentialOAuthRefresh is what a credential needs to renew itself.
type CredentialOAuthRefresh struct {
	RefreshToken      param.Opt[string]                      `json:"refresh_token,omitzero"`
	TokenEndpoint     param.Opt[string]                      `json:"token_endpoint,omitzero"`
	ClientID          param.Opt[string]                      `json:"client_id,omitzero"`
	TokenEndpointAuth param.Opt[CredentialTokenEndpointAuth] `json:"token_endpoint_auth,omitzero"`
	Resource          param.Opt[string]                      `json:"resource,omitzero"`
	Scope             param.Opt[string]                      `json:"scope,omitzero"`
}

// CredentialAuthParam is how a credential authenticates, on create or update.
//
// The API discriminates four shapes on `type`, and one struct carries them all.
// Type is always required - even an update that nulls every secret field must
// still say which shape it is updating, or the server cannot tell which
// credential kind the request is for.
type CredentialAuthParam struct {
	Type CredentialAuthType `json:"type"`

	// Static bearer and MCP OAuth.
	Token        param.Opt[string] `json:"token,omitzero"`
	AccessToken  param.Opt[string] `json:"access_token,omitzero"`
	MCPServerURL param.Opt[string] `json:"mcp_server_url,omitzero"`
	ExpiresAt    param.Opt[string] `json:"expires_at,omitzero"`

	Refresh param.Opt[CredentialOAuthRefresh] `json:"refresh,omitzero"`

	// Environment variable.
	SecretName        param.Opt[string]                      `json:"secret_name,omitzero"`
	SecretValue       param.Opt[string]                      `json:"secret_value,omitzero"`
	Networking        param.Opt[CredentialNetworking]        `json:"networking,omitzero"`
	InjectionLocation param.Opt[CredentialInjectionLocation] `json:"injection_location,omitzero"`

	// Provider.
	Provider  param.Opt[string] `json:"provider,omitzero"`
	Scheme    param.Opt[string] `json:"scheme,omitzero"`
	LogicalID param.Opt[string] `json:"logical_id,omitzero"`
	Version   param.Opt[string] `json:"version,omitzero"`

	Extra map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (a CredentialAuthParam) MarshalJSON() ([]byte, error) {
	type shape CredentialAuthParam
	return apijson.MarshalWithExtra(shape(a), a.Extra)
}

// IsZero reports whether no auth was supplied.
func (a CredentialAuthParam) IsZero() bool { return a.Type == "" }

// VaultCredentialAuth is how a credential authenticates, as the API reports it.
//
// Secret material is never echoed back: a response carries the shape and the
// non-secret fields only.
type VaultCredentialAuth struct {
	Type CredentialAuthType `json:"type"`

	MCPServerURL string  `json:"mcp_server_url,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`

	SecretName        string                `json:"secret_name,omitempty"`
	Networking        *CredentialNetworking `json:"networking,omitempty"`
	InjectionLocation map[string]bool       `json:"injection_location,omitempty"`

	Provider  string `json:"provider,omitempty"`
	Scheme    string `json:"scheme,omitempty"`
	LogicalID string `json:"logical_id,omitempty"`
	Version   string `json:"version,omitempty"`

	Extra map[string]any `json:"-"`
}

// UnmarshalJSON implements [json.Unmarshaler].
func (a *VaultCredentialAuth) UnmarshalJSON(data []byte) error {
	type shape VaultCredentialAuth
	return apijson.UnmarshalWithExtra(data, (*shape)(a), []string{
		"type", "mcp_server_url", "expires_at", "secret_name", "networking",
		"injection_location", "provider", "scheme", "logical_id", "version", "refresh",
	}, &a.Extra)
}

// VaultCredential is one secret stored in a vault.
type VaultCredential struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	VaultID     string              `json:"vault_id"`
	DisplayName *string             `json:"display_name"`
	Auth        VaultCredentialAuth `json:"auth"`
	Metadata    map[string]string   `json:"metadata"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
	ArchivedAt  *string             `json:"archived_at"`
}

// VaultCredentialDeleted is the tombstone a delete returns.
type VaultCredentialDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// CredentialValidationHTTPResponse is what the server saw when it probed.
type CredentialValidationHTTPResponse struct {
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type"`
	Body          string `json:"body"`
	BodyTruncated bool   `json:"body_truncated"`
}

// CredentialValidationMCPProbe is the result of the MCP handshake attempt.
type CredentialValidationMCPProbe struct {
	Method       string                            `json:"method"`
	HTTPResponse *CredentialValidationHTTPResponse `json:"http_response"`
}

// CredentialValidationRefresh is the result of the token refresh attempt.
type CredentialValidationRefresh struct {
	Status       string                            `json:"status"`
	HTTPResponse *CredentialValidationHTTPResponse `json:"http_response"`
}

// CredentialValidation is the outcome of validating a credential.
//
// It is a typed result rather than a boolean because "invalid" is not one
// thing: the probe and the refresh fail for different reasons, and a caller
// fixing the credential needs to know which.
type CredentialValidation struct {
	Type            string                       `json:"type"`
	CredentialID    string                       `json:"credential_id"`
	VaultID         string                       `json:"vault_id"`
	ValidatedAt     string                       `json:"validated_at"`
	HasRefreshToken bool                         `json:"has_refresh_token"`
	Status          string                       `json:"status"`
	MCPProbe        CredentialValidationMCPProbe `json:"mcp_probe"`
	Refresh         CredentialValidationRefresh  `json:"refresh"`
}

// CredentialNewParams creates a credential.
type CredentialNewParams struct {
	DisplayName param.Opt[string]   `json:"display_name,omitzero"`
	Auth        CredentialAuthParam `json:"auth,omitzero"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
}

// CredentialUpdateParams updates a credential.
type CredentialUpdateParams struct {
	DisplayName param.Opt[string]             `json:"display_name,omitzero"`
	Auth        CredentialAuthParam           `json:"auth,omitzero"`
	Metadata    param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// CredentialListParams filters and pages a vault's credentials.
//
// include_archived is portable here, unlike on skills and triggers where the
// overlay removes it.
type CredentialListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// List returns a page of a vault's credentials.
func (s VaultCredentialService) List(ctx context.Context, vaultID string, params CredentialListParams, opts ...option.RequestOption) (*pagination.PageCursor[VaultCredential], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[VaultCredential](ctx, s.client, "v1/vaults/"+url.PathEscape(vaultID)+"/credentials", opts...)
}

// Create stores a new credential in a vault.
func (s VaultCredentialService) Create(ctx context.Context, vaultID string, params CredentialNewParams, opts ...option.RequestOption) (*VaultCredential, error) {
	var credential VaultCredential
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials"
	if err := s.client.PostJSON(ctx, path, params, &credential, opts...); err != nil {
		return nil, err
	}
	return &credential, nil
}

// Get reads a credential. Secret material is not echoed back.
func (s VaultCredentialService) Get(ctx context.Context, vaultID, credentialID string, opts ...option.RequestOption) (*VaultCredential, error) {
	var credential VaultCredential
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID)
	if err := s.client.GetJSON(ctx, path, &credential, opts...); err != nil {
		return nil, err
	}
	return &credential, nil
}

// Update updates a credential. The verb is POST.
func (s VaultCredentialService) Update(ctx context.Context, vaultID, credentialID string, params CredentialUpdateParams, opts ...option.RequestOption) (*VaultCredential, error) {
	var credential VaultCredential
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID)
	if err := s.client.PostJSON(ctx, path, params, &credential, opts...); err != nil {
		return nil, err
	}
	return &credential, nil
}

// Delete permanently deletes a credential and returns its tombstone.
func (s VaultCredentialService) Delete(ctx context.Context, vaultID, credentialID string, opts ...option.RequestOption) (*VaultCredentialDeleted, error) {
	var deleted VaultCredentialDeleted
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Archive archives a credential and returns it.
func (s VaultCredentialService) Archive(ctx context.Context, vaultID, credentialID string, opts ...option.RequestOption) (*VaultCredential, error) {
	var credential VaultCredential
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID) + "/archive"
	if err := s.client.PostJSON(ctx, path, nil, &credential, opts...); err != nil {
		return nil, err
	}
	return &credential, nil
}

// Validate checks a credential's MCP OAuth configuration.
//
// The sub-path is mcp_oauth_validate rather than a bare /validate, because it
// exercises one specific mechanism rather than validating the credential in
// general.
func (s VaultCredentialService) Validate(ctx context.Context, vaultID, credentialID string, opts ...option.RequestOption) (*CredentialValidation, error) {
	var validation CredentialValidation
	path := "v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credentialID) + "/mcp_oauth_validate"
	if err := s.client.PostJSON(ctx, path, nil, &validation, opts...); err != nil {
		return nil, err
	}
	return &validation, nil
}
