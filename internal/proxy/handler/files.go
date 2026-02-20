package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// FilesUpload handles POST /v1/files — upload a file.
func (h *Handlers) FilesUpload(w http.ResponseWriter, r *http.Request) {
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/files", apiKey, "")
}

// FilesList handles GET /v1/files — list files.
func (h *Handlers) FilesList(w http.ResponseWriter, r *http.Request) {
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/files", apiKey, "")
}

// FilesGet handles GET /v1/files/{file_id} — get file info.
func (h *Handlers) FilesGet(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/files/"+fileID, apiKey, "")
}

// FilesGetContent handles GET /v1/files/{file_id}/content — download file content.
func (h *Handlers) FilesGetContent(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/files/"+fileID+"/content", apiKey, "")
}

// FilesDelete handles DELETE /v1/files/{file_id} — delete a file.
func (h *Handlers) FilesDelete(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/files/"+fileID, apiKey, "")
}
