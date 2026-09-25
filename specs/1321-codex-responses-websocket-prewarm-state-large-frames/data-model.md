# Data Model: HO-1321 Codex Responses WebSocket adapter

## Existing Entities

### Proxy model route

Source: `ProxyModelTable` / runtime model list.

Relevant fields:

- `model`: expected wildcard `openai/*`
- `openai_subscription_credential_ids`: ordered credential candidates
- `openai_subscription_transport`: expected `chatgpt_codex_backend`
- request model: `openai/gpt-5.5`

### OpenAI subscription credential

Existing resolved runtime credential data:

- access token for ChatGPT Codex backend
- optional `ChatGPT-Account-Id`
- refresh/disabled state

Credential secrets must not be serialized into specs, tests, logs, PR body, or thread comments.

## New In-Memory Runtime Concept

### ResponsesWebSocketConnectionState

Connection-local only; not persisted and not shared across WebSocket connections.

Suggested fields:

- `latestResponseID string`
- `latestRequest map[string]any`
- `latestBackendPayload map[string]any`
- `latestModel string`
- `latestRouteTransport string`
- `latestGenerated bool`

Purpose:

- preserve enough state from a prewarm/full request to make same-connection `previous_response_id` continuation valid;
- support `store:false` / ZDR-compatible semantics without DB persistence;
- avoid forwarding empty/incomplete follow-up requests to ChatGPT Codex backend.

## Request Shapes

### WebSocket handshake

```http
GET /v1/responses HTTP/1.1
Connection: Upgrade
Upgrade: websocket
Authorization: Bearer <tianji-api-key>
OpenAI-Beta: responses_websockets=2026-02-06
x-client-request-id: <request-or-thread-id>
session_id: <codex-session-id>
thread_id: <codex-thread-id>
```

### Prewarm frame

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

### Follow-up frame after prewarm

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "previous_response_id": "resp_prewarm",
  "input": []
}
```

## Adapter Rules

- `type` is WebSocket protocol envelope data and must not be sent to backend.
- `generate:false` is WebSocket adapter control data and must not be sent to backend.
- `stream:true` is backend transport requirement for SSE bridging and may be added by Tianji; the client WebSocket frame need not contain it.
- `store:false` remains required for ChatGPT Codex backend and should default when absent.
- `previous_response_id` can be used only when the referenced id exists in connection-local state or a future proven backend-compatible hydration path exists.
- `input: []` is valid from Codex only when expanded with cached state before calling backend.

## Error Model

When state is missing:

```json
{
  "error": {
    "message": "previous response state not found on this WebSocket connection",
    "type": "invalid_request_error",
    "code": "previous_response_not_found"
  }
}
```

The exact code field can match existing Tianji error model, but tests must assert the response is redacted and diagnosable.
