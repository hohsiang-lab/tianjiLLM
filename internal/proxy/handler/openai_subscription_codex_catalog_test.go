package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCodexCatalogFetcher struct {
	mu        sync.Mutex
	calls     []chatgptcodex.CatalogRequest
	responses []fakeCodexCatalogResponse
}

type fakeCodexCatalogResponse struct {
	catalog chatgptcodex.Catalog
	err     error
}

type sharedRecordingCache struct {
	*recordingCache
}

type blockingCatalogLockCache struct {
	*recordingCache
	lockStarted chan struct{}
	allowLock   chan struct{}
	lockOnce    sync.Once
}

func (c *blockingCatalogLockCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	wait := false
	c.lockOnce.Do(func() {
		close(c.lockStarted)
		wait = true
	})
	if wait {
		<-c.allowLock
	}
	return false, nil
}

func (c *sharedRecordingCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	return c.Get(ctx, key)
}

type flakySharedRecordingCache struct {
	*sharedRecordingCache
	mu       sync.Mutex
	failures int
}

type failingCatalogWriteCache struct {
	*recordingCache
}

func (c *failingCatalogWriteCache) CompareAndSet(context.Context, string, []byte, []byte, time.Duration) (bool, error) {
	return false, nil
}

func (c *flakySharedRecordingCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures > 0 {
		c.failures--
		return nil, errors.New("shared cache unavailable")
	}
	return c.sharedRecordingCache.GetShared(ctx, key)
}

type invalidationRaceCache struct {
	*sharedRecordingCache
	mu                     sync.Mutex
	failures               int
	invalidationSetStarted chan struct{}
	allowInvalidationSet   chan struct{}
	refreshLockBlocked     chan struct{}
	refreshLockBlockedOnce sync.Once
}

func (c *invalidationRaceCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures > 0 {
		c.failures--
		return nil, errors.New("shared cache unavailable")
	}
	return c.sharedRecordingCache.GetShared(ctx, key)
}

func (c *invalidationRaceCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if bytes.Contains(value, []byte(`"last_error_reason":"invalidated"`)) {
		select {
		case <-c.invalidationSetStarted:
		default:
			close(c.invalidationSetStarted)
		}
		<-c.allowInvalidationSet
	}
	return c.recordingCache.Set(ctx, key, value, ttl)
}

func (c *invalidationRaceCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	acquired, err := c.recordingCache.AcquireLock(ctx, key, token, ttl)
	if !acquired && c.refreshLockBlocked != nil {
		c.refreshLockBlockedOnce.Do(func() { close(c.refreshLockBlocked) })
	}
	return acquired, err
}

func (f *fakeCodexCatalogFetcher) FetchCatalog(_ context.Context, request chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, request)
	if len(f.responses) == 0 {
		return chatgptcodex.Catalog{}, errors.New("unexpected catalog request")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.catalog, response.err
}

func (f *fakeCodexCatalogFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newCodexCatalogHarness(t *testing.T, fixtures map[string]openAISubscriptionTestCredential) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, fixtures)
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	return h
}

func TestOpenAISubscriptionCodexCatalogUsesFreshCacheWithoutRefetch(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}
	h.CodexCatalogFetcher = fetcher

	first := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	second := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	require.Empty(t, first.LastErrorReason)
	require.Empty(t, second.LastErrorReason)
	assert.Equal(t, 1, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogDoesNotAdoptUnchangedExpiredCacheAfterRefreshFailure(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.openAISubscriptionNow = func() time.Time { return now }
	writeCache := &failingCatalogWriteCache{recordingCache: newRecordingCache()}
	h.Cache = writeCache
	catalog := loadTestCodexCatalog(t)
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      catalog,
		FetchedAt:    now.Add(-time.Hour),
		ExpiresAt:    now.Add(-time.Minute),
	})
	oldBody, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Catalog:      catalog,
		FetchedAt:    now.Add(-time.Hour),
		ExpiresAt:    now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, writeCache.Set(context.Background(), codexCatalogCacheKey("cred-a"), oldBody, defaultCodexCatalogCacheTTL))
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{
		err: errors.New("upstream unavailable"),
	}}}

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	require.True(t, result.Degraded)
	model, ok := h.codexCatalogForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})
	assert.False(t, ok)
	assert.Empty(t, model.Slug)
}

func TestOpenAISubscriptionCodexCatalogForParamsPreservesResponsesLite(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	model, ok := h.codexCatalogForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})
	require.True(t, ok)
	require.True(t, model.UseResponsesLite)
}

func TestOpenAISubscriptionCodexCatalogErrorReasonDoesNotParseFreeFormMessages(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "operator disabled text", message: "upstream said operator_disabled but request may retry"},
		{name: "auth failure text", message: "auth_failed_after_refresh was mentioned by upstream"},
		{name: "reconnect text", message: "refresh_token_invalidated appears in a diagnostic payload"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &OpenAISubscriptionCredentialError{
				Code:    OpenAISubscriptionCredentialRefreshErr,
				Message: tt.message,
			}
			reason, permanent := openAISubscriptionCodexCatalogErrorClassification(err)
			assert.Equal(t, "refresh_failed", reason)
			assert.False(t, permanent)
		})
	}
}

