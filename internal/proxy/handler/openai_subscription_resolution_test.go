package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
)

func TestResolveOpenAISubscriptionCredential_ExplicitIDsOnly(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	var lookedUp []string
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		lookedUp = append(lookedUp, id)
		return db.CredentialTable{
			CredentialID:    id,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: encryptOpenAITokenBundle(t, futureOpenAITokenBundle("access-secret"), masterKey),
			CredentialInfo:  []byte(`{"status":"active"}`),
		}, nil
	}
	m.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) {
		t.Fatal("resolver must not scan all credentials")
		return nil, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred_a"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.CredentialID != "cred_a" || resolved.BearerToken != "access-secret" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if len(lookedUp) != 1 || lookedUp[0] != "cred_a" {
		t.Fatalf("looked up IDs = %#v", lookedUp)
	}
}

func TestResolveOpenAISubscriptionCredential_NoAPIKeyFallbackOnFailure(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialType: "api_key"}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	apiKey := "sk-fallback"

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		APIKey:                          &apiKey,
		OpenAISubscriptionCredentialIDs: []string{"cred_a"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	if err == nil {
		t.Fatal("expected resolver error")
	}
	if err.Error() == apiKey {
		t.Fatal("resolver leaked or returned api_key fallback")
	}
}

func TestResolveOpenAISubscriptionCredential_AllUnusableNoAPIKeyFallback(t *testing.T) {
	apiKey := "sk-fallback"
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {Status: "disabled"},
		"cred-b": {Malformed: true},
	})

	_, err := h.resolveOpenAIAPIKeyForParams(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		APIKey:                          &apiKey,
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	})

	if err == nil {
		t.Fatal("expected all-unusable error")
	}
	if err.Error() == apiKey {
		t.Fatal("resolver returned fallback api_key")
	}
	if !strings.Contains(err.Error(), "credential_disabled") || !strings.Contains(err.Error(), "credential_malformed") {
		t.Fatalf("all-unusable error did not preserve reason codes: %v", err)
	}
}

func TestResolveOpenAIAPIKeyForParams_OfficialRejectsAPIKeyFallback(t *testing.T) {
	apiKey := "[REDACTED]"
	h := &Handlers{Config: &config.ProxyConfig{}}

	got, err := h.resolveOpenAIAPIKeyForParams(context.Background(), config.TianjiParams{
		Model:  "openai/gpt-4o",
		APIKey: &apiKey,
	})

	if err == nil {
		t.Fatal("official OpenAI must not fall back to a generic API key")
	}
	if got != "" {
		t.Fatalf("official OpenAI returned a generic API key: %q", got)
	}
}

func TestResolveProvider_CompatibilityModelKeepsAPIKeyPath(t *testing.T) {
	apiKey := "sk-existing"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "gpt-4o-key",
		TianjiParams: config.TianjiParams{
			Model:  "openaicompat/gpt-4o",
			APIKey: &apiKey,
		},
	}}}}

	_, gotAPIKey, gotModel, err := h.resolveProviderFromConfig("gpt-4o-key")
	if err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != apiKey || gotModel != "gpt-4o" {
		t.Fatalf("api-key path changed: apiKey=%q model=%q", gotAPIKey, gotModel)
	}
}

func TestChatGPTCodexBackendResolution_ReturnsBearerMaterial(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:   id,
			CredentialType: CredentialTypeOpenAISubscription,
			CredentialValue: encryptOpenAITokenBundle(t, OpenAISubscriptionTokenBundle{
				AccessToken:  "bearer-access",
				RefreshToken: "refresh-secret",
				ExpiresAt:    time.Now().Add(time.Hour),
				AccountID:    "acct_123",
			}, masterKey),
			CredentialInfo: []byte(`{"status":"active"}`),
		}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred_a"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.BearerToken != "bearer-access" {
		t.Fatalf("BearerToken = %q", resolved.BearerToken)
	}
	if resolved.CodexLogin != nil {
		t.Fatalf("ChatGPT Codex backend must not create a Codex login payload: %#v", resolved.CodexLogin)
	}
}

func TestResolveOpenAISubscriptionCredential_CodexAppServerLoginPayload(t *testing.T) {
	masterKey := "test-master-key-32-bytes-long!!!"
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{
			CredentialID:    id,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: encryptOpenAITokenBundle(t, futureOpenAITokenBundle("access-secret"), masterKey),
			CredentialInfo:  []byte(`{"status":"active"}`),
		}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = masterKey

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred_a"},
	}, openAISubscriptionTransportCodexAppServer)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.CodexLogin == nil {
		t.Fatal("Codex app-server resolution should create a login payload")
	}
	if resolved.CodexLogin.Type != "chatgptAuthTokens" || resolved.CodexLogin.AccessToken != "access-secret" || resolved.CodexLogin.ChatGPTAccountID != "acct_123" {
		t.Fatalf("Codex login = %#v", resolved.CodexLogin)
	}
	if resolved.BearerToken != "" {
		t.Fatalf("Codex app-server must not use HTTP bearer injection, got %q", resolved.BearerToken)
	}
}

func TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken(t *testing.T) {
	now := time.Date(2026, 5, 7, 3, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "stale-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Minute),
		AccountID:    "acct_old",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token":  "codex-access",
			"refresh_token": "codex-refresh",
			"expires_in":    1800,
			"account_id":    "acct_new",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)

	resolved, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
	}, openAISubscriptionTransportCodexAppServer)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.CodexLogin == nil {
		t.Fatal("Codex app-server resolution should create a login payload")
	}
	if resolved.CodexLogin.AccessToken != "codex-access" || resolved.CodexLogin.ChatGPTAccountID != "acct_new" {
		t.Fatalf("Codex login = %#v", resolved.CodexLogin)
	}
	if resolved.BearerToken != "" {
		t.Fatalf("Codex app-server must not use HTTP bearer injection, got %q", resolved.BearerToken)
	}
}

func TestResolveProviderRoute_CompatibilityRejectsSubscriptionSelectors(t *testing.T) {
	apiKey := "compat-api-key"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "compat-model",
		TianjiParams: config.TianjiParams{
			Model:                           "openaicompat/gpt-4o",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"credential"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}}}

	_, err := h.resolveProviderFromConfigRouteWithContext(context.Background(), "compat-model")
	if err == nil || !strings.Contains(err.Error(), "only support the official OpenAI provider") {
		t.Fatalf("expected compatibility subscription-selector rejection, got %v", err)
	}
}

func TestResolveProviderRoute_EmptyCanonicalModelFailsClosed(t *testing.T) {
	apiKey := "legacy-api-key"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "empty-model",
		TianjiParams: config.TianjiParams{
			APIKey: &apiKey,
		},
	}}}}

	_, err := h.resolveProviderFromConfigRouteWithContext(context.Background(), "empty-model")
	if err == nil || !strings.Contains(err.Error(), "canonical model provider is required") {
		t.Fatalf("expected empty canonical model rejection, got %v", err)
	}
}

func futureOpenAITokenBundle(accessToken string) OpenAISubscriptionTokenBundle {
	return OpenAISubscriptionTokenBundle{
		AccessToken:  accessToken,
		RefreshToken: "refresh-secret",
		ExpiresAt:    time.Now().Add(time.Hour),
		AccountID:    "acct_123",
	}
}
