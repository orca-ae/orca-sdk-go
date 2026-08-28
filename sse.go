// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/packages/ssestream"
)

// renderManagedAgentSSE transcodes a Server-Sent Events stream to NDJSON: one
// JSON value per line.
//
// The wire-format parsing lives in packages/ssestream; this is only the
// rendering half. Keeping them apart is what let the parser be fixed against
// the specification without touching the output shape command-line callers
// depend on.
func renderManagedAgentSSE(writer io.Writer, reader io.Reader) error {
	if writer == nil {
		return apierror.Validationf("response writer is required")
	}
	if reader == nil {
		return apierror.Validationf("event stream reader is required")
	}

	decoder := ssestream.NewDecoder(bufio.NewReader(reader))
	for decoder.Next() {
		if err := writeManagedAgentSSEEvent(writer, decoder.Event()); err != nil {
			return err
		}
	}
	return decoder.Err()
}

// writeManagedAgentSSEEvent renders one frame as a single JSON line.
//
// A frame carrying only a payload is written as that payload alone, so the
// common case pipes straight into tools that expect one object per line. A
// frame with framing fields is wrapped so none of them are lost.
func writeManagedAgentSSEEvent(writer io.Writer, event ssestream.Event) error {
	var out interface{}

	if event.Type == "" && event.ID == "" && !event.HasRetry {
		if !event.HasData {
			return nil
		}
		out = decodeSSEPayload(event.Data)
	} else {
		wrapped := map[string]interface{}{}
		if event.Type != "" {
			wrapped["event"] = event.Type
		}
		if event.ID != "" {
			wrapped["id"] = event.ID
		}
		if event.HasRetry {
			wrapped["retry"] = event.Retry
		}
		// Only present when the frame actually carried a data field: a
		// sentinel such as "event: done" has no payload, and inventing an
		// empty one reports something the server never sent.
		if event.HasData {
			wrapped["data"] = decodeSSEPayload(event.Data)
		}
		out = wrapped
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return apierror.Errorf("failed to encode stream event: %w", err)
	}
	_, err = writer.Write(append(encoded, '\n'))
	return err
}

// decodeSSEPayload embeds a JSON payload as JSON rather than as a quoted
// string, so the NDJSON output stays queryable. Anything that does not parse is
// passed through as a string.
func decodeSSEPayload(data []byte) interface{} {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return string(data)
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
		return decoded
	}
	return string(data)
}
