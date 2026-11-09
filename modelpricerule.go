// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// ModelPriceRuleService reads engine v2 catalog snapshots without evaluating billing.
type ModelPriceRuleService struct{ client *Client }

// ModelPriceRuleListParams pages a single catalog snapshot.
type ModelPriceRuleListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// ModelPriceRuleConditionalModel describes the engine wire response.
type ModelPriceRuleConditionalModel struct {
	Provider string `json:"provider"`
	ModelID  string `json:"model_id"`
}

// ModelPriceRuleTokenRange describes the engine wire response.
type ModelPriceRuleTokenRange struct {
	Min int64  `json:"min"`
	Max *int64 `json:"max"`
}

// ModelPriceRuleConditions describes the engine wire response.
type ModelPriceRuleConditions struct {
	ServiceTier        string                   `json:"service_tier"`
	RequestInputTokens ModelPriceRuleTokenRange `json:"request_input_tokens"`
}

// ModelPriceRuleRates describes the engine wire response.
type ModelPriceRuleRates struct {
	InputPerMillionTokens        float64  `json:"input_per_million_tokens"`
	OutputPerMillionTokens       float64  `json:"output_per_million_tokens"`
	CacheReadPerMillionTokens    *float64 `json:"cache_read_per_million_tokens"`
	CacheWritePerMillionTokens   *float64 `json:"cache_write_per_million_tokens"`
	CacheWrite1HPerMillionTokens *float64 `json:"cache_write_1h_per_million_tokens"`
}

// ModelPriceRuleProvenance describes the engine wire response.
type ModelPriceRuleProvenance struct {
	SourceID       string `json:"source_id"`
	DocumentSHA256 string `json:"document_sha256"`
	AdapterVersion string `json:"adapter_version"`
	ObservedAt     string `json:"observed_at"`
}

// ModelPriceRule describes the engine wire response.
type ModelPriceRule struct {
	Conditions          ModelPriceRuleConditions `json:"conditions"`
	Rates               ModelPriceRuleRates      `json:"rates"`
	RuleSetID           string                   `json:"rule_set_id"`
	RuleID              string                   `json:"rule_id"`
	Revision            int64                    `json:"revision"`
	Provider            string                   `json:"provider"`
	ModelID             string                   `json:"model_id"`
	Source              string                   `json:"source"`
	OrganizationID      string                   `json:"organization_id"`
	UsageProfile        string                   `json:"usage_profile"`
	ProfileVersion      string                   `json:"profile_version"`
	Provenance          ModelPriceRuleProvenance `json:"provenance"`
	ProviderEffectiveAt *string                  `json:"provider_effective_at"`
	ActiveFrom          string                   `json:"active_from"`
	RetiredAt           *string                  `json:"retired_at"`
	ValidUntil          *string                  `json:"valid_until"`
}

// ModelPriceRulePage describes the engine wire response.
type ModelPriceRulePage struct {
	fetch             func(context.Context, string) (*ModelPriceRulePage, error)
	terminal          bool
	CatalogID         string                           `json:"catalog_id"`
	Generation        int64                            `json:"generation"`
	PublishedAt       string                           `json:"published_at"`
	Digest            string                           `json:"digest"`
	RequiredFeatures  []string                         `json:"required_features"`
	ConditionalModels []ModelPriceRuleConditionalModel `json:"conditional_models"`
	ExpiresAt         string                           `json:"expires_at"`
	Data              []ModelPriceRule                 `json:"data"`
	NextPage          *string                          `json:"next_page"`
}

// List retrieves one page, retaining metadata and unknown required features.
func (s ModelPriceRuleService) List(ctx context.Context, params ModelPriceRuleListParams, opts ...option.RequestOption) (*ModelPriceRulePage, error) {
	if err := s.client.ensureExtensionVersion(ctx, PricingExtensionGroup, "v2", opts...); err != nil {
		return nil, err
	}
	opts = appendListQuery(opts, params.Limit, params.Page)
	// Resolve once so defaults, per-call controls and cursor changes compose normally.
	cfg, err := s.client.cfg.With(opts...)
	if err != nil {
		return nil, err
	}
	endpoint, err := cfg.ResolveURL("apis/pricing.runorca.ai/v2/modelpricerules")
	if err != nil {
		return nil, err
	}
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	pageCfg := cfg.Clone()
	pageCfg.PathPrefix = ""
	client := &Client{cfg: pageCfg, discovery: s.client.discovery}
	var fetch func(context.Context, string) (*ModelPriceRulePage, error)
	fetch = func(ctx context.Context, cursor string) (*ModelPriceRulePage, error) {
		nextURL := *requestURL
		if cursor != "" {
			query := nextURL.Query()
			query.Set("page", cursor)
			nextURL.RawQuery = query.Encode()
		}
		var result ModelPriceRulePage
		if err := client.GetJSON(ctx, nextURL.String(), &result); err != nil {
			return nil, err
		}
		result.fetch = fetch
		return &result, nil
	}
	return fetch(ctx, "")
}

// HasNextPage reports whether a catalog cursor remains available.
func (p *ModelPriceRulePage) HasNextPage() bool {
	return p != nil && !p.terminal && p.fetch != nil && p.NextPage != nil && *p.NextPage != ""
}

// GetNextPage retains page metadata and fails on 410 without restarting a snapshot.
func (p *ModelPriceRulePage) GetNextPage(ctx context.Context) (*ModelPriceRulePage, error) {
	if !p.HasNextPage() {
		return nil, pagination.ErrNoMorePages
	}
	next, err := p.fetch(ctx, *p.NextPage)
	if err != nil {
		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 410 {
			p.terminal = true
		}
		return nil, err
	}
	if next.CatalogID != p.CatalogID || next.Generation != p.Generation || next.Digest != p.Digest {
		p.terminal = true
		return nil, fmt.Errorf("pricing catalog changed during pagination")
	}
	return next, nil
}

// ModelPriceRuleAutoPager walks rules while exposing the current metadata page.
type ModelPriceRuleAutoPager struct {
	ctx     context.Context
	page    *ModelPriceRulePage
	offset  int
	current ModelPriceRule
	err     error
}

// ListAutoPaging returns a pager that stops on catalog expiry without restarting.
func (s ModelPriceRuleService) ListAutoPaging(ctx context.Context, params ModelPriceRuleListParams, opts ...option.RequestOption) (*ModelPriceRuleAutoPager, error) {
	page, err := s.List(ctx, params, opts...)
	if err != nil {
		return nil, err
	}
	return &ModelPriceRuleAutoPager{ctx: ctx, page: page}, nil
}

// Next advances to a rule, stopping on the first page error.
func (p *ModelPriceRuleAutoPager) Next() bool {
	if p.err != nil || p.page == nil {
		return false
	}
	for p.offset >= len(p.page.Data) {
		if !p.page.HasNextPage() {
			return false
		}
		next, err := p.page.GetNextPage(p.ctx)
		if err != nil {
			p.err = err
			return false
		}
		p.page = next
		p.offset = 0
	}
	p.current = p.page.Data[p.offset]
	p.offset++
	return true
}

// Current returns the current rule.
func (p *ModelPriceRuleAutoPager) Current() ModelPriceRule { return p.current }

// Page returns the complete metadata for the current rule's page.
func (p *ModelPriceRuleAutoPager) Page() *ModelPriceRulePage { return p.page }

// Err returns the error that stopped pagination, including catalog expiry.
func (p *ModelPriceRuleAutoPager) Err() error { return p.err }
