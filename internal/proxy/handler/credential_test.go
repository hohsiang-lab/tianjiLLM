package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
)

func TestCredentialNew_Success(t *testing.T) {
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialName: arg.CredentialName}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{"credential_name":"aws","credential_value":"secret123"}`))
	r.Header.Set("Content-Type", "application/json")
	h.CredentialNew(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialNew_MissingFields(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{"credential_name":"aws"}`))
	r.Header.Set("Content-Type", "application/json")
	h.CredentialNew(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialNew_OpenAISubscriptionStoresCanonicalBundle(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	var got db.CreateCredentialParams
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		got = arg
		return db.CredentialTable{
			CredentialID:    arg.CredentialID,
			CredentialName:  arg.CredentialName,
			CredentialType:  arg.CredentialType,
			CredentialInfo:  arg.CredentialInfo,
			CredentialValue: arg.CredentialValue,
		}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{
		"credential_name":"openai-sub",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"access-secret\",\"refresh_token\":\"refresh-secret\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\",\"id_token\":\"id-secret\"}"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialNew(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	plaintext, err := auth.Decrypt(got.CredentialValue, masterKey)
	require.NoError(t, err)
	assert.NotContains(t, plaintext, "id_token")
	assert.NotContains(t, plaintext, "id-secret")
	assert.Contains(t, plaintext, "access-secret")
	assert.Contains(t, plaintext, "refresh-secret")
}

func TestCredentialNew_OpenAISubscriptionResponseIsRedacted(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    arg.CredentialID,
			CredentialName:  arg.CredentialName,
			CredentialType:  arg.CredentialType,
			CredentialInfo:  arg.CredentialInfo,
			CredentialValue: arg.CredentialValue,
		}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{
		"credential_name":"openai-sub",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"access-secret\",\"refresh_token\":\"refresh-secret\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\"}",
		"credential_info":{"email":"admin@example.com","status":"active"}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialNew(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "credential_value")
	assert.NotContains(t, body, "access-secret")
	assert.NotContains(t, body, "refresh-secret")
	assert.NotContains(t, body, "Bearer ")
	assert.Contains(t, body, "admin@example.com")
}

func TestCredentialNew_RejectsNestedSecretMetadata(t *testing.T) {
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, _ db.CreateCredentialParams) (db.CredentialTable, error) {
		t.Fatal("CreateCredential should not be called")
		return db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{
		"credential_name":"aws",
		"credential_value":"secret123",
		"credential_info":{"oauth":{"access_token":"nested-secret"}}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialNew(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "access_token is secret material")
}

func TestCredentialNew_RejectsSecretLookingMetadataValues(t *testing.T) {
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, _ db.CreateCredentialParams) (db.CredentialTable, error) {
		t.Fatal("CreateCredential should not be called")
		return db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{
		"credential_name":"openai-sub",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"access-secret\",\"refresh_token\":\"refresh-secret\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\"}",
		"credential_info":{"last_error":"Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.signature"}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialNew(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "eyJhbGci")
}

func TestCredentialList_Success(t *testing.T) {
	m := newMockStore()
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list", nil)
	h.CredentialList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialList_ByOrg(t *testing.T) {
	m := newMockStore()
	m.listCredentialsByOrgFn = func(_ context.Context, orgID *string) ([]db.CredentialTable, error) {
		return []db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list?organization_id=org-1", nil)
	h.CredentialList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialInfo_Success(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialName: "aws"}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.CredentialInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialInfo_MissingID(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/", nil)
	h.CredentialInfo(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteCredentialFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/credentials/delete/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.CredentialDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialUpdate_APIKeyStillAllowsOpaqueValue(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	var got db.UpdateCredentialParams
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialType: "api_key"}, nil
	}
	m.updateCredentialFn = func(_ context.Context, arg db.UpdateCredentialParams) error {
		got = arg
		return nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{"credential_id":"cred-1","credential_value":"opaque-secret"}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	plaintext, err := auth.Decrypt(got.CredentialValue, masterKey)
	require.NoError(t, err)
	assert.Equal(t, "opaque-secret", plaintext)
}

func TestCredentialUpdate_OpenAISubscriptionStoresCanonicalBundle(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	var got db.UpdateCredentialParams
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialType: CredentialTypeOpenAISubscription}, nil
	}
	m.updateCredentialFn = func(_ context.Context, arg db.UpdateCredentialParams) error {
		got = arg
		return nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_value":"{\"access_token\":\"access-secret\",\"refresh_token\":\"refresh-secret\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\",\"id_token\":\"id-secret\"}"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	plaintext, err := auth.Decrypt(got.CredentialValue, masterKey)
	require.NoError(t, err)
	assert.NotContains(t, plaintext, "id_token")
	assert.NotContains(t, plaintext, "id-secret")
	assert.Contains(t, plaintext, "access-secret")
	assert.Contains(t, plaintext, "refresh-secret")
}

func TestCredentialUpdate_OpenAISubscriptionUpdatesValueAndSafeMetadata(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	var got db.UpdateCredentialValueAndInfoParams
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:   id,
			CredentialName: "openai-sub",
			CredentialType: CredentialTypeOpenAISubscription,
		}, nil
	}
	m.updateCredentialValueAndInfoFn = func(_ context.Context, arg db.UpdateCredentialValueAndInfoParams) error {
		got = arg
		return nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"new-access\",\"refresh_token\":\"new-refresh\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\"}",
		"credential_info":{"email":"admin@example.com","status":"active","scopes":["openid","offline_access"]}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	stored := decryptOpenAITokenBundle(t, got.CredentialValue, masterKey)
	assert.Equal(t, "new-access", stored.AccessToken)
	assert.Equal(t, "new-refresh", stored.RefreshToken)
	assert.JSONEq(t, `{"email":"admin@example.com","status":"active","scopes":["openid","offline_access"]}`, string(got.CredentialInfo))
	body := w.Body.String()
	assert.NotContains(t, body, "credential_value")
	assert.NotContains(t, body, "new-access")
	assert.NotContains(t, body, "new-refresh")
	assert.Contains(t, body, "admin@example.com")
}

func TestCredentialUpdate_OpenAISubscriptionValueReplaceInvalidatesCodexCatalogCache(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:   id,
			CredentialName: "openai-sub",
			CredentialType: CredentialTypeOpenAISubscription,
		}, nil
	}
	m.updateCredentialFn = func(_ context.Context, _ db.UpdateCredentialParams) error {
		return nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	recording := seedCodexCatalogCacheForTest(h, "cred-1")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"new-access\",\"refresh_token\":\"new-refresh\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\"}"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	_, cachedInMemory := h.CodexCatalogCache.get("cred-1")
	assert.False(t, cachedInMemory)
	assert.NotContains(t, recording.values, codexCatalogCacheKey("cred-1"))
}

func TestCredentialUpdate_OpenAISubscriptionUpdatesSafeMetadataOnly(t *testing.T) {
	m := newMockStore()
	var got db.UpdateCredentialInfoParams
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-existing",
			CredentialInfo:  []byte(`{"status":"active"}`),
			OrganizationID:  nil,
		}, nil
	}
	m.updateCredentialInfoFn = func(_ context.Context, arg db.UpdateCredentialInfoParams) error {
		got = arg
		return nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"openai_subscription",
		"credential_info":{"status":"disabled","disabled_reason":"manual"}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cred-1", got.CredentialID)
	assert.JSONEq(t, `{"status":"disabled","disabled_reason":"manual"}`, string(got.CredentialInfo))
	assert.NotContains(t, w.Body.String(), "credential_value")
	assert.NotContains(t, w.Body.String(), "encrypted-existing")
	assert.Contains(t, w.Body.String(), "disabled")
}

func TestCredentialUpdate_OpenAISubscriptionMetadataOnlyInvalidatesCodexCatalogCache(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-existing",
			CredentialInfo:  []byte(`{"status":"active"}`),
		}, nil
	}
	m.updateCredentialInfoFn = func(_ context.Context, _ db.UpdateCredentialInfoParams) error {
		return nil
	}
	h := mockHandlers(m)
	recording := seedCodexCatalogCacheForTest(h, "cred-1")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"openai_subscription",
		"credential_info":{"status":"disabled","disabled_reason":"manual"}
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	_, cachedInMemory := h.CodexCatalogCache.get("cred-1")
	assert.False(t, cachedInMemory)
	assert.NotContains(t, recording.values, codexCatalogCacheKey("cred-1"))
}

func TestCredentialUpdate_OpenAISubscriptionRejectsAPIKeyRow(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialType: "api_key"}, nil
	}
	m.updateCredentialFn = func(_ context.Context, _ db.UpdateCredentialParams) error {
		t.Fatal("UpdateCredential should not be called")
		return nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"openai_subscription",
		"credential_value":"{\"access_token\":\"new-access\",\"refresh_token\":\"new-refresh\",\"expires_at\":\"2026-05-06T12:00:00Z\",\"account_id\":\"acct_123\"}"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "credential type mismatch")
}

func TestCredentialUpdate_RejectsCredentialTypeChange(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialType: CredentialTypeOpenAISubscription}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/update", strings.NewReader(`{
		"credential_id":"cred-1",
		"credential_type":"api_key",
		"credential_value":"opaque-secret"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CredentialUpdate(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "credential_type cannot be changed")
}

func TestCredentialDelete_IdempotentForMissingCredential(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, _ string) (db.CredentialTable, error) {
		return db.CredentialTable{}, assert.AnError
	}
	m.deleteCredentialFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/credentials/delete/missing", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "missing")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	h.CredentialDelete(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "deleted")
}

