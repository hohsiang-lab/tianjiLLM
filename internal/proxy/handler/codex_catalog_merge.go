package handler

import (
	"slices"

	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
)

// Account entitlements may differ even when their context limits agree.
func mergeCodexCatalogModels(a chatgptcodex.CatalogModel, others ...chatgptcodex.CatalogModel) chatgptcodex.CatalogModel {
	a = normalizeCodexCatalogLimits(a)
	if len(others) == 0 {
		return a
	}
	defaults := []string{a.DefaultReasoningLevel}
	for _, b := range others {
		b = normalizeCodexCatalogLimits(b)
		defaults = append(defaults, b.DefaultReasoningLevel)
		a.ContextWindow = minimumKnownCodexLimit(a.ContextWindow, b.ContextWindow)
		a.MaxContextWindow = minimumKnownCodexLimit(a.MaxContextWindow, b.MaxContextWindow)
		a.AutoCompactTokenLimit.Present = a.AutoCompactTokenLimit.Present && b.AutoCompactTokenLimit.Present
		if a.AutoCompactTokenLimit.Value == nil {
			a.AutoCompactTokenLimit.Value = b.AutoCompactTokenLimit.Value
		} else if b.AutoCompactTokenLimit.Value != nil {
			value := min(*a.AutoCompactTokenLimit.Value, *b.AutoCompactTokenLimit.Value)
			a.AutoCompactTokenLimit.Value = &value
		}
		a.EffectiveContextWindowPercent = min(a.EffectiveContextWindowPercent, b.EffectiveContextWindowPercent)
		a.SupportedReasoningLevels = intersectCodexCapabilities(a.SupportedReasoningLevels, b.SupportedReasoningLevels, func(v chatgptcodex.ReasoningEffortPreset) string { return v.Effort })
		a.AdditionalSpeedTiers = intersectCodexCapabilities(a.AdditionalSpeedTiers, b.AdditionalSpeedTiers, func(v string) string { return v })
		a.InputModalities = intersectCodexCapabilities(a.InputModalities, b.InputModalities, func(v string) string { return v })
		a.ServiceTiers = intersectCodexCapabilities(a.ServiceTiers, b.ServiceTiers, func(v chatgptcodex.ModelServiceTier) string { return v.ID })
		a.UseResponsesLite = a.UseResponsesLite && b.UseResponsesLite
		if a.TruncationPolicy.Mode != b.TruncationPolicy.Mode {
			a.TruncationPolicy = chatgptcodex.TruncationPolicy{}
		} else {
			a.TruncationPolicy.Limit = min(a.TruncationPolicy.Limit, b.TruncationPolicy.Limit)
		}
	}
	if !a.AutoCompactTokenLimit.Present {
		a.AutoCompactTokenLimit.Value = nil
	}
	// Select once: pairwise selection loses defaults before the final intersection.
	levels := []string{"none", "minimal", "low", "medium", "high", "xhigh", "ultra", "max"}
	a.DefaultReasoningLevel = ""
	for _, level := range levels {
		if slices.Contains(defaults, level) && slices.ContainsFunc(a.SupportedReasoningLevels, func(v chatgptcodex.ReasoningEffortPreset) bool { return v.Effort == level }) {
			a.DefaultReasoningLevel = level
			break
		}
	}
	if a.DefaultReasoningLevel == "" {
		for _, level := range levels {
			if slices.ContainsFunc(a.SupportedReasoningLevels, func(v chatgptcodex.ReasoningEffortPreset) bool { return v.Effort == level }) {
				a.DefaultReasoningLevel = level
				break
			}
		}
	}
	return a
}

// Normalize before merging so invalid values cannot be hidden by valid minima.
func normalizeCodexCatalogLimits(model chatgptcodex.CatalogModel) chatgptcodex.CatalogModel {
	if model.ContextWindow != nil && *model.ContextWindow <= 0 {
		model.ContextWindow = nil
	}
	if model.MaxContextWindow != nil && *model.MaxContextWindow <= 0 {
		model.MaxContextWindow = nil
	}
	if model.EffectiveContextWindowPercent < 0 || model.EffectiveContextWindowPercent > maxCodexContextWindowPercent {
		model.EffectiveContextWindowPercent = 0
	}
	if !model.AutoCompactTokenLimit.Present {
		model.AutoCompactTokenLimit.Value = nil
	}
	return model
}

// A missing context limit is unknown, not permission to advertise a larger one.
func minimumKnownCodexLimit(a, b *int64) *int64 {
	if a == nil || b == nil || *a <= 0 || *b <= 0 {
		return nil
	}
	value := min(*a, *b)
	return &value
}

// ponytail: quadratic scan for small catalog lists; use keyed sets if they grow.
func intersectCodexCapabilities[T any](a, b []T, key func(T) string) []T {
	result := make([]T, 0, len(a))
	for _, value := range a {
		if slices.ContainsFunc(b, func(other T) bool { return key(value) == key(other) }) {
			result = append(result, value)
		}
	}
	return result
}
