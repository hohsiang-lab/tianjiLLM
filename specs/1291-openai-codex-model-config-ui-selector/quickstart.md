# Quickstart: HO-1291 implementation verification

## Expected Config

```yaml
model_list:
  - model_name: openai/*
    tianji_params:
      model: openai/*
      openai_subscription_credential_ids:
        - cred-primary
      openai_subscription_transport: chatgpt_codex_backend
```

## Manual UI Check

1. Open `/ui/models`.
2. Click `Add Model`.
3. Fill `Model Name` and `Model` with `openai/*`.
4. Select an OpenAI subscription credential.
5. Select `ChatGPT Codex backend`.
6. Save and verify table shows the model plus safe transport summary.
7. Edit the row and verify the transport is preselected.
8. Clear all subscription credentials and save; verify DB no longer has `openai_subscription_transport`.

## Planning Mockscreen Check

Desktop and mobile mockscreens should show:

- Models table/card with a subscription-backed `openai/*` model.
- Add Model dialog/form with OpenAI subscription credential selection.
- Explicit `OpenAI Subscription Transport` selector with `Platform direct HTTP` and `ChatGPT Codex backend`.
- Custom API Base conflict warning.
- API-key path represented without requiring subscription transport.

## Targeted Test Commands

```bash
go test ./internal/config/... -run 'Test.*OpenAISubscription.*Transport|Test.*OpenAI.*Subscription' -count=1
go test ./internal/ui/... -run 'Test.*OpenAISubscription.*Transport|Test.*ModelOpenAISubscription' -count=1
go test -tags e2e ./test/e2e -run 'TestModelOpenAISubscription_.*Transport|TestModelOpenAISubscription' -count=1
git diff --check origin/main...HEAD
```

## Non-Goals For This Issue

Do not assert real upstream calls to ChatGPT or OpenAI. This issue only proves config/UI persistence and validation.
