// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConnectionAPIVersion       = "k8s.streamnative.io/v1alpha1"
	connectionKind             = "Connection"
	connectionListKind         = "ConnectionList"
	connectionClusterRefLabel  = "compute.streamnative.io/cluster-ref"
	connectionWorkspaceManaged = "compute.streamnative.io/workspace-managed"
)

// ConnectionType defines the supported connection backends.
type ConnectionType string

const (
	ConnectionTypePulsar ConnectionType = "pulsar"
	ConnectionTypeKafka  ConnectionType = "kafka"
	ConnectionTypeOther  ConnectionType = "other"
)

// ConnectionPhase describes the latest observed connection health.
type ConnectionPhase string

const (
	ConnectionPhaseUnknown   ConnectionPhase = "Unknown"
	ConnectionPhaseHealthy   ConnectionPhase = "Healthy"
	ConnectionPhaseUnhealthy ConnectionPhase = "Unhealthy"
	ConnectionPhaseTesting   ConnectionPhase = "Testing"
)

// FlexibleTimestamp is a registry-local timestamp representation that accepts
// either a JSON string or a JSON number on decode.
type FlexibleTimestamp string

func (t *FlexibleTimestamp) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*t = ""
		return nil
	}

	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		*t = FlexibleTimestamp(value)
		return nil
	}

	var value json.Number
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return fmt.Errorf("unsupported timestamp encoding %q: %w", string(trimmed), err)
	}
	*t = FlexibleTimestamp(value.String())
	return nil
}

func (t FlexibleTimestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(t))
}

// ConnectionCondition is the registry wire shape for connection conditions.
type ConnectionCondition struct {
	LastTransitionTime FlexibleTimestamp `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string            `json:"message,omitempty" yaml:"message,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Reason             string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string            `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string            `json:"type,omitempty" yaml:"type,omitempty"`
}

// SecretKeyRef references a key inside a Kubernetes Secret.
type SecretKeyRef struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

// PulsarTLSConfig defines TLS settings for a Pulsar connection.
type PulsarTLSConfig struct {
	Enabled                    bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	AllowInsecureConnection    bool          `json:"allowInsecureConnection,omitempty" yaml:"allowInsecureConnection,omitempty"`
	EnableHostnameVerification *bool         `json:"enableHostnameVerification,omitempty" yaml:"enableHostnameVerification,omitempty"`
	TrustCertsSecretRef        *SecretKeyRef `json:"trustCertsSecretRef,omitempty" yaml:"trustCertsSecretRef,omitempty"`
	ClientCertSecretRef        *SecretKeyRef `json:"clientCertSecretRef,omitempty" yaml:"clientCertSecretRef,omitempty"`
	ClientKeySecretRef         *SecretKeyRef `json:"clientKeySecretRef,omitempty" yaml:"clientKeySecretRef,omitempty"`
}

// PulsarAuthConfig defines auth settings for a Pulsar connection.
type PulsarAuthConfig struct {
	Token       *SecretKeyRef          `json:"token,omitempty" yaml:"token,omitempty"`
	OAuth2      map[string]interface{} `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
	GenericAuth map[string]interface{} `json:"genericAuth,omitempty" yaml:"genericAuth,omitempty"`
}

// PulsarConnectionConfig defines Pulsar-specific connection settings.
type PulsarConnectionConfig struct {
	ServiceURL     string            `json:"serviceUrl,omitempty" yaml:"serviceUrl,omitempty"`
	AdminURL       string            `json:"adminUrl,omitempty" yaml:"adminUrl,omitempty"`
	Authentication *PulsarAuthConfig `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	TLS            *PulsarTLSConfig  `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// KafkaConnectionConfig defines Kafka-specific connection settings.
type KafkaConnectionConfig struct {
	BootstrapServers string                 `json:"bootstrapServers,omitempty" yaml:"bootstrapServers,omitempty"`
	TLS              map[string]interface{} `json:"tls,omitempty" yaml:"tls,omitempty"`
	Authentication   map[string]interface{} `json:"authentication,omitempty" yaml:"authentication,omitempty"`
}

// OtherConnectionConfig defines settings for generic endpoints.
type OtherConnectionConfig struct {
	Endpoint   string            `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Properties map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
	SecretRef  *SecretKeyRef     `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}

// ConnectionSpec mirrors the Connection CRD spec shape used by the registry API.
type ConnectionSpec struct {
	Type   ConnectionType          `json:"type,omitempty" yaml:"type,omitempty"`
	Pulsar *PulsarConnectionConfig `json:"pulsar,omitempty" yaml:"pulsar,omitempty"`
	Kafka  *KafkaConnectionConfig  `json:"kafka,omitempty" yaml:"kafka,omitempty"`
	Other  *OtherConnectionConfig  `json:"other,omitempty" yaml:"other,omitempty"`
}

// ConnectionStatus mirrors the Connection CRD status shape used by the registry API.
type ConnectionStatus struct {
	Conditions         []ConnectionCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ObservedGeneration int64                 `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Phase              ConnectionPhase       `json:"phase,omitempty" yaml:"phase,omitempty"`
	LastTestedAt       FlexibleTimestamp     `json:"lastTestedAt,omitempty" yaml:"lastTestedAt,omitempty"`
	Message            string                `json:"message,omitempty" yaml:"message,omitempty"`
}

