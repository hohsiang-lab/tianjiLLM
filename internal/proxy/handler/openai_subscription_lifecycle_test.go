package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOpenAISubscriptionLifecycleAudits(t *testing.T, h *Handlers) *[]db.InsertAuditLogParams {
	t.Helper()
	h.Config.GeneralSettings.StoreAuditLogs = true
	var audits []db.InsertAuditLogParams
	mockDB, ok := h.DB.(*mockStore)
	require.True(t, ok)
	mockDB.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		audits = append(audits, arg)
		return db.AuditLog{}, nil
	}
	return &audits
}

func assertOpenAISubscriptionLifecycleAudits(t *testing.T, audits *[]db.InsertAuditLogParams, want []string, forbidden []string) {
	t.Helper()
	require.NotEmpty(t, *audits)
	body := ""
	for _, audit := range *audits {
		body += audit.Action
		body += audit.TableName
		body += audit.ObjectID
		body += string(audit.UpdatedValues)
	}
	for _, value := range want {
		assert.Contains(t, body, value)
	}
	for _, value := range forbidden {
		assert.NotContains(t, body, value)
	}
}

func TestOpenAISubscriptionLifecycleTest_FreshCredentialCallsModelsWithBearer(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	upstream := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{
		Models: []openaitest.ModelFixture{{ID: "gpt-4o"}, {ID: "gpt-4.1"}},
	})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.OpenAIUpstreamBaseURL = upstream.BaseURL()
	h.OpenAIUpstreamHTTPClient = openaitest.NewGuardedClient(upstream.Host())

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/test", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialTest(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "cred-refresh", body["credential_id"])
	assert.Equal(t, "test", body["action"])
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, float64(2), body["models_count"])
	assert.Empty(t, tokenServer.Requests())
	require.Len(t, upstream.Requests(), 1)
	assert.Equal(t, "/v1/models", upstream.Requests()[0].Path)
	assert.Equal(t, "Bearer fresh-access", upstream.Requests()[0].Header.Get("Authorization"))
	assert.NotContains(t, w.Body.String(), "fresh-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
}

func TestOpenAISubscriptionLifecycleTest_StaleCredentialRefreshesBeforeModels(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
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
	upstream := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.OpenAIUpstreamBaseURL = upstream.BaseURL()
	h.OpenAIUpstreamHTTPClient = openaitest.NewGuardedClient(upstream.Host(), tokenServer.Host())

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/test", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialTest(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, tokenServer.Requests(), 1)
	require.Len(t, upstream.Requests(), 1)
	assert.Equal(t, "Bearer refreshed-access", upstream.Requests()[0].Header.Get("Authorization"))
	assert.Equal(t, "refreshed-access", store.decryptedBundle(t).AccessToken)
	assert.NotContains(t, w.Body.String(), "refreshed-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
}

func TestOpenAISubscriptionLifecycleTest_UpstreamFailuresAreRedacted(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
			store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
				AccessToken:  "fresh-access",
				RefreshToken: "refresh-secret",
				ExpiresAt:    now.Add(time.Hour),
				AccountID:    "acct_123",
			}, activeOpenAISubscriptionInfo())
			tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `Bearer fresh-access eyJhbGciOiJIUzI1NiJ9.refresh-secret`, status)
			}))
			defer upstream.Close()
			h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
			h.OpenAIUpstreamBaseURL = upstream.URL + "/v1"
			h.OpenAIUpstreamHTTPClient = openaitest.NewGuardedClient(upstream.Listener.Addr().String())
			audits := captureOpenAISubscriptionLifecycleAudits(t, h)

			w := httptest.NewRecorder()
			req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/test", nil), "credential_id", "cred-refresh")
			h.OpenAISubscriptionCredentialTest(w, req)

			reasonCode := fmt.Sprintf("upstream_%d", status)
			require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), reasonCode)
			assert.NotContains(t, w.Body.String(), "fresh-access")
			assert.NotContains(t, w.Body.String(), "refresh-secret")
			assert.NotContains(t, w.Body.String(), "eyJhbGci")
			assertOpenAISubscriptionLifecycleAudits(t, audits,
				[]string{`"action":"test"`, `"status":"failure"`, `"reason_code":"` + reasonCode + `"`},
				[]string{"fresh-access", "refresh-secret", "eyJhbGci", "Bearer"},
			)
		})
	}
}

