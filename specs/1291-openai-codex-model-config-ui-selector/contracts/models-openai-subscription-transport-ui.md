# Contract: Models OpenAI subscription transport UI/config

## Routes

No new route is required.

Changed existing routes:

| Route | Method | Behavior change |
|---|---|---|
| `/ui/models` | GET | Renders transport selector/summary for subscription-capable model forms. |
| `/ui/models/table` | GET | Shows safe transport summary for subscription-backed rows. |
| `/ui/models/create` | POST | Accepts `openai_subscription_transport`. |
| `/ui/models/edit?model_id=...` | GET | Prefills selected transport. |
| `/ui/models/update` | POST | Updates/removes `openai_subscription_transport`. |

## Form Contract

### Codex backend wildcard

```text
model_name=openai/*
model=openai/*
api_base=
openai_subscription_credential_ids=cred-primary
openai_subscription_transport=chatgpt_codex_backend
```

Expected `tianji_params`:

```json
{
  "model": "openai/*",
  "openai_subscription_credential_ids": ["cred-primary"],
  "openai_subscription_transport": "chatgpt_codex_backend"
}
```

### Direct HTTP subscription path

```text
model_name=openai/gpt-5.2-codex
model=openai/gpt-5.2-codex
openai_subscription_credential_ids=cred-primary
openai_subscription_transport=direct_openai_http
```

Expected `tianji_params`:

```json
{
  "model": "openai/gpt-5.2-codex",
  "openai_subscription_credential_ids": ["cred-primary"],
  "openai_subscription_transport": "direct_openai_http"
}
```

### API-key path

```text
model_name=openai/*
model=openai/*
api_key=$OPENAI_API_KEY
```

Expected `tianji_params`:

```json
{
  "model": "openai/*",
  "api_key": "$OPENAI_API_KEY"
}
```

No transport field is required.

## Invalid Combinations

- `openai_subscription_credential_ids` present and `openai_subscription_transport` missing.
- Unknown `openai_subscription_transport`.
- Subscription credentials plus non-default custom `api_base`.
- Subscription credentials on non-OpenAI provider model.

## Safe Display Contract

Allowed:

- Transport label: `Platform direct HTTP` or `ChatGPT Codex backend`.
- Credential safe metadata already allowed by HO-1172.

Forbidden:

- `credential_value`
- raw access token
- raw refresh token
- raw `id_token`
- bearer strings
- JWT-looking strings
- encrypted credential blob
- arbitrary raw credential JSON
