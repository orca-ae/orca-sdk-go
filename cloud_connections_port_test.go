// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"testing"
)

// Ported from orca-sdk-typescript tests/api-resources/cloud/connections.test.ts.
//
// Connections are a StreamNative Cloud extension: every operation resolves under
// the literal /apis/cloud.sn.io/v1 prefix, and a name containing "/" must be
// percent-encoded into a single path segment.

// cloudConnectionOperations is the connections half of the cloud contract table.
func cloudConnectionOperations() []cloudOperation {
	const base = "/apis/cloud.sn.io/v1/connections"
	config := ConnectionConfig{
		Name: "events",
		Spec: ConnectionSpec{Type: ConnectionTypeKafka},
	}

	return []cloudOperation{
		{
			operationID: "listConnections",
			name:        "list",
			method:      "GET",
			path:        base,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewConnectionsClient(client).List(ctx)
				return err
			},
		},
		{
			operationID: "createConnection",
			name:        "create",
			method:      "POST",
			path:        base,
			invoke: func(ctx context.Context, client *Client) error {
				return NewConnectionsClient(client).Create(ctx, config)
			},
		},
		{
			operationID: "getConnection",
			name:        "retrieve",
			method:      "GET",
			// "events/primary" must not split into two path segments.
			path: base + "/events%2Fprimary",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewConnectionsClient(client).Get(ctx, "events/primary")
				return err
			},
		},
		{
			operationID: "updateConnection",
			name:        "update",
			method:      "PUT",
			path:        base + "/events",
			invoke: func(ctx context.Context, client *Client) error {
				return NewConnectionsClient(client).Update(ctx, "events", config)
			},
		},
		{
			operationID: "deleteConnection",
			name:        "delete",
			method:      "DELETE",
			path:        base + "/events",
			invoke: func(ctx context.Context, client *Client) error {
				return NewConnectionsClient(client).Delete(ctx, "events")
			},
		},
		{
			operationID: "testConnection",
			name:        "test",
			method:      "GET",
			path:        base + "/events:test",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewConnectionsClient(client).Test(ctx, "events")
				return err
			},
		},
		{
			operationID: "validateConnection",
			name:        "validate",
			method:      "POST",
			path:        base + "/validate",
			invoke: func(ctx context.Context, client *Client) error {
				return NewConnectionsClient(client).Validate(ctx, config)
			},
		},
	}
}

func TestCloudConnectionsOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudConnectionOperations())
}

// TestCloudConnectionsCreateBody ports "sends the connection body without
// reshaping wire fields", and records where this SDK does not.
//
// Every field except the connection type goes out exactly as supplied. The type
// does not: ConnectionConfig.MarshalJSON upper-cases it, so a caller's "kafka"
// reaches the server as "KAFKA" while the vendored spec declares the enum in
// lower case (pulsar | kafka | other) and the TypeScript SDK forwards it
// verbatim. The assertion below pins that difference rather than hiding it -
// it is a divergence from the contract, not an artifact of this test.
func TestCloudConnectionsCreateBody(t *testing.T) {
	t.Parallel()

	client, transport := cloudTestClient(t)
	err := NewConnectionsClient(client).Create(context.Background(), ConnectionConfig{
		Name:       "events",
		ClusterRef: "cluster-a",
		Spec: ConnectionSpec{
			Type:  ConnectionTypeKafka,
			Kafka: &KafkaConnectionConfig{BootstrapServers: "broker:9092"},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var body map[string]any
	transport.Only(t).JSONBody(t, &body)

	if got := body["name"]; got != "events" {
		t.Errorf("name = %v, want %q", got, "events")
	}
	if got := body["clusterRef"]; got != "cluster-a" {
		t.Errorf("clusterRef = %v, want %q", got, "cluster-a")
	}

	spec, ok := body["spec"].(map[string]any)
	if !ok {
		t.Fatalf("spec = %#v, want an object", body["spec"])
	}
	kafka, ok := spec["kafka"].(map[string]any)
	if !ok {
		t.Fatalf("spec.kafka = %#v, want an object", spec["kafka"])
	}
	if got := kafka["bootstrapServers"]; got != "broker:9092" {
		t.Errorf("spec.kafka.bootstrapServers = %v, want %q", got, "broker:9092")
	}

	// Divergence, asserted so it cannot change unnoticed: the spec enum and the
	// TypeScript SDK both use "kafka" here.
	if got := spec["type"]; got != "KAFKA" {
		t.Errorf("spec.type = %v, want %q (this SDK upper-cases the enum on the wire)", got, "KAFKA")
	}
}

// TestCloudConnectionsCreateSendsJSONContentType asserts the connection
// operations that carry a body send it as JSON, not multipart - the split the
// TypeScript suite makes between its connection and its function tests.
func TestCloudConnectionsCreateSendsJSONContentType(t *testing.T) {
	t.Parallel()

	for _, operation := range []struct {
		name   string
		invoke func(ctx context.Context, client *ConnectionsClient) error
	}{
		{"create", func(ctx context.Context, client *ConnectionsClient) error {
			return client.Create(ctx, ConnectionConfig{Name: "events"})
		}},
		{"update", func(ctx context.Context, client *ConnectionsClient) error {
			return client.Update(ctx, "events", ConnectionConfig{Name: "events"})
		}},
		{"validate", func(ctx context.Context, client *ConnectionsClient) error {
			return client.Validate(ctx, ConnectionConfig{Name: "events"})
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			client, transport := cloudTestClient(t)
			if err := operation.invoke(context.Background(), NewConnectionsClient(client)); err != nil {
				t.Fatalf("invoke() error = %v", err)
			}

			call := transport.Only(t)
			if got := call.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if !json.Valid(call.Body) {
				t.Errorf("body = %q, want valid JSON", string(call.Body))
			}
		})
	}
}

// TestCloudConnectionsGating ports "gates connections before their API request".
func TestCloudConnectionsGating(t *testing.T) {
	t.Parallel()
	assertServiceGated(t, "Cloud.Connections", func(c *Client) any { return c.Cloud.Connections })
}
