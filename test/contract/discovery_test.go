package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Register providers so List() returns something
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

func TestPublicProviders(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/public/providers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string][]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	names := body["data"]
	assert.True(t, len(names) > 0, "should have at least one provider")
	// Check sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] <= names[i], "providers should be sorted: %s > %s", names[i-1], names[i])
	}
}

func TestPublicModelCostMap(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/public/tianji_model_cost_map", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.True(t, len(body) > 0, "model cost map should not be empty")
}

func TestModelGroupInfo_WithRouter(t *testing.T) {
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model: "openai/gpt-4o",
			},
		},
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model: "openai/gpt-4o-2024-05-13",
			},
		},
	}

	rtr := router.New(models, strategy.NewShuffle(), router.RouterSettings{})
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: &handler.Handlers{
			Config: &config.ProxyConfig{},
			Router: rtr,
		},
		MasterKey: "sk-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []struct {
			ModelGroup     string   `json:"model_group"`
			Providers      []string `json:"providers"`
			NumDeployments int      `json:"num_deployments"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "gpt-4o", body.Data[0].ModelGroup)
	assert.Equal(t, 2, body.Data[0].NumDeployments)
	assert.Contains(t, body.Data[0].Providers, "openai")
}

func TestModelGroupInfo_FilterByGroup(t *testing.T) {
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o"},
		},
		{
			ModelName:    "claude",
			TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5-20250929"},
		},
	}

	rtr := router.New(models, strategy.NewShuffle(), router.RouterSettings{})
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: &handler.Handlers{
			Config: &config.ProxyConfig{},
			Router: rtr,
		},
		MasterKey: "sk-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info?model_group=gpt-4o", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []struct {
			ModelGroup string `json:"model_group"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "gpt-4o", body.Data[0].ModelGroup)
}

func TestModelGroupInfo_NoRouter(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []any `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Len(t, body.Data, 0)
}
