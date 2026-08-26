// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ManagedAgentsClient provides generic operations for the managed agents registry APIs.
//
// It does not inject a beta header or beta_version request-body field. anthropic-beta is accepted
// and ignored by the portable core contract, while orca-beta opts into a different wire dialect;
// neither is appropriate as an unconditional CLI default.
type ManagedAgentsClient struct {
	client *Client
}

// NewManagedAgentsClient creates a managed agents registry client.
func NewManagedAgentsClient(client *Client) *ManagedAgentsClient {
	return &ManagedAgentsClient{client: client}
}

// DoJSON sends a JSON request and returns the decoded JSON response.
func (c *ManagedAgentsClient) DoJSON(ctx context.Context, method, path string, payload interface{}) (interface{}, error) {
	var result interface{}
	if err := c.client.doJSON(ctx, method, path, payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns a managed agents object or collection.
func (c *ManagedAgentsClient) Get(ctx context.Context, path string) (interface{}, error) {
	return c.DoJSON(ctx, http.MethodGet, path, nil)
}

// GetToWriter streams a managed agents raw GET response to writer.
func (c *ManagedAgentsClient) GetToWriter(ctx context.Context, path string, writer io.Writer) error {
	return c.client.GetToWriter(ctx, path, writer)
}

// GetStream streams a managed agents raw GET response to a handler.
func (c *ManagedAgentsClient) GetStream(
	ctx context.Context,
	path string,
	accept string,
	handle func(io.Reader) error,
) error {
	return c.client.GetStream(ctx, path, accept, handle)
}

// Create sends a JSON create request.
func (c *ManagedAgentsClient) Create(ctx context.Context, path string, payload interface{}) (interface{}, error) {
	return c.DoJSON(ctx, http.MethodPost, path, payload)
}

// Update sends a JSON update request using the requested HTTP method.
func (c *ManagedAgentsClient) Update(ctx context.Context, method, path string, payload interface{}) (interface{}, error) {
	return c.DoJSON(ctx, method, path, payload)
}

// Delete sends a delete request and returns the decoded JSON response when present.
func (c *ManagedAgentsClient) Delete(ctx context.Context, path string) (interface{}, error) {
	return c.DoJSON(ctx, http.MethodDelete, path, nil)
}

// Archive sends an archive request and returns the decoded JSON response when present.
func (c *ManagedAgentsClient) Archive(ctx context.Context, path string) (interface{}, error) {
	return c.DoJSON(ctx, http.MethodPost, path, nil)
}

// DoMultipart sends a multipart request and returns the decoded JSON response when present.
func (c *ManagedAgentsClient) DoMultipart(
	ctx context.Context,
	method string,
	path string,
	payload MultipartRequest,
) (interface{}, error) {
	response, err := c.client.doMultipart(ctx, method, path, payload, true)
	if err != nil {
		return nil, err
	}
	if len(response) == 0 {
		return nil, nil
	}
	var result interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to decode %s %s response: %w", method, path, err)
	}
	return result, nil
}
