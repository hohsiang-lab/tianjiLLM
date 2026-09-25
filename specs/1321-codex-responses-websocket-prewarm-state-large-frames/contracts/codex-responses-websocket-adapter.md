# Contract: Codex Responses WebSocket adapter

## Handshake

```http
GET /v1/responses HTTP/1.1
Authorization: Bearer <tianji-api-key>
Connection: Upgrade
Upgrade: websocket
OpenAI-Beta: responses_websockets=2026-02-06
x-client-request-id: <request-id>
session_id: <session-id>
thread_id: <thread-id>
```

Required:

- authenticate Tianji API key;
- accept the WebSocket upgrade;
- keep the connection open for sequential `response.create` frames;
- process one in-flight response at a time.

## Prewarm Frame

Client sends:

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "generate": false,
  "tools": [],
  "input": [{ "type": "message", "role": "user", "content": [{ "type": "input_text", "text": "say OK" }] }]
}
```

Tianji must:

- consume `type`;
- consume `generate:false`;
- resolve `model` through DB-managed `openai/*`;
- cache enough request state for later `previous_response_id`;
- emit or pass through a response id;
- not call backend with a `generate` field.

## Follow-Up Frame

Client sends:

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "previous_response_id": "resp_prewarm",
  "input": []
}
```

Tianji must:

- resolve `previous_response_id` from same-connection cache;
- reconstruct a backend-valid request from cached state;
- preserve `store:false` and backend streaming requirements;
- return Responses-compatible generated events;
- update latest connection-local response state after success.

## Large Frame

Client may send frame payloads larger than 32KiB due to instructions, tools, and metadata.

Tianji must:

- accept realistic Codex frames above 32KiB;
- enforce a bounded maximum;
- fail safely above that maximum without logging full request content.

## Missing State

If the client references a `previous_response_id` unavailable on the current connection and no full input context is present, Tianji returns a redacted invalid request error; it does not forward an incomplete backend payload.
