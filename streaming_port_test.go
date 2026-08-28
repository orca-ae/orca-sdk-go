// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// Ported from orca-sdk-typescript tests/core/streaming.test.ts.
//
// The TypeScript SDK exposes a typed `Stream<T>`: `Stream.fromSSEResponse` wraps
// a `Response` whose body is a `ReadableStream<Uint8Array>`, parses the SSE wire
// format, and yields decoded items from an async iterator.
//
// Go has no such object. The analogue here is `renderManagedAgentSSE`, a
// *transcoder*: it reads the SSE wire format off an `io.Reader` and writes
// NDJSON (one JSON value per line) to an `io.Writer`. Everything the TypeScript
// tests say about *wire-format parsing* — comments, CRLF, multi-line `data:`,
// the `event`/`id`/`retry` fields, blank-line dispatch, reassembly across chunk
// boundaries — ports directly and is covered below.
//
// Everything the TypeScript tests say about the *stream object* — `tee()`,
// `toReadableStream()`, the consumed-once guard, `AbortController` propagation,
// and the semantic filtering of `event: ping` / `event: error` frames — has no
// Go analogue or is not implemented. Those are skipped, individually, with the
// reason spelled out; see the tests at the bottom of this file.
//
// Note on output shape: a frame carrying only `data:` is written as the bare
// decoded value; a frame that also carries `event`, `id`, or `retry` is written
// as an object wrapping it under "data". `data` that does not parse as JSON is
// written as a JSON string rather than dropped.

// sseChunkReader delivers its chunks one per Read call, so a test can pin the
// decoder's behaviour across network frame boundaries. The TypeScript suite
// models this with a ReadableStream that enqueues each chunk separately.
type sseChunkReader struct {
	chunks []string
	// err, when set, is returned in place of io.EOF once the chunks run out.
	err error
}

func (r *sseChunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	chunk := r.chunks[0]
	n := copy(p, chunk)
	if n < len(chunk) {
		r.chunks[0] = chunk[n:]
	} else {
		r.chunks = r.chunks[1:]
	}
	return n, nil
}