func TestOpenAISubscriptionLifecycleTest_NoRealOpenAIGuardBlocksDefaultHost(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.OpenAIUpstreamBaseURL = "https://api.openai.com/v1"
	h.OpenAIUpstreamHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/test", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialTest(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "upstream_request_failed")
	assert.Empty(t, tokenServer.Requests())
	assert.NotContains(t, w.Body.String(), "fresh-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
}

func TestOpenAISubscriptionLifecycleRefresh_ForcesRefreshAndRedactsResponse(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
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

	for range 2 {
		w := httptest.NewRecorder()
		req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/refresh", nil), "credential_id", "cred-refresh")
		h.OpenAISubscriptionCredentialRefresh(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
		assert.Contains(t, w.Body.String(), "last_refresh_at")
		assert.NotContains(t, w.Body.String(), "forced-access")
		assert.NotContains(t, w.Body.String(), "refresh-secret")
	}
	assert.Equal(t, "forced-access", store.decryptedBundle(t).AccessToken)
	assert.Equal(t, "refresh-secret", store.decryptedBundle(t).RefreshToken)
	assert.Len(t, tokenServer.Requests(), 2)
}

func TestOpenAISubscriptionLifecycleRefresh_FailurePersistsRedactedMetadata(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "Bearer fresh-access refresh-secret eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	audits := captureOpenAISubscriptionLifecycleAudits(t, h)

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/refresh", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialRefresh(w, req)

	require.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), string(OpenAISubscriptionCredentialRefreshErr))
	assert.NotContains(t, w.Body.String(), "fresh-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
	assert.NotContains(t, w.Body.String(), "eyJhbGci")
	info := store.credentialInfo(t)
	assert.Equal(t, "refresh_failed", info.Status)
	assert.NotContains(t, info.LastError, "fresh-access")
	assert.NotContains(t, info.LastError, "refresh-secret")
	assert.NotContains(t, info.LastError, "eyJhbGci")
	assertOpenAISubscriptionLifecycleAudits(t, audits,
		[]string{`"action":"refresh"`, `"status":"failure"`, `"reason_code":"refresh_failed"`},
		[]string{"fresh-access", "refresh-secret", "eyJhbGci", "Bearer"},
	)
}

func TestOpenAISubscriptionLifecycleDisable_IdempotentLocalMetadataOnly(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	for range 2 {
		w := httptest.NewRecorder()
		req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/disable", nil), "credential_id", "cred-refresh")
		h.OpenAISubscriptionCredentialDisable(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
		assert.NotContains(t, w.Body.String(), "fresh-access")
		assert.NotContains(t, w.Body.String(), "refresh-secret")
	}
	info := store.credentialInfo(t)
	assert.Equal(t, "disabled", info.Status)
	assert.Equal(t, "operator_disabled", info.DisabledReason)
	assert.Empty(t, tokenServer.Requests())
}

func TestOpenAISubscriptionLifecycleDisable_ExcludedByExistingResolution(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/disable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialDisable(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	_, err := h.resolveUsableOpenAISubscriptionBundle(context.Background(), "cred-refresh")
	require.Error(t, err)
	assert.Equal(t, OpenAISubscriptionCredentialDisabled, openAISubscriptionErrorCode(err))
}

func TestOpenAISubscriptionLifecycleEnable_RestoresOperatorDisabledCredential(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{
		Email:          "admin@example.com",
		Scopes:         []string{"openid", "offline_access"},
		Status:         "disabled",
		DisabledReason: openAISubscriptionOperatorDisabledReason,
		LastError:      "previous safe error",
	})
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/enable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialEnable(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"action":"enable"`)
	assert.NotContains(t, w.Body.String(), "fresh-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
	info := store.credentialInfo(t)
	assert.Equal(t, "active", info.Status)
	assert.Empty(t, info.DisabledReason)
	assert.Equal(t, "admin@example.com", info.Email)
	assert.Equal(t, []string{"openid", "offline_access"}, info.Scopes)
	assert.Equal(t, "previous safe error", info.LastError)
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Equal(t, 1, infoWrites)

	resolved, err := h.resolveUsableOpenAISubscriptionBundle(context.Background(), "cred-refresh")
	require.NoError(t, err)
	assert.Equal(t, "fresh-access", resolved.AccessToken)
	assert.Empty(t, tokenServer.Requests())
}

func TestOpenAISubscriptionLifecycleEnable_IdempotentForActiveCredential(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/enable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialEnable(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"action":"enable"`)
	info := store.credentialInfo(t)
	assert.Equal(t, "active", info.Status)
	assert.Empty(t, info.DisabledReason)
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Equal(t, 1, infoWrites)
	assert.Empty(t, tokenServer.Requests())
}

func TestOpenAISubscriptionLifecycleEnable_RejectsAuthFailureDisabledReason(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{
		Email:          "admin@example.com",
		Status:         "disabled",
		DisabledReason: string(OpenAISubscriptionCredentialAuthErr),
		LastError:      "auth failed",
	})
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/enable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialEnable(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"action":"enable"`)
	assert.Contains(t, w.Body.String(), string(OpenAISubscriptionCredentialDisabled))
	info := store.credentialInfo(t)
	assert.Equal(t, "disabled", info.Status)
	assert.Equal(t, string(OpenAISubscriptionCredentialAuthErr), info.DisabledReason)
	valueWrites, infoWrites := store.writeCounts()
	assert.Zero(t, valueWrites)
	assert.Zero(t, infoWrites)
	assert.Empty(t, tokenServer.Requests())
}

func TestOpenAISubscriptionLifecycleDisable_FailureAuditIsRedacted(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	audits := captureOpenAISubscriptionLifecycleAudits(t, h)
	mockDB, ok := h.DB.(*mockStore)
	require.True(t, ok)
	mockDB.updateCredentialInfoFn = func(_ context.Context, _ db.UpdateCredentialInfoParams) error {
		return errors.New("db update failed with fresh-access refresh-secret")
	}

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/disable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialDisable(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), string(OpenAISubscriptionCredentialLookupErr))
	assert.NotContains(t, w.Body.String(), "fresh-access")
	assert.NotContains(t, w.Body.String(), "refresh-secret")
	assertOpenAISubscriptionLifecycleAudits(t, audits,
		[]string{`"action":"disable"`, `"status":"failure"`, `"reason_code":"credential_lookup_failed"`},
		[]string{"fresh-access", "refresh-secret", "Bearer", "eyJhbGci"},
	)
}

func TestOpenAISubscriptionLifecycle_RejectsWrongTypeAndMissingSafely(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		if credentialID == "api-key" {
			return db.CredentialTable{
				CredentialID:    "api-key",
				CredentialType:  "api_key",
				CredentialValue: "sk-fallback-must-not-leak",
			}, nil
		}
		return db.CredentialTable{}, errors.New("missing")
	}
	h := mockHandlers(m)
	h.Config.GeneralSettings.MasterKey = openAISubscriptionRefreshTestMasterKey

	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{name: "wrong type", id: "api-key", want: string(OpenAISubscriptionCredentialWrongType)},
		{name: "missing", id: "missing", want: string(OpenAISubscriptionCredentialLookupErr)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/"+tc.id+"/test", nil), "credential_id", tc.id)
			h.OpenAISubscriptionCredentialTest(w, req)
			assert.NotEqual(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tc.want)
			assert.NotContains(t, w.Body.String(), "sk-fallback-must-not-leak")
		})
	}
}

func TestOpenAISubscriptionLifecycle_DeleteCompatibilityRemainsIdempotent(t *testing.T) {
	m := newMockStore()
	remaining := true
	m.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		if !remaining {
			return db.CredentialTable{}, errors.New("missing")
		}
		return db.CredentialTable{
			CredentialID:    credentialID,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-secret",
		}, nil
	}
	m.deleteCredentialFn = func(_ context.Context, _ string) error {
		remaining = false
		return nil
	}
	h := mockHandlers(m)

	for range 2 {
		w := httptest.NewRecorder()
		req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodDelete, "/credentials/delete/cred-delete", nil), "credential_id", "cred-delete")
		h.CredentialDelete(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"status":"deleted"`)
		assert.NotContains(t, w.Body.String(), "encrypted-secret")
	}
}

