// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"

	"github.com/orca-ae/orca-sdk-go/option"
)

// DiscoveryService reports what a deployment can do.
type DiscoveryService struct {
	client *Client
}

// Groups lists the extension API groups this deployment serves.
//
// An empty list is a normal, fully-functional deployment with no extensions
// installed. It is not an error, and it is not the same as a 404 from /apis
// itself, which means the deployment predates discovery entirely.
//
// This is the raw probe and is deliberately uncached, so a caller polling while
// an extension is being installed sees it appear. The cloud service tree keeps
// its own cache for gating.
func (s DiscoveryService) Groups(ctx context.Context, opts ...option.RequestOption) (*APIGroupList, error) {
	return s.client.GetAPIGroups(ctx, opts...)
}

// PolicyGroupResources lists the resources advertised by policy.runorca.ai/v1.
func (s DiscoveryService) PolicyGroupResources(ctx context.Context, opts ...option.RequestOption) (*APIResourceList, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var result APIResourceList
	if err := s.client.getRootJSON(ctx, "apis/policy.runorca.ai/v1", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}

// PricingGroupResources lists the resources advertised by pricing.runorca.ai/v1.
func (s DiscoveryService) PricingGroupResources(ctx context.Context, opts ...option.RequestOption) (*APIResourceList, error) {
	if err := s.client.ensurePricingExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var result APIResourceList
	if err := s.client.getRootJSON(ctx, "apis/pricing.runorca.ai/v1", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}