func TestOpenAISubscriptionCodexCatalogClassifiesKnownPermanentErrors(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantReason    string
		wantPermanent bool
	}{
		{
			name:          "credential disabled",
			err:           &OpenAISubscriptionCredentialError{Code: OpenAISubscriptionCredentialDisabled},
			wantReason:    string(OpenAISubscriptionCredentialDisabled),
			wantPermanent: true,
		},
		{
			name:          "refresh token invalidated",
			err:           &OpenAISubscriptionCredentialError{Code: OpenAISubscriptionCredentialReconnect},
			wantReason:    string(OpenAISubscriptionCredentialReconnect),
			wantPermanent: true,
		},
		{
			name:          "authentication failed after refresh",
			err:           &OpenAISubscriptionCredentialError{Code: OpenAISubscriptionCredentialAuthErr},
			wantReason:    string(OpenAISubscriptionCredentialAuthErr),
			wantPermanent: true,
		},
		{
			name: "operator disabled metadata",
			err: &OpenAISubscriptionCredentialError{
				Code:                OpenAISubscriptionCredentialRefreshErr,
				NonSelectableReason: "operator_disabled",
			},
			wantReason:    "operator_disabled",
			wantPermanent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, permanent := openAISubscriptionCodexCatalogErrorClassification(tt.err)
			assert.Equal(t, tt.wantReason, reason)
			assert.Equal(t, tt.wantPermanent, permanent)
		})
	}
}

func TestOpenAISubscriptionCodexCatalogClassifiesTemporaryErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "refresh failure", err: &OpenAISubscriptionCredentialError{Code: OpenAISubscriptionCredentialRefreshErr}},
		{name: "rate limited", err: &OpenAISubscriptionCredentialError{Code: OpenAISubscriptionCredentialRateLimited}},
		{name: "quota exceeded", err: errors.New("quota exceeded")},
		{name: "upstream unavailable", err: errors.New("upstream unavailable")},
		{
			name: "unknown metadata reason",
			err: &OpenAISubscriptionCredentialError{
				Code:                OpenAISubscriptionCredentialRefreshErr,
				NonSelectableReason: "operator disabled by an administrator",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, permanent := openAISubscriptionCodexCatalogErrorClassification(tt.err)
			assert.Equal(t, "refresh_failed", reason)
			assert.False(t, permanent)
		})
	}
}

func TestCodexCatalogEligibilityUsesExplicitPermanentMarker(t *testing.T) {
	result := OpenAISubscriptionCodexCatalogResult{}
	assert.False(t, result.PermanentlyUnavailable)
}

func TestOpenAISubscriptionCodexCatalogMarksKnownPermanentCredentialStates(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		disabledReason string
		wantReason     string
	}{
		{
			name:       "credential disabled",
			status:     "disabled",
			wantReason: string(OpenAISubscriptionCredentialDisabled),
		},
		{
			name:       "credential disabled with surrounding whitespace",
			status:     " disabled ",
			wantReason: string(OpenAISubscriptionCredentialDisabled),
		},
		{
			name:           "refresh token invalidated",
			status:         "refresh_failed",
			disabledReason: string(OpenAISubscriptionCredentialReconnect),
			wantReason:     string(OpenAISubscriptionCredentialReconnect),
		},
		{
			name:           "authentication failed after refresh",
			status:         "disabled",
			disabledReason: string(OpenAISubscriptionCredentialAuthErr),
			wantReason:     string(OpenAISubscriptionCredentialAuthErr),
		},
		{
			name:           "operator disabled",
			status:         "refresh_failed",
			disabledReason: "operator_disabled",
			wantReason:     "operator_disabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
				"cred-a": {
					AccessToken:    "access-a",
					AccountID:      "acct-a",
					Status:         tt.status,
					DisabledReason: tt.disabledReason,
				},
			})

			result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

			require.True(t, result.Degraded)
			require.True(t, result.PermanentlyUnavailable)
			assert.Equal(t, tt.wantReason, result.LastErrorReason)
		})
	}
}

func TestOpenAISubscriptionCodexCatalogKeepsTemporaryCredentialStatesEligible(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "generic refresh failure", err: errors.New("upstream unavailable")},
		{
			name: "rate limited",
			err: &OpenAISubscriptionCredentialError{
				Code:    OpenAISubscriptionCredentialRateLimited,
				Message: "quota temporarily unavailable",
			},
		},
		{
			name: "free form permanent-looking message",
			err: &OpenAISubscriptionCredentialError{
				Code:    OpenAISubscriptionCredentialRefreshErr,
				Message: "refresh_token_invalidated appears in an upstream diagnostic",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
				"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
			})
			h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{err: tt.err}}}

			result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

			require.True(t, result.Degraded)
			assert.False(t, result.PermanentlyUnavailable)
			assert.Equal(t, "refresh_failed", result.LastErrorReason)
		})
	}
}

func TestOpenAISubscriptionCodexCatalogClearsPermanentMarkerAfterTemporaryFailure(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID:           "cred-a",
		Catalog:                loadTestCodexCatalog(t),
		Degraded:               true,
		PermanentlyUnavailable: true,
		LastErrorReason:        string(OpenAISubscriptionCredentialReconnect),
		LastRefreshAttemptAt:   now.Add(-2 * time.Minute),
		FetchedAt:              now.Add(-time.Hour),
		ExpiresAt:              now.Add(-time.Minute),
	})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{err: errors.New("upstream unavailable")}}}

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	require.True(t, result.Degraded)
	assert.False(t, result.PermanentlyUnavailable)
	assert.Equal(t, "refresh_failed", result.LastErrorReason)
}

