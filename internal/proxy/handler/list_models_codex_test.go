package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestCodexCatalog(t *testing.T) chatgptcodex.Catalog {
	t.Helper()
	body, err := os.ReadFile("../../provider/chatgptcodex/testdata/codex_catalog_sol_terra_luna.json")
	require.NoError(t, err)
	catalog, err := chatgptcodex.ParseCatalog(body)
	require.NoError(t, err)
	return catalog
}

func TestListModelsIncludesCodexModelsResponse(t *testing.T) {
	h := newTestHandlers()
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model: "openai/*",
		},
	}}

	w := httptest.NewRecorder()
	h.ListModels(w, httptest.NewRequest(http.MethodGet, "/models", nil))

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Slug                     string `json:"slug"`
			DisplayName              string `json:"display_name"`
			SupportedReasoningLevels []struct {
				Effort      string `json:"effort"`
				Description string `json:"description"`
			} `json:"supported_reasoning_levels"`
			ShellType                 string `json:"shell_type"`
			Visibility                string `json:"visibility"`
			SupportedInAPI            bool   `json:"supported_in_api"`
			Priority                  int    `json:"priority"`
			BaseInstructions          string `json:"base_instructions"`
			TruncationPolicy          any    `json:"truncation_policy"`
			SupportsParallelToolCalls bool   `json:"supports_parallel_tool_calls"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "list", resp.Object)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "openai/*", resp.Data[0].ID)
	require.Len(t, resp.Models, 1)
	require.Equal(t, "openai/*", resp.Models[0].Slug)
	require.Equal(t, "openai/*", resp.Models[0].DisplayName)
	require.NotEmpty(t, resp.Models[0].SupportedReasoningLevels)
	require.Equal(t, "default", resp.Models[0].ShellType)
	require.Equal(t, "list", resp.Models[0].Visibility)
	require.True(t, resp.Models[0].SupportedInAPI)
	require.Equal(t, 1, resp.Models[0].Priority)
	require.NotNil(t, resp.Models[0].TruncationPolicy)
	require.True(t, resp.Models[0].SupportsParallelToolCalls)
}

func TestListModelsPreservesOpenAICompatibleShape(t *testing.T) {
	h := newTestHandlers()
	h.Config.ModelList = []config.ModelConfig{{ModelName: "gpt-4o"}}

	w := httptest.NewRecorder()
	h.ListModels(w, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "list", resp.Object)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "gpt-4o", resp.Data[0].ID)
	require.Equal(t, "model", resp.Data[0].Object)
	require.Equal(t, "tianji", resp.Data[0].OwnedBy)
}

func TestListModelsHiddenAliasesExcludedFromBothShapes(t *testing.T) {
	h := newTestHandlers()
	h.Config.ModelList = []config.ModelConfig{
		{ModelName: "visible-model"},
		{ModelName: "hidden-model"},
	}
	h.Router = router.New(h.Config.ModelList, strategy.NewShuffle(), router.RouterSettings{
		ModelGroupAlias: map[string]router.ModelGroupAliasItem{
			"hidden-model": {Model: "visible-model", Hidden: true},
		},
	})

	w := httptest.NewRecorder()
	h.ListModels(w, httptest.NewRequest(http.MethodGet, "/models", nil))

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "visible-model", resp.Data[0].ID)
	require.Len(t, resp.Models, 1)
	require.Equal(t, "visible-model", resp.Models[0].Slug)
}

func TestCodexModelInfoFromConfigUsesExplicitDefaults(t *testing.T) {
	maxInputTokens := 128000
	apiKey := "sk-sensitive-test-key"
	model := codexModelInfoFromConfig(config.ModelConfig{
		ModelName: "openai/gpt-5.5",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-5.5",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"credential-id-sensitive"},
		},
		ModelInfo: &config.ModelInfo{
			MaxInputTokens: &maxInputTokens,
		},
	}, 7)

	require.Equal(t, "openai/gpt-5.5", model.Slug)
	require.Equal(t, "openai/gpt-5.5", model.DisplayName)
	require.Equal(t, "medium", model.DefaultReasoningLevel)
	require.NotEmpty(t, model.SupportedReasoningLevels)
	require.Equal(t, "default", model.ShellType)
	require.Equal(t, "list", model.Visibility)
	require.True(t, model.SupportedInAPI)
	require.Equal(t, 7, model.Priority)
	require.Equal(t, "tokens", model.TruncationPolicy.Mode)
	require.Equal(t, int64(128000), model.TruncationPolicy.Limit)
	require.Equal(t, int64(128000), *model.ContextWindow)
	require.True(t, model.SupportsParallelToolCalls)
	require.Equal(t, []string{"text"}, model.InputModalities)

	raw, err := json.Marshal(model)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(raw), apiKey))
	require.False(t, strings.Contains(string(raw), "credential-id-sensitive"))
}

func TestListModelsSubscriptionCatalogUsesGlobalPoolForOfficialModel(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"global-credential": {AccessToken: "access-global", AccountID: "acct-global"},
	})
	now := time.Now().UTC()
	h.openAISubscriptionNow = func() time.Time { return now }
	catalog := loadTestCodexCatalog(t)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "global-credential",
		Catalog:      catalog,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "openai/gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.True(t, response.Models[0].UseResponsesLite)
	require.Equal(t, int64(372000), *response.Models[0].ContextWindow)
	require.Equal(t, []string{"text", "image"}, response.Models[0].InputModalities)
}

func TestListModelsSubscriptionCatalogAlwaysUsesGlobalPoolForOfficialModel(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"global-credential": {AccessToken: "access-global", AccountID: "acct-global"},
	})
	now := time.Now().UTC()
	h.openAISubscriptionNow = func() time.Time { return now }
	globalCatalog := loadTestCodexCatalog(t)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "global-credential",
		Catalog:      globalCatalog,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	wrongContextWindow := int64(1)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "legacy-per-model",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:          "gpt-5.6-sol",
			ContextWindow: &wrongContextWindow,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-5.6-sol",
			OpenAISubscriptionCredentialIDs: []string{"legacy-per-model"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.Equal(t, int64(372000), *response.Models[0].ContextWindow)
}

func TestListModelsSubscriptionCatalogRejectsNonOfficialSubscriptionFields(t *testing.T) {
	h := newTestHandlers()
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	wrongContextWindow := int64(1)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "organization-bound-credential",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:          "openaicompat/gpt-4o",
			ContextWindow: &wrongContextWindow,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "compat-alias",
		TianjiParams: config.TianjiParams{
			Model:                           "openaicompat/gpt-4o",
			OpenAISubscriptionCredentialIDs: []string{"organization-bound-credential"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.Equal(t, int64(128000), *response.Models[0].ContextWindow)
}

func TestListModelsSubscriptionCatalogEnrichesSolTerraLuna(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	catalog := loadTestCodexCatalog(t)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: catalog, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{
		{ModelName: "sol-alias", TianjiParams: config.TianjiParams{Model: "gpt-5.6-sol", OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend}},
		{ModelName: "terra-alias", TianjiParams: config.TianjiParams{Model: "gpt-5.6-terra", OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend}},
		{ModelName: "luna-alias", TianjiParams: config.TianjiParams{Model: "gpt-5.6-luna", OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend}},
	}
	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 3)
	require.Equal(t, "sol-alias", response.Models[0].Slug)
	require.Equal(t, int64(372000), *response.Models[0].ContextWindow)
	require.Equal(t, int64(872000), *response.Models[0].MaxContextWindow)
	require.True(t, response.Models[0].UseResponsesLite)
	require.False(t, response.Models[0].SupportsParallelToolCalls)
	require.Nil(t, response.Models[0].AutoCompactTokenLimit)
	require.Equal(t, int64(10000), response.Models[0].TruncationPolicy.Limit)
	require.Equal(t, []string{"text", "image"}, response.Models[0].InputModalities)
	require.Equal(t, "ultra", response.Models[1].DefaultReasoningLevel)
}

func TestListModelsSubscriptionCatalogExcludesPermanentCredential(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-active": {AccessToken: "access-active", AccountID: "acct-active"},
		"cred-invalid": {
			AccessToken:    "access-invalid",
			AccountID:      "acct-invalid",
			Status:         "refresh_failed",
			DisabledReason: "refresh_token_invalidated",
		},
	})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{
		catalog: loadTestCodexCatalog(t),
	}}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.Equal(t, []string{"text", "image"}, response.Models[0].InputModalities)
	require.Equal(t, "max", response.Models[0].SupportedReasoningLevels[1].Effort)
}

func TestListModelsSubscriptionCatalogKeepsTemporaryCredentialFailureInGate(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-active": {AccessToken: "access-active", AccountID: "acct-active"},
		"cred-temporary": {
			AccessToken:    "access-temporary",
			AccountID:      "acct-temporary",
			Status:         "refresh_failed",
			DisabledReason: "refresh_failed",
		},
	})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{
		catalog: loadTestCodexCatalog(t),
	}}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.Equal(t, []string{"text"}, response.Models[0].InputModalities)
}

func TestListModelsWildcardCatalogRequiresAllCredentialsToUseResponsesLite(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.5",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-b",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug: "gpt-5.5",
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                       "openai/*",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)

	require.Len(t, response.Models, 1)
	assert.False(t, response.Models[0].UseResponsesLite)
	assert.True(t, response.Models[0].SupportsParallelToolCalls)
}

func TestListModelsSubscriptionCatalogMatchesProviderPrefixedOpenAIModel(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-5.6-sol",
		TianjiParams: config.TianjiParams{
			Model:                       "openai/gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	require.Equal(t, "gpt-5.6-sol", response.Models[0].Slug)
	require.Equal(t, int64(372000), *response.Models[0].ContextWindow)
	require.Nil(t, response.Models[0].AutoCompactTokenLimit)
	require.Equal(t, int64(10000), response.Models[0].TruncationPolicy.Limit)
	require.Equal(t, []string{"text", "image"}, response.Models[0].InputModalities)
}

func TestListModelsSubscriptionCatalogMarksResponsesLiteWildcardCapability(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.5",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                       "openai/*",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	assert.Equal(t, "openai/*", response.Models[0].Slug)
	assert.True(t, response.Models[0].UseResponsesLite)
	assert.False(t, response.Models[0].SupportsParallelToolCalls)
}

func TestListModelsSubscriptionCatalogKeepsRuntimeRouteBoundary(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{{ModelName: "sol-alias", TianjiParams: config.TianjiParams{Model: "gpt-5.6-sol", OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend}}}
	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	require.Equal(t, "sol-alias", response.Models[0].Slug)
}

func TestListModelsSubscriptionCatalogFallsBackWhenCredentialsDisagree(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-b", Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-terra"}}}, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	require.Equal(t, int64(128000), *response.Models[0].ContextWindow)
	require.Equal(t, []string{"text"}, response.Models[0].InputModalities)
}

func TestListModelsSubscriptionCatalogFallsBackWhenCredentialCapabilitiesDiffer(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	firstCatalog := loadTestCodexCatalog(t)
	secondCatalog := loadTestCodexCatalog(t)
	secondCatalog.Models[0].InputModalities = []string{"text"}
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: firstCatalog, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-b", Catalog: secondCatalog, FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	require.Equal(t, int64(128000), *response.Models[0].ContextWindow)
	require.Equal(t, []string{"text"}, response.Models[0].InputModalities)
}

func TestCodexModelInfoFromConfigPreservesExplicitUpstreamNullAutoCompact(t *testing.T) {
	catalog := loadTestCodexCatalog(t)
	sol, ok := catalog.Model("gpt-5.6-sol")
	require.True(t, ok)
	info := codexModelInfoFromCatalog(codexModelInfoFromConfig(config.ModelConfig{ModelName: "sol-alias"}, 1), sol)
	require.Nil(t, info.AutoCompactTokenLimit)
}

func TestListModelsSubscriptionCatalogSerializesExplicitUpstreamNullAutoCompact(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now, ExpiresAt: now.Add(time.Hour)})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	w := httptest.NewRecorder()
	h.ListModels(w, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Models, 1)
	autoCompact, present := response.Models[0]["auto_compact_token_limit"]
	require.True(t, present)
	require.Nil(t, autoCompact)
}

func TestListModelsSubscriptionCatalogFallsBackToStaticMetadataWithoutCache(t *testing.T) {
	h := newTestHandlers()
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "sol-alias",
		TianjiParams: config.TianjiParams{
			Model:                       "gpt-5.6-sol",
			OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	response := h.buildListModelsResponse(context.Background(), h.Config.ModelList)
	require.Len(t, response.Models, 1)
	require.Equal(t, int64(128000), *response.Models[0].ContextWindow)
	require.Equal(t, []string{"text"}, response.Models[0].InputModalities)
}
