package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssistantsCreate_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/assistants", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer sk-test-key")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "asst_abc123",
			"object": "assistant",
			"model":  "gpt-4o",
			"name":   "Test Assistant",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"model":"gpt-4o","name":"Test Assistant","instructions":"You are helpful."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/assistants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "asst_abc123", resp["id"])
}

func TestAssistantsGet_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/assistants/asst_abc123", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "asst_abc123",
			"object": "assistant",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/assistants/asst_abc123", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssistantsDelete_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/assistants/asst_abc123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "asst_abc123",
			"object":  "assistant.deleted",
			"deleted": true,
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	req := httptest.NewRequest(http.MethodDelete, "/v1/assistants/asst_abc123", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestThreadsCreate_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/threads", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "thread_abc123",
			"object": "thread",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/v1/threads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMessagesCreate_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/threads/thread_abc123/messages", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "msg_abc123",
			"object": "thread.message",
			"role":   "user",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"role":"user","content":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/threads/thread_abc123/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRunsCreate_PassThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/threads/thread_abc123/runs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           "run_abc123",
			"object":       "thread.run",
			"status":       "queued",
			"assistant_id": "asst_abc123",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"assistant_id":"asst_abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/threads/thread_abc123/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssistants_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/assistants", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
