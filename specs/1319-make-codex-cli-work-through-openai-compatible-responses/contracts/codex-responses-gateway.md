# Contract: Codex CLI through Tianji `/v1/responses`

## Client Configuration

Observed owner setup:

```toml
model = "openai/gpt-5.5"
openai_base_url = "https://tianji.hohsiang.com.tw/v1"
```

This contract treats client config as already verified by Linear evidence. Server work begins at Tianji `/v1/responses`.

## Primary Transport

### Request

```http
GET /v1/responses HTTP/1.1
Host: tianji.hohsiang.com.tw
Connection: Upgrade
Upgrade: websocket
OpenAI-Beta: responses_websockets=2026-02-06
x-client-request-id: <codex-thread-or-request-id>
session_id: <codex-session-id>
thread_id: <codex-thread-id>
Authorization: Bearer <tianji-api-key>
```

### First websocket frame

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "say OK" }]
    }
  ]
}
```

### Required behavior

- Must not return plain `405 Method Not Allowed` as the normal Codex path.
- Must authenticate with Tianji API key.
- Must preserve or safely consume Codex handshake headers needed for request/session identity.
- Must resolve the requested model to `openai/gpt-5.5` / wildcard `openai/*` from the `response.create` frame.
- Must return Responses WebSocket-compatible events for the generated response.
- Must use a supported Codex-compatible transport or pause for owner confirmation if the only available design is fallback-based.
- Must not count route registration, `426`, or any other non-405 response as success unless the first `response.create` frame is covered.
- Must not require HTTP `stream` in the WebSocket frame; the server bridge is responsible for requesting backend streaming events.
- Must keep the connection open for later sequential `response.create` frames after one response completes.

## HTTP Responses Request

### Request

```http
POST /v1/responses HTTP/1.1
Authorization: Bearer <tianji-api-key>
Content-Type: application/json

{"model":"openai/gpt-5.5","input":"say OK"}
```

### Required behavior

- Resolve `openai/gpt-5.5` through DB-managed wildcard `openai/*`.
- Use the intended OpenAI subscription credential path.
- Refresh stale credentials where possible.
- Return a successful Codex-usable response for a healthy route.

## Failure Contract

Credential failures must return redacted errors:

```json
{
  "error": {
    "message": "<actionable redacted message>",
    "type": "authentication_error",
    "code": "<stable code>"
  }
}
```

Errors must not include:

- access token
- refresh token
- authorization code
- raw bearer header
- encrypted credential payload

## No-Regression Contract

Existing `/v1/chat/completions` DB-managed Codex subscription route must still support:

- `model = "openai/gpt-5.5"`
- `stream = true`
- SSE response containing normalized OpenAI-compatible deltas and `data: [DONE]`

Non-streaming chat-completions must keep the deterministic upstream contract failure already covered by HO-1314.
