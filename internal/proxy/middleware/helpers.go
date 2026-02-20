package middleware

import (
	"encoding/json"
	"net/http"
)

// Context keys shared across middleware.
var (
	tokenHashKey  contextKey = ContextKeyTokenHash
	rpmLimitKey   contextKey = "rpm_limit"
	tpmLimitKey   contextKey = "tpm_limit"
	modelGroupKey contextKey = "model_group"
)

// writeJSONResponse writes a JSON response to the writer.
func writeJSONResponse(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}
