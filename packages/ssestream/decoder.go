// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package ssestream decodes Server-Sent Events.
//
// The wire format looks trivial and is not. Its edge cases - three different
// line terminators, a leading byte order mark, fields that must be ignored
// rather than rejected, a final frame that must be discarded - all fail
// silently when handled wrong: the stream keeps going and the consumer sees
// plausible but incorrect events. That is why this is a package with its own
// tests rather than a loop inside a resource method.
package ssestream

import (
	"bufio"
	"bytes"
)

// maxLineLength bounds a single field line. A stream is attacker-adjacent in
// the sense that the client cannot control how much a server sends before a
// terminator, so an unbounded line is an unbounded allocation.
const maxLineLength = 1 << 20

// byteOrderMark is U+FEFF encoded as UTF-8. Exactly one leading occurrence is
// stripped before parsing; anything more is data.
var byteOrderMark = []byte{0xEF, 0xBB, 0xBF}

// Event is one decoded frame.
type Event struct {
	// Type is the frame's "event" field, empty when it carried none.
	Type string

	// ID is the frame's "id" field, used by a client resuming a stream.
	ID string

	// Retry is the reconnection delay in milliseconds the server suggested.
	// HasRetry reports whether the frame carried one, since zero is a
	// meaningful value.
	Retry    int
	HasRetry bool

	// Data is the frame's payload, with the trailing newline removed. HasData
	// distinguishes a frame that carried an empty data field from one that
	// carried none at all - a sentinel such as "event: done" has no payload,
	// and inventing an empty one for it reports something the server never
	// sent.
	Data    []byte
	HasData bool
}

// IsEmpty reports whether the frame carried no fields at all, which is what a
// run of blank lines produces. Such frames are not dispatched.
func (e Event) IsEmpty() bool {
	return e.Type == "" && e.ID == "" && !e.HasRetry && !e.HasData
}

// Decoder reads frames from a Server-Sent Events stream.
type Decoder struct {
	scanner *bufio.Scanner
	event   Event
	data    bytes.Buffer
	err     error
	first   bool
}

// NewDecoder returns a Decoder reading from r.
func NewDecoder(r *bufio.Reader) *Decoder {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLineLength)
	scanner.Split(scanLines)
	return &Decoder{scanner: scanner, first: true}
}

// Next advances to the next complete frame, reporting whether there was one.
//
// A frame still being buffered when the stream ends is discarded, per the
// specification. Dispatching it instead would make a connection cut mid-frame
// indistinguishable from a clean end, which is precisely the case a consumer
// needs to tell apart.
func (d *Decoder) Next() bool {
	if d.err != nil {
		return false
	}

	for d.scanner.Scan() {
		line := d.scanner.Bytes()

		if d.first {
			d.first = false
			line = bytes.TrimPrefix(line, byteOrderMark)
		}

		if len(line) == 0 {
			if d.event.IsEmpty() {
				// A run of blank lines dispatches nothing.
				d.reset()
				continue
			}
			d.finishData()
			return true
		}

		// A line beginning with a colon is a comment, used to keep connections
		// alive through proxies that time out idle sockets.
		if line[0] == ':' {
			continue
		}

		d.consumeField(line)
	}

	if err := d.scanner.Err(); err != nil {
		d.err = err
	}
	d.reset()
	return false
}

func (d *Decoder) consumeField(line []byte) {
	field, value, found := bytes.Cut(line, []byte(":"))
	if !found {
		// A line with no colon is a field name with an empty value.
		value = nil
	}
	// Exactly one leading space is part of the framing, not the value.
	value = bytes.TrimPrefix(value, []byte(" "))

	switch string(field) {
	case "event":
		d.event.Type = string(value)
	case "id":
		// A NUL in an id is required to be ignored rather than stored.
		if !bytes.ContainsRune(value, 0) {
			d.event.ID = string(value)
		}
	case "retry":
		// Digits only. A value like "1.5", "-5" or "" is ignored rather than
		// fatal: a single malformed keep-alive from a proxy must not discard
		// every event that follows it.
		if retry, ok := parseRetry(value); ok {
			d.event.Retry, d.event.HasRetry = retry, true
		}
	case "data":
		d.data.Write(value)
		d.data.WriteByte('\n')
		d.event.HasData = true
	}
}

// parseRetry accepts only a non-empty run of ASCII digits, so a signed or
// fractional value is rejected rather than reaching the consumer as a negative
// reconnection delay.
func parseRetry(value []byte) (int, bool) {
	if len(value) == 0 {
		return 0, false
	}
	retry := 0
	for _, b := range value {
		if b < '0' || b > '9' {
			return 0, false
		}
		retry = retry*10 + int(b-'0')
		if retry > 1<<30 {
			return 0, false
		}
	}
	return retry, true
}

func (d *Decoder) finishData() {
	data := d.data.Bytes()
	d.event.Data = bytes.TrimSuffix(data, []byte("\n"))
}

func (d *Decoder) reset() {
	d.event = Event{}
	d.data.Reset()
}

// Event returns the frame Next advanced to.
func (d *Decoder) Event() Event {
	event := d.event
	// Hand out a copy of the payload: the buffer is reused for the next frame.
	if event.HasData {
		event.Data = append([]byte(nil), event.Data...)
	}
	d.reset()
	return event
}

// Err returns the first error that stopped decoding, if any.
func (d *Decoder) Err() error { return d.err }

// scanLines splits on CRLF, LF, or a lone CR.
//
// bufio.ScanLines handles only the first two, which turns a CR-delimited stream
// into one enormous line that is then emitted as a single corrupted value - no
// error, no diagnostic, just wrong data.
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			return i + 1, data[:i], nil
		case '\r':
			if i+1 < len(data) {
				if data[i+1] == '\n' {
					return i + 2, data[:i], nil
				}
				return i + 1, data[:i], nil
			}
			if atEOF {
				return i + 1, data[:i], nil
			}
			// A trailing CR could still turn out to be the first half of a
			// CRLF, so wait for the byte that decides it.
			return 0, nil, nil
		}
	}

	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