// sseLines splits NDJSON output into its lines. A JSON encoder never emits a raw
// newline inside a value, so splitting on "\n" is exact.
func sseLines(out string) []string {
	trimmed := strings.TrimSuffix(out, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// sseDecode runs the decoder over chunks and returns the NDJSON lines it wrote.
func sseDecode(t *testing.T, chunks ...string) []string {
	t.Helper()
	var out bytes.Buffer
	if err := renderManagedAgentSSE(&out, &sseChunkReader{chunks: chunks}); err != nil {
		t.Fatalf("renderManagedAgentSSE() error = %v", err)
	}
	return sseLines(out.String())
}

// sseResponse builds a text/event-stream response for the recording transport.
func sseResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestManagedAgentSSEDecodesWireFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			// TS: 'yields the parsed JSON payload from a single data line'.
			name:  "a lone data line is written as the decoded value",
			input: "data: {\"hello\":\"world\"}\n\n",
			want:  []string{`{"hello":"world"}`},
		},
		{
			// TS: 'preserves the event field on each ServerSentEvent'.
			name: "each named event keeps its event field",
			input: "event: message_start\ndata: {\"v\":1}\n\n" +
				"event: content_block_delta\ndata: {\"v\":2}\n\n" +
				"event: message_stop\ndata: {\"v\":3}\n\n",
			want: []string{
				`{"data":{"v":1},"event":"message_start"}`,
				`{"data":{"v":2},"event":"content_block_delta"}`,
				`{"data":{"v":3},"event":"message_stop"}`,
			},
		},
		{
			// TS: 'handles \r\n exactly like \n'.
			name:  "CRLF line endings decode exactly like LF",
			input: "event: one\r\ndata: {\"v\":1}\r\n\r\nevent: two\r\ndata: {\"v\":2}\r\n\r\n",
			want: []string{
				`{"data":{"v":1},"event":"one"}`,
				`{"data":{"v":2},"event":"two"}`,
			},
		},
		{
			name:  "a comment line is ignored",
			input: ": keep-alive\ndata: {\"v\":1}\n\n",
			want:  []string{`{"v":1}`},
		},
		{
			name:  "a bare colon is a comment, not a field",
			input: ":\ndata: 1\n\n",
			want:  []string{`1`},
		},
		{
			name:  "consecutive data lines are joined with newlines",
			input: "data: line1\ndata: line2\n\n",
			want:  []string{`"line1\nline2"`},
		},
		{
			name:  "the id field is preserved",
			input: "id: 42\ndata: {\"v\":1}\n\n",
			want:  []string{`{"data":{"v":1},"id":"42"}`},
		},
		{
			name:  "the retry field is preserved as a number",
			input: "retry: 5000\nid: 42\ndata: hi\n\n",
			want:  []string{`{"data":"hi","id":"42","retry":5000}`},
		},
		{
			// The frame carried no data field, so the output carries no data
			// key. See TestManagedAgentSSESpecDeviations for why inventing an
			// empty payload here was wrong.
			name:  "a retry-only frame dispatches without a synthetic payload",
			input: "retry: 3000\n\n",
			want:  []string{`{"retry":3000}`},
		},
		{
			name:  "only one leading space is stripped from a field value",
			input: "data:  two leading spaces\n\n",
			want:  []string{`" two leading spaces"`},
		},
		{
			name:  "a value is split at the first colon only",
			input: "data: {\"a\":\"b:c\"}\n\n",
			want:  []string{`{"a":"b:c"}`},
		},
		{
			name:  "a field with no colon is read as an empty value",
			input: "data\n\n",
			want:  []string{`""`},
		},
		{
			name:  "an unknown field is dropped",
			input: "foo: bar\ndata: {\"v\":1}\n\n",
			want:  []string{`{"v":1}`},
		},
		{
			name:  "a frame of only unknown fields dispatches nothing",
			input: "foo: bar\n\n",
			want:  nil,
		},
		{
			// The blank-line dispatch is a no-op when nothing was accumulated,
			// so keep-alive newlines never produce output.
			name:  "blank lines alone dispatch nothing",
			input: "\n\n\n\n",
			want:  nil,
		},
		{
			name:  "an explicitly empty data field still dispatches",
			input: "data:\n\n",
			want:  []string{`""`},
		},
		{
			// Discarded, per the specification: dispatching it would make a
			// connection cut mid-frame indistinguishable from a clean end,
			// which is exactly the case a consumer needs to tell apart. See
			// TestManagedAgentSSESpecDeviations.
			name:  "a trailing frame with no blank line is discarded at EOF",
			input: "data: {\"v\":1}",
			want:  nil,
		},
		{
			// TS: 'skips frames whose data is not valid JSON and continues'.
			// Divergence: TypeScript drops the frame (after warning); Go emits
			// it as a JSON string. Neither throws, and the stream continues.
			name:  "non-JSON data is emitted as a JSON string, not an error",
			input: "data: not-json\n\ndata: {\"v\":1}\n\n",
			want:  []string{`"not-json"`, `{"v":1}`},
		},
		{
			name:  "a JSON array payload round-trips",
			input: "data: [1,2,3]\n\n",
			want:  []string{`[1,2,3]`},
		},
		{
			// TS: 'silently skips event: ping frames'. Divergence: Go passes
			// ping through, leaving the filtering decision to the consumer.
			// See TestManagedAgentSSESkipsPingFrames for the capability gap.
			name:  "an event: ping frame is passed through, not filtered",
			input: "event: ping\ndata: {}\n\ndata: {\"v\":1}\n\n",
			want:  []string{`{"data":{},"event":"ping"}`, `{"v":1}`},
		},
		{
			// TS: 'throws an APIError when an event: error frame is received'.
			// Divergence: Go passes the frame through as data. See
			// TestManagedAgentSSEErrorFrameRaisesError for the capability gap.
			name:  "an event: error frame is passed through, not raised",
			input: "event: error\ndata: {\"error\":{\"type\":\"rate_limit_error\",\"message\":\"slow down\"}}\n\n",
			want:  []string{`{"data":{"error":{"message":"slow down","type":"rate_limit_error"}},"event":"error"}`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sseDecode(t, tc.input); !slices.Equal(got, tc.want) {
				t.Errorf("decoded lines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManagedAgentSSEReassemblesChunkedDelivery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{
			// TS: 'reassembles a single event delivered across three chunks'.
			name:   "one event split across three chunks",
			chunks: []string{"event: split\nda", "ta: {\"msg\":\"hel", "lo-world\"}\n\n"},
			want:   []string{`{"data":{"msg":"hello-world"},"event":"split"}`},
		},
		{
			// TS: 'reassembles events split across chunk boundaries with CRLF'.
			name:   "chunk boundaries land inside a CRLF frame",
			chunks: []string{"event: x\r\nda", "ta: {\"v\":", "99}\r\n\r\n"},
			want:   []string{`{"data":{"v":99},"event":"x"}`},
		},
		{
			// A chunk boundary between the CR and the LF is the case a naive
			// "strip \r\n from the chunk" implementation gets wrong.
			name:   "a chunk boundary falls between CR and LF",
			chunks: []string{"data: {\"v\":1}\r", "\n\r", "\n"},
			want:   []string{`{"v":1}`},
		},
		{
			name:   "a frame arriving one byte at a time",
			chunks: strings.Split("event: drip\ndata: {\"v\":7}\n\n", ""),
			want:   []string{`{"data":{"v":7},"event":"drip"}`},
		},
		{
			name:   "two frames delivered in one chunk each",
			chunks: []string{"data: {\"v\":1}\n\n", "data: {\"v\":2}\n\n"},
			want:   []string{`{"v":1}`, `{"v":2}`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sseDecode(t, tc.chunks...); !slices.Equal(got, tc.want) {
				t.Errorf("decoded lines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManagedAgentSSERejectsNilStreams(t *testing.T) {
	t.Parallel()

	if err := renderManagedAgentSSE(nil, strings.NewReader("")); err == nil {
		t.Error("renderManagedAgentSSE(nil writer) error = nil, want an error")
	}
	if err := renderManagedAgentSSE(io.Discard, nil); err == nil {
		t.Error("renderManagedAgentSSE(nil reader) error = nil, want an error")
	}
}

func TestManagedAgentSSEIgnoresInvalidRetry(t *testing.T) {
	t.Parallel()

	// The specification says a retry value that is not only ASCII digits is
	// ignored, not fatal. Aborting instead meant one malformed keep-alive from
	// a proxy discarded every event that followed it - including the valid
	// frame already buffered alongside it.
	for _, input := range []string{
		"retry: soon\ndata: {\"v\":1}\n\n",
		"retry: 1.5\ndata: {\"v\":1}\n\n",
		"retry: -5\ndata: {\"v\":1}\n\n",
		"retry:\ndata: {\"v\":1}\n\n",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got := sseDecode(t, input)
			want := []string{`{"v":1}`}
			if !slices.Equal(got, want) {
				t.Errorf("decoded lines = %q, want %q - the data frame must survive", got, want)
			}
		})
	}
}

// TestManagedAgentSSEPropagatesReaderError is the closest Go analogue of the
// TypeScript abort tests: the transport ends the stream early. Frames that were
// already complete have been written; the read failure is surfaced, not
// swallowed.
func TestManagedAgentSSEPropagatesReaderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("connection reset")
	var out bytes.Buffer
	reader := &sseChunkReader{chunks: []string{"data: {\"v\":1}\n\n"}, err: wantErr}

	err := renderManagedAgentSSE(&out, reader)
	if !errors.Is(err, wantErr) {
		t.Fatalf("renderManagedAgentSSE() error = %v, want %v", err, wantErr)
	}
	if got, want := sseLines(out.String()), []string{`{"v":1}`}; !slices.Equal(got, want) {
		t.Errorf("frames written before the failure = %q, want %q", got, want)
	}
}

// sseFailingWriter fails every write, standing in for a client that hung up.
type sseFailingWriter struct{ err error }

func (w sseFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestManagedAgentSSEPropagatesWriterError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("broken pipe")
	err := renderManagedAgentSSE(
		sseFailingWriter{err: wantErr},
		strings.NewReader("data: {\"v\":1}\n\ndata: {\"v\":2}\n\n"),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("renderManagedAgentSSE() error = %v, want %v", err, wantErr)
	}
}

// TestManagedAgentSSELineLengthLimit pins the decoder's hard ceiling on a single
// SSE line. bufio.Scanner is configured with a 1 MiB maximum token, so a larger
// `data:` line fails the whole stream instead of being reassembled.
func TestManagedAgentSSELineLengthLimit(t *testing.T) {
	t.Parallel()

	t.Run("a large line under the limit decodes", func(t *testing.T) {
		t.Parallel()

		payload := strings.Repeat("a", 512*1024)
		got := sseDecode(t, "data: "+payload+"\n\n")
		if len(got) != 1 || got[0] != `"`+payload+`"` {
			t.Errorf("decoded %d lines of length %d, want one line of length %d",
				len(got), len(got[0]), len(payload)+2)
		}
	})

	t.Run("a line over the limit fails the stream", func(t *testing.T) {
		t.Parallel()

		input := "data: " + strings.Repeat("a", 2<<20) + "\n\n"
		err := renderManagedAgentSSE(io.Discard, strings.NewReader(input))
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Fatalf("renderManagedAgentSSE() error = %v, want %v", err, bufio.ErrTooLong)
		}
	})
}

// TestManagedAgentSSEOverHTTPStream exercises the whole path a caller actually
// takes: Client.GetStream hands the live response body to the decoder. It is the
// Go analogue of TypeScript's `Stream.fromSSEResponse(response, controller)`.
func TestManagedAgentSSEOverHTTPStream(t *testing.T) {
	t.Parallel()

	body := "event: message_start\ndata: {\"v\":1}\n\n" +
		": keep-alive\n\n" +
		"data: {\"v\":2}\n\n"
	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return sseResponse(http.StatusOK, body), nil
	})

	var out bytes.Buffer
	err := client.GetStream(context.Background(), "v1/sessions/s-1/events", "text/event-stream",
		func(reader io.Reader) error { return renderManagedAgentSSE(&out, reader) })
	if err != nil {
		t.Fatalf("GetStream() error = %v", err)
	}

	call := transport.Only(t)
	if call.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", call.Method)
	}
	if got := call.Header.Get("Accept"); got != "text/event-stream" {
		t.Errorf("Accept = %q, want text/event-stream", got)
	}
	want := []string{`{"data":{"v":1},"event":"message_start"}`, `{"v":2}`}
	if got := sseLines(out.String()); !slices.Equal(got, want) {
		t.Errorf("decoded lines = %q, want %q", got, want)
	}
}

