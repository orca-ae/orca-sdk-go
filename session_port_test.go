// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Ported from orca-sdk-typescript tests/api-resources/sessions/*.test.ts.

func TestSessionCreate(t *testing.T) {
	t.Parallel()

	t.Run("posts to the sessions collection with the agent shorthand", func(t *testing.T) {
		t.Parallel()

		// A session is never created under /v1/agents/{id}/sessions.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Create(context.Background(), SessionNewParams{
			Agent:         AgentRef("agent_abc"),
			EnvironmentID: "env_123",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/sessions" {
			t.Errorf("request = %s %s, want POST /v1/sessions", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"agent":"agent_abc","environment_id":"env_123"}`)
	})

	t.Run("sends every optional field", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Create(context.Background(), SessionNewParams{
			Agent: SessionAgentParam{
				Type:    SessionAgentRef,
				ID:      "agent_abc",
				Version: param.Int(2),
			},
			EnvironmentID: "env_123",
			VaultIDs:      []string{"vault_1", "vault_2"},
			Title:         param.String("My Session"),
			Metadata:      map[string]string{"key": "value"},
			Resources:     []SessionResourceParam{FileResource("file_abc")},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		assertJSONBody(t, transport.Only(t), `{"agent":{"id":"agent_abc","type":"agent","version":2},`+
			`"environment_id":"env_123","vault_ids":["vault_1","vault_2"],`+
			`"title":"My Session","metadata":{"key":"value"},`+
			`"resources":[{"type":"file","file_id":"file_abc"}]}`)
	})

	t.Run("sends the agent_id compatibility form as-is", func(t *testing.T) {
		t.Parallel()

		// No `agent` field is synthesized from it: the two are different
		// request shapes, and the server distinguishes them.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Create(context.Background(), SessionNewParams{
			AgentID:       param.String("agent_abc"),
			EnvironmentID: "env_123",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"agent_id":"agent_abc","environment_id":"env_123"}`)
	})

	t.Run("forwards agent overrides and initial events verbatim", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Create(context.Background(), SessionNewParams{
			Agent: SessionAgentParam{
				Type:    SessionAgentRefWithOverrides,
				ID:      "agent_abc",
				Version: param.Int(2),
				System:  param.String("Override system prompt"),
			},
			EnvironmentID: "env_123",
			InitialEvents: []SessionEventParam{UserMessage("Start")},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"agent":{"type":"agent_with_overrides","id":"agent_abc",`+
			`"version":2,"system":"Override system prompt"},"environment_id":"env_123",`+
			`"initial_events":[{"type":"user.message","content":[{"type":"text","text":"Start"}]}]}`)
	})
}

func TestSessionGet(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"session_1","type":"session",`+
			`"outcome_evaluations":[],"resources":[],"archived_at":null}`), nil
	})

	session, err := client.Sessions.Get(context.Background(), "session_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if session.OutcomeEvaluations == nil {
		t.Error("OutcomeEvaluations = nil, want an empty slice - it is a required array")
	}
	if session.Resources == nil {
		t.Error("Resources = nil, want an empty slice - it is a required array")
	}
	if session.ArchivedAt != nil {
		t.Errorf("ArchivedAt = %v, want nil", session.ArchivedAt)
	}
}

