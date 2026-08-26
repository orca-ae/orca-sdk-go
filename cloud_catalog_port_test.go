// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Ported from orca-sdk-typescript
// tests/api-resources/cloud/discovery-health-catalog.test.ts and
// tests/api-resources/cloud/agents/providers.test.ts.
//
// Four small read-only surfaces share this file because they share a shape:
// resource discovery for the group itself, the group's health probes, the
// built-in connector catalog, and managed-agent provider discovery.

// cloudCatalogOperations covers group discovery, health, catalog, and providers.
func cloudCatalogOperations() []cloudOperation {
	const (
		health   = "/apis/cloud.sn.io/v1/health"
		catalog  = "/apis/cloud.sn.io/v1/catalog"
		provider = "/apis/cloud.sn.io/v1/agents/providers"
	)

	return []cloudOperation{
		{
			operationID: "getApiResources",
			name:        "apiResources.list",
			method:      "GET",
			// The group's own resource list is the prefix itself, trailing
			// slash included - it is not a resource under the prefix.
			path: "/apis/cloud.sn.io/v1/",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetCloudAPIResources(ctx)
				return err
			},
		},
		{
			operationID: "health",
			name:        "health.check",
			method:      "GET",
			path:        health,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewHealthClient(client).Health(ctx)
				return err
			},
		},
		{
			operationID: "isInitialized",
			name:        "health.ready",
			method:      "GET",
			path:        health + "/ready",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewHealthClient(client).Ready(ctx)
				return err
			},
		},
		{
			operationID: "liveness",
			name:        "health.live",
			method:      "GET",
			path:        health + "/live",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewHealthClient(client).Live(ctx)
				return err
			},
		},
		{
			operationID: "getKafkaConnectorList",
			name:        "catalog.kafka.list",
			method:      "GET",
			path:        catalog + "/kafka",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).ListKafkaConnectors(ctx)
				return err
			},
		},
		{
			operationID: "getKafkaConnectorConfigDefinition",
			name:        "catalog.kafka.retrieve",
			method:      "GET",
			path:        catalog + "/kafka/plug%2Fin",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).GetKafkaConfigDefinition(ctx, "plug/in")
				return err
			},
		},
		{
			operationID: "getSinkList",
			name:        "catalog.sinks.list",
			method:      "GET",
			path:        catalog + "/sinks",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).ListSinks(ctx)
				return err
			},
		},
		{
			operationID: "getSinkConfigDefinition",
			name:        "catalog.sinks.retrieve",
			method:      "GET",
			path:        catalog + "/sinks/plug%2Fin",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).GetSinkConfigDefinition(ctx, "plug/in")
				return err
			},
		},
		{
			operationID: "getSourceList",
			name:        "catalog.sources.list",
			method:      "GET",
			path:        catalog + "/sources",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).ListSources(ctx)
				return err
			},
		},
		{
			operationID: "getSourceConfigDefinition",
			name:        "catalog.sources.retrieve",
			method:      "GET",
			path:        catalog + "/sources/plug%2Fin",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewCatalogClient(client).GetSourceConfigDefinition(ctx, "plug/in")
				return err
			},
		},
		{
			operationID: "listProviders",
			name:        "agents.providers.list",
			method:      "GET",
			path:        provider,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewProvidersClient(client).List(ctx)
				return err
			},
		},
		{
			operationID: "getProvider",
			name:        "agents.providers.retrieve",
			method:      "GET",
			path:        provider + "/openai",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewProvidersClient(client).Get(ctx, "openai")
				return err
			},
		},
	}
}

func TestCloudCatalogOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudCatalogOperations())
}

// cloudProviderFixture mirrors the PROVIDER_FIXTURE of the TypeScript suite.
const cloudProviderFixture = `{
	"name": "openai",
	"type": "openai",
	"api_url": "https://api.openai.example",
	"api_version": "2024-01-01",
	"api_key_env": "OPENAI_API_KEY",
	"api_key_configured": true
}`

