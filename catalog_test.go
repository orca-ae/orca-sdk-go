// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogClientListSourcesUsesCatalogSourcesPath(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/sources", `[{"name":"kafka","description":"Kafka source"}]`)

	items, err := NewCatalogClient(client).ListSources(context.Background())
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}
	if len(items) != 1 || items[0].Name != "kafka" {
		t.Fatalf("ListSources() = %#v, want kafka source", items)
	}
}

func TestCatalogClientGetSourceConfigDefinitionUsesEscapedPath(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/sources/kafka%2Fv1", `[{"fieldName":"bootstrapServers","typeName":"java.lang.String"}]`)

	items, err := NewCatalogClient(client).GetSourceConfigDefinition(context.Background(), "kafka/v1")
	if err != nil {
		t.Fatalf("GetSourceConfigDefinition() error = %v", err)
	}
	if len(items) != 1 || items[0].FieldName != "bootstrapServers" {
		t.Fatalf("GetSourceConfigDefinition() = %#v, want bootstrapServers field", items)
	}
}

func TestCatalogClientListSinksUsesCatalogSinksPath(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/sinks", `[{"name":"jdbc","description":"JDBC sink"}]`)

	items, err := NewCatalogClient(client).ListSinks(context.Background())
	if err != nil {
		t.Fatalf("ListSinks() error = %v", err)
	}
	if len(items) != 1 || items[0].Name != "jdbc" {
		t.Fatalf("ListSinks() = %#v, want jdbc sink", items)
	}
}

func TestCatalogClientGetSinkConfigDefinitionUsesEscapedPath(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/sinks/jdbc%2Fv1", `[{"fieldName":"jdbcUrl","typeName":"java.lang.String"}]`)

	items, err := NewCatalogClient(client).GetSinkConfigDefinition(context.Background(), "jdbc/v1")
	if err != nil {
		t.Fatalf("GetSinkConfigDefinition() error = %v", err)
	}
	if len(items) != 1 || items[0].FieldName != "jdbcUrl" {
		t.Fatalf("GetSinkConfigDefinition() = %#v, want jdbcUrl field", items)
	}
}

func TestCatalogClientListKafkaConnectors(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/kafka", `[{"name":"jdbc","description":"JDBC connector"}]`)
	items, err := NewCatalogClient(client).ListKafkaConnectors(context.Background())
	if err != nil {
		t.Fatalf("ListKafkaConnectors() error = %v", err)
	}
	if len(items) != 1 || items[0].Name != "jdbc" {
		t.Fatalf("items = %#v", items)
	}
}

func TestCatalogClientGetKafkaConfigDefinitionUsesEscapedPath(t *testing.T) {
	t.Parallel()

	client := newCatalogTestClient(t, http.MethodGet, "/apis/cloud.sn.io/v1/catalog/kafka/jdbc%2Fv1", `[{"fieldName":"connection.url","typeName":"java.lang.String"}]`)
	items, err := NewCatalogClient(client).GetKafkaConfigDefinition(context.Background(), "jdbc/v1")
	if err != nil {
		t.Fatalf("GetKafkaConfigDefinition() error = %v", err)
	}
	if len(items) != 1 || items[0].FieldName != "connection.url" {
		t.Fatalf("items = %#v", items)
	}
}

func newCatalogTestClient(t *testing.T, wantMethod, wantPath, response string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != wantMethod {
			t.Fatalf("method = %q, want %q", r.Method, wantMethod)
		}
		if r.URL.EscapedPath() != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.EscapedPath(), wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}
