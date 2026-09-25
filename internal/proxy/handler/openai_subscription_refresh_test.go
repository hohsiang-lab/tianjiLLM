package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const openAISubscriptionRefreshTestMasterKey = "test-master-key-32-bytes-long!!!"

type openAISubscriptionRefreshStore struct {
	masterKey string

	mu           sync.Mutex
	credential   db.CredentialTable
	valueWrites  int
	infoWrites   int
	infoWriteErr error
}

func newOpenAISubscriptionRefreshStore(t *testing.T, bundle OpenAISubscriptionTokenBundle, info OpenAISubscriptionCredentialInfo) *openAISubscriptionRefreshStore {
	t.Helper()
	infoJSON, err := json.Marshal(info)
	require.NoError(t, err)
	return &openAISubscriptionRefreshStore{
		masterKey: openAISubscriptionRefreshTestMasterKey,
		credential: db.CredentialTable{
			CredentialID:    "cred-refresh",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: encryptOpenAITokenBundle(t, bundle, openAISubscriptionRefreshTestMasterKey),
			CredentialInfo:  infoJSON,
		},
	}
}

func (s *openAISubscriptionRefreshStore) mock() *mockStore {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if credentialID != s.credential.CredentialID {
			return db.CredentialTable{}, errors.New("credential not found")
		}
		return s.credential, nil
	}
	m.updateCredentialValueAndInfoFn = func(_ context.Context, arg db.UpdateCredentialValueAndInfoParams) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.valueWrites++
		s.credential.CredentialValue = arg.CredentialValue
		s.credential.CredentialInfo = arg.CredentialInfo
		return nil
	}
	m.updateCredentialInfoFn = func(_ context.Context, arg db.UpdateCredentialInfoParams) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.infoWriteErr != nil {
			return s.infoWriteErr
		}
		s.infoWrites++
		s.credential.CredentialInfo = arg.CredentialInfo
		return nil
	}
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return []db.CredentialTable{s.credential}, nil
	}
	m.listOpenAISubscriptionRefreshCandidatesFn = func(_ context.Context) ([]db.CredentialTable, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return []db.CredentialTable{s.credential}, nil
	}
	return m
}

func (s *openAISubscriptionRefreshStore) decryptedBundle(t *testing.T) OpenAISubscriptionTokenBundle {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return decryptOpenAITokenBundle(t, s.credential.CredentialValue, s.masterKey)
}

func (s *openAISubscriptionRefreshStore) credentialInfo(t *testing.T) OpenAISubscriptionCredentialInfo {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var info OpenAISubscriptionCredentialInfo
	require.NoError(t, json.Unmarshal(s.credential.CredentialInfo, &info))
	return info
}

func (s *openAISubscriptionRefreshStore) writeCounts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.valueWrites, s.infoWrites
}

func newOpenAISubscriptionRefreshHarness(t *testing.T, now time.Time, store *openAISubscriptionRefreshStore, tokenServer *openaitest.OAuthServer) *Handlers {
	t.Helper()
	h := mockHandlers(store.mock())
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = store.masterKey
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		TokenURL: tokenServer.TokenURL(),
		ClientID: "app_test",
	}
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	h.openAISubscriptionNow = func() time.Time { return now }
	return h
}

func activeOpenAISubscriptionInfo() OpenAISubscriptionCredentialInfo {
	return OpenAISubscriptionCredentialInfo{
		Email:  "admin@example.com",
		Scopes: []string{"openid", "offline_access"},
		Status: "active",
	}
}

func TestResolveOpenAISubscriptionCredential_FreshTokenSkipsRefresh(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.NoError(t, err)
	assert.Equal(t, "fresh-access", resolved.BearerToken)
	assert.Empty(t, tokenServer.Requests())
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Zero(t, infoWrites)
}

