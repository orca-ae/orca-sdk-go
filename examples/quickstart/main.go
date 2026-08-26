// Command quickstart probes a deployment: what core API versions it serves,
// whether it is healthy, and which extension groups it advertises.
//
// The discovery step matters because the StreamNative Cloud extension surface
// is optional. A deployment that does not advertise cloud.sn.io is a normal,
// fully-functional engine - it simply has no connections to list - so this
// checks before calling instead of treating the 404 as a failure.
package main

import (
	"context"
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

	versions, err := client.GetAPIVersions(ctx)
	if err != nil {
		log.Fatalf("reading API versions: %v", err)
	}
	fmt.Printf("core versions: %v (preferred %s)\n", versions.Versions, versions.PreferredVersion)

	health, err := client.GetHealthz(ctx)
	if err != nil {
		log.Fatalf("reading health: %v", err)
	}
	fmt.Printf("health: %s (%s)\n", health.Status, health.Service)

	groups, err := client.GetAPIGroups(ctx)
	if err != nil {
		log.Fatalf("reading API groups: %v", err)
	}
	if len(groups.Groups) == 0 {
		fmt.Println("extensions: none installed")
		return
	}
	for _, group := range groups.Groups {
		fmt.Printf("extension group: %s (preferred %s)\n", group.Name, group.PreferredVersion.GroupVersion)
	}

	if !groups.HasGroup(orca.CloudExtensionGroup) {
		fmt.Printf("extensions: %s not available on this deployment\n", orca.CloudExtensionGroup)
		return
	}

	connections, err := orca.NewConnectionsClient(client).List(ctx)
	if err != nil {
		log.Fatalf("listing connections: %v", err)
	}
	fmt.Printf("connections: %d\n", len(connections))
	for _, connection := range connections {
		fmt.Printf("  %s (%s)\n", connection.Name, connection.Spec.Type)
	}
}
