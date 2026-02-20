package prompt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	src, err := New("langfuse", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "langfuse", src.Name())
}

func TestRegistry_UnknownSource(t *testing.T) {
	_, err := New("nonexistent", nil)
	assert.Error(t, err)
}

func TestLangfuseSource_GetPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/public/v2/prompts/")

		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": []map[string]any{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Hello {{name}}"},
			},
			"type": "chat",
		})
	}))
	defer server.Close()

	src := &LangfuseSource{
		publicKey: "pk-test",
		secretKey: "sk-test",
		baseURL:   server.URL,
		cache:     make(map[string]cachedPrompt),
		ttl:       5 * time.Minute,
	}

	result, err := src.GetPrompt(context.Background(), "test-prompt", PromptOptions{
		Variables: map[string]string{"name": "Alice"},
	})
	require.NoError(t, err)

	require.Len(t, result.Messages, 2)
	assert.Equal(t, "system", result.Messages[0].Role)
	assert.Equal(t, "Hello Alice", result.Messages[1].Content)
}

func TestLangfuseSource_GetPromptByVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "42", r.URL.Query().Get("version"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": "Direct text prompt",
			"type":   "text",
		})
	}))
	defer server.Close()

	src := &LangfuseSource{
		publicKey: "pk-test",
		secretKey: "sk-test",
		baseURL:   server.URL,
		cache:     make(map[string]cachedPrompt),
		ttl:       5 * time.Minute,
	}

	version := 42
	result, err := src.GetPrompt(context.Background(), "my-prompt", PromptOptions{
		Version: &version,
	})
	require.NoError(t, err)

	require.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
	assert.Equal(t, "Direct text prompt", result.Messages[0].Content)
}

func TestLangfuseSource_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": "cached prompt",
			"type":   "text",
		})
	}))
	defer server.Close()

	src := &LangfuseSource{
		publicKey: "pk-test",
		secretKey: "sk-test",
		baseURL:   server.URL,
		cache:     make(map[string]cachedPrompt),
		ttl:       5 * time.Minute,
	}

	ctx := context.Background()
	opts := PromptOptions{}

	_, _ = src.GetPrompt(ctx, "cached-prompt", opts)
	_, _ = src.GetPrompt(ctx, "cached-prompt", opts)

	assert.Equal(t, 1, callCount)
}

func TestLangfuseSource_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	src := &LangfuseSource{
		publicKey: "pk-test",
		secretKey: "sk-test",
		baseURL:   server.URL,
		cache:     make(map[string]cachedPrompt),
		ttl:       5 * time.Minute,
	}

	_, err := src.GetPrompt(context.Background(), "my-prompt", PromptOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestLangfuseSource_VariableSubstitution(t *testing.T) {
	src := &LangfuseSource{
		cache: make(map[string]cachedPrompt),
		ttl:   5 * time.Minute,
	}

	resolved := &ResolvedPrompt{
		Messages: []model.Message{
			{Role: "user", Content: "Hello {{name}}, your ID is {{id}}"},
		},
		Metadata: map[string]string{},
	}

	result := src.applyVariables(resolved, map[string]string{
		"name": "Bob",
		"id":   "42",
	})

	assert.Equal(t, "Hello Bob, your ID is 42", result.Messages[0].Content)
}

func TestLangfuseSource_NoVariables(t *testing.T) {
	src := &LangfuseSource{}

	resolved := &ResolvedPrompt{
		Messages: []model.Message{
			{Role: "user", Content: "No vars here"},
		},
	}

	result := src.applyVariables(resolved, nil)
	assert.Same(t, resolved, result) // same pointer, no copy
}

func TestLangfuseSource_ByLabel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "production", r.URL.Query().Get("label"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": "prod prompt",
			"type":   "text",
		})
	}))
	defer server.Close()

	src := &LangfuseSource{
		publicKey: "pk-test",
		secretKey: "sk-test",
		baseURL:   server.URL,
		cache:     make(map[string]cachedPrompt),
		ttl:       5 * time.Minute,
	}

	label := "production"
	_, err := src.GetPrompt(context.Background(), "my-prompt", PromptOptions{
		Label: &label,
	})
	require.NoError(t, err)
}