func TestSessionUpdate(t *testing.T) {
	t.Parallel()

	t.Run("uses POST", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Update(context.Background(), "session_1", SessionUpdateParams{
			Title: param.String("Updated Title"),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST, not PUT", call.Method)
		}
		assertJSONBody(t, call, `{"title":"Updated Title"}`)
	})

	t.Run("sends agent overrides, vaults and metadata", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Update(context.Background(), "session_1", SessionUpdateParams{
			Agent: SessionAgentOverridesParam{
				Tools: []AgentTool{{
					Type:        AgentToolCustom,
					Name:        "lookup_order",
					Description: "Look up an order",
					InputSchema: &AgentCustomToolInputSchema{Type: "object"},
				}},
				McpServers: []AgentMcpServerParam{{
					Name: "orders",
					Type: param.String("url"),
					URL:  "https://mcp.example.com",
				}},
			},
			VaultIDs: []string{"vault_new"},
			Metadata: param.New(map[string]*string{"updated": ptr("true")}),
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		assertJSONBody(t, transport.Only(t), `{"agent":{"tools":[{"type":"custom","name":"lookup_order",`+
			`"description":"Look up an order","input_schema":{"type":"object"}}],`+
			`"mcp_servers":[{"name":"orders","type":"url","url":"https://mcp.example.com"}]},`+
			`"vault_ids":["vault_new"],"metadata":{"updated":"true"}}`)
	})

	t.Run("cannot express the fields update removed", func(t *testing.T) {
		t.Parallel()

		// environment_id, resources and an `agent` reference were removed from
		// the update request. Accepting them would compile and then fail at the
		// server, so the parameter type does not model them at all.
		var params SessionUpdateParams
		fields := structFieldNames(params)
		for _, forbidden := range []string{"EnvironmentID", "Resources", "AgentID"} {
			if slices.Contains(fields, forbidden) {
				t.Errorf("SessionUpdateParams has a %s field, want update to accept overrides only", forbidden)
			}
		}
	})
}

func TestSessionList(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	_, err := client.Sessions.List(context.Background(), SessionListParams{
		AgentID:         param.String("agent_abc"),
		Limit:           param.Int(10),
		IncludeArchived: param.Bool(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	call := transport.Only(t)
	if call.Path() != "/v1/sessions" {
		t.Errorf("path = %q, want /v1/sessions", call.Path())
	}
	query := call.Query()
	for key, want := range map[string]string{"agent_id": "agent_abc", "limit": "10", "include_archived": "true"} {
		if got := query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestSessionDelete(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"session_xyz","type":"session_deleted"}`), nil
	})

	deleted, err := client.Sessions.Delete(context.Background(), "session_xyz")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if got := transport.Only(t).Method; got != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", got)
	}
	if deleted.Type != "session_deleted" {
		t.Errorf("tombstone type = %q, want %q", deleted.Type, "session_deleted")
	}
}

func TestSessionArchive(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"s1","archived_at":"2026-01-02T00:00:00Z"}`), nil
	})

	session, err := client.Sessions.Archive(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	call := transport.Only(t)
	if call.Method != http.MethodPost || call.Path() != "/v1/sessions/s1/archive" {
		t.Errorf("request = %s %s, want POST /v1/sessions/s1/archive", call.Method, call.Path())
	}
	if session.ArchivedAt == nil {
		t.Error("ArchivedAt = nil, want the archived session back")
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func TestSessionEventsList(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, nil)
	_, err := client.Sessions.Events.List(context.Background(), "s1", SessionEventListParams{
		Limit:        param.Int(10),
		Page:         param.String("p2"),
		Order:        param.New(SessionEventOrderAsc),
		CreatedAtGT:  param.String("2026-01-01T00:00:00Z"),
		CreatedAtGTE: param.String("2026-01-02T00:00:00Z"),
		CreatedAtLT:  param.String("2026-02-01T00:00:00Z"),
		CreatedAtLTE: param.String("2026-02-02T00:00:00Z"),
		Types:        []string{"user.message", "agent.message"},
		Subpath:      param.String("sub"),
	})
	if err != nil {
		t.Fatalf("Events.List() error = %v", err)
	}

	call := transport.Only(t)
	if got, want := call.Path(), "/v1/sessions/s1/events"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}

	query := call.Query()
	// The bracket keys are literal parameter names, not nested objects.
	for key, want := range map[string]string{
		"limit":           "10",
		"page":            "p2",
		"order":           "asc",
		"created_at[gt]":  "2026-01-01T00:00:00Z",
		"created_at[gte]": "2026-01-02T00:00:00Z",
		"created_at[lt]":  "2026-02-01T00:00:00Z",
		"created_at[lte]": "2026-02-02T00:00:00Z",
		"subpath":         "sub",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got, want := query["types"], []string{"user.message", "agent.message"}; !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v - the parameter repeats", got, want)
	}
}

func TestSessionEventsSend(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[{"id":"evt_1","type":"user.message"}]}`), nil
	})

	events, err := client.Sessions.Events.Send(context.Background(), "s1",
		[]SessionEventParam{UserMessage("Hello!")})
	if err != nil {
		t.Fatalf("Events.Send() error = %v", err)
	}

	call := transport.Only(t)
	if call.Method != http.MethodPost || call.Path() != "/v1/sessions/s1/events" {
		t.Errorf("request = %s %s, want POST /v1/sessions/s1/events", call.Method, call.Path())
	}
	assertJSONBody(t, call, `{"events":[{"type":"user.message","content":[{"type":"text","text":"Hello!"}]}]}`)

	if len(events) != 1 || events[0].ID != "evt_1" {
		t.Errorf("events = %+v, want the persisted forms back", events)
	}

	t.Run("several events go in one call", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"data":[]}`), nil
		})
		_, err := client.Sessions.Events.Send(context.Background(), "s1", []SessionEventParam{
			UserMessage("Hello!"),
			{Type: "user.interrupt"},
		})
		if err != nil {
			t.Fatalf("Events.Send() error = %v", err)
		}
		assertJSONBody(t, transport.Only(t), `{"events":[{"type":"user.message",`+
			`"content":[{"type":"text","text":"Hello!"}]},{"type":"user.interrupt"}]}`)
	})
}

