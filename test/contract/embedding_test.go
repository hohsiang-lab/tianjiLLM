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

func TestEmbedding_ValidRequest(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{
		"model": "gpt-4o",
		"input": "Hello world"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// The request will go to real OpenAI so we may get 200 or 502
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadGateway,
		"expected 200 or 502, got %d", w.Code)

	if w.Code == http.StatusOK {
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "list", resp["object"])
		assert.Contains(t, resp, "data")
		assert.Contains(t, resp, "usage")
	}
}

func TestEmbedding_InvalidModel(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{
		"model": "nonexistent-embedding",
		"input": "Hello world"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEmbedding_InvalidJSON(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader("{broken"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEmbedding_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"model": "gpt-4o", "input": "Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
