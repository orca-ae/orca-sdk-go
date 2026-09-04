// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
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
	GuardrailStateScopeTurn          GuardrailStateScope = "turn"
	GuardrailStateScopeSession       GuardrailStateScope = "session"
	GuardrailStateScopeSubjectWindow GuardrailStateScope = "subject_window"
)

// GuardrailRuleKind discriminates the two guardrail rule shapes.
type GuardrailRuleKind string

const (
	GuardrailRuleBuiltin    GuardrailRuleKind = "builtin"
	GuardrailRuleExpression GuardrailRuleKind = "expression"
)

// GuardrailRule configures either a named builtin or an expression.
//
// Builtin and Params apply when Kind is builtin. Expression, OnFalse, and
// Reason apply when Kind is expression.
type GuardrailRule struct {
	Kind GuardrailRuleKind `json:"kind"`

	Builtin string         `json:"builtin,omitzero"`
	Params  map[string]any `json:"params,omitzero"`

	Expression string           `json:"expression,omitzero"`
	OnFalse    GuardrailVerdict `json:"on_false,omitzero"`
	Reason     string           `json:"reason,omitzero"`
}

// MarshalJSON implements [json.Marshaler].
func (r GuardrailRule) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case GuardrailRuleBuiltin:
		return json.Marshal(struct {
			Kind    GuardrailRuleKind `json:"kind"`
			Builtin string            `json:"builtin"`
			Params  map[string]any    `json:"params,omitzero"`
		}{Kind: r.Kind, Builtin: r.Builtin, Params: r.Params})
	case GuardrailRuleExpression:
		return json.Marshal(struct {
			Kind       GuardrailRuleKind `json:"kind"`
			Expression string            `json:"expression"`
			OnFalse    GuardrailVerdict  `json:"on_false"`
			Reason     string            `json:"reason,omitzero"`
		}{Kind: r.Kind, Expression: r.Expression, OnFalse: r.OnFalse, Reason: r.Reason})
	default:
		return json.Marshal(struct {
			Kind GuardrailRuleKind `json:"kind"`
		}{Kind: r.Kind})
	}
}

// UnmarshalJSON implements [json.Unmarshaler] and selects the declared rule shape.
func (r *GuardrailRule) UnmarshalJSON(data []byte) error {
	var discriminator struct {
		Kind GuardrailRuleKind `json:"kind"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return err
	}

	switch discriminator.Kind {
	case GuardrailRuleBuiltin:
		var decoded struct {
			Kind    GuardrailRuleKind `json:"kind"`
			Builtin string            `json:"builtin"`
			Params  map[string]any    `json:"params"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*r = GuardrailRule{Kind: decoded.Kind, Builtin: decoded.Builtin, Params: decoded.Params}
		return nil
	case GuardrailRuleExpression:
		var decoded struct {
			Kind       GuardrailRuleKind `json:"kind"`
			Expression string            `json:"expression"`
			OnFalse    GuardrailVerdict  `json:"on_false"`
			Reason     string            `json:"reason"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*r = GuardrailRule{
			Kind: decoded.Kind, Expression: decoded.Expression, OnFalse: decoded.OnFalse,
			Reason: decoded.Reason,
		}
		return nil
	default:
		var decoded struct {
			Kind GuardrailRuleKind `json:"kind"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*r = GuardrailRule{Kind: decoded.Kind}
		return nil
	}
}

// Guardrail is a policy applied to one or more agent-loop phases.
type Guardrail struct {
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
