# Tasks: Fix API Key Authentication

**Input**: Design documents from `/specs/001-fix-api-key-auth/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

**Tests**: Included — FR-011 mandates unit tests for auth middleware wiring and integration test for full virtual key auth path.

**Organization**: Tasks grouped by user story. This is a surgical bug fix — 3 source files + 2 new test files. No schema changes, no new API endpoints.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Exact file paths in all descriptions

---

## Phase 1: Setup (Verify Baseline)

**Purpose**: Confirm the existing codebase compiles before touching anything.

- [x] T001 Run `make build` from project root to verify baseline compilation passes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create `DBValidator` adapter — the single struct that bridges `*db.Queries` to
the existing `TokenValidator` + `GuardrailProvider` interfaces. **Every user story depends on this.**

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Create `internal/proxy/middleware/db_validator.go` with `ErrKeyNotFound` and `ErrDBUnavailable` sentinel errors, and full `DBValidator` struct implementation — `ValidateToken(ctx, hash)` mapping `pgx.ErrNoRows` → `ErrKeyNotFound` and other DB errors → `ErrDBUnavailable`, plus `GetGuardrails(ctx, hash)` returning `VerificationToken.Policies`

**Checkpoint**: `DBValidator` adapter exists and satisfies both `TokenValidator` and `GuardrailProvider` interfaces — user story implementation can now begin.

---

## Phase 3: User Story 1 — API Key Authentication Works (Priority: P1) 🎯 MVP

**Goal**: Virtual API keys created via `/key/generate` authenticate successfully against the database. Blocked keys return 403, invalid keys return 401, DB outage returns 503.

**Independent Test**: Create a virtual key, send `POST /v1/chat/completions` with `Authorization: Bearer <virtual-key>`, verify 200 (not 401).

### Tests for User Story 1 ⚠️

> **Write these tests FIRST — they MUST FAIL before implementation**

- [x] T003 [P] [US1] Write failing unit test for `DBValidator.ValidateToken` covering all four paths (pgx.ErrNoRows→ErrKeyNotFound, DB error→ErrDBUnavailable, blocked=true, success with userID/teamID) in `internal/proxy/middleware/db_validator_test.go`
- [x] T004 [P] [US1] Write failing integration test for full virtual key authentication path (valid key → 200, invalid key → 401, blocked key → 403) in `test/integration/virtual_key_auth_test.go`

### Implementation for User Story 1

- [x] T005 [US1] Modify `internal/proxy/middleware/auth.go` to handle `errors.Is(err, ErrDBUnavailable)` → return HTTP 503 Service Unavailable (in the virtual key validation branch)
- [x] T006 [US1] Add structured logging to `internal/proxy/middleware/auth.go` using `log.Printf` — INFO on successful virtual key auth, WARN on auth failure (ErrKeyNotFound/blocked), ERROR on DB failure (ErrDBUnavailable) — consistent with existing `log.Printf("JWT validation failed: %v", err)` pattern at line 80
- [x] T007 [US1] Wire `DBValidator` into `cmd/tianji/main.go` — pass `DBQueries: &middleware.DBValidator{DB: queries}` to `proxy.ServerConfig` when `queries != nil` (approximately line 423, inside the ServerConfig struct literal)

**Checkpoint**: Virtual key authentication fully functional. `make test` passes. quickstart.md Steps 1–2 verified.

---

## Phase 4: User Story 2 — Master Key Still Works (Priority: P2)

**Goal**: Master key authentication is unaffected by the DBValidator wiring. No regression introduced.

**Independent Test**: Send `POST /v1/chat/completions` with `Authorization: Bearer $MASTER_KEY` and verify 200 — same as before the fix.

**Note**: No implementation changes needed — master key path in `auth.go` is unaffected. This phase adds regression coverage only.

### Tests for User Story 2 ⚠️

- [x] T008 [P] [US2] Add regression test for master key authentication in `internal/proxy/middleware/auth_test.go` — verify that when `DBValidator` is wired, a valid master key request still authenticates as `is_master_key=true` and bypasses the DB lookup entirely

**Checkpoint**: Master key regression confirmed. quickstart.md Step 3 verified.

---

## Phase 5: User Story 3 — Virtual Key Guardrails Apply (Priority: P3)

**Goal**: When a virtual key has `Policies` set, those guardrail names are loaded into the request context and enforced. The `GetGuardrails` method (already implemented in T002) is correctly invoked by the auth middleware via the `GuardrailProvider` interface check.

**Independent Test**: Create a virtual key with a guardrail configured; send a request violating the guardrail; verify the request is blocked with an appropriate error.

### Tests for User Story 3 ⚠️

- [x] T009 [P] [US3] Add unit test for `DBValidator.GetGuardrails` in `internal/proxy/middleware/db_validator_test.go` — verify it returns `VerificationToken.Policies` slice on success, empty slice for key with no policies, and error on DB failure
- [x] T010 [P] [US3] Add integration test for guardrail context loading in `test/integration/virtual_key_auth_test.go` — verify that after virtual key auth, `ContextKeyGuardrails` is populated when the key has associated policies

### Implementation for User Story 3

- [x] T011 [US3] Verify `internal/proxy/middleware/auth.go` performs the `GuardrailProvider` interface type assertion after `ValidateToken` succeeds and calls `GetGuardrails` to populate `ContextKeyGuardrails` — patch only if the assertion is missing or broken

**Checkpoint**: Guardrail context correctly populated for virtual key requests. quickstart.md Step 4 (blocked key) validated.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, lint clean-up, and DB outage scenario confirmation.

- [x] T012 [P] Run `make check` (golangci-lint + go test -race -cover + go build) from project root and fix any lint or test failures
- [ ] T013 Manually verify quickstart.md Step 5 scenario (DB outage → 503) by stopping PostgreSQL and confirming the proxy returns 503 not 401 or 500

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — run immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS all user stories**
- **User Stories (Phases 3–5)**: All depend on Phase 2 completion (T002 must exist)
  - US1 (Phase 3): No dependency on US2 or US3
  - US2 (Phase 4): No dependency on US1 or US3 (but wiring from T007 must be present for integration validation)
  - US3 (Phase 5): No dependency on US2; `GetGuardrails` is already in T002
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Start after T002 — no inter-story dependencies
- **US2 (P2)**: Start after T002 — no inter-story dependencies (master key path untouched)
- **US3 (P3)**: Start after T002 — `GetGuardrails` already implemented; phase is test + verify only

### Within Each User Story

- Tests (T003, T004) MUST be written and FAIL before implementation (T005–T007)
- Auth middleware change (T005, T006) before wiring (T007) — T007 depends on T005 to compile correctly
- Core fix (T007) before integration test validation

### Parallel Opportunities

- T003 and T004 can run in parallel (different files, both are new)
- T005 and T006 can run in parallel within Phase 3 (same file but non-overlapping hunks — exercise caution)
- T008 (Phase 4) can start as soon as T002 is done, in parallel with Phase 3 work
- T009 and T010 (Phase 5) can run in parallel
- T012 (Polish) is independent of T013

---

## Parallel Example: User Story 1

```bash
# After T002 completes, launch all US1 tests together (must FAIL first):
Task: "T003 — unit test for DBValidator.ValidateToken in db_validator_test.go"
Task: "T004 — integration test skeleton in test/integration/virtual_key_auth_test.go"