func TestCodexCatalogWildcardEligibilityUsesExplicitPermanentMarker(t *testing.T) {
	params := config.TianjiParams{
		Model:                           "openai/*",
		OpenAISubscriptionCredentialIDs: []string{"cred-active", "cred-retry"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	active := loadTestCodexCatalog(t)

	model, ok := (&Handlers{}).codexCatalogForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		if credentialID == "cred-retry" {
			return OpenAISubscriptionCodexCatalogResult{
				CredentialID:           credentialID,
				Degraded:               true,
				PermanentlyUnavailable: true,
				LastErrorReason:        string(OpenAISubscriptionCredentialReconnect),
			}
		}
		return OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID, Catalog: active}
	})

	require.True(t, ok)
	assert.True(t, model.UseResponsesLite)
}

func TestCodexCatalogForParamsRejectsTemporaryDegradedCatalog(t *testing.T) {
	params := config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-active", "cred-temporary"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	catalog := loadTestCodexCatalog(t)

	model, ok := (&Handlers{}).codexCatalogForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		if credentialID == "cred-temporary" {
			return OpenAISubscriptionCodexCatalogResult{
				CredentialID:    credentialID,
				Catalog:         catalog,
				Degraded:        true,
				LastErrorReason: "refresh_failed",
			}
		}
		return OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID, Catalog: catalog}
	})

	assert.False(t, ok)
	assert.Empty(t, model)
}

func TestCodexCatalogWildcardRejectsTemporaryDegradedCatalog(t *testing.T) {
	params := config.TianjiParams{
		Model:                           "openai/*",
		OpenAISubscriptionCredentialIDs: []string{"cred-active", "cred-temporary"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	catalog := loadTestCodexCatalog(t)

	model, ok := (&Handlers{}).codexCatalogForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		if credentialID == "cred-temporary" {
			return OpenAISubscriptionCodexCatalogResult{
				CredentialID:    credentialID,
				Catalog:         catalog,
				Degraded:        true,
				LastErrorReason: "refresh_failed",
			}
		}
		return OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID, Catalog: catalog}
	})

	assert.False(t, ok)
	assert.Empty(t, model)
}

func TestOpenAISubscriptionCodexCatalogUsageRefreshPreservesInvalidatedReason(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "expired-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(-time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   401,
		Response: map[string]any{
			"error":             "refresh_token_invalidated",
			"error_description": "Your session has ended. Please log in again.",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}
	h.CodexCatalogFetcher = fetcher

	loaded, err := h.loadOpenAISubscriptionCredential(context.Background(), "cred-refresh")
	require.NoError(t, err)
	_, refreshErr := h.refreshOpenAISubscriptionCredential(context.Background(), loaded, false)
	require.Error(t, refreshErr)
	assert.Equal(t, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionErrorCode(refreshErr))
	var typed *OpenAISubscriptionCredentialError
	require.ErrorAs(t, refreshErr, &typed)
	assert.Equal(t, string(OpenAISubscriptionCredentialReconnect), typed.NonSelectableReason)

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-refresh")

	require.True(t, result.PermanentlyUnavailable)
	assert.Equal(t, string(OpenAISubscriptionCredentialReconnect), result.LastErrorReason)
	assert.Zero(t, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogCachePersistsPermanentMarker(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	disk, err := cache.NewDiskCache(t.TempDir())
	require.NoError(t, err)
	h.Cache = disk
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	result := OpenAISubscriptionCodexCatalogResult{
		CredentialID:           "cred-a",
		Degraded:               true,
		PermanentlyUnavailable: true,
		LastErrorReason:        string(OpenAISubscriptionCredentialReconnect),
		LastRefreshAttemptAt:   now,
	}

	require.True(t, h.persistOpenAISubscriptionCodexCatalogResult(context.Background(), result, nil))
	restored, ok := h.getCachedOpenAISubscriptionCodexCatalogResult(context.Background(), "cred-a")
	require.True(t, ok)
	assert.True(t, restored.PermanentlyUnavailable)
	assert.Equal(t, result.LastErrorReason, restored.LastErrorReason)
}

func TestOpenAISubscriptionCodexCatalogCacheOmitsFalsePermanentMarker(t *testing.T) {
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Degraded:     true,
	})
	require.NoError(t, err)
	assert.NotContains(t, string(body), "permanently_unavailable")
}

func TestOpenAISubscriptionCodexCatalogResultOmitsFalsePermanentMarker(t *testing.T) {
	body, err := json.Marshal(OpenAISubscriptionCodexCatalogResult{})
	require.NoError(t, err)
	assert.NotContains(t, string(body), "permanently_unavailable")

	body, err = json.Marshal(OpenAISubscriptionCodexCatalogResult{PermanentlyUnavailable: true})
	require.NoError(t, err)
	assert.Contains(t, string(body), `"permanently_unavailable":true`)
}