func TestLoadOpenAISubscriptionCredentialFromRowTreatsWhitespaceDisabledAsPermanent(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	store.mu.Lock()
	store.credential.CredentialInfo = []byte(`{"status":" disabled "}`)
	row := store.credential
	store.mu.Unlock()

	_, err := h.loadOpenAISubscriptionCredentialFromRow(row)

	require.Error(t, err)
	assert.Equal(t, OpenAISubscriptionCredentialDisabled, openAISubscriptionErrorCode(err))
}

func TestForceRefreshOpenAISubscriptionCredential_BypassesFreshness(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "forced-access",
			"expires_in":   1800,
			"account_id":   "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	refreshed, err := h.forceRefreshOpenAISubscriptionCredential(context.Background(), "cred-refresh")

	require.NoError(t, err)
	assert.Equal(t, "forced-access", refreshed.BearerToken)
	require.Len(t, tokenServer.Requests(), 1)
	assert.Equal(t, "refresh_token", tokenServer.Requests()[0].Form.Get("grant_type"))
	assert.Equal(t, "forced-access", store.decryptedBundle(t).AccessToken)
	assert.Equal(t, "refresh-secret", store.decryptedBundle(t).RefreshToken)
}

func TestForceRefreshOpenAISubscriptionCredential_SharesSingleflightWithUsageRefresh(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "[REDACTED]",
		RefreshToken: "[REDACTED]",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "[REDACTED]",
		Response: map[string]any{
			"access_token":  "[REDACTED]",
			"refresh_token": "[REDACTED]",
			"expires_in":    1800,
			"account_id":    "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	transport := &blockingRefreshRoundTripper{
		base:    tokenServer.Client().Transport,
		entered: make(chan struct{}),
		second:  make(chan struct{}),
		release: make(chan struct{}),
	}
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: transport}

	forceDone := make(chan error, 1)
	go func() {
		_, err := h.forceRefreshOpenAISubscriptionCredential(context.Background(), "cred-refresh")
		forceDone <- err
	}()
	select {
	case <-transport.entered:
	case <-time.After(time.Second):
		t.Fatal("force refresh did not reach OAuth token endpoint")
	}

	usageDone := make(chan error, 1)
	go func() {
		_, err := h.refreshOpenAISubscriptionCredentialForUsageSnapshot(context.Background(), "cred-refresh")
		usageDone <- err
	}()
	select {
	case <-transport.second:
		t.Fatal("usage refresh started a concurrent OAuth refresh for the same credential")
	case <-time.After(100 * time.Millisecond):
	}

	close(transport.release)
	require.NoError(t, <-forceDone)
	require.NoError(t, <-usageDone)
	require.Equal(t, int32(1), transport.calls.Load())
}

type blockingRefreshRoundTripper struct {
	base    http.RoundTripper
	entered chan struct{}
	second  chan struct{}
	release chan struct{}
	calls   atomic.Int32
	once    sync.Once
}

func (t *blockingRefreshRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/oauth/token" {
		call := t.calls.Add(1)
		if call == 2 {
			select {
			case <-t.second:
			default:
				close(t.second)
			}
		}
		t.once.Do(func() { close(t.entered) })
		<-t.release
	}
	return t.base.RoundTrip(req)
}

func TestResolveOpenAISubscriptionCredential_RefreshesBeforeExpiryBuffer(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"scope":         "openid email",
			"account_id":    "acct_456",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.NoError(t, err)
	assert.Equal(t, "new-access", resolved.BearerToken)
	require.Len(t, tokenServer.Requests(), 1)
	assert.Equal(t, "refresh_token", tokenServer.Requests()[0].Form.Get("grant_type"))
}

func TestOpenAISubscriptionProactiveRefreshJob_RefreshesIdleCredentialInsideBuffer(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "idle-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(20 * time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token":  "proactive-access",
			"refresh_token": "proactive-refresh",
			"expires_in":    3600,
			"account_id":    "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Eligible: 1, Refreshed: 1}, result)
	require.Len(t, tokenServer.Requests(), 1)
	assert.Equal(t, "proactive-access", store.decryptedBundle(t).AccessToken)
}

