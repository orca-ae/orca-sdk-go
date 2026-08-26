// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import util "github.com/apache/pulsar-client-go/pulsaradmin/pkg/utils"

type (
	ConsumerConfig             = util.ConsumerConfig
	ProducerConfig             = util.ProducerConfig
	Resources                  = util.Resources
	WindowConfig               = util.WindowConfig
	BatchSourceConfig          = util.BatchSourceConfig
	FunctionStatus             = util.FunctionStatus
	FunctionInstanceStatus     = util.FunctionInstanceStatus
	FunctionInstanceStatusData = util.FunctionInstanceStatusData
	SourceStatus               = util.SourceStatus
	SourceInstanceStatus       = util.SourceInstanceStatus
	SourceInstanceStatusData   = util.SourceInstanceStatusData
	SinkStatus                 = util.SinkStatus
	SinkInstanceStatus         = util.SinkInstanceStatus
	SinkInstanceStatusData     = util.SinkInstanceStatusData
)

// FunctionState preserves value presence so zero and empty values can be written.
type FunctionState struct {
	Key         string  `json:"key" yaml:"key"`
	StringValue *string `json:"stringValue,omitempty" yaml:"stringValue,omitempty"`
	ByteValue   *[]byte `json:"byteValue,omitempty" yaml:"byteValue,omitempty"`
	NumberValue *int64  `json:"numberValue,omitempty" yaml:"numberValue,omitempty"`
	Version     int64   `json:"version,omitempty" yaml:"version,omitempty"`
}

// FunctionInstanceStatsDataBase mirrors the Pulsar Function stats wire format.
type FunctionInstanceStatsDataBase struct {
	ReceivedTotal              int64   `json:"receivedTotal" yaml:"receivedTotal"`
	ProcessedSuccessfullyTotal int64   `json:"processedSuccessfullyTotal" yaml:"processedSuccessfullyTotal"`
	SystemExceptionsTotal      int64   `json:"systemExceptionsTotal" yaml:"systemExceptionsTotal"`
	UserExceptionsTotal        int64   `json:"userExceptionsTotal" yaml:"userExceptionsTotal"`
	AvgProcessLatency          float64 `json:"avgProcessLatency" yaml:"avgProcessLatency"`
}

// FunctionInstanceStatsData contains metrics for one function instance.
// Java FunctionInstanceStatsDataImpl serializes the one-minute window as "1min".
type FunctionInstanceStatsData struct {
	FunctionInstanceStatsDataBase `json:",inline" yaml:",inline"`
	OneMin                        FunctionInstanceStatsDataBase `json:"1min" yaml:"1min"`
	LastInvocation                int64                         `json:"lastInvocation" yaml:"lastInvocation"`
	UserMetrics                   map[string]float64            `json:"userMetrics,omitempty" yaml:"userMetrics,omitempty"`
}

// FunctionInstanceStats associates metrics with one instance ID.
type FunctionInstanceStats struct {
	InstanceID int                       `json:"instanceId" yaml:"instanceId"`
	Metrics    FunctionInstanceStatsData `json:"metrics" yaml:"metrics"`
}

// FunctionStats contains aggregate and per-instance function metrics.
type FunctionStats struct {
	FunctionInstanceStatsDataBase `json:",inline" yaml:",inline"`
	OneMin                        FunctionInstanceStatsDataBase `json:"1min" yaml:"1min"`
	LastInvocation                int64                         `json:"lastInvocation" yaml:"lastInvocation"`
	Instances                     []FunctionInstanceStats       `json:"instances,omitempty" yaml:"instances,omitempty"`
}

// UpdateOptionsImpl mirrors the update options payload accepted by registry create/update APIs.
type UpdateOptionsImpl struct {
	UpdateAuthData bool `json:"updateAuthData,omitempty" yaml:"updateAuthData,omitempty"`
}

// RegistryFunctionConfig is the workspace registry function schema.
type RegistryFunctionConfig struct {
	util.FunctionConfig `yaml:",inline"`
	Connection          string `json:"connection,omitempty" yaml:"connection,omitempty"`
	SNServiceAccount    string `json:"snServiceAccount,omitempty" yaml:"snServiceAccount,omitempty"`
}

// RegistrySourceConfig is the workspace registry source schema.
type RegistrySourceConfig struct {
	util.SourceConfig `yaml:",inline"`
	Connection        string `json:"connection,omitempty" yaml:"connection,omitempty"`
	LogTopic          string `json:"logTopic,omitempty" yaml:"logTopic,omitempty"`
	SNServiceAccount  string `json:"snServiceAccount,omitempty" yaml:"snServiceAccount,omitempty"`
	SourceType        string `json:"sourceType,omitempty" yaml:"sourceType,omitempty"`
}

// RegistrySinkConfig is the workspace registry sink schema.
type RegistrySinkConfig struct {
	util.SinkConfig  `yaml:",inline"`
	Connection       string `json:"connection,omitempty" yaml:"connection,omitempty"`
	LogTopic         string `json:"logTopic,omitempty" yaml:"logTopic,omitempty"`
	SNServiceAccount string `json:"snServiceAccount,omitempty" yaml:"snServiceAccount,omitempty"`
}
