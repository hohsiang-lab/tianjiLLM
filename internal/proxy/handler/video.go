package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// VideoCreate handles POST /v1/videos.
func (h *Handlers) VideoCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "videos")
}

// VideoGet handles GET /v1/videos/{video_id}.
func (h *Handlers) VideoGet(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "video_id")
	r.Header.Set("X-TianjiLLM-Video-ID", videoID)
	h.proxyPassthrough(w, r, "videos")
}

// VideoContent handles GET /v1/videos/{video_id}/content.
func (h *Handlers) VideoContent(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "video_id")
	r.Header.Set("X-TianjiLLM-Video-ID", videoID)
	h.proxyPassthrough(w, r, "videos/content")
}