// TestCloudProvidersList ports "Providers.list()".
func TestCloudProvidersList(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK,
			`[`+cloudProviderFixture+`,{"name":"azure","type":"azure"}]`), nil
	})

	providers, err := NewProvidersClient(client).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if got := transport.Only(t).Path(); got != "/apis/cloud.sn.io/v1/agents/providers" {
		t.Errorf("path = %q, want /apis/cloud.sn.io/v1/agents/providers", got)
	}
	if len(providers) != 2 {
		t.Fatalf("providers = %#v, want 2", providers)
	}
	if providers[0].Name != "openai" || !providers[0].APIKeyConfigured {
		t.Errorf("providers[0] = %#v, want the openai fixture with api_key_configured true", providers[0])
	}
}

// TestCloudProvidersRetrieve ports "Providers.retrieve()".
func TestCloudProvidersRetrieve(t *testing.T) {
	t.Parallel()

	t.Run("sends GET to the provider path", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, cloudProviderFixture), nil
		})

		provider, err := NewProvidersClient(client).Get(context.Background(), "openai")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got := transport.Only(t).Path(); got != "/apis/cloud.sn.io/v1/agents/providers/openai" {
			t.Errorf("path = %q, want the provider path", got)
		}
		if provider.Name != "openai" {
			t.Errorf("provider.Name = %q, want openai", provider.Name)
		}
	})

	t.Run("URL-encodes a provider name with special characters", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		if _, err := NewProvidersClient(client).Get(context.Background(), "my/provider"); err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		path := transport.Only(t).Path()
		if !strings.Contains(path, "my%2Fprovider") {
			t.Errorf("path = %q, want it to contain my%%2Fprovider", path)
		}
		if strings.Contains(path, "/my/provider") {
			t.Errorf("path = %q, want the slash encoded, not a second path segment", path)
		}
	})

	t.Run("returns a local provider with the expected shape", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
				"name": "local",
				"type": "local",
				"api_url": "http://localhost:8080",
				"api_key_configured": false
			}`), nil
		})

		provider, err := NewProvidersClient(client).Get(context.Background(), "local")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if provider.Name != "local" || provider.APIKeyConfigured {
			t.Errorf("provider = %#v, want the local provider with api_key_configured false", provider)
		}
	})
}

// TestCloudHealthProbesDecodeABareBoolean asserts what the health operations
// actually return: the endpoints answer with a bare JSON boolean, not an object.
func TestCloudHealthProbesDecodeABareBoolean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invoke func(ctx context.Context, client *HealthClient) (bool, error)
	}{
		{"check", func(ctx context.Context, client *HealthClient) (bool, error) { return client.Health(ctx) }},
		{"ready", func(ctx context.Context, client *HealthClient) (bool, error) { return client.Ready(ctx) }},
		{"live", func(ctx context.Context, client *HealthClient) (bool, error) { return client.Live(ctx) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, "true"), nil
			})

			healthy, err := tc.invoke(context.Background(), NewHealthClient(client))
			if err != nil {
				t.Fatalf("invoke() error = %v", err)
			}
			if !healthy {
				t.Error("healthy = false, want true")
			}
		})
	}
}

// TestCloudAPIResourcesDecodesTheGroupResourceList asserts group discovery
// returns the advertised resource list rather than just a status.
func TestCloudAPIResourcesDecodesTheGroupResourceList(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"kind": "APIResourceList",
			"group_version": "cloud.sn.io/v1",
			"resources": [{"name": "connections", "namespaced": true, "kind": "Connection"}]
		}`), nil
	})

	resources, err := client.GetCloudAPIResources(context.Background())
	if err != nil {
		t.Fatalf("GetCloudAPIResources() error = %v", err)
	}

	if got := transport.Only(t).Path(); got != "/apis/cloud.sn.io/v1/" {
		t.Errorf("path = %q, want /apis/cloud.sn.io/v1/", got)
	}
	if resources.GroupVersion != "cloud.sn.io/v1" {
		t.Errorf("group_version = %q, want cloud.sn.io/v1", resources.GroupVersion)
	}
	if len(resources.Resources) != 1 || resources.Resources[0].Name != "connections" {
		t.Errorf("resources = %#v, want the advertised connections resource", resources.Resources)
	}
}

// TestCloudDiscoveryHealthCatalogGating ports "gates %s before its API request".
func TestCloudDiscoveryHealthCatalogGating(t *testing.T) {
	t.Parallel()
	t.Skip(cloudGatingUnimplemented)
}

// TestCloudProvidersGating ports "Providers cloud extension gating".
func TestCloudProvidersGating(t *testing.T) {
	t.Parallel()
	t.Skip(cloudGatingUnimplemented)
}