func TestCredentialDelete_OpenAISubscriptionInvalidatesCodexCatalogCache(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:   id,
			CredentialType: CredentialTypeOpenAISubscription,
		}, nil
	}
	m.deleteCredentialFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	recording := seedCodexCatalogCacheForTest(h, "cred-1")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/credentials/delete/cred-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "cred-1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	h.CredentialDelete(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	_, cachedInMemory := h.CodexCatalogCache.get("cred-1")
	assert.False(t, cachedInMemory)
	assert.NotContains(t, recording.values, codexCatalogCacheKey("cred-1"))
}

func TestCredentialCRUD_UsesOnlySyntheticOpenAITokenFixtures(t *testing.T) {
	fixture := `{"access_token":"access-secret","refresh_token":"refresh-secret","expires_at":"2026-05-06T12:00:00Z","account_id":"acct_123"}`

	assert.NotContains(t, fixture, "api.openai.com")
	assert.NotContains(t, fixture, "platform.openai.com")
	assert.NotContains(t, fixture, "sk-")
	assert.NotContains(t, fixture, "sess-")
}

func TestOpenAISubscriptionCredential_SaveEncryptsTokenBundle(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	expiresAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	bundle := OpenAISubscriptionTokenBundle{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    expiresAt,
		AccountID:    "acct_123",
	}
	m := newMockStore()
	var got db.CreateCredentialParams
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		got = arg
		return db.CredentialTable{CredentialID: arg.CredentialID}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	_, err := h.SaveOpenAISubscriptionCredential(context.Background(), "openai-sub", bundle, OpenAISubscriptionCredentialInfo{}, nil)
	require.NoError(t, err)
	assert.Equal(t, CredentialTypeOpenAISubscription, got.CredentialType)
	assert.NotContains(t, got.CredentialValue, bundle.AccessToken)
	assert.NotContains(t, got.CredentialValue, bundle.RefreshToken)

	plaintext, err := auth.Decrypt(got.CredentialValue, masterKey)
	require.NoError(t, err)
	var decrypted OpenAISubscriptionTokenBundle
	require.NoError(t, json.Unmarshal([]byte(plaintext), &decrypted))
	assert.Equal(t, bundle.AccessToken, decrypted.AccessToken)
	assert.Equal(t, bundle.RefreshToken, decrypted.RefreshToken)
	assert.Equal(t, bundle.ExpiresAt, decrypted.ExpiresAt)
	assert.Equal(t, bundle.AccountID, decrypted.AccountID)
}

