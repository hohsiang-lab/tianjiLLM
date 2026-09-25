package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
)

const (
	codexDefaultContextWindow = int64(128000)
)

type listModelsResponse struct {
	Object string                `json:"object"`
	Data   []openAIModelListItem `json:"data"`
	Models []codexModelInfo      `json:"models"`
}

type openAIModelListItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func openAIModelListItemFromConfig(model config.ModelConfig) openAIModelListItem {
	return openAIModelListItem{
		ID:      model.ModelName,
		Object:  "model",
		OwnedBy: "tianji",
	}
}

type codexReasoningEffortPreset struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type codexModelServiceTier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type codexTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int64  `json:"limit"`
}

type codexModelInfo struct {
	Slug                          string                       `json:"slug"`
	DisplayName                   string                       `json:"display_name"`
	Description                   *string                      `json:"description"`
	DefaultReasoningLevel         string                       `json:"default_reasoning_level"`
	SupportedReasoningLevels      []codexReasoningEffortPreset `json:"supported_reasoning_levels"`
	ShellType                     string                       `json:"shell_type"`
	Visibility                    string                       `json:"visibility"`
	SupportedInAPI                bool                         `json:"supported_in_api"`
	Priority                      int                          `json:"priority"`
	AdditionalSpeedTiers          []string                     `json:"additional_speed_tiers"`
	ServiceTiers                  []codexModelServiceTier      `json:"service_tiers"`
	AvailabilityNux               any                          `json:"availability_nux"`
	Upgrade                       any                          `json:"upgrade"`
	BaseInstructions              string                       `json:"base_instructions"`
	ModelMessages                 any                          `json:"model_messages"`
	SupportsReasoningSummaries    bool                         `json:"supports_reasoning_summaries"`
	DefaultReasoningSummary       string                       `json:"default_reasoning_summary"`
	SupportVerbosity              bool                         `json:"support_verbosity"`
	DefaultVerbosity              any                          `json:"default_verbosity"`
	ApplyPatchToolType            string                       `json:"apply_patch_tool_type"`
	WebSearchToolType             string                       `json:"web_search_tool_type"`
	TruncationPolicy              codexTruncationPolicy        `json:"truncation_policy"`
	SupportsParallelToolCalls     bool                         `json:"supports_parallel_tool_calls"`
	SupportsImageDetailOriginal   bool                         `json:"supports_image_detail_original"`
	ContextWindow                 *int64                       `json:"context_window,omitempty"`
	MaxContextWindow              *int64                       `json:"max_context_window,omitempty"`
	AutoCompactTokenLimit         *int64                       `json:"auto_compact_token_limit"`
	EffectiveContextWindowPercent int64                        `json:"effective_context_window_percent"`
	ExperimentalSupportedTools    []string                     `json:"experimental_supported_tools"`
	InputModalities               []string                     `json:"input_modalities"`
	SupportsSearchTool            bool                         `json:"supports_search_tool"`
	UseResponsesLite              bool                         `json:"use_responses_lite"`
}

func buildListModelsResponse(models []config.ModelConfig) listModelsResponse {
	data := make([]openAIModelListItem, 0, len(models))
	codexModels := make([]codexModelInfo, 0, len(models))
	for i, m := range models {
		data = append(data, openAIModelListItemFromConfig(m))
		codexModels = append(codexModels, codexModelInfoFromConfig(m, i+1))
	}
	return listModelsResponse{
		Object: "list",
		Data:   data,
		Models: codexModels,
	}
}

func (h *Handlers) buildListModelsResponse(ctx context.Context, models []config.ModelConfig) listModelsResponse {
	response := buildListModelsResponse(models)
	for index, model := range models {
		if catalog, ok := h.codexCatalogForModel(ctx, model); ok {
			response.Models[index] = codexModelInfoFromCatalog(response.Models[index], catalog)
		}
	}
	return response
}

