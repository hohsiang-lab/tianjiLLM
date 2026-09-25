package middleware

import (
	"encoding/json"
	"log"
	"net/http"
)

// Context keys shared across middleware.
var (
	tokenHashKey  contextKey = ContextKeyTokenHash
	rpmLimitKey   contextKey = "rpm_limit"
	tpmLimitKey   contextKey = "tpm_limit"
	modelGroupKey contextKey = "model_group"
	maxBudgetKey  contextKey = "max_budget"
	spendKey      contextKey = "spend"
)

// writeJSONResponse writes a JSON response to the writer.
func writeJSONResponse(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("warn: failed to write JSON response: %v", err)
	}
}
