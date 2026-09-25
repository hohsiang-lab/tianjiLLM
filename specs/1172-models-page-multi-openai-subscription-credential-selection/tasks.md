# Tasks: Models page multi OpenAI subscription credential selection

**Input**: SpecKit artifacts in `specs/1172-models-page-multi-openai-subscription-credential-selection/`
**Prerequisites**: Linear state must move to `In Progress` before production implementation。

## Phase 0: Governance

- [x] T001 Confirm Linear HO-1172 is `In Progress` before editing production files。
- [x] T002 Confirm work continues in the existing HO-1172 issue worktree and branch。
- [x] T003 Re-read HO-1167/HO-1186 relevant contracts before implementation。

## Phase 1: RED tests first

- [x] T004 [P] Add E2E `TestModelCreate_OpenAISubscriptionCredentialMultiSelect` that seeds two `openai_subscription` credentials and one `api_key`, opens Add Model, selects two credentials, submits, and expects DB exact IDs。
- [x] T005 [P] Add E2E `TestModelEdit_OpenAISubscriptionCredentialRoundTrip` that preloads one selected ID, asserts edit prefill, changes selection, saves, and verifies DB exact IDs。
- [x] T006 [P] Add E2E `TestModelCreate_BlocksSubscriptionCredentialsWithCustomAPIBase` that asserts conflict error and no DB mutation。
- [x] T007 [P] Add E2E `TestModelCreate_APIKeyRegressionWithoutSubscriptionCredentials` for no selected IDs with `api_key` / custom `api_base` existing behavior。
- [x] T008 [P] Add E2E `TestModelOpenAISubscriptionCredentials_NoSecretDOM` with token-looking credential metadata/value and assert rendered HTML omits secrets。
- [x] T009 [P] Add handler/unit test for parsing repeated `openai_subscription_credential_ids`: trim, drop blanks, dedupe, preserve order。
- [x] T010 [P] Add handler/unit test for selected credential summary including missing configured IDs。
- [x] T011 Run targeted tests and confirm failures are missing implementation, not fixture/environment failure。

## Phase 2: Safe credential option adapter

- [x] T012 Add safe OpenAI subscription credential option type in `internal/ui/pages/models.templ` or adjacent Go helper file。
- [x] T013 Add `loadOpenAISubscriptionCredentialOptions(ctx)` using existing sqlc-backed DB methods and filtering `credential_type="openai_subscription"`。
- [x] T014 Parse only safe metadata fields (`email`, `status`) with redaction; never pass `credential_value`。
- [x] T015 Add selected summary builder that preserves configured order and marks missing IDs。

## Phase 3: Models view models and table summary

- [x] T016 Extend `ModelRow` with selected `OpenAISubscriptionCredentialIDs` and safe credential summaries。
- [x] T017 Extend `ModelsPageData` with available OpenAI subscription credential options。
- [x] T018 Update `buildModelRowFromDB` and `buildModelRowFromConfig` to parse `openai_subscription_credential_ids` from DB/config。
- [x] T019 Update Models table UI to show compact OpenAI subscription credential summary without nested cards。
- [x] T020 Ensure table search/pagination refresh still renders summaries。

## Phase 4: Create/Edit selector UI

- [x] T021 Add reusable `openAISubscriptionCredentialSelector(...)` templ component with checkbox list and selected count。
- [x] T022 Render selector in Add Model dialog。
- [x] T023 Render selector in Edit Model dialog with checked existing IDs and missing badges。
- [x] T024 Add conflict hint for selected credentials + non-empty `api_base` while keeping server validation authoritative。
- [x] T025 Add `data-testid` selectors for E2E targeting and ARIA labels for accessibility。
- [x] T026 Regenerate templ output。

## Phase 5: Create/update persistence

- [x] T027 Update `handleModelCreate` to parse selected IDs and write `openai_subscription_credential_ids` only when non-empty。
- [x] T028 Update `handleModelCreate` to reject selected IDs plus custom `api_base` without closing dialog。
- [x] T029 Update `handleModelUpdate` to set/delete `openai_subscription_credential_ids` while preserving unknown fields。
- [x] T030 Update `handleModelUpdate` to reject selected IDs plus custom `api_base` without closing dialog。
- [x] T031 Preserve API-key edit behavior: empty `api_key` input keeps existing key。

## Phase 6: Security and regression

- [x] T032 Assert generic `api_key` credentials never appear in selector。
- [x] T033 Assert token-looking metadata/value strings never appear in Models table or dialogs。
- [x] T034 Assert missing selected IDs are visible and not silently removed before save。
- [x] T035 Assert existing `TestModelCreate_WithOptionalFields` and API-key preservation tests still pass。

## Phase 7: Verification and state progression

- [x] T036 Run `templ generate`。
- [x] T037 Run targeted E2E for HO-1172 Models subscription credential selection（E2E execution deferred to CI per Sima; local `go test -c -tags e2e` compile passed）。
- [x] T038 Run affected `internal/ui` tests。
- [x] T039 Run affected credential/callback/proxy tests if helper code touches shared redaction or credential adapters（N/A: no shared credential/proxy code changed）。
- [x] T040 Run `go tool golangci-lint run`。
- [x] T041 Run `git diff --check origin/main...HEAD`。
- [x] T042 Verify PR diff contains only HO-1172 implementation files plus this specs directory。
- [x] T043 Move Linear to `In Review` and run review gate after original implementation tasks complete。

## Phase 8: Waiting Merge runner follow-up

- [x] T044 Move Linear from `Waiting Merge` back to `In Progress` before editing workflow files。
- [x] T045 Update `.github/workflows/ci.yml` so `lint`、`test`、`e2e`、`build`、`docker` use `runs-on: staging-monster-ci`。
- [x] T046 Update SpecKit artifacts to record the owner-requested CI runner scope change。
- [x] T047 Validate workflow YAML parses and no `ubuntu-latest` remains in TianjiLLM CI jobs。
- [x] T048 Commit and push the runner follow-up to PR #158。
- [ ] T049 Re-run review/CI gates before returning HO-1172 to `Waiting Merge`。

## Scope Stop

After Todo planning, stop at `Waiting` with draft PR and mockscreen evidence. Waiting Merge is blocked until Linear moves through `In Progress`, `In Review`, `Waiting CI`, and CI/result-screen gates。
