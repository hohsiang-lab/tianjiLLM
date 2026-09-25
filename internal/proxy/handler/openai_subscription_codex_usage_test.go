package handler

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCodexUsageFetcher struct {
	mu        sync.Mutex
	calls     []chatgptcodex.UsageRequest
	responses []fakeCodexUsageResponse
}

type fakeCodexResetConsumer struct {
	mu        sync.Mutex
	calls     []chatgptcodex.ResetCreditRequest
	responses []fakeCodexResetResponse
}

type fakeCodexUsageResponse struct {
	snapshot chatgptcodex.UsageSnapshot
	err      error
}

type fakeCodexResetResponse struct {
	result chatgptcodex.ResetCreditResult
	err    error
}

type recordingCache struct {
	mu      sync.Mutex
	values  map[string][]byte
	setTTLs []time.Duration
	sets    int
	mgets   int
	gets    int
}

func newRecordingCache() *recordingCache {
	return &recordingCache{values: make(map[string][]byte)}
}

func (c *recordingCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	return c.values[key], nil
}

func (c *recordingCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	c.setTTLs = append(c.setTTLs, ttl)
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *recordingCache) CompareAndSet(_ context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.values[key]
	if (expected == nil) != !ok || (ok && !bytes.Equal(current, expected)) {
		return false, nil
	}
	c.sets++
	c.setTTLs = append(c.setTTLs, ttl)
	c.values[key] = append([]byte(nil), value...)
	return true, nil
}

func (c *recordingCache) AcquireLock(_ context.Context, key, token string, _ time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.values[key]; ok {
		return false, nil
	}
	c.values[key] = []byte(token)
	return true, nil
}

func (c *recordingCache) ReleaseLock(_ context.Context, key, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if current, ok := c.values[key]; ok && bytes.Equal(current, []byte(token)) {
		delete(c.values, key)
	}
	return nil
}

func (c *recordingCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, key)
	return nil
}

func (c *recordingCache) MGet(_ context.Context, keys ...string) ([][]byte, error) {
	c.mu.Lock()
	c.mgets++
	defer c.mu.Unlock()
	results := make([][]byte, len(keys))
	for i, key := range keys {
		results[i] = c.values[key]
	}
	return results, nil
}

func (c *recordingCache) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

func (c *recordingCache) getCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets
}

func (c *recordingCache) mgetCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mgets
}

func (c *recordingCache) lastTTL() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.setTTLs) == 0 {
		return 0
	}
	return c.setTTLs[len(c.setTTLs)-1]
}

func (f *fakeCodexUsageFetcher) Fetch(_ context.Context, req chatgptcodex.UsageRequest) (chatgptcodex.UsageSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return chatgptcodex.UsageSnapshot{}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp.snapshot, resp.err
}

func (f *fakeCodexUsageFetcher) callsSnapshot() []chatgptcodex.UsageRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]chatgptcodex.UsageRequest(nil), f.calls...)
}

func (f *fakeCodexResetConsumer) ConsumeResetCredit(_ context.Context, req chatgptcodex.ResetCreditRequest) (chatgptcodex.ResetCreditResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return chatgptcodex.ResetCreditResult{}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp.result, resp.err
}

func (f *fakeCodexResetConsumer) callsSnapshot() []chatgptcodex.ResetCreditRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]chatgptcodex.ResetCreditRequest(nil), f.calls...)
}

func requireCodexUsageCalls(t *testing.T, fetcher *fakeCodexUsageFetcher, count int) []chatgptcodex.UsageRequest {
	t.Helper()
	var calls []chatgptcodex.UsageRequest
	require.Eventually(t, func() bool {
		calls = fetcher.callsSnapshot()
		return len(calls) == count
	}, time.Second, 10*time.Millisecond)
	return calls
}

func testUsageSnapshot() chatgptcodex.UsageSnapshot {
	v := 0.06
	return chatgptcodex.UsageSnapshot{
		Email:         "operator@example.com",
		PlanType:      "pro",
		PrimaryWindow: chatgptcodex.UsageWindow{Name: "primary_5h", UsedPercent: &v, ResetAt: "2026-05-13T07:30:00Z", Status: "available"},
	}
}

