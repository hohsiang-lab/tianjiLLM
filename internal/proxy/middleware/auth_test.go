package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// spyErrorLogger captures LogAuthError calls for testing (goroutine-safe).
type spyErrorLogger struct {
	mu    sync.Mutex
	calls []authErrorCall
}

type authErrorCall struct {
	requestID  string
	apiKeyHash string
	statusCode int
	errorMsg   string
}

func (s *spyErrorLogger) LogAuthError(_ context.Context, requestID string, apiKeyHash string, statusCode int, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, authErrorCall{requestID, apiKeyHash, statusCode, errorMsg})
}

func (s *spyErrorLogger) snapshot() []authErrorCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]authErrorCall, len(s.calls))
	copy(cp, s.calls)
	return cp
}

// mockValidator implements TokenValidator for auth middleware tests.
type mockValidator struct {
	info *TokenInfo
	err  error
}

func (m *mockValidator) ValidateToken(_ context.Context, _ string) (*TokenInfo, error) {
	return m.info, m.err
}

// countingQuerier records how many times GetVerificationToken is called.
type countingQuerier struct {
	calls int
}

func (c *countingQuerier) GetVerificationToken(_ context.Context, _ string) (db.VerificationToken, error) {
	c.calls++
	return db.VerificationToken{}, nil
}

func (c *countingQuerier) GetTeam(_ context.Context, _ string) (db.TeamTable, error) {
	return db.TeamTable{}, nil
}

func (c *countingQuerier) GetOrganization(_ context.Context, _ string) (db.OrganizationTable, error) {
	return db.OrganizationTable{}, nil
}

func (c *countingQuerier) GetBudget(_ context.Context, _ string) (db.BudgetTable, error) {
	return db.BudgetTable{}, nil
}

func TestMasterKey_BypassesDBLookup(t *testing.T) {
	const masterKey = "sk-master-secret"

	counter := &countingQuerier{}
	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: masterKey,
		Validator: &DBValidator{DB: counter},
	})

	called := false
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		isMaster, _ := r.Context().Value(ContextKeyIsMasterKey).(bool)
		assert.True(t, isMaster, "is_master_key should be true for master key requests")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+masterKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called, "downstream handler should be called")
	assert.Equal(t, 0, counter.calls, "DB should NOT be queried for master key requests")
}

func TestMasterKey_WrongKeyGoesToDB(t *testing.T) {
	const masterKey = "sk-master-secret"

	counter := &countingQuerier{}
	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: masterKey,
		Validator: &DBValidator{DB: counter},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-wrong-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	// Non-master key hits the DB exactly once (ValidateToken returns everything in a single call)
	assert.Equal(t, 1, counter.calls, "DB should be queried exactly once for non-master-key requests")
}

