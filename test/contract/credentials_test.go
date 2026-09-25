package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentialNew_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"credential_name":"test-cred","credential_value":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/credentials/new", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCredentialList_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/credentials/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCredentialInfo_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/credentials/info/test-cred-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCredentialDelete_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodDelete, "/credentials/delete/test-cred-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCredentialNew_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"credential_name":"test","credential_value":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/credentials/new", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
