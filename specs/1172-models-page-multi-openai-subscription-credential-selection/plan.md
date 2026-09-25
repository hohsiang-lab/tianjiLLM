# Implementation Plan: Models page multi OpenAI subscription credential selection

**Branch**: `HO-1172-models-page-multi-openai-subscription-credential`
**Spec**: `specs/1172-models-page-multi-openai-subscription-credential-selection/spec.md`
**Linear**: HO-1172
**Phase**: Todo planning only; production implementation starts only after Linear moves to `In Progress`。

## Technical Context

- Language/runtime: Go 1.26 module `github.com/praxisllmlab/tianjiLLM`。
- UI stack: server-rendered `templ` components plus HTMX partial swaps and existing Tailwind utility classes。
- Models UI: `internal/ui/handler_models.go` and `internal/ui/pages/models.templ`。
- Existing model persistence: `ProxyModelTable.tianji_params` JSON with `model`, `api_base`, `api_key`, `tpm`, `rpm`, unknown-field preservation in update path。
- Subscription config field: `config.TianjiParams.OpenAISubscriptionCredentialIDs []string` maps to `openai_subscription_credential_ids` from HO-1167。
- Credential source: `CredentialTable` rows from sqlc-backed `ListCredentials` / `GetCredential`，filtered to `credential_type="openai_subscription"`。
- Safe metadata pattern: HO-1186 `handler_credentials.go` parses narrow `openAISubscriptionInfo` and redacts strings before rendering。
- Existing E2E: `test/e2e/models_create_test.go`、`models_edit_test.go`、`credentials_test.go`、fixtures in `test/e2e/helpers_test.go`。
- CI runner follow-up: `.github/workflows/ci.yml` currently defines one workflow with `lint`、`test`、`e2e`、`build`、`docker` jobs; owner requested routing these jobs to the staging ARC scale set runner label `staging-monster-ci` after the original Waiting Merge gate。

## Constitution Check

- **Spec-first**: Todo state allows only SpecKit artifacts, mockscreen, branch, draft PR。
- **Repo reality first**: Plan is grounded in existing Models/Credentials UI handlers, `templ` pages, DB queries, and E2E fixtures。
- **TDD**: First implementation phase writes failing UI E2E before production changes。
- **No new frontend stack**: Reuse existing Go `templ` + HTMX + component library。
- **Secret boundary**: View models must never include `credential_value` or raw token metadata。
- **State gate**: Implementation remains blocked until Linear `In Progress`。

## Architecture

### Data Adapter

Add model-credential option loading in `internal/ui/handler_models.go` or a small adjacent helper:

- `loadOpenAISubscriptionCredentialOptions(ctx)`:
  - calls existing `h.DB.ListCredentials(ctx)`
  - filters `CredentialType == "openai_subscription"`
  - parses only safe metadata (`email`, `status`)
  - returns stable option rows sorted by credential name or ID
  - never decrypts `CredentialValue`
- `subscriptionCredentialSummary(ids []string, options []Option)`:
  - preserves configured order
  - marks missing/wrong-type IDs as missing badges
  - avoids silently dropping old config before explicit save

If implementation needs a narrower query, add a sqlc query under the existing DB pattern. Do not use handwritten SQL in UI handlers.

### View Models

Extend `pages.ModelRow` and `pages.ModelsPageData` with fields similar to:

- `OpenAISubscriptionCredentialIDs []string`
- `OpenAISubscriptionCredentials []ModelCredentialSummary`
- `OpenAISubscriptionCredentialOptions []ModelCredentialOption`

Create/Edit forms need option data. Table rows need summary data.

### Create Handler

In `handleModelCreate`:

1. Parse repeated form values `r.Form["openai_subscription_credential_ids"]` after `r.ParseForm()`。
2. Trim blank IDs and deduplicate while preserving order。
3. If selected IDs are non-empty and `api_base` is non-empty, render error through `ModelsTableWithToast` and keep dialog open。
4. Build `tianji_params` with `openai_subscription_credential_ids` only when selected IDs are non-empty。
5. Preserve existing `api_key`, `api_base`, `tpm`, `rpm` behavior when selected IDs are empty。

### Edit Handler

In `handleModelUpdate`:

1. Parse repeated selected credential IDs。
2. If selected IDs are non-empty and form `api_base` is non-empty, render error and keep dialog open。
3. Merge into existing parsed `tianji_params`。
4. If selected IDs is empty, delete `openai_subscription_credential_ids` from the JSON map。
5. Preserve `api_key` when input is empty and preserve unknown fields。

### Templ UI

Add a reusable selector component in `internal/ui/pages/models.templ`:

- Use checkbox list with repeated `name="openai_subscription_credential_ids"` values。
- Include credential name, ID, safe email, status badge。
- Include visible selected-count summary。
- Use stable `data-testid` for E2E targeting。
- Disable or visually mark conflict with custom `api_base` through small inline script only if needed; server validation remains authoritative。

Models table gains a compact "OpenAI subscription" column or augments API Key column with selected credential badges. The UI should avoid nested cards and keep table width readable by using short badges with tooltip/title metadata.

### Validation UX

Server-side validation is mandatory. Client-side behavior is convenience only:

- Conflict: selected IDs + non-empty `api_base`。
- Error response must not close create/edit dialog。
- Dialog inputs should remain available for correction。
- API-key-only model flow must not see subscription-specific errors。

