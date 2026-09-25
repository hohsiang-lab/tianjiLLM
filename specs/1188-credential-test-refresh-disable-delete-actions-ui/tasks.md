# Tasks: Credential Test/Refresh/Disable/Delete actions UI

**Input**: `spec.md`, `plan.md`, Linear HO-1188
**State**: Todo planning complete; all tasks remain unchecked until Linear moves to `In Progress`.

## Phase 1 - Tests First

- [x] T001 Extend `test/e2e/credentials_test.go` with list-page lifecycle action happy paths for `Test`、`Refresh`、`Disable`、`Delete`.
- [x] T002 Add E2E fixture support for action responses/upstream behavior using seeded DB and mockable OpenAI upstream; no live OpenAI request.
- [x] T003 Add E2E assertion that `Test` success shows safe toast and optional models count without changing credential row visibility.
- [x] T004 Add E2E assertion that `Refresh` updates `last_refresh_at` / safe status metadata after server-rendered refresh.
- [x] T005 Add E2E assertion that `Disable` requires confirm; cancelled confirm does not mutate state.
- [x] T006 Add E2E assertion that accepted `Disable` marks row/detail as `Disabled` and shows safe disabled reason.
- [x] T007 Add E2E assertion that `Delete` requires confirm; cancelled confirm keeps row/detail intact.
- [x] T008 Add E2E assertion that accepted `Delete` removes the row from list and routes detail users to `/ui/credentials` or safe deleted state.
- [x] T009 Add E2E lifecycle error cases for `credential_missing`、`credential_wrong_type`、`refresh_failed`、`upstream_401` or mockable equivalent.
- [x] T010 Add DOM/toast/attribute secret assertions after success and failure: no `credential_value`、access token、refresh token、`id_token`、JWT、bearer string、authorization code、encrypted blob、raw upstream response.
- [x] T011 Keep existing HO-1186 credentials list/detail/quota/redaction tests green.

## Phase 2 - UI Routes and Handlers

- [x] T012 Add protected UI routes in `internal/ui/routes.go` under existing `sessionAuth`: `/ui/credentials/{credential_id}/test|refresh|disable|delete`.
- [x] T013 Add action handler methods in `internal/ui/handler_credentials.go`.
- [x] T014 Validate selected credential exists and is `credential_type = "openai_subscription"` before rendering action controls/results.
- [x] T015 Reuse existing lifecycle/delete backend behavior or shared helper; do not duplicate OpenAI test/refresh token logic in UI.
- [x] T016 Convert backend lifecycle responses/errors into safe UI action result view model.
- [x] T017 Reload credential/list data from DB-safe metadata after each successful mutation.

## Phase 3 - Page Partials and Toasts

- [x] T018 Add action toolbar view model fields to `CredentialRow` and `CredentialDetailData` only as needed.
- [x] T019 Render list action column in `internal/ui/pages/credentials.templ`.
- [x] T020 Render detail action toolbar near header/status area.
- [x] T021 Add reusable credential action toolbar partial using existing `button` variants.
- [x] T022 Add reusable credentials list/detail partial swap targets so actions can refresh only affected UI regions.
- [x] T023 Add `toast.Script()` to credentials pages if not already loaded.
- [x] T024 Add toast wrapper functions using existing `toast.Toast` + HTMX OOB pattern.
- [x] T025 Add `hx-confirm` copy for `Disable` and `Delete`; no confirm for `Test` / `Refresh`.
- [x] T026 Ensure forms/HTMX attributes include only credential ID/path/context and no secret-bearing values.
- [x] T027 Ensure mobile/narrow layouts wrap action buttons without overlapping table/detail content.

## Phase 4 - Delete and Detail Navigation

- [x] T028 Define delete-from-list success behavior: refresh list/table with credential removed and success toast.
- [x] T029 Define delete-from-detail success behavior: redirect/swap to safe deleted state with link back to `/ui/credentials`.
- [x] T030 Add safe not-found behavior for missing/deleted credential detail route if needed.

## Phase 5 - Generation and Verification

- [x] T031 Run `templ generate ./internal/ui/pages/...` or repo-equivalent generation command.
- [x] T032 Run targeted UI tests: `go test ./internal/ui/...`.
- [x] T033 Compile E2E package: `go test -c -tags e2e ./test/e2e`.
- [x] T034 Run targeted credentials E2E locally when `E2E_DATABASE_URL` is available, otherwise document compile-only local gate and rely on CI browser E2E. Targeted credentials E2E passed locally with a temporary Postgres database.
- [x] T035 Run affected backend lifecycle tests to ensure UI did not break API contracts: `go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionCredential|TestCredentialDelete' -count=1`.
- [x] T036 Run `git diff --check origin/main...HEAD`.
- [x] T037 Review changed UI for no new frontend framework/package and no duplicated lifecycle/OpenAI token logic.
- [ ] T038 Capture result-screen evidence before Waiting Merge: list actions and detail actions screenshots were captured and posted to the issue thread; toast/disabled/deleted result-screen evidence remains for the Waiting Merge visual gate.

## Scope Confirmation

- [x] T039 Confirm through draft PR/Linear Waiting gate that scope is limited to existing Credentials UI actions, existing lifecycle/delete APIs, safe metadata refresh, HTMX confirm/toast, and offline UI E2E.
