package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func TestHandleOpenAIConnectDisabledBlocksBrowserAndControls(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)
	h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?scope=global", nil)
	addUISessionCookie(t, h, req)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.True(t, w.Header().Get("Location") == "", "disabled Connect must not redirect")
	connectURL, reason := h.openAIConnectURL(req)
	assert.Empty(t, connectURL)
	assert.Equal(t, "OpenAI Connect is disabled", reason)
	page := httptest.NewRecorder()
	require.NoError(t, pages.CredentialsPage(pages.CredentialsPageData{ConnectURL: connectURL, ConnectDisabledReason: reason}).Render(context.Background(), page))
	for _, control := range []string{"openai-device-auth-dialog", "openai-callback-url-dialog", "Browser callback fallback"} {
		assert.False(t, strings.Contains(page.Body.String(), control), "disabled control: %s", control)
	}
}

func TestHandleOpenAIDeviceStartDisabledHidesRetryAndFallback(t *testing.T) {
	server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
	h, router, _ := newOpenAIDeviceTestRouter(t, server)
	h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
	for _, htmx := range []bool{false, true} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
		addUISessionCookie(t, h, req)
		setDeviceCSRFBody(t, h, req, "global")
		if htmx {
			req.Header.Set("HX-Request", "true")
		}
		router.ServeHTTP(w, req)
		want := http.StatusServiceUnavailable
		if htmx {
			want = http.StatusOK
		}
		assert.Equal(t, want, w.Code)
		assert.True(t, strings.Contains(w.Body.String(), "OpenAI Connect is disabled"))
		for _, control := range []string{"/ui/openai/device/start", "/ui/openai/connect"} {
			assert.False(t, strings.Contains(w.Body.String(), control), "disabled control: %s", control)
		}
	}
	assert.Zero(t, len(server.Requests()))
}

func TestHandleOpenAIDeviceStatusDisabledHidesConnectControls(t *testing.T) {
	for _, state := range []openaioauth.DeviceAuthStatus{openaioauth.DeviceAuthStatusPending} {
		t.Run(string(state), func(t *testing.T) {
			server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
			h, router, _ := newOpenAIDeviceTestRouter(t, server)
			req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/status", nil)
			addUISessionCookie(t, h, req)
			setDeviceCSRFBody(t, h, req, "global")
			cookie, err := req.Cookie(sessionCookieName)
			require.NoError(t, err)
			ctx, err := h.getSessionManager().Load(context.Background(), cookie.Value)
			require.NoError(t, err)
			binding, err := h.openAIDeviceSessionBinding(req.WithContext(ctx))
			require.NoError(t, err)
			store := openaioauth.NewDeviceStore(h.Cache)
			record, err := store.Create(ctx, openaioauth.DeviceAuthRecord{
				DeviceAuthID: "status-disabled-device", UserCode: "status-disabled-code", SessionBinding: binding,
				TokenPollURL: server.DeviceTokenURL(), VerificationURI: server.DeviceVerificationURL(), RedirectURI: server.URL() + "/deviceauth/callback",
			}, time.Minute)
			require.NoError(t, err)
			record.Status = state
			require.NoError(t, store.Save(ctx, record))
			req.URL.RawQuery = url.Values{"flow_id": {record.FlowID}}.Encode()
			h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			for _, control := range []string{"/ui/openai/device/start", "/ui/openai/connect", "hx-trigger="} {
				assert.False(t, strings.Contains(w.Body.String(), control), "disabled control: %s", control)
			}
			assert.Zero(t, len(server.Requests()))
		})
	}
}

func TestHandleOpenAIConnect_RequiresSession(t *testing.T) {
	_, router := newOpenAIConnectTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?org_id=org_123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("Location"))
}

func TestHandleOpenAIConnect_StoresStateAndRedirects(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?org_id=org_123", nil)
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusSeeOther, w.Code)
	location := w.Header().Get("Location")
	require.NotEmpty(t, location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	raw, err := h.Cache.Get(context.Background(), openaioauth.CacheKey(state))
	require.NoError(t, err)
	require.NotNil(t, raw)
	var record openaioauth.StateRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	assert.Equal(t, "org_123", record.OrgID)
	assert.Equal(t, state, record.State)
	assert.Equal(t, "http://localhost:1455/auth/callback", record.RedirectURI)
	assert.Equal(t, openai.ChallengeFromVerifier(record.CodeVerifier), parsed.Query().Get("code_challenge"))
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	assert.Equal(t, "http://localhost:1455/auth/callback", parsed.Query().Get("redirect_uri"))
}

