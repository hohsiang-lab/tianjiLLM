package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

// BenchmarkChatCompletion measures throughput of the proxy handling non-streaming
// chat completions with a mock upstream.
func BenchmarkChatCompletion(b *testing.B) {
	// Mock upstream that returns a minimal valid response
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stop := "stop"
		resp := model.ModelResponse{
			ID:      "chatcmpl-bench",
			Object:  "chat.completion",
			Model:   "gpt-4o",
			Created: 1700000000,
			Choices: []model.Choice{
				{
					Index:        0,
					FinishReason: &stop,
					Message: &model.Message{
						Role:    "assistant",
						Content: "Hello!",
					},
				},
			},
			Usage: model.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	apiKey := "sk-test"
	upstreamURL := upstream.URL
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "gpt-4o",
					TianjiParams: config.TianjiParams{
						Model:   "openai/gpt-4o",
						APIKey:  &apiKey,
						APIBase: &upstreamURL,
					},
				},
			},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer sk-master")
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

// BenchmarkListModels measures throughput of the models listing endpoint.
func BenchmarkListModels(b *testing.B) {
	apiKey := "sk-test"
	models := make([]config.ModelConfig, 50)
	for i := range models {
		name := "model-" + strings.Repeat("x", 3)
		models[i] = config.ModelConfig{
			ModelName:    name,
			TianjiParams: config.TianjiParams{Model: "openai/" + name, APIKey: &apiKey},
		}
	}

	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{ModelList: models},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer sk-master")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

// BenchmarkHealthCheck measures throughput of the health endpoint.
func BenchmarkHealthCheck(b *testing.B) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: handlers,
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
	}
}