func TestChatGPTCodexTransportForParamsUsesCachedResponsesLite(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "gpt-5.6-sol", []resolvedOpenAISubscriptionCredential{{CredentialID: "cred-a"}})
	require.True(t, transport.UseResponsesLite)
}

func TestChatGPTCodexTransportForParamsRefreshesExpiredLiteCache(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
		FetchedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	})
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{
		catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug: "gpt-5.6-sol",
		}}},
	}}}
	h.CodexCatalogFetcher = fetcher

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "gpt-5.6-sol", []resolvedOpenAISubscriptionCredential{{CredentialID: "cred-a"}})

	assert.False(t, transport.UseResponsesLite)
	assert.Equal(t, 1, fetcher.callCount())
}

func TestChatGPTCodexTransportForParamsRefreshesCatalogsConcurrently(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	started := make(chan string, 2)
	release := make(chan struct{})
	h.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, request chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		started <- request.AccountID
		<-release
		return chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol"}}}, nil
	})

	done := make(chan chatgptcodex.Transport, 1)
	go func() {
		done <- h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
			Model:                           "gpt-5.6-sol",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		}, "gpt-5.6-sol", []resolvedOpenAISubscriptionCredential{
			{CredentialID: "cred-a"},
			{CredentialID: "cred-b"},
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("catalog refresh did not start")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("catalog refreshes are serialized")
	}
	close(release)
	transport := <-done
	assert.False(t, transport.UseResponsesLite)
}

func TestChatGPTCodexTransportForParamsRefreshesMissingResponsesLiteCatalog(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{
		catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
	}}}

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "gpt-5.6-sol", []resolvedOpenAISubscriptionCredential{{CredentialID: "cred-a"}})
	require.True(t, transport.UseResponsesLite)
}

func TestChatGPTCodexTransportForParamsUsesActualLiteCandidatesWhenCatalogMetadataDiffers(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	now := time.Now().UTC()
	first := chatgptcodex.CatalogModel{Slug: "gpt-5.6-sol", UseResponsesLite: true, InputModalities: []string{"text", "image"}}
	second := first
	second.InputModalities = []string{"text"}
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{first}},
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-b",
		Catalog:      chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{second}},
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "missing-configured-credential"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "gpt-5.6-sol", []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a"},
		{CredentialID: "cred-b"},
	})
	require.True(t, transport.UseResponsesLite)
}

func TestChatGPTCodexTransportForParamsFallsBackToConfiguredModelForAlias(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "sol-alias", []resolvedOpenAISubscriptionCredential{{CredentialID: "cred-a"}})
	require.True(t, transport.UseResponsesLite)
}

func TestChatGPTCodexTransportForParamsDoesNotUseWildcardForConcreteMiss(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Now().UTC()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug:             "gpt-5.6-sol",
			UseResponsesLite: true,
		}}},
		FetchedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	transport := h.chatGPTCodexBackendTransportForParams(context.Background(), config.TianjiParams{
		Model:                           "openai/*",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, "openai/gpt-5.5", []resolvedOpenAISubscriptionCredential{{CredentialID: "cred-a"}})

	assert.False(t, transport.UseResponsesLite)
}

func TestOpenAISubscriptionCodexCatalogSeparatesCredentialEntitlements(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{
		{catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-a"}}}},
		{catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-b"}}}},
	}}
	h.CodexCatalogFetcher = fetcher

	first := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	second := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-b")
	_, firstHasB := first.Catalog.Model("gpt-b")
	_, secondHasA := second.Catalog.Model("gpt-a")
	assert.False(t, firstHasB)
	assert.False(t, secondHasA)
	assert.Equal(t, 2, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogInvalidatesOnCredentialRefresh(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now, ExpiresAt: now.Add(time.Hour)})

	h.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	_, ok := h.CodexCatalogCache.get("cred-a")
	assert.False(t, ok)
}

func TestOpenAISubscriptionCodexCatalogCoalescesConcurrentRefreshes(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	started := make(chan struct{})
	release := make(chan struct{})
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}
	h.CodexCatalogFetcher = catalogFetcherFunc(func(ctx context.Context, request chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		fetcher.mu.Lock()
		fetcher.calls = append(fetcher.calls, request)
		fetcher.mu.Unlock()
		close(started)
		<-release
		return loadTestCodexCatalog(t), nil
	})

	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() { defer wait.Done(); h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a") }()
	}
	<-started
	close(release)
	wait.Wait()
	assert.Equal(t, 1, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogPublishesBeforeSingleflightRelease(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	cache := &blockingCatalogLockCache{
		recordingCache: newRecordingCache(),
		lockStarted:    make(chan struct{}),
		allowLock:      make(chan struct{}),
	}
	h.Cache = cache
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}
	h.CodexCatalogFetcher = fetcher

	firstResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		firstResult <- h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-cache.lockStarted

	second := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	close(cache.allowLock)
	first := <-firstResult

	require.Empty(t, first.LastErrorReason)
	require.Empty(t, second.LastErrorReason)
	require.NotEmpty(t, first.Catalog.Models)
	require.NotEmpty(t, second.Catalog.Models)
	assert.Equal(t, 1, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogInitializesNilCacheConcurrently(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.CodexCatalogCache = nil
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}

	const callers = 32
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			<-start
			result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
			require.Empty(t, result.LastErrorReason)
		}()
	}
	close(start)
	wait.Wait()
	require.NotNil(t, h.CodexCatalogCache)
}

