# Quickstart: Subscription wildcard routing regression verification

## Todo Scope

This branch is Todo planning only. Do not run implementation commands until Linear HO-1292 moves to `In Progress`.

## Future Implementation Verification

Run targeted regression tests:

```bash
go test ./internal/proxy/handler -run 'TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard|TestChatGPTCodexBackendTransport' -count=1
go test ./internal/provider/chatgptcodex ./internal/provider/openai ./internal/config -count=1
go test ./test/contract -run 'TestWildcard|TestResponsesCreate' -count=1
git diff --check origin/main...HEAD
```

## OpenClaw `openai/*` Deployment Verification Note

After rollout, verify routing without printing secrets:

1. Configure OpenClaw to call TianjiLLM through the normal OpenAI-compatible provider base URL and a TianjiLLM API key.
2. Send a small non-streaming `openai/<target-model>` request through the OpenClaw surface being validated.
3. Inspect TianjiLLM safe route evidence, not bearer headers:
   - expected subscription Codex route: backend path/class indicates `/backend-api/codex/responses` or `chatgpt_codex_backend`
   - wrong route: Platform `/v1/chat/completions` is used with a subscription-backed model
   - auth failure: 401 with safe auth diagnostic
   - missing scope: 403 with safe `missing_scope` diagnostic
   - quota/rate-limit: 429 with safe quota/rate-limit diagnostic
4. Verify normal API-key `openai/*` aliases still call OpenAI-compatible `/chat/completions` and are not marked as `chatgpt_codex_backend`.

## Secret Safety

Do not print:

- `Authorization` header values
- OpenAI access tokens
- OpenAI refresh tokens
- TianjiLLM API keys
- encrypted credential values
- raw credential JSON
- shell env dumps that may contain credentials

Use redacted log fields, request path, status code, error type/code, and model alias only.
