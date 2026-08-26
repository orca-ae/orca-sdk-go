// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/url"
)

// ProvidersClient provides registry operations for managed-agent provider discovery.
type ProvidersClient struct {
	client *Client
}

// NewProvidersClient creates a workspace providers client. Agent provider discovery is a
// StreamNative Cloud extension, served under CloudExtensionBasePath.
func NewProvidersClient(client *Client) *ProvidersClient {
	return &ProvidersClient{client: client.WithPathPrefix(CloudExtensionBasePath)}
}

// List returns all managed-agent providers visible in the current workspace.
func (c *ProvidersClient) List(ctx context.Context) ([]AgentProviderInfo, error) {
	var result []AgentProviderInfo
	if err := c.client.GetJSON(ctx, "/agents/providers", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one managed-agent provider by name.
func (c *ProvidersClient) Get(ctx context.Context, name string) (*AgentProviderInfo, error) {
	var result AgentProviderInfo
	if err := c.client.GetJSON(ctx, "/agents/providers/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
