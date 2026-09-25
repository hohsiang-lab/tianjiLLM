# Contract: OpenAI Subscription Config and Credential Resolution

## YAML Contract

Valid subscription config:

```yaml
model_list:
  - model_name: codex-subscription
    tianji_params:
      model: openai/gpt-5.2-codex
      openai_subscription_credential_ids:
        - cred_openai_subscription_main
```

Valid existing API-key config:

```yaml
model_list:
  - model_name: gpt-4o-api-key
    tianji_params:
      model: openai/gpt-4o
      api_key: "$OPENAI_API_KEY"
```

Invalid custom base:

```yaml
model_list:
  - model_name: custom-openai
    tianji_params:
      model: openai/gpt-4o
      api_base: https://proxy.example.test/v1
      openai_subscription_credential_ids:
        - cred_openai_subscription_main
```

Invalid custom provider:

```yaml
model_list:
  - model_name: openrouter-gpt
    tianji_params:
      model: openrouter/openai/gpt-5.2
      openai_subscription_credential_ids:
        - cred_openai_subscription_main
```

## Validation Contract

`config.Load` / `LoadWithSecrets` must return a config error for:

- empty ID in the list
- duplicate ID in the list
- non-empty `api_base` combined with subscription IDs
- provider prefix not equal to `openai`
- OpenAI-compatible custom provider attempting subscription credentials

## Runtime Contract

No subscription IDs:

```text
selected deployment -> existing api_key/APIBase provider resolution
```

Subscription IDs:

```text
selected deployment -> exact credential ID lookup -> openai_subscription type check -> decrypt/refresh/status check -> transport-specific output
```

Failure:

```json
{
  "error": {
    "type": "credential_resolution_error",
    "message": "OpenAI subscription credential cred_x is disabled"
  }
}
```

No failure path may return API-key fallback when IDs were configured.

## Direct HTTP Contract

Output boundary:

```json
{
  "transport": "direct_openai_http",
  "base_url": "https://api.openai.com/v1",
  "source_credential_id": "cred_openai_subscription_main",
  "bearer_token": "<secret>"
}
```

The bearer token is secret and must not appear in logs, HTTP responses, or persisted error data.

## Codex App-Server Contract

Output boundary:

```json
{
  "method": "account/login/start",
  "params": {
    "type": "chatgptAuthTokens",
    "accessToken": "<secret>",
    "chatgptAccountId": "acct_123",
    "chatgptPlanType": "plus"
  }
}
```

Exact field names are version-sensitive and must be verified against the checked Codex app-server protocol source during implementation.

## Local Stdio Env Contract

Given a subscription-style profile, the spawned app-server env must omit:

- `CODEX_API_KEY`
- `OPENAI_API_KEY`

Other env values are preserved unless another issue owns additional filtering.

## WebSocket Contract

`appServer.authToken` and connection headers authenticate the app-server connection only. OpenAI subscription credentials must still be sent through app-server account login RPC after connection initialization.
