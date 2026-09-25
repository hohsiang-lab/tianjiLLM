package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAISubscriptionSuccessCallbackAttributionUsesServingCredential(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer access-a":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
		case "Bearer access-b":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(strings.Join([]string{
				`data: {"type":"response.output_text.delta","response_id":"resp-test","delta":"hi"}`,
				``,
				`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n")))
		default:
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Callbacks = reg
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKeyOrgID, "org-a")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := cap.wait(t, 2*time.Second)
	require.NotNil(t, data.OpenAISubscription)
	assert.Equal(t, "cred-b", data.OpenAISubscription.CredentialID)
	assert.Equal(t, "cred-b", data.UpstreamTokenKey)
	assert.Equal(t, "openai", data.OpenAISubscription.Provider)
	assert.Equal(t, "org-a", data.OpenAISubscription.OrganizationID)
	assert.Equal(t, "success", data.OpenAISubscription.Status)
	assert.NotContains(t, data.APIKey, "access-b")
}

func TestOpenAISubscriptionChatUnsupportedDoesNotEmitSuccessCallback(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		http.Error(w, "unexpected upstream call", http.StatusInternalServerError)
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newOpenAISubscriptionEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Callbacks = reg
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ChatCompletion(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"type":"not_supported"`)
	assert.Zero(t, upstreamCalls)
	cap.assertNoLog(t)
}

func TestOpenAISubscriptionRefreshFailureAuditIsRedacted(t *testing.T) {
	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "Bearer refresh-secret rejected",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.Config.GeneralSettings.StoreAuditLogs = true
	m := store.mock()

	var got db.InsertAuditLogParams
	m.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		got = arg
		return db.AuditLog{}, nil
	}
	h.DB = m

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Equal(t, "refresh", got.Action)
	assert.Equal(t, "CredentialTable", got.TableName)
	assert.Equal(t, "cred-refresh", got.ObjectID)
	assert.Contains(t, string(got.UpdatedValues), `"status":"failure"`)
	assert.Contains(t, string(got.UpdatedValues), `"reason_code":"refresh_failed"`)
	assert.NotContains(t, string(got.UpdatedValues), "refresh-secret")
}

func TestOpenAISubscriptionDeleteAuditIsRedacted(t *testing.T) {
	m := newMockStore()
	cred := db.CredentialTable{
		CredentialID:    "cred-delete",
		CredentialType:  CredentialTypeOpenAISubscription,
		CredentialValue: "encrypted-secret",
		CredentialInfo:  []byte(`{"status":"active","email":"admin@example.com"}`),
	}
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		require.Equal(t, "cred-delete", id)
		return cred, nil
	}
	m.deleteCredentialFn = func(_ context.Context, id string) error {
		require.Equal(t, "cred-delete", id)
		return nil
	}
	var got db.InsertAuditLogParams
	m.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		got = arg
		return db.AuditLog{}, nil
	}
	h := &Handlers{DB: m, Config: &config.ProxyConfig{}}
	h.Config.GeneralSettings.StoreAuditLogs = true

	req := httptest.NewRequest(http.MethodDelete, "/credentials/delete/cred-delete", nil)
	req = withOpenAISubscriptionChiURLParam(req, "credential_id", "cred-delete")
	w := httptest.NewRecorder()

	h.CredentialDelete(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	require.Equal(t, http.StatusOK, w.Code, string(body))
	assert.Equal(t, "delete", got.Action)
	assert.Equal(t, "CredentialTable", got.TableName)
	assert.Equal(t, "cred-delete", got.ObjectID)
	assert.Contains(t, string(got.UpdatedValues), `"status":"success"`)
	assert.NotContains(t, string(got.UpdatedValues), "encrypted-secret")
}

func withOpenAISubscriptionChiURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