func TestHandleOpenAIConnect_UsesConfiguredRedirectURIAndStoresIt(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)
	h.Config.GeneralSettings.OpenAIOAuth.RedirectURI = "http://127.0.0.1:1455/auth/callback"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?org_id=org_123", nil)
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusSeeOther, w.Code)
	parsed, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:1455/auth/callback", parsed.Query().Get("redirect_uri"))

	state := parsed.Query().Get("state")
	raw, err := h.Cache.Get(context.Background(), openaioauth.CacheKey(state))
	require.NoError(t, err)
	var record openaioauth.StateRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	assert.Equal(t, "http://127.0.0.1:1455/auth/callback", record.RedirectURI)
}

func TestHandleOpenAIConnect_RejectsMissingOrgID(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect", nil)
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "org_id")
}

func TestHandleOpenAIConnect_GlobalScopeStoresEmptyOrgID(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?scope=global", nil)
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusSeeOther, w.Code)
	parsed, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	raw, err := h.Cache.Get(context.Background(), openaioauth.CacheKey(state))
	require.NoError(t, err)
	var record openaioauth.StateRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	assert.Empty(t, record.OrgID)
}

func TestHandleOpenAIConnect_UsesEndpointOverride(t *testing.T) {
	h, router := newOpenAIConnectTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/openai/connect?org_id=org_123", nil)
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusSeeOther, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, h.Config.GeneralSettings.OpenAIOAuth.AuthorizeURL)
	assert.NotContains(t, location, "auth.openai.com")
}

func TestHandleOpenAIDeviceStartRequiresPersistence(t *testing.T) {
	for _, htmx := range []bool{false, true} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
			h, router, _ := newOpenAIDeviceTestRouter(t, server)
			h.DB = nil
			req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
			addUISessionCookie(t, h, req)
			setDeviceCSRFBody(t, h, req, "global")
			want := http.StatusServiceUnavailable
			if htmx {
				req.Header.Set("HX-Request", "true")
				want = http.StatusOK // Start errors intentionally keep the retry form swappable.
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, want, w.Code)
			assert.True(t, strings.Contains(w.Body.String(), "credential persistence is offline"))
			assert.False(t, strings.Contains(w.Body.String(), "hx-trigger="))
			assert.Zero(t, len(server.Requests()), "offline persistence must prevent every provider request")
			assert.True(t, h.proxyLifecycleHandlers().DB == nil)
		})
	}
}

func TestHandleOpenAIDeviceStart_RendersCodeWithoutProviderIdentifier(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
	addUISessionCookie(t, h, req)
	setDeviceCSRFBody(t, h, req, "global")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	body := w.Body.String()
	assert.Contains(t, body, oauthServer.DeviceVerificationURL())
	assert.Contains(t, body, "ABCD-EFGH")
	assert.NotContains(t, body, "device-auth-id")
	assert.NotContains(t, body, "access-token")
}

func TestHandleOpenAIDeviceStartFallsBackToCallbackWhenUnavailable(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
	addUISessionCookie(t, h, req)
	setDeviceCSRFBody(t, h, req, "global")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Use browser callback instead")
	assert.Contains(t, w.Body.String(), "/ui/openai/connect?scope=global")
}

func TestHandleOpenAIDeviceStartHTMXFailureReturnsSwappableFragment(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
	req.Header.Set("HX-Request", "true")
	addUISessionCookie(t, h, req)
	setDeviceCSRFBody(t, h, req, "global")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Use browser callback instead")
	assert.Contains(t, w.Body.String(), "Try device login again")
	assert.NotContains(t, w.Body.String(), `id="openai-device-auth-dialog-content"`)
}

func TestOpenAIDeviceCSRFTokenIsSeparateAndSessionPersisted(t *testing.T) {
	h := newTestHandler(t)
	firstRequest := httptest.NewRequest(http.MethodGet, "/ui/credentials", nil)
	setAdminSession(t, firstRequest, h)

	var firstToken, firstBinding string
	middleware := h.getSessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		firstToken, err = h.openAIDeviceCSRFToken(r)
		require.NoError(t, err)
		firstBinding, err = h.openAIDeviceSessionBinding(r)
		require.NoError(t, err)
	}))
	firstResponse := httptest.NewRecorder()
	middleware.ServeHTTP(firstResponse, firstRequest)
	require.NotEmpty(t, firstToken)
	assert.NotEqual(t, firstToken, firstBinding)

	secondRequest := httptest.NewRequest(http.MethodGet, "/ui/credentials", nil)
	for _, cookie := range firstResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			secondRequest.AddCookie(cookie)
		}
	}
	var secondToken string
	secondMiddleware := h.getSessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		secondToken, err = h.openAIDeviceCSRFToken(r)
		require.NoError(t, err)
	}))
	secondMiddleware.ServeHTTP(httptest.NewRecorder(), secondRequest)
	assert.Equal(t, firstToken, secondToken)
}

