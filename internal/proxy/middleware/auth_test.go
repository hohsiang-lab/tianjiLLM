package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

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
