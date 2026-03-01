package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAC1_Route_MetricsEndpoint_Returns200 verifies that /metrics is registered
// in setupRoutes() and returns 200 with # HELP lines.
func TestAC1_Route_MetricsEndpoint_Returns200(t *testing.T) {
	srv := newTestServer(t, "http://localhost:9999")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code,
		"/metrics route should be registered and return 200")
	ct := rec.Header().Get("Content-Type")
	assert.True(t, strings.Contains(ct, "text/plain"),
		"Content-Type should contain text/plain, got: %s", ct)
	assert.Contains(t, rec.Body.String(), "# HELP",
		"response should contain Prometheus HELP lines")
}

// TestAC2_Route_MetricsEndpoint_NoAuthRequired verifies that /metrics
// is outside the auth middleware (no Authorization header needed).
func TestAC2_Route_MetricsEndpoint_NoAuthRequired(t *testing.T) {
	srv := newTestServer(t, "http://localhost:9999")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// Explicitly no Authorization header
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	// If /metrics is behind auth middleware, this would be 401
	assert.Equal(t, http.StatusOK, rec.Code,
		"/metrics should not require auth (should not return 401)")
}
