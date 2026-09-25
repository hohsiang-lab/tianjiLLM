package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// proxyModelResponse wraps ProxyModelTable for JSON responses, ensuring
// JSONB fields (tianji_params, model_info) are serialized as JSON objects
// rather than base64-encoded byte arrays.
type proxyModelResponse struct {
	ModelID      string             `json:"model_id"`
	ModelName    string             `json:"model_name"`
	TianjiParams json.RawMessage    `json:"tianji_params"`
	ModelInfo    json.RawMessage    `json:"model_info"`
	CreatedAt    pgtype.Timestamptz `json:"created_at"`
	CreatedBy    string             `json:"created_by"`
	UpdatedAt    pgtype.Timestamptz `json:"updated_at"`
	UpdatedBy    string             `json:"updated_by"`
}

func normalizeProxyModelTianjiParams(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return raw, nil
	}
	params := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	modelRaw := params["model"]
	if len(modelRaw) == 0 {
		modelRaw = params["Model"]
		if len(modelRaw) > 0 {
			params["model"] = modelRaw
			delete(params, "Model")
		}
	}
	var modelName string
	if err := json.Unmarshal(modelRaw, &modelName); err != nil {
		return raw, nil
	}
	providerName, _ := provider.ParseModelName(modelName)
	if strings.TrimSpace(modelName) == "" || providerName != "openai" {
		return raw, nil
	}

	for _, key := range []string{
		"api_key", "APIKey",
		"api_base", "APIBase",
		"openai_subscription_credential_ids", "OpenAISubscriptionCredentialIDs",
		"openai_subscription_transport", "OpenAISubscriptionTransport",
	} {
		delete(params, key)
	}
	transport, err := json.Marshal(config.OpenAISubscriptionTransportChatGPTCodexBackend)
	if err != nil {
		return nil, err
	}
	params["openai_subscription_transport"] = transport
	return json.Marshal(params)
}

func normalizedProxyModelTianjiParams(raw json.RawMessage) json.RawMessage {
	normalized, err := normalizeProxyModelTianjiParams(raw)
	if err != nil {
		return raw
	}
	return normalized
}

func normalizedProxyModelInfo(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return raw
	}
	info := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &info); err != nil {
		return raw
	}
	delete(info, "access_control")
	normalized, err := json.Marshal(info)
	if err != nil {
		return raw
	}
	return normalized
}

func toProxyModelResponse(m db.ProxyModelTable) proxyModelResponse {
	return proxyModelResponse{
		ModelID:      m.ModelID,
		ModelName:    m.ModelName,
		TianjiParams: normalizedProxyModelTianjiParams(m.TianjiParams),
		ModelInfo:    normalizedProxyModelInfo(m.ModelInfo),
		CreatedAt:    m.CreatedAt,
		CreatedBy:    m.CreatedBy,
		UpdatedAt:    m.UpdatedAt,
		UpdatedBy:    m.UpdatedBy,
	}
}

// ModelNew handles POST /model/new.
func (h *Handlers) ModelNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ModelID      string          `json:"model_id"`
		ModelName    string          `json:"model_name"`
		TianjiParams json.RawMessage `json:"tianji_params"`
		ModelInfo    json.RawMessage `json:"model_info"`
		CreatedBy    string          `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	normalizedTianjiParams, err := normalizeProxyModelTianjiParams(req.TianjiParams)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid tianji_params: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	normalizedModelInfo := normalizedProxyModelInfo(req.ModelInfo)

	result, err := h.DB.CreateProxyModel(r.Context(), db.CreateProxyModelParams{
		ModelID:      req.ModelID,
		ModelName:    req.ModelName,
		TianjiParams: []byte(normalizedTianjiParams),
		ModelInfo:    []byte(normalizedModelInfo),
		CreatedBy:    req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create model: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	if err := h.RefreshRuntimeModels(r.Context()); err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"model":           toProxyModelResponse(result),
			"runtime_refresh": "failed",
		})
		return
	}

	writeJSON(w, http.StatusCreated, toProxyModelResponse(result))
}

// ModelInfo handles GET /model/info.
func (h *Handlers) ModelInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	modelID := r.URL.Query().Get("model_id")
	if modelID != "" {
		result, err := h.DB.GetProxyModel(r.Context(), modelID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "model not found", Type: "not_found"},
			})
			return
		}
		writeJSON(w, http.StatusOK, toProxyModelResponse(result))
		return
	}

	result, err := h.DB.ListProxyModels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list models: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	resp := make([]proxyModelResponse, len(result))
	for i, m := range result {
		resp[i] = toProxyModelResponse(m)
	}
	writeJSON(w, http.StatusOK, resp)
}

// ModelUpdate handles POST /model/update.
func (h *Handlers) ModelUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ModelID      string          `json:"model_id"`
		ModelName    string          `json:"model_name"`
		TianjiParams json.RawMessage `json:"tianji_params"`
		ModelInfo    json.RawMessage `json:"model_info"`
		UpdatedBy    string          `json:"updated_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	normalizedTianjiParams, err := normalizeProxyModelTianjiParams(req.TianjiParams)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid tianji_params: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	normalizedModelInfo := normalizedProxyModelInfo(req.ModelInfo)

	result, err := h.DB.UpdateProxyModel(r.Context(), db.UpdateProxyModelParams{
		ModelID:      req.ModelID,
		ModelName:    req.ModelName,
		TianjiParams: []byte(normalizedTianjiParams),
		ModelInfo:    []byte(normalizedModelInfo),
		UpdatedBy:    req.UpdatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update model: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	if err := h.RefreshRuntimeModels(r.Context()); err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"model":           toProxyModelResponse(result),
			"runtime_refresh": "failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, toProxyModelResponse(result))
}

// ModelDelete handles POST /model/delete.
func (h *Handlers) ModelDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	modelID := chi.URLParam(r, "model_id")
	if modelID == "" {
		var req struct {
			ModelID string `json:"model_id"`
		}
		if err := decodeJSON(r, &req); err == nil {
			modelID = req.ModelID
		}
	}

	if err := h.DB.DeleteProxyModel(r.Context(), modelID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete model: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	if err := h.RefreshRuntimeModels(r.Context()); err != nil {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "deleted", "runtime_refresh": "failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
