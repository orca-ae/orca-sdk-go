// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import "context"

const (
	// CloudExtensionGroup is the DNS-scoped name of the StreamNative Cloud extension group, as
	// advertised by GET /apis. This is the one constant every path builder and capability check
	// must share - see the package doc comment on CloudExtensionBasePath for why callers of the
	// path builders still must not use it to construct expected test paths.
	CloudExtensionGroup = "cloud.sn.io"

	// CloudExtensionBasePath is the path prefix every StreamNative Cloud extension resource is
	// served under (health, connections, catalog, functions, sources, sinks,
	// Kafka Connect, packages, agent providers). Core managed-agent resources -
	// agents, sessions, memory stores, files, skills, vaults, environments, triggers - are not under this
	// prefix; they resolve under "v1" instead.
	//
	// Production code may reference this constant freely. Tests proving a path builder's output
	// must not: asserting against the same constant the implementation uses would still pass if
	// the constant itself were wrong, so URL-shape tests spell the literal string out instead.
	CloudExtensionBasePath = "apis/" + CloudExtensionGroup + "/v1"
)

// APIGroupVersion is one version entry for an API group, as reported by GET /apis.
type APIGroupVersion struct {
	GroupVersion string `json:"group_version" yaml:"group_version"`
	Version      string `json:"version" yaml:"version"`
}

// APIGroup is one extension group a deployment advertises via GET /apis.
type APIGroup struct {
	Name             string            `json:"name" yaml:"name"`
	Versions         []APIGroupVersion `json:"versions" yaml:"versions"`
	PreferredVersion APIGroupVersion   `json:"preferred_version" yaml:"preferred_version"`
}

// APIGroupList is the GET /apis response body: the extension groups this deployment serves beyond
// the core managed-agents surface. An empty Groups list is a normal, fully-functional self-hosted
// engine with no extensions installed - it is not an error, and it is not the same thing as a 404
// from /apis itself, which means the deployment predates discovery entirely and cannot serve any
// extension group no matter what it is asked for.
type APIGroupList struct {
	Kind   string     `json:"kind" yaml:"kind"`
	Groups []APIGroup `json:"groups" yaml:"groups"`
}

// APIResource describes one resource advertised by an extension API group.
type APIResource struct {
	Name       string `json:"name" yaml:"name"`
	Namespaced bool   `json:"namespaced" yaml:"namespaced"`
	Kind       string `json:"kind" yaml:"kind"`
}

// APIResourceList is the resource discovery response for one extension API group version.
type APIResourceList struct {
	Kind         string        `json:"kind" yaml:"kind"`
	GroupVersion string        `json:"group_version" yaml:"group_version"`
	Resources    []APIResource `json:"resources" yaml:"resources"`
}

// HasGroup reports whether name appears in the group list. Safe to call on a nil list.
func (l *APIGroupList) HasGroup(name string) bool {
	if l == nil {
		return false
	}
	for _, group := range l.Groups {
		if group.Name == name {
			return true
		}
	}
	return false
}

// GetAPIGroups calls authenticated GET /apis at the host root, listing the extension groups (if
// any) beyond the core managed-agents surface. Distinguish a 404 (server predates discovery) from a
// successful response with an empty Groups list (server supports discovery but has no extensions
// installed) - callers that gate a command on a specific group should treat those as different
// diagnoses.
func (c *Client) GetAPIGroups(ctx context.Context) (*APIGroupList, error) {
	var result APIGroupList
	if err := c.GetJSON(ctx, "apis", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCloudAPIResources calls authenticated GET /apis/cloud.sn.io/v1/ at the host root.
func (c *Client) GetCloudAPIResources(ctx context.Context) (*APIResourceList, error) {
	var result APIResourceList
	if err := c.GetJSON(ctx, CloudExtensionBasePath+"/", &result); err != nil {
		return nil, err
	}
	return &result, nil
}
