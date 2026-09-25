# Tasks: HO-1342 Re-enable disabled OpenAI subscription credentials

**Input**: Design documents from `/specs/1342-re-enable-disabled-openai-subscription-credentials/`
**Prerequisites**: Linear state must move to In Progress before production implementation.

## Phase 1 - RED Coverage

- [x] T001 Re-read Linear HO-1342, this SpecKit package, and current OpenAI subscription lifecycle/UI code before implementation.
- [x] T002 Add failing lifecycle test proving enable changes operator-disabled metadata to active/allowed and clears `disabled_reason`.
- [x] T003 Add failing lifecycle test proving enable is idempotent for already active credentials.
- [x] T004 Add failing lifecycle test proving enable rejects non-operator disabled reasons (`auth_failed_after_refresh`, `refresh_failed`) with metadata unchanged.
- [x] T005 Add failing regression test proving disabled credential resolution fails before enable and succeeds after enable with the same fresh token bundle.
- [x] T006 Add failing audit/redaction test proving enable success/failure does not leak access token, refresh token, bearer string, JWT-like values, encrypted credential value, or raw metadata.
- [x] T007 Add failing RBAC test proving `/credentials/openai-subscription/{id}/enable` is proxy-admin only.
- [ ] T008 Add failing proxy route test proving `POST /credentials/openai-subscription/{id}/enable` dispatches to the lifecycle handler.
- [ ] T009 Add failing UI route/dispatcher test proving `/ui/credentials/{id}/enable` invokes proxy enable and re-renders table/detail targets.
- [x] T010 Add failing UI list test proving disabled rows render `Enable` and active rows render `Disable`.
- [x] T011 Add failing UI detail test proving disabled credential detail renders `Enable` and not `Disable`.
- [x] T012 Add failing UI copy test proving success/error messages say enable, not disable.

## Phase 2 - Proxy Lifecycle

- [x] T013 Add `openAISubscriptionLifecycleActionEnable`.
- [x] T014 Implement `OpenAISubscriptionCredentialEnable` beside disable lifecycle code.
- [x] T015 Reuse `loadOpenAISubscriptionCredentialMetadata` to validate DB presence and credential type.
- [x] T016 For operator-disabled credentials, set active/allowed status and clear `DisabledReason`.
- [x] T017 Preserve safe metadata fields (`Email`, `Scopes`, `LastRefreshAt`) and keep/clear redacted `LastError` according to final UI decision.
- [x] T018 Keep enable metadata-only; do not decrypt or update `CredentialValue`.
- [x] T019 Make enable idempotent for already active credentials.
- [x] T020 Reject unsupported disabled reasons with a safe lifecycle error and unchanged metadata.
- [x] T021 Add success/failure lifecycle audit entries with action `enable`.

## Phase 3 - Routes and RBAC

- [x] T022 Register proxy route `POST /credentials/openai-subscription/{credential_id}/enable`.
- [x] T023 Add RBAC regression coverage for the enable path.
- [x] T024 Register protected UI route `POST /ui/credentials/{credential_id}/enable`.

## Phase 4 - UI Action Switching

- [x] T025 Extend credential row/detail data model with disabled/action state needed by the templates.
- [x] T026 Add `handleCredentialEnable`.
- [x] T027 Extend `invokeCredentialLifecycle` to dispatch action `enable`.
- [x] T028 Add enable success/error copy.
- [x] T029 Update credentials list template so disabled credentials render `Enable` instead of `Disable`.
- [x] T030 Update credential detail template so disabled credentials render `Enable` instead of `Disable`.
- [x] T031 Ensure HTMX target re-render shows active status and clears disabled reason after successful enable.
- [x] T032 Regenerate templ output with `make ui`.

## Phase 5 - Regression Safety

- [x] T033 Prove test/refresh/disable/delete actions still behave as before.
- [x] T034 Prove existing Codex usage refresh and `/ui/usage` Codex tab behavior is unchanged.
- [x] T035 Prove wrong-type and missing credentials return safe errors for enable.
- [ ] T036 Prove DB update failure path is redacted.
- [x] T037 Prove no new OAuth reconnect or upstream OpenAI call is made by enable.

## Phase 6 - Verification

- [x] T038 Run `go test ./internal/proxy/handler -run 'OpenAISubscriptionLifecycle.*Enable|OpenAISubscriptionLifecycle.*Disable|ResolveOpenAISubscription' -count=1`.
- [x] T039 Run `go test ./internal/auth -run 'CredentialsRemainProxyAdminOnly' -count=1`.
- [x] T040 Run `go test ./internal/ui -run 'Credential.*Enable|Credentials' -count=1`.
- [x] T041 Run `go test ./internal/ui/pages -run 'Credential.*Enable|Credentials' -count=1`.
- [x] T042 Run existing affected OpenAI subscription lifecycle/routing tests.
- [x] T043 Run `make ui` when templ files change.
- [x] T044 Run `git diff --check`.
- [x] T045 Verify implementation PR diff contains production/test files plus this SpecKit package only.
- [ ] T046 Record verification evidence in PR and Linear/thread without secrets.

## Scope Stop

Linear HO-1342 must move out of Todo before production implementation starts. Todo output is this docs-only SpecKit package plus scope confirmation.
