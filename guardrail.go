// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// GuardrailService manages guardrails served by the policy extension.
//
// Every operation is gated on the deployment advertising policy.runorca.ai.
type GuardrailService struct {
	client *Client
}

// GuardrailPhase is a point in the agent loop where a guardrail can run.
type GuardrailPhase string

const (
	GuardrailPhaseRequest     GuardrailPhase = "request"
	GuardrailPhaseToolCall    GuardrailPhase = "tool_call"
	GuardrailPhaseToolResult  GuardrailPhase = "tool_result"
	GuardrailPhaseResponse    GuardrailPhase = "response"
	GuardrailPhaseLLMRequest  GuardrailPhase = "llm_request"
	GuardrailPhaseLLMResponse GuardrailPhase = "llm_response"
)

// GuardrailScope controls how a guardrail is attached.
type GuardrailScope string

const (
	GuardrailScopeOrganization GuardrailScope = "organization"
	GuardrailScopeWorkspace    GuardrailScope = "workspace"
	GuardrailScopeExplicit     GuardrailScope = "explicit"
)

// GuardrailVerdict is the action an expression guardrail takes when it fails.
type GuardrailVerdict string

const (
	GuardrailVerdictAsk  GuardrailVerdict = "ask"
	GuardrailVerdictDeny GuardrailVerdict = "deny"
)

// GuardrailStateScope controls how long a stateful builtin retains state.
type GuardrailStateScope string

const (
	// GuardrailStateScopeWorkspaceWindow shares state across a workspace window.
	GuardrailStateScopeWorkspaceWindow GuardrailStateScope = "workspace_window"
	GuardrailStateScopeTurn            GuardrailStateScope = "turn"
	GuardrailStateScopeSession         GuardrailStateScope = "session"
	GuardrailStateScopeSubjectWindow   GuardrailStateScope = "subject_window"
)

// GuardrailRuleKind discriminates known and future guardrail rule shapes.
type GuardrailRuleKind string

const (
	// GuardrailRuleDogwood selects the Dogwood policy language.
	GuardrailRuleDogwood    GuardrailRuleKind = "dogwood"
	GuardrailRuleBuiltin    GuardrailRuleKind = "builtin"
	GuardrailRuleExpression GuardrailRuleKind = "expression"
)

// GuardrailMode controls whether policy violations are enforced or observed.
type GuardrailMode string

const (
	// GuardrailModeEnforce enforces policy verdicts.
	GuardrailModeEnforce GuardrailMode = "enforce"
	// GuardrailModeObserve records violations without enforcement.
	GuardrailModeObserve GuardrailMode = "observe"
)

// GuardrailMCPManifest supplies tool definitions for a Dogwood policy.
type GuardrailMCPManifest struct {
	Server string           `json:"server"`
	URL    string           `json:"url"`
	Tools  []map[string]any `json:"tools"`
}

// GuardrailRule holds a known rule or the full wire object for a future kind.
// Create and Update accept only known kinds; decoding preserves unknown kinds.
type GuardrailRule struct {
	Kind         GuardrailRuleKind      `json:"kind"`
	Builtin      string                 `json:"builtin,omitzero"`
	Params       map[string]any         `json:"params,omitzero"`
	Expression   string                 `json:"expression,omitzero"`
	OnFalse      GuardrailVerdict       `json:"on_false,omitzero"`
	Reason       string                 `json:"reason,omitzero"`
	Policy       string                 `json:"policy,omitzero"`
	AppliesTo    string                 `json:"applies_to,omitzero"`
	History      string                 `json:"history,omitzero"`
	MCPManifests []GuardrailMCPManifest `json:"mcp_manifests,omitzero"`
	// Raw contains the complete object only for an unknown kind.
	Raw json.RawMessage `json:"-"`
}

// Validate checks known rule authoring without restricting future responses.
func (r GuardrailRule) Validate() error {
	switch r.Kind {
	case GuardrailRuleBuiltin:
		if r.Builtin == "" {
			return fmt.Errorf("builtin rule requires builtin")
		}
	case GuardrailRuleExpression:
		if r.Expression == "" || (r.OnFalse != GuardrailVerdictAsk && r.OnFalse != GuardrailVerdictDeny) {
			return fmt.Errorf("expression rule requires expression and on_false ask or deny")
		}
	case GuardrailRuleDogwood:
		if r.Policy == "" {
			return fmt.Errorf("dogwood rule requires policy")
		}
		if r.AppliesTo != "" && r.AppliesTo != "all_tools" && r.AppliesTo != "mcp_tools" {
			return fmt.Errorf("invalid dogwood applies_to %q", r.AppliesTo)
		}
		if r.History != "" && r.History != "session" && r.History != "principal" {
			return fmt.Errorf("invalid dogwood history %q", r.History)
		}
		if len(r.MCPManifests) > 64 {
			return fmt.Errorf("dogwood allows at most 64 MCP manifests")
		}
		for _, manifest := range r.MCPManifests {
			parsed, err := url.Parse(manifest.URL)
			if manifest.Server == "" || err != nil || parsed.Scheme == "" || manifest.Tools == nil || len(manifest.Tools) > 1000 {
				return fmt.Errorf("dogwood MCP manifest requires server, absolute URL and at most 1000 tools")
			}
		}
	default:
		return fmt.Errorf("cannot author unknown guardrail rule kind %q", r.Kind)
	}
	return nil
}

