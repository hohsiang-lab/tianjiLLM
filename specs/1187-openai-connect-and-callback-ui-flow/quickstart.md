# Quickstart: OpenAI connect and callback UI flow

## Local Verification Commands

```bash
go test ./test/e2e -tags e2e -run 'TestOpenAIConnect|TestOpenAICallback' -count=1
go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -count=1
go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -count=1
go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -count=1
git diff --check origin/main...HEAD
```

## Manual Smoke Flow

1. Start TianjiLLM with local DB/cache and OpenAI OAuth endpoint overrides pointing at a local mock server.
2. Log in to `/ui/login`.
3. Open `/ui/credentials`.
4. Click `Connect OpenAI`.
5. Confirm browser navigates to the configured mock authorize endpoint with `state` and PKCE query.
6. Complete the mock callback to `/oauth/openai/callback?state=<state>&code=code-ok`.
7. Confirm success page says OpenAI is connected and has an action back to `/ui/credentials`.
8. Return to `/ui/credentials` and confirm the OpenAI subscription credential appears with safe metadata only.

## Failure Smoke Cases

```text
/oauth/openai/callback?state=missing&code=code-ok
/oauth/openai/callback?state=<valid-state>&error=access_denied&error_description=Denied
/oauth/openai/callback?state=<valid-state>
```

Each page must render a readable failure and must not expose raw token/code/verifier/upstream body.
