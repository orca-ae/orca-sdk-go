// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/internal/requestconfig"
	"github.com/orca-ae/orca-sdk-go/option"
)

// discoveryCache remembers which extension groups a deployment advertises.
//
// Every cloud call has to know before it goes out, and asking on each one would
// double the request count for a fact that does not change while a process
// runs. Results are keyed by effective base URL, because one program may talk
// to several deployments through per-call options and they will not agree.
//
// Only successes are cached: a discovery attempt that failed says nothing about
// the deployment, and remembering it would turn one bad moment into a permanent
// refusal to make any cloud call at all.
type discoveryCache struct {
	mu       sync.Mutex
	groups   map[string]*APIGroupList
	inFlight map[string]*discoveryCall
}

// discoveryCall is one in-flight probe that concurrent callers wait on, so a
// burst of cloud calls at startup issues one GET /apis rather than one each.
type discoveryCall struct {
	done   chan struct{}
	groups *APIGroupList
	err    error
}

func newDiscoveryCache() *discoveryCache {
	return &discoveryCache{
		groups:   map[string]*APIGroupList{},
		inFlight: map[string]*discoveryCall{},
	}
}

// resolve returns the cached groups for key, running fetch if needed.
func (c *discoveryCache) resolve(key string, fetch func() (*APIGroupList, error)) (*APIGroupList, error) {
	c.mu.Lock()
	if groups, ok := c.groups[key]; ok {
		c.mu.Unlock()
		return groups, nil
	}
	if call, ok := c.inFlight[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.groups, call.err
	}

	call := &discoveryCall{done: make(chan struct{})}
	c.inFlight[key] = call
	c.mu.Unlock()

	call.groups, call.err = fetch()

	c.mu.Lock()
	delete(c.inFlight, key)
	if call.err == nil {
		c.groups[key] = call.groups
	}
	c.mu.Unlock()
	close(call.done)

	return call.groups, call.err
}

// ensureExtensionAvailable checks that the deployment serves group before a
// request that depends on it goes out.
//
// Without this, calling a cloud method against a deployment that has no cloud
// extension produces a 404 from a path the caller has no reason to doubt, which
// reads as "that connection does not exist" rather than "this deployment has no
// connections at all".
func (c *Client) ensureExtensionAvailable(ctx context.Context, group string, opts ...option.RequestOption) error {
	cfg, err := c.cfg.With(opts...)
	if err != nil {
		return err
	}

	baseURL := ""
	if cfg.BaseURL != nil {
		baseURL = cfg.BaseURL.String()
	}

	groups, err := c.discovery.resolve(baseURL, func() (*APIGroupList, error) {
		return c.getAPIGroupsWithConfig(ctx, cfg)
	})
	if err != nil {
		// A deployment that predates discovery has no /apis at all. That is not
		// a missing resource, it is an older server, so it is reported as an
		// unavailable extension with a version hint rather than as a 404 the
		// caller would try to debug at the resource path.
		var notFound *apierror.NotFoundError
		if errors.As(err, &notFound) {
			fmt.Fprintf(cfg.WarningWriter,
				"warning: GET /apis returned 404 — this deployment predates extension discovery "+
					"and cannot serve the %q extension group. Confirm the server version if this "+
					"is unexpected.\n", group)
			return &apierror.ExtensionNotAvailableError{
				Group:   group,
				BaseURL: baseURL,
				Reason:  "the deployment does not serve an extension discovery endpoint",
			}
		}
		return err
	}

	if groups.HasGroup(group) {
		return nil
	}

	// An empty list is a normal, fully-functional deployment with no extensions
	// installed - not an error condition in itself, and not a reason to warn.
	reason := "the deployment advertises no extension groups"
	if len(groups.Groups) > 0 {
		reason = fmt.Sprintf("the deployment advertises %s", quotedGroupNames(groups))
	}
	return &apierror.ExtensionNotAvailableError{Group: group, BaseURL: baseURL, Reason: reason}
}

// ensureCloudExtension gates a StreamNative Cloud extension call.
func (c *Client) ensureCloudExtension(ctx context.Context, opts ...option.RequestOption) error {
	return c.ensureExtensionAvailable(ctx, CloudExtensionGroup, opts...)
}

// getAPIGroupsWithConfig performs the discovery request using an already
// resolved config, so the probe carries the same deployment, credential and
// per-call controls as the call that triggered it.
func (c *Client) getAPIGroupsWithConfig(ctx context.Context, cfg *requestconfig.RequestConfig) (*APIGroupList, error) {
	// Discovery is served at the host root. A client scoped to an API group
	// would otherwise probe {prefix}/apis, which does not exist.
	probeCfg := cfg.Clone()
	probeCfg.PathPrefix = ""

	var result APIGroupList
	probe := &Client{cfg: probeCfg, discovery: c.discovery}
	if err := probe.GetJSON(ctx, "apis", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func quotedGroupNames(groups *APIGroupList) string {
	names := make([]string, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		names = append(names, fmt.Sprintf("%q", group.Name))
	}
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%v", names)
}
