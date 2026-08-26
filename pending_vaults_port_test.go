// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "testing"

// Ported from orca-sdk-typescript
// tests/api-resources/vaults/{vaults,credentials}.test.ts.
//
// Specifies the typed Vaults resource and its credentials sub-resource, which
// this SDK does not implement yet — see managed_agents.go, an untyped
// passthrough. See pending_agents_port_test.go for the shared table type.

// pendingVaultsSpec is the wire contract of the typed Vaults resource.
var pendingVaultsSpec = []pendingPortOp{
	{
		Name:   "vaults.create",
		Method: "POST",
		Path:   "/v1/vaults",
		Body:   `{"display_name":"My Vault"}`,
	},
	{
		Name:   "vaults.create (all optional fields)",
		Method: "POST",
		Path:   "/v1/vaults",
		Body:   `{"display_name":"Full Vault","metadata":{"env":"prod","owner":"alice"}}`,
	},
	{
		Name:     "vaults.retrieve",
		Method:   "GET",
		Path:     "/v1/vaults/{vaultID}",
		Response: `{"display_name":"Test Vault","metadata":{"team":"platform"},"archived_at":null}`,
		Note: "display_name is a required string and metadata a required map; " +
			"archived_at is nullable. The ID is escaped",
	},
	{
		Name:   "vaults.update",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}",
		Body:   `{"display_name":"New Name","metadata":{"updated":"yes"}}`,
		Note:   "POST, not PUT",
	},
	{
		Name:   "vaults.update (clearing nulls)",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}",
		Body:   `{"display_name":null,"metadata":{"obsolete":null}}`,
		Note: "the body is exactly this — display_name:null clears the name and a " +
			"null metadata value removes that key; neither is dropped",
	},
	{
		Name:   "vaults.list",
		Method: "GET",
		Path:   "/v1/vaults",
		Query:  []string{"limit", "include_archived"},
		Note:   "page-token pagination via next_page; the overlay removes the provider filter",
	},
	{
		Name:     "vaults.delete",
		Method:   "DELETE",
		Path:     "/v1/vaults/{vaultID}",
		Response: `{"id":"vault_abc123","type":"vault_deleted"}`,
	},
	{
		Name:     "vaults.archive",
		Method:   "POST",
		Path:     "/v1/vaults/{vaultID}/archive",
		Response: `{"archived_at":"2026-01-02T00:00:00Z"}`,
		Note: "an /archive sub-path; the ID is escaped inside it " +
			"(vault/slash => vault%2Fslash/archive)",
	},
}

// pendingVaultCredentialsSpec is the wire contract of the typed vault
// credentials sub-resource.
var pendingVaultCredentialsSpec = []pendingPortOp{
	{
		Name:   "vaults.credentials.list",
		Method: "GET",
		Path:   "/v1/vaults/{vaultID}/credentials",
		Query:  []string{"limit", "page", "include_archived"},
		Note: "include_archived IS part of the portable credential list params " +
			"(the overlay keeps it here, unlike skills and triggers). Page-token " +
			"pagination; the vault ID is escaped",
	},
	{
		Name:   "vaults.credentials.create",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}/credentials",
		Body: `{"display_name":"New Credential","auth":{"type":"static_bearer",` +
			`"token":"secret","mcp_server_url":"https://mcp.example.com"},` +
			`"metadata":{"owner":"platform"}}`,
		Note: "auth is a discriminated union keyed on `type`",
	},
	{
		Name:   "vaults.credentials.create (environment_variable auth)",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}/credentials",
		Body: `{"auth":{"type":"environment_variable","secret_name":"SERVICE_TOKEN",` +
			`"secret_value":"secret","networking":{"type":"limited",` +
			`"allowed_hosts":["api.example.com"]},"injection_location":{"header":true}}}`,
	},
	{
		Name:   "vaults.credentials.retrieve",
		Method: "GET",
		Path:   "/v1/vaults/{vaultID}/credentials/{credentialID}",
	},
	{
		Name:   "vaults.credentials.update",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}/credentials/{credentialID}",
		Body:   `{"display_name":"Renamed","metadata":{"owner":"platform"}}`,
		Note:   "POST, not PUT — updateCredential is POST per the OpenAPI spec",
	},
	{
		Name:   "vaults.credentials.update (nullable secret, metadata patch)",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}/credentials/{credentialID}",
		Body:   `{"auth":{"type":"static_bearer","token":null},"metadata":{"obsolete":null}}`,
		Note: "the body is exactly this. An auth update still requires the `type` " +
			"discriminator even when every secret field is null",
	},
	{
		Name:     "vaults.credentials.delete",
		Method:   "DELETE",
		Path:     "/v1/vaults/{vaultID}/credentials/{credentialID}",
		Response: `{"id":"cred_abc123","type":"vault_credential_deleted"}`,
		Note:     "both path parameters are escaped",
	},
	{
		Name:     "vaults.credentials.archive",
		Method:   "POST",
		Path:     "/v1/vaults/{vaultID}/credentials/{credentialID}/archive",
		Response: `{"archived_at":"2026-01-02T00:00:00Z"}`,
	},
	{
		Name:   "vaults.credentials.validate",
		Method: "POST",
		Path:   "/v1/vaults/{vaultID}/credentials/{credentialID}/mcp_oauth_validate",
		Response: `{"type":"vault_credential_validation","credential_id":"cred_abc123",` +
			`"vault_id":"vault_abc123","validated_at":"2026-01-02T00:00:00Z",` +
			`"has_refresh_token":true,"status":"valid",` +
			`"mcp_probe":{"method":"initialize","http_response":null},` +
			`"refresh":{"status":"succeeded","http_response":null}}`,
		Note: "the sub-path is mcp_oauth_validate (not /validate); the result is a " +
			"typed CredentialValidation, not a bare boolean",
	},
}

func TestPendingVaults(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingVaultsSpec)
}

func TestPendingVaultCredentials(t *testing.T) {
	t.Skip(pendingManagedAgents)

	pendingPortSurface(t, pendingVaultCredentialsSpec)
}
