package orca

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKafkaConnectClientCreateConnectorUsesResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apis/cloud.sn.io/v1/connectors/kafka/connectors" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var payload CreateConnectorRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Config["sn.connection"] != "conn-1" {
			t.Fatalf("payload.Config = %#v", payload.Config)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ConnectorInfo{
			Name:  payload.Name,
			Type:  "source",
			Tasks: []ConnectorTaskID{{Connector: payload.Name, Task: 0}},
		})
	}))
	defer server.Close()

	client := newKafkaConnectTestClient(t, server)
	got, err := client.CreateConnector(context.Background(), CreateConnectorRequest{
		Name: "conn-test",
		Config: map[string]string{
			"connector.class": "org.example.Connector",
			"sn.connection":   "conn-1",
		},
	})
	if err != nil {
		t.Fatalf("CreateConnector() error = %v", err)
	}
	if got.Name != "conn-test" || got.Type != "source" || len(got.Tasks) != 1 || got.Tasks[0].Task != 0 {
		t.Fatalf("got = %#v", got)
	}
}

func TestKafkaConnectClientUpdateConnectorConfigUsesResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.EscapedPath() != "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my%2Fconnector/config" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(ConnectorInfo{Name: "my/connector", Type: "sink"})
	}))
	defer server.Close()

	got, err := newKafkaConnectTestClient(t, server).UpdateConnectorConfig(
		context.Background(), "my/connector", map[string]string{"topics": "orders"},
	)
	if err != nil {
		t.Fatalf("UpdateConnectorConfig() error = %v", err)
	}
	if got.Name != "my/connector" || got.Type != "sink" {
		t.Fatalf("got = %#v", got)
	}
}

func TestKafkaConnectClientGetConnectorDoesNotFetchStatus(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ConnectorInfo{Name: "my-connector"})
	}))
	defer server.Close()

	if _, err := newKafkaConnectTestClient(t, server).GetConnector(context.Background(), "my-connector"); err != nil {
		t.Fatalf("GetConnector() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestKafkaConnectClientPatchConnectorConfigIsExplicitlyUnsupported(t *testing.T) {
	t.Parallel()

	client := NewKafkaConnectClient(nil)
	_, err := client.PatchConnectorConfig(context.Background(), "connector", map[string]string{"topics": "a"})
	if err == nil {
		t.Fatal("PatchConnectorConfig() expected unsupported error")
	}
}

func TestKafkaConnectClientGetConnectorStatusPreservesNestedState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/status" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/status")
		}
		_, _ = w.Write([]byte(`{"name":"my-connector","type":"sink","connector":{"state":"FAILED","worker_id":"worker-a","trace":"connector trace"},"tasks":[{"id":1,"state":"FAILED","worker_id":"worker-b","trace":"task trace"}]}`))
	}))
	defer server.Close()

	status, err := newKafkaConnectTestClient(t, server).GetConnectorStatus(context.Background(), "my-connector")
	if err != nil {
		t.Fatalf("GetConnectorStatus() error = %v", err)
	}
	if status.Connector.State != ConnectorStateFailed || status.Connector.Trace != "connector trace" {
		t.Fatalf("status.Connector = %#v", status.Connector)
	}
	if len(status.Tasks) != 1 || status.Tasks[0].Trace != "task trace" {
		t.Fatalf("status.Tasks = %#v", status.Tasks)
	}
}

func TestKafkaConnectClientTaskEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/tasks":
			_, _ = w.Write([]byte(`[{"id":{"connector":"my-connector","task":1},"config":{"topics":"orders"}}]`))
		case "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/tasks-config":
			_, _ = w.Write([]byte(`{"my-connector-1":{"topics":"orders"}}`))
		case "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/tasks/1/status":
			_, _ = w.Write([]byte(`{"id":1,"state":"RUNNING","worker_id":"worker-1"}`))
		case "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/tasks/1/restart":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %q", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newKafkaConnectTestClient(t, server)
	tasks, err := client.GetConnectorTasks(context.Background(), "my-connector")
	if err != nil || len(tasks) != 1 || tasks[0].ID.Task != 1 || tasks[0].Config["topics"] != "orders" {
		t.Fatalf("tasks = %#v, err = %v", tasks, err)
	}
	deprecated, err := client.GetConnectorTasksConfig(context.Background(), "my-connector")
	if err != nil || deprecated["my-connector-1"]["topics"] != "orders" {
		t.Fatalf("deprecated = %#v, err = %v", deprecated, err)
	}
	status, err := client.GetTaskStatus(context.Background(), "my-connector", 1)
	if err != nil || status.State != ConnectorStateRunning {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	if err := client.RestartTask(context.Background(), "my-connector", 1); err != nil {
		t.Fatalf("RestartTask() error = %v", err)
	}
}

func TestKafkaConnectClientPluginAndHealthEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/cloud.sn.io/v1/connectors/kafka/health":
			_ = json.NewEncoder(w).Encode(WorkerStatus{Status: "healthy", Message: "ready"})
		case "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins":
			if r.URL.Query().Get("connectorsOnly") != "false" {
				t.Fatalf("query = %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]PluginInfo{{Class: "org.example.Connector"}})
		case "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins/catalog":
			_ = json.NewEncoder(w).Encode([]FunctionMeshConnectorDefinition{{ID: "jdbc", Version: "1.0.0"}})
		case "/apis/cloud.sn.io/v1/connectors/kafka/connector-plugins/org.example.Connector/config":
			_ = json.NewEncoder(w).Encode([]ConfigKeyInfo{{Name: "topics", Required: true}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newKafkaConnectTestClient(t, server)
	health, err := client.GetHealth(context.Background())
	if err != nil || health.Status != "healthy" {
		t.Fatalf("health = %#v, err = %v", health, err)
	}
	plugins, err := client.ListPlugins(context.Background(), false)
	if err != nil || len(plugins) != 1 {
		t.Fatalf("plugins = %#v, err = %v", plugins, err)
	}
	catalog, err := client.ListPluginCatalog(context.Background())
	if err != nil || len(catalog) != 1 || catalog[0].ID != "jdbc" {
		t.Fatalf("catalog = %#v, err = %v", catalog, err)
	}
	config, err := client.DescribePluginConfig(context.Background(), "org.example.Connector")
	if err != nil || len(config) != 1 || !config[0].Required {
		t.Fatalf("config = %#v, err = %v", config, err)
	}
}

func TestKafkaConnectClientValidateConfigIsUnsupported(t *testing.T) {
	t.Parallel()

	result, err := NewKafkaConnectClient(nil).ValidateConfig(context.Background(), "plugin", nil)
	if result != nil || !errors.Is(err, ErrKafkaConnectConfigValidationUnsupported) {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestKafkaConnectClientActiveTopicsAndRestartOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/topics":
			_, _ = w.Write([]byte(`{"my-connector":{"topics":["orders"]}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/topics:reset":
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPost && r.URL.Path == "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector:restart":
			if r.URL.Query().Get("includeTasks") != "true" || r.URL.Query().Get("onlyFailed") != "true" {
				t.Fatalf("query = %q", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"name":"my-connector","connector":{"state":"RUNNING"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := newKafkaConnectTestClient(t, server)
	topics, err := client.GetActiveTopics(context.Background(), "my-connector")
	if err != nil || topics["my-connector"]["topics"][0] != "orders" {
		t.Fatalf("topics = %#v, err = %v", topics, err)
	}
	if err := client.ResetActiveTopics(context.Background(), "my-connector"); err != nil {
		t.Fatalf("ResetActiveTopics() error = %v", err)
	}
	status, err := client.RestartConnectorWithOptions(context.Background(), "my-connector", RestartConnectorOptions{IncludeTasks: true, OnlyFailed: true})
	if err != nil || status == nil || status.Connector.State != ConnectorStateRunning {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
}

func TestKafkaConnectClientOffsetEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/apis/cloud.sn.io/v1/connectors/kafka/connectors/my-connector/offsets"
		if r.URL.Path != path {
			t.Fatalf("path = %q", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(ConnectorOffsets{Offsets: []map[string]interface{}{{"offset": map[string]interface{}{"kafka_offset": 12}}}})
		case http.MethodPatch:
			var payload ConnectorOffsets
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Offsets) != 1 {
				t.Fatalf("payload = %#v, err = %v", payload, err)
			}
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("method = %q", r.Method)
		}
	}))
	defer server.Close()

	client := newKafkaConnectTestClient(t, server)
	if _, err := client.GetOffsets(context.Background(), "my-connector"); err != nil {
		t.Fatalf("GetOffsets() error = %v", err)
	}
	if err := client.AlterOffsets(context.Background(), "my-connector", ConnectorOffsets{Offsets: []map[string]interface{}{{"offset": map[string]interface{}{"kafka_offset": 13}}}}); err != nil {
		t.Fatalf("AlterOffsets() error = %v", err)
	}
	if err := client.ResetOffsets(context.Background(), "my-connector"); err != nil {
		t.Fatalf("ResetOffsets() error = %v", err)
	}
}

func newKafkaConnectTestClient(t *testing.T, server *httptest.Server) *KafkaConnectClient {
	t.Helper()
	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return NewKafkaConnectClient(baseClient)
}