// MarshalJSON preserves complete unknown objects and selects known fields.
func (r GuardrailRule) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case GuardrailRuleBuiltin:
		return json.Marshal(struct {
			Kind    GuardrailRuleKind `json:"kind"`
			Builtin string            `json:"builtin"`
			Params  map[string]any    `json:"params,omitzero"`
		}{r.Kind, r.Builtin, r.Params})
	case GuardrailRuleExpression:
		return json.Marshal(struct {
			Kind       GuardrailRuleKind `json:"kind"`
			Expression string            `json:"expression"`
			OnFalse    GuardrailVerdict  `json:"on_false"`
			Reason     string            `json:"reason,omitzero"`
		}{r.Kind, r.Expression, r.OnFalse, r.Reason})
	case GuardrailRuleDogwood:
		return json.Marshal(struct {
			Kind         GuardrailRuleKind      `json:"kind"`
			Policy       string                 `json:"policy"`
			AppliesTo    string                 `json:"applies_to,omitzero"`
			History      string                 `json:"history,omitzero"`
			MCPManifests []GuardrailMCPManifest `json:"mcp_manifests,omitzero"`
		}{r.Kind, r.Policy, r.AppliesTo, r.History, r.MCPManifests})
	default:
		if len(r.Raw) != 0 {
			return append([]byte(nil), r.Raw...), nil
		}
		return json.Marshal(struct {
			Kind GuardrailRuleKind `json:"kind"`
		}{r.Kind})
	}
}