func TestMissingToken_Returns401(t *testing.T) {
	authMW := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master"})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMissingToken_OpenAISubscriptionLifecycleReturns401(t *testing.T) {
	authMW := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master"})

	req := httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-1/test", nil)
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVirtualKey_ValidKeyAuthenticates(t *testing.T) {
	uid := "user-42"
	tid := "team-99"
	validator := &mockValidator{info: &TokenInfo{UserID: &uid, TeamID: &tid}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	called := false
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		isMaster, _ := r.Context().Value(ContextKeyIsMasterKey).(bool)
		assert.False(t, isMaster, "is_master_key should be false for virtual key requests")
		assert.Equal(t, uid, r.Context().Value(ContextKeyUserID))
		assert.Equal(t, tid, r.Context().Value(ContextKeyTeamID))
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key-abc123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called, "downstream handler should be called for valid virtual key")
}

func TestVirtualKey_BlockedKeyReturns403(t *testing.T) {
	validator := &mockValidator{info: &TokenInfo{Blocked: true}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-blocked-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestVirtualKey_NilValidator_NonMasterKeyReturns401(t *testing.T) {
	// cfg.Validator is nil — no DB lookup possible, non-master key must be rejected
	authMW := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master"})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-some-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVirtualKey_DBUnavailableReturns503(t *testing.T) {
	validator := &mockValidator{err: ErrDBUnavailable}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-some-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestErrorLogger_BlockedKeyLogs403(t *testing.T) {
	spy := &spyErrorLogger{}
	validator := &mockValidator{info: &TokenInfo{Blocked: true}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey:   "sk-master",
		Validator:   validator,
		ErrorLogger: spy,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-blocked-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	// logFailure is called in a goroutine; wait briefly for it
	require.Eventually(t, func() bool { return len(spy.snapshot()) == 1 }, time.Second, 10*time.Millisecond)
	got := spy.snapshot()
	assert.Equal(t, 403, got[0].statusCode)
	assert.Equal(t, "API key is blocked", got[0].errorMsg)
}

func TestErrorLogger_InvalidKeyLogs401(t *testing.T) {
	spy := &spyErrorLogger{}
	validator := &mockValidator{err: ErrKeyNotFound}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey:   "sk-master",
		Validator:   validator,
		ErrorLogger: spy,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-bad-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Eventually(t, func() bool { return len(spy.snapshot()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, 401, spy.snapshot()[0].statusCode)
}

func TestErrorLogger_MissingKeyLogs401(t *testing.T) {
	spy := &spyErrorLogger{}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey:   "sk-master",
		ErrorLogger: spy,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	// No Authorization header
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Eventually(t, func() bool { return len(spy.snapshot()) == 1 }, time.Second, 10*time.Millisecond)
	got := spy.snapshot()
	assert.Equal(t, 401, got[0].statusCode)
	assert.Equal(t, "missing API key", got[0].errorMsg)
}

func TestJWTValidationFailure_DoesNotLogRawToken(t *testing.T) {
	const rawJWT = "eyJhbGciOiJIUzI1NiIsImtpZCI6Im1pc3NpbmciLCJ0eXAiOiJKV1QifQ.eyJzdWIiOiJ1c2VyIn0.d2l0aG91dC1hLXJlYWwtc2lnbmF0dXJl"
	spy := &spyErrorLogger{}
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	validator := auth.NewJWTValidator(auth.JWTConfig{})
	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey:     "sk-master",
		JWTValidator:  validator,
		EnableJWTAuth: true,
		ErrorLogger:   spy,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+rawJWT)
	req = req.WithContext(logger.WithContext(req.Context()))
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NotContains(t, logs.String(), rawJWT)
	require.Eventually(t, func() bool { return len(spy.snapshot()) == 1 }, time.Second, 10*time.Millisecond)
	got := spy.snapshot()[0]
	assert.Equal(t, 401, got.statusCode)
	assert.Equal(t, "invalid JWT token", got.errorMsg)
	assert.NotContains(t, got.errorMsg, rawJWT)
	assert.NotContains(t, got.apiKeyHash, rawJWT)
}

func TestErrorLogger_SuccessNoLog(t *testing.T) {
	spy := &spyErrorLogger{}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey:   "sk-master",
		ErrorLogger: spy,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Give goroutine time to fire (it shouldn't)
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, spy.snapshot(), "no error log for successful auth")
}

// BenchmarkAuthMiddleware_VirtualKey measures end-to-end latency of the auth
// middleware for virtual key requests. Validates SC-004: response latency
// does not increase compared to baseline.
func BenchmarkAuthMiddleware_VirtualKey(b *testing.B) {
	uid := "user-bench"
	tid := "team-bench"
	validator := &mockValidator{info: &TokenInfo{UserID: &uid, TeamID: &tid}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer sk-virtual-key-bench")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

// --- extractClientIP tests ---

func TestExtractClientIP_CFConnectingIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	assert.Equal(t, "1.2.3.4", extractClientIP(req))
}

func TestExtractClientIP_CFConnectingIPTakesPrecedenceOverRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	req.Header.Set("CF-Connecting-IP", "203.0.113.42")
	assert.Equal(t, "203.0.113.42", extractClientIP(req))
}

func TestExtractClientIP_InvalidCFHeader_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	assert.Equal(t, "10.0.0.1", extractClientIP(req))
}

func TestExtractClientIP_NoCFHeader_UsesRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:9000"
	assert.Equal(t, "192.168.1.1", extractClientIP(req))
}

func TestExtractClientIP_RemoteAddrWithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1" // no port
	assert.Equal(t, "192.168.1.1", extractClientIP(req))
}

func TestExtractClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("CF-Connecting-IP", "2001:db8::1")
	assert.Equal(t, "2001:db8::1", extractClientIP(req))
}

// --- ContextKeyRequesterIP injection tests ---

func TestAuthMiddleware_MasterKey_InjectsRequesterIP(t *testing.T) {
	var gotIP string
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master"})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotIP, _ = r.Context().Value(ContextKeyRequesterIP).(string)
		}),
	)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "1.2.3.4", gotIP)
}

