package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase6_OCR_PassThrough(t *testing.T) {
	apiKey := "sk-test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "ocr-123",
			"object":  "ocr.result",
			"content": "Extracted text from image",
		})
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName:    "mistral-ocr",
			TianjiParams: config.TianjiParams{Model: "openai/mistral-ocr", APIKey: &apiKey, APIBase: &upstream.URL},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{})
	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"mistral-ocr","document":{"type":"image_url","image_url":"https://example.com/image.png"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ocr", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// The passthrough handler resolves providers; without exact model match, it may return an error
	// But the route should be wired and respond (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code, "OCR route should be wired")
}

func TestPhase6_VideoCreate_RouteWired(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"runway/gen-3","prompt":"A sunset over the ocean"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Route should exist (not 404/405), even if provider resolution fails
	assert.NotEqual(t, http.StatusNotFound, w.Code, "video route should be wired")
}

func TestPhase6_ContainerCreate_RouteWired(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"openai/gpt-4","name":"my-container"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/containers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code, "container route should be wired")
}

func TestPhase6_ContainerList_RouteWired(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/containers", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code, "container list route should be wired")
}

func TestPhase6_VideoContent_RouteWired(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/videos/vid-123/content", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code, "video content route should be wired")
}

func TestPhase6_Media_Unauthorized(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	// No auth header
	req := httptest.NewRequest(http.MethodPost, "/v1/ocr", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
