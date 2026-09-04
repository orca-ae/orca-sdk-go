// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/packages/param"
)

const modelPriceFixtureJSON = `{"type":"model_price","provider":"provider-a","model_id":"model-alpha",` +
	`"input_per_million_tokens":3,"output_per_million_tokens":15,` +
	`"cache_read_per_million_tokens":0.3,"cache_write_per_million_tokens":3.75}`

func TestModelPriceOperations(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/apis":
			return jsonResponse(http.StatusOK, extensionGroupsJSON(PricingExtensionGroup)), nil
		case req.URL.Path == "/apis/pricing.runorca.ai/v1/modelprices":
			return jsonResponse(http.StatusOK, `{"data":[`+modelPriceFixtureJSON+`],"next_page":null}`), nil
		default:
			return jsonResponse(http.StatusOK, modelPriceFixtureJSON), nil
		}
	})
	ctx := context.Background()

	page, err := client.ModelPrices.List(ctx, ModelPriceListParams{Limit: param.Int(10), Page: param.String("next")})
	if err != nil || len(page.Data) != 1 || page.Data[0].OutputPerMillionTokens != 15 {
		t.Fatalf("List() = %#v, %v", page, err)
	}
	listCall := transport.Last(t)
	if got := listCall.Path(); got != "/apis/pricing.runorca.ai/v1/modelprices" {
		t.Errorf("List() path = %q", got)
	}
	if got := listCall.Query().Get("limit"); got != "10" {
		t.Errorf("limit = %q, want 10", got)
	}
	if got := listCall.Query().Get("page"); got != "next" {
		t.Errorf("page = %q, want next", got)
	}

	price, err := client.ModelPrices.Get(ctx, "model/alpha", ModelPriceGetParams{Provider: param.String("provider-a")})
	if err != nil || price.ModelID != "model-alpha" {
		t.Fatalf("Get() = %#v, %v", price, err)
	}
	getCall := transport.Last(t)
	if got := getCall.Path(); got != "/apis/pricing.runorca.ai/v1/modelprices/model%2Falpha" {
		t.Errorf("Get() path = %q", got)
	}
	if got := getCall.Query().Get("provider"); got != "provider-a" {
		t.Errorf("provider = %q, want provider-a", got)
	}
}

func TestModelPriceGateStopsBusinessRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invoke func(context.Context, *Client) error
	}{
		{name: "list", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.ModelPrices.List(ctx, ModelPriceListParams{})
			return err
		}},
		{name: "get", invoke: func(ctx context.Context, client *Client) error {
			_, err := client.ModelPrices.Get(ctx, "model-alpha", ModelPriceGetParams{})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, transport := newRecordingClient(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/apis" {
					return jsonResponse(http.StatusOK, extensionGroupsJSON()), nil
				}
				return jsonResponse(http.StatusInternalServerError, `{}`), nil
			})
			err := tc.invoke(context.Background(), client)
			var unavailable *ExtensionNotAvailableError
			if !errors.As(err, &unavailable) || unavailable.Group != PricingExtensionGroup {
				t.Fatalf("error = %v, want pricing ExtensionNotAvailableError", err)
			}
			if calls := transport.Calls(); len(calls) != 1 || calls[0].Path() != "/apis" {
				t.Fatalf("requests = %#v, want only GET /apis", calls)
			}
		})
	}
}
