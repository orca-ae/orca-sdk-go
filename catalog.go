// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"net/url"
)

// CatalogClient provides registry operations for built-in connector catalog discovery.
type CatalogClient struct {
	client *Client
}

// NewCatalogClient creates a workspace catalog client. The built-in connector catalog is a
// StreamNative Cloud extension, served under CloudExtensionBasePath.
func NewCatalogClient(client *Client) *CatalogClient {
	return &CatalogClient{client: client.WithPathPrefix(CloudExtensionBasePath)}
}

// ListSources returns all built-in source connector definitions visible in the current workspace.
func (c *CatalogClient) ListSources(ctx context.Context) ([]ConnectorDefinition, error) {
	var result []ConnectorDefinition
	if err := c.client.GetJSON(ctx, "/catalog/sources", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetSourceConfigDefinition returns the configuration field definitions for a built-in source connector.
func (c *CatalogClient) GetSourceConfigDefinition(ctx context.Context, name string) ([]ConfigFieldDefinition, error) {
	var result []ConfigFieldDefinition
	if err := c.client.GetJSON(ctx, "/catalog/sources/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListSinks returns all built-in sink connector definitions visible in the current workspace.
func (c *CatalogClient) ListSinks(ctx context.Context) ([]ConnectorDefinition, error) {
	var result []ConnectorDefinition
	if err := c.client.GetJSON(ctx, "/catalog/sinks", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetSinkConfigDefinition returns the configuration field definitions for a built-in sink connector.
func (c *CatalogClient) GetSinkConfigDefinition(ctx context.Context, name string) ([]ConfigFieldDefinition, error) {
	var result []ConfigFieldDefinition
	if err := c.client.GetJSON(ctx, "/catalog/sinks/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListKafkaConnectors returns built-in Kafka Connect connector definitions.
func (c *CatalogClient) ListKafkaConnectors(ctx context.Context) ([]ConnectorDefinition, error) {
	var result []ConnectorDefinition
	if err := c.client.GetJSON(ctx, "/catalog/kafka", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetKafkaConfigDefinition returns configuration fields for a Kafka connector.
func (c *CatalogClient) GetKafkaConfigDefinition(ctx context.Context, name string) ([]ConfigFieldDefinition, error) {
	var result []ConfigFieldDefinition
	if err := c.client.GetJSON(ctx, "/catalog/kafka/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return result, nil
}
