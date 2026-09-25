# Quickstart: HO-1172 implementation and verification

## Governance

Production implementation is blocked until Linear HO-1172 is moved to `In Progress`。

```bash
linear_get_issue HO-1172
```

Expected before implementation: state is `In Progress`。

## Implementation Checklist

1. Add failing E2E tests for create/edit multi-select, invalid custom `api_base`, API-key regression, and no-secret DOM。
2. Add handler/unit tests for parsing selected IDs and building safe credential summaries。
3. Implement safe credential option loading from existing DB methods。
4. Extend Models page data and row view models。
5. Add selector UI to Create/Edit dialogs and table summaries。
6. Update create/update handlers to write/delete `openai_subscription_credential_ids`。
7. Regenerate templ output。
8. Run targeted tests and diff sanity。

## Manual Verification

With seeded DB:

1. Open `/ui/models`。
2. Click `Add Model`。
3. Select `OpenAI primary` and `OpenAI backup` credentials。
4. Leave `API Base` empty。
5. Submit and confirm table shows subscription credential badges。
6. Inspect DB `ProxyModelTable.tianji_params` and confirm exact selected IDs。
7. Edit the model, remove one credential, save, and confirm DB round-trip。
8. Try selecting a credential with `API Base=https://custom.api.com`; confirm error and no DB mutation。
9. Create an API-key model without selected credentials; confirm existing behavior remains unchanged。

## Commands

```bash
templ generate
go test ./internal/ui/... -run 'Test.*Model.*Subscription|TestMaskAPIKey|TestHandleSyncPricing' -count=1
go test ./test/e2e -tags e2e -run 'TestModel(Create|Edit)_OpenAISubscription|TestModelOpenAISubscription|TestModelCreate_APIKeyRegression' -count=1
go test ./internal/proxy/handler/... ./internal/callback/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

## Expected PR Evidence

- PR layer: TianjiLLM admin Models UI。
- Runtime boundary: browser -> HTMX Models handlers -> `ProxyModelTable.tianji_params` and `CredentialTable` safe metadata。
- Test command: targeted Models E2E plus `internal/ui` tests。
- CI evidence: full PR workflow green。
- Gap/follow-up: none expected; lifecycle and refresh behavior remain sibling backend issue scope。