func codexModelInfoFromConfig(model config.ModelConfig, priority int) codexModelInfo {
	slug := strings.TrimSpace(model.ModelName)
	displayName := slug
	if model.ModelInfo != nil && strings.TrimSpace(model.ModelInfo.ID) != "" {
		displayName = strings.TrimSpace(model.ModelInfo.ID)
	}

	description := fmt.Sprintf("Tianji runtime model alias %s", slug)
	if upstream := strings.TrimSpace(model.TianjiParams.Model); upstream != "" && upstream != slug {
		description = fmt.Sprintf("Tianji runtime model alias %s backed by %s", slug, upstream)
	}

	contextWindow := codexContextWindow(model.ModelInfo)
	autoCompact := (contextWindow * 9) / 10

	return codexModelInfo{
		Slug:                  slug,
		DisplayName:           displayName,
		Description:           &description,
		DefaultReasoningLevel: "medium",
		SupportedReasoningLevels: []codexReasoningEffortPreset{
			{Effort: "low", Description: "Lower reasoning effort."},
			{Effort: "medium", Description: "Balanced reasoning effort."},
			{Effort: "high", Description: "Higher reasoning effort."},
			{Effort: "xhigh", Description: "Extra high reasoning effort."},
		},
		ShellType:                  "default",
		Visibility:                 "list",
		SupportedInAPI:             true,
		Priority:                   priority,
		AdditionalSpeedTiers:       []string{},
		ServiceTiers:               []codexModelServiceTier{},
		AvailabilityNux:            nil,
		Upgrade:                    nil,
		BaseInstructions:           "",
		ModelMessages:              nil,
		SupportsReasoningSummaries: true,
		DefaultReasoningSummary:    "auto",
		SupportVerbosity:           true,
		DefaultVerbosity:           nil,
		ApplyPatchToolType:         "freeform",
		WebSearchToolType:          "text",
		TruncationPolicy: codexTruncationPolicy{
			Mode:  "tokens",
			Limit: contextWindow,
		},
		SupportsParallelToolCalls:     true,
		SupportsImageDetailOriginal:   false,
		ContextWindow:                 &contextWindow,
		MaxContextWindow:              &contextWindow,
		AutoCompactTokenLimit:         &autoCompact,
		EffectiveContextWindowPercent: 95,
		ExperimentalSupportedTools:    []string{},
		InputModalities:               []string{"text"},
		SupportsSearchTool:            false,
	}
}

func codexModelInfoFromCatalog(base codexModelInfo, upstream chatgptcodex.CatalogModel) codexModelInfo {
	if strings.Contains(base.Slug, "*") {
		base.UseResponsesLite = upstream.UseResponsesLite
		if upstream.UseResponsesLite {
			base.SupportsParallelToolCalls = false
		}
		return base
	}
	if upstream.DisplayName != "" {
		base.DisplayName = upstream.DisplayName
	}
	if upstream.Description != nil {
		base.Description = upstream.Description
	}
	if upstream.ContextWindow != nil {
		base.ContextWindow = upstream.ContextWindow
	}
	if upstream.MaxContextWindow != nil {
		base.MaxContextWindow = upstream.MaxContextWindow
	}
	if upstream.EffectiveContextWindowPercent != 0 {
		base.EffectiveContextWindowPercent = upstream.EffectiveContextWindowPercent
	}
	if upstream.AutoCompactTokenLimit.Present {
		base.AutoCompactTokenLimit = upstream.AutoCompactTokenLimit.Value
	}
	if upstream.TruncationPolicy.Mode != "" {
		base.TruncationPolicy = codexTruncationPolicy{Mode: upstream.TruncationPolicy.Mode, Limit: upstream.TruncationPolicy.Limit}
	}
	if upstream.DefaultReasoningLevel != "" {
		base.DefaultReasoningLevel = upstream.DefaultReasoningLevel
	}
	base.SupportedReasoningLevels = make([]codexReasoningEffortPreset, len(upstream.SupportedReasoningLevels))
	for i, level := range upstream.SupportedReasoningLevels {
		base.SupportedReasoningLevels[i] = codexReasoningEffortPreset{Effort: level.Effort, Description: level.Description}
	}
	base.AdditionalSpeedTiers = append([]string(nil), upstream.AdditionalSpeedTiers...)
	base.InputModalities = append([]string(nil), upstream.InputModalities...)
	base.UseResponsesLite = upstream.UseResponsesLite
	if upstream.UseResponsesLite {
		base.SupportsParallelToolCalls = false
	}
	base.ServiceTiers = make([]codexModelServiceTier, len(upstream.ServiceTiers))
	for i, tier := range upstream.ServiceTiers {
		base.ServiceTiers[i] = codexModelServiceTier{ID: tier.ID, Name: tier.Name, Description: tier.Description}
	}
	return base
}

func codexContextWindow(info *config.ModelInfo) int64 {
	if info == nil {
		return codexDefaultContextWindow
	}
	switch {
	case info.MaxInputTokens != nil && *info.MaxInputTokens > 0:
		return int64(*info.MaxInputTokens)
	case info.MaxTokens != nil && *info.MaxTokens > 0:
		return int64(*info.MaxTokens)
	default:
		return codexDefaultContextWindow
	}
}
