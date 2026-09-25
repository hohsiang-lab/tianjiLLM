package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callbackStateReadBarrierCache struct {
	*cache.DualCache
	afterRead func()
}

func (c *callbackStateReadBarrierCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	body, err := c.DualCache.GetShared(ctx, key)
	if err == nil {
		c.afterRead()
	}
	return body, err
}

func TestOpenAIOAuthCallback_StaleConsumeExchangesAndSavesOnce(t *testing.T) {
	for _, pasted := range []bool{false, true} {
		t.Run(fmt.Sprintf("stale_pasted_%t", pasted), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			first, _, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
			store, ok := first.DB.(*mockStore)
			require.True(t, ok)
			second := mockHandlers(store)
			second.Config = first.Config
			second.OpenAIOAuthHTTPClient = first.OpenAIOAuthHTTPClient
			server := miniredis.RunT(t)
			newCache := func() *cache.DualCache {
				client := redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = client.Close() })
				return cache.NewDualCache(cache.NewMemoryCache(), cache.NewRedisCache(client))
			}
			captured, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(resume) })
			first.Cache = &callbackStateReadBarrierCache{DualCache: newCache(), afterRead: sync.OnceFunc(func() {
				close(captured)
				select {
				case <-resume:
				case <-ctx.Done():
				}
			})}
			second.Cache = newCache()
			save := store.createCredentialFn
			var saves atomic.Int32
			store.createCredentialFn = func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
				saves.Add(1)
				return save(ctx, arg)
			}
			record := seedOpenAIState(t, first.Cache, "org_state")
			invoke := func(h *Handlers, pasted bool) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				callbackURL := record.RedirectURI + "?state=" + url.QueryEscape(record.State) + "&code=code-ok"
				if pasted {
					req := httptest.NewRequest(http.MethodPost, "/ui/openai/callback-url", strings.NewReader(url.Values{"callback_url": {callbackURL}}.Encode())).WithContext(ctx)
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					h.OpenAIOAuthPastedCallback(w, req)
				} else {
					h.OpenAIOAuthCallback(w, httptest.NewRequest(http.MethodGet, callbackURL, nil).WithContext(ctx))
				}
				return w
			}
			var stale *httptest.ResponseRecorder
			go func() {
				defer close(done)
				stale = invoke(first, pasted)
			}()
			t.Cleanup(func() { release(); <-done })
			select {
			case <-captured:
			case <-ctx.Done():
				t.Fatal("callback did not reach the shared-read barrier")
			}
			lockKey := openaioauth.CacheKey(record.State) + ":consume"
			leaseTTL := server.TTL(lockKey)
			require.Positive(t, leaseTTL)
			server.FastForward(leaseTTL + time.Second)
			require.False(t, server.Exists(lockKey))
			require.True(t, server.Exists(openaioauth.CacheKey(record.State)))
			winner := invoke(second, !pasted)
			release()
			<-done
			exchanges := len(oauthServer.Requests())
			t.Logf("CALLBACK exchanges=%d saves=%d winner_status=%d stale_status=%d", exchanges, saves.Load(), winner.Code, stale.Code)
			assert.Equal(t, http.StatusOK, winner.Code)
			assert.Equal(t, http.StatusBadRequest, stale.Code)
			assert.Equal(t, 1, exchanges)
			assert.Equal(t, int32(1), saves.Load())
			assert.Equal(t, http.StatusBadRequest, invoke(first, pasted).Code)
			assert.Equal(t, http.StatusBadRequest, invoke(second, !pasted).Code)
			assert.Equal(t, 1, len(oauthServer.Requests()))
			assert.Equal(t, int32(1), saves.Load())
		})
	}
}

