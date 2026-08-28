// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package param_test

import (
	"encoding/json"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// The three states an optional request field can be in are all distinct on the
// wire, and the API relies on the distinction: omitting `version` from an agent
// update means "no optimistic concurrency check", while sending `"metadata":
// {"remove": null}` means "delete this key". Collapsing absent into null - the
// usual outcome of modelling optional fields with pointers and omitempty -
// silently changes what the request asks for.
type body struct {
	Absent  param.Opt[string] `json:"absent,omitzero"`
	Null    param.Opt[string] `json:"null,omitzero"`
	Present param.Opt[string] `json:"present,omitzero"`
}

func TestMarshalDistinguishesAbsentNullAndPresent(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(body{
		Null:    param.Null[string](),
		Present: param.String("value"),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"null":null,"present":"value"}`
	if got := string(encoded); got != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshalZeroValueIsNotAbsent(t *testing.T) {
	t.Parallel()

	// An explicit zero must survive. If Opt reported IsZero by delegating to
	// the wrapped value, `limit: 0` and `include_archived: false` would vanish
	// from the query the caller asked for.
	type numbers struct {
		Limit    param.Opt[int64]   `json:"limit,omitzero"`
		Archived param.Opt[bool]    `json:"archived,omitzero"`
		Ratio    param.Opt[float64] `json:"ratio,omitzero"`
		Name     param.Opt[string]  `json:"name,omitzero"`
	}

	encoded, err := json.Marshal(numbers{
		Limit:    param.Int(0),
		Archived: param.Bool(false),
		Ratio:    param.Float(0),
		Name:     param.String(""),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"limit":0,"archived":false,"ratio":0,"name":""}`
	if got := string(encoded); got != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestUnmarshalRecordsNullSeparatelyFromAbsent(t *testing.T) {
	t.Parallel()

	var decoded body
	if err := json.Unmarshal([]byte(`{"null":null,"present":"value"}`), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	tests := []struct {
		name       string
		field      param.Opt[string]
		wantAbsent bool
		wantNull   bool
		wantValue  string
		wantValid  bool
	}{
		{name: "absent", field: decoded.Absent, wantAbsent: true},
		{name: "null", field: decoded.Null, wantNull: true},
		{name: "present", field: decoded.Present, wantValue: "value", wantValid: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.field.IsZero(); got != tc.wantAbsent {
				t.Errorf("IsZero() = %v, want %v", got, tc.wantAbsent)
			}
			if got := tc.field.IsNull(); got != tc.wantNull {
				t.Errorf("IsNull() = %v, want %v", got, tc.wantNull)
			}
			value, valid := tc.field.Value()
			if valid != tc.wantValid {
				t.Errorf("Value() valid = %v, want %v", valid, tc.wantValid)
			}
			if value != tc.wantValue {
				t.Errorf("Value() = %q, want %q", value, tc.wantValue)
			}
		})
	}
}

func TestOrReturnsFallbackUnlessPresent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field param.Opt[string]
		want  string
	}{
		{name: "absent", field: param.Opt[string]{}, want: "fallback"},
		{name: "null", field: param.Null[string](), want: "fallback"},
		{name: "present", field: param.String("value"), want: "value"},
		{name: "present zero value", field: param.String(""), want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.field.Or("fallback"); got != tc.want {
				t.Errorf("Or() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRoundTripPreservesState(t *testing.T) {
	t.Parallel()

	original := body{Null: param.Null[string](), Present: param.String("value")}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded body
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded != original {
		t.Errorf("round trip = %+v, want %+v", decoded, original)
	}
}

func TestNestedStructsMarshalThroughOpt(t *testing.T) {
	t.Parallel()

	// Nullable object fields are common in update requests: agent update sends
	// `"multiagent": null` to clear the field.
	type inner struct {
		Name string `json:"name"`
	}
	type outer struct {
		Config param.Opt[inner] `json:"config,omitzero"`
	}

	encoded, err := json.Marshal(outer{Config: param.New(inner{Name: "x"})})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"config":{"name":"x"}}`; got != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}

	encoded, err = json.Marshal(outer{Config: param.Null[inner]()})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"config":null}`; got != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}