func TestOpenAIDeviceStartRequiresCSRF(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", strings.NewReader("scope=global"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "form has expired")
	assert.Empty(t, countOpenAIDeviceRequests(oauthServer.Requests()))
}

func TestHandleOpenAICallbackURLRejectsMissingCSRF(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/callback-url", strings.NewReader("callback_url=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUISessionCookie(t, h, req)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "invalid form session")
	assert.Zero(t, countOpenAIDeviceRequests(oauthServer.Requests()))
}

func TestOpenAIDeviceViewsObserveExpiry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		interval  int64
		remaining time.Duration
		want      int64
	}{
		{"near_expiry", 90, 2 * time.Second, 2},
		{"round_up", 120, 1500 * time.Millisecond, 2},
		{"subsecond", 90, 500 * time.Millisecond, 1},
		{"expired", 90, -time.Second, 1},
		{"ordinary", 5, time.Minute, 5},
		{"default", 0, time.Minute, 5},
		{"long_supported", 120, 5 * time.Minute, 120},
	} {
		for _, phase := range []string{"start", "status"} {
			t.Run(tc.name+"/"+phase, func(t *testing.T) {
				h, _ := newOpenAIConnectTestRouter(t)
				expiry := time.Now().Add(tc.remaining)
				var view pages.OpenAIDeviceAuthView
				if phase == "start" {
					view = openAIDeviceStartView(proxyhandler.OpenAIDeviceAuthStart{FlowID: "flow-interval", IntervalSeconds: tc.interval, ExpiresAt: expiry})
				} else {
					view = h.openAIDeviceStatusView(proxyhandler.OpenAIDeviceAuthStatus{FlowID: "flow-interval", IntervalSeconds: tc.interval, ExpiresAt: expiry, Status: openaioauth.DeviceAuthStatusPending})
				}
				var body strings.Builder
				if phase == "start" {
					require.NoError(t, pages.OpenAIDeviceAuthStarted(view).Render(context.Background(), &body))
				} else {
					require.NoError(t, pages.OpenAIDeviceAuthStatus(view).Render(context.Background(), &body))
				}
				trigger := regexp.MustCompile(`hx-trigger="([^"]+)"`).FindStringSubmatch(body.String())
				require.Len(t, trigger, 2)
				assert.Equal(t, fmt.Sprintf("every %ds", tc.want), trigger[1], "interval_expiry_observation")
			})
		}
	}
}

func TestHandleOpenAIDeviceStatusUsesSessionOwnershipAndProviderInterval(t *testing.T) {
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
	h, router, _ := newOpenAIDeviceTestRouter(t, oauthServer)

	startW := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start", nil)
	addUISessionCookie(t, h, startReq)
	setDeviceCSRFBody(t, h, startReq, "global")
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)
	flowID := regexp.MustCompile(`flow_id=([^&"]+)`).FindStringSubmatch(startW.Body.String())
	require.Len(t, flowID, 2)

	statusW := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodPost, "/ui/openai/device/status?flow_id="+flowID[1], nil)
	for _, cookie := range startW.Result().Cookies() {
		statusReq.AddCookie(cookie)
	}
	setDeviceCSRFBody(t, h, statusReq, "global")
	router.ServeHTTP(statusW, statusReq)

	require.Equal(t, http.StatusOK, statusW.Code)
	assert.Equal(t, "no-store", statusW.Header().Get("Cache-Control"))
	assert.Contains(t, statusW.Body.String(), "Waiting for OpenAI authorization")
	assert.Contains(t, statusW.Body.String(), `hx-post="/ui/openai/device/status?flow_id=`)
	assert.Zero(t, countOpenAIDeviceTokenRequests(oauthServer.Requests()))
}

