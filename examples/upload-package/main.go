// Command upload-package uploads a package to the workspace registry and reads
// its metadata back.
//
// Packages are a StreamNative Cloud extension, so this checks discovery first:
// calling the endpoint on a deployment without the extension group would fail
// with a bare 404 that says nothing about why.
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
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "usage: %s <type> <name> <version> <file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  e.g. %s function my-function 1.0.0 ./target/my-function.jar\n", os.Args[0])
		os.Exit(2)
	}
	packageType, packageName, version, filePath := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	client, err := orcaenv.Client()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	groups, err := client.GetAPIGroups(ctx)
	if err != nil {
		log.Fatalf("reading API groups: %v", err)
	}
	if !groups.HasGroup(orca.CloudExtensionGroup) {
		log.Fatalf("this deployment does not serve %s, so it has no package registry", orca.CloudExtensionGroup)
	}

	packages := orca.NewPackagesClient(client)

	metadata := orca.PackageMetadata{
		Description: "Uploaded by the orca-sdk-go upload-package example",
		Properties:  map[string]string{"source": "example"},
	}
	if err := packages.Upload(ctx, packageType, packageName, version, filePath, metadata); err != nil {
		log.Fatalf("uploading %s/%s@%s: %v", packageType, packageName, version, err)
	}
	fmt.Printf("uploaded %s/%s@%s from %s\n", packageType, packageName, version, filePath)

	stored, err := packages.GetMetadata(ctx, packageType, packageName, version)
	if err != nil {
		log.Fatalf("reading back metadata: %v", err)
	}
	fmt.Printf("description: %s\n", stored.Description)
	fmt.Printf("properties:  %v\n", stored.Properties)
}
