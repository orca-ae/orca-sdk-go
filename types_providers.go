// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

// AgentProviderInfo describes a managed-agent provider exposed by the registry.
type AgentProviderInfo struct {
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	Type             string `json:"type,omitempty" yaml:"type,omitempty"`
	APIURL           string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	APIVersion       string `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	BetaVersion      string `json:"beta_version,omitempty" yaml:"beta_version,omitempty"`
	APIKeyEnv        string `json:"api_key_env,omitempty" yaml:"api_key_env,omitempty"`
	APIKeyConfigured bool   `json:"api_key_configured,omitempty" yaml:"api_key_configured,omitempty"`
}