func TestPreloadOpenAISubscriptionCodexUsageFetchesGlobalCredentials(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.10, 0.20, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.20, 1.00, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "exhausted")},
	}}
	h.CodexUsageFetcher = fetcher
	h.Config.ModelList = []config.ModelConfig{
		{
			ModelName: "gpt-5.5",
			TianjiParams: config.TianjiParams{
				Model:                       "openai/gpt-5.5",
				OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
			},
		},
		{
			ModelName: "openai/*",
			TianjiParams: config.TianjiParams{
				Model:                       "openai/*",
				OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
			},
		},
		{
			ModelName: "direct-openai",
			TianjiParams: config.TianjiParams{
				Model: "openai/gpt-4o",
			},
		},
	}
	h.RuntimeModels = NewRuntimeModelSource(context.Background(), h.Config, nil, nil)

	result := h.PreloadOpenAISubscriptionCodexUsage(context.Background())

	assert.Equal(t, OpenAISubscriptionCodexUsagePreloadResult{CredentialIDs: 2, Refreshed: 2}, result)
	calls := fetcher.callsSnapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "access-a", calls[0].AccessToken)
	assert.Equal(t, "acct-a", calls[0].AccountID)
	assert.Equal(t, "access-b", calls[1].AccessToken)
	assert.Equal(t, "acct-b", calls[1].AccountID)

	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	assert.True(t, metaA.Selectable)
	require.True(t, metaB.Known)
	assert.False(t, metaB.Available)
	assert.False(t, metaB.Selectable)
}

func TestOpenAISubscriptionCodexUsage_RateLimitOnlySnapshotIsMeaningful(t *testing.T) {
	allowed := false
	limitReached := true
	result := OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimit: &chatgptcodex.UsageRateLimit{
				Allowed:      &allowed,
				LimitReached: &limitReached,
			},
		},
	}

	assert.True(t, codexUsageResultHasSnapshot(result))
}

func TestOpenAISubscriptionCodexUsage_ResetCreditsOnlySnapshotIsMeaningful(t *testing.T) {
	availableCount := 0
	result := OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimitResetCredits: &chatgptcodex.UsageResetCredits{
				AvailableCount: &availableCount,
			},
		},
	}

	assert.True(t, codexUsageResultHasSnapshot(result))
}

func TestOpenAISubscriptionCodexUsage_BucketWindowOnlySnapshotIsMeaningful(t *testing.T) {
	used := 0.0
	assert.True(t, codexUsageResultHasSnapshot(OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			AdditionalBuckets: []chatgptcodex.UsageBucket{{
				Name: "spark",
				PrimaryWindow: chatgptcodex.UsageWindow{
					UsedPercent: &used,
				},
			}},
		},
	}))
	assert.False(t, codexUsageResultHasSnapshot(OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			AdditionalBuckets: []chatgptcodex.UsageBucket{{Name: "spark"}},
		},
	}))
}

func TestOpenAISubscriptionCodexUsage_UsesCacheWithinTTL(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: testUsageSnapshot()}}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	first := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)
	second := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", first.Status)
	require.Equal(t, "fresh", second.Status)
	require.Len(t, fetcher.calls, 1)
	assert.Equal(t, "fresh-access", fetcher.calls[0].AccessToken)
	assert.Equal(t, "acct_123", fetcher.calls[0].AccountID)
}

func TestOpenAISubscriptionCodexUsage_RefreshIntervalUsesConfig(t *testing.T) {
	seconds := 15
	h := &Handlers{Config: &config.ProxyConfig{
		TianjiSettings: config.TianjiSettings{
			CodexUsageRefreshIntervalSeconds: &seconds,
		},
	}}

	assert.Equal(t, 15*time.Second, h.codexUsageRefreshInterval())
}

