// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package ssestream

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

// chunkReader hands out one chunk per Read, so a frame can be split at any byte
// boundary. Real streams arrive this way, and a decoder that only works when a
// whole frame lands in one buffer works only in tests.
type chunkReader struct {
	chunks []string
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[0])
	if n < len(r.chunks[0]) {
		r.chunks[0] = r.chunks[0][n:]
		return n, nil
	}
	r.chunks = r.chunks[1:]
	return n, nil
}

func decodeAll(t *testing.T, r io.Reader) ([]Event, error) {
	t.Helper()
	decoder := NewDecoder(bufio.NewReader(r))
	var events []Event
	for decoder.Next() {
		events = append(events, decoder.Event())
	}
	return events, decoder.Err()
}

func TestDecoderSplitsAcrossChunkBoundaries(t *testing.T) {
	t.Parallel()

	// A CR is the one byte that cannot be interpreted on its own: it ends the
	// line unless the next byte is LF, in which case they end it together. When
	// the chunk stops between them, the decoder has to wait for the byte that
	// decides rather than guessing - guessing wrong invents an extra blank
	// line, which dispatches a frame early and splits one event into two.
	tests := []struct {
		name   string
		chunks []string
	}{
		{name: "CRLF split across chunks", chunks: []string{"data: a\r", "\n\r\n"}},
		{name: "one byte at a time", chunks: strings.Split("data: a\r\n\r\n", "")},
		{name: "frame split mid-field", chunks: []string{"da", "ta: ", "a\n", "\n"}},
		{name: "blank line split from its frame", chunks: []string{"data: a\n", "\n"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			events, err := decodeAll(t, &chunkReader{chunks: tc.chunks})
			if err != nil {
				t.Fatalf("decode error = %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("decoded %d events, want 1: %+v", len(events), events)
			}
			if got := string(events[0].Data); got != "a" {
				t.Errorf("data = %q, want %q", got, "a")
			}
		})
	}
}

func TestDecoderFieldParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Event
	}{
		{
			name:  "exactly one leading space is stripped",
			input: "data:  padded\n\n",
			want:  Event{Data: []byte(" padded"), HasData: true},
		},
		{
			name:  "a field with no colon has an empty value",
			input: "data\n\n",
			want:  Event{Data: []byte(""), HasData: true},
		},
		{
			name:  "comments are ignored",
			input: ": keep-alive\ndata: x\n\n",
			want:  Event{Data: []byte("x"), HasData: true},
		},
		{
			name:  "unknown fields are ignored",
			input: "banana: yes\ndata: x\n\n",
			want:  Event{Data: []byte("x"), HasData: true},
		},
		{
			name:  "multiple data lines join with newlines",
			input: "data: a\ndata: b\n\n",
			want:  Event{Data: []byte("a\nb"), HasData: true},
		},
		{
			name:  "a data-less frame reports no payload",
			input: "event: done\n\n",
			want:  Event{Type: "done"},
		},
		{
			name:  "digits-only retry is accepted",
			input: "retry: 3000\n\n",
			want:  Event{Retry: 3000, HasRetry: true},
		},
		{
			name:  "a signed retry is ignored",
			input: "retry: -5\ndata: x\n\n",
			want:  Event{Data: []byte("x"), HasData: true},
		},
		{
			name:  "an id containing NUL is ignored",
			input: "id: a\x00b\ndata: x\n\n",
			want:  Event{Data: []byte("x"), HasData: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			events, err := decodeAll(t, strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("decode error = %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("decoded %d events, want 1: %+v", len(events), events)
			}
			got := events[0]
			if got.Type != tc.want.Type || got.ID != tc.want.ID ||
				got.Retry != tc.want.Retry || got.HasRetry != tc.want.HasRetry ||
				got.HasData != tc.want.HasData || string(got.Data) != string(tc.want.Data) {
				t.Errorf("event = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDecoderDiscardsUnterminatedFrame(t *testing.T) {
	t.Parallel()

	// Without a terminating blank line the frame is incomplete, and a consumer
	// has no way to tell a truncated stream from a complete one if it is
	// dispatched anyway.
	events, err := decodeAll(t, strings.NewReader("data: a\n\ndata: b\n"))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("decoded %d events, want 1: %+v", len(events), events)
	}
	if got := string(events[0].Data); got != "a" {
		t.Errorf("data = %q, want %q", got, "a")
	}
}

func TestDecoderEventDataIsNotAliased(t *testing.T) {
	t.Parallel()

	// The payload buffer is reused between frames, so a caller holding on to
	// an earlier event must not see it rewritten by a later one.
	events, err := decodeAll(t, strings.NewReader("data: first\n\ndata: second\n\n"))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("decoded %d events, want 2", len(events))
	}
	if got := string(events[0].Data); got != "first" {
		t.Errorf("first event data = %q, want %q - it was overwritten by the next frame", got, "first")
	}
}
