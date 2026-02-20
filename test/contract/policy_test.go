package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyCreate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"name":"test-policy","conditions":{"model":"gpt-4o"},"guardrails_add":["pii_check"]}`
	req := httptest.NewRequest(http.MethodPost, "/policy/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyGet_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/policy/test-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyList_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/policy/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyUpdate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"name":"updated-policy","guardrails_add":["content_filter"]}`
	req := httptest.NewRequest(http.MethodPut, "/policy/test-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyDelete_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodDelete, "/policy/test-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyAttachmentCreate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"policy_name":"test-policy","scope":"team","teams":["team-1"]}`
	req := httptest.NewRequest(http.MethodPost, "/policy/attachment", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyAttachmentGet_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/policy/attachment/att-1", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyAttachmentList_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/policy/attachment/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyTestPipeline_NoPolicyEngine(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"model":"gpt-4o","user_id":"u1","team_id":"t1"}`
	req := httptest.NewRequest(http.MethodPost, "/policy/test-pipeline", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without policy engine, should return 503 or empty result
	assert.True(t, w.Code == http.StatusServiceUnavailable || w.Code == http.StatusOK)
}

func TestPolicyResolvedGuardrails_NoPolicyEngine(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/policy/resolved-guardrails?model=gpt-4o", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.True(t, w.Code == http.StatusServiceUnavailable || w.Code == http.StatusOK)
}

func TestPolicyCreate_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"name":"test-policy"}`
	req := httptest.NewRequest(http.MethodPost, "/policy/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
