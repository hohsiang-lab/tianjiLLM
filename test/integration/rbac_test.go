package integration

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tianjiAuth "github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// testJWTSetup creates a JWT validator with a test ECDSA key pair.
func testJWTSetup(t *testing.T) (*tianjiAuth.JWTValidator, *ecdsa.PrivateKey) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtValidator := tianjiAuth.NewJWTValidator(tianjiAuth.JWTConfig{})
	jwtValidator.SetKeys(map[string]any{
		"test-kid": &privateKey.PublicKey,
	})

	return jwtValidator, privateKey
}

func signToken(t *testing.T, claims jwt.Claims, key *ecdsa.PrivateKey) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "test-kid"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func newRBACTestServer(t *testing.T, jwtValidator *tianjiAuth.JWTValidator, upstreamURL string) *proxy.Server {
	t.Helper()
	apiKey := "sk-test"
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
		},
	}

	if upstreamURL != "" {
		for i := range cfg.ModelList {
			cfg.ModelList[i].TianjiParams.APIBase = &upstreamURL
		}
	}

	handlers := &handler.Handlers{Config: cfg}
	rbacEngine := tianjiAuth.NewRBACEngine()

	return proxy.NewServerWithAuth(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	}, middleware.AuthConfig{
		MasterKey:     cfg.GeneralSettings.MasterKey,
		JWTValidator:  jwtValidator,
		RBACEngine:    rbacEngine,
		EnableJWTAuth: true,
	})
}

func TestRBAC_AdminCanAccessAdminRoutes(t *testing.T) {
	jwtValidator, privateKey := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	tokenStr := signToken(t, &tianjiAuth.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Scopes: []string{"tianji_proxy_admin"},
	}, privateKey)

	// Admin should access /key/list (admin route)
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// 503 because no DB, but NOT 401/403 — auth passed
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRBAC_InternalUserCanAccessCompletions(t *testing.T) {
	jwtValidator, privateKey := testJWTSetup(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "hi"}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer upstream.Close()

	srv := newRBACTestServer(t, jwtValidator, upstream.URL)

	tokenStr := signToken(t, &tianjiAuth.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, privateKey)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBAC_InternalUserCannotAccessAdminRoutes(t *testing.T) {
	jwtValidator, privateKey := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	tokenStr := signToken(t, &tianjiAuth.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-456",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, privateKey)

	// Internal user should NOT access /key/generate (admin route)
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBAC_TeamRoleCanAccessTeamRoutes(t *testing.T) {
	jwtValidator, privateKey := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	tokenStr := signToken(t, &tianjiAuth.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "team-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TeamID: "team-abc",
	}, privateKey)

	// Team role should access /key/list (team-level route)
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// 503 because no DB, but NOT 403
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRBAC_MasterKeyBypassesJWT(t *testing.T) {
	jwtValidator, _ := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	// Master key should bypass JWT validation entirely
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// 503 because no DB, but auth passed
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRBAC_InvalidJWTRejected(t *testing.T) {
	jwtValidator, _ := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	// Send a fake JWT-looking token (3 segments but invalid)
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRBAC_VirtualKeyFallback(t *testing.T) {
	jwtValidator, _ := testJWTSetup(t)
	srv := newRBACTestServer(t, jwtValidator, "")

	// Send a non-JWT token (virtual key format) — should hit 401 since no DB validator
	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer sk-some-virtual-key")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
