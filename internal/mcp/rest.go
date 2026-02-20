package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RESTHandler provides REST wrappers for MCP operations.
type RESTHandler struct {
	Manager *MCPServerManager
}

// Handler returns an http.Handler with routes for the REST API.
func (h *RESTHandler) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/tools/list", h.ListTools)
	r.Post("/tools/call", h.CallTool)
	return r
}

// ListTools handles GET /mcp-rest/tools/list.
func (h *RESTHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	tools := h.Manager.ListTools()

	type toolResponse struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema any    `json:"inputSchema"`
	}

	resp := struct {
		Tools []toolResponse `json:"tools"`
	}{
		Tools: make([]toolResponse, 0, len(tools)),
	}

	for _, t := range tools {
		resp.Tools = append(resp.Tools, toolResponse{
			Name:        t.PrefixedName,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CallTool handles POST /mcp-rest/tools/call.
func (h *RESTHandler) CallTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	result, err := h.Manager.CallTool(r.Context(), req.Name, req.Arguments)
	if err != nil {
		http.Error(w, `{"error":"tool call failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
