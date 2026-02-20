package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessGroupNew_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"group_alias":"test-group","models":["gpt-4o"]}`
	req := httptest.NewRequest(http.MethodPost, "/model_access_group/new", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAccessGroupInfo_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/model_access_group/info/test-group-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAccessGroupUpdate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"group_id":"test-group-id","models":["gpt-4o","claude-3"]}`
	req := httptest.NewRequest(http.MethodPost, "/model_access_group/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAccessGroupDelete_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodDelete, "/model_access_group/delete/test-group-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAccessGroupNew_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"group_alias":"test","models":["gpt-4o"]}`
	req := httptest.NewRequest(http.MethodPost, "/model_access_group/new", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
