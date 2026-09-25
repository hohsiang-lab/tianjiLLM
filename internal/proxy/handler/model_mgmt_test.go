package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestModelNew_NormalizesOfficialOpenAILegacyFields(t *testing.T) {
	m := newMockStore()
	var received db.CreateProxyModelParams
	m.createProxyModelFn = func(_ context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
		received = arg
		return db.ProxyModelTable{ModelID: arg.ModelID, ModelName: arg.ModelName, TianjiParams: arg.TianjiParams, ModelInfo: arg.ModelInfo}, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) { return nil, nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/new", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4o","tianji_params":{"model":"openai/gpt-4o","api_key":"legacy-api-key","api_base":"https://platform.example/v1","openai_subscription_credential_ids":["legacy"],"openai_subscription_transport":"direct_openai_http"},"model_info":{"mode":"chat","access_control":{"allowed_orgs":["org-legacy"]}}}`))
	r.Header.Set("Content-Type", "application/json")

	h.ModelNew(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var params map[string]any
	assert.NoError(t, json.Unmarshal(received.TianjiParams, &params))
	assert.NotContains(t, params, "api_key")
	assert.NotContains(t, params, "api_base")
	assert.NotContains(t, params, "openai_subscription_credential_ids")
	assert.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, params["openai_subscription_transport"])

	var modelInfo map[string]any
	assert.NoError(t, json.Unmarshal(received.ModelInfo, &modelInfo))
	assert.Equal(t, "chat", modelInfo["mode"])
	assert.NotContains(t, modelInfo, "access_control")
}

func TestToProxyModelResponse_OmitsLegacyModelAccessControl(t *testing.T) {
	response := toProxyModelResponse(db.ProxyModelTable{
		ModelInfo: []byte(`{"mode":"chat","access_control":{"allowed_orgs":["org-legacy"]}}`),
	})

	var modelInfo map[string]any
	assert.NoError(t, json.Unmarshal(response.ModelInfo, &modelInfo))
	assert.Equal(t, "chat", modelInfo["mode"])
	assert.NotContains(t, modelInfo, "access_control")
}

func TestNormalizeProxyModelTianjiParams_CanonicalizesLegacyModelKey(t *testing.T) {
	raw := json.RawMessage(`{"Model":"openai/gpt-4o","api_key":"legacy-api-key"}`)
	normalized, err := normalizeProxyModelTianjiParams(raw)
	assert.NoError(t, err)

	var params map[string]any
	assert.NoError(t, json.Unmarshal(normalized, &params))
	assert.Equal(t, "openai/gpt-4o", params["model"])
	assert.NotContains(t, params, "Model")
	assert.NotContains(t, params, "api_key")
	assert.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, params["openai_subscription_transport"])
}

func TestModelUpdate_NormalizesLegacyModelAccessControl(t *testing.T) {
	m := newMockStore()
	var received db.UpdateProxyModelParams
	m.updateProxyModelFn = func(_ context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error) {
		received = arg
		return db.ProxyModelTable{ModelID: arg.ModelID, ModelName: arg.ModelName, TianjiParams: arg.TianjiParams, ModelInfo: arg.ModelInfo}, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) { return nil, nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/update", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4o","tianji_params":{},"model_info":{"mode":"chat","access_control":{"allowed_orgs":["org-legacy"]}}}`))
	r.Header.Set("Content-Type", "application/json")

	h.ModelUpdate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var modelInfo map[string]any
	assert.NoError(t, json.Unmarshal(received.ModelInfo, &modelInfo))
	assert.Equal(t, "chat", modelInfo["mode"])
	assert.NotContains(t, modelInfo, "access_control")
}

func TestModelNew_PreservesCompatibilityAPIKeyFields(t *testing.T) {
	m := newMockStore()
	var received db.CreateProxyModelParams
	m.createProxyModelFn = func(_ context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
		received = arg
		return db.ProxyModelTable{ModelID: arg.ModelID, ModelName: arg.ModelName, TianjiParams: arg.TianjiParams, ModelInfo: arg.ModelInfo}, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) { return nil, nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/new", strings.NewReader(`{"model_id":"m1","model_name":"compat","tianji_params":{"model":"openaicompat/gpt-4o","api_key":"compat-api-key","api_base":"https://compat.example/v1"},"model_info":{}}`))
	r.Header.Set("Content-Type", "application/json")

	h.ModelNew(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var params map[string]any
	assert.NoError(t, json.Unmarshal(received.TianjiParams, &params))
	assert.Equal(t, "compat-api-key", params["api_key"])
	assert.Equal(t, "https://compat.example/v1", params["api_base"])
}

func TestModelNew_Success(t *testing.T) {
	m := newMockStore()
	m.createProxyModelFn = func(_ context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: arg.ModelID, ModelName: arg.ModelName, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return nil, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/new", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4","tianji_params":{},"model_info":{}}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelNew(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestModelInfo_ByID(t *testing.T) {
	m := newMockStore()
	m.getProxyModelFn = func(_ context.Context, id string) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: id, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/model/info?model_id=m1", nil)
	h.ModelInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelInfo_List(t *testing.T) {
	m := newMockStore()
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return []db.ProxyModelTable{{ModelID: "m1", TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/model/info", nil)
	h.ModelInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateProxyModelFn = func(_ context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: arg.ModelID, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return nil, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/update", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4o","tianji_params":{},"model_info":{}}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteProxyModelFn = func(_ context.Context, _ string) error { return nil }
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return nil, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/delete", strings.NewReader(`{"model_id":"m1"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
