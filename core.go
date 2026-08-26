// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"

	"github.com/orca-ae/orca-sdk-go/option"
)

// APIVersions describes the core API versions served by a deployment.
type APIVersions struct {
	Kind             string   `json:"kind" yaml:"kind"`
	Versions         []string `json:"versions" yaml:"versions"`
	PreferredVersion string   `json:"preferred_version" yaml:"preferred_version"`
}

// CoreProbeStatus is the response returned by the core liveness and readiness probes.
type CoreProbeStatus struct {
	Status  string `json:"status" yaml:"status"`
	Service string `json:"service" yaml:"service"`
}

// GetAPIVersions calls authenticated GET /api at the deployment host root.
func (c *Client) GetAPIVersions(ctx context.Context, opts ...option.RequestOption) (*APIVersions, error) {
	var result APIVersions
	if err := c.getRootJSON(ctx, "api", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHealthz calls GET /healthz at the deployment host root.
func (c *Client) GetHealthz(ctx context.Context, opts ...option.RequestOption) (*CoreProbeStatus, error) {
	return c.getCoreProbe(ctx, "healthz", opts...)
}

// GetReadyz calls GET /readyz at the deployment host root.
func (c *Client) GetReadyz(ctx context.Context, opts ...option.RequestOption) (*CoreProbeStatus, error) {
	return c.getCoreProbe(ctx, "readyz", opts...)
}

func (c *Client) getCoreProbe(ctx context.Context, path string, opts ...option.RequestOption) (*CoreProbeStatus, error) {
	var result CoreProbeStatus
	if err := c.getRootJSON(ctx, path, &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}
