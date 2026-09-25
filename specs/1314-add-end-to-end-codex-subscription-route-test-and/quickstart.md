# Quickstart: HO-1314 Codex subscription route

## Planning Gate

This Todo phase is docs-only. Do not run implementation commands until Linear HO-1314 is moved to `In Progress`.

## Targeted Verification After Implementation Starts

```bash
go test ./internal/proxy/handler -run 'TestCodexSubscriptionRoute|TestLoadOpenAISubscriptionCredential|TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard' -count=1
go test ./internal/provider/chatgptcodex -run 'Test.*Payload|Test.*Stream' -count=1
go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/provider/openai -count=1
git diff --check origin/main...HEAD
```

If SQL or generated DB code changes:

```bash
sqlc generate
go test ./internal/db ./internal/proxy/handler -count=1
git diff --check origin/main...HEAD
```

## Secret-Safe Live Verification Shape

Use a redacted request through Tianji/OpenClaw:

```bash
curl -sS -N "$TIANJI_BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer <redacted-master-or-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-5.5",
    "stream": true,
    "messages": [{"role": "user", "content": "Say OK."}]
  }'
```

Expected:

- SSE chunk(s) arrive as `data: {...}`.
- Stream terminates with `data: [DONE]`.
- Logs/diagnostics identify route class as `chatgpt_codex_backend` or equivalent safe marker.
- No token, cookie, decrypted credential, refresh token, or raw `Authorization` value is printed.

Failure classification:

- `credential_missing`: configured ID is not present in `CredentialTable`.
- `credential_lookup_failed`: DB/query/scan/runtime read failure; inspect schema/query/runtime, not OAuth reconnect first.
- `credential_disabled` or `auth_failed_after_refresh`: reconnect or repair credential lifecycle.
- Codex 401/403/429: upstream auth/scope/quota class, not local wildcard route failure.
- Platform `/v1/chat/completions` path: wrong route regression.
