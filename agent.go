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

// AgentService manages agents.
type AgentService struct {
	client *Client

	// Versions reads the historical snapshots of an agent. A version is created
	// as a side effect of updating, never directly.
	Versions AgentVersionService
}

func newAgentService(client *Client) AgentService {
	return AgentService{client: client, Versions: AgentVersionService{client: client}}
}

// ---------------------------------------------------------------------------
// Model selection
// ---------------------------------------------------------------------------

// ModelEffortType is how much reasoning effort a model should spend.
type ModelEffortType string

const (
	ModelEffortLow    ModelEffortType = "low"
	ModelEffortMedium ModelEffortType = "medium"
	ModelEffortHigh   ModelEffortType = "high"
	ModelEffortXHigh  ModelEffortType = "xhigh"
	ModelEffortMax    ModelEffortType = "max"
)

// ModelSpeed selects between a model's standard and fast variants.
type ModelSpeed string

const (
	ModelSpeedStandard ModelSpeed = "standard"
	ModelSpeedFast     ModelSpeed = "fast"
)

// ModelEffort wraps an effort level, which the API sends as an object rather
// than a bare string so it can gain fields without a breaking change.
type ModelEffort struct {
	Type ModelEffortType `json:"type"`
}

// AgentModelParam selects the model an agent runs on.
//
// The API accepts either a bare model id or an object, and this sends whichever
// the caller's values imply: an ID on its own is written as the string
// shorthand, and setting any of the other fields promotes it to the object
// form. Modelling that as one type rather than a union keeps the common case a
// single assignment.
type AgentModelParam struct {
	// ID is the model identifier, e.g. "claude-sonnet-4-6".
	ID string

	// Provider qualifies the model when the deployment serves more than one.
	Provider param.Opt[string]

	// Speed selects a model variant.
	Speed param.Opt[ModelSpeed]

	// Effort sets how much reasoning effort to spend.
	Effort param.Opt[ModelEffort]
}

// Model returns a model parameter carrying only an id, which is sent as the
// string shorthand.
func Model(id string) AgentModelParam { return AgentModelParam{ID: id} }

// IsZero reports whether no model was selected, so an omitzero-tagged field
// leaves the key out entirely.
func (m AgentModelParam) IsZero() bool {
	return m.ID == "" && !m.Provider.Valid() && !m.Speed.Valid() && !m.Effort.Valid()
}

// MarshalJSON implements [json.Marshaler].
func (m AgentModelParam) MarshalJSON() ([]byte, error) {
	if !m.Provider.Valid() && !m.Speed.Valid() && !m.Effort.Valid() {
		return json.Marshal(m.ID)
	}
	type object struct {
		ID       string                 `json:"id"`
		Provider param.Opt[string]      `json:"provider,omitzero"`
		Speed    param.Opt[ModelSpeed]  `json:"speed,omitzero"`
		Effort   param.Opt[ModelEffort] `json:"effort,omitzero"`
	}
	return json.Marshal(object{ID: m.ID, Provider: m.Provider, Speed: m.Speed, Effort: m.Effort})
}

// AgentModel is the model an agent runs on, as the API reports it.
type AgentModel struct {
	ID       string       `json:"id"`
	Provider string       `json:"provider,omitzero"`
	Speed    ModelSpeed   `json:"speed,omitzero"`
	Effort   *ModelEffort `json:"effort,omitzero"`
}

// ---------------------------------------------------------------------------
// MCP servers, tools and skills
// ---------------------------------------------------------------------------