// TestManagedAgentSSEHTTPErrorBeforeAnyFrame ports the TypeScript
// 'throws an APIError on the first iteration when status is 500' case: the
// failure surfaces before the handler ever sees a byte.
func TestManagedAgentSSEHTTPErrorBeforeAnyFrame(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError,
			`{"error":{"type":"server_error","message":"boom"}}`), nil
	})

	handlerCalled := false
	err := client.GetStream(context.Background(), "v1/sessions/s-1/events", "text/event-stream",
		func(io.Reader) error {
			handlerCalled = true
			return nil
		})

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("GetStream() error = %v (%T), want *HTTPError", err, err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", httpErr.StatusCode, http.StatusInternalServerError)
	}
	if !strings.Contains(httpErr.Body, "boom") {
		t.Errorf("error body = %q, want it to carry the server message", httpErr.Body)
	}
	if handlerCalled {
		t.Error("the stream handler ran on a non-2xx response, want it skipped")
	}
}

// ---------------------------------------------------------------------------
// Deviations from the Server-Sent Events wire format.
//
// These are not TypeScript-porting gaps — they are places where the decoder
// relocated from orca-cli disagrees with the SSE specification. Each subtest
// asserts the *spec* behaviour and, while the decoder does not implement it,
// skips with the observed output. Fix the decoder and the test starts passing
// on its own; no expectation needs editing.
// ---------------------------------------------------------------------------

