// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

func TestDiscoveryRecoveryOverHTTP(t *testing.T) {
	for _, initial := range []string{oldPricingGroups, apisBody(CloudExtensionGroup)} {
		t.Run(initial, func(t *testing.T) {
			now := time.Unix(100, 0)
			probes, catalogs := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer refresh-key" || r.Header.Get("X-Tenant") != "tenant" {
					t.Errorf("unexpected request %s %s headers=%v", r.Method, r.URL, r.Header)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.EscapedPath() {
				case "/apis":
					probes++
					if probes == 1 {
						_, _ = w.Write([]byte(initial))
					} else {
						_, _ = w.Write([]byte(engineGroups))
					}
				case "/apis/pricing.runorca.ai/v2/modelpricerules":
					catalogs++
					_, _ = w.Write([]byte(`{"catalog_id":"recovered","data":[]}`))
				default:
					t.Errorf("unexpected path %s", r.URL)
				}
			}))
			defer server.Close()
			client, err := New(option.WithBaseURL(server.URL), option.WithAuthToken("client-key"), option.WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			client.discovery.now = func() time.Time { return now }
			opts := []option.RequestOption{option.WithAuthToken("refresh-key"), option.WithHeader("X-Tenant", "tenant")}
			ctx := context.Background()
			_, err = client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{}, opts...)
			requireMissingExtension(t, err)
			if probes != 1 || catalogs != 0 {
				t.Fatalf("fresh miss: probes=%d catalogs=%d", probes, catalogs)
			}
			now = now.Add(15 * time.Second)
			page, err := client.ModelPriceRules.List(ctx, ModelPriceRuleListParams{}, opts...)
			if err != nil || page.CatalogID != "recovered" || probes != 2 || catalogs != 1 {
				t.Fatalf("page=%+v err=%v probes=%d catalogs=%d", page, err, probes, catalogs)
			}
		})
	}
}
