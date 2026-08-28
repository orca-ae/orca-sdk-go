// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"testing"

	util "github.com/apache/pulsar-client-go/pulsaradmin/pkg/utils"
)

// Ported from orca-sdk-typescript
// tests/api-resources/cloud/sink-source-connectors.test.ts and
// tests/api-resources/cloud/kafka-connect.test.ts.
//
// Sinks and sources are the same thirteen operations over two different
// collections, so the table is generated per kind exactly as the TypeScript
// source does. Kafka Connect is a separate, larger surface and is spelled out.

const (
	cloudConnectorName        = "arch/ive"
	cloudConnectorEncoded     = "arch%2Five"
	cloudConnectorInstanceID  = "instance/0"
	cloudKafkaConnectorName   = "events/primary"
	cloudKafkaConnectorsBase  = "/apis/cloud.sn.io/v1/connectors/kafka/connectors"
	cloudKafkaNamedConnector  = cloudKafkaConnectorsBase + "/events%2Fprimary"
	cloudKafkaPluginsBase     = "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins"
	cloudKafkaConnectTaskID   = 3
	cloudSinksCollectionPath  = "/apis/cloud.sn.io/v1/connectors/sinks"
	cloudSourceCollectionPath = "/apis/cloud.sn.io/v1/connectors/sources"
)

// cloudConnectorAction is one of the thirteen operations sinks and sources
// share, with the operationId each kind maps it to.
type cloudConnectorAction struct {
	action string
	method string
	// collection marks the operations addressed at the collection rather than
	// at one named connector.
	collection bool
	// suffix hangs off the percent-encoded connector name.
	suffix            string
	sinkOperationID   string
	sourceOperationID string
	invokeSink        func(ctx context.Context, client *SinksClient) error
	invokeSource      func(ctx context.Context, client *SourcesClient) error
}

func cloudConnectorActions() []cloudConnectorAction {
	sinkConfig := RegistrySinkConfig{SinkConfig: util.SinkConfig{Name: cloudConnectorName}}
	sourceConfig := RegistrySourceConfig{SourceConfig: util.SourceConfig{Name: cloudConnectorName}}

	return []cloudConnectorAction{
		{
			action: "create", method: "POST",
			sinkOperationID: "registerSinkWithDefaults", sourceOperationID: "registerSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Create(ctx, sinkConfig, "", "sink://archive@latest")
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Create(ctx, sourceConfig, "", "source://events@latest")
			},
		},
		{
			action: "retrieve", method: "GET",
			sinkOperationID: "getSinkInfoWithDefaults", sourceOperationID: "getSourceInfoWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				_, err := client.Get(ctx, cloudConnectorName)
				return err
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				_, err := client.Get(ctx, cloudConnectorName)
				return err
			},
		},
		{
			action: "update", method: "PUT",
			sinkOperationID: "updateSinkWithDefaults", sourceOperationID: "updateSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Update(ctx, cloudConnectorName, sinkConfig, "", "", nil)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Update(ctx, cloudConnectorName, sourceConfig, "", "", nil)
			},
		},
		{
			action: "delete", method: "DELETE",
			sinkOperationID: "deregisterSinkWithDefaults", sourceOperationID: "deregisterSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Delete(ctx, cloudConnectorName)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Delete(ctx, cloudConnectorName)
			},
		},
		{
			action: "retrieveInstanceStatus", method: "GET", suffix: "/instance%2F0/status",
			sinkOperationID:   "getSinkInstanceStatusWithDefaults",
			sourceOperationID: "getSourceInstanceStatusWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				_, err := client.InstanceStatus(ctx, cloudConnectorName, cloudConnectorInstanceID)
				return err
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				_, err := client.InstanceStatus(ctx, cloudConnectorName, cloudConnectorInstanceID)
				return err
			},
		},
		{
			action: "retrieveStatus", method: "GET", suffix: "/status",
			sinkOperationID: "getSinkStatusWithDefaults", sourceOperationID: "getSourceStatusWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				_, err := client.Status(ctx, cloudConnectorName)
				return err
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				_, err := client.Status(ctx, cloudConnectorName)
				return err
			},
		},
		{
			action: "list", method: "GET", collection: true,
			sinkOperationID: "listSinkWithDefaults", sourceOperationID: "listSourcesWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				_, err := client.List(ctx)
				return err
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				_, err := client.List(ctx)
				return err
			},
		},
		{
			action: "restart", method: "POST", suffix: ":restart",
			sinkOperationID: "restartSinkAllWithDefaults", sourceOperationID: "restartSourceAllWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Restart(ctx, cloudConnectorName)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Restart(ctx, cloudConnectorName)
			},
		},
		{
			action: "restartInstance", method: "POST", suffix: "/instance%2F0:restart",
			sinkOperationID: "restartSinkWithDefaults", sourceOperationID: "restartSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.RestartInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.RestartInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
		},
		{
			action: "start", method: "POST", suffix: ":start",
			sinkOperationID: "startSinkAllWithDefaults", sourceOperationID: "startSourceAllWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Start(ctx, cloudConnectorName)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Start(ctx, cloudConnectorName)
			},
		},
		{
			action: "startInstance", method: "POST", suffix: "/instance%2F0:start",
			sinkOperationID: "startSinkWithDefaults", sourceOperationID: "startSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.StartInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.StartInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
		},
		{
			action: "stop", method: "POST", suffix: ":stop",
			sinkOperationID: "stopSinkAllWithDefaults", sourceOperationID: "stopSourceAllWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.Stop(ctx, cloudConnectorName)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.Stop(ctx, cloudConnectorName)
			},
		},
		{
			action: "stopInstance", method: "POST", suffix: "/instance%2F0:stop",
			sinkOperationID: "stopSinkWithDefaults", sourceOperationID: "stopSourceWithDefaults",
			invokeSink: func(ctx context.Context, client *SinksClient) error {
				return client.StopInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
			invokeSource: func(ctx context.Context, client *SourcesClient) error {
				return client.StopInstance(ctx, cloudConnectorName, cloudConnectorInstanceID)
			},
		},
	}
}

