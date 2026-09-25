# Quickstart: OpenAI OAuth Connect/Callback State Lifecycle

## Targeted Tests

Run state-store tests:

```bash
go test ./internal/openaioauth/... -v
```

Run UI connect tests:

```bash
go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -v
```

Run callback tests:

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -v
```

Run route registration tests:

```bash
go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -v
```

## Offline Mock Requirement

Tests must point OpenAI OAuth endpoints at the HO-1190 mock harness:

```go
oauthServer := openaitest.NewOAuthServer(t)
cfg := config.OpenAIOAuthConfig{
    AuthorizeURL: oauthServer.AuthorizeURL(),
    TokenURL:     oauthServer.TokenURL(),
    ClientID:     "app_test",
}
```

Tests must use a guarded client for callback token exchange paths so accidental live calls to `auth.openai.com` fail.

## Manual Local Config Shape

```yaml
general_settings:
  public_base_url: https://tianji.example.com
  openai_oauth:
    client_id: app_EMoamEEZ73f0CkXaXp7hrann
```

Expected derived callback:

```text
https://tianji.example.com/oauth/openai/callback
```

## Browser Flow Shape

```text
1. Admin opens /ui/openai/connect?org_id=org_123 with valid UI session.
2. Tianji stores state + code_verifier + org_id in cache with TTL.
3. Tianji redirects to configured OpenAI authorize URL.
4. OpenAI redirects to /oauth/openai/callback?state=...&code=...
5. Tianji consumes state, exchanges code, persists credential for stored org_id.
6. Tianji renders readable success page.
```

## Failure Cases to Verify

- No UI session on connect redirects to `/ui/login`.
- Missing `org_id` on connect stores no state.
- Expired state on callback performs no token exchange.
- Mismatched/unknown state on callback performs no token exchange.
- Provider `error=access_denied` consumes valid state and renders a readable error.
- Token exchange failure consumes valid state and redacts token/verifier/raw-response sentinels.
- Callback with query `org_id=evil` still persists using stored state `org_id`.
