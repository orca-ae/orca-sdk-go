// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/vaults/*.test.ts.

func TestVaults(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Vaults.Create(context.Background(), VaultNewParams{
			DisplayName: "My Vault",
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/vaults" {
			t.Errorf("request = %s %s, want POST /v1/vaults", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"display_name":"My Vault"}`)
	})

	t.Run("create with metadata", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Vaults.Create(context.Background(), VaultNewParams{
			DisplayName: "Full Vault",
			Metadata:    map[string]string{"env": "prod", "owner": "alice"},
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t),
			`{"display_name":"Full Vault","metadata":{"env":"prod","owner":"alice"}}`)
	})

	t.Run("retrieve decodes the required and nullable fields", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"v1","display_name":"Test Vault",`+
				`"metadata":{"team":"platform"},"archived_at":null}`), nil
		})

		vault, err := client.Vaults.Get(context.Background(), "vault/slash")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if vault.DisplayName != "Test Vault" {
			t.Errorf("DisplayName = %q, want %q", vault.DisplayName, "Test Vault")
		}
		if vault.Metadata["team"] != "platform" {
			t.Errorf("Metadata = %v, want team=platform", vault.Metadata)
		}
		if vault.ArchivedAt != nil {
			t.Errorf("ArchivedAt = %v, want nil", vault.ArchivedAt)
		}
		if got, want := transport.Only(t).Path(), "/v1/vaults/vault%2Fslash"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("update uses POST", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Update(context.Background(), "v1", VaultUpdateParams{
			DisplayName: param.String("New Name"),
			Metadata:    param.New(map[string]*string{"updated": ptr("yes")}),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"display_name":"New Name","metadata":{"updated":"yes"}}`)
	})

	t.Run("update sends clearing nulls exactly", func(t *testing.T) {
		t.Parallel()

		// display_name:null clears the name and a null metadata value removes
		// that key. Neither is dropped.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Update(context.Background(), "v1", VaultUpdateParams{
			DisplayName: param.Null[string](),
			Metadata:    param.New(map[string]*string{"obsolete": nil}),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"display_name":null,"metadata":{"obsolete":null}}`)
	})

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.List(context.Background(), VaultListParams{
			Limit:           param.Int(10),
			IncludeArchived: param.Bool(true),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		query := transport.Only(t).Query()
		if query.Get("limit") != "10" || query.Get("include_archived") != "true" {
			t.Errorf("query = %v, want limit=10 and include_archived=true", query)
		}
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"vault_abc123","type":"vault_deleted"}`), nil
		})
		deleted, err := client.Vaults.Delete(context.Background(), "vault_abc123")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if got := transport.Only(t).Method; got != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", got)
		}
		if deleted.Type != "vault_deleted" {
			t.Errorf("tombstone type = %q, want vault_deleted", deleted.Type)
		}
	})

	t.Run("archive escapes the id inside the sub-path", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"archived_at":"2026-01-02T00:00:00Z"}`), nil
		})
		vault, err := client.Vaults.Archive(context.Background(), "vault/slash")
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		call := transport.Only(t)
		if got, want := call.Path(), "/v1/vaults/vault%2Fslash/archive"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if vault.ArchivedAt == nil {
			t.Error("ArchivedAt = nil, want the archived vault back")
		}
	})
}

