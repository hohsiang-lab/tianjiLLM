package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

type openAIOAuthCallbackInput struct {
	State                    string
	Code                     string
	ProviderError            string
	ProviderErrorDescription string
}

type openAIIDTokenClaims struct {
	Email   string `json:"email"`
	Sub     string `json:"sub"`
	Profile struct {
		Email string `json:"email"`
	} `json:"https://api.openai.com/profile"`
	Auth struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
		AccountID        string `json:"account_id"`
		ChatGPTUserID    string `json:"chatgpt_user_id"`
		UserID           string `json:"user_id"`
		Organizations    []struct {
			ID        string `json:"id"`
			AccountID string `json:"account_id"`
		} `json:"organizations"`
	} `json:"https://api.openai.com/auth"`
}

func (h *Handlers) OpenAIOAuthCallback(w http.ResponseWriter, r *http.Request) {
	h.completeOpenAIOAuthCallback(w, r, openAIOAuthCallbackInput{
		State:                    r.URL.Query().Get("state"),
		Code:                     r.URL.Query().Get("code"),
		ProviderError:            r.URL.Query().Get("error"),
		ProviderErrorDescription: r.URL.Query().Get("error_description"),
	})
}

func (h *Handlers) OpenAIOAuthPastedCallback(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		writeOpenAIOAuthPage(w, http.StatusServiceUnavailable, "OpenAI connection failed", "OpenAI OAuth is not configured.")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", "The pasted callback URL could not be read.")
		return
	}
	expectedRedirectURI, err := openAIOAuthRedirectURIForHandler(h)
	if err != nil {
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", "OpenAI OAuth redirect URI is not configured correctly.")
		return
	}
	input, err := parseOpenAIPastedCallbackURL(r.Form.Get("callback_url"), expectedRedirectURI)
	if err != nil {
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", redact.String(err.Error()))
		return
	}
	h.completeOpenAIOAuthCallback(w, r, input)
}

func (h *Handlers) completeOpenAIOAuthCallback(w http.ResponseWriter, r *http.Request, input openAIOAuthCallbackInput) {
	if h.Config == nil {
		writeOpenAIOAuthPage(w, http.StatusServiceUnavailable, "OpenAI connection failed", "OpenAI OAuth is not configured.")
		return
	}
	if !h.openAIOAuthConfig().Enabled {
		writeOpenAIOAuthPage(w, http.StatusServiceUnavailable, "OpenAI connection failed", ErrOpenAIConnectDisabled.Error())
		return
	}
	record, err := openaioauth.NewStateStore(h.Cache).Consume(r.Context(), input.State)
	if err != nil {
		message := "The OpenAI connection state is invalid. Please start again from the TianjiLLM UI."
		if errors.Is(err, openaioauth.ErrExpiredState) {
			message = "The OpenAI connection state expired. Please start again from the TianjiLLM UI."
		}
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", message)
		return
	}

	if providerErr := input.ProviderError; providerErr != "" {
		message := providerErr
		if desc := input.ProviderErrorDescription; desc != "" {
			message += ": " + desc
		}
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", redact.String(message))
		return
	}

	code := input.Code
	if code == "" {
		writeOpenAIOAuthPage(w, http.StatusBadRequest, "OpenAI connection failed", "OpenAI did not return an authorization code.")
		return
	}

	tokenBundle, err := openai.ExchangeCode(
		r.Context(),
		h.openAIOAuthHTTPClient(),
		h.Config.GeneralSettings.OpenAIOAuth,
		code,
		record.RedirectURI,
		record.CodeVerifier,
	)
	if err != nil {
		writeOpenAIOAuthPage(w, http.StatusBadGateway, "OpenAI connection failed", "OpenAI token exchange failed. Please start again from the TianjiLLM UI.")
		return
	}

	now := time.Now().UTC()
	credentialBundle := openAISubscriptionTokenBundle(tokenBundle, now)
	credentialInfo := openAISubscriptionCredentialInfo(tokenBundle, now)
	var organizationID *string
	if record.OrgID != "" {
		organizationID = &record.OrgID
	}
	cred, err := h.SaveOpenAISubscriptionCredential(r.Context(), "OpenAI Subscription", credentialBundle, credentialInfo, organizationID)
	if err != nil {
		h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
			Provider:       "openai",
			OrganizationID: record.OrgID,
			Action:         "connect",
			Status:         "failure",
			ReasonCode:     "save_failed",
			Metadata:       map[string]any{"error": err.Error()},
		})
		writeOpenAIOAuthPage(w, http.StatusInternalServerError, "OpenAI connection failed", "OpenAI credential could not be saved.")
		return
	}
	h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
		CredentialID:   cred.CredentialID,
		Provider:       "openai",
		OrganizationID: record.OrgID,
		Action:         "connect",
		Status:         "success",
	})

	writeOpenAIOAuthPage(w, http.StatusOK, "OpenAI connected", "OpenAI subscription credential connected successfully.")
}