func TestOpenAISubscriptionCodexUsage_AsyncTimeoutUsesConfig(t *testing.T) {
	seconds := 20
	h := &Handlers{Config: &config.ProxyConfig{
		TianjiSettings: config.TianjiSettings{
			CodexUsageAsyncTimeoutSeconds: &seconds,
		},
	}}

	assert.Equal(t, 20*time.Second, h.codexUsageAsyncTimeout())
}

func TestOpenAISubscriptionCodexUsage_RefreshesAfterFreshnessInterval(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	firstSnapshot := testUsageSnapshot()
	secondSnapshot := testUsageSnapshot()
	secondSnapshot.Email = "updated@example.com"
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: firstSnapshot},
		{snapshot: secondSnapshot},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	first := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)
	h.openAISubscriptionNow = func() time.Time { return now.Add(11 * time.Second) }
	second := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", first.Status)
	require.Equal(t, "fresh", second.Status)
	assert.Equal(t, "operator@example.com", first.Snapshot.Email)
	assert.Equal(t, "updated@example.com", second.Snapshot.Email)
	require.Len(t, fetcher.calls, 2)
}

func TestOpenAISubscriptionCodexUsage_RefreshesOnceAfter401(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "refreshed-access",
			"expires_in":   1800,
			"account_id":   "acct_123",
		},
	}}})
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusUnauthorized, Reason: "auth_error"}},
		{snapshot: testUsageSnapshot()},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", true)

	require.Equal(t, "fresh", got.Status)
	require.Len(t, fetcher.calls, 2)
	assert.Equal(t, "stale-access", fetcher.calls[0].AccessToken)
	assert.Equal(t, "refreshed-access", fetcher.calls[1].AccessToken)
}

func TestOpenAISubscriptionCodexUsageReset_ConsumesCreditAndRefreshesUsage(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	resetter := &fakeCodexResetConsumer{responses: []fakeCodexResetResponse{{
		result: chatgptcodex.ResetCreditResult{Code: "reset", WindowsReset: 1},
	}}}
	availableCount := 0
	snapshot := testUsageSnapshot()
	snapshot.RateLimitResetCredits = &chatgptcodex.UsageResetCredits{AvailableCount: &availableCount}
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: snapshot}}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexResetConsumer = resetter
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	got := h.OpenAISubscriptionCodexUsageReset(context.Background(), "cred-refresh")

	require.Empty(t, got.LastErrorReason)
	assert.Equal(t, "reset", got.Code)
	assert.Equal(t, int64(1), got.WindowsReset)
	assert.Equal(t, "fresh", got.Usage.Status)
	assert.Equal(t, "operator@example.com", got.Usage.Snapshot.Email)
	resetCalls := resetter.callsSnapshot()
	require.Len(t, resetCalls, 1)
	assert.Equal(t, "fresh-access", resetCalls[0].AccessToken)
	assert.Equal(t, "acct_123", resetCalls[0].AccountID)
	assert.NotEmpty(t, resetCalls[0].RedeemRequestID)
	usageCalls := fetcher.callsSnapshot()
	require.Len(t, usageCalls, 1)
	assert.Equal(t, "fresh-access", usageCalls[0].AccessToken)
	assert.Equal(t, "acct_123", usageCalls[0].AccountID)
}

func TestOpenAISubscriptionCodexUsageReset_RefreshesCredentialAfter401(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "refreshed-access",
			"expires_in":   1800,
			"account_id":   "acct_123",
		},
	}}})
	resetter := &fakeCodexResetConsumer{responses: []fakeCodexResetResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusUnauthorized, Reason: "auth_error"}},
		{result: chatgptcodex.ResetCreditResult{Code: "reset", WindowsReset: 1}},
	}}
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: testUsageSnapshot()}}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexResetConsumer = resetter
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	got := h.OpenAISubscriptionCodexUsageReset(context.Background(), "cred-refresh")

	require.Empty(t, got.LastErrorReason)
	assert.Equal(t, "reset", got.Code)
	resetCalls := resetter.callsSnapshot()
	require.Len(t, resetCalls, 2)
	assert.Equal(t, "stale-access", resetCalls[0].AccessToken)
	assert.Equal(t, "refreshed-access", resetCalls[1].AccessToken)
	assert.Equal(t, resetCalls[0].RedeemRequestID, resetCalls[1].RedeemRequestID)
	assert.NotEmpty(t, resetCalls[0].RedeemRequestID)
	usageCalls := fetcher.callsSnapshot()
	require.Len(t, usageCalls, 1)
	assert.Equal(t, "refreshed-access", usageCalls[0].AccessToken)
}

