// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "io"

// DecodeSSE reads a Server-Sent Events stream from reader and writes each event
// to writer as one JSON object per line (NDJSON). An event carrying only data is
// written as that data alone; an event with an event, id, or retry field is
// wrapped as {"event":…,"id":…,"retry":…,"data":…}. Event data that parses as
// JSON is embedded as JSON rather than as a quoted string.
//
// It pairs with Client.GetStream, which hands the response body to a handler:
//
//	client.GetStream(ctx, path, "text/event-stream", func(r io.Reader) error {
//		return orca.DecodeSSE(os.Stdout, r)
//	})
//
// This is the only exported entry point added during the relocation from
// orca-cli; the decoder itself (sse.go) was copied unchanged.
func DecodeSSE(writer io.Writer, reader io.Reader) error {
	return renderManagedAgentSSE(writer, reader)
}