type catalogFetcherFunc func(context.Context, chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error)

func (f catalogFetcherFunc) FetchCatalog(ctx context.Context, request chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
	return f(ctx, request)
}

func TestOpenAISubscriptionCodexCatalogServesLastKnownGoodOnRefreshFailure(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{err: errors.New("upstream unavailable")}}}

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	assert.True(t, result.Degraded)
	assert.Equal(t, "refresh_failed", result.LastErrorReason)
	_, ok := result.Catalog.Model("gpt-5.6-sol")
	assert.True(t, ok)
}

func TestOpenAISubscriptionCodexCatalogBacksOffAfterRefreshFailure(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	fetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{
		{err: errors.New("upstream unavailable")},
		{err: errors.New("unexpected retry")},
	}}
	h.CodexCatalogFetcher = fetcher

	first := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	second := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	assert.True(t, first.Degraded)
	assert.True(t, second.Degraded)
	assert.Equal(t, 1, fetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogPersistsDegradedBackoff(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	persistent := newRecordingCache()

	first := newCodexCatalogHarness(t, fixtures)
	first.Cache = persistent
	first.openAISubscriptionNow = func() time.Time { return now }
	first.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		FetchedAt:    now.Add(-time.Hour),
		ExpiresAt:    now.Add(-time.Minute),
	})
	firstFetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{err: errors.New("upstream unavailable")}}}
	first.CodexCatalogFetcher = firstFetcher
	result := first.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	require.True(t, result.Degraded)

	second := newCodexCatalogHarness(t, fixtures)
	second.Cache = persistent
	second.openAISubscriptionNow = func() time.Time { return now }
	secondFetcher := &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{err: errors.New("unexpected retry")}}}
	second.CodexCatalogFetcher = secondFetcher
	result = second.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	assert.True(t, result.Degraded)
	assert.Equal(t, "refresh_failed", result.LastErrorReason)
	assert.Equal(t, 0, secondFetcher.callCount())
	assert.Equal(t, 1, firstFetcher.callCount())
}

func TestOpenAISubscriptionCodexCatalogDoesNotOverwriteNewerSharedSuccess(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
	old := OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		FetchedAt:    now.Add(-2 * time.Hour),
		ExpiresAt:    now.Add(-time.Hour),
	}
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: old.CredentialID,
		Catalog:      old.Catalog,
		FetchedAt:    old.FetchedAt,
		ExpiresAt:    old.ExpiresAt,
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), body, defaultCodexCatalogCacheTTL))

	failureStarted := make(chan struct{})
	releaseFailure := make(chan struct{})
	failure := newCodexCatalogHarness(t, fixtures)
	failure.Cache = shared
	failure.openAISubscriptionNow = func() time.Time { return now }
	failure.CodexCatalogFetcher = catalogFetcherFunc(func(ctx context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(failureStarted)
		<-releaseFailure
		return chatgptcodex.Catalog{}, errors.New("upstream unavailable")
	})

	successStarted := make(chan struct{})
	releaseSuccess := make(chan struct{})
	success := newCodexCatalogHarness(t, fixtures)
	success.Cache = shared
	success.openAISubscriptionNow = func() time.Time { return now }
	success.CodexCatalogFetcher = catalogFetcherFunc(func(ctx context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(successStarted)
		<-releaseSuccess
		return chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol", UseResponsesLite: true}}}, nil
	})

	failureResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		failureResult <- failure.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-failureStarted

	successResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		successResult <- success.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-successStarted

	close(releaseSuccess)
	require.False(t, (<-successResult).Degraded)
	close(releaseFailure)
	require.False(t, (<-failureResult).Degraded)

	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.False(t, persisted.Degraded)
	assert.Equal(t, now, persisted.FetchedAt)
	assert.True(t, persisted.Catalog.Models[0].UseResponsesLite)
}

func TestOpenAISubscriptionCodexCatalogPersistsLaterSuccessAfterSharedFailure(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
	old := OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		FetchedAt:    now.Add(-2 * time.Hour),
		ExpiresAt:    now.Add(-time.Hour),
	}
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: old.CredentialID,
		Catalog:      old.Catalog,
		FetchedAt:    old.FetchedAt,
		ExpiresAt:    old.ExpiresAt,
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), body, defaultCodexCatalogCacheTTL))

	failureStarted := make(chan struct{})
	releaseFailure := make(chan struct{})
	failure := newCodexCatalogHarness(t, fixtures)
	failure.Cache = shared
	failure.openAISubscriptionNow = func() time.Time { return now }
	failure.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(failureStarted)
		<-releaseFailure
		return chatgptcodex.Catalog{}, errors.New("upstream unavailable")
	})

	successStarted := make(chan struct{})
	releaseSuccess := make(chan struct{})
	success := newCodexCatalogHarness(t, fixtures)
	success.Cache = shared
	success.openAISubscriptionNow = func() time.Time { return now }
	success.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(successStarted)
		<-releaseSuccess
		return chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol", UseResponsesLite: true}}}, nil
	})

	failureResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		failureResult <- failure.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-failureStarted

	successResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		successResult <- success.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-successStarted

	close(releaseFailure)
	require.True(t, (<-failureResult).Degraded)
	close(releaseSuccess)
	require.False(t, (<-successResult).Degraded)

	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.False(t, persisted.Degraded)
	assert.Equal(t, now, persisted.FetchedAt)
	assert.True(t, persisted.Catalog.Models[0].UseResponsesLite)
}

