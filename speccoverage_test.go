// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The surface was originally built from the TypeScript SDK rather than from the
// spec, and inherited its omissions: two core operations were missing and only
// turned up when a consumer needed them. This walks the vendored spec instead,
// so the next omission fails here rather than in whoever needs it next.

// specOnlyPaths are spec paths this SDK deliberately does not implement, each
// with the reason it is excluded. Anything else missing is a gap.
var specOnlyPaths = map[string]string{
	"/v1/git-creds": "the deployment overlay removes it (remove: true), so it is not portable " +
		"across both supported backends",
}

// pathTemplate normalises a path so spec placeholders and SDK escapes compare:
// /v1/vaults/{vault_id}/credentials/{credential_id} and the Go expression that
// builds it both become /v1/vaults/{}/credentials/{}.
var (
	specPlaceholder = regexp.MustCompile(`\{[a-z_]+\}`)
	pathEscapeCall  = regexp.MustCompile(`url\.PathEscape\([^)]*\)`)
	quotedOrHole    = regexp.MustCompile(`"([^"]*)"|(\{\})`)
)

// sdkPaths returns every request path the SDK's resource files build under one
// of prefixes.
func sdkPaths(t *testing.T, prefixes ...string) map[string]struct{} {
	t.Helper()

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	paths := map[string]struct{}{}
	for _, name := range sources {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, line := range strings.Split(string(source), "\n") {
			// Collapse each escaped path parameter to a hole, then stitch the
			// string literals back together to recover the template.
			expr := pathEscapeCall.ReplaceAllString(line, "{}")
			var built strings.Builder
			for _, match := range quotedOrHole.FindAllStringSubmatch(expr, -1) {
				built.WriteString(match[1] + match[2])
			}
			template := built.String()
			for _, prefix := range prefixes {
				if strings.HasPrefix(template, prefix) {
					paths["/"+strings.TrimSuffix(template, "/")] = struct{}{}
					break
				}
			}
		}
	}
	return paths
}

func TestEveryCoreSpecPathIsImplemented(t *testing.T) {
	t.Parallel()

	spec, err := os.ReadFile("openapi/managed-agents.yaml")
	if err != nil {
		t.Fatalf("reading the vendored core spec: %v", err)
	}

	declared := regexp.MustCompile(`(?m)^  (/v1/[a-zA-Z0-9/{}_.-]+):`)
	var specPaths []string
	for _, match := range declared.FindAllStringSubmatch(string(spec), -1) {
		specPaths = append(specPaths, match[1])
	}
	slices.Sort(specPaths)
	specPaths = slices.Compact(specPaths)

	if len(specPaths) == 0 {
		t.Fatal("no /v1 paths found in the spec; the pattern or the spec changed")
	}

	built := sdkPaths(t, "v1/")
	for _, path := range specPaths {
		if reason, excluded := specOnlyPaths[path]; excluded {
			t.Logf("skipping %s: %s", path, reason)
			continue
		}
		if _, ok := built[specPlaceholder.ReplaceAllString(path, "{}")]; !ok {
			t.Errorf("the core spec declares %s but no resource builds it", path)
		}
	}
}

func TestNoResourcePathIsAbsentFromTheSpec(t *testing.T) {
	t.Parallel()

	// The other direction: a path the SDK builds that no spec declares is an
	// invented endpoint, which fails only against a real deployment.
	spec, err := os.ReadFile("openapi/managed-agents.yaml")
	if err != nil {
		t.Fatalf("reading the vendored core spec: %v", err)
	}
	declared := regexp.MustCompile(`(?m)^  (/v1/[a-zA-Z0-9/{}_.-]+):`)

	specTemplates := map[string]struct{}{}
	for _, match := range declared.FindAllStringSubmatch(string(spec), -1) {
		specTemplates[specPlaceholder.ReplaceAllString(match[1], "{}")] = struct{}{}
	}

	for path := range sdkPaths(t, "v1/") {
		if _, ok := specTemplates[path]; !ok {
			t.Errorf("a resource builds %s but the core spec declares no such path", path)
		}
	}
}

func TestEveryPolicyPricingSpecPathIsImplemented(t *testing.T) {
	t.Parallel()

	spec, err := os.ReadFile("openapi/managed-agents-extensions.yaml")
	if err != nil {
		t.Fatalf("reading the vendored policy and pricing spec: %v", err)
	}

	declared := regexp.MustCompile(`(?m)^  (/apis/(?:policy|pricing)\.runorca\.ai/v1[a-zA-Z0-9/{}_.-]*):`)
	var specPaths []string
	for _, match := range declared.FindAllStringSubmatch(string(spec), -1) {
		specPaths = append(specPaths, match[1])
	}
	slices.Sort(specPaths)
	specPaths = slices.Compact(specPaths)
	if len(specPaths) == 0 {
		t.Fatal("no policy or pricing extension paths found in the spec")
	}

	built := sdkPaths(t, "apis/policy.runorca.ai/v1", "apis/pricing.runorca.ai/v1")
	for _, path := range specPaths {
		if _, ok := built[specPlaceholder.ReplaceAllString(path, "{}")]; !ok {
			t.Errorf("the extension spec declares %s but no resource builds it", path)
		}
	}
	for path := range built {
		found := false
		for _, declaredPath := range specPaths {
			if specPlaceholder.ReplaceAllString(declaredPath, "{}") == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("a resource builds %s but the extension spec declares no such path", path)
		}
	}
}
