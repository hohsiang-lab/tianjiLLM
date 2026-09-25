# Tasks: Encrypted OpenAI Subscription Credential Persistence

**Input**: Design documents from `/specs/1176-openai-subscription-credential-persistence/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are mandatory. Write tests first and confirm they fail before implementation.

## Phase 1: Tests First - Encrypted Persistence (US1)

**Goal**: Prove OpenAI subscription token bundles are encrypted and metadata is safe.

- [x] T001 [P] [US1] Write failing `TestOpenAISubscriptionCredential_SaveEncryptsTokenBundle` in `internal/proxy/handler/credential_test.go`.
- [x] T002 [P] [US1] Write failing `TestOpenAISubscriptionCredential_SaveStoresSafeMetadata` in `internal/proxy/handler/credential_test.go`.
- [x] T003 [P] [US1] Write failing `TestOpenAISubscriptionCredential_RejectsMissingSecretFields` in `internal/proxy/handler/credential_test.go`.
- [x] T004 [US1] Run targeted tests and confirm they compile and fail for missing implementation.

## Phase 2: Tests First - Redacted List/Info (US2)

**Goal**: Prove management APIs never expose token material by default.

- [x] T005 [P] [US2] Write failing `TestCredentialList_RedactsCredentialValueAndReturnsInfo` in `internal/proxy/handler/credential_test.go`.
- [x] T006 [P] [US2] Write failing `TestCredentialInfo_RedactsCredentialValueAndReturnsInfo` in `internal/proxy/handler/credential_test.go`.
- [x] T007 [P] [US2] Write failing `TestCredentialList_RedactsAPIKeyCredentialsToo` in `internal/proxy/handler/credential_test.go`.
- [x] T008 [US2] Run targeted tests and confirm they fail on current direct DB row/basic response behavior.

## Phase 3: Tests First - Refresh Rotation (US3)

**Goal**: Prove refresh updates persist rotated tokens and safe metadata correctly.

- [x] T009 [P] [US3] Write failing `TestOpenAISubscriptionCredential_UpdateRotatedRefreshToken` in `internal/proxy/handler/credential_test.go`.
- [x] T010 [P] [US3] Write failing `TestOpenAISubscriptionCredential_UpdatePreservesRefreshTokenWhenOmitted` in `internal/proxy/handler/credential_test.go`.
- [x] T011 [P] [US3] Write failing `TestOpenAISubscriptionCredential_UpdateFailureMetadataOnly` in `internal/proxy/handler/credential_test.go`.
- [x] T012 [US3] Run targeted tests and confirm they fail for missing refresh persistence helper/query.

## Phase 4: Foundational Implementation

**Goal**: Add typed payload validation and sqlc update support.

- [x] T013 [P] Add `OpenAISubscriptionTokenBundle` and `OpenAISubscriptionCredentialInfo` helper types in the credential handler/service boundary.
- [x] T014 [P] Add metadata sanitizer that rejects token field names in `credential_info`.
- [x] T015 Add `UpdateCredentialValueAndInfo` query to `internal/db/queries/credential.sql`.
- [x] T016 Run `make generate`.
- [x] T017 Update `internal/db/interface.go` and `internal/proxy/handler/mock_store_test.go` for the generated query method.

## Phase 5: Implement US1 - Encrypted Persistence

**Goal**: Persist new OpenAI subscription credentials correctly.

- [x] T018 [US1] Implement create/save helper that validates bundle and metadata.
- [x] T019 [US1] Encrypt token bundle JSON with existing `auth.Encrypt`.
- [x] T020 [US1] Persist using `CreateCredential` with `credential_type=openai_subscription`.
- [x] T021 [US1] Run US1 tests and confirm they pass.

## Phase 6: Implement US2 - Redacted Responses

**Goal**: Return safe metadata without credential values.

- [x] T022 [US2] Implement shared redacted credential response mapper.
- [x] T023 [US2] Update `CredentialList` to use redacted mapper instead of returning DB rows directly.
- [x] T024 [US2] Update `CredentialInfo` to include safe `credential_info` metadata and omit `credential_value`.
- [x] T025 [US2] Run US2 tests and existing credential handler tests.

## Phase 7: Implement US3 - Refresh Rotation

**Goal**: Persist refreshed token bundles and metadata.

- [x] T026 [US3] Implement refresh update helper that decrypts existing bundle when preserving old refresh token is needed.
- [x] T027 [US3] If refresh response includes new `refresh_token`, overwrite stored refresh token.
- [x] T028 [US3] If refresh response omits `refresh_token`, preserve old refresh token.
- [x] T029 [US3] Implement metadata-only failure update path.
- [x] T030 [US3] Run US3 tests and confirm they pass.

## Phase 8: Verification and PR Hygiene

- [x] T031 Run `go test ./internal/auth/... ./internal/db/... ./internal/proxy/handler/... -v`.
- [x] T032 Run `git diff --check`.
- [x] T033 Confirm no migration files were added unless a real schema gap was found.
- [x] T034 Confirm `git diff --name-only origin/main...HEAD` is limited to HO-1176 scope.
- [x] T035 Commit and push implementation changes.

## Dependencies

- US1, US2, and US3 tests can be written independently.
- sqlc query generation blocks refresh update implementation.
- Redacted response mapper should land before handler response changes.
- Implementation must not start until Linear HO-1176 is moved to In Progress.
