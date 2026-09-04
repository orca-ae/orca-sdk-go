// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// ModelPriceService reads effective prices from the pricing extension.
//
// Every operation is gated on the deployment advertising pricing.runorca.ai.
type ModelPriceService struct {
	client *Client
}

// ModelPrice is the effective token price used for cost accounting.
type ModelPrice struct {
	Type                       string  `json:"type"`
	Provider                   string  `json:"provider"`
	ModelID                    string  `json:"model_id"`
	InputPerMillionTokens      float64 `json:"input_per_million_tokens"`
	OutputPerMillionTokens     float64 `json:"output_per_million_tokens"`
	CacheReadPerMillionTokens  float64 `json:"cache_read_per_million_tokens"`
	CacheWritePerMillionTokens float64 `json:"cache_write_per_million_tokens"`
}

// ModelPriceListParams pages effective model prices.
type ModelPriceListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// ModelPriceGetParams optionally qualifies a model by provider.
type ModelPriceGetParams struct {
	Provider param.Opt[string]
}

// List returns one page of effective model prices.
func (s ModelPriceService) List(ctx context.Context, params ModelPriceListParams, opts ...option.RequestOption) (*pagination.PageCursor[ModelPrice], error) {
	if err := s.client.ensurePricingExtension(ctx, opts...); err != nil {
		return nil, err
	}
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[ModelPrice](ctx, s.client, "apis/pricing.runorca.ai/v1/modelprices", opts...)
}

// ListAutoPaging returns an iterator over all effective model prices.
func (s ModelPriceService) ListAutoPaging(ctx context.Context, params ModelPriceListParams, opts ...option.RequestOption) (*pagination.PageCursorAutoPager[ModelPrice], error) {
	page, err := s.List(ctx, params, opts...)
	if err != nil {
		return nil, err
	}
	return page.AutoPager(ctx), nil
}

// Get retrieves the effective price for a model and optional provider.
func (s ModelPriceService) Get(ctx context.Context, modelID string, params ModelPriceGetParams, opts ...option.RequestOption) (*ModelPrice, error) {
	if err := s.client.ensurePricingExtension(ctx, opts...); err != nil {
		return nil, err
	}
	if provider, ok := params.Provider.Value(); ok {
		opts = append(opts, option.WithQuery("provider", provider))
	}
	var price ModelPrice
	path := "apis/pricing.runorca.ai/v1/modelprices/" + url.PathEscape(modelID)
	if err := s.client.GetJSON(ctx, path, &price, opts...); err != nil {
		return nil, err
	}
	return &price, nil
}
