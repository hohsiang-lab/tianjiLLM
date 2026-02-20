package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// RAGIngest handles POST /v1/rag/ingest — document ingestion pipeline.
// Forwards to the configured RAG provider (or returns not-configured if none).
func (h *Handlers) RAGIngest(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "rag not configured", Type: "internal_error"},
		})
		return
	}

	h.proxyPassthrough(w, r, "rag/ingest")
}

// RAGQuery handles POST /v1/rag/query — RAG query pipeline.
func (h *Handlers) RAGQuery(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "rag not configured", Type: "internal_error"},
		})
		return
	}

	h.proxyPassthrough(w, r, "rag/query")
}
