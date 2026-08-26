package orca

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionsClientList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connections" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connections")
		}

		response := []ConnectionConfig{
			{
				Name:   "conn-1",
				Spec:   ConnectionSpec{Type: ConnectionTypePulsar},
				Status: &ConnectionStatus{Phase: ConnectionPhaseHealthy},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewConnectionsClient(baseClient)
	connections, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(connections) != 1 {
		t.Fatalf("List() len = %d, want %d", len(connections), 1)
	}
	if connections[0].Name != "conn-1" {
		t.Fatalf("List() name = %q, want %q", connections[0].Name, "conn-1")
	}
	if connections[0].Spec.Type != ConnectionTypePulsar {
		t.Fatalf("List() type = %q, want %q", connections[0].Spec.Type, ConnectionTypePulsar)
	}
}

func TestConnectionsClientCreate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/apis/cloud.sn.io/v1/connections" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/apis/cloud.sn.io/v1/connections")
		}

		payloadBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read payload: %v", err)
		}

		var payload struct {
			Name string `json:"name"`
			Spec struct {
				Type string `json:"type"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		if payload.Name != "conn-1" {
			t.Fatalf("payload.Name = %q, want %q", payload.Name, "conn-1")
		}
		if payload.Spec.Type != "KAFKA" {
			t.Fatalf("payload.Spec.Type = %q, want %q", payload.Spec.Type, "KAFKA")
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client := NewConnectionsClient(baseClient)
	err = client.Create(context.Background(), ConnectionConfig{
		Name: "conn-1",
		Spec: ConnectionSpec{Type: ConnectionTypeKafka},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestConnectionsClientRemainingOperations(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/connections/conn%2F1":
			_, _ = w.Write([]byte(`{"name":"conn/1","clusterRef":"cluster-1","internal":true,"spec":{"type":"KAFKA"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/apis/cloud.sn.io/v1/connections/validate":
			assertConnectionRequestType(t, r, "PULSAR")
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/connections/conn%2F1":
			assertConnectionRequestType(t, r, "OTHER")
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/connections/conn%2F1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/apis/cloud.sn.io/v1/connections/conn%2F1:test":
			_, _ = w.Write([]byte(`{"name":"conn/1","healthy":true,"phase":"Healthy"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	baseClient, err := NewClient(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client := NewConnectionsClient(baseClient)
	ctx := context.Background()
	got, err := client.Get(ctx, "conn/1")
	if err != nil || got.ClusterRef != "cluster-1" || !got.Internal || got.Spec.Type != ConnectionTypeKafka {
		t.Fatalf("got = %#v, err = %v", got, err)
	}
	if err := client.Validate(ctx, ConnectionConfig{Spec: ConnectionSpec{Type: ConnectionTypePulsar}}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := client.Update(ctx, "conn/1", ConnectionConfig{Spec: ConnectionSpec{Type: ConnectionTypeOther}}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	health, err := client.Test(ctx, "conn/1")
	if err != nil || !health.Healthy {
		t.Fatalf("health = %#v, err = %v", health, err)
	}
	if err := client.Delete(ctx, "conn/1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func assertConnectionRequestType(t *testing.T, r *http.Request, want string) {
	t.Helper()
	var payload struct {
		Spec struct {
			Type string `json:"type"`
		} `json:"spec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Spec.Type != want {
		t.Fatalf("payload.Spec.Type = %q, want %q", payload.Spec.Type, want)
	}
}

func TestConnectionConfigConversions(t *testing.T) {
	t.Parallel()

	config := ConnectionConfig{
		ClusterRef: "cluster-1",
		Internal:   true,
		Name:       "conn-1",
		Spec:       ConnectionSpec{Type: ConnectionTypeOther},
		Status:     &ConnectionStatus{Phase: ConnectionPhaseTesting},
	}

	connection := config.ToConnection()
	if connection.APIVersion != ConnectionAPIVersion {
		t.Fatalf("connection.APIVersion = %q, want %q", connection.APIVersion, ConnectionAPIVersion)
	}
	if connection.Kind != connectionKind {
		t.Fatalf("connection.Kind = %q, want %q", connection.Kind, connectionKind)
	}
	if connection.Labels[connectionClusterRefLabel] != "cluster-1" {
		t.Fatalf("connection labels = %#v", connection.Labels)
	}
	if connection.Annotations[connectionWorkspaceManaged] != "true" {
		t.Fatalf("connection annotations = %#v", connection.Annotations)
	}
	converted := ConnectionConfigFromConnection(connection)
	if converted.ClusterRef != "cluster-1" || !converted.Internal {
		t.Fatalf("converted = %#v", converted)
	}

	list := ToConnectionList([]ConnectionConfig{config})
	if list.Kind != connectionListKind {
		t.Fatalf("list.Kind = %q, want %q", list.Kind, connectionListKind)
	}
	if len(list.Items) != 1 {
		t.Fatalf("list.Items len = %d, want %d", len(list.Items), 1)
	}
}

func TestConnectionConfigUnmarshalAcceptsNumericConditionTimestamp(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "clusterRef": "cluster-1",
	  "name": "conn-1",
	  "spec": {"type": "KAFKA"},
	  "status": {
	    "phase": "Healthy",
	    "conditions": [
	      {
	        "type": "ConnectivityReady",
	        "status": "True",
	        "lastTransitionTime": 1741766400000
	      }
	    ],
	    "lastTestedAt": 1741766400000
	  }
	}`)

	var cfg ConnectionConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if cfg.Status == nil {
		t.Fatal("cfg.Status is nil")
	}
	if cfg.ClusterRef != "cluster-1" {
		t.Fatalf("cfg.ClusterRef = %q, want %q", cfg.ClusterRef, "cluster-1")
	}
	if cfg.Spec.Type != ConnectionTypeKafka {
		t.Fatalf("cfg.Spec.Type = %q, want %q", cfg.Spec.Type, ConnectionTypeKafka)
	}
	if got := string(cfg.Status.LastTestedAt); got != "1741766400000" {
		t.Fatalf("cfg.Status.LastTestedAt = %q, want %q", got, "1741766400000")
	}
	if len(cfg.Status.Conditions) != 1 {
		t.Fatalf("cfg.Status.Conditions len = %d, want %d", len(cfg.Status.Conditions), 1)
	}
	if got := string(cfg.Status.Conditions[0].LastTransitionTime); got != "1741766400000" {
		t.Fatalf("cfg.Status.Conditions[0].LastTransitionTime = %q, want %q", got, "1741766400000")
	}
}

func TestConnectionConfigMarshalPreservesUnknownType(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(ConnectionConfig{
		Name: "conn-1",
		Spec: ConnectionSpec{Type: ConnectionType("custom")},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded struct {
		Spec struct {
			Type string `json:"type"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Spec.Type != "custom" {
		t.Fatalf("decoded.Spec.Type = %q, want %q", decoded.Spec.Type, "custom")
	}
}

func TestConnectionConfigUnmarshalPreservesUnknownType(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "name": "conn-1",
	  "spec": {"type": "CUSTOM"}
	}`)

	var cfg ConnectionConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if cfg.Spec.Type != ConnectionType("CUSTOM") {
		t.Fatalf("cfg.Spec.Type = %q, want %q", cfg.Spec.Type, ConnectionType("CUSTOM"))
	}
}
