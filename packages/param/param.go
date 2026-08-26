// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package param models optional request fields.
//
// An optional field in an Orca request body has three distinct meanings on the
// wire, and the API acts on all three:
//
//   - absent - the key is not sent, and the server leaves the field alone.
//   - null - the key is sent as null, and the server clears the field.
//   - present - the key is sent with a value.
//
// The usual Go spelling of "optional" - a pointer with `omitempty` - can only
// express two of those, and picks the wrong two: a nil pointer omits the key so
// there is no way to send null, while a non-nil pointer to a zero value gets
// dropped by omitempty even though the caller asked for `limit: 0`.
//
// [Opt] carries the state explicitly and reports it through IsZero, which the
// `omitzero` struct tag consults. Tag every optional field with `omitzero`:
//
//	type AgentUpdateParams struct {
//		Version  param.Opt[int64]  `json:"version,omitzero"`
//		Metadata param.Opt[Meta]   `json:"metadata,omitzero"`
//	}
//
// With that tag an absent Opt disappears from the request, a null Opt encodes
// as null, and a present Opt encodes its value - including a zero value.
package param

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// state is which of the three wire meanings an [Opt] carries. The zero value is
// absent, so a field nobody set is a field nobody sends.
type state uint8

const (
	stateAbsent state = iota
	stateNull
	statePresent
)

// Opt is an optional request field that distinguishes absent from null from a
// value. The zero Opt is absent.
//
// Opt is comparable whenever T is, which makes it usable as a struct field in
// tests that compare whole request bodies.
type Opt[T any] struct {
	value T
	state state
}

// New returns an Opt carrying value.
func New[T any](value T) Opt[T] {
	return Opt[T]{value: value, state: statePresent}
}

// Null returns an Opt that encodes as JSON null, asking the server to clear the
// field rather than leave it alone.
func Null[T any]() Opt[T] {
	return Opt[T]{state: stateNull}
}

// String returns an Opt carrying value. It exists so callers can write
// param.String("x") instead of param.New[string]("x") at the call sites where
// the type is not already inferable.
func String(value string) Opt[string] { return New(value) }

// Int returns an Opt carrying value.
func Int(value int64) Opt[int64] { return New(value) }

// Bool returns an Opt carrying value.
func Bool(value bool) Opt[bool] { return New(value) }

// Float returns an Opt carrying value.
func Float(value float64) Opt[float64] { return New(value) }

// IsZero reports whether the field is absent. encoding/json calls it for fields
// tagged `omitzero`, which is how an absent Opt is left out of the request.
//
// It deliberately ignores the wrapped value: an Opt holding the zero value of T
// is present, not absent, because the caller asked for `limit: 0`.
func (o Opt[T]) IsZero() bool { return o.state == stateAbsent }

// IsNull reports whether the field is an explicit null.
func (o Opt[T]) IsNull() bool { return o.state == stateNull }

// Valid reports whether the field carries a value, as opposed to being absent
// or null.
func (o Opt[T]) Valid() bool { return o.state == statePresent }

// Value returns the wrapped value and whether there was one. An absent or null
// Opt returns the zero value of T and false.
func (o Opt[T]) Value() (T, bool) {
	if o.state != statePresent {
		var zero T
		return zero, false
	}
	return o.value, true
}

// Or returns the wrapped value, or fallback when the field is absent or null.
func (o Opt[T]) Or(fallback T) T {
	if o.state != statePresent {
		return fallback
	}
	return o.value
}

// MarshalJSON implements [json.Marshaler].
//
// It is not reached for an absent field, because `omitzero` drops the field
// before marshalling. An Opt marshalled without that tag - which is a tagging
// mistake - encodes as null rather than silently emitting a zero value that the
// caller never asked to send.
func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if o.state != statePresent {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements [json.Unmarshaler].
//
// encoding/json passes a literal null through to this method rather than
// skipping the field, which is what lets a response distinguish a field the
// server nulled from one it omitted.
func (o *Opt[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		var zero T
		o.value, o.state = zero, stateNull
		return nil
	}
	if err := json.Unmarshal(data, &o.value); err != nil {
		return err
	}
	o.state = statePresent
	return nil
}

// String implements [fmt.Stringer] so an Opt in a test failure or log line
// reads as its state rather than as an opaque struct.
func (o Opt[T]) String() string {
	switch o.state {
	case stateNull:
		return "null"
	case statePresent:
		return fmt.Sprintf("%v", o.value)
	default:
		return "absent"
	}
}
