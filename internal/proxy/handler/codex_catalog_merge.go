package handler

import (
	"slices"

	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
)

// Account entitlements may differ even when their context limits agree.
func mergeCodexCatalogModels(a, b chatgptcodex.CatalogModel) chatgptcodex.CatalogModel {
	a.ContextWindow = minimumKnownCodexLimit(a.ContextWindow, b.ContextWindow)
	a.MaxContextWindow = minimumKnownCodexLimit(a.MaxContextWindow, b.MaxContextWindow)
	a.AutoCompactTokenLimit.Present = a.AutoCompactTokenLimit.Present || b.AutoCompactTokenLimit.Present
	if a.AutoCompactTokenLimit.Value == nil {
		a.AutoCompactTokenLimit.Value = b.AutoCompactTokenLimit.Value
	} else if b.AutoCompactTokenLimit.Value != nil {
		a.AutoCompactTokenLimit.Value = minimumKnownCodexLimit(a.AutoCompactTokenLimit.Value, b.AutoCompactTokenLimit.Value)
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
	// Defaults are preferences, not capabilities; choose the lower common default.
	levels := []string{"none", "minimal", "low", "medium", "high", "xhigh", "ultra", "max"}
	defaults := []string{a.DefaultReasoningLevel, b.DefaultReasoningLevel}
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
