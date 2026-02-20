package handler

import (
	"net/http"
)

// VectorStoreFilesCreate handles POST /v1/vector_stores/{id}/files.
func (h *Handlers) VectorStoreFilesCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreFilesList handles GET /v1/vector_stores/{id}/files.
func (h *Handlers) VectorStoreFilesList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreFilesGet handles GET /v1/vector_stores/{id}/files/{file_id}.
func (h *Handlers) VectorStoreFilesGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreFilesDelete handles DELETE /v1/vector_stores/{id}/files/{file_id}.
func (h *Handlers) VectorStoreFilesDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreSearch handles POST /v1/vector_stores/{id}/search.
func (h *Handlers) VectorStoreSearch(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreCreate handles POST /v1/vector_stores.
func (h *Handlers) VectorStoreCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreList handles GET /v1/vector_stores.
func (h *Handlers) VectorStoreList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreGet handles GET /v1/vector_stores/{id}.
func (h *Handlers) VectorStoreGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// VectorStoreDelete handles DELETE /v1/vector_stores/{id}.
func (h *Handlers) VectorStoreDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
