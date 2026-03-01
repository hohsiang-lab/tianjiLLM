package scheduler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock implementations ---

type mockBudgetResetter struct{ err error }

func (m *mockBudgetResetter) ResetBudgetForExpiredTokens(_ context.Context) error { return m.err }

type mockSpendLogDeleter struct{ err error }

func (m *mockSpendLogDeleter) DeleteOldSpendLogs(_ context.Context, _ pgtype.Timestamptz) error {
	return m.err
}

type mockCredentialLister struct {
	creds []db.CredentialTable
	err   error
}

func (m *mockCredentialLister) ListCredentials(_ context.Context) ([]db.CredentialTable, error) {
	return m.creds, m.err
}

type mockExpiredTokenLister struct {
	tokens []db.VerificationToken
	err    error
}

func (m *mockExpiredTokenLister) ListExpiredTokens(_ context.Context) ([]db.VerificationToken, error) {
	return m.tokens, m.err
}

type mockArchiver struct{ err error }

func (m *mockArchiver) Archive(_ context.Context, _, _ time.Time) error { return m.err }

type mockFlusher struct{ flushed bool }

func (m *mockFlusher) Flush() { m.flushed = true }

type mockKeyFetcher struct {
	key string
	err error
}

func (m *mockKeyFetcher) FetchKey(_ context.Context, _ string) (string, error) {
	return m.key, m.err
}

type mockKeySwapper struct{ swapped []string }

func (m *mockKeySwapper) SwapKey(cred, _ string) { m.swapped = append(m.swapped, cred) }

// --- tests ---

func TestBudgetResetJob(t *testing.T) {
	j := &BudgetResetJob{DB: &mockBudgetResetter{}}
	assert.Equal(t, "budget_reset", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestBudgetResetJob_Error(t *testing.T) {
	j := &BudgetResetJob{DB: &mockBudgetResetter{err: errors.New("db error")}}
	assert.Error(t, j.Run(context.Background()))
}

func TestSpendLogCleanupJob(t *testing.T) {
	j := &SpendLogCleanupJob{DB: &mockSpendLogDeleter{}, Retention: 90 * 24 * time.Hour}
	assert.Equal(t, "spend_log_cleanup", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestSpendLogCleanupJob_Error(t *testing.T) {
	j := &SpendLogCleanupJob{DB: &mockSpendLogDeleter{err: errors.New("db error")}, Retention: 90 * 24 * time.Hour}
	assert.Error(t, j.Run(context.Background()))
}

func TestCredentialRefreshJob(t *testing.T) {
	j := &CredentialRefreshJob{DB: &mockCredentialLister{creds: []db.CredentialTable{{CredentialName: "openai"}}}}
	assert.Equal(t, "credential_refresh", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestCredentialRefreshJob_Error(t *testing.T) {
	j := &CredentialRefreshJob{DB: &mockCredentialLister{err: errors.New("db error")}}
	assert.Error(t, j.Run(context.Background()))
}

func TestKeyRotationJob(t *testing.T) {
	j := &KeyRotationJob{DB: &mockExpiredTokenLister{tokens: []db.VerificationToken{{Token: "tok1"}}}}
	assert.Equal(t, "key_rotation", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestKeyRotationJob_Error(t *testing.T) {
	j := &KeyRotationJob{DB: &mockExpiredTokenLister{err: errors.New("db error")}}
	assert.Error(t, j.Run(context.Background()))
}

func TestSpendArchivalJob(t *testing.T) {
	j := &SpendArchivalJob{Archiver: &mockArchiver{}, Retention: 30 * 24 * time.Hour}
	assert.Equal(t, "spend_archival", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestSpendBatchWriteJob(t *testing.T) {
	mf := &mockFlusher{}
	j := &SpendBatchWriteJob{Flusher: mf}
	assert.Equal(t, "spend_batch_write", j.Name())
	require.NoError(t, j.Run(context.Background()))
	assert.True(t, mf.flushed)
}

func TestProviderKeyRotationJob(t *testing.T) {
	swapper := &mockKeySwapper{}
	j := &ProviderKeyRotationJob{
		Fetcher:     &mockKeyFetcher{key: "new-key-123"},
		Swapper:     swapper,
		Credentials: []string{"openai", "anthropic"},
	}
	assert.Equal(t, "provider_key_rotation", j.Name())
	require.NoError(t, j.Run(context.Background()))
	assert.Equal(t, []string{"openai", "anthropic"}, swapper.swapped)
}

func TestProviderKeyRotationJob_FetchError(t *testing.T) {
	swapper := &mockKeySwapper{}
	j := &ProviderKeyRotationJob{
		Fetcher:     &mockKeyFetcher{err: errors.New("vault unavailable")},
		Swapper:     swapper,
		Credentials: []string{"openai"},
	}
	// Should not error — logs and continues
	require.NoError(t, j.Run(context.Background()))
	assert.Empty(t, swapper.swapped)
}

func TestHealthCheckJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	j := &HealthCheckJob{
		Endpoints: []string{srv.URL},
		Client:    srv.Client(),
	}
	assert.Equal(t, "health_check", j.Name())
	require.NoError(t, j.Run(context.Background()))
}

func TestHealthCheckJob_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	j := &HealthCheckJob{
		Endpoints: []string{srv.URL},
		Client:    srv.Client(),
	}
	require.NoError(t, j.Run(context.Background()))
}

func TestHealthCheckJob_Unreachable(t *testing.T) {
	j := &HealthCheckJob{
		Endpoints: []string{"http://127.0.0.1:1"},
		Client:    &http.Client{Timeout: 100 * time.Millisecond},
	}
	require.NoError(t, j.Run(context.Background()))
}

type mockPolicyLoader struct{ err error }

func (m *mockPolicyLoader) Load(_ context.Context) error { return m.err }

func TestPolicyHotReloadJob(t *testing.T) {
	j := &PolicyHotReloadJob{Engine: &mockPolicyLoader{}}
	assert.Equal(t, "policy_hot_reload", j.Name())
	assert.NoError(t, j.Run(context.Background()))
}

func TestPolicyHotReloadJob_Error(t *testing.T) {
	j := &PolicyHotReloadJob{Engine: &mockPolicyLoader{err: errors.New("load error")}}
	assert.Error(t, j.Run(context.Background()))
}
