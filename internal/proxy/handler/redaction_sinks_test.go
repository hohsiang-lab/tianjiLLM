package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const redactionSinkJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2NvdW50Ijoib3duZXIifQ.M3g1h2IcxIqiOEgw07gF3f1hzoLlOTkeij54jpwte5c"

func TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets(t *testing.T) {
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
		LastError:      "refresh failed Authorization: Bearer " + redactionSinkJWT,
		DisabledReason: "provider returned access_token=access-secret",
	})
	require.NoError(t, err)

	body := string(got.CredentialInfo)
	assert.NotContains(t, body, redactionSinkJWT)
	assert.NotContains(t, body, "access-secret")
	assert.Contains(t, body, "disabled")
	assert.Contains(t, body, "[REDACTED]")
}

func TestCredentialInfo_RedactsNestedAccountPayload(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialName:  "openai-sub",
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: "encrypted-secret",
			CredentialInfo: []byte(`{
				"email":"admin@example.com",
				"account":{"profile":"kept","Access_Token":"access-secret","claims":["` + redactionSinkJWT + `"]}
			}`),
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
	assert.NotContains(t, w.Body.String(), "Access_Token")
	assert.NotContains(t, w.Body.String(), "access-secret")
	assert.NotContains(t, w.Body.String(), redactionSinkJWT)
	assert.Contains(t, w.Body.String(), "admin@example.com")
	assert.Contains(t, w.Body.String(), "kept")
}

func TestRecordErrorLog_RedactsUpstreamTokenText(t *testing.T) {
	t.Parallel()

	spy := newSpyDBTX()
	h := &Handlers{Config: &config.ProxyConfig{}, DB: db.New(spy)}
	ctx := context.WithValue(context.Background(), chiMiddleware.RequestIDKey, "req-redact")

	h.recordErrorLog(ctx, &model.ChatCompletionRequest{Model: "gpt-4"}, nil, fmt.Errorf("upstream body: Authorization: Bearer %s", redactionSinkJWT))

	spy.waitExec(t, 2*time.Second)
	args := spy.firstArgs(t)
	require.Len(t, args, 13)
	errorMessage, ok := args[6].(string)
	require.True(t, ok)
	assert.NotContains(t, errorMessage, redactionSinkJWT)
	assert.Contains(t, errorMessage, "[REDACTED]")
}

func TestCreateAuditLog_RedactsCredentialPayloads(t *testing.T) {
	m := newMockStore()
	var got db.InsertAuditLogParams
	m.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		got = arg
		return db.AuditLog{}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.StoreAuditLogs = true

	h.createAuditLog(context.Background(), "updated", "CredentialTable", "cred-1", "", "", map[string]any{
		"access_token": "before-secret",
	}, map[string]any{
		"nested": map[string]any{"refresh_token": "after-secret", "status": "disabled"},
	})

	assert.NotContains(t, string(got.BeforeValue), "before-secret")
	assert.NotContains(t, string(got.UpdatedValues), "after-secret")
	assert.Contains(t, string(got.BeforeValue), "[REDACTED]")
	assert.Contains(t, string(got.UpdatedValues), "disabled")
}

func TestDispatchEvent_RedactsCredentialPayload(t *testing.T) {
	got := make(chan hook.ManagementEvent, 1)
	dispatcher := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var event hook.ManagementEvent
		require.NoError(t, json.NewDecoder(r.Body).Decode(&event))
		got <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer dispatcher.Close()

	h := &Handlers{EventDispatcher: hook.NewManagementEventDispatcher(dispatcher.URL)}
	h.dispatchEvent(context.Background(), "credential_updated", "cred-1", map[string]any{
		"credential_value": "secret-value",
		"authorization":    "Bearer " + redactionSinkJWT,
		"status":           "disabled",
	})

	select {
	case event := <-got:
		body, err := json.Marshal(event.Payload)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "secret-value")
		assert.NotContains(t, string(body), redactionSinkJWT)
		assert.Contains(t, string(body), "[REDACTED]")
		assert.Contains(t, string(body), "disabled")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for management event")
	}
}
