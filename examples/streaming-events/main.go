// Command streaming-events follows a session's event stream and prints each
// event as one JSON object per line.
//
// The server speaks Server-Sent Events. DecodeSSE transcodes that wire format
// to NDJSON, which is what makes the output pipeable into jq and friends:
//
//	go run ./streaming-events sess_123 | jq -c 'select(.type == "message")'
//
// GetStream hands the raw body to a callback rather than buffering it, so this
// prints events as they arrive instead of waiting for the session to finish.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	orca "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/examples/orcaenv"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <session-id>\n", os.Args[0])
		os.Exit(2)
	}
	sessionID := os.Args[1]

	client, err := orcaenv.Client()
	if err != nil {
		log.Fatal(err)
	}

	agents := orca.NewManagedAgentsClient(client)
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/events/stream"

	err = agents.GetStream(context.Background(), path, "text/event-stream", func(body io.Reader) error {
		return orca.DecodeSSE(os.Stdout, body)
	})
	if err != nil {
		log.Fatalf("streaming session %s: %v", sessionID, err)
	}
}
