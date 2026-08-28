// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apijson"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// TriggerService manages triggers: the rules that start sessions on their own.
//
// Triggers are core, served under /v1/triggers. The deployment overlay promoted
// them out of the cloud extension group, so there is deliberately no
// Cloud.Triggers - mounting them there would put them behind the extension gate
// and on a path the core engine does not serve.
type TriggerService struct {
	client *Client

	// Sessions lists the sessions a trigger created.
	Sessions TriggerSessionService
}

func newTriggerService(client *Client) TriggerService {
	return TriggerService{client: client, Sessions: TriggerSessionService{client: client}}
}

// TriggerSessionMode is how a trigger maps incoming events onto sessions.
type TriggerSessionMode string

const (
	// TriggerSessionPerEvent starts a session for every event.
	TriggerSessionPerEvent TriggerSessionMode = "SESSION_PER_EVENT"

	// TriggerSessionPerTopic keeps one session per source topic. A cron source
	// does not support it - there is no topic to key on.
	TriggerSessionPerTopic TriggerSessionMode = "SESSION_PER_TOPIC"

	// TriggerSessionPerKey keeps one session per message key, which is what
	// preserves per-entity ordering.
	TriggerSessionPerKey TriggerSessionMode = "SESSION_PER_KEY"

	// TriggerSessionShared routes every event into one session.
	TriggerSessionShared TriggerSessionMode = "SHARED"
)

// TriggerSourceType discriminates what a trigger listens to.
type TriggerSourceType string

const (
	TriggerSourceCron   TriggerSourceType = "cron"
	TriggerSourceKafka  TriggerSourceType = "kafka"
	TriggerSourcePulsar TriggerSourceType = "pulsar"
)

// TriggerSchemaConfig describes how to decode one part of a message.
type TriggerSchemaConfig struct {
	Subject string           `json:"subject,omitzero"`
	Type    string           `json:"type,omitzero"`
	Version param.Opt[int64] `json:"version,omitzero"`

	Extra map[string]any `json:"-"`
}

