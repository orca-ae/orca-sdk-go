// Command upload-package uploads a package to the workspace registry and reads
// its metadata back.
//
// Packages are a StreamNative Cloud extension. The SDK checks discovery before
// the call, so a deployment without the extension produces a clear diagnosis
// rather than a 404 from a path that looks like it should exist.
package main

import (
	"context"
	"errors"
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

	metadata := orca.PackageMetadata{
		Description: "Uploaded by the orca-sdk-go upload-package example",
		Properties:  map[string]string{"source": "example"},
	}

	err = client.Cloud.Packages.Upload(ctx, packageType, packageName, version, filePath, metadata)
	if err != nil {
		var unavailable *orca.ExtensionNotAvailableError
		if errors.As(err, &unavailable) {
			log.Fatalf("this deployment has no package registry: %v", unavailable)
		}
		log.Fatalf("uploading %s/%s@%s: %v", packageType, packageName, version, err)
	}
	fmt.Printf("uploaded %s/%s@%s from %s\n", packageType, packageName, version, filePath)

	stored, err := client.Cloud.Packages.GetMetadata(ctx, packageType, packageName, version)
	if err != nil {
		log.Fatalf("reading back metadata: %v", err)
	}
	fmt.Printf("description: %s\n", stored.Description)
	fmt.Printf("properties:  %v\n", stored.Properties)
}
