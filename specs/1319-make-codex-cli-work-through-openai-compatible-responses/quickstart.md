# Quickstart: HO-1319 verification

## Local RED/Green Tests

```bash
go test ./internal/proxy/handler -run 'Responses|OpenAISubscription|Codex' -count=1
GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e
git diff --check
```

## Codex CLI Manual Verification

Use placeholders only:

```toml
model = "openai/gpt-5.5"
openai_base_url = "https://tianji.hohsiang.com.tw/v1"
```

Expected result:

- Codex CLI completes `say OK` through Tianji.
- No repeated websocket failure / HTTP fallback loop is visible.
- Tianji logs identify the selected model and route without exposing token material.

## Direct HTTP Probe

```bash
curl -sS https://tianji.hohsiang.com.tw/v1/responses \
  -H "Authorization: Bearer ${TIANJI_OPENCLAW_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"openai/gpt-5.5","input":"say OK"}'
```

Expected:

- Success response from the intended subscription route.
- No `openai_subscription_reauthorization_required`.
- No disabled-credential failure for the intended active credential.

## WebSocket Probe

Implementation must provide a concrete probe command after choosing the final websocket handling approach.

Minimum expected evidence:

- `GET /v1/responses` with websocket upgrade headers succeeds with Codex headers including `OpenAI-Beta: responses_websockets=2026-02-06`, `x-client-request-id`, `session_id`, and `thread_id`.
- The first websocket frame is `response.create` with `model=openai/gpt-5.5` and user `input`; the client frame does not need HTTP `stream`.
- The same websocket connection can accept a later sequential `response.create`.
- Tianji returns Responses-compatible websocket events and completes the simple `say OK` prompt.
- Codex CLI primary transport path succeeds or a non-fallback integration path is explicitly owner-approved.