func parseOpenAIPastedCallbackURL(rawCallbackURL, expectedRedirectURI string) (openAIOAuthCallbackInput, error) {
	rawCallbackURL = strings.TrimSpace(rawCallbackURL)
	if rawCallbackURL == "" {
		return openAIOAuthCallbackInput{}, fmt.Errorf("paste a valid callback URL from the localhost OpenAI tab")
	}
	parsed, err := url.Parse(rawCallbackURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return openAIOAuthCallbackInput{}, fmt.Errorf("paste a valid callback URL from the localhost OpenAI tab")
	}
	expected, err := url.Parse(expectedRedirectURI)
	if err != nil || expected.Scheme == "" || expected.Host == "" {
		return openAIOAuthCallbackInput{}, fmt.Errorf("openai oauth redirect URI is not configured correctly")
	}
	if parsed.Scheme != expected.Scheme || parsed.Host != expected.Host || parsed.Path != expected.Path {
		return openAIOAuthCallbackInput{}, fmt.Errorf("paste the localhost callback URL from the OpenAI authorization tab")
	}
	values := parsed.Query()
	input := openAIOAuthCallbackInput{
		State:                    strings.TrimSpace(values.Get("state")),
		Code:                     strings.TrimSpace(values.Get("code")),
		ProviderError:            strings.TrimSpace(values.Get("error")),
		ProviderErrorDescription: strings.TrimSpace(values.Get("error_description")),
	}
	if input.State == "" {
		return openAIOAuthCallbackInput{}, fmt.Errorf("the pasted callback URL is missing state")
	}
	if input.ProviderError == "" && input.Code == "" {
		return openAIOAuthCallbackInput{}, fmt.Errorf("the pasted callback URL is missing an authorization code")
	}
	return input, nil
}

func openAIOAuthRedirectURIForHandler(h *Handlers) (string, error) {
	cfg := h.openAIOAuthConfig()
	cfg = config.ResolveOpenAIOAuthConfig(cfg)
	return config.ResolveOpenAIOAuthRedirectURI(cfg)
}

func (h *Handlers) openAIOAuthHTTPClient() *http.Client {
	if h.OpenAIOAuthHTTPClient != nil {
		return h.OpenAIOAuthHTTPClient
	}
	return http.DefaultClient
}

func openAISubscriptionTokenBundle(bundle *openai.TokenBundle, now time.Time) OpenAISubscriptionTokenBundle {
	if bundle == nil {
		return OpenAISubscriptionTokenBundle{}
	}
	expiresAt := time.Time{}
	if bundle.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(bundle.ExpiresIn) * time.Second)
	}
	return OpenAISubscriptionTokenBundle{
		AccessToken:  bundle.AccessToken,
		RefreshToken: bundle.RefreshToken,
		ExpiresAt:    expiresAt,
		AccountID:    openAISubscriptionAccountID(bundle),
	}
}