func TestOpenAISubscriptionCodexCatalogBoundsRefreshAndServesLastKnownGood(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogRefreshTimeout = 20 * time.Millisecond
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-a", Catalog: loadTestCodexCatalog(t), FetchedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
	h.CodexCatalogFetcher = catalogFetcherFunc(func(ctx context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), h.CodexCatalogRefreshTimeout)
		<-ctx.Done()
		return chatgptcodex.Catalog{}, ctx.Err()
	})

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	assert.True(t, result.Degraded)
	assert.Equal(t, "refresh_failed", result.LastErrorReason)
	_, ok := result.Catalog.Model("gpt-5.6-sol")
	assert.True(t, ok)
}

func TestOpenAISubscriptionCodexCatalogRejectsEmptyRefresh(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h := newCodexCatalogHarness(t, fixtures)
	h.Cache = newRecordingCache()
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		FetchedAt:    now.Add(-2 * time.Hour),
		ExpiresAt:    now.Add(-time.Hour),
	})
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{
		{catalog: chatgptcodex.Catalog{}},
	}}

	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	assert.True(t, result.Degraded)
	_, ok := result.Catalog.Model("gpt-5.6-sol")
	assert.True(t, ok)
}

func TestOpenAISubscriptionCodexCatalogInvalidationFencesInFlightRefresh(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h := newCodexCatalogHarness(t, fixtures)
	shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
	h.Cache = shared
	h.openAISubscriptionNow = func() time.Time { return now }
	old := OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		FetchedAt:    now.Add(-2 * time.Hour),
		ExpiresAt:    now.Add(-time.Hour),
	}
	oldBody, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: old.CredentialID,
		Catalog:      old.Catalog,
		FetchedAt:    old.FetchedAt,
		ExpiresAt:    old.ExpiresAt,
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), oldBody, defaultCodexCatalogCacheTTL))
	h.CodexCatalogCache.put(old)

	started := make(chan struct{})
	release := make(chan struct{})
	h.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(started)
		<-release
		return loadTestCodexCatalog(t), nil
	})

	resultCh := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		resultCh <- h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-started

	h.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	close(release)
	result := <-resultCh

	assert.True(t, result.Degraded)
	assert.Equal(t, "invalidated", result.LastErrorReason)
	_, ok := result.Catalog.Model("gpt-5.6-sol")
	assert.False(t, ok)
	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.True(t, persisted.Degraded)
	assert.Equal(t, "invalidated", persisted.LastErrorReason)
}

func TestOpenAISubscriptionCodexCatalogDoesNotPersistAfterFailedInvalidationRead(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h := newCodexCatalogHarness(t, fixtures)
	shared := &flakySharedRecordingCache{
		sharedRecordingCache: &sharedRecordingCache{recordingCache: newRecordingCache()},
		failures:             2,
	}
	h.Cache = shared
	h.openAISubscriptionNow = func() time.Time { return now }

	started := make(chan struct{})
	release := make(chan struct{})
	h.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(started)
		<-release
		return loadTestCodexCatalog(t), nil
	})

	resultCh := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		resultCh <- h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-started

	h.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	close(release)
	result := <-resultCh

	assert.True(t, result.Degraded)
	assert.Equal(t, "invalidated", result.LastErrorReason)
	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.True(t, persisted.Degraded)
	assert.Equal(t, "invalidated", persisted.LastErrorReason)
}

func TestOpenAISubscriptionCodexCatalogInvalidationWinsAbsentKeyRace(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	shared := &invalidationRaceCache{
		sharedRecordingCache:   &sharedRecordingCache{recordingCache: newRecordingCache()},
		invalidationSetStarted: make(chan struct{}),
		allowInvalidationSet:   make(chan struct{}),
		refreshLockBlocked:     make(chan struct{}),
	}

	refresh := newCodexCatalogHarness(t, fixtures)
	refresh.Cache = shared
	refresh.openAISubscriptionNow = func() time.Time { return now }
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	refresh.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(refreshStarted)
		<-releaseRefresh
		return chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol"}}}, nil
	})

	invalidate := newCodexCatalogHarness(t, fixtures)
	invalidate.Cache = shared
	invalidate.openAISubscriptionNow = func() time.Time { return now }

	refreshResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		refreshResult <- refresh.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-refreshStarted
	shared.mu.Lock()
	shared.failures = 1
	shared.mu.Unlock()

	invalidationDone := make(chan struct{})
	go func() {
		invalidate.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
		close(invalidationDone)
	}()
	<-shared.invalidationSetStarted

	close(releaseRefresh)
	<-shared.refreshLockBlocked
	close(shared.allowInvalidationSet)
	refreshResolved := <-refreshResult
	require.True(t, refreshResolved.Degraded)
	<-invalidationDone

	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.True(t, persisted.Degraded)
	assert.Equal(t, "invalidated", persisted.LastErrorReason)
}

