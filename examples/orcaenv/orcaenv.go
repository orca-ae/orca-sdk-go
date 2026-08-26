// Package orcaenv builds an SDK client from the environment so each example can
// stay focused on the API it demonstrates rather than on credential plumbing.
package orcaenv

import (
	"fmt"
	"os"
	"strings"

	orca "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/option"
)

// Client returns a client for the deployment named by ORCA_BASE_URL.
//
// The two credential classes are not interchangeable: ORCA_API_KEY is a
// workspace API key sent as x-api-key, ORCA_ACCESS_TOKEN is a StreamNative
// Cloud OIDC token sent as Authorization: Bearer. The server reads x-api-key
// first and treats it as authoritative whenever present, so supplying both
// would silently ignore the token - this refuses instead.
//
// The SDK would read these variables on its own; they are read here so the
// error message can say which one is missing.
func Client() (*orca.Client, error) {
	baseURL := strings.TrimSpace(os.Getenv("ORCA_BASE_URL"))
	if baseURL == "" {
		return nil, fmt.Errorf("ORCA_BASE_URL is required (the deployment host root, with no /v1 suffix)")
	}

	apiKey := strings.TrimSpace(os.Getenv("ORCA_API_KEY"))
	accessToken := strings.TrimSpace(os.Getenv("ORCA_ACCESS_TOKEN"))

	credential := option.WithoutAuthentication()
	switch {
	case apiKey != "" && accessToken != "":
		return nil, fmt.Errorf("set exactly one of ORCA_API_KEY or ORCA_ACCESS_TOKEN, not both")
	case apiKey != "":
		credential = option.WithAPIKey(apiKey)
	case accessToken != "":
		credential = option.WithAuthToken(accessToken)
	default:
		return nil, fmt.Errorf("one of ORCA_API_KEY or ORCA_ACCESS_TOKEN is required")
	}

	return orca.New(
		option.WithBaseURL(baseURL),
		credential,
		option.WithWarningWriter(os.Stderr),
	)
}
