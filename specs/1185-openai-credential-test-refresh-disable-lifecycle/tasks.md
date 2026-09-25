# Tasks: OpenAI Credential Test/Refresh/Disable Lifecycle APIs

**Input**: Design documents from `/specs/1185-openai-credential-test-refresh-disable-lifecycle/`
**Prerequisites**: Linear state must move to In Progress before production implementation。

## Phase 1: Tests First

- [x] T001 [P] Add failing handler test for test API using fresh `openai_subscription` credential and mock `GET /v1/models`。
- [x] T002 [P] Add failing handler test for test API refreshing stale credential before `/v1/models`。
- [x] T003 [P] Add failing handler test for test API upstream 401/403/429/5xx returning safe `reason_code` only。
- [x] T004 [P] Add failing handler test for explicit refresh success, including persisted encrypted bundle and safe `last_refresh_at` response。
- [x] T005 [P] Add failing handler test for refresh response omitting rotated refresh token and preserving existing refresh token。
- [x] T006 [P] Add failing handler test for refresh failure (`invalid_grant` / malformed JSON) persisting redacted failure metadata。
- [x] T007 [P] Add failing handler test for disable active credential setting `status=disabled` and `disabled_reason=operator_disabled`。
- [x] T008 [P] Add failing handler test for disable already-disabled credential returning idempotent success/no-op。
- [x] T009 [P] Add failing wrong-type tests proving lifecycle APIs reject `api_key` credentials with safe errors。
- [x] T010 [P] Add audit tests proving `test`、`refresh`、`disable` success/failure payloads are redacted。
- [x] T011 [P] Add no-real-OpenAI guard test proving lifecycle tests cannot call `api.openai.com` or `auth.openai.com`。
- [x] T012 [P] Add failing delete compatibility regression proving existing `openai_subscription` delete behavior remains idempotent/redacted。
- [x] T013 Run targeted tests and confirm failures are implementation gaps, not fixture/environment failures。

## Phase 2: Route and Permission Contract

- [x] T014 Add lifecycle routes under existing `/credentials` management router。
- [x] T015 Ensure lifecycle routes are protected by existing `AuthMiddleware`。
- [x] T016 Ensure lifecycle routes require `RoleProxyAdmin` or equivalent existing credential-management permission。
- [x] T017 Add route/RBAC regression tests for unauthenticated and non-admin callers。

## Phase 3: Lifecycle Service Helpers

- [x] T018 Add shared lifecycle response builder with `credential_id`、`action`、`status`、`reason_code`、redacted `metadata`。
- [x] T019 Add helper to load and validate target `openai_subscription` credential by ID。
- [x] T020 Reuse existing `OpenAISubscriptionCredentialError` codes where possible; add only stable lifecycle-specific codes if needed。
- [x] T021 Ensure all helper errors route through shared redaction before response/audit/metadata。

## Phase 4: Test Credential API

- [x] T022 Implement test API handler。
- [x] T023 For stale token, reuse existing refresh path before upstream test。
- [x] T024 Call configured/mockable OpenAI-compatible `GET /v1/models` with subscription bearer。
- [x] T025 Return only safe status metadata such as `models_count`、safe `request_id`、`last_refresh_at`。
- [x] T026 Map upstream failures to stable safe `reason_code` values。
- [x] T027 Audit test success/failure lifecycle events。

## Phase 5: Explicit Refresh API

- [x] T028 Implement refresh API handler using existing refresh helper。
- [x] T029 Ensure refresh API force-refreshes even when current token is locally fresh。
- [x] T030 Preserve current refresh-token rotation behavior when upstream omits `refresh_token`。
- [x] T031 Persist success metadata with `status=active` and safe `last_refresh_at`。
- [x] T032 Persist failure metadata with `status=refresh_failed` and redacted `last_error`。
- [x] T033 Audit refresh success/failure lifecycle events。

## Phase 6: Disable API

- [x] T034 Implement disable API handler。
- [x] T035 Update local `credential_info` to `status=disabled` and `disabled_reason=operator_disabled` while preserving safe existing metadata。
- [x] T036 Make disable idempotent for already-disabled credentials。
- [x] T037 Prove disabled credentials are excluded by existing subscription candidate resolution。
- [x] T038 Ensure disable does not call remote OpenAI revoke/logout/delete APIs。
- [x] T039 Audit disable success/no-op/failure lifecycle events。

## Phase 7: Redaction and Regression Verification

- [x] T040 Add assertions that lifecycle responses omit `credential_value`、access tokens、refresh tokens、`id_token`、bearer strings、JWTs、encrypted blobs、fallback API keys。
- [x] T041 Add assertions that persisted `credential_info` omits raw upstream/token response material。
- [x] T042 Add assertions that audit payloads omit secret material。
- [x] T043 Run targeted lifecycle tests。
- [x] T044 Run existing credential CRUD tests。
- [x] T045 Run existing OpenAI subscription refresh/routing tests。
- [x] T046 Run `go test ./internal/proxy/handler/... ./internal/testutil/openaitest/... -count=1`。
- [x] T047 Run `go tool golangci-lint run`。
- [x] T048 Run `git diff --check origin/main...HEAD`。
- [x] T049 Verify PR diff includes only HO-1185 implementation files plus `specs/1185-openai-credential-test-refresh-disable-lifecycle/`。

## Scope Stop

Linear HO-1185 moved to In Progress before production implementation. Remaining gate is PR update + review scene。
