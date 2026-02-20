package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyGenerate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"key_name": "test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without DB, should return 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestKeyGenerate_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"key_name": "test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestKeyList_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
