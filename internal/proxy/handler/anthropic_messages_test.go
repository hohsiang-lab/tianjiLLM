package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anthropicTestHandlers creates a Handlers wired with the given API key for upstream.
func anthropicTestHandlers(upstreamURL, apiKey string) *Handlers {
	return &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "anthropic-test",
			TianjiParams: config.TianjiParams{
				Model:   "anthropic/some-model",
				APIKey:  &apiKey,
				APIBase: &upstreamURL,
			},
		}},
	}}
}

const (
	testOAuthKey   = "sk-ant-oat01-test-oauth-token"
	testRegularKey = "sk-ant-api03-test-regular-key"
)

// TestAnthropicMessages_ForwardsBodyUnmodified verifies the upstream receives
// the exact body sent by the client.
func TestAnthropicMessages_ForwardsBodyUnmodified(t *testing.T) {
	t.Parallel()
	const body = `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}],"max_tokens":10}`

	var received []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, body, string(received))
}

// TestAnthropicMessages_ReplacesAuthWithUpstreamOAuth verifies the upstream
// receives the OAuth token, not the client's virtual key.
func TestAnthropicMessages_ReplacesAuthWithUpstreamOAuth(t *testing.T) {
	t.Parallel()

	var gotAuth, gotXAPIKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotXAPIKey = r.Header.Get("x-api-key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer sk-virtual-key-should-be-replaced")
	req.Header.Set("x-api-key", "sk-virtual-should-be-removed")
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, "Bearer "+testOAuthKey, gotAuth)
	assert.Empty(t, gotXAPIKey, "client x-api-key must be stripped to prevent credential leakage")
}

// TestAnthropicMessages_PreservesClientBetaHeaders verifies client's
// anthropic-beta values are preserved and merged with OAuth beta (deduped).
func TestAnthropicMessages_PreservesClientBetaHeaders(t *testing.T) {
	t.Parallel()

	var gotBeta string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBeta = r.Header.Get("anthropic-beta")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("anthropic-beta", "claude-code-20250219,oauth-2025-04-20")
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	// Should contain both values, with oauth-2025-04-20 appearing exactly once (deduped)
	assert.Contains(t, gotBeta, "claude-code-20250219")
	assert.Contains(t, gotBeta, "oauth-2025-04-20")
	// Count occurrences of oauth-2025-04-20
	count := strings.Count(gotBeta, "oauth-2025-04-20")
	assert.Equal(t, 1, count, "oauth-2025-04-20 should appear exactly once (deduped)")
}

// TestAnthropicMessages_ForwardsAllClientHeaders verifies non-auth headers
// from the client (User-Agent, x-app, X-Stainless-*) reach upstream.
func TestAnthropicMessages_ForwardsAllClientHeaders(t *testing.T) {
	t.Parallel()

	var gotHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("User-Agent", "claude-cli/2.1.81 (external, cli)")
	req.Header.Set("x-app", "cli")
	req.Header.Set("X-Stainless-Lang", "js")
	req.Header.Set("X-Stainless-Runtime", "node")
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, "claude-cli/2.1.81 (external, cli)", gotHeaders.Get("User-Agent"))
	assert.Equal(t, "cli", gotHeaders.Get("x-app"))
	assert.Equal(t, "js", gotHeaders.Get("X-Stainless-Lang"))
	assert.Equal(t, "node", gotHeaders.Get("X-Stainless-Runtime"))
}

// TestAnthropicMessages_ForwardsQueryParams verifies ?beta=true is forwarded.
func TestAnthropicMessages_ForwardsQueryParams(t *testing.T) {
	t.Parallel()

	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages?beta=true", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Contains(t, gotQuery, "beta=true")
}

