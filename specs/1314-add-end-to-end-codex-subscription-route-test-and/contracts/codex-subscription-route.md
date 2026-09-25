# Contract: Codex subscription route E2E

## Runtime Model Config

Subscription-backed wildcard route:

```yaml
model_name: openai/*
tianji_params:
  model: openai/*
  openai_subscription_credential_ids:
    - cred-fixture
  openai_subscription_transport: chatgpt_codex_backend
```

API-key wildcard guard:

```yaml
model_name: openai/*
tianji_params:
  model: openai/*
  api_key: sk-fixture
  api_base: http://platform.local/v1
```

## Client Request

```json
{
  "model": "openai/gpt-5.5",
  "stream": true,
  "messages": [
    {"role": "user", "content": "hello"}
  ]
}
```

## Expected Codex Backend Request

Path:

```text
POST /backend-api/codex/responses
```

Required safe assertions:

- `Authorization` is set internally from resolved subscription credential but never printed in test failure output.
- `ChatGPT-Account-Id` is set when the credential bundle has `account_id`.
- `originator` defaults to `codex_cli_rs`.
- `OpenAI-Beta` is `responses=experimental`.
- `Accept` is `text/event-stream` for streaming requests.

Expected JSON shape:

```json
{
  "model": "gpt-5.5",
  "store": false,
  "stream": true,
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [
        {"type": "input_text", "text": "hello"}
      ]
    }
  ]
}
```

## Expected Client SSE

Minimum successful output:

```text
data: {"object":"chat.completion.chunk",...}

data: [DONE]
```

## Error Contract

Credential lookup classes:

- `credential_missing`: no row matched the configured credential ID.
- `credential_lookup_failed`: DB/query/scan/runtime failure while reading an existing or expected row.
- `credential_wrong_type`: row exists but is not `openai_subscription`.
- `credential_disabled`: row metadata disables the credential.
- `credential_malformed`: metadata, decryption, or token bundle is invalid.
- `refresh_failed`: refresh request or persistence failed.
- `auth_failed_after_refresh`: upstream stayed 401 after forced refresh.

Security:

- Caller response must not contain fake access token, refresh token, ID token, cookie, decrypted JSON, or raw `Authorization`.
- Test logs must not print raw token-bearing headers.
