package scheduler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobNames(t *testing.T) {
	tests := []struct {
		job  Job
		want string
	}{
		{&BudgetResetJob{}, "budget_reset"},
		{&SpendLogCleanupJob{}, "spend_log_cleanup"},
		{&PolicyHotReloadJob{}, "policy_hot_reload"},
		{&SpendArchivalJob{}, "spend_archival"},
		{&SpendBatchWriteJob{}, "spend_batch_write"},
		{&CredentialRefreshJob{}, "credential_refresh"},
		{&KeyRotationJob{}, "key_rotation"},
		{&HealthCheckJob{}, "health_check"},
	}
	for _, tt := range tests {
		if got := tt.job.Name(); got != tt.want {
			t.Errorf("%T.Name() = %q, want %q", tt.job, got, tt.want)
		}
	}
}

// mockFlusher implements SpendFlusher for testing.
type mockFlusher struct{ flushed bool }

func (m *mockFlusher) Flush() { m.flushed = true }

func TestSpendBatchWriteJob_Run(t *testing.T) {
	f := &mockFlusher{}
	j := &SpendBatchWriteJob{Flusher: f}
	err := j.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.flushed {
		t.Error("expected Flush() to be called")
	}
}

// mockKeyFetcher/Swapper for ProviderKeyRotationJob.
type mockKeyFetcher struct{ key string }

func (m *mockKeyFetcher) FetchKey(_ context.Context, _ string) (string, error) {
	return m.key, nil
}

type mockKeySwapper struct{ swapped map[string]string }

func (m *mockKeySwapper) SwapKey(cred, key string) {
	if m.swapped == nil {
		m.swapped = make(map[string]string)
	}
	m.swapped[cred] = key
}

func TestProviderKeyRotationJob_Run(t *testing.T) {
	fetcher := &mockKeyFetcher{key: "new-secret-key"}
	swapper := &mockKeySwapper{}
	j := &ProviderKeyRotationJob{
		Fetcher:     fetcher,
		Swapper:     swapper,
		Credentials: []string{"openai", "anthropic"},
	}
	err := j.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if swapper.swapped["openai"] != "new-secret-key" {
		t.Errorf("expected key to be swapped for openai")
	}
}

func TestHealthCheckJob_Run(t *testing.T) {
	// Use a test server that responds with 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	j := &HealthCheckJob{
		Endpoints: []string{srv.URL},
		Client:    srv.Client(),
	}
	err := j.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthCheckJob_Run_BadEndpoint(t *testing.T) {
	j := &HealthCheckJob{
		Endpoints: []string{"http://invalid.endpoint.example.com"},
		Client:    &http.Client{Timeout: 100 * time.Millisecond},
	}
	err := j.Run(context.Background())
	// Should not return error even if endpoint is unreachable
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type mockArchiver struct{ err error }

func (m *mockArchiver) Archive(ctx context.Context, from, to time.Time) error {
	return m.err
}

func TestSpendArchivalJob_Run_Success(t *testing.T) {
	j := &SpendArchivalJob{Archiver: &mockArchiver{}, Retention: 24 * time.Hour}
	if err := j.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSpendArchivalJob_Run_Error(t *testing.T) {
	j := &SpendArchivalJob{Archiver: &mockArchiver{err: errTest}, Retention: time.Hour}
	if err := j.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestProviderKeyRotationJob_Name(t *testing.T) {
	j := &ProviderKeyRotationJob{}
	if j.Name() != "provider_key_rotation" {
		t.Fatalf("got %q", j.Name())
	}
}

var errTest = errors.New("test error")
