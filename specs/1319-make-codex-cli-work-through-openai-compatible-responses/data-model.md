# Data Model: HO-1319 Codex CLI `/v1/responses`

## Existing Entities

### Proxy model

Source: `ProxyModelTable` / runtime model list.

Relevant `tianji_params` fields:

- `model`: `openai/*`
- `openai_subscription_credential_ids`: ordered credential IDs
- `openai_subscription_transport`: expected to distinguish `direct_openai_http`, `codex_app_server`, and `chatgpt_codex_backend`
- `api_base`: must not be mixed with subscription routing unless existing validation explicitly allows it

### OpenAI subscription credential

Existing encrypted credential record.

Relevant runtime fields after safe resolution:

- `credential_id`
- `bearer_token` for direct HTTP or Codex backend transports
- `account_id`
- refresh token bundle metadata
- disabled / refresh-failed health state

Secrets must not be serialized into test failure messages, docs, PR bodies, or issue comments.

## Request Shapes

### Codex CLI Responses WebSocket handshake

Observed shape:

- Method: `GET`
- Path: `/v1/responses`
- Headers include websocket upgrade semantics.
- Codex-specific headers include `OpenAI-Beta: responses_websockets=2026-02-06`, `x-client-request-id`, `session_id`, and `thread_id`.

### Codex CLI Responses WebSocket frame

The first primary-transport model request is a websocket message, not a query parameter:

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

The implementation should resolve `model` from this `response.create` frame for the primary transport path, force backend SSE streaming internally, and keep the WebSocket available for later sequential `response.create` frames. Tests must not treat a handshake-only success as sufficient.

### Codex CLI HTTP request

Observed shape:

```json
{
  "model": "openai/gpt-5.5",
  "input": "say OK"
}
```

## Transport Decision

For model `openai/gpt-5.5` resolved through wildcard `openai/*`:

| Config state | Expected behavior |
| --- | --- |
| `openai_subscription_transport = "chatgpt_codex_backend"` | Use Codex-compatible subscription transport for the Codex CLI `/v1/responses` path. |
| no subscription credential IDs + API key | Preserve generic OpenAI-compatible `/v1/responses` proxying. |
| subscription credentials + direct HTTP intended | Preserve official OpenAI direct HTTP Responses API behavior. |
| disabled credential | Return redacted actionable error; do not leak token material. |
| stale refreshable credential | Force refresh and retry through the same intended transport. |

## Response Contract

Minimum success contract for acceptance:

- Codex CLI receives a valid response for simple prompt `say OK`.
- Tianji logs and tests can prove the selected model is `openai/gpt-5.5`.
- WebSocket tests prove handshake headers, `response.create` handling without HTTP `stream`, backend SSE bridging, and sequential frame reuse.
- The route does not rely on repeated fallback/reconnect noise.

Exact response format depends on Codex CLI transport mode and should be captured in tests from the chosen handler path.