func TestOpenAIOAuthCallbackDisabledDoesNotConsumeOrExchange(t *testing.T) {
	for _, pasted := range []bool{false, true} {
		t.Run(fmt.Sprint(pasted), func(t *testing.T) {
			h, c, server, saved := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
			record := seedOpenAIState(t, c, "org-disabled")
			h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
			callbackURL := "http://localhost:1455/auth/callback?state=" + url.QueryEscape(record.State) + "&code=disabled-callback-fixture"
			w := httptest.NewRecorder()
			if pasted {
				req := httptest.NewRequest(http.MethodPost, "/ui/openai/callback-url", strings.NewReader(url.Values{"callback_url": {callbackURL}}.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				h.OpenAIOAuthPastedCallback(w, req)
			} else {
				h.OpenAIOAuthCallback(w, httptest.NewRequest(http.MethodGet, callbackURL, nil))
			}
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
			assert.True(t, strings.Contains(w.Body.String(), "OpenAI Connect is disabled"))
			assert.Zero(t, len(server.Requests()))
			assert.True(t, *saved == nil, "disabled callback must not save a credential")
			raw, err := c.Get(context.Background(), openaioauth.CacheKey(record.State))
			require.NoError(t, err)
			assert.True(t, len(raw) > 0, "disabled callback must not consume state")
		})
	}
}

func TestOpenAIOAuthCallback_SuccessConsumesStateAndPersistsCredential(t *testing.T) {
	h, c, oauthServer, got := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, *got)
	assert.Equal(t, CredentialTypeOpenAISubscription, (*got).CredentialType)
	require.NotNil(t, (*got).OrganizationID)
	assert.Equal(t, "org_state", *(*got).OrganizationID)
	assertStateDeleted(t, c, record.State)
	require.Len(t, oauthServer.Requests(), 1)
	oauthServer.AssertPublicClientPKCE(t, oauthServer.Requests()[0])
	assert.Equal(t, record.CodeVerifier, oauthServer.Requests()[0].Form.Get("code_verifier"))
	assert.Equal(t, record.RedirectURI, oauthServer.Requests()[0].Form.Get("redirect_uri"))
}

func TestOpenAIOAuthCallback_GlobalScopePersistsCredentialWithoutOrganization(t *testing.T) {
	h, c, _, got := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, *got)
	assert.Nil(t, (*got).OrganizationID)
}

func TestOpenAIOAuthCallback_SavesCredentialWithIDTokenAccountClaims(t *testing.T) {
	idToken := fakeOpenAIIDToken(t, map[string]any{
		"email": "jwt-admin@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_from_id_token",
			"chatgpt_user_id":    "user_from_id_token",
		},
	})
	h, c, _, got := newOpenAIOAuthCallbackHarnessWithResponse(t, http.StatusOK, map[string]any{
		"access_token":  "access-secret",
		"refresh_token": "refresh-secret",
		"id_token":      idToken,
		"expires_in":    3600,
		"scope":         "openid profile email",
	})
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, *got)
	decrypted := decryptOpenAITokenBundle(t, (*got).CredentialValue, h.getMasterKey())
	assert.Equal(t, "acct_from_id_token", decrypted.AccountID)
	var info OpenAISubscriptionCredentialInfo
	require.NoError(t, json.Unmarshal((*got).CredentialInfo, &info))
	assert.Equal(t, "jwt-admin@example.com", info.Email)
}

func TestOpenAIOAuthCallback_UsesStoredRedirectURIForExchange(t *testing.T) {
	h, c, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIStateWithRedirectURI(t, c, "org_state", "http://127.0.0.1:1455/auth/callback")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, oauthServer.Requests(), 1)
	assert.Equal(t, "http://127.0.0.1:1455/auth/callback", oauthServer.Requests()[0].Form.Get("redirect_uri"))
}

func TestOpenAIOAuthCallback_IgnoresQueryOrgID(t *testing.T) {
	h, c, _, got := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_from_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok&org_id=evil&organization_id=evil&credential_id=evil", nil)
	h.OpenAIOAuthCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, (*got).OrganizationID)
	assert.Equal(t, "org_from_state", *(*got).OrganizationID)
}