// MarshalJSON implements [json.Marshaler].
func (c TriggerSchemaConfig) MarshalJSON() ([]byte, error) {
	type shape TriggerSchemaConfig
	return apijson.MarshalWithExtra(shape(c), c.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (c *TriggerSchemaConfig) UnmarshalJSON(data []byte) error {
	type shape TriggerSchemaConfig
	return apijson.UnmarshalWithExtra(data, (*shape)(c), []string{"subject", "type", "version"}, &c.Extra)
}

// TriggerSource is what a trigger listens to.
//
// One struct carries all three shapes. A messaging source takes exactly one of
// Topics or TopicPattern: supplying both is ambiguous, and supplying neither
// leaves the trigger with nothing to consume.
type TriggerSource struct {
	Type TriggerSourceType `json:"type"`

	// Cron.
	Schedule param.Opt[string] `json:"schedule,omitzero"`
	Timezone param.Opt[string] `json:"timezone,omitzero"`

	// Kafka and Pulsar.
	Connection       param.Opt[string] `json:"connection,omitzero"`
	Topics           []string          `json:"topics,omitzero"`
	TopicPattern     param.Opt[string] `json:"topic_pattern,omitzero"`
	SubscriptionName param.Opt[string] `json:"subscription_name,omitzero"`

	ConsumerAdditionalConfig map[string]string              `json:"consumer_additional_config,omitzero"`
	InputSchemaConfigs       map[string]TriggerSchemaConfig `json:"input_schema_configs,omitzero"`

	Extra map[string]any `json:"-"`
}

// IsZero reports whether no source was supplied.
func (s TriggerSource) IsZero() bool { return s.Type == "" }

// MarshalJSON implements [json.Marshaler].
func (s TriggerSource) MarshalJSON() ([]byte, error) {
	type shape TriggerSource
	return apijson.MarshalWithExtra(shape(s), s.Extra)
}

// UnmarshalJSON implements [json.Unmarshaler].
func (s *TriggerSource) UnmarshalJSON(data []byte) error {
	type shape TriggerSource
	return apijson.UnmarshalWithExtra(data, (*shape)(s), []string{
		"type", "schedule", "timezone", "connection", "topics", "topic_pattern",
		"subscription_name", "consumer_additional_config", "input_schema_configs",
	}, &s.Extra)
}

// TriggerSessionTemplate describes the sessions a trigger creates.
type TriggerSessionTemplate struct {
	EnvironmentID param.Opt[string]             `json:"environment_id,omitzero"`
	VaultIDs      []string                      `json:"vault_ids,omitzero"`
	Metadata      param.Opt[map[string]*string] `json:"metadata,omitzero"`

	Extra map[string]any `json:"-"`
}

// IsZero reports whether no session template was supplied.
func (t TriggerSessionTemplate) IsZero() bool {
	return !t.EnvironmentID.Valid() && t.VaultIDs == nil && t.Metadata.IsZero() && t.Extra == nil
}

// MarshalJSON implements [json.Marshaler].
func (t TriggerSessionTemplate) MarshalJSON() ([]byte, error) {
	type shape TriggerSessionTemplate
	return apijson.MarshalWithExtra(shape(t), t.Extra)
}

// Trigger starts sessions on its own, on a schedule or from a message stream.
type Trigger struct {
	ID          string             `json:"id"`
	Type        string             `json:"type,omitzero"`
	Name        string             `json:"name,omitzero"`
	Status      string             `json:"status,omitzero"`
	SessionMode TriggerSessionMode `json:"session_mode,omitzero"`
	Source      TriggerSource      `json:"source,omitzero"`
	Replicas    int64              `json:"replicas,omitzero"`
	CreatedAt   string             `json:"created_at,omitzero"`
	UpdatedAt   string             `json:"updated_at,omitzero"`
}

// TriggerDeleted is the tombstone a delete returns.
type TriggerDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// TriggerNewParams creates a trigger.
//
// The managed-deployment messaging fields go to the core path unchanged: a
// backend implementing only the narrower cron subset returns its own error for
// a combination it does not support, rather than this SDK guessing which
// backend it is talking to.
type TriggerNewParams struct {
	Name        string                 `json:"name"`
	Agent       SessionAgentParam      `json:"agent,omitzero"`
	SessionMode TriggerSessionMode     `json:"session_mode,omitzero"`
	Source      TriggerSource          `json:"source,omitzero"`
	Session     TriggerSessionTemplate `json:"session,omitzero"`
	Replicas    param.Opt[int64]       `json:"replicas,omitzero"`
}

// TriggerUpdateParams updates a trigger.
type TriggerUpdateParams struct {
	Name        param.Opt[string]      `json:"name,omitzero"`
	SessionMode TriggerSessionMode     `json:"session_mode,omitzero"`
	Source      TriggerSource          `json:"source,omitzero"`
	Session     TriggerSessionTemplate `json:"session,omitzero"`
	Replicas    param.Opt[int64]       `json:"replicas,omitzero"`
}

// TriggerListParams filters and pages triggers.
//
// The overlay removes include_archived from the trigger list params, so it is
// absent here rather than sent and ignored.
type TriggerListParams struct {
	AgentID param.Opt[string]
	Limit   param.Opt[int64]
	Page    param.Opt[string]
}

// Create creates a trigger.
func (s TriggerService) Create(ctx context.Context, params TriggerNewParams, opts ...option.RequestOption) (*Trigger, error) {
	var trigger Trigger
	if err := s.client.PostJSON(ctx, "v1/triggers", params, &trigger, opts...); err != nil {
		return nil, err
	}
	return &trigger, nil
}

// List returns a page of triggers.
func (s TriggerService) List(ctx context.Context, params TriggerListParams, opts ...option.RequestOption) (*pagination.PageCursor[Trigger], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	opts = appendStringQuery(opts, "agent_id", params.AgentID)
	return ListPage[Trigger](ctx, s.client, "v1/triggers", opts...)
}

// Get reads a trigger.
func (s TriggerService) Get(ctx context.Context, triggerID string, opts ...option.RequestOption) (*Trigger, error) {
	var trigger Trigger
	if err := s.client.GetJSON(ctx, "v1/triggers/"+url.PathEscape(triggerID), &trigger, opts...); err != nil {
		return nil, err
	}
	return &trigger, nil
}

// Update updates a trigger. The verb is POST.
func (s TriggerService) Update(ctx context.Context, triggerID string, params TriggerUpdateParams, opts ...option.RequestOption) (*Trigger, error) {
	var trigger Trigger
	if err := s.client.PostJSON(ctx, "v1/triggers/"+url.PathEscape(triggerID), params, &trigger, opts...); err != nil {
		return nil, err
	}
	return &trigger, nil
}

// Delete permanently deletes a trigger and returns its tombstone.
func (s TriggerService) Delete(ctx context.Context, triggerID string, opts ...option.RequestOption) (*TriggerDeleted, error) {
	var deleted TriggerDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/triggers/"+url.PathEscape(triggerID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}

// Pause stops a trigger from starting new sessions and returns it.
func (s TriggerService) Pause(ctx context.Context, triggerID string, opts ...option.RequestOption) (*Trigger, error) {
	var trigger Trigger
	path := "v1/triggers/" + url.PathEscape(triggerID) + "/pause"
	if err := s.client.PostJSON(ctx, path, nil, &trigger, opts...); err != nil {
		return nil, err
	}
	return &trigger, nil
}

// Unpause resumes a paused trigger and returns it.
func (s TriggerService) Unpause(ctx context.Context, triggerID string, opts ...option.RequestOption) (*Trigger, error) {
	var trigger Trigger
	path := "v1/triggers/" + url.PathEscape(triggerID) + "/unpause"
	if err := s.client.PostJSON(ctx, path, nil, &trigger, opts...); err != nil {
		return nil, err
	}
	return &trigger, nil
}