func TestSessionEventsStream(t *testing.T) {
	t.Parallel()

	t.Run("yields events one at a time", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK,
				`data: {"id":"evt_1","type":"user.message"}`+"\n\n"+
					`data: {"id":"evt_2","type":"agent.message"}`+"\n\n"), nil
		})

		stream := client.Sessions.Events.Stream(context.Background(), "s1", SessionEventStreamParams{})
		defer stream.Close()

		var ids []string
		for stream.Next() {
			ids = append(ids, stream.Current().ID)
		}
		if err := stream.Err(); err != nil {
			t.Fatalf("stream error = %v", err)
		}
		if want := []string{"evt_1", "evt_2"}; !slices.Equal(ids, want) {
			t.Errorf("ids = %v, want %v", ids, want)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/s1/events/stream"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q, want text/event-stream", got)
		}
		// Passing no parameters must produce an empty query string.
		if got := call.URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}
	})

	t.Run("sends the stream controls it was given", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, ""), nil
		})

		stream := client.Sessions.Events.Stream(context.Background(), "s1", SessionEventStreamParams{
			FromCursor:  param.String("cursor-1"),
			Subpath:     param.String("sub"),
			EventDeltas: []string{"text", "tool_use"},
		})
		for stream.Next() {
		}
		_ = stream.Close()

		query := transport.Only(t).Query()
		if got := query.Get("from_cursor"); got != "cursor-1" {
			t.Errorf("from_cursor = %q, want cursor-1", got)
		}
		if got := query.Get("subpath"); got != "sub" {
			t.Errorf("subpath = %q, want sub", got)
		}
		if got, want := query["event_deltas"], []string{"text", "tool_use"}; !slices.Equal(got, want) {
			t.Errorf("event_deltas = %v, want %v", got, want)
		}
	})

	t.Run("survives arbitrary chunk boundaries", func(t *testing.T) {
		t.Parallel()

		// The TypeScript suite feeds its parser one byte at a time; this does
		// the same, because a frame split across reads is the normal case on a
		// real connection.
		body := `data: {"id":"evt_1","type":"user.message"}` + "\n\n" +
			`data: {"id":"evt_2","type":"agent.message"}` + "\n\n"
		chunks := make([]string, 0, len(body))
		for _, b := range []byte(body) {
			chunks = append(chunks, string(b))
		}

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			res := sseResponse(http.StatusOK, "")
			res.Body = &sseChunkReadCloser{reader: &sseChunkReader{chunks: chunks}}
			return res, nil
		})

		stream := client.Sessions.Events.Stream(context.Background(), "s1", SessionEventStreamParams{})
		defer stream.Close()

		var ids []string
		for stream.Next() {
			ids = append(ids, stream.Current().ID)
		}
		if err := stream.Err(); err != nil {
			t.Fatalf("stream error = %v", err)
		}
		if want := []string{"evt_1", "evt_2"}; !slices.Equal(ids, want) {
			t.Errorf("ids = %v, want %v", ids, want)
		}
	})
}

