// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

const oldPricingGroups = `{"groups":[{"name":"cloud.sn.io"},{"name":"pricing.runorca.ai","versions":[{"version":"v1","group_version":"pricing.runorca.ai/v1"}]}]}`

func requireMissingExtension(t *testing.T, err error) {
	t.Helper()
	var missing *ExtensionNotAvailableError
	if !errors.As(err, &missing) {
		t.Fatalf("err=%v, want ExtensionNotAvailableError", err)
	}
}

func TestDiscoveryMissingCapabilityRecovery(t *testing.T) {
	for _, snapshot := range []string{oldPricingGroups, apisBody(CloudExtensionGroup)} {
		t.Run(snapshot, func(t *testing.T) {
			now := time.Unix(100, 0)
			probes := 0
			client, transport := newRecordingClientWith(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/apis" {
					probes++
					if probes == 1 {
						// TTL starts when the successful fetch completes.
						now = now.Add(5 * time.Second)
						return jsonResponse(200, snapshot), nil
					}
					return jsonResponse(200, engineGroups), nil
				}
				return jsonResponse(200, `{"catalog_id":"recovered","data":[]}`), nil
			})
			client.discovery.now = func() time.Time { return now }
			ctx := context.Background()
			if err := client.Cloud.EnsureAvailable(ctx); err != nil {
				t.Fatal(err)
			}
			now = now.Add(15*time.Second - time.Nanosecond)
			_, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{})
			requireMissingExtension(t, err)
			if probes != 1 {
				t.Fatalf("fresh first miss fetched twice: probes=%d", probes)
			}
			now = now.Add(time.Nanosecond)
			if err := client.Cloud.EnsureAvailable(ctx); err != nil || probes != 1 {
				t.Fatalf("supported group refreshed: err=%v probes=%d", err, probes)
			}
			page, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{})
			if err != nil || page.CatalogID != "recovered" || probes != 2 {
				t.Fatalf("page=%+v err=%v probes=%d", page, err, probes)
			}
			if transport.Last(t).Path() != "/apis/pricing.runorca.ai/v2/modelpricerules" {
				t.Fatal(transport.Last(t).Path())
			}
			now = now.Add(time.Hour)
			if _, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{}); err != nil || probes != 2 {
				t.Fatalf("positive cache expired: err=%v probes=%d", err, probes)
			}
		})
	}
}

func TestDiscoveryRefreshFailureKeepsSuccessfulSnapshot(t *testing.T) {
	for _, status := range []int{404, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			now := time.Unix(100, 0)
			client, transport, warnings := newGatedClient(t, func(n int) (*http.Response, error) {
				switch n {
				case 1:
					return jsonResponse(200, oldPricingGroups), nil
				case 2:
					return jsonResponse(status, `{}`), nil
				default:
					return jsonResponse(200, engineGroups), nil
				}
			})
			client.discovery.now = func() time.Time { return now }
			ctx := context.Background()
			if err := client.Cloud.EnsureAvailable(ctx); err != nil {
				t.Fatal(err)
			}
			now = now.Add(15 * time.Second)
			_, err := client.Discovery.PricingV2GroupResources(ctx)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status || transport.probes() != 2 {
				t.Fatalf("err=%v probes=%d", err, transport.probes())
			}
			var notFound *NotFoundError
			if status == 404 && !errors.As(err, &notFound) {
				t.Fatalf("refresh err=%v, want NotFoundError", err)
			}
			if warnings.Len() != 0 {
				t.Fatalf("refresh warned about pre-discovery server: %s", warnings)
			}
			if err := client.Cloud.EnsureAvailable(ctx); err != nil || transport.probes() != 2 {
				t.Fatalf("lost supported group: err=%v probes=%d", err, transport.probes())
			}
			if err := client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v1"); err != nil || transport.probes() != 2 {
				t.Fatalf("lost supported version: err=%v probes=%d", err, transport.probes())
			}
			if err := client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2"); err != nil || transport.probes() != 3 {
				t.Fatalf("failure cached or success time advanced: err=%v probes=%d", err, transport.probes())
			}
		})
	}
}

func TestDiscoveryInitial404IsUnavailableAndUncached(t *testing.T) {
	client, transport, warnings := newGatedClient(t, func(n int) (*http.Response, error) {
		if n <= 2 {
			return jsonResponse(404, `{}`), nil
		}
		return jsonResponse(200, apisBody(CloudExtensionGroup)), nil
	})
	for range 2 {
		err := client.Cloud.EnsureAvailable(context.Background())
		requireMissingExtension(t, err)
		var notFound *NotFoundError
		if errors.As(err, &notFound) || warnings.Len() == 0 {
			t.Fatalf("err=%v warnings=%s", err, warnings)
		}
		warnings.Reset()
	}
	if err := client.Cloud.EnsureAvailable(context.Background()); err != nil || transport.probes() != 3 {
		t.Fatalf("err=%v probes=%d", err, transport.probes())
	}
}

