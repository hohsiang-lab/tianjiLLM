# Contract: ChatGPT Codex backend transport

## Config Contract

### Transport selection

Per-model config must explicitly select the backend transport:

```yaml
tianji_params:
  openai_subscription_credential_ids: ["cred-a"]
  openai_subscription_transport: chatgpt_codex_backend
```

### Backend defaults

```yaml
general_settings:
  openai_oauth:
    codex_backend_base_url: https://chatgpt.com/backend-api
    codex_backend_originator: codex_cli_rs
```

The final request URL is normalized to:

```text
<codex_backend_base_url>/codex/responses
```

## HTTP Request Contract

### Method and URL

```text
POST https://chatgpt.com/backend-api/codex/responses
```

### Required headers

```text
Authorization: Bearer <subscription_access_token>
Content-Type: application/json
OpenAI-Beta: responses=experimental
originator: codex_cli_rs
```

### Conditional headers

```text
ChatGPT-Account-Id: <subscription_account_id>
Accept: text/event-stream
```

`ChatGPT-Account-Id` is sent when `account_id` exists. `Accept: text/event-stream` is required for streaming requests and acceptable for backend responses that stream SSE.

## Body Contract

The body must be Responses-style JSON, not raw Platform chat completions JSON copied blindly.

Minimum fields:

```json
{
  "model": "gpt-5.2-codex",
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "hi"
        }
      ]
    }
  ],
  "store": false,
  "stream": false
}
```

Implementation may include supported options such as instructions or text format when mapped from existing Tianji request params.

## Error Contract

- Credential lookup/refresh failures return a safe Tianji error.
- Backend 401/403/429/5xx responses are mapped through existing upstream error handling where possible.
- Raw access token, refresh token, encrypted credential value, raw credential JSON, and full bearer header must never appear in response body or logs.

## Non-Goals

- This contract does not define Codex app-server `account/login/start`.
- This contract does not replace OpenAI Platform `/v1/responses` or `/v1/chat/completions`.
- This contract does not make all existing `openai_subscription_credential_ids` routes use ChatGPT backend automatically.
