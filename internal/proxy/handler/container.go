package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ContainerCreate handles POST /v1/containers.
func (h *Handlers) ContainerCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "containers")
}

// ContainerGet handles GET /v1/containers/{container_id}.
func (h *Handlers) ContainerGet(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "container_id")
	r.Header.Set("X-TianjiLLM-Container-ID", containerID)
	h.proxyPassthrough(w, r, "containers")
}

// ContainerList handles GET /v1/containers.
func (h *Handlers) ContainerList(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "containers")
}

// ContainerDelete handles DELETE /v1/containers/{container_id}.
func (h *Handlers) ContainerDelete(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "container_id")
	r.Header.Set("X-TianjiLLM-Container-ID", containerID)
	h.proxyPassthrough(w, r, "containers")
}

// ContainerFiles handles POST/GET/DELETE on /v1/containers/{container_id}/files/*.
func (h *Handlers) ContainerFiles(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "container_id")
	r.Header.Set("X-TianjiLLM-Container-ID", containerID)
	h.proxyPassthrough(w, r, "containers/files")
}
