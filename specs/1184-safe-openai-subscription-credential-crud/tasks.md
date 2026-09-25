# Tasks: Safe OpenAI Subscription Credential CRUD

**Input**: Design documents from `/specs/1184-safe-openai-subscription-credential-crud/`
**Prerequisites**: Linear state must move to In Progress before production implementation。

## Phase 1: Tests First

- [x] T001 [P] Add failing create-response redaction test for `credential_type=openai_subscription`。
- [x] T002 [P] Add failing create metadata secret-rejection test covering nested token fields and bearer/JWT-looking values。
- [x] T003 [P] Add failing list safe-fields-only test for subscription credentials。
- [x] T004 [P] Add failing info safe-fields-only test for subscription credentials。
- [x] T005 [P] Add failing update value+metadata test for an existing `openai_subscription` row。
- [x] T006 [P] Add failing update type-mismatch test for an existing `api_key` row。
- [x] T007 [P] Add failing update type-change rejection test when request attempts to mutate `credential_type`。
- [x] T008 [P] Add failing idempotent local delete test for existing and missing subscription credential IDs。
- [x] T009 [P] Add failing permission regression test proving `/credentials` remains proxy-admin-only。
- [x] T010 [P] Add negative test guard proving CRUD tests use no real OpenAI host, account, or network call。

## Phase 2: CRUD Request Contract

- [x] T011 Update credential create/update request parsing only as needed to support safe `credential_info` updates。
- [x] T012 Keep API-key create/update compatibility for opaque `credential_value`。
- [x] T013 Explicitly require `credential_type=openai_subscription` semantics before applying subscription token bundle validation。
- [x] T014 Reject client attempts to change an existing row's `credential_type`。

## Phase 3: Safe Persistence

- [x] T015 Reuse `normalizeOpenAISubscriptionCredentialValue` for create/update token bundle canonicalization。
- [x] T016 Reuse `sanitizeCredentialInfoJSON` / shared `redact` helpers for all create/update metadata。
- [x] T017 Persist subscription value+metadata through existing sqlc-backed Store methods。
- [x] T018 Add sqlc query/interface changes only if existing generated methods cannot persist required fields safely。

## Phase 4: Safe Responses

- [x] T019 Ensure create response uses `redactCredential`。
- [x] T020 Ensure list response uses `redactCredential` for every row。
- [x] T021 Ensure info response uses `redactCredential`。
- [x] T022 Add assertions that responses omit `credential_value`, access/refresh/id tokens, bearer strings, JWTs, and encrypted blobs。

## Phase 5: Delete Semantics

- [x] T023 Make delete idempotent for missing credential IDs unless DB delete itself fails。
- [x] T024 Keep delete local-only; do not add OpenAI revoke/logout/delete calls。
- [x] T025 Keep delete lifecycle audit payloads narrow and redacted when an existing subscription row is known。
- [x] T026 Keep delete behavior compatible for existing non-subscription credentials unless subscription path is split。

## Phase 6: Permission and Regression Verification

- [x] T027 Verify `/credentials` route remains wrapped by `AuthMiddleware`。
- [x] T028 Verify `/credentials` remains mapped to `RoleProxyAdmin` in RBAC。
- [x] T029 Run targeted handler tests for OpenAI subscription CRUD。
- [x] T030 Run affected auth/handler packages。
- [x] T031 Run `git diff --check origin/main...HEAD`。
- [x] T032 Confirm implementation diff contains no real OpenAI credentials or hosts, and production code started only after Linear moved to In Progress。