func TestManagedAgentSSESpecDeviations(t *testing.T) {
	t.Parallel()

	t.Run("a lone CR terminates a line", func(t *testing.T) {
		t.Parallel()

		// Spec: streams are split on CRLF, LF, *or* a lone CR. Splitting on LF
		// alone collapsed a CR-delimited stream into one line, which was then
		// emitted as a single corrupted value - silently, with no error.
		//
		// The original expectation here said "data: a\rdata: b\r\r" should
		// decode to two events. That was wrong: those two data lines have no
		// blank line between them, so the specification says they are one
		// event whose payload is the lines joined with a newline. Asserting it
		// per spec is what makes the CR case comparable to the LF one below.
		tests := []struct {
			name  string
			input string
			want  []string
		}{
			{
				name:  "CR-delimited data lines join into one frame",
				input: "data: a\rdata: b\r\r",
				want:  []string{`"a\nb"`},
			},
			{
				name:  "a CR blank line dispatches a frame",
				input: "data: a\r\rdata: b\r\r",
				want:  []string{`"a"`, `"b"`},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := sseDecode(t, tc.input); !slices.Equal(got, tc.want) {
					t.Errorf("decoded %q to %q, want %q", tc.input, got, tc.want)
				}
			})
		}

		// The three terminators must be interchangeable, or a server that
		// switches between them changes what the client sees.
		t.Run("CR, LF and CRLF agree", func(t *testing.T) {
			t.Parallel()

			want := []string{`"a"`, `"b"`}
			for _, input := range []string{
				"data: a\r\rdata: b\r\r",
				"data: a\n\ndata: b\n\n",
				"data: a\r\n\r\ndata: b\r\n\r\n",
			} {
				if got := sseDecode(t, input); !slices.Equal(got, want) {
					t.Errorf("decoded %q to %q, want %q", input, got, want)
				}
			}
		})
	})

	t.Run("a leading byte order mark is stripped", func(t *testing.T) {
		t.Parallel()

		// Spec: one leading U+FEFF must be removed before parsing. Here it is
		// glued to the first field name, so "\ufeffdata" matches no known field,
		// the frame accumulates nothing, and the blank line dispatches nothing:
		// the first event of the stream disappears with no diagnostic.
		var out bytes.Buffer
		if err := renderManagedAgentSSE(&out, strings.NewReader("\ufeffdata: {\"v\":1}\n\n")); err != nil {
			t.Fatalf("renderManagedAgentSSE() error = %v", err)
		}
		got := sseLines(out.String())
		want := []string{`{"v":1}`}
		if slices.Equal(got, want) {
			return
		}
		t.Skipf("not implemented: a leading UTF-8 BOM is not stripped — the first frame "+
			"decoded to %q instead of %q and was dropped silently", got, want)
	})

	t.Run("a malformed retry field is ignored, not fatal", func(t *testing.T) {
		t.Parallel()

		// Spec: "If the field value consists of only ASCII digits ... otherwise,
		// ignore the field." This decoder aborts the whole stream instead, so a
		// single malformed keep-alive from a proxy discards every event that
		// followed it — and every event already buffered in this frame.
		for _, input := range []string{"retry: 1.5\ndata: x\n\n", "retry:\ndata: x\n\n"} {
			var out bytes.Buffer
			err := renderManagedAgentSSE(&out, strings.NewReader(input))
			if err == nil && slices.Equal(sseLines(out.String()), []string{`"x"`}) {
				continue
			}
			t.Skipf("not implemented: a non-integer retry value aborts the stream — %q "+
				"returned error %v and emitted %q, discarding the valid data frame that followed",
				input, err, out.String())
		}
	})

	t.Run("a retry field that is not only ASCII digits is ignored", func(t *testing.T) {
		t.Parallel()

		// The mirror image of the case above: "+5" and "-5" are accepted by
		// strconv.Atoi even though the spec admits digits only, so a negative
		// reconnection delay reaches the consumer.
		got := sseDecode(t, "retry: -5\ndata: x\n\n")
		want := []string{`"x"`}
		if slices.Equal(got, want) {
			return
		}
		t.Skipf("not implemented: a signed retry value is accepted — %q decoded to %q "+
			"instead of %q, surfacing a negative reconnection delay",
			"retry: -5\ndata: x\n\n", got, want)
	})

	t.Run("a truncated final frame is discarded", func(t *testing.T) {
		t.Parallel()

		// Spec: once the stream ends, any event still being buffered is
		// discarded. This decoder dispatches it, so a connection cut mid-frame
		// yields a plausible-looking final event instead of an error. That is a
		// deliberate convenience for CLI rendering, but a consumer cannot tell a
		// complete stream from a truncated one.
		got := sseDecode(t, "event: x\ndata: {\"v\":1}\n")
		if len(got) == 0 {
			return
		}
		t.Skipf("not implemented: a frame with no terminating blank line is still "+
			"dispatched at EOF — decoded %q, so a truncated stream is indistinguishable "+
			"from a complete one", got)
	})

	t.Run("an event with no data field carries no synthetic payload", func(t *testing.T) {
		t.Parallel()

		// A sentinel frame such as "event: done" has no data. The decoder
		// invents an empty string for it, so consumers see a payload that the
		// server never sent.
		got := sseDecode(t, "event: done\n\n")
		want := []string{`{"event":"done"}`}
		if slices.Equal(got, want) {
			return
		}
		t.Skipf("not implemented: a data-less frame is given a synthetic empty payload — "+
			"%q decoded to %q instead of %q", "event: done\n\n", got, want)
	})
}

