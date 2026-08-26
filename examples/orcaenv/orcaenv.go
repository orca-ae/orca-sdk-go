// Package orcaenv builds an SDK client from the environment so each example can
// stay focused on the API it demonstrates rather than on credential plumbing.
package orcaenv

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	orca "github.com/orca-ae/orca-sdk-go"
)

// Client returns a client for the deployment named by ORCA_BASE_URL.
//
// The two credential classes are not interchangeable: ORCA_API_KEY is a
// workspace API key sent as x-api-key, ORCA_ACCESS_TOKEN is a StreamNative
// Cloud OIDC token sent as Authorization: Bearer. The server reads x-api-key
// first and treats it as authoritative whenever present, so supplying both
// would silently ignore the token - this refuses instead.
func Client() (*orca.Client, error) {
	baseURL := strings.TrimSpace(os.Getenv("ORCA_BASE_URL"))
	if baseURL == "" {
		return nil, fmt.Errorf("ORCA_BASE_URL is required (the deployment host root, with no /v1 suffix)")
	}

	apiKey := strings.TrimSpace(os.Getenv("ORCA_API_KEY"))
	accessToken := strings.TrimSpace(os.Getenv("ORCA_ACCESS_TOKEN"))

	switch {
	case apiKey != "" && accessToken != "":
		return nil, fmt.Errorf("set exactly one of ORCA_API_KEY or ORCA_ACCESS_TOKEN, not both")
	case apiKey != "":
		return orca.NewAPIKeyClient(baseURL, apiKey, http.DefaultClient)
	case accessToken != "":
		return orca.NewClient(baseURL, accessToken, http.DefaultClient)
	default:
		return nil, fmt.Errorf("one of ORCA_API_KEY or ORCA_ACCESS_TOKEN is required")
	}
}