func TestOpenAISubscriptionCredential_SaveForFlowIsIdempotent(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "session-idempotent")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	expected := record
	record.Status = openaioauth.DeviceAuthStatusExchanging
	applied, err := h.saveDeviceAuthStateIfCurrent(context.Background(), store, expected, &record)
	require.NoError(t, err)
	require.True(t, applied)
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	createCalls := 0
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		createCalls++
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType, OrganizationID: arg.OrganizationID}, nil
	}
	first, err := h.SaveOpenAISubscriptionCredentialForFlow(context.Background(), &record, "OpenAI Subscription", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
	require.NoError(t, err)
	second, err := h.SaveOpenAISubscriptionCredentialForFlow(context.Background(), &record, "OpenAI Subscription", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
	require.NoError(t, err)
	assert.Equal(t, first.CredentialID, second.CredentialID)
	assert.Equal(t, 1, createCalls)
}

func TestOpenAISubscriptionCredential_SaveStoresSafeMetadata(t *testing.T) {
	refreshedAt := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
	info := OpenAISubscriptionCredentialInfo{
		Email:         "admin@example.com",
		Scopes:        []string{"openid", "offline_access"},
		Status:        "active",
		LastRefreshAt: &refreshedAt,
	}
	m := newMockStore()
	var got db.CreateCredentialParams
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		got = arg
		return db.CredentialTable{CredentialID: arg.CredentialID}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"

	_, err := h.SaveOpenAISubscriptionCredential(context.Background(), "openai-sub", validOpenAITokenBundle(), info, nil)
	require.NoError(t, err)

	var stored map[string]any
	require.NoError(t, json.Unmarshal(got.CredentialInfo, &stored))
	assert.Equal(t, info.Email, stored["email"])
	assert.Equal(t, info.Status, stored["status"])
	assert.NotContains(t, stored, "access_token")
	assert.NotContains(t, stored, "refresh_token")
	assert.NotContains(t, stored, "credential_value")
}

func TestOpenAISubscriptionCredential_RejectsMissingSecretFields(t *testing.T) {
	h := mockHandlers(newMockStore())
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"

	_, err := h.SaveOpenAISubscriptionCredential(context.Background(), "openai-sub", OpenAISubscriptionTokenBundle{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{}, nil)
	require.Error(t, err)
}

func TestOpenAISubscriptionCredential_RejectsMissingAccountID(t *testing.T) {
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: arg.CredentialID}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	bundle := validOpenAITokenBundle()
	bundle.AccountID = ""

	_, err := h.SaveOpenAISubscriptionCredential(context.Background(), "openai-sub", bundle, OpenAISubscriptionCredentialInfo{}, nil)

	require.EqualError(t, err, "account_id required")
}

func TestCredentialList_RedactsCredentialValueAndReturnsInfo(t *testing.T) {
	m := newMockStore()
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{{
			CredentialID:    "cred-1",
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-secret",
			CredentialInfo:  []byte(`{"email":"admin@example.com","status":"active"}`),
		}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list", nil)

	h.CredentialList(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "credential_value")
	assert.NotContains(t, w.Body.String(), "encrypted-secret")
	assert.Contains(t, w.Body.String(), `"credential_info"`)
	assert.Contains(t, w.Body.String(), "admin@example.com")
}

func TestCredentialInfo_RedactsCredentialValueAndReturnsInfo(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-secret",
			CredentialInfo:  []byte(`{"email":"admin@example.com","status":"active"}`),
		}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	h.CredentialInfo(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "credential_value")
	assert.NotContains(t, w.Body.String(), "encrypted-secret")
	assert.Contains(t, w.Body.String(), `"credential_info"`)
	assert.Contains(t, w.Body.String(), "admin@example.com")
}

func TestCredentialInfo_RedactsNestedSecretMetadata(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-secret",
			CredentialInfo:  []byte(`{"oauth":{"access_token":"nested-secret","label":"visible"}}`),
		}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	h.CredentialInfo(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "access_token")
	assert.NotContains(t, w.Body.String(), "nested-secret")
	assert.Contains(t, w.Body.String(), "visible")
}

