//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

const credentialsSecretJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJjcmVkZW50aWFsIn0.Secr3tSecr3tSecr3t"

func TestCredentialsList_SidebarNavigationAndSubscriptionFilter(t *testing.T) {
	f := setup(t)
	f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-openai-a",
		Name: "OpenAI primary",
		Info: map[string]any{"email": "primary@example.com", "status": "active"},
	})
	f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-openai-b",
		Name: "OpenAI backup",
		Info: map[string]any{"email": "backup@example.com", "status": "refresh_failed"},
	})
	f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-api-key",
		Name: "Generic key",
		Type: "api_key",
		Info: map[string]any{"email": "generic@example.com", "status": "active"},
	})

	_, err := f.Page.Goto(testServer.URL + "/ui/")
	require.NoError(t, err)
	require.NoError(t, f.Page.WaitForLoadState())
	require.NoError(t, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name:  "Credentials",
		Exact: playwright.Bool(true),
	}).First().Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/credentials"))
	f.WaitStable()

	body := f.Text("body")
	assert.Contains(t, body, "Credentials")
	assert.Contains(t, body, "OpenAI primary")
	assert.Contains(t, body, "primary@example.com")
	assert.Contains(t, body, "OpenAI backup")
	assert.Contains(t, body, "backup@example.com")
	assert.NotContains(t, body, "Generic key")
	assert.NotContains(t, body, "generic@example.com")
}

func TestCredentialsList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToCredentials()

	assert.Contains(t, f.Text("body"), "No OpenAI subscription credentials found")
}

func TestCredentialDetail_NavigateFromListAndMetadata(t *testing.T) {
	f := setup(t)
	lastRefresh := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "credentials-detail-org"})
	credID := f.SeedCredential(SeedCredentialOpts{
		ID:             "cred-detail",
		Name:           "OpenAI detail",
		OrganizationID: orgID,
		Info: map[string]any{
			"email":           "detail@example.com",
			"status":          "disabled",
			"last_refresh_at": lastRefresh,
			"last_error":      "refresh failed with redacted upstream error",
			"disabled_reason": "manual operator disable",
		},
	})
	f.NavigateToCredentials()

	require.NoError(t, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: "OpenAI detail",
	}).First().Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/credentials/"+credID))
	f.WaitStable()

	body := f.Text("body")
	assert.Contains(t, body, "OpenAI detail")
	assert.Contains(t, body, "detail@example.com")
	assert.Contains(t, body, "Disabled")
	assert.Contains(t, body, "refresh failed with redacted upstream error")
	assert.Contains(t, body, "manual operator disable")
	assert.Contains(t, body, orgID)
}

func TestCredentialDetail_QuotaStatesAndUnknownFallback(t *testing.T) {
	f := setup(t)
	now := time.Now().UTC()
	allowedID := f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-quota-allowed",
		Name: "OpenAI quota allowed",
		Info: map[string]any{"email": "quota@example.com", "status": "active"},
	})
	f.SeedOpenAIQuotaState(allowedID, callback.OpenAIQuotaState{
		Status: callback.OpenAIQuotaStatusAllowed,
		Requests: callback.OpenAIQuotaDimension{
			Limit: 1000, LimitKnown: true,
			Remaining: 250, RemainingKnown: true,
			ResetAt: now.Add(30 * time.Minute), ResetKnown: true,
			Utilization: 0.75, UtilizationKnown: true,
		},
		Tokens: callback.OpenAIQuotaDimension{
			Limit: 200000, LimitKnown: true,
			Remaining: 50000, RemainingKnown: true,
			ResetAt: now.Add(45 * time.Minute), ResetKnown: true,
			Utilization: 0.75, UtilizationKnown: true,
		},
		UpdatedAt: now,
	})
	rejectedID := f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-quota-rejected",
		Name: "OpenAI quota rejected",
		Info: map[string]any{"email": "rejected@example.com", "status": "active"},
	})
	f.SeedOpenAIQuotaState(rejectedID, callback.OpenAIQuotaState{
		Status:          callback.OpenAIQuotaStatusRejected,
		QuotaResetAt:    now.Add(time.Hour),
		QuotaResetKnown: true,
		UpdatedAt:       now,
	})
	exhaustedID := f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-quota-exhausted",
		Name: "OpenAI quota exhausted",
		Info: map[string]any{"email": "exhausted@example.com", "status": "active"},
	})
	f.SeedOpenAIQuotaState(exhaustedID, callback.OpenAIQuotaState{
		Status:    callback.OpenAIQuotaStatusExhausted,
		UpdatedAt: now,
	})
	unknownID := f.SeedCredential(SeedCredentialOpts{
		ID:   "cred-quota-unknown",
		Name: "OpenAI quota unknown",
		Info: map[string]any{"email": "unknown@example.com", "status": "active"},
	})

	f.NavigateToCredentialDetail(allowedID)
	body := f.Text("body")
	assert.Contains(t, body, "Allowed")
	assert.Contains(t, body, "Requests")
	assert.Contains(t, body, "Tokens")
	assert.Contains(t, body, "1,000")
	assert.Contains(t, body, "250")
	assert.Contains(t, body, "200,000")
	assert.Contains(t, body, "50,000")
	assert.Contains(t, body, "75.0%")

	f.NavigateToCredentialDetail(rejectedID)
	assert.Contains(t, f.Text("body"), "Rejected")

	f.NavigateToCredentialDetail(exhaustedID)
	assert.Contains(t, f.Text("body"), "Exhausted")

	f.NavigateToCredentialDetail(unknownID)
	body = f.Text("body")
	assert.Contains(t, body, "Unknown")
	assert.Contains(t, body, "No upstream quota data recorded yet")
}

