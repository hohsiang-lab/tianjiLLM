# Contract: Codex backend chat payload `store:false`

## Route

```text
POST /v1/chat/completions
```

Selected model config:

```yaml
model_name: openai/*
tianji_params:
  model: openai/*
  openai_subscription_credential_ids: ["cred-a"]
  openai_subscription_transport: chatgpt_codex_backend
```

## Upstream Request

```text
POST <codex_backend_base_url>/codex/responses
```

Required body fields:

```json
{
  "model": "gpt-5.5",
  "instructions": "system: You are a connectivity probe.",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Reply exactly OK."
        }
      ]
    }
  ],
  "store": false
}
```

Rules:

- `store` must be present.
- `store` must be `false`.
- Raw OpenAI chat `messages` must not be forwarded to Codex backend.
- `Authorization` uses the resolved subscription bearer but must never be printed in test output.

## Client `store` Compatibility

| Client request | Expected route result |
| --- | --- |
| omit `store` | accepted; upstream body contains `store:false` |
| `store:false` | accepted; upstream body contains `store:false` |
| `store:true` | local HTTP 400; upstream not called |
| `store:"false"` | local HTTP 400; upstream not called |
| `store:false` plus unsupported `parallel_tool_calls` | local HTTP 400 for unsupported unknown parameters |

## Normal OpenAI Provider Boundary

API-key-backed OpenAI chat-completions routes are not part of the Codex backend contract. Their existing `ExtraParams` pass-through behavior must remain unchanged.
