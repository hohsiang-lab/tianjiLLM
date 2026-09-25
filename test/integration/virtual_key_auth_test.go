package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// testHashToken computes the SHA256 hex hash of a token — mirrors middleware.hashToken.
func testHashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// memToken holds per-key data for the in-memory test validator.
type memToken struct {
	userID     string
	teamID     string
	blocked    bool
	guardrails []string
}

// memValidator is an in-memory TokenValidator for tests.
type memValidator struct {
	tokens map[string]memToken // keyed by SHA256 hash
	dbDown bool
}

func (m *memValidator) ValidateToken(_ context.Context, tokenHash string) (*middleware.TokenInfo, error) {
	if m.dbDown {
		return nil, middleware.ErrDBUnavailable
	}
	tok, ok := m.tokens[tokenHash]
	if !ok {
		return nil, middleware.ErrKeyNotFound
	}
	uid, tid := tok.userID, tok.teamID
	return &middleware.TokenInfo{
		UserID:     &uid,
		TeamID:     &tid,
		Blocked:    tok.blocked,
		Guardrails: tok.guardrails,
	}, nil
}

// newVirtualKeyServer creates a proxy server backed by the given TokenValidator.
func newVirtualKeyServer(t *testing.T, validator middleware.TokenValidator) *proxy.Server {
	t.Helper()
	apiKey := "sk-upstream-test"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:  "openai/gpt-4o",
					APIKey: &apiKey,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
			Port:      4000,
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
		DBQueries: validator,
	})
}

// chatBody is a minimal /v1/chat/completions request body.
const chatBody = `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`

// T004: virtual key authentication — valid/invalid/blocked/db-down scenarios.

func TestVirtualKeyAuth_ValidKey(t *testing.T) {
	const virtualKey = "sk-virtual-valid"
	validator := &memValidator{
		tokens: map[string]memToken{
			testHashToken(virtualKey): {userID: "user-1", teamID: "team-1"},
		},
	}
	srv := newVirtualKeyServer(t, validator)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+virtualKey)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Auth must pass — upstream may fail with 502, but never 401/403.
	assert.NotEqual(t, http.StatusUnauthorized, w.Code, "valid virtual key must not get 401")
	assert.NotEqual(t, http.StatusForbidden, w.Code, "valid virtual key must not get 403")
}

func TestVirtualKeyAuth_InvalidKey(t *testing.T) {
	validator := &memValidator{tokens: map[string]memToken{}}
	srv := newVirtualKeyServer(t, validator)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-unknown-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestVirtualKeyAuth_BlockedKey(t *testing.T) {
	const blockedKey = "sk-virtual-blocked"
	validator := &memValidator{
		tokens: map[string]memToken{
			testHashToken(blockedKey): {userID: "user-2", blocked: true},
		},
	}
	srv := newVirtualKeyServer(t, validator)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+blockedKey)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestVirtualKeyAuth_DBUnavailable(t *testing.T) {
	validator := &memValidator{dbDown: true}
	srv := newVirtualKeyServer(t, validator)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(chatBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-any-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// T010: guardrail names must be populated in context after virtual key auth.
// Uses auth middleware directly so we can inspect the request context.
func TestVirtualKeyAuth_GuardrailsInContext(t *testing.T) {
	const virtualKey = "sk-guardrail-key"
	wantGuardrails := []string{"no-pii", "no-violence"}

	validator := &memValidator{
		tokens: map[string]memToken{
			testHashToken(virtualKey): {
				userID:     "user-3",
				guardrails: wantGuardrails,
			},
		},
	}

	authMW := middleware.NewAuthMiddleware(middleware.AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	var gotGuardrails []string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if names, ok := r.Context().Value(middleware.ContextKeyGuardrails).([]string); ok {
			gotGuardrails = names
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+virtualKey)
	w := httptest.NewRecorder()
	authMW(testHandler).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, wantGuardrails, gotGuardrails, "guardrail names must be populated in context after virtual key auth")
}