// cloudSinkAndSourceOperations expands the shared action table over both kinds.
func cloudSinkAndSourceOperations() []cloudOperation {
	var operations []cloudOperation
	for _, action := range cloudConnectorActions() {
		sinkPath := cloudSinksCollectionPath
		sourcePath := cloudSourceCollectionPath
		if !action.collection {
			sinkPath += "/" + cloudConnectorEncoded + action.suffix
			sourcePath += "/" + cloudConnectorEncoded + action.suffix
		}

		invokeSink := action.invokeSink
		invokeSource := action.invokeSource
		operations = append(operations,
			cloudOperation{
				operationID: action.sinkOperationID,
				name:        "sinks." + action.action,
				method:      action.method,
				path:        sinkPath,
				invoke: func(ctx context.Context, client *Client) error {
					return invokeSink(ctx, NewSinksClient(client))
				},
			},
			cloudOperation{
				operationID: action.sourceOperationID,
				name:        "sources." + action.action,
				method:      action.method,
				path:        sourcePath,
				invoke: func(ctx context.Context, client *Client) error {
					return invokeSource(ctx, NewSourcesClient(client))
				},
			},
		)
	}
	return operations
}

// cloudKafkaConnectOperations is the Kafka Connect half of the cloud contract table.
func cloudKafkaConnectOperations() []cloudOperation {
	name := cloudKafkaConnectorName

	return []cloudOperation{
		{
			operationID: "healthCheck",
			name:        "kafka.health",
			method:      "GET",
			path:        "/apis/cloud.sn.io/v1/connectors/kafka/health",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetHealth(ctx)
				return err
			},
		},
		{
			operationID: "serverInfo",
			name:        "kafka.serverInfo",
			method:      "GET",
			path:        "/apis/cloud.sn.io/v1/connectors/kafka",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetInfo(ctx)
				return err
			},
		},
		{
			operationID: "getConnectorConfigDef",
			name:        "plugins.retrieveConfig",
			method:      "GET",
			path:        cloudKafkaPluginsBase + "/plugin%2Fname/config",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).DescribePluginConfig(ctx, "plugin/name")
				return err
			},
		},
		{
			operationID: "listConnectorPlugins",
			name:        "plugins.list",
			method:      "GET",
			path:        cloudKafkaPluginsBase,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).ListPlugins(ctx, false)
				return err
			},
		},
		{
			operationID: "listConnectorPluginsCatalog",
			name:        "plugins.listCatalog",
			method:      "GET",
			path:        cloudKafkaPluginsBase + "/catalog",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).ListPluginCatalog(ctx)
				return err
			},
		},
		{
			operationID: "getOffsets",
			name:        "connectors.retrieveOffsets",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/offsets",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetOffsets(ctx, name)
				return err
			},
		},
		{
			operationID: "resetConnectorOffsets",
			name:        "connectors.resetOffsets",
			method:      "DELETE",
			path:        cloudKafkaNamedConnector + "/offsets",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).ResetOffsets(ctx, name)
			},
		},
		{
			operationID: "alterConnectorOffsets",
			name:        "connectors.updateOffsets",
			method:      "PATCH",
			path:        cloudKafkaNamedConnector + "/offsets",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).AlterOffsets(ctx, name, ConnectorOffsets{})
			},
		},
		{
			operationID: "listConnectors",
			name:        "connectors.list",
			method:      "GET",
			path:        cloudKafkaConnectorsBase,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).ListConnectors(ctx)
				return err
			},
		},
		{
			operationID: "createConnector",
			name:        "connectors.create",
			method:      "POST",
			path:        cloudKafkaConnectorsBase,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).CreateConnector(ctx, CreateConnectorRequest{
					Name:   "events",
					Config: map[string]string{"connector.class": "Example"},
				})
				return err
			},
		},
		{
			operationID: "getConnector",
			name:        "connectors.retrieve",
			method:      "GET",
			path:        cloudKafkaNamedConnector,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetConnector(ctx, name)
				return err
			},
		},
		{
			operationID: "destroyConnector",
			name:        "connectors.delete",
			method:      "DELETE",
			path:        cloudKafkaNamedConnector,
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).DeleteConnector(ctx, name)
			},
		},
		{
			operationID: "getConnectorActiveTopics",
			name:        "connectors.retrieveActiveTopics",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/topics",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetActiveTopics(ctx, name)
				return err
			},
		},
		{
			operationID: "getConnectorConfig",
			name:        "connectors.retrieveConfig",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/config",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetConnectorConfig(ctx, name)
				return err
			},
		},
		{
			operationID: "putConnectorConfig",
			name:        "connectors.updateConfig",
			method:      "PUT",
			path:        cloudKafkaNamedConnector + "/config",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).UpdateConnectorConfig(
					ctx, name, map[string]string{"tasks.max": "2"},
				)
				return err
			},
		},
		{
			operationID: "getConnectorStatus",
			name:        "connectors.retrieveStatus",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/status",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetConnectorStatus(ctx, name)
				return err
			},
		},
		{
			operationID: "getTaskConfigs",
			name:        "connectors.listTasks",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/tasks",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetConnectorTasks(ctx, name)
				return err
			},
		},
		{
			operationID: "getTaskStatus",
			name:        "connectors.retrieveTaskStatus",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/tasks/3/status",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetTaskStatus(ctx, name, cloudKafkaConnectTaskID)
				return err
			},
		},
		{
			operationID: "getTasksConfig",
			name:        "connectors.retrieveTasksConfig",
			method:      "GET",
			path:        cloudKafkaNamedConnector + "/tasks-config",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).GetConnectorTasksConfig(ctx, name)
				return err
			},
		},
		{
			operationID: "pauseConnector",
			name:        "connectors.pause",
			method:      "PUT",
			path:        cloudKafkaNamedConnector + ":pause",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).PauseConnector(ctx, name)
			},
		},
		{
			operationID: "resetConnectorActiveTopics",
			name:        "connectors.resetActiveTopics",
			method:      "PUT",
			path:        cloudKafkaNamedConnector + "/topics:reset",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).ResetActiveTopics(ctx, name)
			},
		},
		{
			operationID: "restartConnector",
			name:        "connectors.restart",
			method:      "POST",
			path:        cloudKafkaNamedConnector + ":restart",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewKafkaConnectClient(client).RestartConnectorWithOptions(ctx, name,
					RestartConnectorOptions{IncludeTasks: true, OnlyFailed: true})
				return err
			},
		},
		{
			operationID: "restartTask",
			name:        "connectors.restartTask",
			method:      "POST",
			path:        cloudKafkaNamedConnector + "/tasks/3/restart",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).RestartTask(ctx, name, cloudKafkaConnectTaskID)
			},
		},
		{
			operationID: "resumeConnector",
			name:        "connectors.resume",
			method:      "PUT",
			path:        cloudKafkaNamedConnector + ":resume",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).ResumeConnector(ctx, name)
			},
		},
		{
			operationID: "stopConnector",
			name:        "connectors.stop",
			method:      "PUT",
			path:        cloudKafkaNamedConnector + ":stop",
			invoke: func(ctx context.Context, client *Client) error {
				return NewKafkaConnectClient(client).StopConnector(ctx, name)
			},
		},
	}
}