func TestCredentialDetail_SecretMaterialNotRendered(t *testing.T) {
	f := setup(t)
	secretAPIKey := "sk-testsecret1234567890"
	secretBearer := "Bearer " + credentialsSecretJWT
	f.SeedCredential(SeedCredentialOpts{
		ID:    "cred-secret-boundary",
		Name:  "Secret boundary",
		Value: "encrypted-value-containing-" + secretAPIKey,
		Info: map[string]any{
			"email":           "safe@example.com",
			"status":          "refresh_failed",
			"last_error":      "upstream returned " + secretBearer,
			"disabled_reason": "api_key=" + secretAPIKey,
			"access_token":    "access-token-secret",
			"refresh_token":   "refresh-token-secret",
			"id_token":        credentialsSecretJWT,
		},
	})

	f.NavigateToCredentialDetail("cred-secret-boundary")

	html, err := f.Page.Locator("body").InnerHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "safe@example.com")
	assert.Contains(t, html, "[REDACTED]")
	assert.NotContains(t, html, secretAPIKey)
	assert.NotContains(t, html, credentialsSecretJWT)
	assert.NotContains(t, html, "Bearer")
	assert.NotContains(t, html, "access-token-secret")
	assert.NotContains(t, html, "refresh-token-secret")
	assert.NotContains(t, html, "credential_value")
	assert.NotContains(t, html, "encrypted-value-containing")
}