### Security

Display only:

- `credential_name`
- `credential_id`
- safe/redacted `email`
- safe/redacted `status`

Never pass to templates:

- `CredentialValue`
- decrypted token bundle
- `access_token`
- `refresh_token`
- `id_token`
- bearer/JWT-looking values
- arbitrary raw `credential_info` map

## Plan Review Evidence

- Repo reality: `internal/ui/pages/models.templ` currently renders Create/Edit dialogs with `api_base` and `api_key`; `internal/ui/handler_models.go` builds/merges `tianji_params` maps and preserves unknown fields on edit。
- Repo reality: `internal/ui/handler_credentials.go` already demonstrates safe OpenAI subscription metadata parsing and list filtering by `credential_type`。
- Repo reality: `test/e2e/models_create_test.go` and `models_edit_test.go` already cover API-key create/edit and are the regression base。
- Context7 `/a-h/templ`: dynamic attributes and `{ attrs... }` spreads are supported, matching current component patterns for IDs, ARIA labels, HTMX attributes, and data-testid fields。
- Context7 `/bigskysoftware/htmx`: `hx-post` forms can submit form data and swap HTML into `hx-target`; `hx-target` and `hx-swap` match the existing Models table/dialog pattern。
- grep.app: public code search for `templ.Attributes` shows common Go `templ` code uses `templ.Attributes` maps for input/button attributes; exact public `templ` + HTMX Models-form prior art was not useful, so repo-local Tianji UI patterns remain the stronger source。
- External OpenAI docs are not a dependency for this UI because implementation must not call OpenAI; it only reads stored credential metadata produced by sibling backend issues。

## Failing Tests

| Test | File | Initial failure | Covers |
| --- | --- | --- | --- |
| `TestModelCreate_OpenAISubscriptionCredentialMultiSelect` | `test/e2e/models_openai_subscription_credentials_test.go` | selector missing | FR-001, FR-003, FR-004 |
| `TestModelEdit_OpenAISubscriptionCredentialRoundTrip` | same | edit prefill missing | FR-002, FR-007 |
| `TestModelCreate_BlocksSubscriptionCredentialsWithCustomAPIBase` | same | validation missing | FR-006 |
| `TestModelCreate_APIKeyRegressionWithoutSubscriptionCredentials` | same or existing create test | regression guard | FR-005 |
| `TestModelOpenAISubscriptionCredentials_NoSecretDOM` | same | safe rendering missing | FR-009, FR-010 |
| `TestBuildModelRow_OpenAISubscriptionCredentialIDs` | `internal/ui/handler_models_test.go` | row field missing | FR-008, FR-011 |
| `TestParseModelOpenAISubscriptionCredentialIDs` | `internal/ui/handler_models_test.go` | parser missing | FR-004, FR-007 |

## Implementation Phases

### Phase 1 - RED tests first

Add E2E and handler tests listed above. Confirm failures are missing UI/parser behavior, not environment failure.

### Phase 2 - Safe credential option adapter

Add safe option loading/filtering and summary helpers using existing DB methods and redaction pattern.

### Phase 3 - Models view model and templ selector

Extend Models page data/rows, add selector component, render create/edit fields and table summaries, regenerate templ output.

### Phase 4 - Create/update persistence

Parse repeated credential IDs, validate conflict with `api_base`, write/delete `openai_subscription_credential_ids`, preserve API-key and unknown-field behavior.

### Phase 5 - Verification

Run targeted E2E, UI tests, credential/security tests, `templ generate`, lint, and diff sanity before state transitions.

## Verification Commands

```bash
go test ./test/e2e -tags e2e -run 'TestModel(Create|Edit)_OpenAISubscription|TestModelOpenAISubscription|TestModelCreate_APIKeyRegression' -count=1
go test ./internal/ui/... -run 'Test.*Model.*Subscription|TestMaskAPIKey|TestHandleSyncPricing' -count=1
go test ./internal/proxy/handler/... ./internal/callback/... -count=1
go tool golangci-lint run
templ generate
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/ci.yml")'
git diff --check origin/main...HEAD
```

If local E2E lacks required Postgres/browser services, CI E2E remains the merge gate, but implementation must still run available targeted unit/render tests locally.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| UI silently drops missing selected IDs | Represent missing IDs explicitly and only remove on admin save。 |
| Subscription IDs save with custom `api_base` | Server-side create/update validation plus E2E conflict test。 |
| Token metadata leaks into DOM | Narrow safe struct, redaction, DOM secret assertion。 |
| API-key flow regresses | Keep no-selection branch identical and run existing create/edit tests。 |
| Unknown `tianji_params` lost on edit | Merge into existing map and test unknown field preservation。 |
| Selector becomes too wide in table/dialog | Use compact badges, stable dimensions, wrapping text, no nested cards。 |
| Staging runner migration accidentally leaves some CI jobs on GitHub-hosted runners | `rg -n "runs-on:|ubuntu-latest|staging-monster-ci" .github/workflows/ci.yml` and PR CI pickup evidence before returning to Waiting Merge。 |

## Todo Gate Status

Spec/plan/tasks/analyze are complete and ready for draft PR review after mockscreen evidence and docs-only diff verification. Production implementation remains blocked until Linear `In Progress`。
