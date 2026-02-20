package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AgentCreate handles POST /v1/agents.
func (h *Handlers) AgentCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		AgentName       string   `json:"agent_name"`
		TianjiParams    any      `json:"tianji_params"`
		AgentCardParams any      `json:"agent_card_params"`
		AccessGroups    []string `json:"access_groups"`
		CreatedBy       string   `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil || req.AgentName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent_name required", Type: "invalid_request_error"},
		})
		return
	}

	tianjiJSON, _ := json.Marshal(req.TianjiParams)
	cardJSON, _ := json.Marshal(req.AgentCardParams)

	agent, err := h.DB.CreateAgent(r.Context(), db.CreateAgentParams{
		AgentName:         req.AgentName,
		TianjiParams:      tianjiJSON,
		AgentCardParams:   cardJSON,
		AgentAccessGroups: req.AccessGroups,
		CreatedBy:         req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create agent: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "created", "AgentsTable", agent.AgentID, req.CreatedBy, "", nil, req)
	writeJSON(w, http.StatusOK, agent)
}

// AgentGet handles GET /v1/agents/{agent_id}.
func (h *Handlers) AgentGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	agentID := chi.URLParam(r, "agent_id")
	if agentID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent_id required", Type: "invalid_request_error"},
		})
		return
	}

	agent, err := h.DB.GetAgent(r.Context(), agentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, agent)
}

// AgentList handles GET /v1/agents.
func (h *Handlers) AgentList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	agents, err := h.DB.ListAgents(r.Context(), db.ListAgentsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list agents: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": agents})
}

// AgentUpdate handles PUT /v1/agents/{agent_id}.
func (h *Handlers) AgentUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	agentID := chi.URLParam(r, "agent_id")
	if agentID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent_id required", Type: "invalid_request_error"},
		})
		return
	}

	var req struct {
		AgentName       string   `json:"agent_name"`
		TianjiParams    any      `json:"tianji_params"`
		AgentCardParams any      `json:"agent_card_params"`
		AccessGroups    []string `json:"access_groups"`
		UpdatedBy       string   `json:"updated_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	tianjiJSON, _ := json.Marshal(req.TianjiParams)
	cardJSON, _ := json.Marshal(req.AgentCardParams)

	agent, err := h.DB.UpdateAgent(r.Context(), db.UpdateAgentParams{
		AgentID:           agentID,
		AgentName:         req.AgentName,
		TianjiParams:      tianjiJSON,
		AgentCardParams:   cardJSON,
		AgentAccessGroups: req.AccessGroups,
		UpdatedBy:         req.UpdatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update agent: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "updated", "AgentsTable", agentID, req.UpdatedBy, "", nil, req)
	writeJSON(w, http.StatusOK, agent)
}

// AgentPatch handles PATCH /v1/agents/{agent_id}.
func (h *Handlers) AgentPatch(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	agentID := chi.URLParam(r, "agent_id")
	if agentID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent_id required", Type: "invalid_request_error"},
		})
		return
	}

	var req struct {
		AgentName       string   `json:"agent_name"`
		TianjiParams    any      `json:"tianji_params"`
		AgentCardParams any      `json:"agent_card_params"`
		AccessGroups    []string `json:"access_groups"`
		UpdatedBy       string   `json:"updated_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	var tianjiJSON, cardJSON []byte
	if req.TianjiParams != nil {
		tianjiJSON, _ = json.Marshal(req.TianjiParams)
	}
	if req.AgentCardParams != nil {
		cardJSON, _ = json.Marshal(req.AgentCardParams)
	}

	agent, err := h.DB.PatchAgent(r.Context(), db.PatchAgentParams{
		AgentID:           agentID,
		AgentName:         req.AgentName,
		TianjiParams:      tianjiJSON,
		AgentCardParams:   cardJSON,
		AgentAccessGroups: req.AccessGroups,
		UpdatedBy:         req.UpdatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "patch agent: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, agent)
}

// AgentDelete handles DELETE /v1/agents/{agent_id}.
func (h *Handlers) AgentDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	agentID := chi.URLParam(r, "agent_id")
	if agentID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteAgent(r.Context(), agentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete agent: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "deleted", "AgentsTable", agentID, "", "", nil, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "agent_id": agentID})
}
