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

func TestCodexCatalogSingleCredentialPreservesUpstreamDefault(t *testing.T) {
	model := chatgptcodex.CatalogModel{DefaultReasoningLevel: "low"}
	result := mergeCodexCatalogModels(model)
	require.Equal(t, model, result)
}

func TestCodexCatalogThreeCredentialPermutations(t *testing.T) {
	var models []chatgptcodex.CatalogModel
	require.NoError(t, json.Unmarshal([]byte(`[
 {"slug":"model","default_reasoning_level":"high","supported_reasoning_levels":[{"effort":"medium"},{"effort":"high"}],"auto_compact_token_limit":100},
 {"slug":"model","default_reasoning_level":"low","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"}],"auto_compact_token_limit":0},
 {"slug":"model","default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"}],"auto_compact_token_limit":200}
 ]`), &models))
	for _, ids := range [][]string{{"a", "b", "c"}, {"a", "c", "b"}, {"b", "a", "c"}, {"b", "c", "a"}, {"c", "a", "b"}, {"c", "b", "a"}} {
		t.Run(ids[0]+ids[1]+ids[2], func(t *testing.T) {
			result, ok := (&Handlers{}).codexCatalogForParamsWithLookup(config.TianjiParams{
				Model: "openai/model", OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend, OpenAISubscriptionCredentialIDs: ids,
			}, func(id string) OpenAISubscriptionCodexCatalogResult {
				return OpenAISubscriptionCodexCatalogResult{Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{models[int(id[0]-'a')]}}}
			})
			require.True(t, ok)
			require.Equal(t, "medium", result.DefaultReasoningLevel)
			require.NotNil(t, result.AutoCompactTokenLimit.Value)
			require.Zero(t, *result.AutoCompactTokenLimit.Value)
		})
	}
}

func TestCodexCompactNumericMinimumPreservesBoundaryValues(t *testing.T) {
	for _, tc := range []struct{ values, expected string }{
		{`100,0,200`, `0`},
		{`100,-1,200`, `-1`},
		{`100,null,200`, `100`},
		{`null,null,null`, `null`},
	} {
		t.Run(tc.values, func(t *testing.T) {
			var values []json.RawMessage
			require.NoError(t, json.Unmarshal([]byte("["+tc.values+"]"), &values))
			models := make([]chatgptcodex.CatalogModel, len(values))
			for i, value := range values {
				require.NoError(t, json.Unmarshal([]byte(`{"auto_compact_token_limit":`+string(value)+`}`), &models[i]))
			}
			before, err := json.Marshal(models)
			require.NoError(t, err)
			for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
				result := mergeCodexCatalogModels(models[order[0]], models[order[1]], models[order[2]])
				raw, marshalErr := json.Marshal(result.AutoCompactTokenLimit.Value)
				require.NoError(t, marshalErr)
				require.Equal(t, tc.expected, string(raw), "order=%v", order)
			}
			after, err := json.Marshal(models)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestCodexCatalogUnknownPolicyAndPercentAreOmittedAndLogged(t *testing.T) {
	for _, tc := range []struct{ name, metadata string }{
		{"missing", `{"slug":"model","context_window":500000,"max_context_window":872000}`},
		{"invalid", `{"slug":"model","context_window":500000,"max_context_window":872000,"effective_context_window_percent":101,"truncation_policy":{"mode":"tokens","limit":0}}`},
		{"negative", `{"slug":"model","context_window":500000,"max_context_window":872000,"effective_context_window_percent":-1,"truncation_policy":{"mode":"tokens","limit":-1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var model chatgptcodex.CatalogModel
			require.NoError(t, json.Unmarshal([]byte(tc.metadata), &model))
			var output bytes.Buffer
			writer := log.Writer()
			log.SetOutput(&output)
			t.Cleanup(func() { log.SetOutput(writer) })
			h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred": {AccessToken: "access", AccountID: "account"}})
			now := time.Now().UTC()
			h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred", Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{model}}, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
			response := h.buildListModelsResponse(context.Background(), []config.ModelConfig{{ModelName: "model", TianjiParams: config.TianjiParams{Model: "openai/model"}}})
			raw, err := json.Marshal(response.Models[0])
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			require.NotContains(t, fields, "effective_context_window_percent")
			require.NotContains(t, fields, "truncation_policy")
			require.Equal(t, "500000", string(fields["context_window"]))
			require.Contains(t, output.String(), "effective_context_window_percent")
			require.Contains(t, output.String(), "truncation_policy")
		})
	}
}

func TestCodexCatalogConflictingPoliciesRemainUnknown(t *testing.T) {
	var models []chatgptcodex.CatalogModel
	require.NoError(t, json.Unmarshal([]byte(`[
 {"truncation_policy":{"mode":"tokens","limit":100}},
 {"truncation_policy":{"mode":"bytes","limit":100}},
 {"truncation_policy":{"mode":"tokens","limit":200}}
 ]`), &models))
	result := mergeCodexCatalogModels(models[0], models[1:]...)
	info := codexModelInfoFromCatalog(codexModelInfo{}, result)
	raw, err := json.Marshal(info)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.NotContains(t, fields, "truncation_policy")
}
