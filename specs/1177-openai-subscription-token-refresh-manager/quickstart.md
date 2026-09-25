# Quickstart: OpenAI Subscription Token Refresh Manager

## Scope

These commands are for the future In Progress implementation. Todo planning must not run production-code edits.

## Targeted Test Commands

```bash
go test ./internal/provider/openai/... -run 'TestOpenAIRefreshToken' -v
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscriptionCredential_.*Refresh|TestResolveOpenAISubscriptionCredential_Typed|TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken' -v
```

## Broader Verification

```bash
go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/testutil/openaitest/... -v
```

## Manual Fixture Shape

Use only synthetic token values.

```go
oauthServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{
    TokenFixtures: []openaitest.TokenFixture{{
        RefreshToken: "old-refresh-token",
        Response: map[string]any{
            "access_token":  "new-access-token",
            "refresh_token": "new-refresh-token",
            "token_type":    "Bearer",
            "expires_in":    3600,
            "scope":         "openid profile email",
        },
    }},
})
```

## Expected Checks

- Fresh token path records zero OAuth mock requests.
- Near-expiry path records one `grant_type=refresh_token` request.
- Concurrent same-credential path records one OAuth mock request.
- Refresh request has no `client_secret` and no Basic auth.
- Rotated refresh token is persisted.
- Omitted refresh token preserves existing stored refresh token.
- Failure metadata is redacted.
- `errors.As` exposes typed error codes.

## No-Real-OpenAI Guard

Use:

```go
h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(oauthServer.Host())
```

Tests must fail if any code tries to call `auth.openai.com` or `api.openai.com`.
