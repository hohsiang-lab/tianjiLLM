# Contract: Subscription wildcard routing regression

## Subscription Codex Wildcard Request

Given:

```yaml
model_list:
  - model_name: "openai/*"
    tianji_params:
      model: "openai/*"
      openai_subscription_credential_ids:
        - "cred-a"
      openai_subscription_transport: "chatgpt_codex_backend"
```

When caller sends:

```json
{
  "model": "openai/gpt-5.5",
  "messages": [{ "role": "user", "content": "hello" }]
}
```

Then:

- wildcard lookup resolves the route from `openai/*`
- transport class is `chatgpt_codex_backend`
- upstream path is `/backend-api/codex/responses`
- Platform `/v1/chat/completions` receives zero calls
- response is OpenAI-compatible chat completion JSON on success

## Wrong-Route Failure Contract

If subscription wildcard traffic reaches Platform `/v1/chat/completions`, the test must fail with:

- route class/path that was wrong
- model alias requested
- redacted credential/token evidence only

The failure must not include bearer values.

## Codex Diagnostics Contract

For mocked Codex backend failures:

| Upstream status | Upstream code | Expected external status | Expected fields |
| --- | --- | --- | --- |
| 401 | `invalid_token` | 401 | `error.type`, `error.code`, safe `error.message` |
| 403 | `missing_scope` | 403 | `error.type`, `error.code`, safe `error.message` |
| 429 | `insufficient_quota` | 429 | `error.type`, `error.code`, safe `error.message` |

## API-Key Wildcard Contract

Given:

```yaml
model_list:
  - model_name: "openai/*"
    tianji_params:
      model: "openai/*"
      api_key: "sk-api-key-sentinel"
      api_base: "<local-platform-mock>/v1"
```

When caller sends `/v1/chat/completions` with `model = "openai/gpt-4o-mini"`, then:

- route uses normal OpenAI-compatible provider
- upstream path is `/chat/completions` or equivalent base-relative chat-completions path
- Codex backend receives zero calls
- request body contains `messages`
- request body does not contain Codex-only `input` or `instructions` unless caller originally supplied compatible fields handled by the normal OpenAI provider
