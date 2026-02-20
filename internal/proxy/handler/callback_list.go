package handler

import "net/http"

// CallbackList handles GET /callback/list.
func (h *Handlers) CallbackList(w http.ResponseWriter, r *http.Request) {
	callbacks := []string{}
	if h.Callbacks != nil {
		callbacks = h.Callbacks.Names()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"callbacks": callbacks,
		"count":     len(callbacks),
	})
}
