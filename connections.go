// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/url"
)

// ConnectionsClient provides registry operations for Connection resources.
type ConnectionsClient struct {
	client *Client
}

// NewConnectionsClient creates a Connection registry client. Connections are a StreamNative Cloud
// extension, served under CloudExtensionBasePath.
func NewConnectionsClient(client *Client) *ConnectionsClient {
	return &ConnectionsClient{client: client.scoped(CloudExtensionBasePath)}
}

// List returns all connections visible in the current workspace.
func (c *ConnectionsClient) List(ctx context.Context) ([]ConnectionConfig, error) {
	var result []ConnectionConfig
	if err := c.client.GetJSON(ctx, "/connections", &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Get returns one connection by name.
func (c *ConnectionsClient) Get(ctx context.Context, name string) (*ConnectionConfig, error) {
	var result ConnectionConfig
	if err := c.client.GetJSON(ctx, "/connections/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Create creates a connection.
func (c *ConnectionsClient) Create(ctx context.Context, cfg ConnectionConfig) error {
	return c.client.PostJSON(ctx, "/connections", cfg, nil)
}

// Validate checks a connection configuration without creating it.
func (c *ConnectionsClient) Validate(ctx context.Context, cfg ConnectionConfig) error {
	return c.client.PostJSON(ctx, "/connections/validate", cfg, nil)
}

// Update updates a connection.
func (c *ConnectionsClient) Update(ctx context.Context, name string, cfg ConnectionConfig) error {
	return c.client.PutJSON(ctx, "/connections/"+url.PathEscape(name), cfg, nil)
}

// Delete deletes a connection by name.
func (c *ConnectionsClient) Delete(ctx context.Context, name string) error {
	return c.client.Delete(ctx, "/connections/"+url.PathEscape(name))
}

// Test runs a connection health test.
func (c *ConnectionsClient) Test(ctx context.Context, name string) (*ConnectionHealthStatus, error) {
	var result ConnectionHealthStatus
	if err := c.client.GetJSON(ctx, "/connections/"+url.PathEscape(name)+":test", &result); err != nil {
		return nil, err
	}

	return &result, nil
}
