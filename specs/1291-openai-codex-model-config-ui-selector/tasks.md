# Tasks: OpenAI subscription Codex transport model config/UI selector

**Input**: [spec.md](spec.md), [plan.md](plan.md), [contracts/models-openai-subscription-transport-ui.md](contracts/models-openai-subscription-transport-ui.md)

## Phase 1 - RED tests

- [X] T001 Add config validation tests for missing, unknown, `direct_openai_http`, and `chatgpt_codex_backend` `openai_subscription_transport`.
- [X] T002 Add or keep runtime DB decoder regression test proving `openai_subscription_transport` maps into typed `config.TianjiParams`.
- [X] T003 Add UI handler unit tests for Create/Update persisting, switching, and removing transport.
- [X] T004 Add E2E create test for subscription-backed wildcard + Codex transport DB round-trip.
- [X] T005 Add E2E edit test for transport prefill/switch/remove.
- [X] T006 Add API-key `openai/*` regression test proving no transport is required without subscription credentials.

## Phase 2 - Config/data model

- [X] T007 Confirm and reuse the existing `config.TianjiParams.OpenAISubscriptionTransport` field from `origin/main`; do not re-add it.
- [X] T008 Add or expose shared allowed transport constants/validation helper for `direct_openai_http` and `chatgpt_codex_backend`.
- [X] T009 Update config validation to require known transport only when subscription credentials are present, while preserving the existing Codex-without-credentials rejection.
- [X] T010 Confirm and regression-test the existing runtime DB model-source decoder mapping for `openai_subscription_transport`.

## Phase 3 - Models UI handlers

- [X] T011 Parse `openai_subscription_transport` in Create/Update handlers.
- [X] T012 Persist transport when subscription credentials are selected.
- [X] T013 Remove transport when subscription credentials are cleared.
- [X] T014 Preserve unknown `tianji_params` fields during Edit.
- [X] T015 Return actionable UI error when transport is missing/unknown for selected credentials.

## Phase 4 - Models UI template

- [X] T016 Extend `pages.ModelRow` with safe transport summary.
- [X] T017 Add Create transport selector near OpenAI subscription credential selector.
- [X] T018 Add Edit transport selector with prefilled value.
- [X] T019 Add table summary badge/label for subscription transport.
- [X] T020 Ensure selector/table DOM contains no token material.

## Phase 5 - Validation and docs

- [X] T021 Regenerate templ output.
- [X] T022 Run targeted config/UI/E2E tests from quickstart.
- [X] T023 Run `make lint` or repo-standard lint gate if required.
- [X] T024 Run `git diff --check origin/main...HEAD`.
- [X] T025 Update this tasks file with implementation evidence before review.

## Implementation evidence

- `go test ./internal/config -run 'TestValidateOpenAISubscriptionTransport|TestValidateOpenAISubscriptionCredentialIDs' -count=1` passed.
- `go test ./internal/ui -run 'TestModelOpenAISubscription' -count=1` passed.
- `go test ./internal/proxy/handler -run 'TestRuntimeModelSource_DBManagedModelPreservesOpenAISubscriptionTransport|TestRuntimeModelSource_FindModelConfigUsesDBManagedModel' -count=1` passed.
- `go tool golangci-lint run` passed with 0 issues.
- `git diff --check origin/main...HEAD` and `git diff --check` passed.
- Local E2E execution was not run by agent per Sima implementation rule; E2E specs were added for CI coverage.

## Waiting artifact-review recovery - mockscreen

- [X] T026 Re-read Linear HO-1291, comments, PR #164, worktree, and Discord thread attachments.
- [X] T027 Diagnose original Todo mockscreen miss from thread/artifact/gate evidence.
- [X] T028 Generate desktop mockscreen PNG outside git.
- [X] T029 Generate mobile mockscreen PNG outside git.
- [X] T030 Visual sanity check screenshots for required selector structure, readability, and overlap.
- [X] T031 Upload desktop and mobile mockscreens to Discord thread.
- [X] T032 Update `analyze.md` with root cause and mockscreen scope alignment.

## Scope Guard

- Do not implement ChatGPT backend HTTP calls in this issue.
- Do not change OpenAI API-key provider routing.
- Do not call real OpenAI or ChatGPT endpoints.

## Waiting artifact-review recovery - main drift

- [X] T033 Re-read latest `origin/main` after HO-1288/HO-1290 merged.
- [X] T034 Update SpecKit docs so implementation does not re-add config/runtime pieces already present on main.

## Waiting CI recovery - result-screen gate

- [X] T035 Verify latest PR head CI run `25629676219` is green for `lint`, `test`, `e2e`, and `build`.
- [X] T036 Capture implementation desktop result screen from the actual Models UI showing the Add Model subscription transport selector.
- [X] T037 Capture implementation mobile result screen from the actual Models UI showing the Edit Model transport prefill.
- [X] T038 Upload desktop/mobile result screens to Discord thread before Waiting Merge.