func TestCredentialList_RedactsAPIKeyCredentialsToo(t *testing.T) {
	m := newMockStore()
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{{
			CredentialID:    "cred-1",
			CredentialName:  "api-key",
			CredentialType:  "api_key",
			CredentialValue: "encrypted-api-key",
			CredentialInfo:  []byte(`{}`),
		}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list", nil)

	h.CredentialList(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "credential_value")
	assert.NotContains(t, w.Body.String(), "encrypted-api-key")
}

func TestOpenAISubscriptionCredential_UpdateRotatedRefreshToken(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	var got db.UpdateCredentialValueAndInfoParams
	m := openAISubscriptionUpdateMockStore(t, masterKey, func(arg db.UpdateCredentialValueAndInfoParams) {
		got = arg
	})
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	err := h.UpdateOpenAISubscriptionCredential(context.Background(), "cred-1", OpenAISubscriptionTokenBundle{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC),
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{Status: "active"})
	require.NoError(t, err)

	stored := decryptOpenAITokenBundle(t, got.CredentialValue, masterKey)
	assert.Equal(t, "new-refresh", stored.RefreshToken)
	assert.Equal(t, "new-access", stored.AccessToken)
}

func TestOpenAISubscriptionCredential_UpdatePreservesRefreshTokenWhenOmitted(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	var got db.UpdateCredentialValueAndInfoParams
	m := openAISubscriptionUpdateMockStore(t, masterKey, func(arg db.UpdateCredentialValueAndInfoParams) {
		got = arg
	})
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	err := h.UpdateOpenAISubscriptionCredential(context.Background(), "cred-1", OpenAISubscriptionTokenBundle{
		AccessToken: "new-access",
		ExpiresAt:   time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC),
		AccountID:   "acct_123",
	}, OpenAISubscriptionCredentialInfo{Status: "active"})
	require.NoError(t, err)

	stored := decryptOpenAITokenBundle(t, got.CredentialValue, masterKey)
	assert.Equal(t, "old-refresh", stored.RefreshToken)
	assert.Equal(t, "new-access", stored.AccessToken)
}

