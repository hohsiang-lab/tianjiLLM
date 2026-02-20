package contract

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

func newAliasTestServer(t *testing.T, upstreamURL string) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
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
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{
		NumRetries: 1,
		ModelGroupAlias: map[string]router.ModelGroupAliasItem{
			"my-alias":     {Model: "gpt-4o", Hidden: false},
			"hidden-alias": {Model: "gpt-4o", Hidden: true},
		},
	})

	handlers := &handler.Handlers{
		Config: cfg,
		Router: rtr,
	}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func TestAlias_ResolvesToCorrectModelGroup(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"model":   "gpt-4o",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hi"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	srv := newAliasTestServer(t, upstream.URL)

	body := `{"model":"my-alias","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAlias_HiddenNotInModels(t *testing.T) {
	srv := newAliasTestServer(t, "http://unused")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Collect model IDs
	var ids []string
	for _, m := range resp.Data {
		ids = append(ids, m.ID)
	}

	// hidden-alias should NOT appear
	assert.NotContains(t, ids, "hidden-alias")
	// gpt-4o should appear (not hidden)
	assert.Contains(t, ids, "gpt-4o")
}
