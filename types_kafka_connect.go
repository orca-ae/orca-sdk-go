// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

// ConnectorState represents the state name of a Kafka Connect connector or task.
type ConnectorState string

const (
	ConnectorStateRunning ConnectorState = "RUNNING"
	ConnectorStatePaused  ConnectorState = "PAUSED"
	ConnectorStateFailed  ConnectorState = "FAILED"
	ConnectorStateStopped ConnectorState = "STOPPED"
)

// ConnectorTaskID identifies one Kafka Connect task.
type ConnectorTaskID struct {
	Connector string `json:"connector,omitempty" yaml:"connector,omitempty"`
	Task      int    `json:"task,omitempty" yaml:"task,omitempty"`
}

// ConnectorInfo is the response returned by connector create/get/update APIs.
type ConnectorInfo struct {
	Name   string            `json:"name,omitempty" yaml:"name,omitempty"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
	Tasks  []ConnectorTaskID `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Type   string            `json:"type,omitempty" yaml:"type,omitempty"`
}

// ConnectorRuntimeState contains connector-level runtime state.
type ConnectorRuntimeState struct {
	State    ConnectorState `json:"state,omitempty" yaml:"state,omitempty"`
	WorkerID string         `json:"worker_id,omitempty" yaml:"worker_id,omitempty"`
	Message  string         `json:"msg,omitempty" yaml:"msg,omitempty"`
	Trace    string         `json:"trace,omitempty" yaml:"trace,omitempty"`
}

// TaskState contains one task's runtime state.
type TaskState struct {
	ID       int            `json:"id,omitempty" yaml:"id,omitempty"`
	State    ConnectorState `json:"state,omitempty" yaml:"state,omitempty"`
	WorkerID string         `json:"worker_id,omitempty" yaml:"worker_id,omitempty"`
	Message  string         `json:"msg,omitempty" yaml:"msg,omitempty"`
	Trace    string         `json:"trace,omitempty" yaml:"trace,omitempty"`
}

// ConnectorStateInfo is the response returned by connector status and expanded restart APIs.
type ConnectorStateInfo struct {
	Name      string                `json:"name,omitempty" yaml:"name,omitempty"`
	Connector ConnectorRuntimeState `json:"connector,omitempty" yaml:"connector,omitempty"`
	Tasks     []TaskState           `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Type      string                `json:"type,omitempty" yaml:"type,omitempty"`
}

// ConnectorStatus is retained as a compatibility alias for ConnectorStateInfo.
type ConnectorStatus = ConnectorStateInfo

// TaskConfig contains one task identifier and its configuration.
type TaskConfig struct {
	ID     ConnectorTaskID   `json:"id,omitempty" yaml:"id,omitempty"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

// TaskInfo is the Kafka Connect API name for a task configuration response.
type TaskInfo = TaskConfig

// PluginInfo contains connector plugin metadata.
type PluginInfo struct {
	Class   string `json:"class,omitempty" yaml:"class,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ConfigKeyInfo describes one Kafka Connect plugin configuration key.
type ConfigKeyInfo struct {
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Type          string   `json:"type,omitempty" yaml:"type,omitempty"`
	Required      bool     `json:"required,omitempty" yaml:"required,omitempty"`
	DefaultValue  string   `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	Importance    string   `json:"importance,omitempty" yaml:"importance,omitempty"`
	Documentation string   `json:"documentation,omitempty" yaml:"documentation,omitempty"`
	Group         string   `json:"group,omitempty" yaml:"group,omitempty"`
	OrderInGroup  int      `json:"order_in_group,omitempty" yaml:"order_in_group,omitempty"`
	Width         string   `json:"width,omitempty" yaml:"width,omitempty"`
	DisplayName   string   `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Dependents    []string `json:"dependents,omitempty" yaml:"dependents,omitempty"`
	Order         int      `json:"order,omitempty" yaml:"order,omitempty"`
}

// FunctionMeshConnectorDefinition describes an entry in the installed plugin catalog.
type FunctionMeshConnectorDefinition struct {
	ConnectorDefinition
	ID                           string                  `json:"id,omitempty" yaml:"id,omitempty"`
	Version                      string                  `json:"version,omitempty" yaml:"version,omitempty"`
	ImageRegistry                string                  `json:"imageRegistry,omitempty" yaml:"imageRegistry,omitempty"`
	ImageRepository              string                  `json:"imageRepository,omitempty" yaml:"imageRepository,omitempty"`
	ImageTag                     string                  `json:"imageTag,omitempty" yaml:"imageTag,omitempty"`
	TypeClassName                string                  `json:"typeClassName,omitempty" yaml:"typeClassName,omitempty"`
	SourceTypeClassName          string                  `json:"sourceTypeClassName,omitempty" yaml:"sourceTypeClassName,omitempty"`
	SinkTypeClassName            string                  `json:"sinkTypeClassName,omitempty" yaml:"sinkTypeClassName,omitempty"`
	JarFullName                  string                  `json:"jarFullName,omitempty" yaml:"jarFullName,omitempty"`
	DefaultSchemaType            string                  `json:"defaultSchemaType,omitempty" yaml:"defaultSchemaType,omitempty"`
	DefaultSerdeClassName        string                  `json:"defaultSerdeClassName,omitempty" yaml:"defaultSerdeClassName,omitempty"`
	IconLink                     string                  `json:"iconLink,omitempty" yaml:"iconLink,omitempty"`
	SinkDocLink                  string                  `json:"sinkDocLink,omitempty" yaml:"sinkDocLink,omitempty"`
	SourceDocLink                string                  `json:"sourceDocLink,omitempty" yaml:"sourceDocLink,omitempty"`
	SinkConfigFieldDefinitions   []ConfigFieldDefinition `json:"sinkConfigFieldDefinitions,omitempty" yaml:"sinkConfigFieldDefinitions,omitempty"`
	SourceConfigFieldDefinitions []ConfigFieldDefinition `json:"sourceConfigFieldDefinitions,omitempty" yaml:"sourceConfigFieldDefinitions,omitempty"`
	Jar                          string                  `json:"jar,omitempty" yaml:"jar,omitempty"`
}

// CreateConnectorRequest is the registry payload for connector creation.
type CreateConnectorRequest struct {
	Name         string            `json:"name,omitempty" yaml:"name,omitempty"`
	InitialState string            `json:"initial_state,omitempty" yaml:"initial_state,omitempty"`
	Config       map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

// RestartConnectorOptions controls connector/task restart scope.
type RestartConnectorOptions struct {
	IncludeTasks bool
	OnlyFailed   bool
}

// ServerInfo contains workspace Kafka Connect server metadata.
type ServerInfo struct {
	Version        string `json:"version,omitempty" yaml:"version,omitempty"`
	Commit         string `json:"commit,omitempty" yaml:"commit,omitempty"`
	KafkaClusterID string `json:"kafka_cluster_id,omitempty" yaml:"kafka_cluster_id,omitempty"`
}

// WorkerStatus contains Kafka Connect health status.
type WorkerStatus struct {
	Status  string `json:"status,omitempty" yaml:"status,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// ConnectorOffsets mirrors the offsets payload.
type ConnectorOffsets struct {
	Offsets []map[string]interface{} `json:"offsets,omitempty" yaml:"offsets,omitempty"`
}

// ConnectorActiveTopics mirrors Kafka Connect's nested active-topics response.
type ConnectorActiveTopics map[string]map[string][]string

// Message contains a mutation result message.
type Message struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}