func TestHandleOpenAIDeviceStatusHTMXRecoversTerminalSave(t *testing.T) {
	server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{
		DeviceAuth: &openaitest.DeviceAuthOptions{PollResponses: []openaitest.DevicePollResponse{{Response: map[string]any{
			"authorization_code": "ui-code", "code_verifier": "ui-verifier", "code_challenge": openai.ChallengeFromVerifier("ui-verifier"),
		}}}},
		TokenFixtures: []openaitest.TokenFixture{{Code: "ui-code", Response: map[string]any{
			"access_token": "ui-access", "refresh_token": "ui-refresh", "expires_in": 3600, "account_id": "ui-account",
		}}},
	})
	h, router, persistence := newOpenAIDeviceTestRouter(t, server)
	sharedCache, ok := h.Cache.(uiTestSharedMemoryCache)
	require.True(t, ok)
	faults := &uiDeviceFaultCache{uiTestSharedMemoryCache: sharedCache, failSuccessSave: true}
	h.Cache = faults
	start, record := startOpenAIDeviceTestFlow(t, h, router, "org-ui-flow")
	persistence.createRows = pgxmock.NewRows(credentialColumns())
	persistence.ExpectQuery(`INSERT INTO "CredentialTable"`).
		WithArgs(pgxmock.AnyArg(), "OpenAI Subscription", openAISubscriptionCredentialType, pgxmock.AnyArg(), pgxmock.AnyArg(), &record.OrgID, "").
		WillReturnRows(persistence.createRows)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, openAIDeviceFlowRequest(start, record.FlowID, "status"))
	assert.Equal(t, http.StatusServiceUnavailable, first.Code, "transport errors must not swap the original polling DOM")
	assert.Equal(t, "no-store", first.Header().Get("Cache-Control"))
	for _, fragment := range []string{"Device login failed", "scope=global", "<form", "openai-device-auth-status"} {
		assert.False(t, strings.Contains(first.Body.String(), fragment), "must not manufacture %s", fragment)
	}
	require.Equal(t, 1, faults.failures)
	require.Equal(t, 1, persistence.createCalls)
	require.NoError(t, persistence.ExpectationsWereMet(), "credential insert must succeed before reconciliation is configured")
	require.Len(t, persistence.created, 7)
	credentialID, ok := persistence.created[0].(string)
	require.True(t, ok)
	require.NotEmpty(t, credentialID)
	credentialValue, ok := persistence.created[3].(string)
	require.True(t, ok)
	assert.True(t, credentialValue != "" && !strings.Contains(credentialValue, "ui-access"), "credential must be encrypted")
	organizationID, ok := persistence.created[5].(*string)
	require.True(t, ok)
	require.NotNil(t, organizationID)
	assert.Equal(t, record.OrgID, *organizationID)
	stored, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusExchanging, stored.Status)
	assert.Equal(t, record.OrgID, stored.OrgID)
	assert.Empty(t, stored.CredentialID)

	// Only the row captured from the successful insert can be reconciled.
	persistence.readRows = true
	persistence.ExpectQuery(`SELECT .+ FROM "CredentialTable" WHERE credential_id = \$1`).
		WithArgs(credentialID).WillReturnRows(pgxmock.NewRows(credentialColumns()).AddRow(
		persistence.created[0], persistence.created[1], persistence.created[2], persistence.created[3], persistence.created[4], persistence.created[5],
		pgtype.Timestamptz{}, "", pgtype.Timestamptz{}, ""))
	second := httptest.NewRecorder()
	router.ServeHTTP(second, openAIDeviceFlowRequest(start, record.FlowID, "status"))
	require.Equal(t, http.StatusOK, second.Code)
	assert.True(t, strings.Contains(second.Body.String(), "OpenAI credential connected."))
	assert.False(t, strings.Contains(second.Body.String(), "hx-trigger="))
	final, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, final.Status)
	assert.Equal(t, record.FlowID, final.FlowID)
	assert.Equal(t, record.OrgID, final.OrgID)
	assert.Equal(t, record.SessionBinding, final.SessionBinding)
	assert.True(t, record.ExpiresAt.Equal(final.ExpiresAt))
	assert.Equal(t, credentialID, final.CredentialID)
	assert.Equal(t, 1, persistence.createCalls)
	assert.Equal(t, 1, countOpenAIDeviceTokenRequests(server.Requests()))
	assert.Equal(t, 3, len(server.Requests()), "one start, poll and exchange; no replay")
	for _, secret := range []string{"device-auth-id", "ui-code", "ui-verifier", "ui-access", "ui-refresh"} {
		assert.False(t, strings.Contains(first.Body.String()+second.Body.String(), secret), "response must stay display-safe")
	}
}