// sseChunkReadCloser adapts the chunk reader to a response body.
type sseChunkReadCloser struct {
	reader *sseChunkReader
}

func (r *sseChunkReadCloser) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *sseChunkReadCloser) Close() error               { return nil }

// ---------------------------------------------------------------------------
// Files
// ---------------------------------------------------------------------------

func TestSessionFiles(t *testing.T) {
	t.Parallel()

	t.Run("list uses ID cursors", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Files.List(context.Background(), "s1", SessionFileListParams{
			Limit:   param.Int(10),
			AfterID: param.String("file_1"),
		})
		if err != nil {
			t.Fatalf("Files.List() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/s1/files"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Query().Get("after_id"); got != "file_1" {
			t.Errorf("after_id = %q, want file_1", got)
		}
	})

	t.Run("a backwards walk keeps its direction", func(t *testing.T) {
		t.Parallel()

		var calls int
		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(http.StatusOK, `{"data":[{"id":"file_3"}],"has_more":true,`+
					`"first_id":"file_3","last_id":"file_2"}`), nil
			}
			return jsonResponse(http.StatusOK, `{"data":[],"has_more":false}`), nil
		})

		page, err := client.Sessions.Files.List(context.Background(), "s1", SessionFileListParams{
			Limit:    param.Int(10),
			BeforeID: param.String("file_1"),
		})
		if err != nil {
			t.Fatalf("Files.List() error = %v", err)
		}
		if _, err := page.GetNextPage(context.Background()); err != nil {
			t.Fatalf("GetNextPage() error = %v", err)
		}

		next := transport.Calls()[1].Query()
		if got := next.Get("before_id"); got != "file_3" {
			t.Errorf("before_id = %q, want file_3", got)
		}
		if next.Has("after_id") {
			t.Errorf("after_id = %q, want it absent on a backwards walk", next.Get("after_id"))
		}
		if got := next.Get("limit"); got != "10" {
			t.Errorf("limit = %q, want the caller's 10 to survive", got)
		}
	})

	t.Run("escapes both path parameters", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Sessions.Files.Get(context.Background(), "session/slash", "file/slash"); err != nil {
			t.Fatalf("Files.Get() error = %v", err)
		}
		want := "/v1/sessions/session%2Fslash/files/file%2Fslash"
		if got := transport.Only(t).Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("download asks for octet-stream and returns the raw body", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			res := jsonResponse(http.StatusOK, "file-bytes")
			res.Header.Set("Content-Type", "application/octet-stream")
			return res, nil
		})

		var buf bytes.Buffer
		if err := client.Sessions.Files.Download(context.Background(), "s1", "f1", &buf); err != nil {
			t.Fatalf("Files.Download() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/s1/files/f1/content"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Header.Get("Accept"); got != "application/octet-stream" {
			t.Errorf("Accept = %q, want application/octet-stream", got)
		}
		if got := buf.String(); got != "file-bytes" {
			t.Errorf("body = %q, want %q", got, "file-bytes")
		}
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"file_abc","type":"file_deleted"}`), nil
		})

		deleted, err := client.Sessions.Files.Delete(context.Background(), "s1", "file_abc")
		if err != nil {
			t.Fatalf("Files.Delete() error = %v", err)
		}
		if got := transport.Only(t).Method; got != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", got)
		}
		if deleted.Type != "file_deleted" {
			t.Errorf("tombstone type = %q, want file_deleted", deleted.Type)
		}
	})

	t.Run("every operation stays under /v1/sessions", func(t *testing.T) {
		t.Parallel()

		// No /v1/registry prefix and no /apis extension prefix, on either
		// backend: session files are core, not an extension.
		client, transport := newRecordingClient(t, nil)
		ctx := context.Background()
		_, _ = client.Sessions.Files.List(ctx, "s1", SessionFileListParams{})
		_, _ = client.Sessions.Files.Get(ctx, "s1", "f1")
		_ = client.Sessions.Files.Download(ctx, "s1", "f1", &bytes.Buffer{})
		_, _ = client.Sessions.Files.Delete(ctx, "s1", "f1")

		for _, call := range transport.Calls() {
			path := call.Path()
			if len(path) < len("/v1/sessions") || path[:len("/v1/sessions")] != "/v1/sessions" {
				t.Errorf("path = %q, want it under /v1/sessions", path)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Resources
// ---------------------------------------------------------------------------

func TestSessionResources(t *testing.T) {
	t.Parallel()

	t.Run("add sends the resource object as the body", func(t *testing.T) {
		t.Parallel()

		// Not wrapped in a "resource" key.
		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Resources.Add(context.Background(), "s1", FileResource("file_doc"))
		if err != nil {
			t.Fatalf("Resources.Add() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/sessions/s1/resources" {
			t.Errorf("request = %s %s, want POST /v1/sessions/s1/resources", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"type":"file","file_id":"file_doc"}`)
	})

	t.Run("list pages with limit and page", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Resources.List(context.Background(), "s1", SessionResourceListParams{
			Limit: param.Int(5),
			Page:  param.String("p2"),
		})
		if err != nil {
			t.Fatalf("Resources.List() error = %v", err)
		}
		query := transport.Only(t).Query()
		if query.Get("limit") != "5" || query.Get("page") != "p2" {
			t.Errorf("query = %v, want limit=5 and page=p2", query)
		}
	})

	t.Run("update rotates the token with POST", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Resources.Update(context.Background(), "s1", "r1",
			SessionResourceUpdateParams{AuthorizationToken: param.String("updated-token")})
		if err != nil {
			t.Fatalf("Resources.Update() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", call.Method)
		}
		if got, want := call.Path(), "/v1/sessions/s1/resources/r1"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		assertJSONBody(t, call, `{"authorization_token":"updated-token"}`)
	})

	t.Run("delete returns a tombstone", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"resource_abc","type":"session_resource_deleted"}`), nil
		})
		deleted, err := client.Sessions.Resources.Delete(context.Background(), "s1", "resource_abc")
		if err != nil {
			t.Fatalf("Resources.Delete() error = %v", err)
		}
		if deleted.Type != "session_resource_deleted" {
			t.Errorf("tombstone type = %q, want session_resource_deleted", deleted.Type)
		}
	})
}

// ---------------------------------------------------------------------------
// Threads
// ---------------------------------------------------------------------------

func TestSessionThreads(t *testing.T) {
	t.Parallel()

	t.Run("retrieve decodes the nullable thread fields", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"id":"t1","type":"session_thread","status":"idle",`+
				`"stats":null,"usage":null,"parent_thread_id":null,"archived_at":null}`), nil
		})

		thread, err := client.Sessions.Threads.Get(context.Background(), "session/slash", "thread/slash")
		if err != nil {
			t.Fatalf("Threads.Get() error = %v", err)
		}

		// Unlike a session, a thread's stats and usage are nullable.
		if thread.Stats != nil || thread.Usage != nil {
			t.Errorf("stats = %v, usage = %v, want both nil", thread.Stats, thread.Usage)
		}
		if thread.ParentThreadID != nil {
			t.Errorf("ParentThreadID = %v, want nil for the primary thread", thread.ParentThreadID)
		}
		want := "/v1/sessions/session%2Fslash/threads/thread%2Fslash"
		if got := transport.Only(t).Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("list accepts limit and page only", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Threads.List(context.Background(), "s1", SessionThreadListParams{
			Limit: param.Int(5),
			Page:  param.String("p2"),
		})
		if err != nil {
			t.Fatalf("Threads.List() error = %v", err)
		}

		query := transport.Only(t).Query()
		if query.Has("include_archived") || query.Has("order") {
			t.Errorf("query = %v, want neither include_archived nor order", query)
		}

		fields := structFieldNames(SessionThreadListParams{})
		if len(fields) != 2 {
			t.Errorf("SessionThreadListParams fields = %v, want only Limit and Page", fields)
		}
	})

	t.Run("archive posts to the archive sub-path", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		if _, err := client.Sessions.Threads.Archive(context.Background(), "s1", "t1"); err != nil {
			t.Fatalf("Threads.Archive() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/sessions/s1/threads/t1/archive" {
			t.Errorf("request = %s %s, want POST /v1/sessions/s1/threads/t1/archive", call.Method, call.Path())
		}
	})

	t.Run("there is no create", func(t *testing.T) {
		t.Parallel()

		// Threads are spawned by the coordinator. A create method would suggest
		// the caller controls something the server owns.
		if _, ok := any(SessionThreadService{}).(interface {
			Create(context.Context, string) (*SessionThread, error)
		}); ok {
			t.Error("SessionThreadService has a Create method, want threads to be server-spawned")
		}
	})

	t.Run("events list accepts pagination only", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Sessions.Threads.Events.List(context.Background(), "s1", "t1",
			SessionThreadEventListParams{Limit: param.Int(5), Page: param.String("p2")})
		if err != nil {
			t.Fatalf("Threads.Events.List() error = %v", err)
		}
		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/s1/threads/t1/events"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if call.Query().Has("order") {
			t.Error("query carries order, want it unsupported here")
		}
	})

	t.Run("the thread stream hangs off /stream, not /events/stream", func(t *testing.T) {
		t.Parallel()

		// The one place the thread and session stream paths diverge.
		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, ""), nil
		})

		stream := client.Sessions.Threads.Events.Stream(context.Background(), "s1", "t1",
			SessionThreadEventStreamParams{})
		for stream.Next() {
		}
		_ = stream.Close()

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/s1/threads/t1/stream"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}

		// A thread stream takes no subpath: the thread already scopes it.
		fields := structFieldNames(SessionThreadEventStreamParams{})
		if slices.Contains(fields, "Subpath") {
			t.Error("SessionThreadEventStreamParams has a Subpath field, want it unsupported")
		}
	})
}

