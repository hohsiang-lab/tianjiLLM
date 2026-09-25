# Quickstart: OpenAI Subscription Config and Credential Resolution

## Current Todo State

This is a docs-only planning branch. Do not start production implementation until Linear moves to In Progress.

## Example Config

```yaml
model_list:
  - model_name: codex-subscription
    tianji_params:
      model: openai/gpt-5.2-codex
      openai_subscription_credential_ids:
        - cred_openai_subscription_main
```

## Expected Behavior

1. Config load parses `openai_subscription_credential_ids`.
2. Validation confirms provider is official OpenAI/default base and no `api_base` is configured.
3. Router selects the configured deployment normally.
4. Resolver looks up exactly `cred_openai_subscription_main`.
5. Resolver rejects missing/disabled/invalid credentials with no API-key fallback.
6. Direct OpenAI HTTP paths receive bearer material for later injection.
7. Codex app-server paths receive an `account/login/start` payload using `chatgptAuthTokens`.
8. Local stdio app-server launches strip `CODEX_API_KEY` and `OPENAI_API_KEY`.
9. WebSocket app-server connection auth remains separate from OpenAI account auth.

## Verification Commands After Implementation

```bash
go test ./internal/config/... -run 'TestLoad_OpenAISubscription|TestValidate_OpenAISubscription' -v
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscription|TestResolveProvider_NoSubscription|TestDirectOpenAIHTTPResolution' -v
go test ./internal/codexapp/... -run 'TestCodexAppServer' -v
go test ./internal/config/... ./internal/proxy/handler/... ./internal/router/... -v
```

## Manual Review Checklist

- Config with omitted IDs still uses existing `api_key`.
- Config with IDs plus `api_base` fails before serving traffic.
- Config with custom provider plus IDs fails before serving traffic.
- Resolver never scans all org credentials.
- Resolver never falls back to `api_key` after a configured subscription ID fails.
- App-server login RPC and env shaping are tested against the checked Codex protocol version.
