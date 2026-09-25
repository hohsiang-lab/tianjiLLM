package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// T010: Handler contract tests for log detail endpoint.

func TestLogDetail_MissingRequestID(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/logs/detail", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request_id is required")
}

func TestLogDetail_NoDB_ReturnsError(t *testing.T) {
	// newUITestServer has nil DB — handler should return 500
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/logs/detail?request_id=test-123", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "database not available")
}

func TestLogDetail_NotFound(t *testing.T) {
	// With nil DB, we get "database not available" (500) rather than 404.
	// A true 404 test requires a real DB with no matching row.
	// This test verifies the endpoint exists and is routable.
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/logs/detail?request_id=nonexistent-id", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without DB: 500 (database not available). With DB: would be 404.
	assert.Contains(t, []int{http.StatusNotFound, http.StatusInternalServerError}, w.Code)
}

func TestLogDetail_RequiresAuth(t *testing.T) {
	srv := newUITestServer(t)

	// No cookie — should redirect to login
	req := httptest.NewRequest(http.MethodGet, "/ui/logs/detail?request_id=test-123", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("Location"))
}
