# Tasks: Virtual Key OAuth Prefix

**Input**: Design documents from `/specs/196-virtual-key-oauth-prefix/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No project initialization needed — this is a modification to an existing codebase. Phase 1 is empty.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No foundational work needed — all required infrastructure (auth middleware, DB schema, key hashing) already exists and does NOT change.

---

## Phase 3: User Story 1 — API 產生新格式 Virtual Key (Priority: P1)

**Goal**: API endpoints `/key/generate` and `/key/regenerate` 產生 `sk-ant-oat01-tianji-<60 hex>` 格式的 virtual key（80 chars）

**Independent Test**: 呼叫 `/key/generate`，驗證回傳的 key 以 `sk-ant-oat01-tianji-` 開頭且長度為 80

### Tests for User Story 1 (MANDATORY - Principle IV)

> **MANDATORY: Write these tests FIRST. Confirm they FAIL before any implementation task.**

- [x] T001 [P] [US1] Write `TestGenerateAPIKey_HasOAuthPrefix` — assert `strings.HasPrefix(key, "sk-ant-oat01-tianji-")` in `internal/proxy/handler/key_test.go`
- [x] T002 [P] [US1] Write `TestGenerateAPIKey_MinLength80` — assert `len(key) >= 80` in `internal/proxy/handler/key_test.go`
- [x] T003 [P] [US1] Write `TestGenerateAPIKey_ExactLength80` — assert `len(key) == 80` (prefix 20 + hex 60) in `internal/proxy/handler/key_test.go`
- [x] T004 [P] [US1] Write `TestGenerateAPIKey_PassesOAuthCheck` — assert `anthropic.IsOAuthToken(key) == true` in `internal/proxy/handler/key_test.go`
- [x] T005 [P] [US1] Write `TestGenerateAPIKey_CryptoRandom` — generate 1000 keys, assert no duplicates in `internal/proxy/handler/key_test.go`
- [x] T006 [P] [US1] Write `TestGenerateAPIKey_HexOnly` — assert random part matches `[0-9a-f]{60}` in `internal/proxy/handler/key_test.go`
- [x] T007 [US1] Run `go test ./internal/proxy/handler/... -run "TestGenerateAPIKey_" -v` to confirm all tests compile and FAIL

### Implementation for User Story 1

- [x] T008 [US1] Update `generateVirtualKey()` in `internal/proxy/handler/key.go` — replace `"sk-" + uuid.New().String()` with `"sk-ant-oat01-tianji-" + hex.EncodeToString(crypto/rand 30 bytes)`, add import `crypto/rand` + `encoding/hex`, remove `uuid` import
- [x] T009 [US1] Update `KeyRegenerate()` in `internal/proxy/handler/key_ext.go` line 32 — replace `"sk-" + uuid.New().String()` with call to `generateVirtualKey()`, remove `uuid` import
- [x] T010 [US1] Run `go test ./internal/proxy/handler/... -run "TestGenerateAPIKey_" -v` to confirm all tests PASS

**Checkpoint**: API key generation produces new format. UI still produces old format.

---

## Phase 4: User Story 2 — UI 產生新格式 Virtual Key (Priority: P1)

**Goal**: UI key creation and regeneration 產生相同的 `sk-ant-oat01-tianji-<60 hex>` 格式

**Independent Test**: 在 UI 建立 key，驗證格式一致

### Tests for User Story 2 (MANDATORY - Principle IV)

> **MANDATORY: Write these tests FIRST. Confirm they FAIL before any implementation task.**

- [x] T011 [P] [US2] Write `TestUIGenerateAPIKey_HasOAuthPrefix` — assert `strings.HasPrefix(key, "sk-ant-oat01-tianji-")` in `internal/ui/handler_keys_test.go`
- [x] T012 [P] [US2] Write `TestUIGenerateAPIKey_MinLength80` — assert `len(key) >= 80` in `internal/ui/handler_keys_test.go`
- [x] T013 [P] [US2] Write `TestUIGenerateAPIKey_MatchesAPIFormat` — assert UI key matches same regex as API key in `internal/ui/handler_keys_test.go`
- [x] T014 [P] [US2] Write `TestMaskAPIKey_NewFormat` — assert `maskAPIKey("sk-ant-oat01-tianji-<80 char key>")` shows truncated display in `internal/ui/handler_models_test.go`
- [x] T015 [P] [US2] Write `TestMaskAPIKey_OldFormat` — assert `maskAPIKey("sk-abc123")` backward-compatible display in `internal/ui/handler_models_test.go`
- [x] T016 [US2] Run `go test ./internal/ui/... -run "TestUIGenerateAPIKey_|TestMaskAPIKey_" -v` to confirm all tests compile and FAIL

### Implementation for User Story 2

- [x] T017 [US2] Update `generateAPIKey()` in `internal/ui/handler_keys.go` line 713-717 — change to `"sk-ant-oat01-tianji-" + hex.EncodeToString(30 bytes)`, update random bytes from 24 to 30
- [x] T018 [US2] Update `maskAPIKey()` in `internal/ui/handler_models.go` line 395-402 — replace hardcoded `"sk-..."` with dynamic prefix truncation (show first N chars + `...` + last 4 chars)
- [x] T019 [US2] Run `go test ./internal/ui/... -run "TestUIGenerateAPIKey_|TestMaskAPIKey_" -v` to confirm all tests PASS

**Checkpoint**: Both API and UI produce identical new format keys. Old keys still work (no auth changes).

---

## Phase 5: User Story 3 — 舊 Key 向後相容 (Priority: P2)

**Goal**: 驗證既有的 `sk-` 前綴 virtual key 不受影響

**Independent Test**: 使用舊格式 key 發送請求，確認認證通過

### Tests for User Story 3 (MANDATORY - Principle IV)

> **Principle IV exception**: US3 has zero implementation changes (auth middleware is prefix-agnostic). These are pass-on-write verification tests that confirm no regression. They should PASS immediately after writing.

- [x] T020 [P] [US3] Write `TestIsOAuthToken_TianjiVirtualKey` — assert `IsOAuthToken("sk-ant-oat01-tianji-" + 60 hex)` returns true in `internal/provider/anthropic/oauth_test.go`
- [x] T021 [P] [US3] Write `TestAuthMiddleware_OldFormatKey` — assert old `sk-<uuid>` key passes auth in `test/contract/key_management_test.go`
- [x] T022 [P] [US3] Write `TestAuthMiddleware_NewFormatKey` — assert new `sk-ant-oat01-tianji-<hex>` key passes auth in `test/contract/key_management_test.go`
- [x] T023 [US3] Run tests to confirm they compile and PASS

### Implementation for User Story 3

No implementation needed — auth middleware (`internal/proxy/middleware/auth.go`) uses SHA256 hash comparison and is completely prefix-agnostic. This phase is verification-only.

**Checkpoint**: All three user stories verified. Old and new key formats both work.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Update E2E tests and run full validation

- [x] T024 [P] Update `generateTestKey()` in `test/e2e/helpers_test.go` line 816 — change prefix to `"sk-ant-oat01-tianji-"` and random bytes to 30
- [x] T025 [P] Update key format assertion in `test/e2e/keys_create_test.go` line 34 — replace `assert.Contains(t, rawKey, "sk-")` with `assert.True(t, strings.HasPrefix(rawKey, "sk-ant-oat01-tianji-"))` and add `assert.Len` >= 80
- [x] T026 [P] Update key format assertions in `test/e2e/keys_regen_test.go` line 29, 88 — same prefix and length assertions as T025
- [x] T027 Run `make test` to verify all unit and contract tests pass
- [x] T028 Run `make lint` to verify no linting errors
- [x] T029 Run `make build` to verify clean compilation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Empty — nothing to do
- **Foundational (Phase 2)**: Empty — nothing to do
- **User Story 1 (Phase 3)**: Can start immediately
- **User Story 2 (Phase 4)**: Can start in parallel with US1 (different files)
- **User Story 3 (Phase 5)**: Can start after US1 and US2 (verification of their changes)
- **Polish (Phase 6)**: Depends on US1 + US2 completion

### User Story Dependencies

- **User Story 1 (P1)**: No dependencies — modifies `key.go`, `key_ext.go`
- **User Story 2 (P1)**: No dependencies — modifies `handler_keys.go`, `handler_models.go`
- **User Story 3 (P2)**: Depends on US1 + US2 (verifies their output format)

### Within Each User Story

1. Write failing tests FIRST (T001-T007, T011-T016, T020-T023)
2. Confirm tests fail
3. Implement changes
4. Confirm tests pass

### Parallel Opportunities

- **US1 tests** (T001-T006): All [P] — different test functions in same file, can be written together
- **US2 tests** (T011-T015): All [P] — different test functions, can be written together
- **US3 tests** (T020-T022): All [P] — different files
- **US1 and US2 implementation**: Can run in parallel (different files: `key.go`/`key_ext.go` vs `handler_keys.go`/`handler_models.go`)
- **Polish tasks** (T024-T026): All [P] — different E2E test files

---

## Parallel Example: User Story 1 + User Story 2

```bash
# Write all US1 tests in parallel (same file, different functions):
T001-T006: All test functions in internal/proxy/handler/key_test.go