func TestOpenAISubscriptionCredential_UpdateInvalidatesCodexCatalogCache(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := openAISubscriptionUpdateMockStore(t, masterKey, func(db.UpdateCredentialValueAndInfoParams) {})
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey
	h.Cache = newRecordingCache()
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: "cred-1"})
	recording, ok := h.Cache.(*recordingCache)
	require.True(t, ok)
	recording.values[codexCatalogCacheKey("cred-1")] = []byte(`{"catalog":"cached"}`)

	err := h.UpdateOpenAISubscriptionCredential(context.Background(), "cred-1", OpenAISubscriptionTokenBundle{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC),
		AccountID:    "acct_123",
	}, OpenAISubscriptionCredentialInfo{Status: "active"})
	require.NoError(t, err)

	_, cachedInMemory := h.CodexCatalogCache.get("cred-1")
	assert.False(t, cachedInMemory)
	assert.NotContains(t, recording.values, codexCatalogCacheKey("cred-1"))
}

func TestOpenAISubscriptionCredential_UpdateFailureMetadataOnly(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	oldEncrypted := encryptOpenAITokenBundle(t, validOpenAITokenBundle(), masterKey)
	m := newMockStore()
	var got db.UpdateCredentialInfoParams
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: oldEncrypted,
		}, nil
	}
	m.updateCredentialInfoFn = func(_ context.Context, arg db.UpdateCredentialInfoParams) error {
		got = arg
		return nil
	}
	h := mockHandlers(m)

	err := h.UpdateOpenAISubscriptionCredentialFailure(context.Background(), "cred-1", OpenAISubscriptionCredentialInfo{
		Status:         "disabled",
		LastError:      "invalid_grant",
		DisabledReason: "refresh_failed",
	})
	require.NoError(t, err)
	assert.Equal(t, "cred-1", got.CredentialID)
	assert.NotEqual(t, oldEncrypted, string(got.CredentialInfo))
	assert.Contains(t, string(got.CredentialInfo), "invalid_grant")
}

func TestOpenAISubscriptionCredential_UpdateRejectsWrongCredentialType(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:   id,
			CredentialType: "api_key",
		}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"

	err := h.UpdateOpenAISubscriptionCredential(context.Background(), "cred-1", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not openai_subscription")
}

func validOpenAITokenBundle() OpenAISubscriptionTokenBundle {
	return OpenAISubscriptionTokenBundle{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		AccountID:    "acct_123",
	}
}

func encryptOpenAITokenBundle(t *testing.T, bundle OpenAISubscriptionTokenBundle, masterKey string) string {
	t.Helper()
	plaintext, err := json.Marshal(bundle)
	require.NoError(t, err)
	encrypted, err := auth.Encrypt(string(plaintext), masterKey)
	require.NoError(t, err)
	return encrypted
}

func decryptOpenAITokenBundle(t *testing.T, encrypted, masterKey string) OpenAISubscriptionTokenBundle {
	t.Helper()
	plaintext, err := auth.Decrypt(encrypted, masterKey)
	require.NoError(t, err)
	var bundle OpenAISubscriptionTokenBundle
	require.NoError(t, json.Unmarshal([]byte(plaintext), &bundle))
	return bundle
}

func openAISubscriptionUpdateMockStore(t *testing.T, masterKey string, capture func(db.UpdateCredentialValueAndInfoParams)) *mockStore {
	t.Helper()
	m := newMockStore()
	oldEncrypted := encryptOpenAITokenBundle(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC),
		AccountID:    "acct_123",
	}, masterKey)
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: oldEncrypted,
			CredentialInfo:  []byte(`{"status":"active"}`),
		}, nil
	}
	m.updateCredentialValueAndInfoFn = func(_ context.Context, arg db.UpdateCredentialValueAndInfoParams) error {
		capture(arg)
		return nil
	}
	return m
}

func seedCodexCatalogCacheForTest(h *Handlers, credentialID string) *recordingCache {
	recording := newRecordingCache()
	h.Cache = recording
	h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	h.CodexCatalogCache.put(OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID})
	recording.values[codexCatalogCacheKey(credentialID)] = []byte(`{"catalog":"cached"}`)
	return recording
}
