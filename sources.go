// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/url"
)

// SourcesClient provides registry operations for workspace sources.
type SourcesClient struct {
	client *Client
}

// NewSourcesClient creates a workspace sources client. Sources are a StreamNative Cloud extension,
// served under CloudExtensionBasePath.
func NewSourcesClient(client *Client) *SourcesClient {
	return &SourcesClient{client: client.scoped(CloudExtensionBasePath)}
}

// List returns all source names visible in the current workspace.
func (c *SourcesClient) List(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(ctx, "/connectors/sources", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one source config by name.
func (c *SourcesClient) Get(ctx context.Context, name string) (*RegistrySourceConfig, error) {
	var result RegistrySourceConfig
	if err := c.client.GetJSON(ctx, "/connectors/sources/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Create creates a source using multipart/form-data.
func (c *SourcesClient) Create(ctx context.Context, cfg RegistrySourceConfig, filePath, packageURL string) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PostMultipart(ctx, "/connectors/sources/"+url.PathEscape(cfg.Name), MultipartRequest{
		File:        file,
		URL:         packageURL,
		ConfigField: "sourceConfig",
		Config:      cfg,
	})
}

// Update updates a source using multipart/form-data.
func (c *SourcesClient) Update(ctx context.Context, name string, cfg RegistrySourceConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PutMultipart(ctx, "/connectors/sources/"+url.PathEscape(name), MultipartRequest{
		File:          file,
		URL:           packageURL,
		ConfigField:   "sourceConfig",
		Config:        cfg,
		UpdateOptions: updateOptions,
	})
}

// Delete deletes a source by name.
func (c *SourcesClient) Delete(ctx context.Context, name string) error {
	return c.client.Delete(ctx, "/connectors/sources/"+url.PathEscape(name))
}

// Start starts all instances for a source.
func (c *SourcesClient) Start(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sources/"+url.PathEscape(name)+":start", nil, nil)
}

// Stop stops all instances for a source.
func (c *SourcesClient) Stop(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sources/"+url.PathEscape(name)+":stop", nil, nil)
}

// Restart restarts all instances for a source.
func (c *SourcesClient) Restart(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/connectors/sources/"+url.PathEscape(name)+":restart", nil, nil)
}

// StartInstance starts one source instance.
func (c *SourcesClient) StartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sourceInstanceActionPath(name, instanceID, "start"), nil, nil)
}

// StopInstance stops one source instance.
func (c *SourcesClient) StopInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sourceInstanceActionPath(name, instanceID, "stop"), nil, nil)
}

// RestartInstance restarts one source instance.
func (c *SourcesClient) RestartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, sourceInstanceActionPath(name, instanceID, "restart"), nil, nil)
}

// Status returns the aggregate source status.
func (c *SourcesClient) Status(ctx context.Context, name string) (*SourceStatus, error) {
	var result SourceStatus
	if err := c.client.GetJSON(ctx, "/connectors/sources/"+url.PathEscape(name)+"/status", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InstanceStatus returns one source instance status.
func (c *SourcesClient) InstanceStatus(ctx context.Context, name, instanceID string) (*SourceInstanceStatusData, error) {
	var result SourceInstanceStatusData
	path := "/connectors/sources/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + "/status"
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func sourceInstanceActionPath(name, instanceID, action string) string {
	return "/connectors/sources/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + ":" + action
}