# After T003/T004 exist and fail, run implementation in sequence:
T005 → T006 → T007 (T005+T006 can overlap if editing different lines of auth.go)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: `make build` passes
2. Complete Phase 2: Create `db_validator.go` (T002)
3. Write failing tests T003, T004
4. Complete US1 implementation T005 → T006 → T007
5. **STOP and VALIDATE**: `make test` passes; quickstart.md Steps 1–2 work
6. This is a shippable fix — virtual key auth restored

### Incremental Delivery

1. Phases 1–2: Foundation ready (T001–T002)
2. Phase 3 + US1 tests: Virtual key auth restored → **Deploy** (MVP)
3. Phase 4 + US2 tests: Regression confirmed → **Confidence**
4. Phase 5 + US3 tests: Guardrails verified → **Complete**
5. Phase 6: Polish → **Ship**

### Single-Developer Strategy

Given this is a 3-file fix, sequential delivery is natural:

```
T001 → T002 → T003+T004 (parallel) → T005 → T006 → T007
     → T008 → T009+T010 (parallel) → T011 → T012 → T013
```

---

## Notes

- [P] tasks = different files or non-overlapping edits, safe to run concurrently
- [US*] label maps task to user story for traceability
- T002 (Foundational) is the only new file in the adapter layer — keeps the fix minimal
- Tests must FAIL before implementation — confirm with `go test ./internal/proxy/middleware/... -run TestDBValidator`
- Commit after each checkpoint for clean rollback points
- quickstart.md contains the exact `curl` commands to manually verify each scenario
- This fix has zero schema changes — `make generate` (sqlc) is NOT required
