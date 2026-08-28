// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import "context"

// HealthClient provides StreamNative Cloud registry service health probes.
type HealthClient struct {
	client *Client
}

// NewHealthClient creates a registry health client under CloudExtensionBasePath.
func NewHealthClient(client *Client) *HealthClient {
	return &HealthClient{client: client.scoped(CloudExtensionBasePath)}
}

// Health reports aggregate registry health.
func (c *HealthClient) Health(ctx context.Context) (bool, error) {
	return c.get(ctx, "/health")
}

// Ready reports registry readiness.
func (c *HealthClient) Ready(ctx context.Context) (bool, error) {
	return c.get(ctx, "/health/ready")
}

// Live reports registry liveness.
func (c *HealthClient) Live(ctx context.Context) (bool, error) {
	return c.get(ctx, "/health/live")
}

func (c *HealthClient) get(ctx context.Context, path string) (bool, error) {
	var result bool
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return false, err
	}
	return result, nil
}
