# Tasks: OpenAI Quota/Rate-limit Header Parser and Store Integration

**Input**: Design documents from `/specs/1181-openai-quota-rate-limit-header-parser-and-store/`
**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/openai-quota-rate-limit-store.md`

**Tests**: Failing tests are mandatory. No production implementation task may begin until the relevant failing parser/store/routing tests are written and confirmed to fail against current behavior.

## Phase 1: Test Baseline

**Purpose**: Prove current behavior is handler-private parser state, not normalized store integration.

- [X] T001 [P] Write parser test for official request exhaustion headers in `internal/proxy/handler/openai_subscription_ratelimit_test.go`
- [X] T002 [P] Write parser test for official token exhaustion headers in `internal/proxy/handler/openai_subscription_ratelimit_test.go`
- [X] T003 [P] Write parser test for non-exhausted request/token headers and derived utilization
- [X] T004 [P] Write parser test for missing headers producing no update and no gate
- [X] T005 [P] Write parser test for malformed integer/duration headers preserving safe unknown state
- [X] T006 [P] Write parser test for remaining greater than limit and utilization clamping
- [X] T007 [P] Write mock quota fixture test for `allowed`
- [X] T008 [P] Write mock quota fixture test for `rejected` with future reset
- [X] T009 [P] Write mock quota fixture test for reset expiry clearing gate
- [X] T010 [P] Write mock quota fixture test for utilization present, missing, and out of range
- [X] T011 Run targeted parser tests and confirm failures are current implementation gaps, not environment/network

**Checkpoint**: Parser expectations are locked before store changes.

## Phase 2: Normalized Store Contract

**Goal**: Add one OpenAI quota/rate-limit state path with explicit unknowns.

- [X] T012 Add OpenAI quota state/store type or interface in the appropriate package
- [X] T013 Represent known/unknown request limit, remaining, reset, and utilization explicitly
- [X] T014 Represent known/unknown token limit, remaining, reset, and utilization explicitly
- [X] T015 Represent optional mock quota status/reset/utilization without calling it official OpenAI API
- [X] T016 Add normalization that clears expired exhausted/rejected gates
- [X] T017 Add merge behavior so partial updates do not erase valid known state accidentally
- [X] T018 Add store tests for set/get, partial update merge, reset expiry, and unknown preservation
- [X] T019 Ensure store keys use credential/account ID and never raw token material

**Checkpoint**: Store can safely hold OpenAI quota state independent of routing.

## Phase 3: Parser Integration

**Goal**: Make parser emit normalized store-facing state.

- [X] T020 Update official OpenAI header parser to return `OpenAIQuotaState`
- [X] T021 Parse reset headers as durations relative to parse time
- [X] T022 Keep RFC3339 reset support only if existing behavior is preserved and tested
- [X] T023 Derive utilization from limit/remaining when possible
- [X] T024 Add or update `openaitest` quota fixture support for allowed/rejected/reset/utilization cases
- [X] T025 Keep absent/malformed headers as no update or partial safe update
- [X] T026 Run parser and mock fixture tests

**Checkpoint**: Header parsing writes provider-appropriate normalized state.

## Phase 4: Routing Store Consumption

**Goal**: Make gate, sticky, and lowest-utilization read the same normalized state.

- [X] T027 Replace handler-private OpenAI rate-limit map reads with normalized store reads
- [X] T028 Update candidate gate to reject only future exhausted/rejected states
- [X] T029 Update sticky selection to re-evaluate when stored state gates the sticky credential
- [X] T030 Update lowest-utilization to score known utilization/capacity and fail open for unknowns
- [X] T031 Add integration test where parsed state gates selected credential on next request
- [X] T032 Add integration test where sticky re-evaluates after stored exhaustion/rejection
- [X] T033 Add integration test where lowest-utilization consumes stored OpenAI state
- [X] T034 Preserve API-key path behavior when no subscription credential IDs are configured

**Checkpoint**: Routing uses one store-backed quota source.

## Phase 5: Response Attempt Store Updates

**Goal**: Update store on every relevant official OpenAI subscription response.

- [X] T035 Update direct official OpenAI request loop to parse/store headers on 2xx responses
- [X] T036 Parse/store headers on 401 first attempt before forced refresh branch discards response
- [X] T037 Parse/store headers on 401 refreshed retry before auth-failure/failover handling
- [X] T038 Parse/store headers on 429 before gating/failover
- [X] T039 Parse/store headers on retryable 5xx before failover
- [X] T040 Add endpoint test proving 429 updates store and gates the credential
- [X] T041 Add endpoint test proving 401 retry path does not lose visible header state
- [X] T042 Add endpoint test proving missing headers preserve current no-data fallback

**Checkpoint**: Store state is updated before return/retry/discard decisions.

## Phase 6: Security and Regression Verification

**Goal**: Prove the integration is safe and scoped.

- [X] T043 Add redaction test for parser/store diagnostics and aggregate errors
- [X] T044 Run targeted parser/store/routing tests:
  `go test ./internal/proxy/handler/... -run 'TestParseOpenAIRateLimit|TestOpenAIQuota|TestOpenAISubscriptionRouting' -count=1 -v`
- [X] T045 Run callback/store tests:
  `go test ./internal/callback/... -run 'TestOpenAIQuota' -count=1 -v`
- [X] T046 Run affected package tests:
  `go test ./internal/proxy/handler/... ./internal/callback/... ./internal/testutil/openaitest/... -count=1`
- [X] T047 Run `git diff --check origin/main...HEAD`
- [X] T048 Verify PR diff includes only HO-1181 implementation files and `specs/1181-openai-quota-rate-limit-header-parser-and-store/`
- [X] T049 Update SpecKit tasks to `[X]` only as implementation tasks complete
- [X] T050 Move Linear to In Review only after implementation gates pass and production code is pushed

## Dependencies and Parallelism

- T001-T010 can be written in parallel by parser/mock fixture surface.
- T012-T019 must define store shape before routing consumes it.
- T020-T026 depends on store state shape.
- T027-T034 depends on parser/store contract.
- T035-T042 depends on routing/store integration and HO-1180 retry surfaces.
- T043-T050 are final verification.

## Scope Stop

This Todo planning PR stops at SpecKit artifacts and draft PR review. Production implementation starts only after Linear HO-1181 moves to **In Progress**.