// UnmarshalJSON rejects malformed known shapes rather than hiding them as unknown.
func (r *GuardrailRule) UnmarshalJSON(data []byte) error {
	var tag struct {
		Kind GuardrailRuleKind `json:"kind"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	if tag.Kind == "" {
		return fmt.Errorf("guardrail rule requires kind")
	}
	switch tag.Kind {
	case GuardrailRuleBuiltin, GuardrailRuleExpression, GuardrailRuleDogwood:
		type shape GuardrailRule
		var decoded shape
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		rule := GuardrailRule(decoded)
		if err := rule.Validate(); err != nil {
			return err
		}
		// Drop fields of other variants, matching the original union behavior.
		switch rule.Kind {
		case GuardrailRuleBuiltin:
			*r = GuardrailRule{Kind: rule.Kind, Builtin: rule.Builtin, Params: rule.Params}
		case GuardrailRuleExpression:
			*r = GuardrailRule{Kind: rule.Kind, Expression: rule.Expression, OnFalse: rule.OnFalse, Reason: rule.Reason}
		case GuardrailRuleDogwood:
			*r = GuardrailRule{Kind: rule.Kind, Policy: rule.Policy, AppliesTo: rule.AppliesTo, History: rule.History, MCPManifests: rule.MCPManifests}
		}
	default:
		*r = GuardrailRule{Kind: tag.Kind, Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}

// Guardrail is a policy applied to one or more agent-loop phases.
type Guardrail struct {
	Mode        GuardrailMode     `json:"mode"`
	Warnings    []string          `json:"warnings,omitzero"`
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Phases      []GuardrailPhase  `json:"phases"`
	Scope       GuardrailScope    `json:"scope"`
	Rule        GuardrailRule     `json:"rule"`
	Metadata    map[string]string `json:"metadata,omitzero"`
	ArchivedAt  *string           `json:"archived_at"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// GuardrailDeleted is the tombstone returned after permanent deletion.
type GuardrailDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// GuardrailType describes one builtin guardrail and its parameter schema.
type GuardrailType struct {
	Name         string               `json:"name"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	Phases       []GuardrailPhase     `json:"phases"`
	Stateful     bool                 `json:"stateful"`
	StateScope   *GuardrailStateScope `json:"stateScope,omitzero"`
	Verdicts     []GuardrailVerdict   `json:"verdicts"`
	ParamsSchema map[string]any       `json:"paramsSchema"`
}

// GuardrailTypeList is the builtin guardrail catalog response.
type GuardrailTypeList struct {
	Data []GuardrailType `json:"data"`
}

// GuardrailNewParams creates a guardrail.
type GuardrailNewParams struct {
	Mode        param.Opt[GuardrailMode]  `json:"mode,omitzero"`
	Name        string                    `json:"name"`
	Description param.Opt[string]         `json:"description,omitzero"`
	Enabled     param.Opt[bool]           `json:"enabled,omitzero"`
	Phases      []GuardrailPhase          `json:"phases,omitzero"`
	Scope       param.Opt[GuardrailScope] `json:"scope,omitzero"`
	Rule        GuardrailRule             `json:"rule"`
	Metadata    map[string]string         `json:"metadata,omitzero"`
}

// GuardrailUpdateParams partially updates a guardrail.
type GuardrailUpdateParams struct {
	Mode        param.Opt[GuardrailMode]      `json:"mode,omitzero"`
	Name        param.Opt[string]             `json:"name,omitzero"`
	Description param.Opt[string]             `json:"description,omitzero"`
	Enabled     param.Opt[bool]               `json:"enabled,omitzero"`
	Phases      param.Opt[[]GuardrailPhase]   `json:"phases,omitzero"`
	Scope       param.Opt[GuardrailScope]     `json:"scope,omitzero"`
	Rule        param.Opt[GuardrailRule]      `json:"rule,omitzero"`
	Metadata    param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// GuardrailListParams filters and pages guardrails.
type GuardrailListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// Create creates a guardrail.
func (s GuardrailService) Create(ctx context.Context, params GuardrailNewParams, opts ...option.RequestOption) (*Guardrail, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if err := params.Rule.Validate(); err != nil {
		return nil, err
	}
	var guardrail Guardrail
	if err := s.client.PostJSON(ctx, "apis/policy.runorca.ai/v1/guardrails", params, &guardrail, opts...); err != nil {
		return nil, err
	}
	return &guardrail, nil
}

// List returns one page of visible guardrails.
func (s GuardrailService) List(ctx context.Context, params GuardrailListParams, opts ...option.RequestOption) (*pagination.PageCursor[Guardrail], error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	opts = appendListQuery(opts, params.Limit, params.Page)
	if includeArchived, ok := params.IncludeArchived.Value(); ok {
		opts = append(opts, option.WithQuery("include_archived", formatBool(includeArchived)))
	}
	return ListPage[Guardrail](ctx, s.client, "apis/policy.runorca.ai/v1/guardrails", opts...)
}

// ListAutoPaging returns an iterator over all visible guardrails.
func (s GuardrailService) ListAutoPaging(ctx context.Context, params GuardrailListParams, opts ...option.RequestOption) (*pagination.PageCursorAutoPager[Guardrail], error) {
	page, err := s.List(ctx, params, opts...)
	if err != nil {
		return nil, err
	}
	return page.AutoPager(ctx), nil
}

// Get retrieves a guardrail by ID.
func (s GuardrailService) Get(ctx context.Context, guardrailID string, opts ...option.RequestOption) (*Guardrail, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var guardrail Guardrail
	path := "apis/policy.runorca.ai/v1/guardrails/" + url.PathEscape(guardrailID)
	if err := s.client.GetJSON(ctx, path, &guardrail, opts...); err != nil {
		return nil, err
	}
	return &guardrail, nil
}

// Update partially updates a guardrail using POST.
func (s GuardrailService) Update(ctx context.Context, guardrailID string, params GuardrailUpdateParams, opts ...option.RequestOption) (*Guardrail, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if rule, ok := params.Rule.Value(); ok {
		if err := rule.Validate(); err != nil {
			return nil, err
		}
	}
	var guardrail Guardrail
	path := "apis/policy.runorca.ai/v1/guardrails/" + url.PathEscape(guardrailID)
	if err := s.client.PostJSON(ctx, path, params, &guardrail, opts...); err != nil {
		return nil, err
	}
	return &guardrail, nil
}

// Archive archives a guardrail.
func (s GuardrailService) Archive(ctx context.Context, guardrailID string, opts ...option.RequestOption) (*Guardrail, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var guardrail Guardrail
	path := "apis/policy.runorca.ai/v1/guardrails/" + url.PathEscape(guardrailID) + "/archive"
	if err := s.client.PostJSON(ctx, path, nil, &guardrail, opts...); err != nil {
		return nil, err
	}
	return &guardrail, nil
}

// Delete permanently deletes an unreferenced guardrail.
func (s GuardrailService) Delete(ctx context.Context, guardrailID string, opts ...option.RequestOption) (*GuardrailDeleted, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var deleted GuardrailDeleted
	path := "apis/policy.runorca.ai/v1/guardrails/" + url.PathEscape(guardrailID)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// ListTypes lists the builtin guardrail catalog and parameter schemas.
func (s GuardrailService) ListTypes(ctx context.Context, opts ...option.RequestOption) (*GuardrailTypeList, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var types GuardrailTypeList
	if err := s.client.GetJSON(ctx, "apis/policy.runorca.ai/v1/guardrailtypes", &types, opts...); err != nil {
		return nil, err
	}
	return &types, nil
}