func TestOpenAIOAuthCallback_ReplayFailsAfterConsume(t *testing.T) {
	h, c, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_state")

	first := httptest.NewRecorder()
	h.OpenAIOAuthCallback(first, httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil))
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	h.OpenAIOAuthCallback(second, httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil))
	assert.Equal(t, http.StatusBadRequest, second.Code)
	assert.Len(t, oauthServer.Requests(), 1)
}

func TestOpenAIOAuthCallback_DoesNotRequireUISessionCookie(t *testing.T) {
	h, c, _, got := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, *got)
}

func TestOpenAIOAuthCallback_StateExpiredFailsReadablePage(t *testing.T) {
	h, c, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := openaioauth.StateRecord{
		State:        "expired-state",
		CodeVerifier: "verifier",
		OrgID:        "org_state",
		RedirectURI:  "http://localhost:1455/auth/callback",
		CreatedAt:    time.Now().Add(-20 * time.Minute),
		ExpiresAt:    time.Now().Add(-10 * time.Minute),
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, c.Set(context.Background(), openaioauth.CacheKey(record.State), body, time.Hour))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
	assert.Empty(t, oauthServer.Requests())
	assertStateDeleted(t, c, record.State)
}

func TestOpenAIOAuthCallback_StateMismatchFailsReadablePage(t *testing.T) {
	h, _, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state=unknown-state&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid")
	assert.Empty(t, oauthServer.Requests())
}

func TestOpenAIOAuthCallback_OpenAIErrorConsumesState(t *testing.T) {
	h, c, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&error=access_denied&error_description=Denied", nil)
	h.OpenAIOAuthCallback(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "access_denied")
	assert.Empty(t, oauthServer.Requests())
	assertStateDeleted(t, c, record.State)
}

func TestOpenAIOAuthCallback_TokenExchangeFailureConsumesStateAndRedacts(t *testing.T) {
	h, c, _, _ := newOpenAIOAuthCallbackHarness(t, http.StatusBadRequest)
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state="+record.State+"&code=code-ok", nil)
	h.OpenAIOAuthCallback(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "token exchange failed")
	assert.NotContains(t, body, "access-secret")
	assert.NotContains(t, body, "refresh-secret")
	assert.NotContains(t, body, record.CodeVerifier)
	assertStateDeleted(t, c, record.State)
}

func TestParseOpenAIPastedCallbackURL_ValidatesHostPathAndFields(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid",
			raw:  "http://localhost:1455/auth/callback?code=code-ok&state=state-ok",
		},
		{
			name:    "malformed",
			raw:     ":// bad",
			wantErr: "valid callback URL",
		},
		{
			name:    "unexpected_host",
			raw:     "https://tianji.example.com/auth/callback?code=code-ok&state=state-ok",
			wantErr: "localhost callback URL",
		},
		{
			name:    "unexpected_path",
			raw:     "http://localhost:1455/oauth/openai/callback?code=code-ok&state=state-ok",
			wantErr: "localhost callback URL",
		},
		{
			name:    "provider_error",
			raw:     "http://localhost:1455/auth/callback?error=access_denied&state=state-ok",
			wantErr: "",
		},
		{
			name:    "missing_state",
			raw:     "http://localhost:1455/auth/callback?code=code-ok",
			wantErr: "state",
		},
		{
			name:    "missing_code",
			raw:     "http://localhost:1455/auth/callback?state=state-ok",
			wantErr: "authorization code",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOpenAIPastedCallbackURL(tc.raw, "http://localhost:1455/auth/callback")
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "state-ok", got.State)
		})
	}
}

func TestOpenAIOAuthPastedCallback_SuccessConsumesStateAndPersistsCredential(t *testing.T) {
	h, c, oauthServer, got := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	record := seedOpenAIState(t, c, "org_state")

	w := httptest.NewRecorder()
	form := "callback_url=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback%3Fstate%3D" + record.State + "%26code%3Dcode-ok"
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/callback-url", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.OpenAIOAuthPastedCallback(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, *got)
	require.NotNil(t, (*got).OrganizationID)
	assert.Equal(t, "org_state", *(*got).OrganizationID)
	assertStateDeleted(t, c, record.State)
	require.Len(t, oauthServer.Requests(), 1)
}

