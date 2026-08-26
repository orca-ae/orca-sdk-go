package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	util "github.com/apache/pulsar-client-go/pulsaradmin/pkg/utils"
)

func TestClientPostMultipartRequiresConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.PostMultipart(context.Background(), "/functions/test", MultipartRequest{})
	if err == nil || !strings.Contains(err.Error(), "multipart config field is required") {
		t.Fatalf("PostMultipart() error = %v, want config field validation", err)
	}
}

func TestFunctionsClientCreateMultipartIncludesConnection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/functions/test-fn")
		}

		fields, files := decodeMultipartRequest(t, r)
		if got := fields["url"]; got != "file:///tmp/function.nar" {
			t.Fatalf("url field = %q, want %q", got, "file:///tmp/function.nar")
		}
		if !bytes.Contains([]byte(fields["functionConfig"]), []byte(`"connection":"conn-1"`)) {
			t.Fatalf("functionConfig missing connection: %s", fields["functionConfig"])
		}
		if !bytes.Contains([]byte(fields["functionConfig"]), []byte(`"snServiceAccount":"svc-1"`)) {
			t.Fatalf("functionConfig missing snServiceAccount: %s", fields["functionConfig"])
		}
		if string(files["data"]) != "function-bytes" {
			t.Fatalf("file part mismatch: %q", string(files["data"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewFunctionsClient(baseClient)
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "function.nar")
	if err := os.WriteFile(filePath, []byte("function-bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = client.Create(context.Background(), RegistryFunctionConfig{
		FunctionConfig:   util.FunctionConfig{Name: "test-fn"},
		Connection:       "conn-1",
		SNServiceAccount: "svc-1",
	}, filePath, "file:///tmp/function.nar")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestFunctionsClientUpdateMultipartIncludesUpdateOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/functions/test-fn")
		}

		fields, files := decodeMultipartRequest(t, r)
		if !bytes.Contains([]byte(fields["updateOptions"]), []byte(`"updateAuthData":true`)) {
			t.Fatalf("updateOptions mismatch: %s", fields["updateOptions"])
		}
		if string(files["data"]) != "function-bytes" {
			t.Fatalf("file part mismatch: %q", string(files["data"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewFunctionsClient(baseClient)
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "function.nar")
	if err := os.WriteFile(filePath, []byte("function-bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = client.Update(context.Background(), "test-fn", RegistryFunctionConfig{
		FunctionConfig: util.FunctionConfig{Name: "test-fn"},
	}, filePath, "", &UpdateOptionsImpl{UpdateAuthData: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestFunctionsClientUpdateMultipartOmitsUpdateOptionsWhenNil(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/functions/test-fn")
		}

		fields, _ := decodeMultipartRequest(t, r)
		if _, ok := fields["updateOptions"]; ok {
			t.Fatalf("unexpected updateOptions field: %s", fields["updateOptions"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewFunctionsClient(baseClient)
	err = client.Update(context.Background(), "test-fn", RegistryFunctionConfig{
		FunctionConfig: util.FunctionConfig{Name: "test-fn"},
	}, "", "", nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestFunctionsClientStatusPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn/status" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/functions/test-fn/status")
		}
		_, _ = w.Write([]byte(`{"numInstances":1,"numRunning":1,"instances":[]}`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewFunctionsClient(baseClient)
	status, err := client.Status(context.Background(), "test-fn")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.NumInstances != 1 {
		t.Fatalf("NumInstances = %d, want 1", status.NumInstances)
	}
}

func TestSourcesClientUpdateMultipartIncludesConnection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sources/test-src" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sources/test-src")
		}

		fields, files := decodeMultipartRequest(t, r)
		if !bytes.Contains([]byte(fields["sourceConfig"]), []byte(`"connection":"conn-2"`)) {
			t.Fatalf("sourceConfig missing connection: %s", fields["sourceConfig"])
		}
		if !bytes.Contains([]byte(fields["sourceConfig"]), []byte(`"snServiceAccount":"svc-2"`)) {
			t.Fatalf("sourceConfig missing snServiceAccount: %s", fields["sourceConfig"])
		}
		if !bytes.Contains([]byte(fields["sourceConfig"]), []byte(`"logTopic":"source-logs"`)) {
			t.Fatalf("sourceConfig missing logTopic: %s", fields["sourceConfig"])
		}
		if !bytes.Contains([]byte(fields["updateOptions"]), []byte(`"updateAuthData":true`)) {
			t.Fatalf("updateOptions mismatch: %s", fields["updateOptions"])
		}
		if string(files["data"]) != "source-bytes" {
			t.Fatalf("file part mismatch: %q", string(files["data"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSourcesClient(baseClient)
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "source.nar")
	if err := os.WriteFile(filePath, []byte("source-bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = client.Update(context.Background(), "test-src", RegistrySourceConfig{
		SourceConfig:     util.SourceConfig{Name: "test-src"},
		Connection:       "conn-2",
		LogTopic:         "source-logs",
		SNServiceAccount: "svc-2",
	}, filePath, "", &UpdateOptionsImpl{UpdateAuthData: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestSourcesClientUpdateMultipartOmitsUpdateOptionsWhenNil(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sources/test-src" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sources/test-src")
		}

		fields, _ := decodeMultipartRequest(t, r)
		if _, ok := fields["updateOptions"]; ok {
			t.Fatalf("unexpected updateOptions field: %s", fields["updateOptions"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSourcesClient(baseClient)
	err = client.Update(context.Background(), "test-src", RegistrySourceConfig{
		SourceConfig: util.SourceConfig{Name: "test-src"},
	}, "", "", nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestSourcesClientStatusPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sources/test-src/status" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sources/test-src/status")
		}
		_, _ = w.Write([]byte(`{"numInstances":2,"numRunning":1,"instances":[]}`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSourcesClient(baseClient)
	status, err := client.Status(context.Background(), "test-src")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.NumInstances != 2 {
		t.Fatalf("NumInstances = %d, want 2", status.NumInstances)
	}
}

func TestSinksClientCreateMultipartIncludesConnection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sinks/test-sink" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sinks/test-sink")
		}

		fields, _ := decodeMultipartRequest(t, r)
		if !bytes.Contains([]byte(fields["sinkConfig"]), []byte(`"connection":"conn-3"`)) {
			t.Fatalf("sinkConfig missing connection: %s", fields["sinkConfig"])
		}
		if !bytes.Contains([]byte(fields["sinkConfig"]), []byte(`"snServiceAccount":"svc-3"`)) {
			t.Fatalf("sinkConfig missing snServiceAccount: %s", fields["sinkConfig"])
		}
		if !bytes.Contains([]byte(fields["sinkConfig"]), []byte(`"logTopic":"sink-logs"`)) {
			t.Fatalf("sinkConfig missing logTopic: %s", fields["sinkConfig"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSinksClient(baseClient)
	err = client.Create(context.Background(), RegistrySinkConfig{
		SinkConfig:       util.SinkConfig{Name: "test-sink"},
		Connection:       "conn-3",
		LogTopic:         "sink-logs",
		SNServiceAccount: "svc-3",
	}, "", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestSinksClientUpdateMultipartIncludesUpdateOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sinks/test-sink" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sinks/test-sink")
		}

		fields, files := decodeMultipartRequest(t, r)
		if !bytes.Contains([]byte(fields["updateOptions"]), []byte(`"updateAuthData":true`)) {
			t.Fatalf("updateOptions mismatch: %s", fields["updateOptions"])
		}
		if string(files["data"]) != "sink-bytes" {
			t.Fatalf("file part mismatch: %q", string(files["data"]))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSinksClient(baseClient)
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sink.nar")
	if err := os.WriteFile(filePath, []byte("sink-bytes"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = client.Update(context.Background(), "test-sink", RegistrySinkConfig{
		SinkConfig: util.SinkConfig{Name: "test-sink"},
	}, filePath, "", &UpdateOptionsImpl{UpdateAuthData: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestFunctionsClientExtendedOperations(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/functions/fn%2Fname/instance%2F0/status":
			_, _ = w.Write([]byte(`{"running":true,"workerId":"worker-1"}`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/functions/fn%2Fname/stats":
			_, _ = w.Write([]byte(`{"receivedTotal":10,"1min":{"receivedTotal":3},"instances":[]}`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/functions/fn%2Fname/instance%2F0/stats":
			_, _ = w.Write([]byte(`{"receivedTotal":4,"1min":{"receivedTotal":2},"userMetrics":{"lag":1.5}}`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/functions/fn%2Fname/state/key%2F1":
			_, _ = w.Write([]byte(`{"key":"key/1","stringValue":"value","version":2}`))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/functions/fn%2Fname/state/key%2F1":
			fields, _ := decodeMultipartRequest(t, r)
			if !strings.Contains(fields["state"], `"key":"key/1"`) || !strings.Contains(fields["state"], `"numberValue":7`) {
				t.Fatalf("state field = %q", fields["state"])
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.EscapedPath(), ":start"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.EscapedPath(), ":stop"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.EscapedPath(), ":restart"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := NewFunctionsClient(baseClient)
	ctx := context.Background()

	status, err := client.InstanceStatus(ctx, "fn/name", "instance/0")
	if err != nil || !status.Running {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	stats, err := client.Stats(ctx, "fn/name")
	if err != nil || stats.OneMin.ReceivedTotal != 3 {
		t.Fatalf("stats = %#v, err = %v", stats, err)
	}
	instanceStats, err := client.InstanceStats(ctx, "fn/name", "instance/0")
	if err != nil || instanceStats.OneMin.ReceivedTotal != 2 || instanceStats.UserMetrics["lag"] != 1.5 {
		t.Fatalf("instanceStats = %#v, err = %v", instanceStats, err)
	}
	state, err := client.GetState(ctx, "fn/name", "key/1")
	if err != nil || state.StringValue == nil || *state.StringValue != "value" || state.Version != 2 {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	numberValue := int64(7)
	if err := client.PutState(ctx, "fn/name", "key/1", FunctionState{Key: "key/1", NumberValue: &numberValue}); err != nil {
		t.Fatalf("PutState() error = %v", err)
	}
	for action, run := range map[string]func() error{
		"start":   func() error { return client.StartInstance(ctx, "fn/name", "instance/0") },
		"stop":    func() error { return client.StopInstance(ctx, "fn/name", "instance/0") },
		"restart": func() error { return client.RestartInstance(ctx, "fn/name", "instance/0") },
	} {
		if err := run(); err != nil {
			t.Fatalf("%s instance error = %v", action, err)
		}
	}
}

func TestFunctionsClientTriggerMultipart(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn:trigger" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		fields, files := decodeMultipartRequest(t, r)
		if fields["topic"] != "persistent://public/default/input" || string(files["dataStream"]) != "payload" {
			t.Fatalf("fields = %#v, files = %#v", fields, files)
		}
		_, _ = w.Write([]byte(`"trigger-result"`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	filePath := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	result, err := NewFunctionsClient(baseClient).Trigger(
		context.Background(), "test-fn", "", filePath, "persistent://public/default/input",
	)
	if err != nil || result != "trigger-result" {
		t.Fatalf("result = %q, err = %v", result, err)
	}
}

func TestFunctionsClientTriggerAcceptsEmptySuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apis/cloud.sn.io/v1/functions/test-fn:trigger" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	result, err := NewFunctionsClient(baseClient).Trigger(
		context.Background(), "test-fn", "payload", "", "persistent://public/default/input",
	)
	if err != nil || result != "" {
		t.Fatalf("result = %q, err = %v", result, err)
	}
}

func TestSourceAndSinkInstanceOperations(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status") {
			_, _ = w.Write([]byte(`{"running":true,"workerId":"worker-1"}`))
			return
		}
		if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, ":start") || strings.HasSuffix(r.URL.Path, ":stop") || strings.HasSuffix(r.URL.Path, ":restart")) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()
	source := NewSourcesClient(baseClient)
	if status, err := source.InstanceStatus(ctx, "source", "0"); err != nil || !status.Running {
		t.Fatalf("source status = %#v, err = %v", status, err)
	}
	for _, run := range []func() error{
		func() error { return source.StartInstance(ctx, "source", "0") },
		func() error { return source.StopInstance(ctx, "source", "0") },
		func() error { return source.RestartInstance(ctx, "source", "0") },
	} {
		if err := run(); err != nil {
			t.Fatalf("source instance action error = %v", err)
		}
	}

	sink := NewSinksClient(baseClient)
	if status, err := sink.InstanceStatus(ctx, "sink", "0"); err != nil || !status.Running {
		t.Fatalf("sink status = %#v, err = %v", status, err)
	}
	for _, run := range []func() error{
		func() error { return sink.StartInstance(ctx, "sink", "0") },
		func() error { return sink.StopInstance(ctx, "sink", "0") },
		func() error { return sink.RestartInstance(ctx, "sink", "0") },
	} {
		if err := run(); err != nil {
			t.Fatalf("sink instance action error = %v", err)
		}
	}
}

func TestRegistrySourceConfigPreservesSourceType(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(RegistrySourceConfig{SourceType: "kafka"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(payload), `"sourceType":"kafka"`) {
		t.Fatalf("payload = %s", payload)
	}
	var decoded RegistrySourceConfig
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.SourceType != "kafka" {
		t.Fatalf("decoded = %#v, err = %v", decoded, err)
	}
}

func TestSinksClientUpdateMultipartOmitsUpdateOptionsWhenNil(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sinks/test-sink" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sinks/test-sink")
		}

		fields, _ := decodeMultipartRequest(t, r)
		if _, ok := fields["updateOptions"]; ok {
			t.Fatalf("unexpected updateOptions field: %s", fields["updateOptions"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSinksClient(baseClient)
	err = client.Update(context.Background(), "test-sink", RegistrySinkConfig{
		SinkConfig: util.SinkConfig{Name: "test-sink"},
	}, "", "", nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestSinksClientStatusPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connectors/sinks/test-sink/status" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connectors/sinks/test-sink/status")
		}
		_, _ = w.Write([]byte(`{"numInstances":3,"numRunning":2,"instances":[]}`))
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewSinksClient(baseClient)
	status, err := client.Status(context.Background(), "test-sink")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.NumRunning != 2 {
		t.Fatalf("NumRunning = %d, want 2", status.NumRunning)
	}
}
