// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// A decoded value is shaped by whatever this SDK models. Callers that have to
// reproduce a response - render it, forward it, store it - need the bytes the
// server actually sent, or they silently drop every field the types do not
// declare.

func TestWithRawJSON(t *testing.T) {
	t.Parallel()

	// A field no Agent type declares. It must survive into the raw bytes and be
	// absent from the decoded value: that difference is the whole point.
	const body = `{"id":"agent_1","name":"demo","undeclared_field":{"nested":[1,2]}}`

	t.Run("captures the body verbatim alongside the typed value", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, body), nil
		})

		var raw []byte
		agent, err := client.Agents.Get(context.Background(), "agent_1", AgentGetParams{},
			option.WithRawJSON(&raw))
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if agent.ID != "agent_1" {
			t.Errorf("decoded id = %q, want agent_1", agent.ID)
		}
		if got := string(raw); got != body {
			t.Errorf("raw = %s, want %s", got, body)
		}

		// The undeclared field is in the bytes and nowhere in the typed value.
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("raw is not valid JSON: %v", err)
		}
		if _, ok := decoded["undeclared_field"]; !ok {
			t.Error("raw dropped undeclared_field, want it preserved")
		}
		reEncoded, err := json.Marshal(agent)
		if err != nil {
			t.Fatalf("re-encoding the typed value: %v", err)
		}
		if bytes.Contains(reEncoded, []byte("undeclared_field")) {
			t.Error("the typed value carries undeclared_field; this test no longer proves anything")
		}
	})

	t.Run("captures a failing response so the server's own words survive", func(t *testing.T) {
		t.Parallel()

		// A caller diagnosing a failure wants what the deployment said, not
		// only what this SDK made of it.
		const failure = `{"error":{"type":"not_found_error","message":"no such agent","hint":"check the id"}}`
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, failure), nil
		})

		var raw []byte
		_, err := client.Agents.Get(context.Background(), "missing", AgentGetParams{},
			option.WithRawJSON(&raw))
		if err == nil {
			t.Fatal("Get() error = nil, want a failure")
		}
		if got := string(raw); got != failure {
			t.Errorf("raw = %s, want %s", got, failure)
		}
	})

	t.Run("works on a list page", func(t *testing.T) {
		t.Parallel()

		const page = `{"data":[{"id":"agent_1"}],"has_more":false,"server_note":"kept"}`
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, page), nil
		})

		var raw []byte
		cursor, err := client.Agents.List(context.Background(), AgentListParams{Limit: param.Int(1)},
			option.WithRawJSON(&raw))
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(cursor.Items()) != 1 {
			t.Errorf("items = %d, want 1", len(cursor.Items()))
		}
		if got := string(raw); got != page {
			t.Errorf("raw = %s, want %s", got, page)
		}
	})

	t.Run("is per call and does not leak onto the client", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"agent_1"}`), nil
		})

		ctx := context.Background()
		var first []byte
		if _, err := client.Agents.Get(ctx, "a1", AgentGetParams{}, option.WithRawJSON(&first)); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if len(first) == 0 {
			t.Fatal("first raw is empty")
		}

		// A second call without the option must not write through the first
		// call's pointer.
		before := string(first)
		if _, err := client.Agents.Get(ctx, "a2", AgentGetParams{}); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if string(first) != before {
			t.Error("the second call overwrote the first call's buffer, want the option scoped to one call")
		}
	})

	t.Run("a nil destination is refused rather than ignored", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, nil)
		_, err := client.Agents.Get(context.Background(), "a1", AgentGetParams{},
			option.WithRawJSON(nil))
		if err == nil {
			t.Fatal("Get() error = nil, want the nil destination refused")
		}
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Errorf("error = %T (%v), want *ValidationError", err, err)
		}
	})
}