func TestHandleOpenAIDeviceCancelHTMXPreservesFlowOnError(t *testing.T) {
	for _, tc := range []struct {
		name            string
		exchanging      bool
		failRead        bool
		failAfterCancel bool
		wantHTTP        int
		wantState       openaioauth.DeviceAuthStatus
	}{
		{"exchange_conflict", true, false, false, http.StatusConflict, openaioauth.DeviceAuthStatusExchanging},
		{"service_read", false, true, false, http.StatusServiceUnavailable, openaioauth.DeviceAuthStatusPending},
		{"cancelled_followup_read", false, false, true, http.StatusServiceUnavailable, openaioauth.DeviceAuthStatusCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
			h, router, persistence := newOpenAIDeviceTestRouter(t, server)
			sharedCache, ok := h.Cache.(uiTestSharedMemoryCache)
			require.True(t, ok)
			faults := &uiDeviceFaultCache{uiTestSharedMemoryCache: sharedCache}
			h.Cache = faults
			start, record := startOpenAIDeviceTestFlow(t, h, router, "org-ui-flow")
			store := openaioauth.NewDeviceStore(h.Cache)
			if tc.exchanging {
				record.Status = openaioauth.DeviceAuthStatusExchanging
				record.Generation = "attempt:ui-fixture"
				require.NoError(t, store.Save(context.Background(), record))
			}
			before, err := store.Get(context.Background(), record.FlowID)
			require.NoError(t, err)
			faults.failNextGet = tc.failRead
			faults.failReadAfterCancel = tc.failAfterCancel
			providerRequests := len(server.Requests())
			first := httptest.NewRecorder()
			router.ServeHTTP(first, openAIDeviceFlowRequest(start, record.FlowID, "cancel"))

			assert.Equal(t, tc.wantHTTP, first.Code, "uncertain cancellation must not swap the original flow")
			assert.Equal(t, "no-store", first.Header().Get("Cache-Control"))
			for _, fragment := range []string{"<form", "scope=global", "Device login cancelled", "Device login failed", "openai-device-auth-status"} {
				assert.False(t, strings.Contains(first.Body.String(), fragment), "must not manufacture %s", fragment)
			}
			after, err := store.Get(context.Background(), record.FlowID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantState, after.Status)
			assert.Equal(t, record.OrgID, after.OrgID)
			assert.True(t, after.OwnedBy(record.SessionBinding))
			assert.True(t, record.ExpiresAt.Equal(after.ExpiresAt))
			if !tc.failAfterCancel {
				assert.True(t, before == after, "service failure must not mutate the flow")
			}
			assert.Equal(t, providerRequests, len(server.Requests()))
			assert.Zero(t, persistence.createCalls)

			if tc.wantState != openaioauth.DeviceAuthStatusPending {
				persistence.readRows = true
				persistence.ExpectQuery(`SELECT .+ FROM "CredentialTable" WHERE credential_id = \$1`).
					WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
			}
			retry := httptest.NewRecorder()
			router.ServeHTTP(retry, openAIDeviceFlowRequest(start, record.FlowID, "status"))
			require.Equal(t, http.StatusOK, retry.Code)
			body := retry.Body.String()
			if tc.failAfterCancel {
				assert.True(t, strings.Contains(body, "Device login cancelled."))
				assert.True(t, strings.Contains(body, `name="org_id" value="`+record.OrgID+`"`))
				assert.False(t, strings.Contains(body, "hx-trigger="))
			} else {
				assert.True(t, strings.Contains(body, `hx-post="/ui/openai/device/status?flow_id=`+record.FlowID+`"`))
				assert.True(t, strings.Contains(body, "hx-trigger="))
			}
			assert.True(t, strings.Contains(body, start.FormValue("csrf_token")))
			assert.False(t, strings.Contains(body, `name="scope" value="global"`))
			assert.Zero(t, persistence.createCalls)
		})
	}
}

