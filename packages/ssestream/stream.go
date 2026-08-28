// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package ssestream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// Frame types the server uses for stream control rather than for payload.
const (
	// eventPing keeps a connection alive through proxies that drop idle
	// sockets. It carries no payload and is not a stream item.
	eventPing = "ping"

	// eventError reports a failure that happened after the response headers
	// were already sent, so it cannot be expressed as an HTTP status.
	eventError = "error"
)

// Stream is a typed sequence of events decoded from a Server-Sent Events
// response.
//
// Use it with the same shape as a [bufio.Scanner], and always close it:
//
//	stream := client.Sessions.Events.Stream(ctx, sessionID)
//	defer stream.Close()
//	for stream.Next() {
//		event := stream.Current()
//	}
//	if err := stream.Err(); err != nil { ... }
//
// Breaking out of the loop is safe - Close aborts the underlying request rather
// than leaving it running to completion.
type Stream[T any] struct {
	ctx     context.Context
	decoder *Decoder
	body    io.Closer
	current T
	err     error
	done    bool
}

// NewStream returns a Stream reading events from body.
//
// The stream takes ownership of body: closing the stream closes it. Passing a
// non-nil err produces a stream that yields nothing and reports that error, so
// a caller can hand a failed request straight through without a nil check at
// every call site.
func NewStream[T any](ctx context.Context, body io.ReadCloser, err error) *Stream[T] {
	if err != nil {
		return &Stream[T]{ctx: ctx, err: err, done: true}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &Stream[T]{
		ctx:     ctx,
		decoder: NewDecoder(bufio.NewReader(body)),
		body:    body,
	}
}

// Next advances to the next event, reporting whether there was one.
//
// Ping frames are skipped: they exist to keep the connection open and are not
// events the caller asked for. An error frame ends the stream and is reported
// through Err, because a failure the server sends mid-stream is still a
// failure - transcoding it as data would leave a consumer unable to tell a
// broken stream from a complete one.
func (s *Stream[T]) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	for {
		// Checked before each frame so a cancelled caller stops promptly rather
		// than at whatever point the server next sends something.
		if err := s.ctx.Err(); err != nil {
			s.err = err
			s.done = true
			return false
		}

		if !s.decoder.Next() {
			s.err = s.decoder.Err()
			s.done = true
			return false
		}

		event := s.decoder.Event()

		switch event.Type {
		case eventPing:
			continue
		case eventError:
			s.err = streamError(event)
			s.done = true
			return false
		}

		if !event.HasData {
			// A sentinel frame with no payload carries no value to decode.
			continue
		}

		var value T
		if err := json.Unmarshal(event.Data, &value); err != nil {
			s.err = fmt.Errorf("failed to decode stream event %q: %w", event.Data, err)
			s.done = true
			return false
		}
		s.current = value
		return true
	}
}

// Current returns the event Next advanced to.
func (s *Stream[T]) Current() T { return s.current }

// Err returns the first error that ended the stream, if any. A stream that
// reached the end of its events reports nil.
func (s *Stream[T]) Err() error { return s.err }

// Close releases the underlying response. It is safe to call more than once,
// and safe to call after breaking out of an iteration.
func (s *Stream[T]) Close() error {
	s.done = true
	if s.body == nil {
		return nil
	}
	body := s.body
	s.body = nil
	return body.Close()
}

// streamError turns an error frame into an error, preferring the server's own
// message when the payload carries one.
func streamError(event Event) error {
	if !event.HasData || len(event.Data) == 0 {
		return fmt.Errorf("stream failed: server sent an error event with no payload")
	}

	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(event.Data, &envelope) == nil {
		if envelope.Error.Message != "" {
			if envelope.Error.Type != "" {
				return fmt.Errorf("stream failed: %s: %s", envelope.Error.Type, envelope.Error.Message)
			}
			return fmt.Errorf("stream failed: %s", envelope.Error.Message)
		}
		if envelope.Message != "" {
			return fmt.Errorf("stream failed: %s", envelope.Message)
		}
	}
	return fmt.Errorf("stream failed: %s", event.Data)
}