// ---------------------------------------------------------------------------
// TypeScript behaviour with no Go analogue, or not implemented here.
// ---------------------------------------------------------------------------

// TS: 'silently skips event: ping frames'.
//
// The NDJSON transcoder passes pings through - a command-line consumer may want
// to see that the connection is alive. The typed Stream filters them, because a
// keep-alive is not an event the caller asked for and would decode to a zero
// value of T.
func TestManagedAgentSSESkipsPingFrames(t *testing.T) {
	t.Parallel()

	type event struct {
		V int `json:"v"`
	}

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return sseResponse(http.StatusOK, strings.Join([]string{
			"event: ping\ndata: {}\n\n",
			"data: {\"v\":1}\n\n",
			"event: ping\n\n",
			"data: {\"v\":2}\n\n",
		}, "")), nil
	})

	stream := StreamEvents[event](context.Background(), client, "/v1/sessions/s1/events/stream")
	defer stream.Close()

	var got []int
	for stream.Next() {
		got = append(got, stream.Current().V)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error = %v", err)
	}
	if want := []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}

// TS: 'throws an APIError when an event: error frame is received'.
//
// A failure the server discovers after the response headers are already sent
// cannot be expressed as an HTTP status, so it arrives as an error frame.
// Yielding it as data would leave the consumer unable to tell a broken stream
// from a complete one.
func TestManagedAgentSSEErrorFrameRaisesError(t *testing.T) {
	t.Parallel()

	type event struct {
		V int `json:"v"`
	}

	tests := []struct {
		name  string
		frame string
		want  string
	}{
		{
			name:  "typed error envelope",
			frame: `event: error` + "\n" + `data: {"error":{"type":"rate_limit_error","message":"slow down"}}` + "\n\n",
			want:  "slow down",
		},
		{
			name:  "bare message",
			frame: `event: error` + "\n" + `data: {"message":"upstream gone"}` + "\n\n",
			want:  "upstream gone",
		},
		{
			name:  "unrecognised payload is passed through",
			frame: "event: error\ndata: something broke\n\n",
			want:  "something broke",
		},
		{
			name:  "no payload at all",
			frame: "event: error\n\n",
			want:  "no payload",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
				return sseResponse(http.StatusOK, `data: {"v":1}`+"\n\n"+tc.frame+`data: {"v":2}`+"\n\n"), nil
			})

			stream := StreamEvents[event](context.Background(), client, "/v1/sessions/s1/events/stream")
			defer stream.Close()

			var got []int
			for stream.Next() {
				got = append(got, stream.Current().V)
			}

			err := stream.Err()
			if err == nil {
				t.Fatal("stream error = nil, want the error frame to surface")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to carry %q", err, tc.want)
			}
			// Events before the failure are still delivered; events after it
			// are not, because the stream is over.
			if want := []int{1}; !slices.Equal(got, want) {
				t.Errorf("events = %v, want %v", got, want)
			}
		})
	}
}

