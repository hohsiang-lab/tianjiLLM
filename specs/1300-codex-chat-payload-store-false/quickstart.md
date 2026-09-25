# Quickstart: HO-1300 verification

## State Gate

Do not run implementation steps until Linear HO-1300 is `In Progress`.

## Targeted RED/Green Commands

```bash
go test ./internal/provider/chatgptcodex -run 'TestBuildPayload_.*Store' -count=1
go test ./internal/proxy/handler -run 'TestChatGPTCodexBackend.*Store|TestChatGPTCodexBackendWildcard' -count=1
go test ./internal/provider/openai -run 'TestTransformRequest_ExtraParams|TestOpenAI.*Store' -count=1
```

## Full Local Gate After Fix

```bash
go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai -count=1
git diff --check origin/main...HEAD
```

## Manual Prod Verification After Merge/Deploy

Use Tianji/OpenClaw API keys only through existing secure environment handling. Do not print token values.

Request shape:

```json
{
  "model": "openai/gpt-5.5",
  "messages": [
    {"role": "system", "content": "You are a connectivity probe."},
    {"role": "user", "content": "Reply exactly OK."}
  ]
}
```

Expected:

- `/v1/chat/completions` no longer returns `Store must be set to false`.
- Logs identify the selected route as `chatgpt_codex_backend` or the mocked/live Codex backend path.
- Failure, if any, should be a later auth/quota/model diagnostic, not missing `store:false`.

Client compatibility check:

```json
{
  "model": "openai/gpt-5.5",
  "messages": [{"role": "user", "content": "Reply exactly OK."}],
  "store": false
}
```

Expected:

- Accepted for Codex backend route.
- Does not return `unknown extra parameters`.

Negative check:

```json
{
  "model": "openai/gpt-5.5",
  "messages": [{"role": "user", "content": "hello"}],
  "store": true
}
```

Expected:

- Local HTTP 400 validation error before upstream.

## Secret-Safety Checklist

- Do not print `Authorization` headers.
- Do not print access tokens or refresh tokens.
- Do not dump `CredentialTable.credential_value`.
- Do not paste raw credential JSON into Linear, GitHub, Discord, or logs.
