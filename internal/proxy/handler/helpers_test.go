package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestWriteJSON_Error(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
		Error: model.ErrorDetail{Message: "bad", Type: "invalid"},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCacheKey_Deterministic(t *testing.T) {
	msgs := []model.Message{
		{Role: "user", Content: "hello"},
	}
	k1 := cacheKey("gpt-4o", msgs)
	k2 := cacheKey("gpt-4o", msgs)
	assert.Equal(t, k1, k2)
	assert.True(t, strings.HasPrefix(k1, "tianji:cache:"))
}

func TestCacheKey_DifferentModel(t *testing.T) {
	msgs := []model.Message{{Role: "user", Content: "hello"}}
	k1 := cacheKey("gpt-4o", msgs)
	k2 := cacheKey("claude-3", msgs)
	assert.NotEqual(t, k1, k2)
}

func TestMergeStrings(t *testing.T) {
	result := mergeStrings([]string{"a", "b"}, []string{"b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestMergeStrings_Empty(t *testing.T) {
	result := mergeStrings(nil, nil)
	assert.Empty(t, result)
}

func TestMergeStrings_OneEmpty(t *testing.T) {
	result := mergeStrings([]string{"a"}, nil)
	assert.Equal(t, []string{"a"}, result)
}

func TestDefaultBaseURL(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{"openai", "https://api.openai.com"},
		{"anthropic", "https://api.anthropic.com"},
		{"gemini", "https://generativelanguage.googleapis.com"},
		{"cohere", "https://api.cohere.ai"},
		{"mistral", "https://api.mistral.ai"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, defaultBaseURL(tt.provider), tt.provider)
	}
}

func TestExtractRequestModel(t *testing.T) {
	body := `{"model": "gpt-4o"}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	m := extractRequestModel(r)
	assert.Equal(t, "gpt-4o", m)
}

func TestExtractRequestModel_NoModel(t *testing.T) {
	body := `{"messages": []}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	m := extractRequestModel(r)
	assert.Equal(t, "", m)
}

func TestExtractRequestModel_InvalidJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("not json"))
	m := extractRequestModel(r)
	assert.Equal(t, "", m)
}