func TestAuthMiddleware_VirtualKey_InjectsRequesterIP(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{UserID: &uid}}
	var gotIP string
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotIP, _ = r.Context().Value(ContextKeyRequesterIP).(string)
		}),
	)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual")
	req.Header.Set("CF-Connecting-IP", "5.6.7.8")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "5.6.7.8", gotIP)
}

// --- Models restriction tests ---

func TestAuthMiddleware_ModelNotAllowed(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"qwen3-30b"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	body := bytes.NewReader([]byte(`{"model":"claude-opus-4-5","messages":[{"role":"user","content":"hi"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "model not allowed")
}

func TestAuthMiddleware_ModelAllowed(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"qwen3-30b"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	called := false
	body := bytes.NewReader([]byte(`{"model":"qwen3-30b","messages":[{"role":"user","content":"hi"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestAuthMiddleware_EmptyModelsAllowAll(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	called := false
	body := bytes.NewReader([]byte(`{"model":"any-model-name","messages":[{"role":"user","content":"hi"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestAuthMiddleware_ModelWildcardMatch(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"qwen3-*"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	called := false
	body := bytes.NewReader([]byte(`{"model":"qwen3-30b","messages":[{"role":"user","content":"hi"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestAuthMiddleware_BodyBufferingPreservesBody(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"qwen3-30b"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	originalBody := `{"model":"qwen3-30b","messages":[{"role":"user","content":"hello world"}]}`
	body := bytes.NewReader([]byte(originalBody))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	var downstreamBody string
	authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		downstreamBody = string(b)
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, originalBody, downstreamBody, "downstream handler must see the full original body")
}

func TestAuthMiddleware_ModelFromURLPath(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"gpt-4"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	called := false
	// No model in body, model extracted from URL path
	req := httptest.NewRequest(http.MethodPost, "/openai/deployments/gpt-4/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestAuthMiddleware_ModelFromURLPath_NotAllowed(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"gpt-4"},
	}}

	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	// Model from path is gpt-4o, but only gpt-4 is allowed
	req := httptest.NewRequest(http.MethodPost, "/openai/deployments/gpt-4o/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	rr := httptest.NewRecorder()

	authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthMiddleware_ModelFromModelsPath(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		Models: []string{"qwen3-30b"},
	}}
	authMW := NewAuthMiddleware(AuthConfig{
		MasterKey: "sk-master",
		Validator: validator,
	})

	for _, path := range []string{"/v1/models/qwen3-30b", "/models/qwen3-30b"} {
		t.Run(path, func(t *testing.T) {
			called := false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer sk-virtual-key")
			rr := httptest.NewRecorder()

			authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.True(t, called)
		})
	}
}

// --- RPM/TPM limit injection tests (T007–T011) ---

func ptrInt64(v int64) *int64 { return &v }

func TestAuthMiddleware_InjectsRPMLimit(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID:   &uid,
		RpmLimit: ptrInt64(10),
	}}

	var gotRPM int64
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotRPM, _ = r.Context().Value(rpmLimitKey).(int64)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, int64(10), gotRPM)
}

func TestAuthMiddleware_InjectsTPMLimit(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID:   &uid,
		TpmLimit: ptrInt64(200000),
	}}

	var gotTPM int64
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotTPM, _ = r.Context().Value(tpmLimitKey).(int64)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, int64(200000), gotTPM)
}

func TestAuthMiddleware_FallbackToTeamRPM(t *testing.T) {
	uid := "user-1"
	tid := "team-42"
	q := &fakeQuerier{
		result: db.VerificationToken{
			UserID:  &uid,
			TeamID:  &tid,
			Blocked: ptr(false),
			// RpmLimit nil at key level → should inherit from team
		},
		team: db.TeamTable{
			TeamID:   tid,
			RpmLimit: ptrInt64(20),
		},
	}
	validator := &DBValidator{DB: q}

	var gotRPM int64
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotRPM, _ = r.Context().Value(rpmLimitKey).(int64)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, int64(20), gotRPM)
}

func TestAuthMiddleware_FallbackToOrgTPM(t *testing.T) {
	uid := "user-1"
	oid := "org-99"
	q := &fakeQuerier{
		result: db.VerificationToken{
			UserID:         &uid,
			OrganizationID: &oid,
			Blocked:        ptr(false),
			// TpmLimit nil at key level, no team → should inherit from org
		},
		org: db.OrganizationTable{
			OrganizationID: oid,
			TpmLimit:       ptrInt64(500000),
		},
	}
	validator := &DBValidator{DB: q}

	var gotTPM int64
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotTPM, _ = r.Context().Value(tpmLimitKey).(int64)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, int64(500000), gotTPM)
}

func TestAuthMiddleware_NullLimitsNoInjection(t *testing.T) {
	uid := "user-1"
	validator := &mockValidator{info: &TokenInfo{
		UserID: &uid,
		// All limits nil
	}}

	var gotRPM, gotTPM int64
	handler := NewAuthMiddleware(AuthConfig{MasterKey: "sk-master", Validator: validator})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			gotRPM, _ = r.Context().Value(rpmLimitKey).(int64)
			gotTPM, _ = r.Context().Value(tpmLimitKey).(int64)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-virtual-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, int64(0), gotRPM, "nil RPM limit should read as zero value")
	assert.Equal(t, int64(0), gotTPM, "nil TPM limit should read as zero value")
}

// --- US5: Key Expiry Tests ---

func TestAuthMiddleware_ExpiredKey(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	authMW := NewAuthMiddleware(AuthConfig{
		Validator: &mockValidator{info: &TokenInfo{Expires: &past}},
	})

	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called for expired key")
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAuthMiddleware_ValidExpiry(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	authMW := NewAuthMiddleware(AuthConfig{
		Validator: &mockValidator{info: &TokenInfo{Expires: &future}},
	})

	called := false
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthMiddleware_NullExpiryNeverExpires(t *testing.T) {
	authMW := NewAuthMiddleware(AuthConfig{
		Validator: &mockValidator{info: &TokenInfo{Expires: nil}},
	})

	called := false
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- US4: Max Parallel Tests (context injection only) ---

func TestAuthMiddleware_InjectsMaxParallel(t *testing.T) {
	maxP := int32(5)
	authMW := NewAuthMiddleware(AuthConfig{
		Validator: &mockValidator{info: &TokenInfo{MaxParallelRequests: &maxP}},
	})

	var gotParallel int
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotParallel, _ = r.Context().Value(maxParallelLimitKey).(int)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 5, gotParallel)
}

// --- Edge Case: rpm_limit=0 blocks all ---

func TestAuthMiddleware_ZeroRPMNotInjected(t *testing.T) {
	zero := int64(0)
	authMW := NewAuthMiddleware(AuthConfig{
		Validator: &mockValidator{info: &TokenInfo{RpmLimit: &zero}},
	})

	var gotRPM int64
	var hasRPM bool
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRPM, hasRPM = r.Context().Value(rpmLimitKey).(int64)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.False(t, hasRPM, "rpm_limit=0 should not be injected (0 means no limit)")
	assert.Equal(t, int64(0), gotRPM)
}

func TestAuthMiddleware_PropagatesAllowedModels(t *testing.T) {
	for name, models := range map[string][]string{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			authMW := NewAuthMiddleware(AuthConfig{
				Validator: &mockValidator{info: &TokenInfo{Models: models}},
			})

			var got []string
			var present bool
			handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, present = r.Context().Value(ContextKeyAllowedModels).([]string)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("Authorization", "Bearer sk-test-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
			require.True(t, present, "virtual-key model scope must be present in context")
			assert.Equal(t, models, got)
		})
	}
}
