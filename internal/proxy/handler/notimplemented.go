package handler

import (
	"fmt"
	"net/http"
)

// NotImplemented returns a 501 handler for endpoints that exist in Python
// TianjiLLM but are not yet implemented in Go (FR-028).
func NotImplemented(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": map[string]any{
				"message": fmt.Sprintf("%s %s is not yet implemented in tianjiLLM", r.Method, endpoint),
				"type":    "not_implemented",
				"code":    "not_implemented",
			},
		})
	}
}