func TestOpenAIDeviceScopeMatchesStoredFlow(t *testing.T) {
	for _, action := range []string{"status", "cancel"} {
		for _, tc := range []struct {
			name, orgID, body, query string
			allowed                  bool
		}{
			{"org_omitted", "org-ui-flow", "", "", true},
			{"org_matching_body", "org-ui-flow", "org_id=org-ui-flow", "", true},
			{"org_matching_alias_query", "org-ui-flow", "", "organization_id=org-ui-flow", true},
			{"org_matching_duplicates", "org-ui-flow", "org_id=org-ui-flow&org_id=org-ui-flow", "organization_id=org-ui-flow", true},
			{"global_omitted", "", "", "", true},
			{"global_matching", "", "scope=global", "scope=global", true},
			{"org_mismatch_body", "org-ui-flow", "org_id=other", "", false},
			{"org_mismatch_query", "org-ui-flow", "", "org_id=other", false},
			{"alias_mismatch_body", "org-ui-flow", "organization_id=other", "", false},
			{"alias_mismatch_query", "org-ui-flow", "", "organization_id=other", false},
			{"org_to_global", "org-ui-flow", "scope=global", "", false},
			{"org_to_global_query", "org-ui-flow", "", "scope=global", false},
			{"global_to_org", "", "org_id=org-ui-flow", "", false},
			{"global_to_alias", "", "scope=global", "organization_id=org-ui-flow", false},
			{"duplicate_org", "org-ui-flow", "org_id=org-ui-flow&org_id=other", "", false},
			{"duplicate_org_reversed", "org-ui-flow", "org_id=other&org_id=org-ui-flow", "", false},
			{"duplicate_alias", "org-ui-flow", "", "organization_id=org-ui-flow&organization_id=other", false},
			{"body_query_conflict", "org-ui-flow", "org_id=org-ui-flow", "org_id=other", false},
			{"query_body_conflict", "org-ui-flow", "org_id=other", "org_id=org-ui-flow", false},
			{"alias_conflict", "org-ui-flow", "org_id=org-ui-flow&organization_id=other", "", false},
			{"alias_conflict_reversed", "org-ui-flow", "org_id=other&organization_id=org-ui-flow", "", false},
			{"duplicate_scope", "org-ui-flow", "scope=&scope=global", "", false},
			{"unsupported_scope", "", "scope=global", "scope=other", false},
			{"org_whitespace", "org-ui-flow", "org_id=+org-ui-flow+", "", false},
			{"org_case", "org-ui-flow", "", "organization_id=ORG-UI-FLOW", false},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{
					DeviceAuth: &openaitest.DeviceAuthOptions{PollResponses: []openaitest.DevicePollResponse{{Response: map[string]any{
						"authorization_code": "scope-code", "code_verifier": "scope-verifier", "code_challenge": openai.ChallengeFromVerifier("scope-verifier"),
					}}}},
					TokenFixtures: []openaitest.TokenFixture{{Code: "scope-code", Response: map[string]any{
						"access_token": "scope-access", "refresh_token": "scope-refresh", "expires_in": 3600, "account_id": "scope-account",
					}}},
				})
				h, router, persistence := newOpenAIDeviceTestRouter(t, server)
				start, record := startOpenAIDeviceTestFlow(t, h, router, tc.orgID)
				require.True(t, time.Now().Before(record.ExpiresAt) && !time.Now().Before(record.NextPollAt))
				var orgID *string
				if record.OrgID != "" {
					orgID = &record.OrgID
				}
				if action == "status" {
					persistence.createRows = pgxmock.NewRows(credentialColumns())
					persistence.ExpectQuery(`INSERT INTO "CredentialTable"`).
						WithArgs(pgxmock.AnyArg(), "OpenAI Subscription", openAISubscriptionCredentialType, pgxmock.AnyArg(), pgxmock.AnyArg(), orgID, "").
						WillReturnRows(persistence.createRows)
				}
				form, err := url.ParseQuery(tc.body)
				require.NoError(t, err)
				form.Set("flow_id", record.FlowID)
				form.Set("csrf_token", start.FormValue("csrf_token"))
				req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/"+action+"?"+tc.query, strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Cookie", start.Header.Get("Cookie"))
				req.Header.Set("HX-Request", "true")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				store := openaioauth.NewDeviceStore(h.Cache)
				if !tc.allowed {
					assert.Equal(t, http.StatusForbidden, w.Code, "explicit scope mismatch must be rejected before operation")
					assert.Len(t, server.Requests(), 1, "only start; no poll or exchange")
					assert.Zero(t, persistence.createCalls)
					after, getErr := store.Get(context.Background(), record.FlowID)
					require.NoError(t, getErr)
					assert.True(t, record == after, "scope rejection must preserve the exact flow")
					if w.Code != http.StatusForbidden {
						return
					}
					// Same eligible flow and session must reach the requested operation.
					w = httptest.NewRecorder()
					allowed := openAIDeviceFlowRequest(start, record.FlowID, action)
					allowed.URL.RawQuery = "scope=global"
					if record.OrgID != "" {
						allowed.URL.RawQuery = "org_id=" + url.QueryEscape(record.OrgID)
					}
					router.ServeHTTP(w, allowed)
				}
				require.Equal(t, http.StatusOK, w.Code)
				after, err := store.Get(context.Background(), record.FlowID)
				require.NoError(t, err)
				assert.Equal(t, record.OrgID, after.OrgID)
				assert.True(t, after.OwnedBy(record.SessionBinding) && after.ExpiresAt.Equal(record.ExpiresAt))
				if action == "status" {
					assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, after.Status)
					assert.Equal(t, 1, countOpenAIDeviceTokenRequests(server.Requests()))
					assert.Len(t, server.Requests(), 3, "start, poll, exchange")
					require.Equal(t, 1, persistence.createCalls)
					assert.Equal(t, orgID, persistence.created[5])
				} else {
					assert.Equal(t, openaioauth.DeviceAuthStatusCancelled, after.Status)
					assert.Len(t, server.Requests(), 1)
					assert.Zero(t, persistence.createCalls)
				}
			})
		}
	}
}

