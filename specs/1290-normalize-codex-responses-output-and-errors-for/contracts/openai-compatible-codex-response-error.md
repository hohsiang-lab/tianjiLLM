# Contract: Codex Responses output/error normalization

## Success Input Contract

The adapter accepts a non-streaming backend Responses-style JSON body:

```json
{
  "id": "resp_123",
  "object": "response",
  "created_at": 1741476777,
  "status": "completed",
  "model": "gpt-5.2-codex",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "hello"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "total_tokens": 15
  }
}
```

## Success Output Contract

For `/v1/chat/completions` callers, Tianji returns:

```json
{
  "id": "resp_123",
  "object": "chat.completion",
  "created": 1741476777,
  "model": "gpt-5.2-codex",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "hello"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 5,
    "total_tokens": 15
  }
}
```

## Actionable Error Contract

When the backend returns HTTP 401, 403, or 429 with an OpenAI-style error body:

```json
{
  "error": {
    "message": "missing required scope: model.request",
    "type": "permission_error",
    "code": "missing_scope"
  }
}
```

Tianji must return the same HTTP status and a safe OpenAI-compatible error body:

```json
{
  "error": {
    "message": "missing required scope: model.request",
    "type": "permission_error",
    "code": "missing_scope",
    "llm_provider": "chatgpt_codex",
    "model": "gpt-5.2-codex"
  }
}
```

Provider/model may be omitted only if the current execution path cannot determine them safely; status/message/type/code must not be omitted when upstream provides them.

## Generic Transform Error Contract

Malformed success bodies, missing assistant output, unreadable body, or unsupported private backend shapes return a safe transform/proxy error. These are not actionable upstream auth/quota failures and may remain HTTP 502.

## Security Contract

The following strings must never appear in response body or logs:

- `Authorization: Bearer <token>`
- access token
- refresh token
- encrypted credential value
- raw credential JSON

Tests should include sentinel token strings to prove redaction.