func TestOpenAISubscriptionCodexUsage_RefreshFailureAfter401DoesNotDisableCredential(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token rejected",
		},
	}}})
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusUnauthorized, Reason: "auth_error"}},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", true)

	assert.Equal(t, "unavailable", got.Status)
	assert.Equal(t, string(OpenAISubscriptionCredentialRefreshErr), got.LastErrorReason)
	require.Len(t, fetcher.calls, 1)
	assert.Equal(t, "stale-access", fetcher.calls[0].AccessToken)
	info := store.credentialInfo(t)
	assert.Equal(t, "active", info.Status)
	assert.Empty(t, info.DisabledReason)
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Zero(t, infoWrites)
}

func TestOpenAISubscriptionCodexUsage_ExpiredTokenRefreshFailureDoesNotDisableCredential(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "expired-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(-time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token rejected",
		},
	}}})
	fetcher := &fakeCodexUsageFetcher{}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	assert.Equal(t, "unavailable", got.Status)
	assert.Equal(t, string(OpenAISubscriptionCredentialRefreshErr), got.LastErrorReason)
	assert.Empty(t, fetcher.calls)
	info := store.credentialInfo(t)
	assert.Equal(t, "active", info.Status)
	assert.Empty(t, info.DisabledReason)
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Zero(t, infoWrites)
}

func TestOpenAISubscriptionCodexUsage_BackoffPreservesLastSuccess(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	snapshot := testUsageSnapshot()
	snapshot.PrimaryWindow.ResetAt = ""
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: snapshot},
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusTooManyRequests, Reason: "rate_limited"}},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageTTL = time.Second
	h.CodexUsageBackoffBase = time.Minute
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 7 * time.Second }

	first := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)
	h.openAISubscriptionNow = func() time.Time { return now.Add(2 * time.Second) }
	second := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", true)

	require.Equal(t, "fresh", first.Status)
	require.Equal(t, "backoff", second.Status)
	assert.Equal(t, "rate_limited", second.LastErrorReason)
	assert.Equal(t, "operator@example.com", second.Snapshot.Email)
	assert.Equal(t, now.Add(time.Minute+9*time.Second), second.BackoffUntil)
	require.Len(t, fetcher.calls, 2)
}

func TestOpenAISubscriptionCodexUsage_EmptySnapshotPreservesLastSuccess(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	snapshot := testUsageSnapshot()
	snapshot.PrimaryWindow.ResetAt = ""
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: snapshot},
		{snapshot: chatgptcodex.UsageSnapshot{}},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageTTL = time.Second
	h.CodexUsageBackoffBase = time.Minute
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }

	first := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)
	h.openAISubscriptionNow = func() time.Time { return now.Add(2 * time.Second) }
	second := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", first.Status)
	require.Equal(t, "unavailable", second.Status)
	assert.Equal(t, "empty_snapshot", second.LastErrorReason)
	assert.Equal(t, "operator@example.com", second.Snapshot.Email)
	require.Len(t, fetcher.calls, 2)
}

func TestOpenAISubscriptionCodexUsage_LoadsCachedSnapshotBeforeFetch(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	cacheBackend := cache.NewMemoryCache()
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Cache = cacheBackend
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-refresh",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now.Add(-5 * time.Second),
		ExpiresAt:    now.Add(5 * time.Hour),
	}, now)
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusTooManyRequests, Reason: "rate_limited"}},
	}}
	h.CodexUsageFetcher = fetcher
	h.CodexUsageTTL = time.Second
	h.CodexUsageBackoffBase = time.Minute
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", got.Status)
	assert.Equal(t, "operator@example.com", got.Snapshot.Email)
	assert.Equal(t, now.Add(-5*time.Second), got.FetchedAt)
	require.Empty(t, fetcher.calls)
}

