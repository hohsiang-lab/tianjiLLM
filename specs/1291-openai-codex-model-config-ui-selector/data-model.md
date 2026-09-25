# Data Model: OpenAI subscription Codex transport selector

## `config.TianjiParams`

Add:

```go
OpenAISubscriptionTransport string `yaml:"openai_subscription_transport,omitempty"`
```

## JSON/YAML Field

```json
{
  "model": "openai/*",
  "openai_subscription_credential_ids": ["cred-a", "cred-b"],
  "openai_subscription_transport": "chatgpt_codex_backend"
}
```

## Allowed Values

| Value | Meaning |
|---|---|
| `direct_openai_http` | Use existing subscription credential bearer token against the official OpenAI Platform API path. |
| `chatgpt_codex_backend` | Use HO-1288 ChatGPT Codex backend transport. |

## Persistence Rules

- If `openai_subscription_credential_ids` is non-empty, `openai_subscription_transport` must be present and known.
- If `openai_subscription_credential_ids` is empty, Models UI should omit `openai_subscription_transport`.
- Edit form must preserve unknown `tianji_params` fields while updating this field.
- Runtime DB decoder must map this key into `config.TianjiParams.OpenAISubscriptionTransport`.

## UI View Model

Extend `pages.ModelRow` with safe transport summary, for example:

```go
OpenAISubscriptionTransport string
```

No credential secret fields are added to the view model.
