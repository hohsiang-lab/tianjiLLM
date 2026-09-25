// Package contract tests for sticky token selection. These tests run the full
// nativeProxy request path (router → handler → selectUpstreamWithThrottle →
// stickySelect → ReverseProxy) against a mock Anthropic server. They verify the
// production scenario from 2026-04-14 where sticky was kept picking the wrong
// token, and that the fix (soonest 7d reset wins) routes correctly.
package contract

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOAuthToken produces an OAuth-prefixed token long enough to look realistic.
// Content is irrelevant — only the prefix matters for anthropic.IsOAuthToken.
func fakeOAuthToken(name string) string {
	return "sk-ant-oat-" + name + "-" + strings.Repeat("x", 60)
}

// routedTokens records which Authorization bearer token each request landed on.
type routedTokens struct {
	mu     sync.Mutex
	tokens []string
}

func (r *routedTokens) record(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = append(r.tokens, token)
}

func (r *routedTokens) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.tokens))
	copy(out, r.tokens)
	return out
}

// mockAnthropicServer returns a server that records the bearer token from each
// request and replies with per-token rate-limit headers plus a canonical JSON body.
func mockAnthropicServer(t *testing.T, routed *routedTokens, headersByToken map[string]http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			t.Errorf("mock received %s, expected POST", r.Method)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		routed.record(token)

		if hdrs, ok := headersByToken[token]; ok && hdrs != nil {
			for k, values := range hdrs {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"id":"msg_x","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)); err != nil {
			t.Errorf("mock failed to write response: %v", err)
		}
	}))
}

// buildStickyHandlers wires a Handlers struct pointing at mockURL with two
// OAuth-format tokens and sticky strategy.
func buildStickyHandlers(t *testing.T, mockURL, tokenA, tokenB string) (*handler.Handlers, *proxy.Server) {
	t.Helper()
	cfg := &config.ProxyConfig{
		NativeUpstreamStrategy: config.StrategySticky,
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet-4-6",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-6",
					APIKey:  &tokenA,
					APIBase: &mockURL,
				},
			},
			{
				ModelName: "claude-sonnet-4-6",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-6",
					APIKey:  &tokenB,
					APIBase: &mockURL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	h := &handler.Handlers{
		Config:         cfg,
		Callbacks:      callback.NewRegistry(),
		RateLimitStore: callback.NewInMemoryRateLimitStore(),
	}
	srv := proxy.NewServer(proxy.ServerConfig{Handlers: h, MasterKey: cfg.GeneralSettings.MasterKey})
	return h, srv
}