func TestOpenAISubscriptionCodexUsage_LegacyBucketCachePreservesWindows(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	cacheBackend := newRecordingCache()
	h := &Handlers{Cache: cacheBackend}
	preservedKey := codexUsageCacheKey("cred-preserve")
	cacheBackend.values[preservedKey] = []byte(`{
		"credential_id":"cred-preserve",
		"status":"fresh",
		"snapshot":{
			"primary_window":{"used_percent":0.04,"reset_at":"2026-05-13T07:30:00Z"},
			"additional_buckets":[
				{"name":"legacy","used_percent":0.20,"reset_at":"2026-05-13T07:30:00Z","status":"available"},
				{"name":"authoritative","primary_window":{"used_percent":0.90,"reset_at":"2026-05-13T08:30:00Z","status":"new"},"used_percent":0.20,"reset_at":"2026-05-13T07:30:00Z","status":"legacy"}
			]
		},
		"fetched_at":"2026-05-13T02:25:00Z",
		"expires_at":"2026-05-13T07:30:00Z"
	}`)
	preserved, ok := h.decodeCachedOpenAISubscriptionCodexUsageResult(context.Background(), "cred-preserve", cacheBackend.values[preservedKey], now)
	require.True(t, ok)
	require.NotNil(t, preserved.Snapshot.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.04, *preserved.Snapshot.PrimaryWindow.UsedPercent, 0.0001)
	require.Len(t, preserved.Snapshot.AdditionalBuckets, 2)
	require.NotNil(t, preserved.Snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.20, *preserved.Snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "2026-05-13T07:30:00Z", preserved.Snapshot.AdditionalBuckets[0].PrimaryWindow.ResetAt)
	assert.Equal(t, "available", preserved.Snapshot.AdditionalBuckets[0].PrimaryWindow.Status)
	require.NotNil(t, preserved.Snapshot.AdditionalBuckets[1].PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.90, *preserved.Snapshot.AdditionalBuckets[1].PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "2026-05-13T08:30:00Z", preserved.Snapshot.AdditionalBuckets[1].PrimaryWindow.ResetAt)
	assert.Equal(t, "new", preserved.Snapshot.AdditionalBuckets[1].PrimaryWindow.Status)
	assert.Contains(t, cacheBackend.values, preservedKey)

	invalidatedKey := codexUsageCacheKey("cred-invalidated")
	cacheBackend.values[invalidatedKey] = []byte(`{
		"credential_id":"cred-invalidated",
		"status":"fresh",
		"snapshot":{
			"additional_buckets":[{"name":"empty"}]
		},
		"fetched_at":"2026-05-13T02:25:00Z",
		"expires_at":"2026-05-13T07:30:00Z"
	}`)
	_, ok = h.decodeCachedOpenAISubscriptionCodexUsageResult(context.Background(), "cred-invalidated", cacheBackend.values[invalidatedKey], now)
	assert.False(t, ok)
	assert.NotContains(t, cacheBackend.values, invalidatedKey)
}

func TestOpenAISubscriptionCodexUsage_LegacyBucketCacheAvoidsRefetch(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	shared := newRecordingCache()
	shared.values[codexUsageCacheKey("cred-refresh")] = []byte(`{
		"credential_id":"cred-refresh",
		"status":"fresh",
		"snapshot":{
			"additional_buckets":[{"name":"legacy","used_percent":0.20,"reset_at":"2026-05-13T07:30:00Z","status":"available"}]
		},
		"fetched_at":"2026-05-13T02:29:55Z",
		"expires_at":"2026-05-13T07:30:00Z"
	}`)
	used := 0.06
	freshSnapshot := chatgptcodex.UsageSnapshot{
		PrimaryWindow: chatgptcodex.UsageWindow{
			UsedPercent: &used,
			ResetAt:     now.Add(time.Hour).Format(time.RFC3339),
		},
	}
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: freshSnapshot}}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Cache = shared
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", got.Status)
	assert.Empty(t, fetcher.calls)
	require.Len(t, got.Snapshot.AdditionalBuckets, 1)
	require.NotNil(t, got.Snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.20, *got.Snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "2026-05-13T07:30:00Z", got.Snapshot.AdditionalBuckets[0].PrimaryWindow.ResetAt)
	assert.Equal(t, "available", got.Snapshot.AdditionalBuckets[0].PrimaryWindow.Status)
}