func TestOpenAISubscriptionProactiveRefreshJob_SkipsCredentialOutsideBuffer(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "idle-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(45 * time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Skipped: 1}, result)
	assert.Empty(t, tokenServer.Requests())
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Zero(t, infoWrites)
}

func TestOpenAISubscriptionProactiveRefreshJob_RefreshTokenInvalidatedMarksReconnectRequired(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(20 * time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusUnauthorized,
		Response: map[string]any{
			"error":             "refresh_token_invalidated",
			"error_description": "Your session has ended. Please log in again.",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Eligible: 1, Failed: 1, ReconnectRequired: 1}, result)
	info := store.credentialInfo(t)
	require.NotNil(t, info.FirstRefreshFailedAt)
	require.NotNil(t, info.LastRefreshFailedAt)
	assert.Equal(t, now, *info.FirstRefreshFailedAt)
	assert.Equal(t, now, *info.LastRefreshFailedAt)
	assert.Equal(t, "refresh_failed", info.Status)
	assert.Equal(t, "refresh_token_invalidated", info.DisabledReason)
	assert.Equal(t, openAISubscriptionReconnectRequiredMsg, info.OperatorMessage)
	assert.Equal(t, openAISubscriptionReconnectRequiredMsg, info.LastError)
	assert.NotContains(t, info.LastError, "refresh-secret")
}

func TestOpenAISubscriptionProactiveRefreshJob_TransientFailureDoesNotClaimReconnectRequired(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(20 * time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusInternalServerError,
		Response: map[string]any{
			"error":             "server_error",
			"error_description": "temporary upstream failure for refresh-secret",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Eligible: 1, Failed: 1}, result)
	info := store.credentialInfo(t)
	assert.Equal(t, "refresh_failed", info.Status)
	assert.Equal(t, "refresh_failed", info.DisabledReason)
	assert.Empty(t, info.OperatorMessage)
	assert.NotEqual(t, openAISubscriptionReconnectRequiredMsg, info.LastError)
	assert.NotContains(t, info.LastError, "refresh-secret")
}

func TestOpenAISubscriptionProactiveRefreshJob_SuccessClearsTransientFailureMetadata(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	firstFailedAt := now.Add(-time.Hour)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(20 * time.Minute),
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{
		Email:                "admin@example.com",
		Scopes:               []string{"openid", "offline_access"},
		Status:               "active",
		FirstRefreshFailedAt: &firstFailedAt,
		LastRefreshFailedAt:  &firstFailedAt,
		LastError:            "temporary failure",
	})
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "new-access",
			"expires_in":   3600,
			"account_id":   "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Eligible: 1, Refreshed: 1}, result)
	info := store.credentialInfo(t)
	assert.Equal(t, "active", info.Status)
	assert.Nil(t, info.FirstRefreshFailedAt)
	assert.Nil(t, info.LastRefreshFailedAt)
	assert.Empty(t, info.LastError)
	assert.Empty(t, info.DisabledReason)
}

