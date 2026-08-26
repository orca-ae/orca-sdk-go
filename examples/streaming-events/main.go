// Command streaming-events follows a session's event stream.
//
//	go run ./streaming-events sess_123
//
// Ping frames are filtered by the stream itself, and an error frame ends it
// through Err rather than arriving as data - so this loop reads only the events
// the caller actually asked for.
package main

import (
	"context"
	"fmt"
	"log"
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

	stream := client.Sessions.Events.Stream(context.Background(), sessionID,
		orca.SessionEventStreamParams{})
	// Closing aborts the request, so breaking out of the loop early does not
	// leave it running.
	defer stream.Close()

	for stream.Next() {
		event := stream.Current()
		fmt.Printf("%s\t%s\n", event.ID, event.Type)
	}
	if err := stream.Err(); err != nil {
		log.Fatalf("streaming session %s: %v", sessionID, err)
	}
}