func TestOpenAIOAuthPastedCallback_RejectsUnexpectedURLWithoutLeakingSecrets(t *testing.T) {
	h, _, oauthServer, _ := newOpenAIOAuthCallbackHarness(t, http.StatusOK)
	rawURL := "https://tianji.example.com/auth/callback?state=state-secret&code=code-secret"

	w := httptest.NewRecorder()
	form := "callback_url=" + url.QueryEscape(rawURL)
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/callback-url", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.OpenAIOAuthPastedCallback(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, rawURL)
	assert.NotContains(t, body, "state-secret")
	assert.NotContains(t, body, "code-secret")
	assert.Empty(t, oauthServer.Requests())
}

func newOpenAIOAuthCallbackHarness(t *testing.T, tokenStatus int) (*Handlers, cache.Cache, *openaitest.OAuthServer, **db.CreateCredentialParams) {
	t.Helper()
	return newOpenAIOAuthCallbackHarnessWithResponse(t, tokenStatus, map[string]any{
		"access_token":  "access-secret",
		"refresh_token": "refresh-secret",
		"expires_in":    3600,
		"scope":         "openid profile email",
		"account_id":    "acct_123",
		"email":         "admin@example.com",
	})
}

func newOpenAIOAuthCallbackHarnessWithResponse(t *testing.T, tokenStatus int, response map[string]any) (*Handlers, cache.Cache, *openaitest.OAuthServer, **db.CreateCredentialParams) {
	t.Helper()
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{
		TokenFixtures: []openaitest.TokenFixture{{
			Code:       "code-ok",
			StatusCode: tokenStatus,
			Response:   response,
		}},
	})
	var got *db.CreateCredentialParams
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		got = &arg
		return db.CredentialTable{
			CredentialID:    arg.CredentialID,
			CredentialName:  arg.CredentialName,
			CredentialType:  arg.CredentialType,
			CredentialValue: arg.CredentialValue,
			CredentialInfo:  arg.CredentialInfo,
			OrganizationID:  arg.OrganizationID,
		}, nil
	}
	h := mockHandlers(m)
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		Enabled:      true,
		AuthorizeURL: oauthServer.AuthorizeURL(),
		TokenURL:     oauthServer.TokenURL(),
		ClientID:     "app_test",
	}
	h.Cache = cache.NewMemoryCache()
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(oauthServer.Host())
	return h, h.Cache, oauthServer, &got
}

func fakeOpenAIIDToken(t *testing.T, payload map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "none", "typ": "JWT"}
	encode := func(value map[string]any) string {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(body)
	}
	return encode(header) + "." + encode(payload) + "." + base64.RawURLEncoding.EncodeToString([]byte("sig"))
}

func seedOpenAIState(t *testing.T, c cache.Cache, orgID string) openaioauth.StateRecord {
	t.Helper()
	return seedOpenAIStateWithRedirectURI(t, c, orgID, "http://localhost:1455/auth/callback")
}

func seedOpenAIStateWithRedirectURI(t *testing.T, c cache.Cache, orgID, redirectURI string) openaioauth.StateRecord {
	t.Helper()
	store := openaioauth.NewStateStore(c)
	record, err := store.Create(context.Background(), orgID, redirectURI, time.Minute)
	require.NoError(t, err)
	return record
}

func assertStateDeleted(t *testing.T, c cache.Cache, state string) {
	t.Helper()
	raw, err := c.Get(context.Background(), openaioauth.CacheKey(state))
	require.NoError(t, err)
	// Atomic consumption clears the record instead of relying on Delete success.
	assert.True(t, len(raw) == 0 || string(raw) == `{}`, "state must be deleted or replaced by an empty terminal marker")
}
