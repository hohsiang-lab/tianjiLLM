# Quickstart: HO-1288 ChatGPT Codex backend transport

## Example Config Shape

Exact field names may change during implementation to match repo conventions, but the behavior must be equivalent:

```yaml
general_settings:
  openai_oauth:
    enabled: true
    codex_backend_base_url: https://chatgpt.com/backend-api
    codex_backend_originator: codex_cli_rs

model_list:
  - model_name: chatgpt/gpt-5.2-codex
    tianji_params:
      model: chatgpt/gpt-5.2-codex
      openai_subscription_credential_ids:
        - cred_openai_subscription_1
      openai_subscription_transport: chatgpt_codex_backend
```

## Expected Runtime Behavior

Calling:

```bash
curl -sS http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer <tianji-key>' \
  -H 'Content-Type: application/json' \
  -d '{"model":"chatgpt/gpt-5.2-codex","messages":[{"role":"user","content":"hi"}]}'
```

Should produce one upstream request to:

```text
POST https://chatgpt.com/backend-api/codex/responses
```

With headers:

```text
Authorization: Bearer <stored subscription access token>
ChatGPT-Account-Id: <stored account_id>
originator: codex_cli_rs
OpenAI-Beta: responses=experimental
Content-Type: application/json
```

## Regression Expectations

API-key OpenAI config remains unchanged:

```yaml
model_list:
  - model_name: gpt-4o
    tianji_params:
      model: openai/gpt-4o
      api_key: sk-...
```

This still calls:

```text
POST https://api.openai.com/v1/chat/completions
```

## Verification Commands

```bash
go test ./internal/config/... -run 'Test.*Codex.*Backend|Test.*OpenAI.*OAuth' -count=1
go test ./internal/provider/... ./internal/proxy/handler/... -run 'Test.*Codex.*Backend|Test.*OpenAIProvider|Test.*OpenAISubscription' -count=1
go test ./internal/config/... ./internal/provider/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Manual Verification Boundary

No manual verification should call real `chatgpt.com` during implementation review. Acceptance evidence should come from mocked upstream tests plus existing local credential resolver tests.
