// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FunctionsClient provides registry operations for workspace functions.
type FunctionsClient struct {
	client *Client
}

// NewFunctionsClient creates a workspace functions client. Functions are a StreamNative Cloud
// extension, served under CloudExtensionBasePath.
func NewFunctionsClient(client *Client) *FunctionsClient {
	return &FunctionsClient{client: client.WithPathPrefix(CloudExtensionBasePath)}
}

// List returns all function names visible in the current workspace.
func (c *FunctionsClient) List(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(ctx, "/functions", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one function config by name.
func (c *FunctionsClient) Get(ctx context.Context, name string) (*RegistryFunctionConfig, error) {
	var result RegistryFunctionConfig
	if err := c.client.GetJSON(ctx, "/functions/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Create creates a function using multipart/form-data.
func (c *FunctionsClient) Create(ctx context.Context, cfg RegistryFunctionConfig, filePath, packageURL string) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PostMultipart(ctx, "/functions/"+url.PathEscape(cfg.Name), MultipartRequest{
		File:        file,
		URL:         packageURL,
		ConfigField: "functionConfig",
		Config:      cfg,
	})
}

// Update updates a function using multipart/form-data.
func (c *FunctionsClient) Update(ctx context.Context, name string, cfg RegistryFunctionConfig, filePath, packageURL string, updateOptions *UpdateOptionsImpl) error {
	file, err := multipartFileFromPath(filePath)
	if err != nil {
		return err
	}
	return c.client.PutMultipart(ctx, "/functions/"+url.PathEscape(name), MultipartRequest{
		File:          file,
		URL:           packageURL,
		ConfigField:   "functionConfig",
		Config:        cfg,
		UpdateOptions: updateOptions,
	})
}

// Delete deletes a function by name.
func (c *FunctionsClient) Delete(ctx context.Context, name string) error {
	return c.client.Delete(ctx, "/functions/"+url.PathEscape(name))
}

// Start starts all instances for a function.
func (c *FunctionsClient) Start(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/functions/"+url.PathEscape(name)+":start", nil, nil)
}

// Stop stops all instances for a function.
func (c *FunctionsClient) Stop(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/functions/"+url.PathEscape(name)+":stop", nil, nil)
}

// Restart restarts all instances for a function.
func (c *FunctionsClient) Restart(ctx context.Context, name string) error {
	return c.client.PostJSON(ctx, "/functions/"+url.PathEscape(name)+":restart", nil, nil)
}

// StartInstance starts one function instance.
func (c *FunctionsClient) StartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, functionInstanceActionPath(name, instanceID, "start"), nil, nil)
}

// StopInstance stops one function instance.
func (c *FunctionsClient) StopInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, functionInstanceActionPath(name, instanceID, "stop"), nil, nil)
}

// RestartInstance restarts one function instance.
func (c *FunctionsClient) RestartInstance(ctx context.Context, name, instanceID string) error {
	return c.client.PostJSON(ctx, functionInstanceActionPath(name, instanceID, "restart"), nil, nil)
}

// Status returns the aggregate function status.
func (c *FunctionsClient) Status(ctx context.Context, name string) (*FunctionStatus, error) {
	var result FunctionStatus
	if err := c.client.GetJSON(ctx, "/functions/"+url.PathEscape(name)+"/status", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InstanceStatus returns one function instance status.
func (c *FunctionsClient) InstanceStatus(ctx context.Context, name, instanceID string) (*FunctionInstanceStatusData, error) {
	var result FunctionInstanceStatusData
	path := "/functions/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + "/status"
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Stats returns aggregate and per-instance function metrics.
func (c *FunctionsClient) Stats(ctx context.Context, name string) (*FunctionStats, error) {
	var result FunctionStats
	if err := c.client.GetJSON(ctx, "/functions/"+url.PathEscape(name)+"/stats", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InstanceStats returns metrics for one function instance.
func (c *FunctionsClient) InstanceStats(ctx context.Context, name, instanceID string) (*FunctionInstanceStatsData, error) {
	var result FunctionInstanceStatsData
	path := "/functions/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + "/stats"
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Trigger invokes a function with inline data or a data stream file.
func (c *FunctionsClient) Trigger(ctx context.Context, name, data, dataFilePath, topic string) (string, error) {
	file, err := multipartFileFromPathWithField(dataFilePath, "dataStream")
	if err != nil {
		return "", err
	}
	fields := map[string]string{}
	if data != "" {
		fields["data"] = data
	}
	if strings.TrimSpace(topic) != "" {
		fields["topic"] = topic
	}
	payload, err := c.client.PostMultipartWithResponse(ctx, "/functions/"+url.PathEscape(name)+":trigger", MultipartRequest{
		File:   file,
		Fields: fields,
	})
	if err != nil {
		return "", err
	}
	// A function without an output topic returns a successful empty response
	// after its input message has been published.
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 || bytes.Equal(trimmedPayload, []byte("null")) {
		return "", nil
	}
	var result string
	if err := json.Unmarshal(payload, &result); err != nil {
		return "", fmt.Errorf("failed to decode function trigger response: %w", err)
	}
	return result, nil
}

// GetState returns one function state value.
func (c *FunctionsClient) GetState(ctx context.Context, name, key string) (*FunctionState, error) {
	var result FunctionState
	path := "/functions/" + url.PathEscape(name) + "/state/" + url.PathEscape(key)
	if err := c.client.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PutState updates one function state value.
func (c *FunctionsClient) PutState(ctx context.Context, name, key string, state FunctionState) error {
	path := "/functions/" + url.PathEscape(name) + "/state/" + url.PathEscape(key)
	return c.client.PostMultipart(ctx, path, MultipartRequest{
		ConfigField: "state",
		Config:      state,
	})
}

func functionInstanceActionPath(name, instanceID, action string) string {
	return "/functions/" + url.PathEscape(name) + "/" + url.PathEscape(instanceID) + ":" + action
}

func multipartFileFromPath(filePath string) (*MultipartFile, error) {
	return multipartFileFromPathWithField(filePath, "data")
}

func multipartFileFromPathWithField(filePath, fieldName string) (*MultipartFile, error) {
	if filePath == "" {
		return nil, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return &MultipartFile{
		FieldName: fieldName,
		FileName:  filepath.Base(filePath),
		Content:   content,
	}, nil
}
