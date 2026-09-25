package scheduler

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestJobNames(t *testing.T) {
	tests := []struct {
		job  Job
		want string
	}{
		{&BudgetResetJob{}, "budget_reset"},
		{&SpendLogCleanupJob{}, "spend_log_cleanup"},
		{&SpendArchivalJob{}, "spend_archival"},
		{&SpendBatchWriteJob{}, "spend_batch_write"},
		{&CredentialRefreshJob{}, "credential_refresh"},
		{&OpenAISubscriptionProactiveRefreshJob{}, "openai_subscription_proactive_refresh"},
		{&OpenAISubscriptionDiscordUsageReportJob{}, "openai_subscription_discord_usage_report"},
		{&KeyRotationJob{}, "key_rotation"},
		{&HealthCheckJob{}, "health_check"},
	}
	for _, tt := range tests {
		if got := tt.job.Name(); got != tt.want {
			t.Errorf("%T.Name() = %q, want %q", tt.job, got, tt.want)
		}
	}
}

func TestOpenAISubscriptionProactiveRefreshJob_RegisteredEveryFiveMinutesWhenDBConfigured(t *testing.T) {
	if OpenAISubscriptionProactiveRefreshInterval != 5*time.Minute {
		t.Fatalf("OpenAISubscriptionProactiveRefreshInterval = %s, want 5m", OpenAISubscriptionProactiveRefreshInterval)
	}
}

func TestOpenAISubscriptionProactiveRefreshJob_RunCallsRefresh(t *testing.T) {
	called := false
	j := &OpenAISubscriptionProactiveRefreshJob{
		RunRefresh: func(context.Context) (any, error) {
			called = true
			return map[string]int{"refreshed": 1}, nil
		},
	}

	err := j.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected RunRefresh to be called")
	}
}

func TestOpenAISubscriptionProactiveRefreshJob_DistributedLockSkipsSecondRunner(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()

	lock := NewDistributedLock(client)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	job := &OpenAISubscriptionProactiveRefreshJob{
		RunRefresh: func(context.Context) (any, error) {
			calls.Add(1)
			if calls.Load() == 1 {
				close(started)
				<-release
			}
			return nil, nil
		},
	}
	locked := NewWithLock(job, lock, time.Second)

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- locked.Run(context.Background())
	}()
	<-started

	secondErr := locked.Run(context.Background())
	close(release)

	if secondErr != nil {
		t.Fatalf("second locked runner should skip without error: %v", secondErr)
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("first runner returned error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("RunRefresh calls = %d, want 1", calls.Load())
	}
}

func TestOpenAISubscriptionDiscordUsageReportJob_RunAndDistributedLock(t *testing.T) {
	if OpenAISubscriptionDiscordUsageReportInterval != 30*time.Minute {
		t.Fatalf("OpenAISubscriptionDiscordUsageReportInterval = %s, want 30m", OpenAISubscriptionDiscordUsageReportInterval)
	}

	var logs bytes.Buffer
	oldLogOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldLogOutput)

	var deadline time.Time
	job := &OpenAISubscriptionDiscordUsageReportJob{
		RunReport: func(ctx context.Context) OpenAISubscriptionDiscordUsageReportJobResult {
			var ok bool
			deadline, ok = ctx.Deadline()
			if !ok {
				t.Error("report run context must have a deadline")
			}
			return OpenAISubscriptionDiscordUsageReportJobResult{
				CredentialCount: 1,
				AttemptedParts:  1,
				DeliveredParts:  1,
			}
		},
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("job Run() error = %v", err)
	}
	if remaining := time.Until(deadline); deadline.IsZero() || remaining <= 0 || remaining > OpenAISubscriptionDiscordUsageReportTimeout {
		t.Fatalf("report deadline = %s, want a remaining duration no later than %s", deadline, OpenAISubscriptionDiscordUsageReportTimeout)
	}

	job.RunReport = func(context.Context) OpenAISubscriptionDiscordUsageReportJobResult {
		return OpenAISubscriptionDiscordUsageReportJobResult{FailureReason: "Bearer scheduler-secret"}
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("safe job failure returned error: %v", err)
	}
	if got := logs.String(); strings.Contains(got, "scheduler-secret") || !strings.Contains(got, "report_failed") {
		t.Fatalf("unsafe scheduler log: %q", got)
	}

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	lock := NewDistributedLock(client)
	var calls atomic.Int32
	release := make(chan struct{})
	firstStarted := make(chan struct{})
	locked := NewWithLock(&OpenAISubscriptionDiscordUsageReportJob{
		RunReport: func(context.Context) OpenAISubscriptionDiscordUsageReportJobResult {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-release
			}
			return OpenAISubscriptionDiscordUsageReportJobResult{}
		},
	}, lock, time.Second)

	firstDone := make(chan error, 1)
	go func() { firstDone <- locked.Run(context.Background()) }()
	<-firstStarted
	if err := locked.Run(context.Background()); err != nil {
		t.Fatalf("second locked runner should skip without error: %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first locked runner returned error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("report runners started = %d, want 1", calls.Load())
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