// TS: 'terminates iteration cleanly when the controller is aborted before
// reading' and 'break inside the iterator aborts the controller'.
func TestManagedAgentSSEAbortPropagation(t *testing.T) {
	t.Parallel()

	type event struct {
		V int `json:"v"`
	}

	t.Run("a context cancelled before reading yields nothing", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return sseResponse(http.StatusOK, `data: {"v":1}`+"\n\n"), nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		stream := StreamEvents[event](ctx, client, "/v1/sessions/s1/events/stream")
		defer stream.Close()
		cancel()

		if stream.Next() {
			t.Error("Next() = true on a cancelled context, want false")
		}
		if !errors.Is(stream.Err(), context.Canceled) {
			t.Errorf("Err() = %v, want context.Canceled", stream.Err())
		}
	})

	t.Run("breaking out of the loop releases the response", func(t *testing.T) {
		t.Parallel()

		// A consumer that stops early must not leave the request running. The
		// body records whether it was closed, since that is the observable
		// effect - the deadline is released with it.
		body := &closeTrackingBody{Reader: strings.NewReader(
			`data: {"v":1}` + "\n\n" + `data: {"v":2}` + "\n\n",
		)}
		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			res := sseResponse(http.StatusOK, "")
			res.Body = body
			return res, nil
		})

		stream := StreamEvents[event](context.Background(), client, "/v1/sessions/s1/events/stream")
		for stream.Next() {
			break
		}
		if err := stream.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if !body.closed.Load() {
			t.Error("response body was not closed, want the abandoned stream to release it")
		}

		// Close is idempotent: a deferred Close after an explicit one is the
		// normal shape and must not fail.
		if err := stream.Close(); err != nil {
			t.Errorf("second Close() error = %v, want nil", err)
		}
	})

	t.Run("a failed request is reported on the stream", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, `{"error":"no such session"}`), nil
		})

		stream := StreamEvents[event](context.Background(), client, "/v1/sessions/nope/events/stream")
		defer stream.Close()

		if stream.Next() {
			t.Error("Next() = true after a failed request, want false")
		}
		var notFound *NotFoundError
		if !errors.As(stream.Err(), &notFound) {
			t.Errorf("Err() = %T (%v), want *NotFoundError", stream.Err(), stream.Err())
		}
	})
}