// Connection is the CR-shaped representation used for file input/output.
type Connection struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              ConnectionSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status            ConnectionStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ConnectionList is the CR-shaped list response used for yaml/json output.
type ConnectionList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Connection `json:"items" yaml:"items"`
}

// ConnectionConfig is the registry transport shape for connections.
type ConnectionConfig struct {
	ClusterRef string            `json:"clusterRef,omitempty" yaml:"clusterRef,omitempty"`
	Internal   bool              `json:"internal,omitempty" yaml:"internal,omitempty"`
	Name       string            `json:"name,omitempty" yaml:"name,omitempty"`
	Spec       ConnectionSpec    `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status     *ConnectionStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type connectionConfigJSON struct {
	ClusterRef string             `json:"clusterRef,omitempty"`
	Internal   bool               `json:"internal,omitempty"`
	Name       string             `json:"name,omitempty"`
	Spec       connectionSpecJSON `json:"spec,omitempty"`
	Status     *ConnectionStatus  `json:"status,omitempty"`
}

type connectionSpecJSON struct {
	Type   ConnectionType          `json:"type,omitempty"`
	Pulsar *PulsarConnectionConfig `json:"pulsar,omitempty"`
	Kafka  *KafkaConnectionConfig  `json:"kafka,omitempty"`
	Other  *OtherConnectionConfig  `json:"other,omitempty"`
}

// ConnectionHealthStatus is the registry health response for connection tests.
type ConnectionHealthStatus struct {
	Healthy      bool   `json:"healthy,omitempty" yaml:"healthy,omitempty"`
	LastTestedAt string `json:"lastTestedAt,omitempty" yaml:"lastTestedAt,omitempty"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Phase        string `json:"phase,omitempty" yaml:"phase,omitempty"`
}

// MarshalJSON keeps the CLI's internal connection type values stable while
// adapting the transport payload to the registry API's enum casing.
func (c ConnectionConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(connectionConfigJSON{
		ClusterRef: c.ClusterRef,
		Internal:   c.Internal,
		Name:       c.Name,
		Spec: connectionSpecJSON{
			Type:   normalizeConnectionTypeForTransport(c.Spec.Type),
			Pulsar: c.Spec.Pulsar,
			Kafka:  c.Spec.Kafka,
			Other:  c.Spec.Other,
		},
		Status: c.Status,
	})
}

// UnmarshalJSON normalizes known transport enum values back to the CLI's
// internal lowercase representation so downstream logic can switch on them.
func (c *ConnectionConfig) UnmarshalJSON(data []byte) error {
	var decoded connectionConfigJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*c = ConnectionConfig{
		ClusterRef: decoded.ClusterRef,
		Internal:   decoded.Internal,
		Name:       decoded.Name,
		Spec: ConnectionSpec{
			Type:   normalizeConnectionTypeForInternal(decoded.Spec.Type),
			Pulsar: decoded.Spec.Pulsar,
			Kafka:  decoded.Spec.Kafka,
			Other:  decoded.Spec.Other,
		},
		Status: decoded.Status,
	}

	return nil
}

// ToConnection converts a registry payload into a CR-shaped Connection object.
func (c ConnectionConfig) ToConnection() Connection {
	connection := Connection{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ConnectionAPIVersion,
			Kind:       connectionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
		},
		Spec: c.Spec,
	}
	if c.ClusterRef != "" {
		connection.Labels = map[string]string{connectionClusterRefLabel: c.ClusterRef}
	}
	if c.Internal {
		connection.Annotations = map[string]string{connectionWorkspaceManaged: "true"}
	}
	if c.Status != nil {
		connection.Status = *c.Status
	}

	return connection
}

// ToConnectionList converts registry payloads into a CR-shaped ConnectionList object.
func ToConnectionList(configs []ConnectionConfig) ConnectionList {
	items := make([]Connection, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, cfg.ToConnection())
	}

	return ConnectionList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ConnectionAPIVersion,
			Kind:       connectionListKind,
		},
		Items: items,
	}
}

// ConnectionConfigFromConnection converts a CR-shaped object to a registry payload.
func ConnectionConfigFromConnection(connection Connection) ConnectionConfig {
	return ConnectionConfig{
		ClusterRef: connection.Labels[connectionClusterRefLabel],
		Internal:   strings.EqualFold(connection.Annotations[connectionWorkspaceManaged], "true"),
		Name:       connection.Name,
		Spec:       connection.Spec,
		Status:     &connection.Status,
	}
}

func normalizeConnectionTypeForTransport(value ConnectionType) ConnectionType {
	switch normalizeConnectionTypeForInternal(value) {
	case ConnectionTypePulsar:
		return ConnectionType(strings.ToUpper(string(ConnectionTypePulsar)))
	case ConnectionTypeKafka:
		return ConnectionType(strings.ToUpper(string(ConnectionTypeKafka)))
	case ConnectionTypeOther:
		return ConnectionType(strings.ToUpper(string(ConnectionTypeOther)))
	default:
		return ConnectionType(strings.TrimSpace(string(value)))
	}
}

func normalizeConnectionTypeForInternal(value ConnectionType) ConnectionType {
	trimmed := strings.TrimSpace(string(value))
	switch strings.ToLower(trimmed) {
	case string(ConnectionTypePulsar):
		return ConnectionTypePulsar
	case string(ConnectionTypeKafka):
		return ConnectionTypeKafka
	case string(ConnectionTypeOther):
		return ConnectionTypeOther
	default:
		return ConnectionType(trimmed)
	}
}
