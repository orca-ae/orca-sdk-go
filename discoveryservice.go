// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

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

// HarnessCapabilities describes the engine wire response.
type HarnessCapabilities struct {
	ManagedSkills bool `json:"managed_skills"`
	Multiagent    bool `json:"multiagent"`
	NativeResume  bool `json:"native_resume"`
}

// HarnessModel describes the engine wire response.
type HarnessModel struct {
	Provider string   `json:"provider"`
	ID       string   `json:"id"`
	Efforts  []string `json:"efforts"`
}

// Harness describes the engine wire response.
type Harness struct {
	ID           string              `json:"id"`
	Provider     string              `json:"provider"`
	Modes        []string            `json:"modes"`
	Capabilities HarnessCapabilities `json:"capabilities"`
	Models       []HarnessModel      `json:"models"`
}

// HarnessList describes the engine wire response.
type HarnessList struct {
	Data []Harness `json:"data"`
}

// RuntimeGroupResources retrieves engine discovery for apis/runtime.runorca.ai/v1.
func (s DiscoveryService) RuntimeGroupResources(ctx context.Context, opts ...option.RequestOption) (*APIResourceList, error) {
	if err := s.client.ensureExtensionVersion(ctx, "runtime.runorca.ai", "v1", opts...); err != nil {
		return nil, err
	}
	var result APIResourceList
	if err := s.client.getRootJSON(ctx, "apis/runtime.runorca.ai/v1", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}

// Harnesses retrieves engine discovery for apis/runtime.runorca.ai/v1/harnesses.
func (s DiscoveryService) Harnesses(ctx context.Context, opts ...option.RequestOption) (*HarnessList, error) {
	if err := s.client.ensureExtensionVersion(ctx, "runtime.runorca.ai", "v1", opts...); err != nil {
		return nil, err
	}
	var result HarnessList
	if err := s.client.getRootJSON(ctx, "apis/runtime.runorca.ai/v1/harnesses", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}

// PricingV2GroupResources retrieves engine discovery for apis/pricing.runorca.ai/v2.
func (s DiscoveryService) PricingV2GroupResources(ctx context.Context, opts ...option.RequestOption) (*APIResourceList, error) {
	if err := s.client.ensureExtensionVersion(ctx, "pricing.runorca.ai", "v2", opts...); err != nil {
		return nil, err
	}
	var result APIResourceList
	if err := s.client.getRootJSON(ctx, "apis/pricing.runorca.ai/v2", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}