func TestOpenAISubscriptionProactiveRefreshJob_RedactsFailureMetadataSecrets(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(20 * time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "Bearer old-access-secret refresh_token=refresh-secret eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	_, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())

	require.NoError(t, err)
	info := store.credentialInfo(t)
	assert.NotContains(t, info.LastError, "old-access-secret")
	assert.NotContains(t, info.LastError, "refresh-secret")
	assert.NotContains(t, info.LastError, "Bearer ")
	assert.NotContains(t, info.LastError, "eyJhbGci")
}

func TestResolveOpenAISubscriptionCredential_ConcurrentRefreshSingleflight(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())

	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh-secret", r.Form.Get("refresh_token"))
		requests.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"shared-access","refresh_token":"shared-refresh","expires_in":1800,"account_id":"acct_123"}`))
	}))
	defer server.Close()

	h := mockHandlers(store.mock())
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = store.masterKey
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		TokenURL: server.URL,
		ClientID: "app_test",
	}
	h.OpenAIOAuthHTTPClient = server.Client()
	h.openAISubscriptionNow = func() time.Time { return now }

	const callers = 8
	errs := make(chan error, callers)
	tokens := make(chan string, callers)
	for range callers {
		go func() {
			resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
				Model:                           "openai/gpt-4o",
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			}, openAISubscriptionTransportChatGPTCodexBackend)
			if err != nil {
				errs <- err
				return
			}
			tokens <- resolved.BearerToken
			errs <- nil
		}()
	}
	<-started
	close(release)

	for range callers {
		require.NoError(t, <-errs)
		assert.Equal(t, "shared-access", <-tokens)
	}
	assert.Equal(t, int32(1), requests.Load())
}

func TestOpenAISubscriptionProactiveRefreshJob_SharesSingleflightWithRequestResolution(t *testing.T) {
	now := time.Date(2026, 6, 18, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())

	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh-secret", r.Form.Get("refresh_token"))
		requests.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"shared-access","refresh_token":"shared-refresh","expires_in":1800,"account_id":"acct_123"}`))
	}))
	defer server.Close()

	h := mockHandlers(store.mock())
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = store.masterKey
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		TokenURL: server.URL,
		ClientID: "app_test",
	}
	h.OpenAIOAuthHTTPClient = server.Client()
	h.openAISubscriptionNow = func() time.Time { return now }

	proactiveDone := make(chan error, 1)
	go func() {
		result, err := h.ProactiveRefreshOpenAISubscriptionCredentials(context.Background())
		if err == nil {
			assert.Equal(t, OpenAISubscriptionProactiveRefreshResult{Scanned: 1, Eligible: 1, Refreshed: 1}, result)
		}
		proactiveDone <- err
	}()
	<-started

	requestDone := make(chan resolvedOpenAISubscriptionCredential, 1)
	requestErr := make(chan error, 1)
	go func() {
		resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
			Model:                           "openai/gpt-4o",
			OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
		}, openAISubscriptionTransportChatGPTCodexBackend)
		if err != nil {
			requestErr <- err
			return
		}
		requestDone <- resolved
		requestErr <- nil
	}()

	close(release)
	require.NoError(t, <-proactiveDone)
	require.NoError(t, <-requestErr)
	assert.Equal(t, "shared-access", (<-requestDone).BearerToken)
	assert.Equal(t, int32(1), requests.Load())
}

func TestResolveOpenAISubscriptionCredential_RefreshPersistsRotatedToken(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "old-refresh",
		Response: map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    1800,
			"account_id":    "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.NoError(t, err)
	stored := store.decryptedBundle(t)
	assert.Equal(t, "new-access", stored.AccessToken)
	assert.Equal(t, "new-refresh", stored.RefreshToken)
	assert.Equal(t, now.Add(1800*time.Second), stored.ExpiresAt)
	valueWrites, infoWrites := store.writeCounts()
	assert.Equal(t, 1, valueWrites)
	assert.Zero(t, infoWrites)
}

func TestResolveOpenAISubscriptionCredential_RefreshFailurePersistsRedactedMetadata(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	original := OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}
	store := newOpenAISubscriptionRefreshStore(t, original, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh_token=refresh-secret was rejected",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Equal(t, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionErrorCode(err))
	assert.NotContains(t, err.Error(), "refresh-secret")
	assert.Equal(t, original, store.decryptedBundle(t))
	info := store.credentialInfo(t)
	assert.Equal(t, "refresh_failed", info.Status)
	assert.Equal(t, "refresh_failed", info.DisabledReason)
	assert.Contains(t, info.LastError, "invalid_grant")
	assert.NotContains(t, info.LastError, "refresh-secret")
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Equal(t, 1, infoWrites)
}

