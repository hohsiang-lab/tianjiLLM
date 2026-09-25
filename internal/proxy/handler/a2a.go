package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// A2AAgentCard handles GET /a2a/{id}/.well-known/agent-card.json — returns agent card.
func (h *Handlers) A2AAgentCard(w http.ResponseWriter, r *http.Request) {
	if h.AgentRegistry == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "a2a not configured", Type: "internal_error"},
		})
		return
	}

	agentID := chi.URLParam(r, "id")
	cfg, ok := h.AgentRegistry.GetAgentByID(agentID)
	if !ok {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "agent not found", Type: "not_found"},
		})
		return
	}

	baseURL := "https://" + r.Host
	if r.TLS == nil {
		baseURL = "http://" + r.Host
	}

	card := a2a.BuildAgentCard(cfg, baseURL)
	writeJSON(w, http.StatusOK, card)
}

// A2AMessage handles POST /a2a/{id} — JSON-RPC 2.0 dispatch.
func (h *Handlers) A2AMessage(w http.ResponseWriter, r *http.Request) {
	if h.AgentRegistry == nil || h.CompletionBridge == nil {
		writeJSON(w, http.StatusServiceUnavailable, a2a.NewErrorResponse(nil, a2a.ErrCodeInternal, "a2a not configured"))
		return
	}

	agentID := chi.URLParam(r, "id")
	cfg, ok := h.AgentRegistry.GetAgentByID(agentID)
	if !ok {
		writeJSON(w, http.StatusNotFound, a2a.NewErrorResponse(nil, a2a.ErrCodeInvalidParams, "agent not found"))
		return
	}

	var rpcReq a2a.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		writeJSON(w, http.StatusBadRequest, a2a.NewErrorResponse(nil, a2a.ErrCodeParse, "invalid JSON-RPC request"))
		return
	}

	if rpcReq.JSONRPC != "2.0" {
		writeJSON(w, http.StatusBadRequest, a2a.NewErrorResponse(rpcReq.ID, a2a.ErrCodeInvalidRequest, "jsonrpc must be 2.0"))
		return
	}

	switch rpcReq.Method {
	case "message/send":
		h.handleA2ASend(w, r, cfg, &rpcReq)
	default:
		writeJSON(w, http.StatusOK, a2a.NewErrorResponse(rpcReq.ID, a2a.ErrCodeMethodNotFound, "method not found: "+rpcReq.Method))
	}
}

func (h *Handlers) handleA2ASend(w http.ResponseWriter, r *http.Request, cfg *a2a.AgentConfig, rpcReq *a2a.JSONRPCRequest) {
	var params a2a.SendMessageParams
	if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
		writeJSON(w, http.StatusOK, a2a.NewErrorResponse(rpcReq.ID, a2a.ErrCodeInvalidParams, "invalid params: "+err.Error()))
		return
	}

	result, err := h.CompletionBridge.SendMessage(r.Context(), cfg, params.Message.Content)
	if err != nil {
		writeJSON(w, http.StatusOK, a2a.NewErrorResponse(rpcReq.ID, a2a.ErrCodeInternal, err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, a2a.NewSuccessResponse(rpcReq.ID, result))
}
