//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openrouter"
)

func TestOpenRouterResponsesRouteE2E_DefaultBase(t *testing.T) {
	f := setup(t)
	var gotPath string
	var gotAuth string
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request", http.StatusBadRequest)
			return
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		gotModel = payload.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_e2e","object":"response","status":"completed"}`))
	}))
	defer upstream.Close()

	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = openRouterHostTransport{
		target: target,
		base:   originalTransport,
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	f.SeedModel(SeedModelOpts{
		ModelName: "openrouter/*",
		Model:     "openrouter/*",
		APIKey:    "e2e-upstream-key",
	})
	require.NoError(t, proxyHandler.RefreshRuntimeModels(context.Background()))

	resp := postProxyResponses(t, `{"model":"openrouter/free","input":"hi"}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	assert.Equal(t, "/api/v1/responses", gotPath)
	assert.Equal(t, "Bearer e2e-upstream-key", gotAuth)
	assert.Equal(t, "free", gotModel)
}

type openRouterHostTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t openRouterHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "openrouter.ai" {
		return t.base.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	copiedURL := *clone.URL
	clone.URL = &copiedURL
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	return t.base.RoundTrip(clone)
}