func TestResolveOpenAISubscriptionCredential_InvalidRefreshResponsePersistsFailureMetadata(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "new-access",
			"account_id":   "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Equal(t, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionErrorCode(err))
	info := store.credentialInfo(t)
	assert.Equal(t, "refresh_failed", info.Status)
	assert.Contains(t, info.LastError, "expires_at required")
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Equal(t, 1, infoWrites)
}

func TestResolveOpenAISubscriptionCredential_RefreshFailureMetadataWriteFailureIsReturned(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	store.infoWriteErr = errors.New("db down for refresh-secret")
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error": "invalid_grant",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Equal(t, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionErrorCode(err))
	assert.Contains(t, err.Error(), "persist failure metadata")
	assert.NotContains(t, err.Error(), "refresh-secret")
}

func TestResolveOpenAISubscriptionCredential_TypedFailureCodes(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		credential db.CredentialTable
		lookupErr  error
		wantCode   OpenAISubscriptionCredentialErrorCode
	}{
		{
			name:      "missing",
			lookupErr: pgx.ErrNoRows,
			wantCode:  OpenAISubscriptionCredentialMissing,
		},
		{
			name:      "lookup failure",
			lookupErr: errors.New("db down for refresh-secret"),
			wantCode:  OpenAISubscriptionCredentialLookupErr,
		},
		{
			name: "wrong type",
			credential: db.CredentialTable{
				CredentialID:   "cred-refresh",
				CredentialType: "api_key",
			},
			wantCode: OpenAISubscriptionCredentialWrongType,
		},
		{
			name: "disabled",
			credential: db.CredentialTable{
				CredentialID:   "cred-refresh",
				CredentialType: CredentialTypeOpenAISubscription,
				CredentialInfo: []byte(`{"status":"disabled"}`),
			},
			wantCode: OpenAISubscriptionCredentialDisabled,
		},
		{
			name: "malformed",
			credential: db.CredentialTable{
				CredentialID:    "cred-refresh",
				CredentialType:  CredentialTypeOpenAISubscription,
				CredentialValue: "not-encrypted",
				CredentialInfo:  []byte(`{"status":"active"}`),
			},
			wantCode: OpenAISubscriptionCredentialMalformed,
		},
		{
			name: "expired without refresh token",
			credential: db.CredentialTable{
				CredentialID:   "cred-refresh",
				CredentialType: CredentialTypeOpenAISubscription,
				CredentialValue: encryptOpenAITokenBundle(t, OpenAISubscriptionTokenBundle{
					AccessToken: "old-access",
					ExpiresAt:   now.Add(-time.Minute),
					AccountID:   "acct_123",
				}, openAISubscriptionRefreshTestMasterKey),
				CredentialInfo: []byte(`{"status":"active"}`),
			},
			wantCode: OpenAISubscriptionCredentialExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockStore()
			m.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
				if tt.lookupErr != nil {
					return db.CredentialTable{}, tt.lookupErr
				}
				if tt.credential.CredentialID == "" || credentialID != tt.credential.CredentialID {
					return db.CredentialTable{}, errors.New("not found")
				}
				return tt.credential, nil
			}
			h := mockHandlers(m)
			h.Config = &config.ProxyConfig{}
			h.Config.GeneralSettings.MasterKey = openAISubscriptionRefreshTestMasterKey
			h.openAISubscriptionNow = func() time.Time { return now }

			_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
				Model:                           "openai/gpt-4o",
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			}, openAISubscriptionTransportChatGPTCodexBackend)

			require.Error(t, err)
			assert.Equal(t, tt.wantCode, openAISubscriptionErrorCode(err))
			assert.NotContains(t, err.Error(), "old-access")
			assert.NotContains(t, err.Error(), "refresh-secret")
		})
	}
}
