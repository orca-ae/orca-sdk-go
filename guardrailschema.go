// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"

	"github.com/orca-ae/orca-sdk-go/option"
)

// DogwoodSchemaEventSchemas describes the engine wire response.
type DogwoodSchemaEventSchemas struct {
	Session   string `json:"session"`
	Principal string `json:"principal"`
}

// DogwoodSchema describes the engine wire response.
type DogwoodSchema struct {
	BaseSchema      string                    `json:"base_schema"`
	EventSchemas    DogwoodSchemaEventSchemas `json:"event_schemas"`
	LanguageVersion string                    `json:"language_version"`
	CoreVersion     string                    `json:"core_version"`
	Limits          map[string]any            `json:"limits"`
}

// DogwoodSchema retrieves the engine's Dogwood language schema and limits.
func (s GuardrailService) DogwoodSchema(ctx context.Context, opts ...option.RequestOption) (*DogwoodSchema, error) {
	if err := s.client.ensurePolicyExtension(ctx, opts...); err != nil {
		return nil, err
	}
	var result DogwoodSchema
	if err := s.client.GetJSON(ctx, "apis/policy.runorca.ai/v1/policyschemas/dogwood", &result, opts...); err != nil {
		return nil, err
	}
	return &result, nil
}
