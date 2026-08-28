// Command quickstart creates an agent, starts a session, sends it a message,
// and follows the reply.
//
// It is the shortest path from an empty deployment to a running agent, and
// shows the four things every other example builds on: typed resources, request
// options, streaming, and typed errors.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	orca "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/examples/orcaenv"
)

func main() {
	client, err := orcaenv.Client()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	agent, err := client.Agents.Create(ctx, orca.AgentNewParams{
		Model: orca.Model("claude-sonnet-4-6"),
		Name:  "quickstart-agent",
	})
	if err != nil {
		log.Fatalf("creating the agent: %v", err)
	}
	fmt.Printf("agent:   %s\n", agent.ID)

	session, err := client.Sessions.Create(ctx, orca.SessionNewParams{
		Agent:         orca.AgentRef(agent.ID),
		EnvironmentID: environmentID(ctx, client),
	})
	if err != nil {
		log.Fatalf("creating the session: %v", err)
	}
	fmt.Printf("session: %s\n", session.ID)

	// The handle binds the session id once, so the calls below do not repeat it.
	handle := client.Session(session.ID)

	if _, err := handle.Events.Send(ctx, []orca.SessionEventParam{
		orca.UserMessage("In one sentence: what is a session?"),
	}); err != nil {
		log.Fatalf("sending the message: %v", err)
	}

	stream := handle.Events.Stream(ctx, orca.SessionEventStreamParams{})
	defer stream.Close()

	for stream.Next() {
		event := stream.Current()
		fmt.Printf("event:   %s\n", event.Type)
		if event.Type == "agent.message" {
			break
		}
	}
	if err := stream.Err(); err != nil {
		log.Fatalf("streaming events: %v", err)
	}
}

// environmentID returns the first environment the deployment offers.
//
// A session needs one, and which one is a deployment detail rather than
// something an example should hard-code.
func environmentID(ctx context.Context, client *orca.Client) string {
	page, err := client.Environments.List(ctx, orca.EnvironmentListParams{})
	if err != nil {
		var notFound *orca.NotFoundError
		if errors.As(err, &notFound) {
			log.Fatal("this deployment serves no environments")
		}
		log.Fatalf("listing environments: %v", err)
	}
	items := page.Items()
	if len(items) == 0 {
		log.Fatal("no environments exist yet; create one before running this example")
	}
	return items[0].ID
}
