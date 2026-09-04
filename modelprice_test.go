// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

func TestModelPriceServiceUsesPricingExtensionOverHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/apis":
			_, _ = w.Write([]byte(extensionGroupsJSON(PricingExtensionGroup)))
		case "/apis/pricing.runorca.ai/v1/modelprices/model%2Fhttp":
			if got := r.URL.Query().Get("provider"); got != "provider-a" {
				t.Fatalf("provider = %q, want provider-a", got)
			}
			_, _ = w.Write([]byte(modelPriceFixtureJSON))
		default:
			t.Fatalf("unexpected path %s", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	price, err := client.ModelPrices.Get(context.Background(), "model/http", ModelPriceGetParams{
		Provider: param.String("provider-a"),
	})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if price.Provider != "provider-a" {
		t.Fatalf("provider = %q, want provider-a", price.Provider)
	}
}
