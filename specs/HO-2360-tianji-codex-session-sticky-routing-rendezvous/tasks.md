# Tasks: Codex session sticky rendezvous routing

**Input**: Design documents from `specs/HO-2360-tianji-codex-session-sticky-routing-rendezvous/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are mandatory. No implementation task may begin until the relevant failing tests are written and confirmed to fail.

## Phase 1: Setup

**Purpose**: Confirm current branch, docs, and existing test surfaces.

- [X] T001 Verify worktree identity and clean starting status for `HO-2360-tianji-codex-session-sticky-routing-rendezvous`
- [X] T002 Review existing routing and Codex tests in `internal/proxy/handler/openai_subscription_routing_test.go`, `internal/proxy/handler/chatgpt_codex_backend_test.go`, `internal/proxy/handler/chatgpt_codex_wildcard_test.go`, and `internal/proxy/handler/responses_test.go`

---

## Phase 2: Foundational

**Purpose**: Define safe internal seams before user stories.

- [X] T003 Add a `codexSessionIdentity` helper contract in `internal/proxy/handler/openai_subscription_routing.go` or a focused sibling file
- [X] T004 Add session-aware Codex ordering/helper signatures in `internal/proxy/handler/openai_subscription_routing.go` without changing non-Codex call sites
- [X] T005 Add deterministic rendezvous selection helper using stdlib hashing in `internal/proxy/handler/openai_subscription_routing.go` or a focused sibling file

**Checkpoint**: Helpers are designed, but implementation must wait for failing tests in each story.

---

## Phase 3: User Story 1 - Same session stable routing (Priority: P1) MVP

**Goal**: Same Codex session sticks to the same credential while reusable.

**Independent Test**: Extract direct/nested session id and order the same session repeatedly over selectable credentials.

### Tests for User Story 1

- [X] T006 [P] [US1] Write failing `TestCodexSessionIdentity_DirectClientMetadataSessionID` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T007 [P] [US1] Write failing `TestCodexSessionIdentity_NestedTurnMetadataSessionID` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T008 [US1] Write failing `TestCodexStickySession_ReusesSelectableCredential` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T009 [US1] Run `go test ./internal/proxy/handler -run 'TestCodexSessionIdentity|TestCodexStickySession_ReusesSelectableCredential'` and confirm the new tests compile and fail

### Implementation for User Story 1

- [X] T010 [US1] Implement metadata extraction and source classification in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T011 [US1] Implement session-aware route key selection for Codex sticky tracks in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T012 [US1] Preserve existing `codexStickyCanReuse...` reuse behavior for session-aware tracks in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T013 [US1] Run `go test ./internal/proxy/handler -run 'TestCodexSessionIdentity|TestCodexStickySession_ReusesSelectableCredential'`

---

## Phase 4: User Story 2 - Rendezvous spread and minimal remap (Priority: P2)

**Goal**: Different sessions distribute across selectable credentials, and credential removal only remaps affected sessions.

**Independent Test**: Deterministic sessions and credential ids exercise rendezvous selection without network or DB.

### Tests for User Story 2

- [X] T014 [P] [US2] Write failing `TestCodexRendezvousSelect_DeterministicSameSession` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T015 [P] [US2] Write failing `TestCodexRendezvousSelect_SpreadsMultipleSessions` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T016 [P] [US2] Write failing `TestCodexRendezvousSelect_RemovingCredentialOnlyRemapsAffectedSessions` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T017 [US2] Write failing `TestCodexStickySession_ReselectsWhenCredentialOverGate` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T018 [US2] Run `go test ./internal/proxy/handler -run 'TestCodexRendezvousSelect|TestCodexStickySession_ReselectsWhenCredentialOverGate'` and confirm the new tests compile and fail

### Implementation for User Story 2

- [X] T019 [US2] Implement rendezvous highest-score selection over currently selectable credentials in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T020 [US2] Wire rendezvous selection only for new or invalidated Codex session sticky entries in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T021 [US2] Ensure over-gate/unavailable sticky credential reselects from remaining selectable credentials in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T022 [US2] Run `go test ./internal/proxy/handler -run 'TestCodexRendezvousSelect|TestCodexStickySession_ReselectsWhenCredentialOverGate'`

---

## Phase 5: User Story 3 - Fallback, entrypoints, and unchanged image behavior (Priority: P3)

**Goal**: Missing session metadata safely falls back, every supported Codex entrypoint passes identity into routing, and image paths stay unchanged.

**Independent Test**: Entry-point tests prove payload/metadata is available before ordering and fallback/image behavior does not regress.

### Tests for User Story 3

- [X] T023 [P] [US3] Write failing `TestCodexSessionIdentity_MalformedNestedMetadataFallsBackSafely` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T024 [P] [US3] Write failing `TestCodexSessionIdentity_BlankSessionIDIsMissing` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T025 [US3] Write failing `TestCodexSessionSticky_MissingSessionUsesExistingRouteKey` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T026 [US3] Write failing `TestChatGPTCodexBackendCompletion_PassesSessionIdentityToRouting` in `internal/proxy/handler/chatgpt_codex_backend_test.go`
- [X] T027 [US3] Write failing `TestResponsesHTTPAndCompact_PassSessionIdentityToRouting` in `internal/proxy/handler/responses_test.go`
- [X] T028 [US3] Write failing `TestResponsesWebSocket_PassesCreateFrameSessionIdentityToRouting` in `internal/proxy/handler/responses_test.go`
- [X] T029 [US3] Write regression `TestChatGPTCodexBackendImageGeneration_RemainsOrgLevelRouting` in `internal/proxy/handler/chatgpt_codex_wildcard_test.go`
- [X] T030 [US3] Run targeted `go test ./internal/proxy/handler -run 'TestCodexSessionIdentity|TestCodexSessionSticky_Missing|TestChatGPTCodexBackendCompletion_Passes|TestResponsesHTTPAndCompact|TestResponsesWebSocket|TestChatGPTCodexBackendImageGeneration_Remains'` and confirm new failing tests fail before implementation

### Implementation for User Story 3

- [X] T031 [US3] Pass session identity from `handleChatGPTCodexResponse` in `internal/proxy/handler/responses.go`
- [X] T032 [US3] Pass session identity from `handleResponsesWebSocketFrame` create frame payload in `internal/proxy/handler/responses.go`
- [X] T033 [US3] Pass session identity from `handleChatGPTCodexCompactResponse` in `internal/proxy/handler/responses_compact.go`
- [X] T034 [US3] Pass session identity from `req.Metadata` in `internal/proxy/handler/chatgpt_codex_backend.go`
- [X] T035 [US3] Add safe fallback routing log/metric fields without raw token/session payload leakage in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T036 [US3] Keep image generation/edit call sites unchanged and verify `internal/provider/chatgptcodex/transport.go` unknown extra params still reject unsupported fields
- [X] T037 [US3] Run targeted `go test ./internal/proxy/handler -run 'TestCodexSessionIdentity|TestCodexSessionSticky_Missing|TestChatGPTCodexBackendCompletion_Passes|TestResponsesHTTPAndCompact|TestResponsesWebSocket|TestChatGPTCodexBackendImageGeneration_Remains'`

---

## Phase 6: Polish & Verification

- [X] T038 Run `go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/config`
- [X] T039 Inspect final diff to confirm no UI setting/toggle/page, DB migration/table, image schema expansion, or non-Codex routing behavior change
- [X] T040 Update PR description with test results, routing evidence, UI N/A, and remaining risk

## Dependencies & Execution Order

- Setup and Foundational tasks precede all user stories.
- US1 is MVP and should land before US2/US3 because route key and extractor shape are shared.
- US2 depends on US1's session-aware sticky track.
- US3 can implement fallback and entrypoint wiring after US1 helper signatures exist.
- Polish depends on all selected user stories.

## Parallel Opportunities

- T006/T007 can run in parallel.
- T014/T015/T016 can run in parallel.
- T023/T024 can run in parallel.
- Entry-point tests T026/T027/T028 can be split if multiple workers coordinate on files.

## Implementation Strategy

1. Write and confirm failing tests for US1 before helper implementation.
2. Implement the smallest session identity + track key path that passes US1.
3. Add rendezvous tests and implementation for US2.
4. Add fallback/entrypoint/image regression tests and wire supported entrypoints for US3.
5. Run full required package tests and diff sanity before handoff.
