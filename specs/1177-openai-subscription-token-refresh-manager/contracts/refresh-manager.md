# Contract: OpenAI Subscription Refresh Manager

## Provider Refresh Helper

Future implementation API shape may vary, but must satisfy:

```go
func RefreshToken(ctx context.Context, client *http.Client, cfg config.OpenAIOAuthConfig, refreshToken string) (*TokenBundle, error)
```

Request behavior:

- Method: `POST`
- URL: `config.ResolveOpenAIOAuthConfig(cfg).TokenURL`
- Content-Type: `application/x-www-form-urlencoded`
- Accept: `application/json`
- Form fields:
  - `grant_type=refresh_token`
  - `refresh_token=<stored refresh token>`
  - `client_id=<resolved OpenAI OAuth client ID>`

Forbidden request behavior:

- No `client_secret`.
- No HTTP Basic auth.
- No token values in error messages.

Response behavior:

- Parse JSON into `TokenBundle`.
- Require `access_token`.
- Require valid positive `expires_in` before storage conversion.
- Preserve raw JSON metadata on `TokenBundle.Raw` when useful for metadata conversion.

## Refresh Manager

Future implementation API shape may vary, but must satisfy:

```go
func (h *Handlers) resolveUsableOpenAISubscriptionBundle(ctx context.Context, credentialID string) (OpenAISubscriptionTokenBundle, OpenAISubscriptionCredentialInfo, error)
```

Required behavior:

- Load credential by ID.
- Require `credential_type=openai_subscription`.
- Parse safe metadata.
- Reject `status=disabled`.
- Decrypt and validate stored bundle.
- Reuse fresh bundle when `expires_at > now + refreshBuffer`.
- Refresh when `expires_at <= now + refreshBuffer`.
- Deduplicate refresh by `credential_id`.
- Reload credential inside the refresh function before network call.
- Persist success with `UpdateOpenAISubscriptionCredential`.
- Persist failure metadata with `UpdateOpenAISubscriptionCredentialFailure`.
- Return typed safe errors.

## Typed Error Contract

Stable error codes:

```text
credential_missing
credential_wrong_type
credential_disabled
credential_malformed
credential_expired
refresh_failed
```

Required behavior:

- `errors.As(err, *OpenAISubscriptionCredentialError)` must expose the code.
- `Error()` must include credential ID and code.
- `Error()` must redact raw upstream details.
- No error path may include access token, refresh token, ID token, bearer token, JWT, or code verifier.

## Resolver Integration

Direct OpenAI HTTP:

```go
resolved.BearerToken = refreshedBundle.AccessToken
```

Codex app-server:

```go
resolved.CodexLogin = ptrCodexLogin(codexapp.NewChatGPTAuthTokensLogin(codexapp.ChatGPTAuthTokens{
    AccessToken: refreshedBundle.AccessToken,
    ChatGPTAccountID: refreshedBundle.AccountID,
}))
```

Rules:

- No API-key fallback after subscription IDs are configured.
- Codex app-server must not use HTTP bearer injection.
- Direct OpenAI HTTP must not receive stale access tokens after a successful refresh.