# Write all US2 tests in parallel (different files):
T011-T013: internal/ui/handler_keys_test.go
T014-T015: internal/ui/handler_models_test.go

# Implement US1 and US2 in parallel (different packages):
T008-T009: internal/proxy/handler/ (API)
T017-T018: internal/ui/ (UI)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 3: User Story 1 (API key generation)
2. **STOP and VALIDATE**: `go test ./internal/proxy/handler/... -v`
3. API already produces new format keys — OpenClaw can use them

### Incremental Delivery

1. User Story 1 → API generates new format → Validate
2. User Story 2 → UI generates new format → Validate
3. User Story 3 → Verify backward compatibility → Validate
4. Polish → E2E tests updated → Full `make test` green

### Single Developer Strategy

US1 and US2 can be done in sequence in under 30 minutes:
1. Write all tests for US1 + US2 (T001-T016)
2. Implement US1 (T008-T009)
3. Implement US2 (T017-T018)
4. Run US3 verification (T020-T023)
5. Polish (T024-T029)

---

## Notes

- Total production files modified: 5 (`key.go`, `key_ext.go`, `handler_keys.go`, `handler_models.go`, `oauth_test.go`)
- Total test files modified/created: 4 (`key_test.go`, `handler_keys_test.go`, `handler_models_test.go`, `key_management_test.go`) + 3 E2E (`helpers_test.go`, `keys_create_test.go`, `keys_regen_test.go`)
- No database migrations
- No config changes
- No new dependencies (removes `uuid` from 2 files, uses stdlib `crypto/rand` + `encoding/hex`)