func TestOpenAISubscriptionCodexCatalogInvalidationFencesExistingKeyAfterReadError(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	shared := &invalidationRaceCache{
		sharedRecordingCache:   &sharedRecordingCache{recordingCache: newRecordingCache()},
		invalidationSetStarted: make(chan struct{}),
		allowInvalidationSet:   make(chan struct{}),
		refreshLockBlocked:     make(chan struct{}),
	}
	oldBody, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Catalog:      loadTestCodexCatalog(t),
		Generation:   0,
		FetchedAt:    now.Add(-time.Hour),
		ExpiresAt:    now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), oldBody, defaultCodexCatalogCacheTTL))

	refresh := newCodexCatalogHarness(t, fixtures)
	refresh.Cache = shared
	refresh.openAISubscriptionNow = func() time.Time { return now }
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	refresh.CodexCatalogFetcher = catalogFetcherFunc(func(_ context.Context, _ chatgptcodex.CatalogRequest) (chatgptcodex.Catalog, error) {
		close(refreshStarted)
		<-releaseRefresh
		return chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol"}}}, nil
	})

	invalidate := newCodexCatalogHarness(t, fixtures)
	invalidate.Cache = shared

	refreshResult := make(chan OpenAISubscriptionCodexCatalogResult, 1)
	go func() {
		refreshResult <- refresh.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	}()
	<-refreshStarted
	shared.mu.Lock()
	shared.failures = 1
	shared.mu.Unlock()

	invalidationDone := make(chan struct{})
	go func() {
		invalidate.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
		close(invalidationDone)
	}()
	<-shared.invalidationSetStarted

	close(releaseRefresh)
	<-shared.refreshLockBlocked
	close(shared.allowInvalidationSet)
	refreshResolved := <-refreshResult
	require.True(t, refreshResolved.Degraded)
	<-invalidationDone

	raw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.True(t, persisted.Degraded)
	assert.Equal(t, "invalidated", persisted.LastErrorReason)
}

func TestOpenAISubscriptionCodexCatalogStaleRefreshCannotReplaceLaterResultAfterInvalidation(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	shared := &flakySharedRecordingCache{
		sharedRecordingCache: &sharedRecordingCache{recordingCache: newRecordingCache()},
		failures:             1,
	}
	old := openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
			Slug: "gpt-old",
		}}},
		Generation: 5,
		FetchedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
	}
	oldBody, err := json.Marshal(old)
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), oldBody, defaultCodexCatalogCacheTTL))

	stale := newCodexCatalogHarness(t, fixtures)
	stale.Cache = shared
	stale.openAISubscriptionNow = func() time.Time { return now }
	stale.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      old.Catalog,
		Generation:   old.Generation,
		FetchedAt:    old.FetchedAt,
		ExpiresAt:    old.ExpiresAt,
	})

	invalidate := newCodexCatalogHarness(t, fixtures)
	invalidate.Cache = shared
	invalidate.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	markerRaw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var marker openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(markerRaw, &marker))
	require.Equal(t, "invalidated", marker.LastErrorReason)
	require.NotEmpty(t, marker.InvalidationToken)

	followup := newCodexCatalogHarness(t, fixtures)
	followup.Cache = shared
	followupResult := OpenAISubscriptionCodexCatalogResult{
		CredentialID:      "cred-a",
		Catalog:           chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-followup"}}},
		Generation:        marker.Generation,
		InvalidationToken: marker.InvalidationToken,
		FetchedAt:         now,
		ExpiresAt:         now.Add(time.Hour),
	}
	require.True(t, followup.persistOpenAISubscriptionCodexCatalogResult(context.Background(), followupResult, markerRaw))

	require.False(t, stale.persistOpenAISubscriptionCodexCatalogResult(context.Background(), OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      old.Catalog,
		Generation:   old.Generation,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	}, oldBody))

	persistedRaw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(persistedRaw, &persisted))
	assert.Equal(t, marker.InvalidationToken, persisted.InvalidationToken)
	assert.Equal(t, "gpt-followup", persisted.Catalog.Models[0].Slug)
}

func TestOpenAISubscriptionCodexCatalogPersistsToDiskCache(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}})
	disk, err := cache.NewDiskCache(t.TempDir())
	require.NoError(t, err)
	h.Cache = disk
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	result := OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-5.6-sol"}}},
		Generation:   1,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	}

	require.True(t, h.persistOpenAISubscriptionCodexCatalogResult(context.Background(), result, nil))

	raw, err := disk.Get(context.Background(), codexCatalogCacheKey("cred-a"))
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	var persisted openAISubscriptionCodexCatalogCacheValue
	require.NoError(t, json.Unmarshal(raw, &persisted))
	assert.Equal(t, result.Catalog.Models[0].Slug, persisted.Catalog.Models[0].Slug)
}

