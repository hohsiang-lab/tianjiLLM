# Implementation Plan: OpenAI subscription Codex transport model config/UI selector

**Branch**: `HO-1291-openai-codex-model-config-ui`
**Linear**: HO-1291
**Spec**: [spec.md](spec.md)
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`.

## Summary

Finish the explicit `openai_subscription_transport` model config path in Models UI so selected OpenAI subscription credentials do not implicitly decide whether a request uses Platform direct HTTP or ChatGPT Codex backend. `origin/main` now already contains the typed config field, runtime DB decoder, and backend routing hook; this issue owns the remaining UI persistence and validation hardening only.

## Technical Context

- Language/runtime: Go + `templ` + HTMX.
- Model config type: `internal/config/config.go` `TianjiParams` already includes `OpenAISubscriptionTransport`.
- Config validation: `internal/config/validate.go` currently validates selected credential IDs, provider name, `api_base`, and rejects Codex transport without credentials; it still needs missing/unknown transport validation for subscription-backed rows.
- DB JSON decode: `internal/proxy/handler/runtime_model_source.go` already maps `openai_subscription_transport` into typed `TianjiParams`.
- Models UI handlers: `internal/ui/handler_models.go` build and merge `tianji_params` from forms.
- Models UI template: `internal/ui/pages/models.templ` already has `openAISubscriptionCredentialsSelector`.
- Existing E2E: `test/e2e/models_openai_subscription_test.go` covers credential selection and custom `api_base` conflict.

## Constitution Check

| Principle | Status | Notes |
|---|---:|---|
| Spec-first | PASS | Todo phase changes only this specs directory. |
| Repo reality first | PASS | Plan was refreshed against `origin/main` `3fa617a`, including the HO-1288/HO-1290 config/runtime/backend baseline. |
| Research before build | PASS | research.md records repo and official OpenAI Codex/API evidence. |
| Failing-tests-first | PASS | tasks.md starts with RED config/UI/E2E tests. |
| Security | PASS | Selector displays config choice only; token material remains forbidden in DOM. |
| Narrow scope | PASS | No transport HTTP implementation or response adapter work here. |

## Architecture Decision

### D1 - Reuse typed config field, not `Overflow`-only behavior

`origin/main` already has:

```go
OpenAISubscriptionTransport string `yaml:"openai_subscription_transport,omitempty"`
```

Keep DB/YAML key stable as `openai_subscription_transport`. Do not re-add or rename this field during HO-1291 implementation.

### D2 - Enum constants should live near config/handler boundary

Use shared constants for:

```text
direct_openai_http
chatgpt_codex_backend
```

`chatgpt_codex_backend` is already exported from `internal/config`; `direct_openai_http` currently exists at the handler boundary. Implementation should expose or centralize an allowlist so validation and UI do not duplicate untested string literals.

### D3 - Require transport only when subscription credentials are selected

Validation rule:

```text
len(openai_subscription_credential_ids) > 0
  -> openai_subscription_transport must be one of known values
len(openai_subscription_credential_ids) == 0
  -> openai_subscription_transport should be omitted/ignored by UI save
```

This preserves old API-key rows and avoids requiring migration for non-subscription models.

### D4 - UI owns explicit selection, not implicit default

Create/Edit forms should include a compact selector near the OpenAI subscription credential selector:

```text
OpenAI Subscription Transport
  Platform direct HTTP
  ChatGPT Codex backend
```

When a credential is selected and no transport has been chosen, server validation should reject. The UI may default the visible selection to Codex for operator convenience only if tests still prove the saved field is explicit.

### D5 - Runtime decode must not drop DB-created transport

`runtime_model_source.go` already maps `model`, `api_key`, `api_base`, `api_version`, `openai_subscription_credential_ids`, `openai_subscription_transport`, `tpm`, `rpm`, `timeout`, `region`, and auto-router keys. HO-1291 should add or keep regression coverage so DB-created rows do not drop this field.

## Target Flow

```text
Admin /ui/models
  -> select model openai/*
  -> select one or more OpenAI subscription credentials
  -> choose ChatGPT Codex backend
  -> POST /ui/models/create
  -> handler validates provider/api_base/transport
  -> ProxyModelTable.tianji_params:
       model: openai/*
       openai_subscription_credential_ids: [...]
       openai_subscription_transport: chatgpt_codex_backend
  -> runtime model source decodes typed TianjiParams.OpenAISubscriptionTransport
```

## Repo Evidence

- `internal/config/config.go` already has `OpenAISubscriptionTransport`.
- `internal/config/validate.go` validates credentials and rejects Codex transport without credentials, but still does not require a known transport when credentials are selected.
- `internal/proxy/handler/openai_subscription_resolution.go` currently has `direct_openai_http`, legacy `codex_app_server`, and `chatgpt_codex_backend`; chat routing can switch to Codex backend when the typed config field is present.
- `internal/ui/handler_models.go` persists `openai_subscription_credential_ids` but no transport field.
- `internal/ui/pages/models.templ` renders credential selector and conflict warning, but no transport selector.
- `test/e2e/models_openai_subscription_test.go` is the right E2E home for create/edit transport coverage.

## External Evidence

- Official OpenAI Help says Codex can be used through ChatGPT plans and ChatGPT account sign-in, which is separate from ordinary API-key usage.
- Official OpenAI Codex CLI docs state Codex can authenticate with a ChatGPT account or API key, reinforcing that subscription-backed and API-key-backed modes are distinct operator choices.
- Official OpenAI API model docs list GPT-5.2-Codex as available on public API endpoints such as `/v1/chat/completions` and `/v1/responses`; they do not document `chatgpt.com/backend-api/codex/responses` as a public Platform endpoint. Therefore Tianji must keep Platform API-key routing valid and make private ChatGPT backend routing explicit.

## Risk Register

| Risk | Mitigation |
|---|---|
| Existing subscription rows without transport become invalid after deploy | Implementation should document whether existing rows need a data update; validation can allow legacy DB rows until edited only if owner accepts. New saves must be explicit. |
| UI silently defaults to wrong backend | Add E2E assertions on saved DB JSON for Codex/direct modes. |
| Stale transport remains after credentials are removed | Add edit test that clears credentials and asserts field removal. |
| API-key OpenAI wildcard breaks | Add regression test for API-key `openai/*` without transport. |
| String literal drift between config/UI/backend | Use constants or a single option list and config validation tests. |
| Token material leaks in selector/table | Reuse HO-1172 safe metadata tests and add transport UI DOM negative assertion if needed. |

## Verification Commands

```bash
go test ./internal/config/... -run 'Test.*OpenAISubscription.*Transport|Test.*OpenAI.*Subscription' -count=1
go test ./internal/ui/... -run 'Test.*OpenAISubscription.*Transport|Test.*ModelOpenAISubscription' -count=1
go test -tags e2e ./test/e2e -run 'TestModelOpenAISubscription_.*Transport|TestModelOpenAISubscription' -count=1
git diff --check origin/main...HEAD
```

## Todo Gate Status

Spec/plan/tasks/analyze are complete when this docs-only branch is pushed and the draft PR exists. Implementation remains blocked until Linear moves to `In Progress`.