// cloudConnectorOperations is every connector operation: sinks, sources, and
// Kafka Connect.
func cloudConnectorOperations() []cloudOperation {
	return append(cloudSinkAndSourceOperations(), cloudKafkaConnectOperations()...)
}

func TestCloudSinkAndSourceOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudSinkAndSourceOperations())
}

func TestCloudKafkaConnectOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudKafkaConnectOperations())
}

// TestCloudSinkAndSourceMultipartBodies ports "encodes create and update
// payloads as multipart forms".
func TestCloudSinkAndSourceMultipartBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		part   string
		want   map[string]any
		invoke func(ctx context.Context, client *Client) error
	}{
		{
			name: "sinks.create", part: "sinkConfig",
			want: map[string]any{"name": "archive", "connection": "events"},
			invoke: func(ctx context.Context, client *Client) error {
				return NewSinksClient(client).Create(ctx, RegistrySinkConfig{
					SinkConfig: util.SinkConfig{Name: "archive"},
					Connection: "events",
				}, "", "")
			},
		},
		{
			name: "sinks.update", part: "sinkConfig",
			want: map[string]any{"name": "archive", "parallelism": 2},
			invoke: func(ctx context.Context, client *Client) error {
				return NewSinksClient(client).Update(ctx, "archive", RegistrySinkConfig{
					SinkConfig: util.SinkConfig{Name: "archive", Parallelism: 2},
				}, "", "", nil)
			},
		},
		{
			name: "sources.create", part: "sourceConfig",
			want: map[string]any{"name": "events", "connection": "events"},
			invoke: func(ctx context.Context, client *Client) error {
				return NewSourcesClient(client).Create(ctx, RegistrySourceConfig{
					SourceConfig: util.SourceConfig{Name: "events"},
					Connection:   "events",
				}, "", "")
			},
		},
		{
			name: "sources.update", part: "sourceConfig",
			want: map[string]any{"name": "events", "parallelism": 2},
			invoke: func(ctx context.Context, client *Client) error {
				return NewSourcesClient(client).Update(ctx, "events", RegistrySourceConfig{
					SourceConfig: util.SourceConfig{Name: "events", Parallelism: 2},
				}, "", "", nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := cloudTestClient(t)
			if err := tc.invoke(context.Background(), client); err != nil {
				t.Fatalf("invoke() error = %v", err)
			}

			call := transport.Only(t)
			fields, files := cloudDecodeMultipart(t, call)
			if len(files) != 0 {
				t.Errorf("file parts = %v, want none when no package file is supplied", cloudFileNames(files))
			}
			if len(fields) != 1 {
				t.Errorf("parts = %v, want only %s", cloudFieldNames(fields), tc.part)
			}

			// A subset check, not an exact one: the embedded pulsaradmin
			// config structs leave their booleans untagged, so the part also
			// carries defaults the caller never set. See
			// TestCloudFunctionsMultipartBodies, which pins that behaviour.
			cloudAssertJSONFieldContains(t, fields, tc.part, tc.want)
		})
	}
}

