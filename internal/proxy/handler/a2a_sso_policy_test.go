package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
	"github.com/stretchr/testify/assert"
)

// --- mocks ---

type mockPolicyEvaluator struct {
	result policy.EvaluateResult
	err    error
}

func (m *mockPolicyEvaluator) Evaluate(_ context.Context, _ policy.MatchRequest) (policy.EvaluateResult, error) {
	return m.result, m.err
}

type mockAgentRegistry struct {
	agent *a2a.AgentConfig
	found bool
}

func (m *mockAgentRegistry) GetAgentByID(_ string) (*a2a.AgentConfig, bool) {
	return m.agent, m.found
}

type mockMessageSender struct {
	result *a2a.SendMessageResult
	err    error
}

func (m *mockMessageSender) SendMessage(_ context.Context, _ *a2a.AgentConfig, _ string) (*a2a.SendMessageResult, error) {
	return m.result, m.err
}

type mockSSOProvider struct {
	loginURL    string
	tokenResp   *auth.TokenResponse
	exchangeErr error
	userInfo    *auth.UserInfo
	userInfoErr error
	mappedRole  auth.Role
}

func (m *mockSSOProvider) LoginURL(_ string) string { return m.loginURL }
func (m *mockSSOProvider) ExchangeCode(_ context.Context, _ string) (*auth.TokenResponse, error) {
	return m.tokenResp, m.exchangeErr
}
func (m *mockSSOProvider) GetUserInfo(_ context.Context, _ string) (*auth.UserInfo, error) {
	return m.userInfo, m.userInfoErr
}
func (m *mockSSOProvider) MapRole(_ []string) auth.Role { return m.mappedRole }

// --- A2A tests ---

func TestA2AAgentCard_NilRegistry(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/a2a/x/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	h.A2AAgentCard(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestA2AAgentCard_NotFound(t *testing.T) {
	h := &Handlers{AgentRegistry: &mockAgentRegistry{found: false}}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req := httptest.NewRequest(http.MethodGet, "/a2a/missing/.well-known/agent-card.json", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AAgentCard(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestA2AAgentCard_Success(t *testing.T) {
	h := &Handlers{AgentRegistry: &mockAgentRegistry{
		agent: &a2a.AgentConfig{AgentName: "test-agent"},
		found: true,
	}}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "a1")
	req := httptest.NewRequest(http.MethodGet, "/a2a/a1/.well-known/agent-card.json", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AAgentCard(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestA2AMessage_NilRegistry(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/a2a/x", nil)
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestA2AMessage_AgentNotFound(t *testing.T) {
	h := &Handlers{
		AgentRegistry:    &mockAgentRegistry{found: false},
		CompletionBridge: &mockMessageSender{},
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	req := httptest.NewRequest(http.MethodPost, "/a2a/missing", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestA2AMessage_InvalidJSON(t *testing.T) {
	h := &Handlers{
		AgentRegistry:    &mockAgentRegistry{agent: &a2a.AgentConfig{}, found: true},
		CompletionBridge: &mockMessageSender{},
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "a1")
	req := httptest.NewRequest(http.MethodPost, "/a2a/a1", bytes.NewReader([]byte(`not json`)))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestA2AMessage_BadJsonrpcVersion(t *testing.T) {
	h := &Handlers{
		AgentRegistry:    &mockAgentRegistry{agent: &a2a.AgentConfig{}, found: true},
		CompletionBridge: &mockMessageSender{},
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "a1")
	body, _ := json.Marshal(map[string]any{"jsonrpc": "1.0", "method": "message/send"})
	req := httptest.NewRequest(http.MethodPost, "/a2a/a1", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestA2AMessage_UnknownMethod(t *testing.T) {
	h := &Handlers{
		AgentRegistry:    &mockAgentRegistry{agent: &a2a.AgentConfig{}, found: true},
		CompletionBridge: &mockMessageSender{},
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "a1")
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "unknown/method"})
	req := httptest.NewRequest(http.MethodPost, "/a2a/a1", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "method not found")
}

func TestA2AMessage_SendSuccess(t *testing.T) {
	h := &Handlers{
		AgentRegistry: &mockAgentRegistry{agent: &a2a.AgentConfig{}, found: true},
		CompletionBridge: &mockMessageSender{
			result: &a2a.SendMessageResult{},
		},
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "a1")
	params, _ := json.Marshal(a2a.SendMessageParams{Message: a2a.UserMessage{Content: "hello"}})
	body, _ := json.Marshal(a2a.JSONRPCRequest{JSONRPC: "2.0", Method: "message/send", Params: params})
	req := httptest.NewRequest(http.MethodPost, "/a2a/a1", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.A2AMessage(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- SSO tests ---

func TestSSOLogin_NilHandler(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/sso/login", nil)
	w := httptest.NewRecorder()
	h.SSOLogin(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestSSOLogin_Success(t *testing.T) {
	h := &Handlers{SSOHandler: &SSOHandler{SSO: &mockSSOProvider{loginURL: "https://sso.example.com/auth"}}}
	req := httptest.NewRequest(http.MethodGet, "/sso/login", nil)
	w := httptest.NewRecorder()
	h.SSOLogin(w, req)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "https://sso.example.com/auth")
}

func TestSSOCallback_NilHandler(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/sso/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	h.SSOCallback(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestSSOCallback_MissingCode(t *testing.T) {
	h := &Handlers{SSOHandler: &SSOHandler{SSO: &mockSSOProvider{}}}
	req := httptest.NewRequest(http.MethodGet, "/sso/callback", nil)
	w := httptest.NewRecorder()
	h.SSOCallback(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSSOCallback_InvalidState(t *testing.T) {
	h := &Handlers{SSOHandler: &SSOHandler{SSO: &mockSSOProvider{}}}
	req := httptest.NewRequest(http.MethodGet, "/sso/callback?code=abc&state=wrong", nil)
	// no cookie → state mismatch
	w := httptest.NewRecorder()
	h.SSOCallback(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSSOCallback_Success(t *testing.T) {
	h := &Handlers{SSOHandler: &SSOHandler{SSO: &mockSSOProvider{
		tokenResp:  &auth.TokenResponse{AccessToken: "tok", IDToken: "id", ExpiresIn: 3600},
		userInfo:   &auth.UserInfo{Sub: "u1", Email: "a@b.com", Name: "Alice", Groups: []string{"admin"}},
		mappedRole: auth.Role("admin"),
	}}}
	req := httptest.NewRequest(http.MethodGet, "/sso/callback?code=abc&state=s1", nil)
	req.AddCookie(&http.Cookie{Name: "tianji_sso_state", Value: "s1"})
	w := httptest.NewRecorder()
	h.SSOCallback(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "u1")
}

// --- PolicyResolvedGuardrails tests ---

func TestPolicyResolvedGuardrails_NilDB(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/policy/guardrails", nil)
	w := httptest.NewRecorder()
	h.PolicyResolvedGuardrails(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPolicyResolvedGuardrails_Success(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{
		DB:        ms,
		PolicyEng: &mockPolicyEvaluator{result: policy.EvaluateResult{}},
	}
	body, _ := json.Marshal(policy.MatchRequest{})
	req := httptest.NewRequest(http.MethodPost, "/policy/guardrails", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PolicyResolvedGuardrails(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
