# Tasks: Token/JWT Redaction and Subscription Credential Status Metadata

**Input**: Design documents from `/specs/1183-token-jwt-redaction-subscription-status/`
**Prerequisites**: Linear state must move to In Progress before implementation.

## Phase 1: Tests First

- [x] T001 [P] Add failing `TestRedactString_RemovesBearerTokenAndJWT` in the chosen shared redaction package.
- [x] T002 [P] Add failing `TestRedactJSONValue_RemovesNestedCredentialFields` for nested maps/arrays and case-insensitive secret field names.
- [x] T003 [P] Add failing `TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets` in `internal/proxy/handler/credential_test.go`.
- [x] T004 [P] Add failing `TestCredentialInfo_RedactsNestedAccountPayload` in `internal/proxy/handler/credential_test.go`.
- [x] T005 [P] Add failing `TestRecordErrorLog_RedactsUpstreamTokenText` for `InsertErrorLogParams.ErrorMessage`.
- [x] T006 [P] Add failing `TestCreateAuditLog_RedactsCredentialPayloads` for `InsertAuditLogParams.BeforeValue` and `UpdatedValues`.
- [x] T007 [P] Add failing `TestJWTValidationFailure_DoesNotLogRawToken` in `internal/proxy/middleware/auth_test.go`.

## Phase 2: Shared Redaction Utility

- [x] T008 Create a shared redaction package with string, JSON value, and raw JSON sanitizers.
- [x] T009 Implement case-insensitive field-name redaction for known secret keys.
- [x] T010 Implement JWT-shaped value and bearer-token string redaction.
- [x] T011 Keep safe metadata fields intact and prove this through tests.

## Phase 3: Credential Metadata Integration

- [x] T012 Route `sanitizeCredentialInfoJSON` and `redactCredential` through the shared redaction helper.
- [x] T013 Sanitize `OpenAISubscriptionCredentialInfo.LastError` before `UpdateOpenAISubscriptionCredentialFailure`.
- [x] T014 Sanitize `DisabledReason` before persistence.
- [x] T015 Preserve existing HO-1176 encrypted bundle and redacted list/info tests.

## Phase 4: Log, Error, Audit, and Event Sinks

- [x] T016 Sanitize provider/upstream error text before `ErrorLogs.error_message` persistence.
- [x] T017 Sanitize audit `beforeValue` / `updatedValues` before `InsertAuditLog`.
- [x] T018 Sanitize management event payloads that can carry credential request/DB rows.
- [x] T019 Keep auth failure responses fixed and token-hash-only; change middleware only if tests reveal leakage.

## Phase 5: Verification

- [x] T020 Run targeted tests for redaction, credentials, auth middleware, and handler error logs.
- [x] T021 Run `go test ./internal/auth/... ./internal/proxy/middleware/... ./internal/proxy/handler/...`.
- [x] T022 Run `git diff --check`.
- [x] T023 Confirm no real OpenAI hosts, accounts, or secrets appear in tests.