// TestCloudKafkaConnectQueryParameters ports "forwards plugin and restart query
// parameters".
func TestCloudKafkaConnectQueryParameters(t *testing.T) {
	t.Parallel()

	t.Run("plugins.list forwards connectorsOnly", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		if _, err := NewKafkaConnectClient(client).ListPlugins(context.Background(), false); err != nil {
			t.Fatalf("ListPlugins() error = %v", err)
		}
		if got := transport.Only(t).Query().Get("connectorsOnly"); got != "false" {
			t.Errorf("connectorsOnly = %q, want %q", got, "false")
		}
	})

	t.Run("plugins.list omits connectorsOnly when unset", func(t *testing.T) {
		t.Parallel()

		// The variadic argument is what lets this SDK preserve the server
		// default instead of forcing a value, so omitting it must send no query.
		client, transport := cloudTestClient(t)
		if _, err := NewKafkaConnectClient(client).ListPlugins(context.Background()); err != nil {
			t.Fatalf("ListPlugins() error = %v", err)
		}
		if got := transport.Only(t).URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}
	})

	t.Run("connectors.restart forwards includeTasks and onlyFailed", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		_, err := NewKafkaConnectClient(client).RestartConnectorWithOptions(
			context.Background(), "events", RestartConnectorOptions{IncludeTasks: true, OnlyFailed: true},
		)
		if err != nil {
			t.Fatalf("RestartConnectorWithOptions() error = %v", err)
		}

		query := transport.Only(t).Query()
		if got := query.Get("includeTasks"); got != "true" {
			t.Errorf("includeTasks = %q, want %q", got, "true")
		}
		if got := query.Get("onlyFailed"); got != "true" {
			t.Errorf("onlyFailed = %q, want %q", got, "true")
		}
	})
}

