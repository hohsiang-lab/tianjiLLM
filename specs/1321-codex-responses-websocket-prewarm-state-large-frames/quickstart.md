# Quickstart: HO-1321 verification

## Local RED/Green Tests

```bash
go test ./internal/proxy/handler -run 'CreateResponseWebSocket|Responses|Codex' -count=1
GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e
git diff --check
```

Expected before implementation:

- prewarm `generate:false` coverage fails because current normalization forwards `generate`;
- follow-up `previous_response_id + input: []` coverage fails because current handler has no connection-local state;
- >32KiB frame coverage fails because current WebSocket path has no raised read limit.

## Synthetic WebSocket Probe Shape

Handshake:

```http
GET /v1/responses HTTP/1.1
Authorization: Bearer ${TIANJI_API_KEY}
Connection: Upgrade
Upgrade: websocket
OpenAI-Beta: responses_websockets=2026-02-06
x-client-request-id: ho-1321-probe
session_id: ho-1321-session
thread_id: ho-1321-thread
```

Prewarm:

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "generate": false,
  "tools": [],
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "say OK" }]
    }
  ]
}
```

Follow-up:

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "previous_response_id": "<warmup-response-id>",
  "input": []
}
```

Expected after implementation:

- backend request does not include `generate`;
- backend request for follow-up is complete and valid;
- WebSocket emits Responses-compatible events and remains sequentially reusable.

## True Codex CLI Verification

Use the existing operator config for Tianji:

```toml
model = "openai/gpt-5.5"
openai_base_url = "https://tianji.hohsiang.com.tw/v1"
```

Run a simple prompt:

```bash
codex exec --json "say OK"
```

Acceptance:

- command exits `status=0`;
- final output contains `OK`;
- JSONL/stderr does not contain `Reconnecting... 2/5`, `websocket closed by server before response.completed`, `Unsupported parameter: generate`, or `read limited at 32769 bytes`.

## No-Regression Checks

- Existing HO-1319 small WebSocket sequential frame test remains green.
- Existing HO-1314 chat-completions streaming route remains green.
- HTTP `/v1/responses` for `openai/gpt-5.5` still returns successful SSE/response through intended Codex backend route.