func TestOpenAISubscriptionCodexCatalogPreservesRemoteInvalidationLineageAcrossLocalGeneration(t *testing.T) {
	tests := []struct {
		name          string
		response      fakeCodexCatalogResponse
		wantDegraded  bool
		wantModelSlug string
	}{
		{
			name:          "refresh failure",
			response:      fakeCodexCatalogResponse{err: errors.New("upstream unavailable")},
			wantDegraded:  true,
			wantModelSlug: "gpt-old",
		},
		{
			name: "refresh success",
			response: fakeCodexCatalogResponse{
				catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-fresh"}}},
			},
			wantModelSlug: "gpt-fresh",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
			now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
			shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
			marker := openAISubscriptionCodexCatalogCacheValue{
				CredentialID:      "cred-a",
				Generation:        1,
				Degraded:          true,
				LastErrorReason:   "invalidated",
				InvalidationToken: "remote-fence",
			}
			markerBody, err := json.Marshal(marker)
			require.NoError(t, err)
			require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), markerBody, defaultCodexCatalogCacheTTL))

			h := newCodexCatalogHarness(t, fixtures)
			h.Cache = shared
			h.openAISubscriptionNow = func() time.Time { return now }
			h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
				CredentialID: "cred-a",
				Catalog:      chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{Slug: "gpt-old"}}},
				Generation:   5,
				FetchedAt:    now.Add(-time.Hour),
				ExpiresAt:    now.Add(-time.Minute),
			})
			h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{test.response}}

			result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

			assert.Equal(t, test.wantDegraded, result.Degraded)
			assert.Equal(t, test.wantModelSlug, result.Catalog.Models[0].Slug)
			assert.Equal(t, marker.InvalidationToken, result.InvalidationToken)
			persistedRaw, err := shared.Get(context.Background(), codexCatalogCacheKey("cred-a"))
			require.NoError(t, err)
			var persisted openAISubscriptionCodexCatalogCacheValue
			require.NoError(t, json.Unmarshal(persistedRaw, &persisted))
			assert.Equal(t, marker.InvalidationToken, persisted.InvalidationToken)
		})
	}
}

func TestCodexResponsesLiteCachedReadsAuthoritativeSharedCatalog(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
	first := newCodexCatalogHarness(t, fixtures)
	first.Cache = shared
	second := newCodexCatalogHarness(t, fixtures)
	second.Cache = shared
	now := time.Now().UTC()
	catalog := chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
		Slug:             "gpt-5.6-sol",
		UseResponsesLite: true,
	}}}
	first.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{
		CredentialID: "cred-a",
		Catalog:      catalog,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Catalog:      catalog,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), body, defaultCodexCatalogCacheTTL))

	second.invalidateOpenAISubscriptionCodexCatalog(context.Background(), "cred-a")

	useResponsesLite, ok := first.codexResponsesLiteForParamsCached(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})

	assert.False(t, ok)
	assert.False(t, useResponsesLite)
}

func TestCodexResponsesLiteCachedRejectsOlderGeneration(t *testing.T) {
	fixtures := map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-a", AccountID: "acct-a"}}
	h := newCodexCatalogHarness(t, fixtures)
	shared := &sharedRecordingCache{recordingCache: newRecordingCache()}
	h.Cache = shared
	now := time.Now().UTC()
	catalog := chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
		Slug:             "gpt-5.6-sol",
		UseResponsesLite: true,
	}}}
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID: "cred-a",
		Catalog:      catalog,
		Generation:   0,
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, shared.Set(context.Background(), codexCatalogCacheKey("cred-a"), body, defaultCodexCatalogCacheTTL))
	h.CodexCatalogCache.Invalidate("cred-a")

	useResponsesLite, ok := h.codexResponsesLiteForParamsCached(context.Background(), config.TianjiParams{
		Model:                           "gpt-5.6-sol",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	})

	assert.False(t, ok)
	assert.False(t, useResponsesLite)
}

func TestOpenAISubscriptionCodexCatalogRedactsSecretsFromCacheAndLogs(t *testing.T) {
	h := newCodexCatalogHarness(t, map[string]openAISubscriptionTestCredential{"cred-a": {AccessToken: "access-secret", RefreshToken: "refresh-secret", AccountID: "acct-a"}})
	h.Cache = newRecordingCache()
	h.CodexCatalogFetcher = &fakeCodexCatalogFetcher{responses: []fakeCodexCatalogResponse{{catalog: loadTestCodexCatalog(t)}}}
	result := h.OpenAISubscriptionCodexCatalog(context.Background(), "cred-a")
	require.Empty(t, result.LastErrorReason)
	cache, ok := h.Cache.(*recordingCache)
	require.True(t, ok)
	require.Equal(t, 1, cache.setCount())
	require.NotContains(t, string(cache.values[codexCatalogCacheKey("cred-a")]), "access-secret")
	require.NotContains(t, string(cache.values[codexCatalogCacheKey("cred-a")]), "refresh-secret")
}

func TestCodexResponsesLiteWildcardRequiresMatchingCatalogModel(t *testing.T) {
	useResponsesLite, ok := codexResponsesLiteForParamsWithLookup(config.TianjiParams{
		Model:                           "openai/gpt-5.6-*",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}, func(string) OpenAISubscriptionCodexCatalogResult {
		return OpenAISubscriptionCodexCatalogResult{
			Catalog: chatgptcodex.Catalog{Models: []chatgptcodex.CatalogModel{{
				Slug:             "gpt-5.6-sol",
				UseResponsesLite: true,
			}}},
		}
	})

	require.True(t, ok)
	assert.True(t, useResponsesLite)
}