func TestOpenAISubscriptionLifecycle_AuditsAreRedacted(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	upstream := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Config.GeneralSettings.StoreAuditLogs = true
	h.OpenAIUpstreamBaseURL = upstream.BaseURL()
	h.OpenAIUpstreamHTTPClient = openaitest.NewGuardedClient(upstream.Host())

	var audits []db.InsertAuditLogParams
	mockDB, ok := h.DB.(*mockStore)
	require.True(t, ok)
	mockDB.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		audits = append(audits, arg)
		return db.AuditLog{}, nil
	}

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/test", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialTest(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	disable := httptest.NewRecorder()
	disableReq := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/disable", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialDisable(disable, disableReq)
	require.Equal(t, http.StatusOK, disable.Code, disable.Body.String())
	require.NotEmpty(t, audits)
	body, err := json.Marshal(audits)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"action":"test"`)
	assert.Contains(t, string(body), `"action":"disable"`)
	assert.NotContains(t, string(body), "fresh-access")
	assert.NotContains(t, string(body), "refresh-secret")
	assert.NotContains(t, string(body), "Bearer")
	assert.NotContains(t, string(body), "eyJhbGci")
}

func TestOpenAISubscriptionLifecycleRefresh_AuditIsRedacted(t *testing.T) {
	now := time.Date(2026, 5, 8, 2, 0, 0, 0, time.UTC)
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
	h.Config.GeneralSettings.StoreAuditLogs = true

	var audits []db.InsertAuditLogParams
	mockDB, ok := h.DB.(*mockStore)
	require.True(t, ok)
	mockDB.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		audits = append(audits, arg)
		return db.AuditLog{}, nil
	}

	w := httptest.NewRecorder()
	req := withOpenAISubscriptionChiURLParam(httptest.NewRequest(http.MethodPost, "/credentials/openai-subscription/cred-refresh/refresh", nil), "credential_id", "cred-refresh")
	h.OpenAISubscriptionCredentialRefresh(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotEmpty(t, audits)
	body, err := json.Marshal(audits)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"action":"refresh"`)
	assert.NotContains(t, string(body), "fresh-access")
	assert.NotContains(t, string(body), "forced-access")
	assert.NotContains(t, string(body), "refresh-secret")
}
