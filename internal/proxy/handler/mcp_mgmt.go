package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type mcpServerRequest struct {
	ServerID        string            `json:"server_id"`
	Alias           string            `json:"alias"`
	Transport       string            `json:"transport"`
	URL             string            `json:"url"`
	Command         string            `json:"command"`
	Args            []string          `json:"args"`
	AuthType        string            `json:"auth_type"`
	AuthToken       string            `json:"auth_token"`
	StaticHeaders   map[string]string `json:"static_headers"`
	AllowedTools    []string          `json:"allowed_tools"`
	DisallowedTools []string          `json:"disallowed_tools"`
}

func (req *mcpServerRequest) headersJSON() []byte {
	if len(req.StaticHeaders) == 0 {
		return []byte("{}")
	}
	b, _ := json.Marshal(req.StaticHeaders)
	return b
}

// MCPServerCreate handles POST /mcp_server.
func (h *Handlers) MCPServerCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req mcpServerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreateMCPServer(r.Context(), db.CreateMCPServerParams{
		ServerID:        req.ServerID,
		Alias:           req.Alias,
		Transport:       req.Transport,
		Url:             req.URL,
		Command:         req.Command,
		Args:            req.Args,
		AuthType:        req.AuthType,
		AuthToken:       req.AuthToken,
		StaticHeaders:   req.headersJSON(),
		AllowedTools:    req.AllowedTools,
		DisallowedTools: req.DisallowedTools,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create mcp server: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// MCPServerGet handles GET /mcp_server/{id}.
func (h *Handlers) MCPServerGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetMCPServer(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "mcp server not found", Type: "not_found"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// MCPServerList handles GET /mcp_server/list.
func (h *Handlers) MCPServerList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListMCPServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list mcp servers: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// MCPServerUpdate handles PUT /mcp_server/{id}.
func (h *Handlers) MCPServerUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	var req mcpServerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UpdateMCPServer(r.Context(), db.UpdateMCPServerParams{
		ID:              id,
		Alias:           req.Alias,
		Transport:       req.Transport,
		Url:             req.URL,
		Command:         req.Command,
		Args:            req.Args,
		AuthType:        req.AuthType,
		AuthToken:       req.AuthToken,
		StaticHeaders:   req.headersJSON(),
		AllowedTools:    req.AllowedTools,
		DisallowedTools: req.DisallowedTools,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update mcp server: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// MCPServerDelete handles DELETE /mcp_server/{id}.
func (h *Handlers) MCPServerDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.DB.DeleteMCPServer(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete mcp server: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