func TestVaultCredentials(t *testing.T) {
	t.Parallel()

	t.Run("list keeps include_archived", func(t *testing.T) {
		t.Parallel()

		// Portable here, unlike on skills and triggers where the overlay
		// removes it.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Credentials.List(context.Background(), "vault/slash", CredentialListParams{
			Limit:           param.Int(10),
			Page:            param.String("p2"),
			IncludeArchived: param.Bool(true),
		})
		if err != nil {
			t.Fatalf("Credentials.List() error = %v", err)
		}
		call := transport.Only(t)
		if got, want := call.Path(), "/v1/vaults/vault%2Fslash/credentials"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		query := call.Query()
		for key, want := range map[string]string{"limit": "10", "page": "p2", "include_archived": "true"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("create with static bearer auth", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Credentials.Create(context.Background(), "v1", CredentialNewParams{
			DisplayName: param.String("New Credential"),
			Auth: CredentialAuthParam{
				Type:         CredentialAuthStaticBearer,
				Token:        param.String("secret"),
				MCPServerURL: param.String("https://mcp.example.com"),
			},
			Metadata: map[string]string{"owner": "platform"},
		})
		if err != nil {
			t.Fatalf("Credentials.Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"display_name":"New Credential",`+
			`"auth":{"type":"static_bearer","token":"secret",`+
			`"mcp_server_url":"https://mcp.example.com"},"metadata":{"owner":"platform"}}`)
	})

	t.Run("create with environment variable auth", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Credentials.Create(context.Background(), "v1", CredentialNewParams{
			Auth: CredentialAuthParam{
				Type:              CredentialAuthEnvironmentVariable,
				SecretName:        param.String("SERVICE_TOKEN"),
				SecretValue:       param.String("secret"),
				Networking:        param.New(LimitedNetworking("api.example.com")),
				InjectionLocation: param.New(CredentialInjectionLocation{Header: param.Bool(true)}),
			},
		})
		if err != nil {
			t.Fatalf("Credentials.Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"auth":{"type":"environment_variable",`+
			`"secret_name":"SERVICE_TOKEN","secret_value":"secret",`+
			`"networking":{"type":"limited","allowed_hosts":["api.example.com"]},`+
			`"injection_location":{"header":true}}}`)
	})

	t.Run("update uses POST", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Credentials.Update(context.Background(), "v1", "c1", CredentialUpdateParams{
			DisplayName: param.String("Renamed"),
			Metadata:    param.New(map[string]*string{"owner": ptr("platform")}),
		})
		if err != nil {
			t.Fatalf("Credentials.Update() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"display_name":"Renamed","metadata":{"owner":"platform"}}`)
	})

	t.Run("an auth update keeps its discriminator even when every secret is null", func(t *testing.T) {
		t.Parallel()

		// Without the type, the server cannot tell which credential kind the
		// request is updating.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Vaults.Credentials.Update(context.Background(), "v1", "c1", CredentialUpdateParams{
			Auth: CredentialAuthParam{
				Type:  CredentialAuthStaticBearer,
				Token: param.Null[string](),
			},
			Metadata: param.New(map[string]*string{"obsolete": nil}),
		})
		if err != nil {
			t.Fatalf("Credentials.Update() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t),
			`{"auth":{"type":"static_bearer","token":null},"metadata":{"obsolete":null}}`)
	})

	t.Run("delete escapes both path parameters", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"cred_abc123","type":"vault_credential_deleted"}`), nil
		})
		deleted, err := client.Vaults.Credentials.Delete(context.Background(), "vault/slash", "cred/slash")
		if err != nil {
			t.Fatalf("Credentials.Delete() error = %v", err)
		}
		want := "/v1/vaults/vault%2Fslash/credentials/cred%2Fslash"
		if got := transport.Only(t).Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if deleted.Type != "vault_credential_deleted" {
			t.Errorf("tombstone type = %q, want vault_credential_deleted", deleted.Type)
		}
	})

	t.Run("archive", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"archived_at":"2026-01-02T00:00:00Z"}`), nil
		})
		credential, err := client.Vaults.Credentials.Archive(context.Background(), "v1", "c1")
		if err != nil {
			t.Fatalf("Credentials.Archive() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/vaults/v1/credentials/c1/archive" {
			t.Errorf("request = %s %s, want POST .../archive", call.Method, call.Path())
		}
		if credential.ArchivedAt == nil {
			t.Error("ArchivedAt = nil, want the archived credential back")
		}
	})

	t.Run("validate posts to mcp_oauth_validate and returns a typed result", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"type":"vault_credential_validation",`+
				`"credential_id":"cred_abc123","vault_id":"vault_abc123",`+
				`"validated_at":"2026-01-02T00:00:00Z","has_refresh_token":true,"status":"valid",`+
				`"mcp_probe":{"method":"initialize","http_response":null},`+
				`"refresh":{"status":"succeeded","http_response":null}}`), nil
		})

		validation, err := client.Vaults.Credentials.Validate(context.Background(), "vault_abc123", "cred_abc123")
		if err != nil {
			t.Fatalf("Credentials.Validate() error = %v", err)
		}

		call := transport.Only(t)
		// The sub-path is mcp_oauth_validate, not a bare /validate.
		want := "/v1/vaults/vault_abc123/credentials/cred_abc123/mcp_oauth_validate"
		if got := call.Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", call.Method)
		}

		// A typed result, not a bare boolean: "invalid" is not one thing, and a
		// caller fixing the credential needs to know which half failed.
		if validation.Status != "valid" {
			t.Errorf("Status = %q, want valid", validation.Status)
		}
		if !validation.HasRefreshToken {
			t.Error("HasRefreshToken = false, want true")
		}
		if validation.MCPProbe.Method != "initialize" {
			t.Errorf("MCPProbe.Method = %q, want initialize", validation.MCPProbe.Method)
		}
		if validation.Refresh.Status != "succeeded" {
			t.Errorf("Refresh.Status = %q, want succeeded", validation.Refresh.Status)
		}
	})
}
