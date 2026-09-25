# Data Model: HO-1300 Codex backend `store:false`

## `chatgptcodex.Payload`

Current owner: `internal/provider/chatgptcodex/payload.go`.

Add field:

```go
Store bool `json:"store"`
```

Rules:

- Always serialize as `false`.
- Do not use `omitempty`.
- Do not expose a way for chat-compatible clients to set `true` for Codex backend.

## Client `store` Extra Parameter

Current owner: `model.ChatCompletionRequest.ExtraParams`.

Because `store` is not a known global chat field, JSON `store` currently lands in `ExtraParams`.

Codex backend validation rules:

| Client value | Codex behavior |
| --- | --- |
| omitted | send `store:false` |
| `false` | accept and send `store:false` |
| `true` | reject locally |
| non-boolean | reject locally |

Normal OpenAI API-key behavior:

- Keep `store` as an `ExtraParams` pass-through field for existing OpenAI provider behavior unless future scope explicitly changes global chat parsing.

## Test Fixtures

### Codex backend mock

- Local `httptest.Server`.
- Records request path, headers, and decoded JSON body.
- Returns a synthetic Codex Responses-like body.
- Must not print bearer token values.

### Store sentinel requests

```json
{
  "model": "openai/gpt-5.5",
  "messages": [
    {"role": "system", "content": "You are a connectivity probe."},
    {"role": "user", "content": "Reply exactly OK."}
  ],
  "store": false
}
```

```json
{
  "model": "openai/gpt-5.5",
  "messages": [
    {"role": "user", "content": "hello"}
  ],
  "store": true
}
```

## Secret Safety

Tests may use fake strings such as `access-secret`, but assertions and failure messages must not dump raw `Authorization` headers, access tokens, refresh tokens, encrypted credential values, or raw credential JSON.