// McpServerDefinition is an MCP server an agent can reach, as reported by the
// API.
type McpServerDefinition struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// AgentMcpServerParam declares an MCP server on a create or update request.
//
// Type is optional and must not be synthesised when the caller omits it: the
// contract makes the discriminator optional here, and sending one the caller
// did not ask for changes the request.
type AgentMcpServerParam struct {
	Name  string            `json:"name"`
	URL   string            `json:"url"`
	Type  param.Opt[string] `json:"type,omitzero"`
	Extra map[string]any    `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (p AgentMcpServerParam) MarshalJSON() ([]byte, error) {
	type shape AgentMcpServerParam
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// AgentPermissionPolicyType is how a tool call is authorised.
type AgentPermissionPolicyType string

const (
	AgentPermissionAlwaysAllow AgentPermissionPolicyType = "always_allow"
	AgentPermissionAlwaysAsk   AgentPermissionPolicyType = "always_ask"
)

// AgentPermissionPolicy authorises tool use. Deployments add their own keys to
// it, which travel in Extra.
type AgentPermissionPolicy struct {
	Type  AgentPermissionPolicyType `json:"type"`
	Extra map[string]any            `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (p AgentPermissionPolicy) MarshalJSON() ([]byte, error) {
	type shape AgentPermissionPolicy
	return apijson.MarshalWithExtra(shape(p), p.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (p *AgentPermissionPolicy) UnmarshalJSON(data []byte) error {
	type shape AgentPermissionPolicy
	return apijson.UnmarshalWithExtra(data, (*shape)(p), []string{"type"}, &p.Extra)
}

// AgentToolConfig configures one tool within a toolset.
type AgentToolConfig struct {
	Enabled          param.Opt[bool]                  `json:"enabled,omitzero"`
	PermissionPolicy param.Opt[AgentPermissionPolicy] `json:"permission_policy,omitzero"`
	Extra            map[string]any                   `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (c AgentToolConfig) MarshalJSON() ([]byte, error) {
	type shape AgentToolConfig
	return apijson.MarshalWithExtra(shape(c), c.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (c *AgentToolConfig) UnmarshalJSON(data []byte) error {
	type shape AgentToolConfig
	return apijson.UnmarshalWithExtra(data, (*shape)(c), []string{"enabled", "permission_policy"}, &c.Extra)
}

// AgentToolType discriminates a tool definition.
type AgentToolType string

const (
	AgentToolBuiltinToolset AgentToolType = "agent_toolset"
	AgentToolMcpToolset     AgentToolType = "mcp_toolset"
	AgentToolCustom         AgentToolType = "custom"
)

// AgentCustomToolInputSchema is the JSON Schema for a custom tool's input.
type AgentCustomToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitzero"`
	Required   []string       `json:"required,omitzero"`
	Extra      map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (s AgentCustomToolInputSchema) MarshalJSON() ([]byte, error) {
	type shape AgentCustomToolInputSchema
	return apijson.MarshalWithExtra(shape(s), s.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (s *AgentCustomToolInputSchema) UnmarshalJSON(data []byte) error {
	type shape AgentCustomToolInputSchema
	return apijson.UnmarshalWithExtra(data, (*shape)(s), []string{"type", "properties", "required"}, &s.Extra)
}

// AgentTool is one entry in an agent's tool list.
//
// The API discriminates three shapes on `type`, and they share enough fields
// that one struct carries all of them: a builtin toolset, an MCP toolset, and a
// single custom tool. Which fields are meaningful follows from Type.
type AgentTool struct {
	// Type selects the shape. Required.
	Type AgentToolType `json:"type"`

	// MCPServerName names the server, for an mcp_toolset.
	MCPServerName string `json:"mcp_server_name,omitzero"`

	// Configs enables and configures individual tools within a toolset. The API
	// accepts either a name-keyed object or a list of entries carrying their own
	// name; both decode into this map.
	Configs map[string]AgentToolConfig `json:"configs,omitzero"`

	// DefaultConfig applies to tools in the set with no entry in Configs.
	DefaultConfig param.Opt[AgentToolConfig] `json:"default_config,omitzero"`

	// Name, Description and InputSchema describe a custom tool, and are
	// required for one.
	Name        string                      `json:"name,omitzero"`
	Description string                      `json:"description,omitzero"`
	InputSchema *AgentCustomToolInputSchema `json:"input_schema,omitzero"`

	// Extra carries provider-specific keys the contract does not enumerate.
	Extra map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (t AgentTool) MarshalJSON() ([]byte, error) {
	type shape AgentTool
	return apijson.MarshalWithExtra(shape(t), t.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
//
// Configs arrives either as an object keyed by tool name or as a list of
// entries each carrying a name. Both are normalised to the map form so callers
// have one thing to read.
func (t *AgentTool) UnmarshalJSON(data []byte) error {
	type shape struct {
		Type          AgentToolType               `json:"type"`
		MCPServerName string                      `json:"mcp_server_name"`
		Configs       json.RawMessage             `json:"configs"`
		DefaultConfig param.Opt[AgentToolConfig]  `json:"default_config"`
		Name          string                      `json:"name"`
		Description   string                      `json:"description"`
		InputSchema   *AgentCustomToolInputSchema `json:"input_schema"`
	}
	var decoded shape
	if err := apijson.UnmarshalWithExtra(data, &decoded,
		[]string{"type", "mcp_server_name", "configs", "default_config", "name", "description", "input_schema"},
		&t.Extra); err != nil {
		return err
	}

	t.Type = decoded.Type
	t.MCPServerName = decoded.MCPServerName
	t.DefaultConfig = decoded.DefaultConfig
	t.Name = decoded.Name
	t.Description = decoded.Description
	t.InputSchema = decoded.InputSchema

	if len(decoded.Configs) == 0 {
		return nil
	}
	if apijson.IsJSONArray(decoded.Configs) {
		var entries []struct {
			Name string `json:"name"`
			AgentToolConfig
		}
		if err := json.Unmarshal(decoded.Configs, &entries); err != nil {
			return err
		}
		t.Configs = make(map[string]AgentToolConfig, len(entries))
		for _, entry := range entries {
			t.Configs[entry.Name] = entry.AgentToolConfig
		}
		return nil
	}
	return json.Unmarshal(decoded.Configs, &t.Configs)
}

// AgentSkillType discriminates a skill reference.
type AgentSkillType string

const (
	AgentSkillProvider AgentSkillType = "anthropic"
	AgentSkillCustom   AgentSkillType = "custom"
)

// AgentSkillParam attaches a skill to an agent.
type AgentSkillParam struct {
	Type    AgentSkillType    `json:"type"`
	SkillID string            `json:"skill_id"`
	Version param.Opt[string] `json:"version,omitzero"`
	Extra   map[string]any    `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (s AgentSkillParam) MarshalJSON() ([]byte, error) {
	type shape AgentSkillParam
	return apijson.MarshalWithExtra(shape(s), s.Extra)
}

// AgentSkill is a skill attached to an agent, as the API reports it.
type AgentSkill struct {
	Type    AgentSkillType `json:"type"`
	SkillID string         `json:"skill_id"`
	Version string         `json:"version"`
}

// ---------------------------------------------------------------------------
// Multi-agent coordination
// ---------------------------------------------------------------------------

// AgentRosterEntryType discriminates a roster entry.
type AgentRosterEntryType string

const (
	AgentRosterEntryAgent AgentRosterEntryType = "agent"
	AgentRosterEntrySelf  AgentRosterEntryType = "self"
)

// AgentRosterEntry names one member of a coordinator's roster.
//
// The API accepts a bare agent id string as shorthand for an agent entry, so
// that is what an entry carrying only an ID encodes to.
type AgentRosterEntry struct {
	Type    AgentRosterEntryType `json:"type"`
	ID      string               `json:"id,omitzero"`
	Version param.Opt[int64]     `json:"version,omitzero"`
}

// RosterAgent returns a roster entry naming another agent by id.
func RosterAgent(id string) AgentRosterEntry {
	return AgentRosterEntry{Type: AgentRosterEntryAgent, ID: id}
}

// RosterSelf returns the roster entry meaning the coordinator itself.
func RosterSelf() AgentRosterEntry { return AgentRosterEntry{Type: AgentRosterEntrySelf} }

// MarshalJSON implements [json.Marshaler].
func (e AgentRosterEntry) MarshalJSON() ([]byte, error) {
	if e.Type == "" && e.ID != "" {
		return json.Marshal(e.ID)
	}
	type shape AgentRosterEntry
	return json.Marshal(shape(e))
}

// UnmarshalJSON implements [json.Unmarshaler].
func (e *AgentRosterEntry) UnmarshalJSON(data []byte) error {
	if apijson.IsJSONString(data) {
		var id string
		if err := json.Unmarshal(data, &id); err != nil {
			return err
		}
		e.Type, e.ID = AgentRosterEntryAgent, id
		return nil
	}
	type shape AgentRosterEntry
	return json.Unmarshal(data, (*shape)(e))
}

// AgentMultiagent makes an agent a coordinator over a roster of others.
type AgentMultiagent struct {
	Type   string             `json:"type"`
	Agents []AgentRosterEntry `json:"agents"`
}

// Coordinator returns a multi-agent definition over the given roster.
func Coordinator(agents ...AgentRosterEntry) AgentMultiagent {
	return AgentMultiagent{Type: "coordinator", Agents: agents}
}

// ---------------------------------------------------------------------------
// The agent itself
// ---------------------------------------------------------------------------

// Agent is a configured agent.
type Agent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`

	// Description and System are present but nullable on the wire.
	Description *string `json:"description"`
	System      *string `json:"system"`

	Model AgentModel `json:"model"`

	// McpServers, Tools and Skills are required arrays: the API sends an empty
	// array rather than omitting them, so a nil here means the field was
	// genuinely absent.
	McpServers []McpServerDefinition `json:"mcp_servers"`
	Tools      []AgentTool           `json:"tools"`
	Skills     []AgentSkill          `json:"skills"`

	// GuardrailIDs is present on policy-extension responses requested with the
	// managed-agents beta header.
	GuardrailIDs []string `json:"guardrail_ids,omitzero"`

	// Multiagent is nullable: most agents are not coordinators.
	Multiagent *AgentMultiagent `json:"multiagent"`

	Metadata map[string]string `json:"metadata"`
	Version  int64             `json:"version"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// ArchivedAt is nil while the agent is active.
	ArchivedAt *string `json:"archived_at"`
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// AgentNewParams creates an agent.
type AgentNewParams struct {
	Model AgentModelParam `json:"model,omitzero"`
	Name  string          `json:"name"`

	Description param.Opt[string]     `json:"description,omitzero"`
	System      param.Opt[string]     `json:"system,omitzero"`
	McpServers  []AgentMcpServerParam `json:"mcp_servers,omitzero"`
	Tools       []AgentTool           `json:"tools,omitzero"`
	Skills      []AgentSkillParam     `json:"skills,omitzero"`
	// GuardrailIDs explicitly attaches policy guardrails. A non-nil empty slice
	// is sent as [] and still requires the policy extension and beta header.
	GuardrailIDs []string                   `json:"guardrail_ids,omitzero"`
	Metadata     map[string]string          `json:"metadata,omitzero"`
	Multiagent   param.Opt[AgentMultiagent] `json:"multiagent,omitzero"`
}

// AgentUpdateParams partially updates an agent.
//
// Every field is optional, and the difference between omitting one and sending
// it as null is load bearing: omitting leaves the field alone, null clears it.
// A metadata value of null removes that single key rather than replacing the
// whole map.
type AgentUpdateParams struct {
	// Version applies optimistic concurrency when set. A versionless update
	// must not synthesize one, or it would fail against any agent that has
	// been touched since.
	Version param.Opt[int64] `json:"version,omitzero"`

	Name        param.Opt[string]                `json:"name,omitzero"`
	Description param.Opt[string]                `json:"description,omitzero"`
	Model       AgentModelParam                  `json:"model,omitzero"`
	System      param.Opt[string]                `json:"system,omitzero"`
	McpServers  param.Opt[[]AgentMcpServerParam] `json:"mcp_servers,omitzero"`
	Tools       param.Opt[[]AgentTool]           `json:"tools,omitzero"`
	Skills      param.Opt[[]AgentSkillParam]     `json:"skills,omitzero"`
	// GuardrailIDs replaces attached guardrails. Use param.Null[[]string]() to
	// clear them; any non-absent value requires the policy extension and beta header.
	GuardrailIDs param.Opt[[]string]        `json:"guardrail_ids,omitzero"`
	Multiagent   param.Opt[AgentMultiagent] `json:"multiagent,omitzero"`

	// Metadata patches individual keys. A nil value removes its key.
	Metadata param.Opt[map[string]*string] `json:"metadata,omitzero"`
}

// AgentGetParams selects which snapshot of an agent to read.
type AgentGetParams struct {
	// Version reads a specific historical snapshot instead of the current one.
	Version param.Opt[int64]
}

// AgentListParams filters and pages a list of agents.
type AgentListParams struct {
	Limit           param.Opt[int64]
	Page            param.Opt[string]
	IncludeArchived param.Opt[bool]
}

// ---------------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------------

// Create creates an agent.
func (s AgentService) Create(ctx context.Context, params AgentNewParams, opts ...option.RequestOption) (*Agent, error) {
	if params.GuardrailIDs != nil {
		if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
			return nil, err
		}
	}
	var agent Agent
	if err := s.client.PostJSON(ctx, "v1/agents", params, &agent, opts...); err != nil {
		return nil, err
	}
	return &agent, nil
}

// Get reads an agent, optionally at a specific version.
func (s AgentService) Get(ctx context.Context, agentID string, params AgentGetParams, opts ...option.RequestOption) (*Agent, error) {
	if version, ok := params.Version.Value(); ok {
		opts = append(opts, option.WithQuery("version", formatInt(version)))
	}
	var agent Agent
	if err := s.client.GetJSON(ctx, "v1/agents/"+url.PathEscape(agentID), &agent, opts...); err != nil {
		return nil, err
	}
	return &agent, nil
}

// Update partially updates an agent.
//
// The verb is POST, not PUT: the contract defines a partial update here, and a
// PUT would imply the body replaces the whole resource.
func (s AgentService) Update(ctx context.Context, agentID string, params AgentUpdateParams, opts ...option.RequestOption) (*Agent, error) {
	if !params.GuardrailIDs.IsZero() {
		if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
			return nil, err
		}
	}
	var agent Agent
	if err := s.client.PostJSON(ctx, "v1/agents/"+url.PathEscape(agentID), params, &agent, opts...); err != nil {
		return nil, err
	}
	return &agent, nil
}

// List returns a page of agents.
func (s AgentService) List(ctx context.Context, params AgentListParams, opts ...option.RequestOption) (*pagination.PageCursor[Agent], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	if includeArchived, ok := params.IncludeArchived.Value(); ok {
		opts = append(opts, option.WithQuery("include_archived", formatBool(includeArchived)))
	}
	return ListPage[Agent](ctx, s.client, "v1/agents", opts...)
}

// Archive archives an agent and returns it.
//
// There is no delete: the deployment overlay removes it, so archiving is how an
// agent is retired.
func (s AgentService) Archive(ctx context.Context, agentID string, opts ...option.RequestOption) (*Agent, error) {
	var agent Agent
	if err := s.client.PostJSON(ctx, "v1/agents/"+url.PathEscape(agentID)+"/archive", nil, &agent, opts...); err != nil {
		return nil, err
	}
	return &agent, nil
}

// ---------------------------------------------------------------------------
// Versions
// ---------------------------------------------------------------------------

// AgentVersionService reads an agent's historical snapshots.
type AgentVersionService struct {
	client *Client
}

// AgentVersionListParams pages an agent's version history.
type AgentVersionListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// List returns a page of an agent's historical versions. Each item is a full
// agent whose Version is the snapshot number.
func (s AgentVersionService) List(ctx context.Context, agentID string, params AgentVersionListParams, opts ...option.RequestOption) (*pagination.PageCursor[Agent], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[Agent](ctx, s.client, "v1/agents/"+url.PathEscape(agentID)+"/versions", opts...)
}