func TestOpenAISubscriptionCodexUsage_StaleSharedCacheRefreshesBeforeResetTTL(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	cacheBackend := cache.NewMemoryCache()
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Cache = cacheBackend
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-refresh",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now.Add(-11 * time.Second),
		ExpiresAt:    now.Add(5 * time.Hour),
	}, now)
	refreshedSnapshot := testUsageSnapshot()
	refreshedSnapshot.Email = "refreshed@example.com"
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: refreshedSnapshot}}}
	h.CodexUsageFetcher = fetcher

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", got.Status)
	assert.Equal(t, "refreshed@example.com", got.Snapshot.Email)
	assert.Equal(t, now, got.FetchedAt)
	require.Len(t, fetcher.calls, 1)
}

func TestOpenAISubscriptionCodexUsage_SuccessPathPersistsToSharedCache(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	shared := newRecordingCache()
	firstFetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: testUsageSnapshot()}}}
	firstHandler := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	firstHandler.Cache = shared
	firstHandler.CodexUsageFetcher = firstFetcher
	firstHandler.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	first := firstHandler.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", first.Status)
	require.Equal(t, 1, shared.setCount())
	require.Len(t, firstFetcher.calls, 1)

	secondFetcher := &fakeCodexUsageFetcher{}
	secondHandler := newOpenAISubscriptionRefreshHarness(t, now.Add(5*time.Second), store, tokenServer)
	secondHandler.Cache = shared
	secondHandler.CodexUsageFetcher = secondFetcher
	secondHandler.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()

	second := secondHandler.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "fresh", second.Status)
	assert.Equal(t, "operator@example.com", second.Snapshot.Email)
	require.Empty(t, secondFetcher.calls)
}

func TestOpenAISubscriptionCodexUsage_CacheTTLUsesEarliestReset(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	snapshot := testUsageSnapshot()
	snapshot.WeeklyWindow = chatgptcodex.UsageWindow{ResetAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339), UsedPercent: float64Ptr(0.2)}
	snapshot.AdditionalBuckets = []chatgptcodex.UsageBucket{{
		Name: "extra",
		PrimaryWindow: chatgptcodex.UsageWindow{
			UsedPercent: float64Ptr(0.1),
			ResetAt:     strconv.FormatInt(now.Add(2*time.Hour).Unix(), 10),
		},
		WeeklyWindow: chatgptcodex.UsageWindow{
			UsedPercent: float64Ptr(0.2),
			ResetAt:     now.Add(3 * time.Hour).Format(time.RFC3339),
		},
	}}
	result := OpenAISubscriptionCodexUsageResult{Snapshot: snapshot}
	assert.True(t, codexUsageResultHasSnapshot(result))
	earliest, ok := codexUsageEarliestReset(snapshot, now)
	require.True(t, ok)
	assert.True(t, earliest.Equal(now.Add(2*time.Hour)))

	expiresAt := (&Handlers{CodexUsageTTL: time.Hour}).codexUsageExpiresAt(snapshot, now)
	assert.True(t, expiresAt.Equal(now.Add(2*time.Hour)))
}

func TestOpenAISubscriptionCodexUsage_PersistsSharedCacheWithResetTTL(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	shared := newRecordingCache()
	h := &Handlers{Cache: shared}
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-refresh",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now,
		ExpiresAt:    now.Add(5 * time.Hour),
	}, now)

	require.Equal(t, 1, shared.setCount())
	assert.Equal(t, 5*time.Hour, shared.lastTTL())
}

