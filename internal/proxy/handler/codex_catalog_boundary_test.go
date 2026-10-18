package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/stretchr/testify/require"
)

func TestCodexCatalogOutputNormalizesWithoutMutatingUpstream(t *testing.T) {
	var upstream chatgptcodex.CatalogModel
	require.NoError(t, json.Unmarshal([]byte(`{"context_window":0,"max_context_window":-1,"effective_context_window_percent":101}`), &upstream))
	before, err := json.Marshal(upstream)
	require.NoError(t, err)
	info := codexModelInfoFromCatalog(codexModelInfo{}, upstream)
	require.Nil(t, info.ContextWindow)
	require.Nil(t, info.MaxContextWindow)
	require.Zero(t, info.EffectiveContextWindowPercent)
	require.False(t, info.AutoCompactTokenLimit.Present)
	after, err := json.Marshal(upstream)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

func TestCodexCatalogBoundaryMetadataIsUnknownAndLogged(t *testing.T) {
	for _, tc := range []struct {
		name              string
		overrides         []string
		omitted, warnings []string
		compact           string
	}{
		{name: "invalid percent", overrides: []string{`{"effective_context_window_percent":101,"auto_compact_token_limit":null}`, `{"auto_compact_token_limit":null}`, `{"effective_context_window_percent":90,"auto_compact_token_limit":null}`}, omitted: []string{"effective_context_window_percent"}, warnings: []string{"effective_context_window_percent"}, compact: "null"},
		{name: "missing percent", overrides: []string{`{"effective_context_window_percent":0,"auto_compact_token_limit":null}`, `{"auto_compact_token_limit":null}`}, omitted: []string{"effective_context_window_percent"}, warnings: []string{"effective_context_window_percent"}, compact: "null"},
		{name: "invalid single context", overrides: []string{`{"context_window":0,"max_context_window":-1,"auto_compact_token_limit":null}`}, omitted: []string{"context_window", "max_context_window"}, warnings: []string{"context metadata incomplete"}, compact: "null"},
		{name: "missing compact plus numeric and null", overrides: []string{`{}`, `{"auto_compact_token_limit":200000}`, `{"auto_compact_token_limit":null}`}, omitted: []string{"auto_compact_token_limit"}, warnings: []string{"auto_compact_token_limit"}},
		{name: "single missing compact", overrides: []string{`{}`}, omitted: []string{"auto_compact_token_limit"}, warnings: []string{"auto_compact_token_limit"}},
		{name: "explicit null", overrides: []string{`{"auto_compact_token_limit":null}`}, compact: "null"},
		{name: "null plus numeric", overrides: []string{`{"auto_compact_token_limit":null}`, `{"auto_compact_token_limit":200000}`}, compact: "200000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orders := [][]int{{0}}
			if len(tc.overrides) == 2 {
				orders = [][]int{{0, 1}, {1, 0}}
			}
			if len(tc.overrides) == 3 {
				orders = [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
			}
			for _, order := range orders {
				t.Run(fmt.Sprint(order), func(t *testing.T) {
					creds := map[string]openAISubscriptionTestCredential{}
					ids := make([]string, len(order))
					for i := range order {
						ids[i] = fmt.Sprintf("cred-%d", i)
						creds[ids[i]] = openAISubscriptionTestCredential{AccessToken: "test", AccountID: ids[i]}
					}
					h := newCodexCatalogHarness(t, creds)
					now := time.Now().UTC()
					for i, index := range order {
						var model chatgptcodex.CatalogModel
						require.NoError(t, json.Unmarshal([]byte(`{"slug":"model","context_window":500000,"max_context_window":872000,"effective_context_window_percent":95,"truncation_policy":{"mode":"tokens","limit":10000}}`), &model))
						require.NoError(t, json.Unmarshal([]byte(tc.overrides[index]), &model))
						before, err := json.Marshal(model)
						require.NoError(t, err)
						// Exercise the same JSON round trip as the shared catalog cache.
						var cached chatgptcodex.CatalogModel
						require.NoError(t, json.Unmarshal(before, &cached))
						require.Equal(t, model.AutoCompactTokenLimit.Present, cached.AutoCompactTokenLimit.Present)
						h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: ids[i], Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{cached}}, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
					}
					var output bytes.Buffer
					writer := log.Writer()
					log.SetOutput(&output)
					defer log.SetOutput(writer)
					response := h.buildListModelsResponse(context.Background(), []config.ModelConfig{{ModelName: "model", TianjiParams: config.TianjiParams{Model: "openai/model", OpenAISubscriptionCredentialIDs: ids}}})
					raw, err := json.Marshal(response.Models[0])
					require.NoError(t, err)
					var fields map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(raw, &fields))
					for _, field := range tc.omitted {
						require.NotContains(t, fields, field)
					}
					for _, warning := range tc.warnings {
						require.Contains(t, output.String(), warning)
					}
					if tc.compact != "" {
						require.Equal(t, tc.compact, string(fields["auto_compact_token_limit"]))
					}
					if len(tc.warnings) == 0 {
						require.Empty(t, output.String())
					}
				})
			}
		})
	}
}