func TestOpenAIDeviceOwnedFlowRequiresCSRF(t *testing.T) {
	for _, action := range []string{"status", "cancel"} {
		for _, token := range []string{"", "wrong"} {
			for _, htmx := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/token=%q/hx=%t", action, token, htmx), func(t *testing.T) {
					server := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{DeviceAuth: &openaitest.DeviceAuthOptions{}})
					h, router, persistence := newOpenAIDeviceTestRouter(t, server)
					start, record := startOpenAIDeviceTestFlow(t, h, router, "org-ui-flow")
					require.Equal(t, openaioauth.DeviceAuthStatusPending, record.Status)
					require.True(t, time.Now().Before(record.ExpiresAt))
					providerRequests := len(server.Requests())
					form := url.Values{"flow_id": {record.FlowID}}
					if token != "" {
						form.Set("csrf_token", token)
					}
					req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/"+action, strings.NewReader(form.Encode()))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					req.Header.Set("Cookie", start.Header.Get("Cookie"))
					if htmx {
						req.Header.Set("HX-Request", "true")
					}
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					assert.Equal(t, http.StatusForbidden, w.Code)
					assert.True(t, strings.Contains(w.Body.String(), "invalid form session"))
					assert.False(t, strings.Contains(w.Body.String(), "<form"))
					assert.Equal(t, providerRequests, len(server.Requests()))
					assert.Zero(t, persistence.createCalls)
					after, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), record.FlowID)
					require.NoError(t, err)
					assert.True(t, record == after, "CSRF rejection must preserve the exact owned flow")
					assert.Empty(t, after.CredentialID)

					// The same session and flow with valid CSRF reaches the provider.
					allowed := httptest.NewRecorder()
					router.ServeHTTP(allowed, openAIDeviceFlowRequest(start, record.FlowID, "status"))
					require.Equal(t, http.StatusOK, allowed.Code)
					assert.Equal(t, providerRequests+1, len(server.Requests()))
					assert.Zero(t, persistence.createCalls)
				})
			}
		}
	}
}

func startOpenAIDeviceTestFlow(t *testing.T, h *UIHandler, router chi.Router, orgID string) (*http.Request, openaioauth.DeviceAuthRecord) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/start?org_id="+url.QueryEscape(orgID), nil)
	addUISessionCookie(t, h, req)
	scope := ""
	if orgID == "" {
		scope = "global"
	}
	setDeviceCSRFBody(t, h, req, scope)
	require.NoError(t, req.ParseForm())
	require.NotEmpty(t, req.FormValue("csrf_token"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	match := regexp.MustCompile(`flow_id=([^&"]+)`).FindStringSubmatch(w.Body.String())
	require.Len(t, match, 2)
	require.True(t, strings.Contains(w.Body.String(), "hx-trigger="))
	require.True(t, strings.Contains(w.Body.String(), req.FormValue("csrf_token")))
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), match[1])
	require.NoError(t, err)
	require.Equal(t, orgID, record.OrgID)
	record.NextPollAt = time.Now().Add(-time.Second)
	require.NoError(t, store.Save(context.Background(), record))
	// Read back the exact timestamped flow used by the routed request.
	record, err = store.Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	return req, record
}

func openAIDeviceFlowRequest(start *http.Request, flowID, action string) *http.Request {
	form := url.Values{"flow_id": {flowID}, "csrf_token": {start.FormValue("csrf_token")}}
	req := httptest.NewRequest(http.MethodPost, "/ui/openai/device/"+action, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", start.Header.Get("Cookie"))
	req.Header.Set("HX-Request", "true")
	return req
}

// Capture actual SQL arguments while still exercising sqlc and pgxmock's Scan.
type uiDeviceTestDB struct {
	pgxmock.PgxPoolIface
	created     []any
	createCalls int
	createRows  *pgxmock.Rows
	readRows    bool
}

func (d *uiDeviceTestDB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, `FROM "CredentialTable"`) && !d.readRows {
		return uiDeviceAbsentRow{}
	}
	if strings.Contains(query, `INSERT INTO "CredentialTable"`) {
		d.createCalls++
		d.created = append([]any(nil), args...)
		if d.createRows != nil {
			d.createRows.AddRow(args[0], args[1], args[2], args[3], args[4], args[5], pgtype.Timestamptz{}, args[6], pgtype.Timestamptz{}, args[6])
		}
	}
	return d.PgxPoolIface.QueryRow(ctx, query, args...)
}

type uiDeviceAbsentRow struct{}

func (uiDeviceAbsentRow) Scan(...any) error { return pgx.ErrNoRows }

func (d *uiDeviceTestDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	d.ExpectBeginTx(opts)
	tx, err := d.PgxPoolIface.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &uiDeviceTestTx{Tx: tx, db: d}, nil
}

type uiDeviceTestTx struct {
	pgx.Tx
	db     *uiDeviceTestDB
	closed bool
}

func (tx *uiDeviceTestTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tx.db.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(args...).WillReturnResult(pgconn.NewCommandTag("SELECT 1"))
	return tx.Tx.Exec(ctx, query, args...)
}
func (tx *uiDeviceTestTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return tx.db.QueryRow(ctx, query, args...)
}
func (tx *uiDeviceTestTx) Commit(ctx context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.closed = true
	tx.db.ExpectCommit()
	return tx.Tx.Commit(ctx)
}
func (tx *uiDeviceTestTx) Rollback(ctx context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.closed = true
	tx.db.ExpectRollback()
	return tx.Tx.Rollback(ctx)
}