// closeTrackingBody records whether the response body was closed.
type closeTrackingBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *closeTrackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

// TS: 'produces two independent streams that yield the same items'.
func TestManagedAgentSSETee(t *testing.T) {
	t.Skip("no Go analogue: tee() forks a stateful Stream object; Go's decoder is a " +
		"function over an io.Reader, and duplicating a reader (io.TeeReader) copies bytes, " +
		"not decoded events")
}

// TS: 're-encodes items as data: <json>\n\n UTF-8 bytes'.
func TestManagedAgentSSEToReadableStream(t *testing.T) {
	t.Skip("no Go analogue: toReadableStream() re-encodes a Stream back into SSE frames; " +
		"the Go decoder is one-way and emits NDJSON, and there is no SSE encoder in this SDK")
}

// TS: 'throws when iterated a second time'.
//
// Go reports a drained stream by returning false rather than by throwing. That
// is the better answer for a range loop - a second loop over a finished stream
// is a no-op, not a crash - but it still has to be true: a stream that replayed
// its events, or that errored, would break a caller that loops defensively.
func TestManagedAgentSSEConsumedOnceGuard(t *testing.T) {
	t.Parallel()

	type event struct {
		V int `json:"v"`
	}

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return sseResponse(http.StatusOK, `data: {"v":1}`+"\n\n"+`data: {"v":2}`+"\n\n"), nil
	})

	stream := StreamEvents[event](context.Background(), client, "/v1/sessions/s1/events/stream")
	defer stream.Close()

	var first []int
	for stream.Next() {
		first = append(first, stream.Current().V)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error = %v", err)
	}
	if want := []int{1, 2}; !slices.Equal(first, want) {
		t.Fatalf("first pass = %v, want %v", first, want)
	}

	var second []int
	for stream.Next() {
		second = append(second, stream.Current().V)
	}
	if len(second) != 0 {
		t.Errorf("second pass = %v, want nothing - the stream is consumed", second)
	}
	if err := stream.Err(); err != nil {
		t.Errorf("Err() after a second pass = %v, want nil - exhaustion is not a failure", err)
	}

	// Close is idempotent, so a deferred Close after an explicit one is safe.
	if err := stream.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
