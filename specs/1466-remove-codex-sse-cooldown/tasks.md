---

description: "Task list for removing Tianji's synthetic Codex failed-SSE cooldown"
---

# Tasks: Remove Codex failed-SSE synthetic cooldown

**Input**: Design documents from `/specs/1466-remove-codex-sse-cooldown/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Required by the feature specification and constitution's failing-tests-first principle.

**Organization**: Tasks are grouped by user story; this is a two-file deletion slice with no new runtime component.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel when files and dependencies do not overlap
- **[Story]**: User story traceability label; setup/foundation/polish tasks omit it
- Every task names an exact repository-relative file or command scope.

## Phase 1: Setup (Repository Baseline)

**Purpose**: Confirm the current source/test boundary before editing.

- [X] T001 Record the clean baseline and current HEAD with `rtk git status --short --branch` and `rtk git rev-parse HEAD`; preserve unrelated `graphify-out/`, `internal/graphify-out/`, and `test/graphify-out/` artifacts.
- [X] T002 [P] Re-run targeted `ccc` search for `recordOpenAISubscriptionCodexResponseFailure`, `codexResponsesFailureBackoffDuration`, and `responses.codex_sse_failure_backoff` under `internal/proxy/handler/` and confirm the planned caller/test references.

## Phase 2: Foundational (Failing-Test Gate)

**Purpose**: Confirm the existing focused test and source boundary before story work.

- [X] T003 Confirm `internal/proxy/handler/responses_codex_test.go` contains the current failed-SSE regression and `internal/proxy/handler/responses.go` copies the HTTP 200/SSE response before failure inspection; record the RED-test command from `specs/1466-remove-codex-sse-cooldown/quickstart.md`.

**Checkpoint**: The baseline is established; no production behavior has changed.

## Phase 3: User Story 1 - Failed SSE is proxied without a Tianji cooldown (Priority: P1) 🎯 MVP

**Goal**: Remove only the post-commit failed-SSE gate while preserving passthrough and failure logging.

**Independent Test**: The focused test in `internal/proxy/handler/responses_codex_test.go` passes with HTTP 200, original `response.failed` content, a failure callback, no success callback, and no newly recorded quota state.

### Tests for User Story 1

- [X] T004 [US1] Rewrite `TestCreateResponse_LogsCodexSSEFailedAsFailureAndBacksOffCredential` as `TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff` in `internal/proxy/handler/responses_codex_test.go`, retaining HTTP 200/body passthrough and failure-log assertions while asserting `openAISubscriptionRateLimitState("cred-a")` is absent (FR-001, FR-002, FR-008; SC-001, SC-002).
- [X] T005 [US1] Run the renamed focused regression test with `rtk go test ./internal/proxy/handler -run '^TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff$' -count=1` and confirm it fails against the current synthetic gate before changing `internal/proxy/handler/responses.go` (FR-001, FR-008; SC-001).

### Implementation for User Story 1

- [X] T006 [US1] Remove the `h.recordOpenAISubscriptionCodexResponseFailure(...)` call from the HTTP 200 failed-SSE branch in `internal/proxy/handler/responses.go`, leaving `logCodexResponsesFailureEvent`, failure logging, and response passthrough unchanged (FR-001, FR-002, FR-003; SC-001, SC-002, SC-004).
- [X] T007 [US1] Delete `codexResponsesFailureBackoffDuration` and `recordOpenAISubscriptionCodexResponseFailure` from `internal/proxy/handler/responses.go`, including the `responses.codex_sse_failure_backoff` log event and no-longer-needed callback quota-state construction (FR-001, FR-009; SC-005).

**Checkpoint**: User Story 1 is independently testable and the only production behavior removed is the synthetic cooldown.

## Phase 4: User Story 2 - Existing upstream-owned protections remain intact (Priority: P1)

**Goal**: Verify official quota, failover, credential lifecycle, and usage snapshot protections were not broadened or weakened.

**Independent Test**: Existing focused handler tests for OpenAI subscription routing, official rate-limit headers, pre-body 429/5xx failover, credential refresh/disable, and Codex usage backoff remain green; no same-request retry is added.

### Tests for User Story 2

- [X] T008 [P] [US2] Run official quota-header and routing-gate regressions in `internal/proxy/handler/openai_subscription_ratelimit_test.go` and `internal/proxy/handler/openai_subscription_routing_test.go`, including `TestParseOpenAIRateLimitHeaders_MissingHeadersNoGate` and `TestOpenAISubscriptionRouting_UsesSharedOpenAIQuotaStoreForGate`; do not modify generic gate code (FR-004; SC-003).
- [X] T009 [P] [US2] Run pre-body failover, stream-commit, and credential lifecycle regressions in `internal/proxy/handler/openai_subscription_endpoints_test.go` and `internal/proxy/handler/openai_subscription_lifecycle_test.go`, including `TestOpenAISubscriptionRouting_FailoverOnOpenAI429BeforeReturn`, `TestOpenAISubscriptionRouting_FailoverOnOpenAI5xxBeforeReturn`, `TestOpenAISubscriptionRouting_StreamsReturnedWithoutPreemptiveRetry`, and the existing refresh/disable tests (FR-003, FR-005, FR-006; SC-003, SC-004).
- [X] T010 [P] [US2] Run Codex usage snapshot cache/backoff regressions in `internal/proxy/handler/openai_subscription_codex_usage_test.go`, including `TestOpenAISubscriptionCodexUsage_BackoffPreservesLastSuccess` and `TestOpenAISubscriptionCodexUsage_UsesCacheWithinTTL`; verify no usage-snapshot files changed (FR-007; SC-003).
- [X] T011 [US2] Run the complete narrow handler package regression with `rtk go test ./internal/proxy/handler -count=1` and confirm the stream-commit test still prevents same-request retry after HTTP 200/SSE bytes (FR-003, FR-008; SC-003, SC-004).

**Checkpoint**: User Stories 1 and 2 both pass without changes to official quota, failover, credential, or usage-snapshot paths.

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Validate scope and documentation artifacts without adding runtime complexity.

- [X] T012 [P] Re-run `rtk ccc index` after source edits and confirm the deleted recorder/constant have no remaining code references under `internal/` (FR-009; SC-005).
- [X] T013 Run `rtk git diff --check`, `rtk git diff --stat`, and `rtk git status --short --branch`; confirm production implementation scope is limited to `internal/proxy/handler/responses.go` and `internal/proxy/handler/responses_codex_test.go`, unrelated generated artifacts remain untouched, and SC-005 is satisfied (FR-009; SC-005).

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No code dependency; establishes evidence and preserves unrelated artifacts.
- **Foundational (Phase 2)**: Depends on Setup; T003 establishes the test/source boundary.
- **User Story 1 (Phase 3)**: Depends on Foundational; T004/T005 establish the RED test before T006/T007 delete the synthetic behavior.
- **User Story 2 (Phase 4)**: Depends on User Story 1; verifies adjacent behavior remains unchanged.
- **Polish (Phase 5)**: Depends on both stories and final source state.

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Phase 2; no dependency on User Story 2.
- **User Story 2 (P1)**: Starts after User Story 1 because its regression suite validates the completed deletion boundary.

### Parallel Opportunities

- T002 and T003 can run in parallel after T001.
- T008, T009, and T010 can run in parallel after T007.
- T012 can run after T011; T013 is the final scope gate.

## Parallel Example: User Story 1

```bash
# RED test first; implementation follows only after the expected failure is observed.
rtk go test ./internal/proxy/handler \
  -run '^TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff$' \
  -count=1
```

## Parallel Example: User Story 2

```bash
# These read-only focused regressions can run independently after the deletion.
rtk go test ./internal/proxy/handler -run 'OpenAI.*(RateLimit|Routing|Failover|Credential|CodexUsage)' -count=1
```

## Implementation Strategy

### MVP First (User Story 1 only)

1. Complete Phase 1 baseline.
2. Rewrite and fail the focused test in Phase 2.
3. Remove the failed-SSE synthetic recorder/constant in Phase 3.
4. Stop and validate HTTP 200/body passthrough, failure logging, and absent local gate.

### Incremental Delivery

1. Add the adjacent regression verification in User Story 2.
2. Run the narrow handler package suite.
3. Finish the scope/diff gate without touching production files outside the two-file boundary.

## Notes

- No setup or foundational runtime code is added; the foundational phase exists only for the required RED-test gate.
- No new contract, data model, dependency, database migration, retry path, or cooldown configuration is warranted.
- The existing generic quota gate remains intentionally in place for independently recorded official upstream states.