func TestSessionOutcome(t *testing.T) {
	t.Parallel()

	t.Run("returns the graded outcome", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"type":"outcome_evaluation","outcome_id":"oc_1",`+
				`"description":"Did the thing","result":"pass","explanation":null,"iteration":2,`+
				`"completed_at":"2026-01-02T00:00:00Z","provider_detail":{"score":0.9}}`), nil
		})

		outcome, err := client.Sessions.Outcome(context.Background(), "session/slash")
		if err != nil {
			t.Fatalf("Outcome() error = %v", err)
		}
		if outcome == nil {
			t.Fatal("Outcome() = nil, want the evaluation")
		}
		if outcome.OutcomeID != "oc_1" || outcome.Result != "pass" || outcome.Iteration != 2 {
			t.Errorf("outcome = %+v, want oc_1/pass/2", outcome)
		}
		if outcome.Explanation != nil {
			t.Errorf("Explanation = %v, want nil - it is nullable", outcome.Explanation)
		}
		// The shape is open-ended, so a provider-specific key must survive.
		if outcome.Extra["provider_detail"] == nil {
			t.Errorf("Extra = %v, want provider_detail preserved", outcome.Extra)
		}

		want := "/v1/sessions/session%2Fslash/outcome"
		if got := transport.Only(t).Path(); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("a null body is no outcome yet, not a failure", func(t *testing.T) {
		t.Parallel()

		// The contract marks the whole response nullable. A session that has
		// not been graded is a normal state, so this must not be an error and
		// must not be an empty-but-present evaluation either.
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `null`), nil
		})

		outcome, err := client.Sessions.Outcome(context.Background(), "s1")
		if err != nil {
			t.Fatalf("Outcome() error = %v, want nil", err)
		}
		if outcome != nil {
			t.Errorf("Outcome() = %+v, want nil", outcome)
		}
	})
}
