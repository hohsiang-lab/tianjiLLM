package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/require"

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

func TestRuntimeModelSource_ListModelsIncludesDBManagedModel(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openai/gpt-4o-mini", ""))

	w := httptest.NewRecorder()
	h.ListModels(w, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"id":"db-chat-model"`)
	require.Contains(t, w.Body.String(), `"slug":"db-chat-model"`)
}

func TestRuntimeModelSource_LegacyModelAccessControlDoesNotRestrict(t *testing.T) {
	modelInfo, err := json.Marshal(map[string]any{
		"mode": "chat",
		"access_control": map[string]any{
			"allowed_orgs": []string{"org-allowed"},
		},
	})
	require.NoError(t, err)

	h := runtimeModelSourceTestHandlers(t, nil, db.ProxyModelTable{
		ModelID:      "legacy-access-model-id",
		ModelName:    "legacy-access-model",
		TianjiParams: []byte(`{"model":"openai/gpt-4o-mini","api_key":"db-api-key"}`),
		ModelInfo:    modelInfo,
		CreatedBy:    "test",
		UpdatedBy:    "test",
	})
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-other")
	ctx = context.WithValue(ctx, middleware.ContextKeyTeamID, "team-other")
	ctx = context.WithValue(ctx, middleware.ContextKeyTokenHash, "key-other")

	listResp := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/models", nil).WithContext(ctx)
	h.ListModels(listResp, listReq)

	require.Equal(t, http.StatusOK, listResp.Code)
	require.Contains(t, listResp.Body.String(), `"id":"legacy-access-model"`)

	routed, _, err := h.runtimeRouter(ctx).Route(ctx, "legacy-access-model", nil)
	require.NoError(t, err)
	require.NotNil(t, routed)
}

func TestRuntimeModelSource_FindModelConfigUsesDBManagedModel(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openai/gpt-4o-mini", "http://db-upstream/v1"))

	cfg, resolvedModel := h.findModelConfig("db-chat-model")

	require.NotNil(t, cfg)
	require.Equal(t, "openai/gpt-4o-mini", cfg.TianjiParams.Model)
	require.Equal(t, "openai/gpt-4o-mini", resolvedModel)
	require.Nil(t, cfg.TianjiParams.APIBase)
	require.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, cfg.TianjiParams.OpenAISubscriptionTransport)
}

func TestRuntimeModelSource_DBManagedOfficialModelUsesGlobalCodexPool(t *testing.T) {
	params, err := json.Marshal(map[string]any{
		"model":                              "openai/*",
		"openai_subscription_credential_ids": []string{"cred-a"},
		"openai_subscription_transport":      config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})
	require.NoError(t, err)
	modelInfo, err := json.Marshal(map[string]any{"mode": "chat"})
	require.NoError(t, err)

	h := runtimeModelSourceTestHandlers(t, nil, db.ProxyModelTable{
		ModelID:      "db-codex-model-id",
		ModelName:    "db-codex-model",
		TianjiParams: params,
		ModelInfo:    modelInfo,
		CreatedBy:    "test",
		UpdatedBy:    "test",
	})

	cfg, resolvedModel := h.findModelConfig("db-codex-model")

	require.NotNil(t, cfg)
	require.Equal(t, "openai/*", resolvedModel)
	require.Empty(t, cfg.TianjiParams.OpenAISubscriptionCredentialIDs)
	require.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, cfg.TianjiParams.OpenAISubscriptionTransport)
}

func TestRuntimeModelSource_ChatCompletionRoutesDBManagedModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer db-api-key", r.Header.Get("Authorization"))

		var req struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "gpt-4o-mini", req.Model)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(upstream.Close)

	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openaicompat/gpt-4o-mini", upstream.URL))

	body := `{"model":"db-chat-model","messages":[{"role":"user","content":"hi"}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ChatCompletion(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"model":"gpt-4o-mini"`)
}

func TestRuntimeModelSource_MergedRouterPreservesExistingSettings(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openai/gpt-4o-mini", ""))
	h.Router = router.New(nil, strategy.NewShuffle(), router.RouterSettings{
		ModelGroupAlias: map[string]router.ModelGroupAliasItem{
			"friendly-alias": {Model: "db-chat-model"},
		},
	})

	d, _, err := h.runtimeRouter(context.Background()).Route(context.Background(), "friendly-alias", nil)

	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, "gpt-4o-mini", d.ModelName)
}

func TestRuntimeModelSource_DuplicateNameDBOverridesYAML(t *testing.T) {
	yamlBase := "http://yaml-upstream/v1"
	h := runtimeModelSourceTestHandlers(t, []config.ModelConfig{{
		ModelName: "shared-model",
		TianjiParams: config.TianjiParams{
			Model:   "openai/yaml-model",
			APIBase: &yamlBase,
		},
	}}, dbProxyModel("shared-model", "openai/db-model", "http://db-upstream/v1"))

	cfg, resolvedModel := h.findModelConfig("shared-model")

	require.NotNil(t, cfg)
	require.Equal(t, "openai/db-model", cfg.TianjiParams.Model)
	require.Equal(t, "openai/db-model", resolvedModel)
	require.Nil(t, cfg.TianjiParams.APIBase)
	require.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, cfg.TianjiParams.OpenAISubscriptionTransport)
}

func TestRuntimeModelSource_DBWildcardUsesExistingSpecificityRules(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil,
		dbProxyModel("db/*", "openai/generic-*", "http://generic-upstream/v1"),
		dbProxyModel("db/special-*", "openai/special-*", "http://special-upstream/v1"),
	)

	cfg, resolvedModel := h.findModelConfig("db/special-alpha")

	require.NotNil(t, cfg)
	require.Equal(t, "openai/special-*", cfg.TianjiParams.Model)
	require.Equal(t, "openai/special-alpha", resolvedModel)
}

func TestProxyModelToConfig_DecodesGoStyleTianjiParams(t *testing.T) {
	cfg, err := proxyModelToConfig(db.ProxyModelTable{
		ModelName:    "legacy-compat",
		TianjiParams: []byte(`{"Model":"openaicompat/gpt-4o-mini","APIKey":"[REDACTED]","APIBase":"http://legacy-upstream/v1","APIVersion":"2024-01-01","TPM":100,"RPM":10,"Timeout":15,"Region":"us-east","OpenAISubscriptionCredentialIDs":["legacy-cred"],"OpenAISubscriptionTransport":"direct_openai_http","Unknown":"kept"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "openaicompat/gpt-4o-mini", cfg.TianjiParams.Model)
	require.NotNil(t, cfg.TianjiParams.APIKey)
	require.Equal(t, "[REDACTED]", *cfg.TianjiParams.APIKey)
	require.NotNil(t, cfg.TianjiParams.APIBase)
	require.Equal(t, "http://legacy-upstream/v1", *cfg.TianjiParams.APIBase)
	require.Equal(t, "2024-01-01", *cfg.TianjiParams.APIVersion)
	require.Equal(t, int64(100), *cfg.TianjiParams.TPM)
	require.Equal(t, int64(10), *cfg.TianjiParams.RPM)
	require.Equal(t, 15, *cfg.TianjiParams.Timeout)
	require.Equal(t, "us-east", cfg.TianjiParams.Region)
	require.Equal(t, []string{"legacy-cred"}, cfg.TianjiParams.OpenAISubscriptionCredentialIDs)
	require.Equal(t, "direct_openai_http", cfg.TianjiParams.OpenAISubscriptionTransport)
	require.Equal(t, "kept", cfg.TianjiParams.Overflow["Unknown"])
}

func TestProxyModelToConfig_NormalizesOfficialOpenAILegacyFields(t *testing.T) {
	cfg, err := proxyModelToConfig(db.ProxyModelTable{
		ModelName:    "official-legacy",
		TianjiParams: []byte(`{"model":"openai/gpt-4o","api_key":"[REDACTED]","api_base":"https://legacy.example/v1","openai_subscription_credential_ids":["legacy-credential"],"openai_subscription_transport":"direct_openai_http"}`),
	})
	require.NoError(t, err)
	require.Nil(t, cfg.TianjiParams.APIKey)
	require.Nil(t, cfg.TianjiParams.APIBase)
	require.Empty(t, cfg.TianjiParams.OpenAISubscriptionCredentialIDs)
	require.Equal(t, "chatgpt_codex_backend", cfg.TianjiParams.OpenAISubscriptionTransport)
}

func TestRuntimeModelSource_ResolveProviderBaseURLUsesDBManagedModel(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openaicompat/gpt-4o-mini", "http://db-upstream/v1"))

	baseURL, apiKey, err := h.resolveProviderBaseURLWithContext(context.Background(), "db-chat-model")

	require.NoError(t, err)
	require.Equal(t, "http://db-upstream/v1", baseURL)
	require.Equal(t, "db-api-key", apiKey)
}

func TestRuntimeModelSource_ResolveProviderBaseURLFallbackUsesDBOpenAIModel(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openaicompat/gpt-4o-mini", "http://db-upstream/v1"))

	baseURL, apiKey, err := h.resolveProviderBaseURLWithContext(context.Background(), "")

	require.NoError(t, err)
	require.Equal(t, "http://db-upstream/v1", baseURL)
	require.Equal(t, "db-api-key", apiKey)
}

func TestRuntimeModelSource_ResolveProviderBaseURLRejectsCompatibilitySubscriptionFields(t *testing.T) {
	params, err := json.Marshal(map[string]any{
		"model":                              "openaicompat/gpt-4o-mini",
		"api_key":                            "[REDACTED]",
		"openai_subscription_credential_ids": []string{"legacy-subscription-id"},
		"openai_subscription_transport":      config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})
	require.NoError(t, err)
	modelInfo, err := json.Marshal(map[string]any{"mode": "chat"})
	require.NoError(t, err)

	h := runtimeModelSourceTestHandlers(t, nil, db.ProxyModelTable{
		ModelID:      "db-compat-model-id",
		ModelName:    "db-compat-model",
		TianjiParams: params,
		ModelInfo:    modelInfo,
		CreatedBy:    "test",
		UpdatedBy:    "test",
	})

	_, _, err = h.resolveProviderBaseURLWithContext(context.Background(), "db-compat-model")
	require.Error(t, err)
	require.ErrorContains(t, err, "only support the official OpenAI provider")
}

func TestRuntimeModelSource_ModelGroupInfoIncludesDBManagedModel(t *testing.T) {
	h := runtimeModelSourceTestHandlers(t, nil, dbProxyModel("db-chat-model", "openai/gpt-4o-mini", ""))

	w := httptest.NewRecorder()
	h.ModelGroupInfo(w, httptest.NewRequest(http.MethodGet, "/model_group/info", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"model_group":"db-chat-model"`)
	require.Contains(t, w.Body.String(), `"providers":["openai"]`)
}

func TestModelManagementRefreshesRuntimeModelSource(t *testing.T) {
	row := dbProxyModel("db-created-model", "openai/gpt-4o-mini", "")
	m := newMockStore()
	m.createProxyModelFn = func(_ context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
		row.ModelID = arg.ModelID
		row.ModelName = arg.ModelName
		row.TianjiParams = arg.TianjiParams
		row.ModelInfo = arg.ModelInfo
		return row, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return []db.ProxyModelTable{row}, nil
	}
	h := &Handlers{
		Config:    &config.ProxyConfig{},
		DB:        m,
		Callbacks: callback.NewRegistry(),
	}

	reqBody := `{"model_id":"db-created-id","model_name":"db-created-model","tianji_params":{"model":"openai/gpt-4o-mini","api_key":"db-api-key"},"model_info":{},"created_by":"test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/model/new", strings.NewReader(reqBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	h.ModelNew(createResp, createReq)
	require.Equal(t, http.StatusCreated, createResp.Code)

	listResp := httptest.NewRecorder()
	h.ListModels(listResp, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	require.Equal(t, http.StatusOK, listResp.Code)
	require.Contains(t, listResp.Body.String(), `"id":"db-created-model"`)
}

func runtimeModelSourceTestHandlers(t *testing.T, yamlModels []config.ModelConfig, dbModels ...db.ProxyModelTable) *Handlers {
	t.Helper()

	m := newMockStore()
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return dbModels, nil
	}
	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList: yamlModels,
		},
		DB:        m,
		Callbacks: callback.NewRegistry(),
	}
	if len(yamlModels) > 0 {
		h.Router = router.New(yamlModels, strategy.NewShuffle(), router.RouterSettings{})
	}
	return h
}

func dbProxyModel(modelName string, upstreamModel string, apiBase string) db.ProxyModelTable {
	params := map[string]any{
		"model":   upstreamModel,
		"api_key": "db-api-key",
	}
	if apiBase != "" {
		params["api_base"] = apiBase
		params["supports_non_stream"] = true
	}
	tianjiParams, _ := json.Marshal(params)
	modelInfo, _ := json.Marshal(map[string]any{
		"mode": "chat",
	})

	return db.ProxyModelTable{
		ModelID:      modelName + "-id",
		ModelName:    modelName,
		TianjiParams: tianjiParams,
		ModelInfo:    modelInfo,
		CreatedBy:    "test",
		UpdatedBy:    "test",
	}
}