// TestCloudKafkaConnectJSONBodies ports "forwards connector JSON request bodies
// without reshaping".
func TestCloudKafkaConnectJSONBodies(t *testing.T) {
	t.Parallel()

	offsets := ConnectorOffsets{Offsets: []map[string]interface{}{
		{"partition": map[string]any{"kafka_topic": "orders"}, "offset": map[string]any{"kafka_offset": 42}},
	}}

	tests := []struct {
		name   string
		want   string
		invoke func(ctx context.Context, client *KafkaConnectClient) error
	}{
		{
			name: "updateOffsets",
			want: `{"offsets":[{"offset":{"kafka_offset":42},"partition":{"kafka_topic":"orders"}}]}`,
			invoke: func(ctx context.Context, client *KafkaConnectClient) error {
				return client.AlterOffsets(ctx, "events", offsets)
			},
		},
		{
			name: "create",
			want: `{"initial_state":"PAUSED","name":"events"}`,
			invoke: func(ctx context.Context, client *KafkaConnectClient) error {
				_, err := client.CreateConnector(ctx, CreateConnectorRequest{
					Name: "events", InitialState: "PAUSED",
				})
				return err
			},
		},
		{
			name: "updateConfig",
			want: `{"tasks.max":"2"}`,
			invoke: func(ctx context.Context, client *KafkaConnectClient) error {
				_, err := client.UpdateConnectorConfig(ctx, "events", map[string]string{"tasks.max": "2"})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, transport := cloudTestClient(t)
			if err := tc.invoke(context.Background(), NewKafkaConnectClient(client)); err != nil {
				t.Fatalf("invoke() error = %v", err)
			}

			// Compare canonically: Go marshals struct fields in declaration
			// order, so a byte comparison would be asserting field order.
			var body any
			transport.Only(t).JSONBody(t, &body)
			if got := cloudJSON(t, body); got != tc.want {
				t.Errorf("body = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestCloudKafkaConnectOmitsAnEmptyOffsetsList records a wire-shape divergence:
// the TypeScript SDK sends {"offsets":[]} verbatim, while ConnectorOffsets tags
// the field omitempty, so an empty list is dropped and the server receives {}.
func TestCloudKafkaConnectOmitsAnEmptyOffsetsList(t *testing.T) {
	t.Parallel()

	client, transport := cloudTestClient(t)
	if err := NewKafkaConnectClient(client).AlterOffsets(
		context.Background(), "events", ConnectorOffsets{Offsets: []map[string]interface{}{}},
	); err != nil {
		t.Fatalf("AlterOffsets() error = %v", err)
	}

	if got := string(transport.Only(t).Body); got != `{}` {
		t.Errorf("body = %s, want {} (omitempty drops an empty offsets list)", got)
	}
}

// TestCloudKafkaConnectPatchConfigIssuesNoRequest records that this SDK's
// PatchConnectorConfig is a deliberate refusal: the Java runtime serves no
// PATCH route for connector config, and the spec declares none either.
func TestCloudKafkaConnectPatchConfigIssuesNoRequest(t *testing.T) {
	t.Parallel()

	client, transport := cloudTestClient(t)
	result, err := NewKafkaConnectClient(client).PatchConnectorConfig(
		context.Background(), "events", map[string]string{"tasks.max": "2"},
	)
	if result != nil || err == nil {
		t.Fatalf("result = %#v, err = %v; want a refusal", result, err)
	}
	if calls := transport.Calls(); len(calls) != 0 {
		t.Errorf("captured %d requests, want none", len(calls))
	}
}

// TestCloudSinkAndSourceGating ports "gates %s before the connector API request".
func TestCloudSinkAndSourceGating(t *testing.T) {
	t.Parallel()

	t.Run("sinks", func(t *testing.T) {
		t.Parallel()
		assertServiceGated(t, "Cloud.Connectors.Sinks", func(c *Client) any { return c.Cloud.Connectors.Sinks })
	})
	t.Run("sources", func(t *testing.T) {
		t.Parallel()
		assertServiceGated(t, "Cloud.Connectors.Sources", func(c *Client) any { return c.Cloud.Connectors.Sources })
	})
}

// TestCloudKafkaConnectGating ports "gates %s before the Kafka API request".
func TestCloudKafkaConnectGating(t *testing.T) {
	t.Parallel()
	assertServiceGated(t, "Cloud.Connectors.Kafka", func(c *Client) any { return c.Cloud.Connectors.Kafka })
}
