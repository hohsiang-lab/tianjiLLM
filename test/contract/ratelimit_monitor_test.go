package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRateLimitTestServer(t *testing.T, store *ratelimit.Store) *proxy.Server {
	t.Helper()
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}
	handlers := &handler.Handlers{
		Config:         cfg,
		RateLimitStore: store,
	}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

// AC#8: GET /internal/ratelimit returns JSON with providers key and correct fields.
func TestRateLimitMonitor_ContractSchema(t *testing.T) {
	store := ratelimit.NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", "800000")
	h.Set("anthropic-ratelimit-tokens-remaining", "650000")
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "850")
	store.ParseAndUpdate("anthropic/test", h)

	srv := newRateLimitTestServer(t, store)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/internal/ratelimit", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-master")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	providers, ok := body["providers"].(map[string]any)
	require.True(t, ok, "response must contain 'providers' key of type object")
	require.Len(t, providers, 1)

	entry, ok := providers["anthropic/test"].(map[string]any)
	require.True(t, ok, "expected provider entry 'anthropic/test'")

	requiredFields := []string{
		"tokens_limit",
		"tokens_remaining",
		"tokens_reset",
		"requests_limit",
		"requests_remaining",
		"updated_at",
	}
	for _, field := range requiredFields {
		assert.Contains(t, entry, field, "provider entry must contain field %q", field)
	}

	assert.Equal(t, float64(800000), entry["tokens_limit"])
	assert.Equal(t, float64(650000), entry["tokens_remaining"])
	assert.Equal(t, float64(1000), entry["requests_limit"])
	assert.Equal(t, float64(850), entry["requests_remaining"])
}

// AC#8: GET /internal/ratelimit with empty store returns empty providers object.
func TestRateLimitMonitor_EmptyStore(t *testing.T) {
	store := ratelimit.NewStore()
	srv := newRateLimitTestServer(t, store)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/internal/ratelimit", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-master")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	providers, ok := body["providers"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, providers)
}