type uiDeviceFaultCache struct {
	uiTestSharedMemoryCache
	failSuccessSave     bool
	failNextGet         bool
	failReadAfterCancel bool
	failures            int
}

func (c *uiDeviceFaultCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	if c.failNextGet {
		c.failNextGet = false
		c.failures++
		return nil, errors.New("flow cache read unavailable")
	}
	return c.MemoryCache.GetShared(ctx, key)
}

func (c *uiDeviceFaultCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.MemoryCache.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	var record openaioauth.DeviceAuthRecord
	if c.failReadAfterCancel && json.Unmarshal(value, &record) == nil && record.Status == openaioauth.DeviceAuthStatusCancelled {
		c.failReadAfterCancel = false
		c.failNextGet = true
	}
	return nil
}

func (c *uiDeviceFaultCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	var record openaioauth.DeviceAuthRecord
	if c.failSuccessSave && json.Unmarshal(value, &record) == nil && record.Status == openaioauth.DeviceAuthStatusSuccess {
		c.failSuccessSave = false
		c.failures++
		return false, errors.New("terminal cache write unavailable")
	}
	applied, err := c.MemoryCache.CompareAndSet(ctx, key, expected, value, ttl)
	if err == nil && applied && c.failReadAfterCancel && json.Unmarshal(value, &record) == nil && record.Status == openaioauth.DeviceAuthStatusCancelled {
		c.failReadAfterCancel = false
		c.failNextGet = true
	}
	return applied, err
}

func newOpenAIConnectTestRouter(t *testing.T) (*UIHandler, chi.Router) {
	t.Helper()
	oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	cfg := &config.ProxyConfig{}
	cfg.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	cfg.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		Enabled:      true,
		AuthorizeURL: oauthServer.AuthorizeURL(),
		TokenURL:     oauthServer.TokenURL(),
		ClientID:     "app_test",
	}
	h := &UIHandler{
		Config:    cfg,
		MasterKey: cfg.GeneralSettings.MasterKey,
		Cache:     cache.NewMemoryCache(),
	}
	router := chi.NewRouter()
	router.Route("/ui", func(r chi.Router) {
		h.RegisterRoutes(r)
	})
	return h, router
}

type uiTestSharedMemoryCache struct{ *cache.MemoryCache }

func (uiTestSharedMemoryCache) SharedCoordinationAvailable() bool { return true }

func newOpenAIDeviceTestRouter(t *testing.T, oauthServer *openaitest.OAuthServer) (*UIHandler, chi.Router, *uiDeviceTestDB) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	mock := &uiDeviceTestDB{PgxPoolIface: pool}
	pool.MatchExpectationsInOrder(false)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	cfg := &config.ProxyConfig{}
	cfg.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	cfg.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		Enabled:   true,
		IssuerURL: oauthServer.URL(),
		TokenURL:  oauthServer.TokenURL(),
		ClientID:  "app_test",
	}
	h := &UIHandler{
		DB:                    db.New(mock),
		Config:                cfg,
		MasterKey:             cfg.GeneralSettings.MasterKey,
		Cache:                 uiTestSharedMemoryCache{MemoryCache: cache.NewMemoryCache()},
		OpenAIOAuthHTTPClient: oauthServer.Client(),
	}
	router := chi.NewRouter()
	router.Route("/ui", func(r chi.Router) {
		h.RegisterRoutes(r)
	})
	return h, router, mock
}

func countOpenAIDeviceRequests(requests []openaitest.RecordedRequest) int {
	count := 0
	for _, request := range requests {
		if strings.HasPrefix(request.Path, "/api/accounts/deviceauth/") {
			count++
		}
	}
	return count
}

func countOpenAIDeviceTokenRequests(requests []openaitest.RecordedRequest) int {
	count := 0
	for _, request := range requests {
		if request.Path == "/api/accounts/deviceauth/token" {
			count++
		}
	}
	return count
}

func setDeviceCSRFBody(t *testing.T, h *UIHandler, req *http.Request, scope string) {
	t.Helper()
	cookie, err := req.Cookie(sessionCookieName)
	require.NoError(t, err)
	manager := h.getSessionManager()
	loadedContext, err := manager.Load(context.Background(), cookie.Value)
	require.NoError(t, err)
	csrfToken, err := h.openAIDeviceCSRFToken(req.WithContext(loadedContext))
	require.NoError(t, err)
	sessionToken, _, err := manager.Commit(loadedContext)
	require.NoError(t, err)
	req.Header.Set("Cookie", (&http.Cookie{Name: sessionCookieName, Value: sessionToken}).String())
	body := url.Values{"scope": {scope}, "csrf_token": {csrfToken}}.Encode()
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

func addUISessionCookie(t *testing.T, h *UIHandler, req *http.Request) {
	t.Helper()
	setAdminSession(t, req, h)
}
