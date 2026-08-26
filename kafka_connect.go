// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// ErrKafkaConnectConfigValidationUnsupported reports the server's documented validation limitation.
var ErrKafkaConnectConfigValidationUnsupported = errors.New("Kafka Connect plugin config validation is not supported by the registry server")

// KafkaConnectClient provides workspace registry operations for Kafka Connect.
type KafkaConnectClient struct {
	client *Client
}

// NewKafkaConnectClient creates a workspace Kafka Connect registry client. Kafka Connect is a
// StreamNative Cloud extension, served under CloudExtensionBasePath.
func NewKafkaConnectClient(client *Client) *KafkaConnectClient {
	if client == nil {
		return &KafkaConnectClient{}
	}
	return &KafkaConnectClient{client: client.scoped(CloudExtensionBasePath)}
}

// GetInfo returns Kafka Connect server info.
func (c *KafkaConnectClient) GetInfo(ctx context.Context) (*ServerInfo, error) {
	var result ServerInfo
	if err := c.client.GetJSON(ctx, "/connectors/kafka", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHealth returns Kafka Connect worker health.
func (c *KafkaConnectClient) GetHealth(ctx context.Context) (*WorkerStatus, error) {
	var result WorkerStatus
	if err := c.client.GetJSON(ctx, "/connectors/kafka/health", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListConnectors lists connector names.
func (c *KafkaConnectClient) ListConnectors(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConnector returns one connector by name.
func (c *KafkaConnectClient) GetConnector(ctx context.Context, name string) (*ConnectorInfo, error) {
	var result ConnectorInfo
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateConnector creates a new connector and returns the server response.
func (c *KafkaConnectClient) CreateConnector(ctx context.Context, req CreateConnectorRequest) (*ConnectorInfo, error) {
	var result ConnectorInfo
	if err := c.client.PostJSON(ctx, "/connectors/kafka/connectors", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateConnectorConfig replaces connector config and returns the server response.
func (c *KafkaConnectClient) UpdateConnectorConfig(ctx context.Context, name string, config map[string]string) (*ConnectorInfo, error) {
	var result ConnectorInfo
	if err := c.client.PutJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+"/config", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PatchConnectorConfig is intentionally disabled because the Java runtime has no PATCH route.
func (c *KafkaConnectClient) PatchConnectorConfig(context.Context, string, map[string]string) (*ConnectorInfo, error) {
	return nil, errors.New("Kafka Connect config PATCH is not supported by the runtime; use UpdateConnectorConfig")
}

// DeleteConnector deletes a connector.
func (c *KafkaConnectClient) DeleteConnector(ctx context.Context, name string) error {
	return c.client.Delete(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name))
}

// PauseConnector pauses a connector.
func (c *KafkaConnectClient) PauseConnector(ctx context.Context, name string) error {
	return c.client.PutJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+":pause", nil, nil)
}

// ResumeConnector resumes a connector.
func (c *KafkaConnectClient) ResumeConnector(ctx context.Context, name string) error {
	return c.client.PutJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+":resume", nil, nil)
}

// RestartConnector restarts only the connector, preserving the previous simple API.
func (c *KafkaConnectClient) RestartConnector(ctx context.Context, name string) error {
	_, err := c.RestartConnectorWithOptions(ctx, name, RestartConnectorOptions{})
	return err
}

// RestartConnectorWithOptions restarts a connector and optionally its tasks.
func (c *KafkaConnectClient) RestartConnectorWithOptions(ctx context.Context, name string, options RestartConnectorOptions) (*ConnectorStateInfo, error) {
	query := url.Values{}
	query.Set("includeTasks", strconv.FormatBool(options.IncludeTasks))
	query.Set("onlyFailed", strconv.FormatBool(options.OnlyFailed))
	path := "/connectors/kafka/connectors/" + url.PathEscape(name) + ":restart?" + query.Encode()
	var result ConnectorStateInfo
	if err := c.client.PostJSON(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	if result.Name == "" && result.Type == "" && result.Connector.State == "" && len(result.Tasks) == 0 {
		return nil, nil
	}
	return &result, nil
}

// StopConnector stops a connector.
func (c *KafkaConnectClient) StopConnector(ctx context.Context, name string) error {
	return c.client.PutJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+":stop", nil, nil)
}

// GetConnectorConfig returns the connector config map.
func (c *KafkaConnectClient) GetConnectorConfig(ctx context.Context, name string) (map[string]string, error) {
	var result map[string]string
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+"/config", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConnectorStatus returns aggregate connector status.
func (c *KafkaConnectClient) GetConnectorStatus(ctx context.Context, name string) (*ConnectorStateInfo, error) {
	var result ConnectorStateInfo
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+"/status", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConnectorTasks returns connector task configurations.
func (c *KafkaConnectClient) GetConnectorTasks(ctx context.Context, name string) ([]TaskConfig, error) {
	var result []TaskConfig
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+"/tasks", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConnectorTasksConfig returns the deprecated task-config map.
func (c *KafkaConnectClient) GetConnectorTasksConfig(ctx context.Context, name string) (map[string]map[string]string, error) {
	var result map[string]map[string]string
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(name)+"/tasks-config", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTaskStatus returns one connector task status.
func (c *KafkaConnectClient) GetTaskStatus(ctx context.Context, connector string, taskID int) (*TaskState, error) {
	var result TaskState
	path := "/connectors/kafka/connectors/" + url.PathEscape(connector) + "/tasks/" + strconv.Itoa(taskID) + "/status"
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RestartTask restarts one connector task.
func (c *KafkaConnectClient) RestartTask(ctx context.Context, connector string, taskID int) error {
	path := "/connectors/kafka/connectors/" + url.PathEscape(connector) + "/tasks/" + strconv.Itoa(taskID) + "/restart"
	return c.client.PostJSON(ctx, path, nil, nil)
}

// DescribePluginConfig returns plugin configuration definitions.
func (c *KafkaConnectClient) DescribePluginConfig(ctx context.Context, pluginName string) ([]ConfigKeyInfo, error) {
	var result []ConfigKeyInfo
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connector-plugins/"+url.PathEscape(pluginName)+"/config", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateConfig always reports unsupported because the Java endpoint only returns HTTP 400.
func (c *KafkaConnectClient) ValidateConfig(context.Context, string, map[string]string) (map[string]interface{}, error) {
	return nil, ErrKafkaConnectConfigValidationUnsupported
}

// ListPlugins lists connector plugins. Optional connectorsOnly preserves the server default when omitted.
func (c *KafkaConnectClient) ListPlugins(ctx context.Context, connectorsOnly ...bool) ([]PluginInfo, error) {
	path := "/connectors/kafka/connector-plugins"
	if len(connectorsOnly) > 0 {
		query := url.Values{}
		query.Set("connectorsOnly", strconv.FormatBool(connectorsOnly[0]))
		path += "?" + query.Encode()
	}
	var result []PluginInfo
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListPluginCatalog returns installed Function Mesh connector definitions.
func (c *KafkaConnectClient) ListPluginCatalog(ctx context.Context) ([]FunctionMeshConnectorDefinition, error) {
	var result []FunctionMeshConnectorDefinition
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connector-plugins/catalog", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetActiveTopics returns topics actively used by a connector.
func (c *KafkaConnectClient) GetActiveTopics(ctx context.Context, connector string) (ConnectorActiveTopics, error) {
	var result ConnectorActiveTopics
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(connector)+"/topics", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ResetActiveTopics clears a connector's active-topic tracking data.
func (c *KafkaConnectClient) ResetActiveTopics(ctx context.Context, connector string) error {
	return c.client.PutJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(connector)+"/topics:reset", nil, nil)
}

// GetOffsets returns current connector offsets.
func (c *KafkaConnectClient) GetOffsets(ctx context.Context, connector string) (*ConnectorOffsets, error) {
	var result ConnectorOffsets
	if err := c.client.GetJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(connector)+"/offsets", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ResetOffsets resets connector offsets.
func (c *KafkaConnectClient) ResetOffsets(ctx context.Context, connector string) error {
	return c.client.Delete(ctx, "/connectors/kafka/connectors/"+url.PathEscape(connector)+"/offsets")
}

// AlterOffsets alters connector offsets.
func (c *KafkaConnectClient) AlterOffsets(ctx context.Context, connector string, offsets ConnectorOffsets) error {
	return c.client.PatchJSON(ctx, "/connectors/kafka/connectors/"+url.PathEscape(connector)+"/offsets", offsets, nil)
}
