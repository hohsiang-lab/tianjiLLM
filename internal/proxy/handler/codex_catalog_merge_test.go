package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/stretchr/testify/require"
)

func TestListModelsMergesLowestCommonCatalogCapabilities(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
				"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
				"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
			})
			var first, second chatgptcodex.Catalog
			require.NoError(t, json.Unmarshal([]byte(`{"models":[{"slug":"gpt-6.1-sol","context_window":500000,"max_context_window":872000,"auto_compact_token_limit":null,"default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"}],"input_modalities":["text","image"],"additional_speed_tiers":["fast","ultrafast"],"service_tiers":[{"id":"priority"}],"use_responses_lite":true,"truncation_policy":{"mode":"tokens","limit":10000}}]}`), &first))
			require.NoError(t, json.Unmarshal([]byte(`{"models":[{"slug":"gpt-6.1-sol","context_window":500000,"max_context_window":872000,"auto_compact_token_limit":null,"default_reasoning_level":"low","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"}],"input_modalities":["text"],"additional_speed_tiers":[],"service_tiers":[],"use_responses_lite":false,"truncation_policy":{"mode":"tokens","limit":8000}}]}`), &second))
			if reverse {
				first, second = second, first
			}
			now := time.Now().UTC()
			h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: first, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
			h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-b", Catalog: second, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
			result := h.buildListModelsResponse(context.Background(), []config.ModelConfig{{ModelName: "gpt-6.1-sol", TianjiParams: config.TianjiParams{Model: "openai/gpt-6.1-sol"}}}).Models[0]
			require.Equal(t, int64(500000), *result.ContextWindow)
			require.Equal(t, int64(872000), *result.MaxContextWindow)
			require.True(t, result.AutoCompactTokenLimit.Present)
			require.Nil(t, result.AutoCompactTokenLimit.Value)
			require.Equal(t, "low", result.DefaultReasoningLevel)
			require.Equal(t, []codexReasoningEffortPreset{{Effort: "low"}, {Effort: "medium"}}, result.SupportedReasoningLevels)
			require.Equal(t, []string{"text"}, result.InputModalities)
			require.Empty(t, result.AdditionalSpeedTiers)
			require.Empty(t, result.ServiceTiers)
			require.False(t, result.UseResponsesLite)
			require.Equal(t, int64(8000), result.TruncationPolicy.Limit)
		})
	}
}

func TestCodexMissingMetadataDoesNotInventLimits(t *testing.T) {
	result := codexModelInfoFromConfig(config.ModelConfig{ModelName: "missing"}, 1)
	require.Nil(t, result.ContextWindow)
	require.Nil(t, result.MaxContextWindow)
	require.False(t, result.AutoCompactTokenLimit.Present)
	require.Zero(t, result.TruncationPolicy.Limit)
	require.Zero(t, result.EffectiveContextWindowPercent)
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"context_window"`)
	require.NotContains(t, string(raw), `"auto_compact_token_limit"`)
}

func TestMergeCodexCatalogLimitsAndMissingValues(t *testing.T) {
	for _, tc := range []struct {
		name, first, second       string
		context, maximum, compact *int64
	}{
		{"minimum", `{"context_window":500000,"max_context_window":872000,"auto_compact_token_limit":450000}`, `{"context_window":272000,"max_context_window":500000,"auto_compact_token_limit":200000}`, new(int64(272000)), new(int64(500000)), new(int64(200000))},
		{"null compact", `{"context_window":500000,"auto_compact_token_limit":null}`, `{"context_window":500000,"auto_compact_token_limit":200000}`, new(int64(500000)), nil, new(int64(200000))},
		{"missing limits", `{}`, `{"context_window":500000,"max_context_window":872000}`, nil, nil, nil},
		{"invalid limits", `{"context_window":0,"max_context_window":-1}`, `{"context_window":500000,"max_context_window":872000}`, nil, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var a, b chatgptcodex.CatalogModel
			require.NoError(t, json.Unmarshal([]byte(tc.first), &a))
			require.NoError(t, json.Unmarshal([]byte(tc.second), &b))
			beforeA, err := json.Marshal(a)
			require.NoError(t, err)
			for _, pair := range [][2]chatgptcodex.CatalogModel{{a, b}, {b, a}} {
				result := mergeCodexCatalogModels(pair[0], pair[1])
				require.Equal(t, tc.context, result.ContextWindow)
				require.Equal(t, tc.maximum, result.MaxContextWindow)
				require.Equal(t, tc.compact, result.AutoCompactTokenLimit.Value)
			}
			afterA, err := json.Marshal(a)
			require.NoError(t, err)
			require.Equal(t, beforeA, afterA)
		})
	}
}

func TestListModelsLogsUnavailableCatalog(t *testing.T) {
	var output bytes.Buffer
	writer := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(writer) })
	h := newTestHandlers()
	response := h.buildListModelsResponse(context.Background(), []config.ModelConfig{{ModelName: "unknown", TianjiParams: config.TianjiParams{Model: "openai/unknown"}}})
	require.Nil(t, response.Models[0].ContextWindow)
	require.Contains(t, output.String(), "Codex catalog metadata unavailable model=unknown")
}
