package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPassthroughEndpoint_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	// Pass-through endpoints require auth
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should get 401 (no auth) or 404 (route not found without pass-through config)
	assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusNotFound,
		"expected 401 or 404, got %d", w.Code)
}
