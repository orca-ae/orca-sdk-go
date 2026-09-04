// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apijson"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// SessionService manages sessions.
type SessionService struct {
	client *Client

	// Events reads, sends, and streams a session's events.
	Events SessionEventService

	// Files manages files attached to a session.
	Files SessionFileService

	// Resources manages the files, repositories, and memory stores a session
	// can reach.
	Resources SessionResourceService

	// Threads reads the threads a session's coordinator spawned.
	Threads SessionThreadService
}

func newSessionService(client *Client) SessionService {
	return SessionService{
		client:    client,
		Events:    SessionEventService{client: client},
		Files:     SessionFileService{client: client},
		Resources: SessionResourceService{client: client},
		Threads: SessionThreadService{
			client: client,
			Events: SessionThreadEventService{client: client},
		},
	}
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

// SessionStatus is where a session is in its lifecycle.
type SessionStatus string

const (
	SessionRescheduling SessionStatus = "rescheduling"
	SessionRunning      SessionStatus = "running"
	SessionIdle         SessionStatus = "idle"
	SessionTerminated   SessionStatus = "terminated"
)

// SessionStats is how long a session has been running.
type SessionStats struct {
	ActiveSeconds   float64 `json:"active_seconds,omitzero"`
	DurationSeconds float64 `json:"duration_seconds,omitzero"`
}

// CacheCreationUsage breaks prompt-cache writes down by time to live.
type CacheCreationUsage struct {
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens,omitzero"`
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens,omitzero"`
}

// SessionUsage is the token usage a session has accumulated.
type SessionUsage struct {
	InputTokens          int64               `json:"input_tokens,omitzero"`
	OutputTokens         int64               `json:"output_tokens,omitzero"`
	CacheReadInputTokens int64               `json:"cache_read_input_tokens,omitzero"`
	CacheCreation        *CacheCreationUsage `json:"cache_creation,omitzero"`
}

// SessionTiming is when a session ran.
type SessionTiming struct {
	StartedAt       *string `json:"started_at"`
	LastActiveAt    *string `json:"last_active_at"`
	ActiveSeconds   float64 `json:"active_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// OutcomeEvaluation is one graded outcome of a session.
type OutcomeEvaluation struct {
	Type        string         `json:"type"`
	OutcomeID   string         `json:"outcome_id"`
	Description string         `json:"description"`
	Result      string         `json:"result"`
	Explanation *string        `json:"explanation"`
	Iteration   int64          `json:"iteration"`
	CompletedAt *string        `json:"completed_at"`
	Extra       map[string]any `json:"-"`
}

// UnmarshalJSON implements [json.Unmarshaler].
func (e *OutcomeEvaluation) UnmarshalJSON(data []byte) error {
	type shape OutcomeEvaluation
	return apijson.UnmarshalWithExtra(data, (*shape)(e),
		[]string{"type", "outcome_id", "description", "result", "explanation", "iteration", "completed_at"},
		&e.Extra)
}

// SessionAgentMember is one agent participating in a session.
type SessionAgentMember struct {
	ID       string            `json:"id,omitzero"`
	Type     string            `json:"type,omitzero"`
	Version  int64             `json:"version,omitzero"`
	Name     string            `json:"name,omitzero"`
	Model    *AgentModel       `json:"model,omitzero"`
	System   *string           `json:"system,omitzero"`
	Tools    []AgentTool       `json:"tools,omitzero"`
	Skills   []AgentSkill      `json:"skills,omitzero"`
	Metadata map[string]string `json:"metadata,omitzero"`
}

// SessionAgentMultiagent is the roster a coordinator session runs.
type SessionAgentMultiagent struct {
	Type   string               `json:"type"`
	Agents []SessionAgentMember `json:"agents"`
}

// SessionAgent is the agent snapshot taken when the session was created.
//
// It is a snapshot: updating the agent afterwards does not change a running
// session, which is what makes a session reproducible.
type SessionAgent struct {
	SessionAgentMember
	Multiagent *SessionAgentMultiagent `json:"multiagent"`
}

// Session is a running or completed agent session.
type Session struct {
	ID            string        `json:"id"`
	Type          string        `json:"type"`
	Agent         SessionAgent  `json:"agent"`
	EnvironmentID string        `json:"environment_id"`
	VaultIDs      []string      `json:"vault_ids"`
	Status        SessionStatus `json:"status"`

	// Title is present but nullable.
	Title *string `json:"title"`

	Stats  SessionStats   `json:"stats"`
	Timing *SessionTiming `json:"timing,omitzero"`
	Usage  SessionUsage   `json:"usage"`

	DeploymentID *string `json:"deployment_id,omitzero"`

	// OutcomeEvaluations and Resources are required arrays: the API sends an
	// empty array rather than omitting them.
	OutcomeEvaluations []OutcomeEvaluation `json:"outcome_evaluations"`
	Resources          []SessionResource   `json:"resources"`

	Metadata map[string]string `json:"metadata"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// ArchivedAt is nil while the session is active.
	ArchivedAt *string `json:"archived_at"`
}

// SessionDeleted is the tombstone a delete returns.
type SessionDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ---------------------------------------------------------------------------
// Agent selection
// ---------------------------------------------------------------------------

// SessionAgentRefType discriminates how a session names its agent.
type SessionAgentRefType string

const (
	// SessionAgentRef names an existing agent as it is.
	SessionAgentRef SessionAgentRefType = "agent"

	// SessionAgentRefWithOverrides names an agent and adjusts it for this
	// session only, without editing the stored agent.
	SessionAgentRefWithOverrides SessionAgentRefType = "agent_with_overrides"
)

// SessionAgentParam selects the agent a session runs.
//
// The API accepts a bare agent id as shorthand, so an entry carrying only an ID
// encodes to the string form. Setting Type promotes it to the object form,
// which is required whenever overrides are supplied.
type SessionAgentParam struct {
	// Type is required for the object form and must be omitted for the
	// shorthand.
	Type SessionAgentRefType `json:"type,omitzero"`

	ID      string           `json:"id,omitzero"`
	Version param.Opt[int64] `json:"version,omitzero"`

	// Overrides, valid only with SessionAgentRefWithOverrides. They are
	// forwarded verbatim; nothing here rewrites them per backend.
	System     param.Opt[string]     `json:"system,omitzero"`
	Model      AgentModelParam       `json:"model,omitzero"`
	Tools      []AgentTool           `json:"tools,omitzero"`
	McpServers []AgentMcpServerParam `json:"mcp_servers,omitzero"`
	Skills     []AgentSkillParam     `json:"skills,omitzero"`

	// GuardrailIDs applies policy guardrails only to this session's agent
	// snapshot. A non-nil empty slice is still an explicit override.
	GuardrailIDs []string `json:"guardrail_ids,omitzero"`

	Extra map[string]any `json:"-"`
}

// AgentRef returns a session agent parameter naming an agent by id, which is
// sent as the string shorthand.
func AgentRef(id string) SessionAgentParam { return SessionAgentParam{ID: id} }

// IsZero reports whether no agent was selected.
func (p SessionAgentParam) IsZero() bool {
	return p.Type == "" && p.ID == "" && !p.Version.Valid() && !p.System.Valid() &&
		p.Model.IsZero() && p.Tools == nil && p.McpServers == nil && p.Skills == nil &&
		p.GuardrailIDs == nil && p.Extra == nil
}

// MarshalJSON implements [json.Marshaler].
func (p SessionAgentParam) MarshalJSON() ([]byte, error) {
	if p.Type == "" && p.ID != "" && !p.Version.Valid() && !p.System.Valid() &&
		p.Model.IsZero() && p.Tools == nil && p.McpServers == nil && p.Skills == nil &&
		p.GuardrailIDs == nil && p.Extra == nil {
		return json.Marshal(p.ID)
	}
	type shape SessionAgentParam
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// SessionAgentOverridesParam adjusts a running session's agent.
//
// Update accepts overrides only. An environment, a resource list, or a
// different agent reference cannot be changed after creation, and the request
// type deliberately cannot express them.
type SessionAgentOverridesParam struct {
	System     param.Opt[string]     `json:"system,omitzero"`
	Model      AgentModelParam       `json:"model,omitzero"`
	Tools      []AgentTool           `json:"tools,omitzero"`
	McpServers []AgentMcpServerParam `json:"mcp_servers,omitzero"`
	Skills     []AgentSkillParam     `json:"skills,omitzero"`
	Extra      map[string]any        `json:"-"`
}

// IsZero reports whether no override was set.
func (p SessionAgentOverridesParam) IsZero() bool {
	return !p.System.Valid() && p.Model.IsZero() && p.Tools == nil &&
		p.McpServers == nil && p.Skills == nil && p.Extra == nil
}

// MarshalJSON implements [json.Marshaler].
func (p SessionAgentOverridesParam) MarshalJSON() ([]byte, error) {
	type shape SessionAgentOverridesParam
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// SessionNewParams creates a session.
//
// A session is created at the /v1/sessions collection, never under an agent.
type SessionNewParams struct {
	// Agent selects the agent to run. Use either this or AgentID.
	Agent SessionAgentParam `json:"agent,omitzero"`

	// AgentID is the compatibility form some deployments accept. It is sent
	// exactly as given, and setting it never synthesizes an Agent field.
	AgentID param.Opt[string] `json:"agent_id,omitzero"`

	EnvironmentID string `json:"environment_id"`

	VaultIDs  []string               `json:"vault_ids,omitzero"`
	Title     param.Opt[string]      `json:"title,omitzero"`
	Metadata  map[string]string      `json:"metadata,omitzero"`
	Resources []SessionResourceParam `json:"resources,omitzero"`

	// InitialEvents seeds the session, so it starts with a prompt already in
	// place rather than needing a follow-up send.
	InitialEvents []SessionEventParam `json:"initial_events,omitzero"`
}

// SessionUpdateParams updates a session's mutable fields.
type SessionUpdateParams struct {
	Title param.Opt[string] `json:"title,omitzero"`

	// Agent carries overrides only - see [SessionAgentOverridesParam].
	Agent SessionAgentOverridesParam `json:"agent,omitzero"`

	VaultIDs []string `json:"vault_ids,omitzero"`

	// Metadata patches individual keys. A nil value removes its key; a null
	// Metadata clears the whole map.
	Metadata param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// SessionListParams filters and pages a list of sessions.
//
// The deployment overlay removes the agent_version, created_at, deployment_id,
// memory_store_id, order, statuses and provider filters, so they are absent
// here rather than sent and ignored.
type SessionListParams struct {
	AgentID         param.Opt[string]
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// ---------------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------------

// Create creates a session.
func (s SessionService) Create(ctx context.Context, params SessionNewParams, opts ...option.RequestOption) (*Session, error) {
	if params.Agent.Type == SessionAgentRefWithOverrides && params.Agent.GuardrailIDs != nil {
		if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
			return nil, err
		}
	}
	var session Session
	if err := s.client.PostJSON(ctx, "v1/sessions", params, &session, opts...); err != nil {
		return nil, err
	}
	return &session, nil
}

// Get reads a session.
func (s SessionService) Get(ctx context.Context, sessionID string, opts ...option.RequestOption) (*Session, error) {
	var session Session
	if err := s.client.GetJSON(ctx, "v1/sessions/"+url.PathEscape(sessionID), &session, opts...); err != nil {
		return nil, err
	}
	return &session, nil
}

// Update updates a session's metadata and agent overrides. The verb is POST.
func (s SessionService) Update(ctx context.Context, sessionID string, params SessionUpdateParams, opts ...option.RequestOption) (*Session, error) {
	var session Session
	if err := s.client.PostJSON(ctx, "v1/sessions/"+url.PathEscape(sessionID), params, &session, opts...); err != nil {
		return nil, err
	}
	return &session, nil
}

// List returns a page of sessions.
func (s SessionService) List(ctx context.Context, params SessionListParams, opts ...option.RequestOption) (*pagination.PageCursor[Session], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendStringQuery(opts, "agent_id", params.AgentID)
	opts = appendBoolQuery(opts, "include_archived", params.IncludeArchived)
	return ListPage[Session](ctx, s.client, "v1/sessions", opts...)
}

// Delete permanently deletes a session and returns its tombstone.
func (s SessionService) Delete(ctx context.Context, sessionID string, opts ...option.RequestOption) (*SessionDeleted, error) {
	var deleted SessionDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/sessions/"+url.PathEscape(sessionID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Outcome returns the session's graded outcome.
//
// The result is nil when the session has no outcome yet: the contract marks the
// whole response nullable, and a session that has not been graded is a normal
// state rather than a failure. Callers must check for nil before reading it.
func (s SessionService) Outcome(ctx context.Context, sessionID string, opts ...option.RequestOption) (*OutcomeEvaluation, error) {
	// Decoded through a pointer so a null body stays distinguishable from an
	// outcome whose every field happens to be empty.
	var outcome *OutcomeEvaluation
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/outcome"
	if err := s.client.GetJSON(ctx, path, &outcome, opts...); err != nil {
		return nil, err
	}
	return outcome, nil
}

// Archive archives a session and returns it.
func (s SessionService) Archive(ctx context.Context, sessionID string, opts ...option.RequestOption) (*Session, error) {
	var session Session
	if err := s.client.PostJSON(ctx, "v1/sessions/"+url.PathEscape(sessionID)+"/archive", nil, &session, opts...); err != nil {
		return nil, err
	}
	return &session, nil
}
