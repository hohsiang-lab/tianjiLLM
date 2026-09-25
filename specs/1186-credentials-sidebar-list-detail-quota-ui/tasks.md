# Tasks: Credentials sidebar list/detail quota UI

**Input**: Design documents from `/specs/1186-credentials-sidebar-list-detail-quota-ui/`
**Prerequisites**: Linear state must move to `In Progress` before production implementation。

## Phase 1: RED UI E2E first

- [x] T001 [P] Add failing E2E for sidebar `Credentials` nav item and `/ui/credentials` active state。
- [x] T002 [P] Add failing E2E list test that seeds two `openai_subscription` credentials and one `api_key`, then asserts only subscription credentials render。
- [x] T003 [P] Add failing E2E list empty-state test for no OpenAI subscription credentials。
- [x] T004 [P] Add failing E2E detail navigation test from list row to `/ui/credentials/{credential_id}`。
- [x] T005 [P] Add failing E2E detail metadata test for email、status、`last_refresh_at`、`last_error`、`disabled_reason`。
- [x] T006 [P] Add failing E2E quota rendering test for known request/token limit、remaining、utilization、reset time。
- [x] T007 [P] Add failing E2E quota/status badge test for `allowed`、`rejected`、`exhausted`、unknown/no-state cases。
- [x] T008 [P] Add failing rendered-DOM secret test that seeds token-looking values and asserts body/HTML omit them。
- [x] T009 Run targeted `TestCredentials*` E2E and confirm failures are missing implementation, not fixture/environment failures。（本機 E2E 需要 `E2E_DATABASE_URL`；CI 執行）

## Phase 2: Route and sidebar wiring

- [x] T010 Add `/ui/credentials` and `/ui/credentials/{credential_id}` protected routes in `internal/ui/routes.go` inside existing `sessionAuth` group。
- [x] T011 Add `Credentials` sidebar nav item in `internal/ui/pages/layout.templ` using existing icon component。
- [x] T012 Regenerate templ output for changed `.templ` files using the repo's existing generation command。

## Phase 3: UI handler data adapter

- [x] T013 Add `internal/ui/handler_credentials.go` with list/detail handlers。
- [x] T014 List handler loads credentials through existing sqlc-backed DB methods and filters `credential_type=openai_subscription`。
- [x] T015 Detail handler loads one credential by ID and rejects/not-founds missing or wrong-type rows safely。
- [x] T016 Add narrow safe metadata parser for `OpenAISubscriptionCredentialInfo` fields only。
- [x] T017 Add quota state adapter from `UIHandler.RateLimitStore.GetOpenAIQuotaState(credentialID, now)`。
- [x] T018 Add status derivation helper that distinguishes credential health metadata from quota gate status。
- [x] T019 Ensure handlers never decrypt `credential_value` or pass raw metadata maps to templates。

## Phase 4: Templ pages

- [x] T020 Add `internal/ui/pages/credentials.templ` data structs for list rows, detail data, quota dimensions, and status badges。
- [x] T021 Implement Credentials list page with table, empty state, row links, safe columns, and stable responsive layout。
- [x] T022 Implement Credential detail page with header, cards, quota dimension table, reset time, last refresh/error/disabled metadata。
- [x] T023 Implement quota progress bar with stable dimensions, known/unknown fallback, and clamped CSS width。
- [x] T024 Implement safe long-text rendering for `last_error` / `disabled_reason` without overlap or hidden token fields。
- [x] T025 Regenerate `credentials_templ.go` and any changed generated templ files。

## Phase 5: Fixture helpers

- [x] T026 Add E2E helper to seed `CredentialTable` rows with encrypted synthetic OpenAI subscription token bundles。
- [x] T027 Add E2E helper to seed generic `api_key` credential for exclusion regression。
- [x] T028 Add E2E helper to seed `RateLimitStore` quota state on the test UI/server handler。
- [x] T029 Add no-real-OpenAI guard assertion for credential UI tests。

## Phase 6: Security and compatibility regression

- [x] T030 Assert list/detail DOM omits `credential_value`、access token、refresh token、`id_token`、JWT、bearer string、encrypted blob、fallback API key。
- [x] T031 Assert unknown quota renders as unknown/not recorded and never as zero healthy usage。
- [x] T032 Assert disabled credential metadata remains visible even if quota state is stale/allowed。
- [x] T033 Assert existing `/ui/keys` navigation and key detail tests still pass。
- [x] T034 Assert existing credential lifecycle handler tests remain green。

## Phase 7: Verification and PR update

- [x] T035 Run `go test ./test/e2e -tags e2e -run 'TestCredentials' -count=1`。（本機缺 `E2E_DATABASE_URL`，CI E2E gate 執行）
- [x] T036 Run affected UI tests, including `go test ./internal/ui/... -count=1`。
- [x] T037 Run affected credential/quota tests, including `go test ./internal/proxy/handler/... ./internal/callback/... -count=1`。
- [x] T038 Run `go tool golangci-lint run`。
- [x] T039 Run `git diff --check origin/main...HEAD`。
- [x] T040 Verify PR diff contains only HO-1186 implementation files plus `specs/1186-credentials-sidebar-list-detail-quota-ui/`。
- [x] T041 Update PR body with UI behavior summary, E2E coverage map, safe metadata guarantee, and remaining CI state。

## Scope Stop

Stop after Todo planning PR moves Linear HO-1186 to `Waiting`. Production implementation starts only after Linear state is moved to `In Progress`.
