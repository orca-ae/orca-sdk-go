// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// EffectiveGuardrailBundleScope describes the engine wire response.
type EffectiveGuardrailBundleScope struct {
	OrgID       string `json:"org_id"`
	WorkspaceID string `json:"workspace_id"`
}

// EffectiveGuardrail describes the engine wire response.
type EffectiveGuardrail struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Tier   string           `json:"tier"`
	Phases []GuardrailPhase `json:"phases"`
	// Rule is unconstrained in the effective bundle contract.
	Rule       json.RawMessage     `json:"rule,omitzero"`
	Stateful   bool                `json:"stateful"`
	Mode       GuardrailMode       `json:"mode"`
	StateScope GuardrailStateScope `json:"state_scope,omitzero"`
	SubagentID string              `json:"subagent_id"`
}

// EffectiveGuardrailBundle describes the engine wire response.
type EffectiveGuardrailBundle struct {
	Schema                string                        `json:"schema"`
	BundleID              string                        `json:"bundle_id"`
	Generation            int64                         `json:"generation"`
	Scope                 EffectiveGuardrailBundleScope `json:"scope"`
	RuntimeConfigRevision string                        `json:"runtime_config_revision"`
	IssuedAt              string                        `json:"issued_at"`
	ExpiresAt             string                        `json:"expires_at"`
	SeedVerdict           string                        `json:"seed_verdict"`
	Guardrails            []EffectiveGuardrail          `json:"guardrails"`
	OmittedRules          int64                         `json:"omitted_rules"`
}

// GuardrailEffectiveParams negotiates rule support and conditional retrieval.
type GuardrailEffectiveParams struct {
	RuleKinds             param.Opt[string]
	GuardrailCapabilities param.Opt[string]
	IfNoneMatch           param.Opt[string]
}

// GuardrailEffectiveResult carries the lease headers even on a bodyless 304.
// Header includes ETag, Cache-Control and Orca-Guardrail-{Issued-At,Expires-At,Generation}.
type GuardrailEffectiveResult struct {
	StatusCode int
	Header     http.Header
	Bundle     *EffectiveGuardrailBundle
}

// Effective retrieves an engine policy bundle or renews its lease with a 304.
// It does not cache bundles or infer support for rule kinds or capabilities.
func (s GuardrailService) Effective(ctx context.Context, params GuardrailEffectiveParams, opts ...option.RequestOption) (*GuardrailEffectiveResult, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if v, ok := params.RuleKinds.Value(); ok {
		opts = append(opts, option.WithQuery("rule_kinds", v))
	}
	if v, ok := params.GuardrailCapabilities.Value(); ok {
		opts = append(opts, option.WithQuery("guardrail_capabilities", v))
	}
	if v, ok := params.IfNoneMatch.Value(); ok {
		opts = append(opts, option.WithHeader("If-None-Match", v))
	}
	cfg, err := s.client.cfg.With(opts...)
	if err != nil {
		return nil, err
	}
	endpoint, err := cfg.ResolveURL("apis/policy.runorca.ai/v1/effectiveguardrails")
	if err != nil {
		return nil, err
	}
	resp, err := cfg.Do(ctx, http.MethodGet, endpoint, nil, http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result := &GuardrailEffectiveResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone()}
	if resp.StatusCode == http.StatusNotModified {
		if cfg.RawBody != nil {
			*cfg.RawBody = nil
		}
		return result, nil
	}
	data, err := readResponseBodyLimited(resp.Body, defaultJSONResponseBodyLimit)
	if err != nil {
		return nil, err
	}
	if cfg.RawBody != nil {
		*cfg.RawBody = data
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apierror.FromResponse(http.MethodGet, endpoint, resp.StatusCode, string(data), resp.Header)
	}
	var bundle EffectiveGuardrailBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, &apierror.DecodeError{Method: http.MethodGet, URL: endpoint, Err: err}
	}
	result.Bundle = &bundle
	return result, nil
}
