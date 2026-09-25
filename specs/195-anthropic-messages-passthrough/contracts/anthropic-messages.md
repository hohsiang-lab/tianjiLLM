# API Contract: POST /v1/messages

## Endpoint

```
POST /v1/messages
POST /messages        (bare path alias)
```

## Request

### Headers (client → proxy)

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | `Bearer <tianji-virtual-key>` or `Bearer <master-key>` |
| `Content-Type` | Yes | `application/json` |
| `anthropic-beta` | No | Comma-separated beta feature flags (forwarded to upstream) |
| `anthropic-version` | No | API version (forwarded; defaults to `2023-06-01`) |

### Query Parameters

All query parameters are forwarded to upstream (e.g., `?beta=true`).

### Body

Anthropic Messages API native format — forwarded unchanged. Only `model` field is read by the proxy.

```json
{
  "model": "claude-sonnet-4-6",
  "messages": [{"role": "user", "content": "hi"}],
  "max_tokens": 1024,
  "stream": true
}
```

## Response

### Non-streaming (stream absent or false)

Upstream Anthropic response forwarded as-is with original status code.

```json
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "content": [{"type": "text", "text": "Hello!"}],
  "model": "claude-sonnet-4-6",
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 8, "output_tokens": 12}
}
```

### Streaming (stream: true)

`Content-Type: text/event-stream` — upstream SSE events forwarded as-is.

```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: message_stop
data: {"type":"message_stop"}
```

### Error Responses

Upstream errors forwarded with original status code and body:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "..."
  }
}
```

Proxy-level errors (auth failure, no config) use TianjiLLM error format:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "code": "..."
  }
}
```
