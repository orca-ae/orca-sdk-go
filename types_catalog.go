// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

// ConnectorDefinition describes a built-in source or sink connector exposed by the registry catalog.
type ConnectorDefinition struct {
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	Description       string `json:"description,omitempty" yaml:"description,omitempty"`
	SourceClass       string `json:"sourceClass,omitempty" yaml:"sourceClass,omitempty"`
	SinkClass         string `json:"sinkClass,omitempty" yaml:"sinkClass,omitempty"`
	SourceConfigClass string `json:"sourceConfigClass,omitempty" yaml:"sourceConfigClass,omitempty"`
	SinkConfigClass   string `json:"sinkConfigClass,omitempty" yaml:"sinkConfigClass,omitempty"`
}

// ConfigFieldDefinition describes one connector configuration field exposed by the registry catalog.
type ConfigFieldDefinition struct {
	FieldName  string            `json:"fieldName,omitempty" yaml:"fieldName,omitempty"`
	TypeName   string            `json:"typeName,omitempty" yaml:"typeName,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}
