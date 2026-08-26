// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/url"
)

// SinksClient provides registry operations for workspace sinks.
type SinksClient struct {
	client *Client
}

// NewSinksClient creates a workspace sinks client. Sinks are a StreamNative Cloud extension,
// served under CloudExtensionBasePath.
func NewSinksClient(client *Client) *SinksClient {
	return &SinksClient{client: client.WithPathPrefix(CloudExtensionBasePath)}
}

// List returns all sink names visible in the current workspace.
func (c *SinksClient) List(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(ctx, "/connectors/sinks", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one sink config by name.
func (c *SinksClient) Get(ctx context.Context, name string) (*RegistrySinkConfig, error) {
	var result RegistrySinkConfig
	if err := c.client.GetJSON(ctx, "/connectors/sinks/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Create creates a sink using multipart/form-data.
func (c *SinksClient) Create(ctx context.Context, cfg RegistrySinkConfig, filePath, packageURL string) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PostMultipart(ctx, "/connectors/sinks/"+url.PathEscape(cfg.Name), MultipartRequest{
		File:        file,
		URL:         packageURL,
		ConfigField: "sinkConfig",
		Config:      cfg,
	})
}

// Update updates a sink using multipart/form-data.
func (c *SinksClient) Update(ctx context.Context, name string, cfg RegistrySinkConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PutMultipart(ctx, "/connectors/sinks/"+url.PathEscape(name), MultipartRequest{
		File:          file,
		URL:           packageURL,
		ConfigField:   "sinkConfig",
		Config:        cfg,
		UpdateOptions: updateOptions,
	})
}

// Delete deletes a sink by name.
func (c *SinksClient) Delete(ctx context.Context, name string) error {
	return c.client.Delete(ctx, "/connectors/sinks/"+url.PathEscape(name))
}

// Start starts all instances for a sink.
func (c *SinksClient) Start(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sinks/"+url.PathEscape(name)+":start", nil, nil)
}

// Stop stops all instances for a sink.
func (c *SinksClient) Stop(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sinks/"+url.PathEscape(name)+":stop", nil, nil)
}

// Restart restarts all instances for a sink.
func (c *SinksClient) Restart(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sinks/"+url.PathEscape(name)+":restart", nil, nil)
}

// StartInstance starts one sink instance.
func (c *SinksClient) StartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sinkInstanceActionPath(name, instanceID, "start"), nil, nil)
}

// StopInstance stops one sink instance.
func (c *SinksClient) StopInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sinkInstanceActionPath(name, instanceID, "stop"), nil, nil)
}

// RestartInstance restarts one sink instance.
func (c *SinksClient) RestartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sinkInstanceActionPath(name, instanceID, "restart"), nil, nil)
}

// Status returns the aggregate sink status.
func (c *SinksClient) Status(ctx context.Context, name string) (*SinkStatus, error) {
	var result SinkStatus
	if err := c.client.GetJSON(ctx, "/connectors/sinks/"+url.PathEscape(name)+"/status", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InstanceStatus returns one sink instance status.
func (c *SinksClient) InstanceStatus(ctx context.Context, name, instanceID string) (*SinkInstanceStatusData, error) {
	var result SinkInstanceStatusData
	path := "/connectors/sinks/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + "/status"
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func sinkInstanceActionPath(name, instanceID, action string) string {
	return "/connectors/sinks/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + ":" + action
}