func sonnetRequest() *http.Request {
	body := `{"model":"claude-sonnet-4-6","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	return req
}

// ratelimitHeaders builds an Anthropic-style rate limit header set.
func ratelimitHeaders(util5h float64, reset5hIn time.Duration, util7d float64, reset7dIn time.Duration) http.Header {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-utilization", fmt.Sprintf("%.2f", util5h))
	h.Set("anthropic-ratelimit-unified-5h-reset", fmt.Sprintf("%d", time.Now().Add(reset5hIn).Unix()))
	h.Set("anthropic-ratelimit-unified-7d-status", "allowed")
	h.Set("anthropic-ratelimit-unified-7d-utilization", fmt.Sprintf("%.2f", util7d))
	h.Set("anthropic-ratelimit-unified-7d-reset", fmt.Sprintf("%d", time.Now().Add(reset7dIn).Unix()))
	// Include Sonnet window — used by the gate layer and lowestUtilizationSelect.
	h.Set("anthropic-ratelimit-unified-7d_sonnet-status", "allowed")
	h.Set("anthropic-ratelimit-unified-7d_sonnet-utilization", fmt.Sprintf("%.2f", util7d))
	h.Set("anthropic-ratelimit-unified-7d_sonnet-reset", fmt.Sprintf("%d", time.Now().Add(reset7dIn).Unix()))
	h.Set("anthropic-ratelimit-unified-reset", fmt.Sprintf("%d", time.Now().Add(reset5hIn).Unix()))
	return h
}

// seedState populates the RateLimitStore with a given token's utilization
// state so the first request can make a throttle decision without having to
// fire a warmup request.
func seedState(store callback.RateLimitStore, apiKey string, util5h float64, reset5hIn time.Duration, util7d float64, reset7dIn time.Duration) {
	key := callback.RateLimitCacheKey(apiKey)
	now := time.Now()
	store.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   key,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       util5h,
		Unified5hReset:             fmt.Sprintf("%d", now.Add(reset5hIn).Unix()),
		Unified7dUtilization:       util7d,
		Unified7dReset:             fmt.Sprintf("%d", now.Add(reset7dIn).Unix()),
		Unified7dSonnetUtilization: util7d,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(reset7dIn).Unix()),
	})
}

// TestSticky5hCycle_SwitchesAwayFrom5hGated verifies that when the current
// sticky token's 5h utilization crosses the gate threshold, requests route to
// the other token. This covers the selectUpstreamWithThrottle gate path +
// single-upstream shortcut in stickySelect.
func TestSticky5hCycle_SwitchesAwayFrom5hGated(t *testing.T) {
	t.Parallel()

	tokenA := fakeOAuthToken("gatedA")
	tokenB := fakeOAuthToken("freshB")

	routed := &routedTokens{}
	mock := mockAnthropicServer(t, routed, map[string]http.Header{
		tokenA: ratelimitHeaders(0.90, 1*time.Hour, 0.59, 2*24*time.Hour),
		tokenB: ratelimitHeaders(0.15, 3*time.Hour, 0.36, 5*24*time.Hour),
	})
	defer mock.Close()

	h, srv := buildStickyHandlers(t, mock.URL, tokenA, tokenB)
	seedState(h.RateLimitStore, tokenA, 0.90, 1*time.Hour, 0.59, 2*24*time.Hour)
	seedState(h.RateLimitStore, tokenB, 0.15, 3*time.Hour, 0.36, 5*24*time.Hour)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, sonnetRequest())
	require.Equal(t, http.StatusOK, w.Code, "response body=%s", w.Body.String())

	got := routed.snapshot()
	require.Len(t, got, 1, "exactly one upstream call")
	assert.Equal(t, tokenB, got[0],
		"A is 5h-gated (0.90 ≥ 0.80), so request must route to B")
}

// TestSticky5hCycle_PrefersSoonest7dReset_MultipleRequests verifies that sticky
// picks the token with the soonest 7d reset and reuses it across multiple requests
// within the same 5h window.
func TestSticky5hCycle_PrefersSoonest7dReset_MultipleRequests(t *testing.T) {
	t.Parallel()

	tokenA := fakeOAuthToken("multiA")
	tokenB := fakeOAuthToken("multiB")

	// B has soonest 7d reset (1d) vs A (5d) — B must be selected regardless of utilization.
	routed := &routedTokens{}
	mock := mockAnthropicServer(t, routed, map[string]http.Header{
		tokenA: ratelimitHeaders(0.10, 4*time.Hour, 0.15, 5*24*time.Hour),
		tokenB: ratelimitHeaders(0.50, 4*time.Hour, 0.50, 1*24*time.Hour),
	})
	defer mock.Close()

	h, srv := buildStickyHandlers(t, mock.URL, tokenA, tokenB)
	seedState(h.RateLimitStore, tokenA, 0.10, 4*time.Hour, 0.15, 5*24*time.Hour)
	seedState(h.RateLimitStore, tokenB, 0.50, 4*time.Hour, 0.50, 1*24*time.Hour)

	const N = 5
	for i := range N {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, sonnetRequest())
		require.Equal(t, http.StatusOK, w.Code, "req %d body=%s", i, w.Body.String())
	}

	got := routed.snapshot()
	require.Len(t, got, N)
	for i, tok := range got {
		assert.Equal(t, tokenB, tok, "req %d: sticky must keep routing to B (soonest 7d reset) within same 5h window", i)
	}
}
