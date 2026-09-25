package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServerWithCallback(t *testing.T, upstream *httptest.Server, modelName, providerModel string) (*proxy.Server, *spyLogger) {
	t.Helper()

	spy := newSpyLogger()
	registry := callback.NewRegistry()
	registry.Register(spy)

	apiKey := "test-api-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: modelName,
				TianjiParams: config.TianjiParams{
					Model:   providerModel,
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	return srv, spy
}

func TestImageGeneration_SpendLog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/images/generations")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1234567890,
			"data": []map[string]any{
				{"url": "https://example.com/image.png"},
			},
		})
	}))
	defer upstream.Close()

	srv, spy := newTestServerWithCallback(t, upstream, "dall-e-3", "openaicompat/dall-e-3")

	body := `{"model": "dall-e-3", "prompt": "a cat", "n": 1, "size": "1024x1024"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data := spy.wait(t, 2*time.Second)
	assert.Equal(t, "dall-e-3", data.Model)
	assert.Equal(t, "image_generation", data.CallType)
	assert.True(t, data.Latency > 0)
}

func TestAudioSpeech_SpendLog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/audio/speech")
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake-audio-data"))
	}))
	defer upstream.Close()

	srv, spy := newTestServerWithCallback(t, upstream, "tts-1", "openaicompat/tts-1")

	body := `{"model": "tts-1", "input": "Hello world", "voice": "alloy"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data := spy.wait(t, 2*time.Second)
	assert.Equal(t, "tts-1", data.Model)
	assert.Equal(t, "audio_speech", data.CallType)
	assert.True(t, data.Latency > 0)
}

func TestModeration_SpendLog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/moderations")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "modr-test",
			"model": "text-moderation-latest",
			"results": []map[string]any{
				{"flagged": false},
			},
		})
	}))
	defer upstream.Close()

	srv, spy := newTestServerWithCallback(t, upstream, "text-moderation-latest", "openaicompat/text-moderation-latest")

	body := `{"model": "text-moderation-latest", "input": "hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/moderations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	data := spy.wait(t, 2*time.Second)
	assert.Equal(t, "text-moderation-latest", data.Model)
	assert.Equal(t, "moderation", data.CallType)
	assert.True(t, data.Latency > 0)
}

func TestImageGeneration_ErrorSkipsCallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "rate limited"},
		})
	}))
	defer upstream.Close()

	srv, spy := newTestServerWithCallback(t, upstream, "dall-e-3", "openaicompat/dall-e-3")

	body := `{"model": "dall-e-3", "prompt": "a cat"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// No callback should fire for error responses
	time.Sleep(200 * time.Millisecond)
	spy.mu.Lock()
	assert.Empty(t, spy.logs, "callback should not fire for error responses")
	spy.mu.Unlock()
}