func TestCredentialActions_ListLifecycleHappyPaths(t *testing.T) {
	f := setup(t)
	modelsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-token-actions", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4.1"}]}`))
	}))
	t.Cleanup(modelsServer.Close)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh-token-actions", r.FormValue("refresh_token"))
		_, _ = w.Write([]byte(`{"access_token":"access-token-refreshed","refresh_token":"refresh-token-refreshed","expires_in":3600,"account_id":"acct-actions"}`))
	}))
	t.Cleanup(tokenServer.Close)
	uiHandler.OpenAIUpstreamBaseURL = modelsServer.URL
	uiHandler.OpenAIUpstreamHTTPClient = modelsServer.Client()
	uiHandler.OpenAIOAuthHTTPClient = tokenServer.Client()
	cfg.GeneralSettings.OpenAIOAuth.TokenURL = tokenServer.URL

	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-actions",
		Name: "OpenAI actions",
		Info: map[string]any{"email": "actions@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-token-actions",
		RefreshToken: "refresh-token-actions",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct-actions",
	})
	f.NavigateToCredentials()

	f.ClickCredentialAction(credID, "test")
	f.WaitToastContaining("2 model(s) available")
	assert.Contains(t, f.Text("#credentials-table"), "OpenAI actions")

	f.ClickCredentialAction(credID, "refresh")
	f.WaitToastContaining("Credential refreshed")
	refreshed, err := testDB.GetCredential(context.Background(), credID)
	require.NoError(t, err)
	var refreshedInfo map[string]any
	require.NoError(t, json.Unmarshal(refreshed.CredentialInfo, &refreshedInfo))
	assert.NotEmpty(t, refreshedInfo["last_refresh_at"])

	f.ClickCredentialAction(credID, "disable")
	f.WaitToastContaining("Credential disabled")
	f.WaitForTextIn("#credentials-table", "Disabled")

	f.ClickCredentialAction(credID, "delete")
	f.WaitToastContaining("Credential deleted")
	assert.NotContains(t, f.Text("#credentials-table"), "OpenAI actions")
}

func TestCredentialActions_ConfirmCancelDoesNotMutate(t *testing.T) {
	f := setup(t)
	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-confirm-cancel",
		Name: "OpenAI confirm cancel",
		Info: map[string]any{"email": "cancel@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-token-cancel",
		RefreshToken: "refresh-token-cancel",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct-cancel",
	})
	f.NavigateToCredentials()
	_, err := f.Page.Evaluate(`() => { window.confirm = () => false }`)
	require.NoError(t, err)

	f.ClickCredentialAction(credID, "disable")
	time.Sleep(250 * time.Millisecond)

	cred, err := testDB.GetCredential(context.Background(), credID)
	require.NoError(t, err)
	var info map[string]any
	require.NoError(t, json.Unmarshal(cred.CredentialInfo, &info))
	assert.Equal(t, "active", info["status"])
	assert.NotContains(t, f.Text("body"), "Credential disabled")

	f.ClickCredentialAction(credID, "delete")
	time.Sleep(250 * time.Millisecond)

	_, err = testDB.GetCredential(context.Background(), credID)
	require.NoError(t, err)
	assert.Contains(t, f.Text("#credentials-table"), "OpenAI confirm cancel")
	assert.NotContains(t, f.Text("body"), "Credential deleted")
}

func TestCredentialActions_DetailDisableAndDelete(t *testing.T) {
	f := setup(t)
	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-detail-actions",
		Name: "OpenAI detail actions",
		Info: map[string]any{"email": "detail-actions@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-token-detail-actions",
		RefreshToken: "refresh-token-detail-actions",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct-detail-actions",
	})
	f.NavigateToCredentialDetail(credID)

	f.ClickCredentialAction(credID, "disable")
	f.WaitToastContaining("Credential disabled")
	f.WaitForTextIn("#credential-detail-content", "operator_disabled")

	f.ClickCredentialAction(credID, "delete")
	f.WaitToastContaining("Credential deleted")
	assert.NotContains(t, f.Text("#credentials-table"), "OpenAI detail actions")
}

func TestCredentialActions_ErrorToastRedactsUpstreamSecret(t *testing.T) {
	f := setup(t)
	secretBearer := "Bearer " + credentialsSecretJWT
	modelsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"`+secretBearer+`"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(modelsServer.Close)
	uiHandler.OpenAIUpstreamBaseURL = modelsServer.URL
	uiHandler.OpenAIUpstreamHTTPClient = modelsServer.Client()

	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-action-error",
		Name: "OpenAI action error",
		Info: map[string]any{"email": "error@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-token-error",
		RefreshToken: "refresh-token-error",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct-error",
	})
	f.NavigateToCredentials()

	f.ClickCredentialAction(credID, "test")
	toastText := f.WaitToastContaining("upstream_401")
	assert.Contains(t, toastText, "upstream_401")

	html, err := f.Page.Locator("body").InnerHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, credentialsSecretJWT)
	assert.NotContains(t, html, secretBearer)
	assert.NotContains(t, html, "access-token-error")
}

func (f *Fixture) SeedEncryptedOpenAISubscriptionCredential(opts SeedCredentialOpts, bundle handler.OpenAISubscriptionTokenBundle) string {
	f.T.Helper()
	raw, err := json.Marshal(bundle)
	require.NoError(f.T, err)
	encrypted, err := auth.Encrypt(string(raw), masterKey)
	require.NoError(f.T, err)
	opts.Value = encrypted
	return f.SeedCredential(opts)
}

func (f *Fixture) ClickCredentialAction(credentialID, action string) {
	f.T.Helper()
	sel := `form[hx-post="/ui/credentials/` + credentialID + `/` + action + `"] button`
	require.NoError(f.T, f.Page.Locator(sel).First().Click())
	f.WaitStable()
}

func (f *Fixture) WaitToastContaining(text string) string {
	f.T.Helper()
	loc := f.Page.Locator("[data-tui-toast]").Filter(playwright.LocatorFilterOptions{
		HasText: text,
	}).Last()
	require.NoError(f.T, loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
	toastText, err := loc.TextContent()
	require.NoError(f.T, err)
	return toastText
}
