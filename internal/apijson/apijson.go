// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package apijson holds the JSON encoding helpers the resource types need
// beyond what encoding/json does on its own.
//
// There are only two: splicing open-ended extra fields into an object, and
// reading a union's discriminator before decoding it. Everything else the API
// needs is expressible with struct tags, `omitzero`, and
// [github.com/orca-ae/orca-sdk-go/packages/param.Opt].
package apijson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalWithExtra encodes value and splices extra into the resulting object.
//
// Several request shapes are open-ended: a tool definition, a permission
// policy, and a toolset config all carry provider-specific keys that no spec
// enumerates, because they belong to whichever provider is behind the
// deployment. Dropping them would silently discard configuration the caller
// asked for, so they travel in an Extra map and are merged back here.
//
// Declared fields win over extra keys of the same name, so a typo in Extra
// cannot quietly override a field the type already models.
func MarshalWithExtra(value any, extra map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return encoded, nil
	}

	var declared map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &declared); err != nil {
		return nil, fmt.Errorf("apijson: %T does not encode to a JSON object: %w", value, err)
	}

	merged := make(map[string]json.RawMessage, len(declared)+len(extra))
	for key, raw := range extra {
		rawEncoded, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("apijson: encoding extra field %q: %w", key, err)
		}
		merged[key] = rawEncoded
	}
	for key, raw := range declared {
		merged[key] = raw
	}
	return json.Marshal(merged)
}

// UnmarshalWithExtra decodes data into value and collects every key the type
// does not declare into extra, so an unrecognised field survives a round trip
// instead of being dropped on the way back out.
func UnmarshalWithExtra(data []byte, value any, declaredFields []string, extra *map[string]any) error {
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}

	known := make(map[string]struct{}, len(declaredFields))
	for _, name := range declaredFields {
		known[name] = struct{}{}
	}

	rest := map[string]any{}
	for key, raw := range all {
		if _, ok := known[key]; ok {
			continue
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return err
		}
		rest[key] = decoded
	}
	if len(rest) > 0 {
		*extra = rest
	}
	return nil
}

// Discriminator reads the named field from a JSON object without decoding the
// rest, so a union can pick its variant before committing to a type.
func Discriminator(data []byte, field string) (string, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	raw, ok := envelope[field]
	if !ok {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("apijson: discriminator %q is not a string: %w", field, err)
	}
	return value, nil
}

// IsJSONString reports whether data is a JSON string, which is how a union
// distinguishes a shorthand from its expanded object form.
func IsJSONString(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

// IsJSONArray reports whether data is a JSON array.
func IsJSONArray(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '['
}