func TestOpenAISubscriptionCodexUsage_CacheTTLFallsBackWhenResetMissing(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	snapshot := testUsageSnapshot()
	snapshot.PrimaryWindow.ResetAt = ""
	expiresAt := (&Handlers{CodexUsageTTL: 2 * time.Minute}).codexUsageExpiresAt(snapshot, now)

	assert.Equal(t, now.Add(2*time.Minute), expiresAt)
}

func TestOpenAISubscriptionCodexUsage_CacheTTLFallsBackWhenAnyPopulatedResetMissing(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	snapshot := testUsageSnapshot()
	snapshot.PrimaryWindow.ResetAt = ""
	snapshot.WeeklyWindow = chatgptcodex.UsageWindow{UsedPercent: float64Ptr(0.4), ResetAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339)}
	expiresAt := (&Handlers{CodexUsageTTL: 2 * time.Minute}).codexUsageExpiresAt(snapshot, now)

	assert.Equal(t, now.Add(2*time.Minute), expiresAt)
}

func TestOpenAISubscriptionCodexUsage_ManualRefreshBypassesSharedCache(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	shared := newRecordingCache()
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Cache = shared
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-refresh",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now.Add(-time.Hour),
		ExpiresAt:    now.Add(5 * time.Hour),
	}, now)
	refreshedSnapshot := testUsageSnapshot()
	refreshedSnapshot.Email = "fresh@example.com"
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: refreshedSnapshot}}}
	h.CodexUsageFetcher = fetcher

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", true)

	require.Equal(t, "fresh", got.Status)
	assert.Equal(t, "fresh@example.com", got.Snapshot.Email)
	require.Len(t, fetcher.calls, 1)
}

func TestOpenAISubscriptionCodexUsage_SharedCacheHitRequiresValidCredential(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	m := newMockStore()
	m.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: "api-key", CredentialType: "api_key"}, nil
	}
	h := &Handlers{DB: m, Config: &config.ProxyConfig{}, Cache: newRecordingCache()}
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "api-key",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	}, now)
	h.openAISubscriptionNow = func() time.Time { return now }

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "api-key", false)

	assert.Equal(t, "credential_wrong_type", got.LastErrorReason)
	assert.Equal(t, "unavailable", got.Status)
}

func TestOpenAISubscriptionCodexUsage_EmptySnapshotDoesNotOverwriteSharedCache(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	shared := newRecordingCache()
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Cache = shared
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-refresh",
		Status:       "fresh",
		Snapshot:     testUsageSnapshot(),
		FetchedAt:    now,
		ExpiresAt:    now.Add(5 * time.Hour),
	}, now)
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{{snapshot: chatgptcodex.UsageSnapshot{}}}}

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", true)

	assert.Equal(t, "empty_snapshot", got.LastErrorReason)
	assert.Equal(t, 1, shared.setCount())
}

func TestOpenAISubscriptionCodexUsage_EmptyBackoffDoesNotHideFirstPageData(t *testing.T) {
	now := time.Date(2026, 5, 13, 2, 30, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: http.StatusTooManyRequests, Reason: "rate_limited"}},
		{snapshot: testUsageSnapshot()},
	}}
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageBackoffBase = time.Minute
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }

	first := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)
	second := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "cred-refresh", false)

	require.Equal(t, "backoff", first.Status)
	assert.Empty(t, first.Snapshot.Email)
	require.Equal(t, "fresh", second.Status)
	assert.Equal(t, "operator@example.com", second.Snapshot.Email)
	require.Len(t, fetcher.calls, 2)
}

func TestOpenAISubscriptionCodexUsage_WrongTypeIsSafe(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: "api-key", CredentialType: "api_key"}, nil
	}
	h := &Handlers{DB: m, Config: &config.ProxyConfig{}}

	got := h.OpenAISubscriptionCodexUsageSnapshot(context.Background(), "api-key", false)

	assert.Equal(t, "credential_wrong_type", got.LastErrorReason)
	assert.Equal(t, "unavailable", got.Status)
}
