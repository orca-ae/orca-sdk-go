// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"reflect"
	"testing"
)

// The shared harness for the ported specification tables.
//
// Each pending_*_port_test.go file carries the exact wire contract of a
// resource this SDK has not typed yet: the HTTP method, path, query parameters,
// request body and response shape a future typed resource must produce.
// Implementing a resource means converting its table into tests that execute
// it, not deleting the table.
//
//	go test -v ./... 2>&1 | grep -c "pending: typed Managed Agents"
//
// reports how much of that surface is still outstanding.

// pendingManagedAgents is the single skip reason for ported tests that specify
// typed Managed Agents resources this SDK does not implement yet.
const pendingManagedAgents = "pending: typed Managed Agents resources"

// pendingPortOp is one operation in a ported specification table.
type pendingPortOp struct {
	// Name is the SDK method the TypeScript suite exercises, e.g. "agents.create".
	Name string

	// Method is the HTTP verb. Note that every core resource updates with POST,
	// never PUT, and archives through a POST .../archive sub-path.
	Method string

	// Path is the request path with {placeholders} for path parameters. Every
	// path parameter is percent-escaped, so an ID of "agent/with/slash" is sent
	// as "agent%2Fwith%2Fslash" and never adds path segments.
	Path string

	// Query names the query parameters the operation accepts. Bracket keys such
	// as "created_at[gte]" are literal parameter names, not nested structures.
	// A repeated parameter is marked "name (repeated)".
	Query []string

	// Body is the JSON request body the SDK must send, verbatim, or "" for
	// operations that send no body. Multipart bodies are described in Note.
	Body string

	// Response is the JSON response body the TypeScript suite asserts on, where
	// its exact shape is part of the contract (tombstones, discriminators).
	Response string

	// Note records a contract detail the fields above cannot carry.
	Note string
}

// pendingPortSurface reports every operation in spec as unimplemented. Tests
// call it after t.Skip(pendingManagedAgents), so it runs only once someone
// lifts the skip - at which point it names exactly what is still missing
// instead of failing with a bare compile error.
func pendingPortSurface(t *testing.T, spec []pendingPortOp) {
	t.Helper()
	for _, op := range spec {
		t.Errorf("no typed resource implements %s: %s %s", op.Name, op.Method, op.Path)
	}
}

// structFieldNames returns the exported field names of a struct value.
//
// It is used to assert that a request type cannot express a field the contract
// removed. Checking the type rather than the request body is what makes the
// guarantee compile-time: a field that does not exist cannot be set by mistake
// and then rejected by the server at run time.
func structFieldNames(value any) []string {
	typ := reflect.TypeOf(value)
	names := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		if field := typ.Field(i); field.IsExported() {
			names = append(names, field.Name)
		}
	}
	return names
}
