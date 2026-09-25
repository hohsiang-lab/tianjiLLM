package chatgptcodex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultCodexCatalogRequestTimeout = 30 * time.Second

// CatalogFetcher retrieves sanitized model metadata for one subscription account.
type CatalogFetcher interface {
	FetchCatalog(context.Context, CatalogRequest) (Catalog, error)
}

type CatalogRequest struct {
	AccessToken string
	AccountID   string
}

type CatalogClient struct {
	BaseURL       string
	Originator    string
	ClientVersion string
	HTTPClient    *http.Client
}

type Catalog struct {
	Models []CatalogModel `json:"models"`
}

type CatalogModel struct {
	Slug                          string                  `json:"slug"`
	DisplayName                   string                  `json:"display_name"`
	Description                   *string                 `json:"description"`
	DefaultReasoningLevel         string                  `json:"default_reasoning_level"`
	SupportedReasoningLevels      []ReasoningEffortPreset `json:"supported_reasoning_levels"`
	AdditionalSpeedTiers          []string                `json:"additional_speed_tiers"`
	ServiceTiers                  []ModelServiceTier      `json:"service_tiers"`
	TruncationPolicy              TruncationPolicy        `json:"truncation_policy"`
	ContextWindow                 *int64                  `json:"context_window"`
	MaxContextWindow              *int64                  `json:"max_context_window"`
	AutoCompactTokenLimit         NullableInt64           `json:"auto_compact_token_limit"`
	EffectiveContextWindowPercent int64                   `json:"effective_context_window_percent"`
	InputModalities               []string                `json:"input_modalities"`
	UseResponsesLite              bool                    `json:"use_responses_lite"`
}

type ReasoningEffortPreset struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}
type ModelServiceTier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type TruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int64  `json:"limit"`
}

// NullableInt64 distinguishes a missing field from an explicit upstream null.
type NullableInt64 struct {
	Present bool
	Value   *int64
}

func (n NullableInt64) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}

func (n *NullableInt64) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	return nil
}

func ParseCatalog(data []byte) (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("parse Codex catalog: %w", err)
	}
	return catalog, nil
}

func (c Catalog) Model(slug string) (CatalogModel, bool) {
	for _, model := range c.Models {
		if model.Slug == slug {
			return model, true
		}
	}
	return CatalogModel{}, false
}

func (c CatalogClient) FetchCatalog(ctx context.Context, req CatalogRequest) (Catalog, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.catalogURL(), nil)
	if err != nil {
		return Catalog{}, fmt.Errorf("create Codex catalog request: %w", err)
	}
	if clientVersion := strings.TrimSpace(c.ClientVersion); clientVersion != "" {
		query := httpReq.URL.Query()
		query.Set("client_version", clientVersion)
		httpReq.URL.RawQuery = query.Encode()
	}
	applyClientVersionHeader(httpReq, c.ClientVersion)
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Accept", "application/json")
	if req.AccountID != "" {
		httpReq.Header.Set("ChatGPT-Account-Id", req.AccountID)
	}
	if c.Originator != "" {
		httpReq.Header.Set("Originator", c.Originator)
	}
	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return Catalog{}, fmt.Errorf("codex catalog request failed: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Catalog{}, fmt.Errorf("codex catalog request failed: status_%d", resp.StatusCode)
	}
	if readErr != nil {
		return Catalog{}, fmt.Errorf("read Codex catalog response: %w", readErr)
	}
	return ParseCatalog(body)
}

func (c CatalogClient) catalogURL() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = "https://chatgpt.com/backend-api"
	}
	switch {
	case strings.HasSuffix(base, "/codex/models"):
		return base
	case strings.HasSuffix(base, "/codex"):
		return base + "/models"
	case strings.HasSuffix(base, "/backend-api"):
		return base + "/codex/models"
	default:
		return base + "/backend-api/codex/models"
	}
}

func (c CatalogClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: defaultCodexCatalogRequestTimeout}
}