func TestDiscoveryRefreshPreservesPerCallOptionsAndDeployment(t *testing.T) {
	now := time.Unix(100, 0)
	probes := map[string]int{}
	delays := &[]time.Duration{}
	client, transport := newRecordingClientWith(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/apis" {
			probes[r.URL.Host]++
			if probes[r.URL.Host] == 1 {
				return jsonResponse(200, oldPricingGroups), nil
			}
			if probes[r.URL.Host] == 2 {
				return jsonResponse(503, `{}`), nil
			}
			return jsonResponse(200, engineGroups), nil
		}
		return jsonResponse(200, `{}`), nil
	}, withRecordedSleep(delays))
	client.discovery.now = func() time.Time { return now }
	ctx := context.Background()
	if err := client.Cloud.EnsureAvailable(ctx); err != nil {
		t.Fatal(err)
	}
	opts := []option.RequestOption{option.WithBaseURL("https://alternate.example.test"), option.WithAuthToken("alternate-key"), option.WithHeader("X-Tenant", "tenant"), option.WithRequestTimeout(7 * time.Second), option.WithMaxRetries(1)}
	_, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{}, opts...)
	requireMissingExtension(t, err)
	now = now.Add(15 * time.Second)
	if _, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{}, opts...); err != nil {
		t.Fatal(err)
	}
	if probes["api.example.test"] != 1 || probes["alternate.example.test"] != 3 || len(*delays) != 1 {
		t.Fatalf("probes=%v delays=%v", probes, *delays)
	}
	for _, call := range transport.Calls()[1:] {
		if call.URL.Host != "alternate.example.test" || call.Header.Get("Authorization") != "Bearer alternate-key" || call.Header.Get("X-Tenant") != "tenant" || call.Header.Get("X-Orca-Timeout") != "7" {
			t.Fatalf("lost per-call options: %+v", call)
		}
	}
}

func TestDiscoveryConcurrentMissingRefresh(t *testing.T) {
	for _, snapshot := range []string{oldPricingGroups, apisBody(CloudExtensionGroup)} {
		t.Run(snapshot, func(t *testing.T) {
			now := time.Unix(100, 0)
			started, release := make(chan struct{}), make(chan struct{})
			client, transport, _ := newGatedClient(t, func(n int) (*http.Response, error) {
				if n == 1 {
					return jsonResponse(200, snapshot), nil
				}
				if n == 2 {
					close(started)
				}
				<-release
				return jsonResponse(200, engineGroups), nil
			})
			client.discovery.now = func() time.Time { return now }
			ctx := context.Background()
			if err := client.Cloud.EnsureAvailable(ctx); err != nil {
				t.Fatal(err)
			}
			now = now.Add(15 * time.Second)
			results := make(chan error, 16)
			entered := make(chan struct{}, 17)
			client.discovery.now = func() time.Time {
				entered <- struct{}{}
				return now
			}
			for range 16 {
				go func() {
					results <- client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2")
				}()
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				close(release)
				t.Fatal("missing capability never refreshed")
			}
			// Hold the fetch until every caller has inspected the stale snapshot.
			for range 16 {
				select {
				case <-entered:
				case <-time.After(time.Second):
					close(release)
					t.Fatal("missing probe did not reach the stale snapshot")
				}
			}
			close(release)
			for range 16 {
				if err := <-results; err != nil {
					t.Error(err)
				}
			}
			if transport.probes() != 2 {
				t.Fatalf("probes=%d, want initial + one refresh", transport.probes())
			}
		})
	}
}

func TestDiscoveryMissingGroupRefresh(t *testing.T) {
	now := time.Unix(100, 0)
	client, transport, _ := newGatedClient(t, func(n int) (*http.Response, error) {
		if n == 1 {
			return jsonResponse(200, apisBody()), nil
		}
		return jsonResponse(200, apisBody(CloudExtensionGroup)), nil
	})
	client.discovery.now = func() time.Time { return now }
	requireMissingExtension(t, client.Cloud.EnsureAvailable(context.Background()))
	now = now.Add(15 * time.Second)
	if err := client.Cloud.EnsureAvailable(context.Background()); err != nil || transport.probes() != 2 {
		t.Fatalf("err=%v probes=%d", err, transport.probes())
	}
}

func TestDiscoveryMissingRefreshRenewsSnapshot(t *testing.T) {
	now := time.Unix(100, 0)
	client, transport, _ := newGatedClient(t, func(n int) (*http.Response, error) {
		if n < 3 {
			return jsonResponse(200, oldPricingGroups), nil
		}
		return jsonResponse(200, engineGroups), nil
	})
	client.discovery.now = func() time.Time { return now }
	ctx := context.Background()
	requireMissingExtension(t, client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2"))
	if transport.probes() != 1 {
		t.Fatal("first miss fetched twice")
	}
	now = now.Add(15 * time.Second)
	requireMissingExtension(t, client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2"))
	if transport.probes() != 2 {
		t.Fatalf("refresh probes=%d", transport.probes())
	}
	now = now.Add(15*time.Second - time.Nanosecond)
	requireMissingExtension(t, client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2"))
	if transport.probes() != 2 {
		t.Fatalf("fresh snapshot refreshed: probes=%d", transport.probes())
	}
	now = now.Add(time.Nanosecond)
	if err := client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2"); err != nil || transport.probes() != 3 {
		t.Fatalf("err=%v probes=%d", err, transport.probes())
	}
}
