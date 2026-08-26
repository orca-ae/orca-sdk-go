// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"net/http"
	"testing"
)

// Ported from orca-sdk-typescript tests/lib/session.test.ts.
//
// The handle is a binding and nothing more. Every test here asserts that it
// produces the same request the equivalent call on client.Sessions would, so a
// convenience cannot quietly become a second implementation.

func TestSessionHandle(t *testing.T) {
	t.Parallel()

	t.Run("exposes the session id without making a request", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		handle := client.Session("session_abc")

		if got := handle.SessionID(); got != "session_abc" {
			t.Errorf("SessionID() = %q, want %q", got, "session_abc")
		}
		if got := len(transport.Calls()); got != 0 {
			t.Errorf("requests = %d, want none - building a handle talks to nothing", got)
		}
	})

	t.Run("events.send", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{"data":[]}`), nil
		})

		_, err := client.Session("session_abc").Events.Send(context.Background(),
			[]SessionEventParam{UserMessage("test")})
		if err != nil {
			t.Fatalf("Events.Send() error = %v", err)
		}

		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/sessions/session_abc/events" {
			t.Errorf("request = %s %s, want POST /v1/sessions/session_abc/events", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"events":[{"type":"user.message","content":[{"type":"text","text":"test"}]}]}`)
	})

	t.Run("events.stream produces an empty query when given nothing", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, ""), nil
		})

		stream := client.Session("session_abc").Events.Stream(context.Background(), SessionEventStreamParams{})
		for stream.Next() {
		}
		_ = stream.Close()

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/session_abc/events/stream"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}
	})

	t.Run("threads.events.stream uses /stream, not /events/stream", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, ""), nil
		})

		stream := client.Session("session_abc").Threads.Events.Stream(context.Background(), "thread_1",
			SessionThreadEventStreamParams{})
		for stream.Next() {
		}
		_ = stream.Close()

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/session_abc/threads/thread_1/stream"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.URL.RawQuery; got != "" {
			t.Errorf("query = %q, want it empty", got)
		}
	})

	t.Run("resources.add", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, nil)
		_, err := client.Session("session_abc").Resources.Add(context.Background(), FileResource("file_doc"))
		if err != nil {
			t.Fatalf("Resources.Add() error = %v", err)
		}
		call := transport.Only(t)
		if call.Method != http.MethodPost || call.Path() != "/v1/sessions/session_abc/resources" {
			t.Errorf("request = %s %s, want POST /v1/sessions/session_abc/resources", call.Method, call.Path())
		}
		assertJSONBody(t, call, `{"type":"file","file_id":"file_doc"}`)
	})

	t.Run("files.download", func(t *testing.T) {
		t.Parallel()

		client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, "raw"), nil
		})

		var buf bytes.Buffer
		err := client.Session("session_abc").Files.Download(context.Background(), "file_1", &buf)
		if err != nil {
			t.Fatalf("Files.Download() error = %v", err)
		}

		call := transport.Only(t)
		if got, want := call.Path(), "/v1/sessions/session_abc/files/file_1/content"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := call.Header.Get("Accept"); got != "application/octet-stream" {
			t.Errorf("Accept = %q, want application/octet-stream", got)
		}
		if buf.String() != "raw" {
			t.Errorf("body = %q, want raw", buf.String())
		}
	})

	t.Run("every handle call matches the unbound call byte for byte", func(t *testing.T) {
		t.Parallel()

		// The point of the handle is that it removes repetition, not that it
		// behaves differently. Comparing the captured requests is what keeps it
		// from drifting into a second implementation.
		const sessionID = "session_abc"

		bound, boundTransport := newRecordingClient(t, nil)
		unbound, unboundTransport := newRecordingClient(t, nil)
		ctx := context.Background()

		handle := bound.Session(sessionID)
		calls := []struct {
			name      string
			viaHandle func() error
			direct    func() error
		}{
			{
				name:      "get",
				viaHandle: func() error { _, err := handle.Get(ctx); return err },
				direct:    func() error { _, err := unbound.Sessions.Get(ctx, sessionID); return err },
			},
			{
				name:      "events.list",
				viaHandle: func() error { _, err := handle.Events.List(ctx, SessionEventListParams{}); return err },
				direct: func() error {
					_, err := unbound.Sessions.Events.List(ctx, sessionID, SessionEventListParams{})
					return err
				},
			},
			{
				name:      "files.list",
				viaHandle: func() error { _, err := handle.Files.List(ctx, SessionFileListParams{}); return err },
				direct: func() error {
					_, err := unbound.Sessions.Files.List(ctx, sessionID, SessionFileListParams{})
					return err
				},
			},
			{
				name:      "resources.list",
				viaHandle: func() error { _, err := handle.Resources.List(ctx, SessionResourceListParams{}); return err },
				direct: func() error {
					_, err := unbound.Sessions.Resources.List(ctx, sessionID, SessionResourceListParams{})
					return err
				},
			},
			{
				name:      "threads.list",
				viaHandle: func() error { _, err := handle.Threads.List(ctx, SessionThreadListParams{}); return err },
				direct: func() error {
					_, err := unbound.Sessions.Threads.List(ctx, sessionID, SessionThreadListParams{})
					return err
				},
			},
			{
				name:      "threads.get",
				viaHandle: func() error { _, err := handle.Threads.Get(ctx, "t1"); return err },
				direct:    func() error { _, err := unbound.Sessions.Threads.Get(ctx, sessionID, "t1"); return err },
			},
			{
				name:      "archive",
				viaHandle: func() error { _, err := handle.Archive(ctx); return err },
				direct:    func() error { _, err := unbound.Sessions.Archive(ctx, sessionID); return err },
			},
		}

		for _, tc := range calls {
			if err := tc.viaHandle(); err != nil {
				t.Fatalf("%s via handle error = %v", tc.name, err)
			}
			if err := tc.direct(); err != nil {
				t.Fatalf("%s direct error = %v", tc.name, err)
			}

			viaHandle := boundTransport.Last(t)
			direct := unboundTransport.Last(t)
			if viaHandle.Method != direct.Method || viaHandle.URL.String() != direct.URL.String() {
				t.Errorf("%s: handle sent %s %s, direct sent %s %s",
					tc.name, viaHandle.Method, viaHandle.URL, direct.Method, direct.URL)
			}
			if string(viaHandle.Body) != string(direct.Body) {
				t.Errorf("%s: handle body = %q, direct body = %q", tc.name, viaHandle.Body, direct.Body)
			}
		}
	})
}