// TestAnthropicMessages_ForwardsAnthropicVersion verifies client's
// anthropic-version is forwarded, or defaults to 2023-06-01.
func TestAnthropicMessages_ForwardsAnthropicVersion(t *testing.T) {
	t.Parallel()

	t.Run("client provides version", func(t *testing.T) {
		var gotVersion string
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotVersion = r.Header.Get("anthropic-version")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
		}))
		defer upstream.Close()

		h := anthropicTestHandlers(upstream.URL, testOAuthKey)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
		req.Header.Set("anthropic-version", "2024-01-01")
		w := httptest.NewRecorder()
		h.nativeProxy(w, req, "anthropic")

		assert.Equal(t, "2024-01-01", gotVersion)
	})

	t.Run("default version", func(t *testing.T) {
		var gotVersion string
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotVersion = r.Header.Get("anthropic-version")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
		}))
		defer upstream.Close()

		h := anthropicTestHandlers(upstream.URL, testOAuthKey)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.nativeProxy(w, req, "anthropic")

		assert.Equal(t, "2023-06-01", gotVersion)
	})
}

// TestAnthropicMessages_SuccessResponse verifies the client receives 200 +
// Anthropic JSON body unmodified.
func TestAnthropicMessages_SuccessResponse(t *testing.T) {
	t.Parallel()
	const respBody = `{"id":"msg_abc","type":"message","role":"assistant","content":[{"type":"text","text":"Hello!"}]}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, respBody, w.Body.String())
}

// TestAnthropicMessages_ErrorResponse verifies upstream errors are forwarded
// with original status code and body.
func TestAnthropicMessages_ErrorResponse(t *testing.T) {
	t.Parallel()
	const errBody = `{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(errBody))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_request_error")
}

// TestAnthropicMessages_StreamingSSE verifies streaming responses are forwarded
// with correct Content-Type.
func TestAnthropicMessages_StreamingSSE(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"stream":true}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, w.Body.String(), "message_start")
	assert.Contains(t, w.Body.String(), "content_block_delta")
	assert.Contains(t, w.Body.String(), "message_stop")
}

// TestAnthropicMessages_NonStreaming verifies complete JSON is returned for
// non-streaming requests.
func TestAnthropicMessages_NonStreaming(t *testing.T) {
	t.Parallel()
	const respBody = `{"id":"msg_ok","type":"message","content":[{"type":"text","text":"hello"}]}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"stream":false}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, respBody, w.Body.String())
}

// TestAnthropicMessages_RegularAPIKey verifies non-OAuth keys use x-api-key.
func TestAnthropicMessages_RegularAPIKey(t *testing.T) {
	t.Parallel()

	var gotXAPIKey, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXAPIKey = r.Header.Get("x-api-key")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testRegularKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, testRegularKey, gotXAPIKey)
	assert.Empty(t, gotAuth, "Authorization should not be set for regular API key")
}

// TestAnthropicMessages_InvalidJSON verifies 400 for malformed body.
func TestAnthropicMessages_InvalidJSON(t *testing.T) {
	t.Parallel()

	// nativeProxy still forwards even with invalid JSON — it just can't extract model.
	// The upstream will reject it, returning its own error.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"invalid JSON"}}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAnthropicMessages_NoModelConfig verifies behavior when no config found.
func TestAnthropicMessages_NoModelConfig(t *testing.T) {
	t.Parallel()

	h := &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{}, // empty — no providers
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "not configured")
}

// TestAnthropicMessages_OAuthSetsDangerousAccess verifies
// anthropic-dangerous-direct-browser-access is set for OAuth tokens.
func TestAnthropicMessages_OAuthSetsDangerousAccess(t *testing.T) {
	t.Parallel()

	var gotHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("anthropic-dangerous-direct-browser-access")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, "true", gotHeader)
}

// TestAnthropicEventLogging_ReturnsOK verifies the stub returns 200.
func TestAnthropicEventLogging_ReturnsOK(t *testing.T) {
	t.Parallel()

	h := &Handlers{Config: &config.ProxyConfig{}}
	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.AnthropicEventLoggingBatch(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

// TestAnthropicMessages_CountTokensPassthrough verifies /v1/messages/count_tokens
// reaches upstream at the correct path.
func TestAnthropicMessages_CountTokensPassthrough(t *testing.T) {
	t.Parallel()

	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"input_tokens":42}`))
	}))
	defer upstream.Close()

	h := anthropicTestHandlers(upstream.URL, testOAuthKey)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[]}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, gotPath, "count_tokens")
}
