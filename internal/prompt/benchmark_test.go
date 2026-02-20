package prompt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkLangfuseGetPrompt_CacheHit(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": "Hello {{name}}",
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
	opts := PromptOptions{Variables: map[string]string{"name": "Alice"}}

	// Prime cache
	_, _ = src.GetPrompt(ctx, "bench-prompt", opts)

	b.ResetTimer()
	for range b.N {
		_, _ = src.GetPrompt(ctx, "bench-prompt", opts)
	}
}

func BenchmarkLangfuseGetPrompt_CacheMiss(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prompt": "Hello {{name}}",
			"type":   "text",
		})
	}))
	defer server.Close()

	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		src := &LangfuseSource{
			publicKey: "pk-test",
			secretKey: "sk-test",
			baseURL:   server.URL,
			cache:     make(map[string]cachedPrompt),
			ttl:       5 * time.Minute,
		}
		b.StartTimer()

		_, _ = src.GetPrompt(context.Background(), "bench-prompt", PromptOptions{
			Variables: map[string]string{"name": "Alice"},
		})
	}
}