func openAISubscriptionCredentialInfo(bundle *openai.TokenBundle, now time.Time) OpenAISubscriptionCredentialInfo {
	info := OpenAISubscriptionCredentialInfo{
		Status:        "active",
		LastRefreshAt: &now,
	}
	if bundle == nil {
		return info
	}
	info.Email = rawString(bundle.Raw, "email")
	if info.Email == "" {
		info.Email = openAISubscriptionEmail(bundle)
	}
	if bundle.Scope != "" {
		info.Scopes = strings.Fields(bundle.Scope)
	}
	return info
}

func openAISubscriptionAccountID(bundle *openai.TokenBundle) string {
	if bundle == nil {
		return ""
	}
	if accountID := rawString(bundle.Raw, "account_id", "accountId", "sub"); accountID != "" {
		return accountID
	}
	claims, ok := parseOpenAIIDTokenClaims(bundle)
	if !ok {
		return ""
	}
	for _, value := range []string{
		claims.Auth.ChatGPTAccountID,
		claims.Auth.AccountID,
		firstOpenAIOrganizationID(claims),
		claims.Auth.ChatGPTUserID,
		claims.Auth.UserID,
		claims.Sub,
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func openAISubscriptionEmail(bundle *openai.TokenBundle) string {
	claims, ok := parseOpenAIIDTokenClaims(bundle)
	if !ok {
		return ""
	}
	if strings.TrimSpace(claims.Email) != "" {
		return strings.TrimSpace(claims.Email)
	}
	return strings.TrimSpace(claims.Profile.Email)
}

func firstOpenAIOrganizationID(claims openAIIDTokenClaims) string {
	for _, org := range claims.Auth.Organizations {
		if strings.TrimSpace(org.AccountID) != "" {
			return strings.TrimSpace(org.AccountID)
		}
		if strings.TrimSpace(org.ID) != "" {
			return strings.TrimSpace(org.ID)
		}
	}
	return ""
}

func parseOpenAIIDTokenClaims(bundle *openai.TokenBundle) (openAIIDTokenClaims, bool) {
	if bundle == nil {
		return openAIIDTokenClaims{}, false
	}
	idToken := bundle.IDToken
	if idToken == "" {
		idToken = rawString(bundle.Raw, "id_token", "idToken")
	}
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 || parts[1] == "" {
		return openAIIDTokenClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return openAIIDTokenClaims{}, false
	}
	var claims openAIIDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return openAIIDTokenClaims{}, false
	}
	return claims, true
}

func rawString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch typed := value.(type) {
			case string:
				return typed
			case fmt.Stringer:
				return typed.String()
			}
		}
	}
	return ""
}

func writeOpenAIOAuthPage(w http.ResponseWriter, status int, title, message string) {
	statusTone := "text-destructive"
	statusBadge := "Connection failed"
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		statusTone = "text-primary"
		statusBadge = "Connected"
	}
	safeTitle := html.EscapeString(title)
	safeMessage := html.EscapeString(message)
	safeBadge := html.EscapeString(statusBadge)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en" class="h-full">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>%s - TianjiLLM</title>
<link rel="stylesheet" href="/ui/static/css/output.css"/>
</head>
<body class="min-h-full bg-background text-foreground antialiased">
<main class="flex min-h-screen items-center justify-center px-6 py-12">
<section class="w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-sm">
<div class="space-y-5">
<div class="space-y-2">
<div class="text-xs font-medium uppercase tracking-wide text-muted-foreground">%s</div>
<h1 class="text-2xl font-semibold %s">%s</h1>
<p class="text-sm leading-6 text-muted-foreground">%s</p>
</div>
<a href="/ui/credentials" class="inline-flex h-9 items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-xs transition-all hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
Back to Credentials
</a>
</div>
</section>
</main>
</body>
</html>`,
		safeTitle,
		safeBadge,
		statusTone,
		safeTitle,
		safeMessage,
	)
}
