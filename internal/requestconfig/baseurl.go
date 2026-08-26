// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package requestconfig

import (
	"fmt"
	"net/url"
	"strings"
)

// legacyBaseURLSuffixes are base-URL path suffixes that used to be required and
// are now stripped with a deprecation warning.
//
// Order matters: /api/v1 and /v1/registry must be checked before the bare /v1,
// because both of them also end in "/v1" - checking the longer, more specific
// suffix first is what makes the whole suffix get stripped instead of just its
// last segment. Leaving ".../api" behind (by stripping only the trailing "/v1"
// of ".../api/v1") would produce a base where core calls still work via the
// /api/v1 alias but extension calls under /apis/... 404 on an undocumented
// path - a partial failure that is far more expensive to debug than a clean
// strip.
var legacyBaseURLSuffixes = []string{"/api/v1", "/v1/registry", "/v1"}

// StripLegacyBaseURLSuffix removes a trailing legacy suffix from a base-URL
// path, if present.
//
// The match is anchored to the end of the string, so a path like
// "/v1/registry-proxy" is left untouched: it does not end in exactly
// "/v1/registry" or "/v1".
func StripLegacyBaseURLSuffix(path string) (stripped string, matchedSuffix string) {
	trimmed := strings.TrimRight(path, "/")
	for _, suffix := range legacyBaseURLSuffixes {
		if strings.HasSuffix(trimmed, suffix) {
			return strings.TrimSuffix(trimmed, suffix), suffix
		}
	}
	return path, ""
}

// NormalizeBaseURL records parsed as the config's base URL, stripping a legacy
// API-path suffix and collapsing trailing slashes.
//
// Collapsing matters because resolution is textual: without it a base of
// "https://host///" resolves "v1/agents" to "https://host///v1/agents", which
// the server sees as three empty path segments rather than the route the caller
// meant. Exactly one trailing slash is what makes a relative path resolve
// against the host root instead of replacing its last segment.
func NormalizeBaseURL(cfg *RequestConfig, parsed *url.URL, original string) {
	if stripped, matchedSuffix := StripLegacyBaseURLSuffix(parsed.Path); matchedSuffix != "" {
		parsed.Path = stripped
		cfg.Warn(legacyBaseURLWarning(original, matchedSuffix, withTrailingSlash(parsed).String()))
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	cfg.BaseURL = parsed
}

func withTrailingSlash(u *url.URL) *url.URL {
	clone := *u
	clone.Path = strings.TrimRight(clone.Path, "/") + "/"
	return &clone
}

func legacyBaseURLWarning(originalBaseURL, matchedSuffix, strippedBaseURL string) string {
	return fmt.Sprintf(
		"warning: registry base URL %q ends with %q, which is no longer part of the base URL — "+
			"every deployment now serves core at the host root (for example, GET {base}/v1/agents). "+
			"Using %q instead. Update --registry-url / ORCA_REGISTRY_URL to the host root; this "+
			"compatibility shim may be removed in a future version.\n",
		originalBaseURL, matchedSuffix, strippedBaseURL)
}
