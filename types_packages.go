// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

// PackageMetadata mirrors the workspace registry package metadata schema.
type PackageMetadata struct {
	Contact          string            `json:"contact,omitempty" yaml:"contact,omitempty"`
	CreateTime       int64             `json:"createTime,omitempty" yaml:"createTime,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	ModificationTime int64             `json:"modificationTime,omitempty" yaml:"modificationTime,omitempty"`
	Properties       map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
}
