# Tasks: OpenAI Subscription Spend and Audit Attribution

**Input**: Design documents from `/specs/1182-openai-subscription-spend-and-audit-attribution/`
**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/subscription-attribution.md`

**Tests**: Failing tests are mandatory. No production implementation task may begin until the relevant attribution/audit/redaction/API-key regression tests are written and confirmed to fail against current behavior.

## Phase 1: Test Baseline

**Purpose**: Lock expected attribution before changing logging/audit code.

- [X] T001 [P] Write failing success test for subscription `credential_id` in captured `callback.LogData`
- [X] T002 [P] Write failing spend tracker test that maps subscription attribution into `SpendLogs.metadata`
- [X] T003 [P] Write failing endpoint test where `cred_a` fails and `cred_b` succeeds, asserting success attribution uses `cred_b`
- [X] T004 [P] Write failing 401 forced-refresh retry test asserting attribution still uses the serving credential
- [X] T005 [P] Write failing refresh-failure attribution test with safe `reason_code=refresh_failed`
- [X] T006 [P] Write failing post-refresh-401 disable attribution test with safe `reason_code=auth_failed_after_refresh`
- [X] T007 [P] Write failing all-credentials-failed test asserting aggregate failure exposes only credential IDs and reason codes
- [X] T008 [P] Write failing API-key regression test asserting no subscription `credential_id` is fabricated
- [X] T009 Run targeted tests and confirm failures are current implementation gaps, not environment/network

**Checkpoint**: Subscription attribution expectations are locked.

## Phase 2: Safe Attribution Model

**Goal**: Define one safe runtime shape for subscription attribution.

- [X] T010 Add safe `OpenAISubscriptionAttribution` type or equivalent
- [X] T011 Add subscription attribution field to `callback.LogData` or an equivalent callback metadata path
- [X] T012 Add subscription attribution field to `spend.SpendRecord` or an equivalent spend metadata path
- [X] T013 Add helper to merge subscription attribution into `SpendLogs.metadata` without overwriting caller metadata
- [X] T014 Add redaction/validation guard ensuring forbidden token/JWT fields cannot be emitted from attribution helpers

**Checkpoint**: Attribution can move through callbacks/spend without carrying secrets.

## Phase 3: Selected Attempt Propagation

**Goal**: Attribute the actual credential that served or failed an OpenAI subscription attempt.

- [X] T015 Update `doOpenAISubscriptionProviderRequest` to expose selected serving credential attribution to caller context or response wrapper
- [X] T016 Update 429/5xx failover path so discarded failed attempts can emit safe failure attribution where relevant
- [X] T017 Update 401 forced-refresh path so refresh failure and post-refresh 401 carry safe credential/reason attribution
- [X] T018 Ensure callback goroutines receive a copied attribution snapshot before async dispatch
- [X] T019 Preserve no-subscription/API-key path with nil/empty subscription attribution
- [X] T020 Run selected-attempt attribution tests

**Checkpoint**: Success and failure paths know the real credential outcome.

## Phase 4: Spend and Callback Integration

**Goal**: Persist safe subscription attribution without corrupting API-key spend.

- [X] T021 Update `buildBaseLogData` or caller path to include subscription attribution when selected
- [X] T022 Update `logSuccess` and streaming success path to preserve subscription attribution
- [X] T023 Update image/audio/embedding/completion success paths that build `callback.LogData` manually
- [X] T024 Update `spend.Tracker.LogSuccess` to copy subscription attribution into `SpendRecord`
- [X] T025 Update `spend.Tracker.Record` to merge safe attribution into `metadata`
- [X] T026 Assert `UpdateVerificationTokenSpend` remains tied to `rec.APIKey` only
- [X] T027 Run spend/callback attribution tests

**Checkpoint**: Spend records and callbacks carry selected subscription attribution safely.

## Phase 5: Lifecycle Audit

**Goal**: Audit connect/refresh/delete/test/disable actions with redacted payloads.

- [X] T028 Add audit event for OpenAI subscription connect success
- [X] T029 Add audit event for OpenAI subscription connect failure where credential ID or org is available
- [X] T030 Add audit event for refresh success
- [X] T031 Add audit event for refresh failure
- [X] T032 Add audit event for post-refresh auth disable
- [X] T033 Add audit event for explicit delete
- [X] T034 Add audit event for test action if the test endpoint/helper exists in implementation scope (skipped — no OpenAI subscription test endpoint exists in current scope)
- [X] T035 Ensure every audit payload uses narrow safe fields and existing `redact.JSONValue`
- [X] T036 Run lifecycle audit tests

**Checkpoint**: Credential lifecycle operations are accountable and redacted.

## Phase 6: Redaction and Regression Verification

**Goal**: Prove the feature does not leak secrets or regress API-key behavior.

- [X] T037 Add negative assertions for access token, refresh token, ID token, bearer string, JWT, encrypted `credential_value`, raw account payload, and fallback API key across spend/audit/error/callback sinks
- [X] T038 Run targeted handler tests:
  `go test ./internal/proxy/handler/... -run 'TestOpenAISubscription.*Attribution|TestOpenAISubscription.*Audit|TestOpenAISubscription.*Redaction|TestOpenAISubscription.*APIKey' -count=1 -v`
- [X] T039 Run spend/callback tests:
  `go test ./internal/spend/... ./internal/callback/... -run 'Test.*Subscription.*Attribution|TestRecord.*Metadata|TestLogData' -count=1 -v`
- [X] T040 Run affected package tests:
  `go test ./internal/proxy/handler/... ./internal/spend/... ./internal/callback/... -count=1`
- [X] T041 Run `git diff --check origin/main...HEAD`
- [X] T042 Verify PR diff contains only HO-1182 implementation files and `specs/1182-openai-subscription-spend-and-audit-attribution/`
- [X] T043 Update SpecKit tasks to `[X]` only as implementation tasks complete
- [X] T044 Move Linear to In Review only after implementation gates pass and production code is pushed

## Dependencies and Parallelism

- T001-T008 can be written in parallel by test surface.
- T010-T014 must define the safe attribution shape before spend/callback integration.
- T015-T020 depends on the attribution shape.
- T021-T027 depends on selected-attempt propagation.
- T028-T036 can proceed after lifecycle operation boundaries are identified.
- T037-T044 are final verification.

## Scope Stop

This Todo planning PR stops at SpecKit artifacts and draft PR review. Production implementation starts only after Linear HO-1182 moves to **In Progress**.
