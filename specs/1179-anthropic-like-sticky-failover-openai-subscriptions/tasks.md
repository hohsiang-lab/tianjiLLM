# Tasks: Anthropic-like Sticky/Failover for OpenAI Subscriptions

**Input**: Design documents from `/specs/1179-anthropic-like-sticky-failover-openai-subscriptions/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/openai-subscription-routing.md

**Tests**: Failing tests are mandatory. No production implementation task may begin until the relevant failing tests are written and confirmed to fail for current single-ID/no-routing behavior.

**Organization**: Tasks are grouped by implementation phase and user story. All tasks remain unchecked in Todo state.

## Phase 1: Test Scaffolding

**Purpose**: Lock expected behavior before changing resolver/routing code.

- [X] T001 [P] [US1] Write failing test `TestResolveOpenAISubscriptionCandidates_IncludesAllConfiguredIDs` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T002 [P] [US1] Write failing test `TestResolveOpenAISubscriptionCandidates_DisabledExcluded` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T003 [P] [US1] Write failing test `TestResolveOpenAISubscriptionCredential_AllUnusableNoAPIKeyFallback` in `internal/proxy/handler/openai_subscription_resolution_test.go`
- [X] T004 [P] [US1] Write failing test `TestResolveOpenAISubscriptionCandidates_PreservesReasonCodes` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T005 [P] [US2] Write failing test `TestOpenAISubscriptionRouting_StickyStablePerOrg` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T006 [P] [US2] Write failing test `TestOpenAISubscriptionRouting_StickySeparateOrgs` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T007 [P] [US2] Write failing test `TestOpenAISubscriptionRouting_StickyReevaluatesUnavailableCredential` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T008 [P] [US2] Write failing test `TestOpenAISubscriptionRouting_RoundRobinDefault` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T009 [P] [US3] Write failing test `TestParseOpenAIRateLimitHeaders_RequestsExhausted` in `internal/proxy/handler/openai_subscription_ratelimit_test.go`
- [X] T010 [P] [US3] Write failing test `TestParseOpenAIRateLimitHeaders_TokensExhausted` in `internal/proxy/handler/openai_subscription_ratelimit_test.go`
- [X] T011 [P] [US3] Write failing test `TestParseOpenAIRateLimitHeaders_MissingHeadersNoGate` in `internal/proxy/handler/openai_subscription_ratelimit_test.go`
- [X] T012 [P] [US3] Write failing test `TestOpenAISubscriptionEndpoints_UpdateRateLimitStateOn200And429` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T013 [P] [US4] Write failing test `TestOpenAISubscriptionRouting_FailoverAfterRefreshFailure` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T014 [P] [US4] Write failing test `TestOpenAISubscriptionRouting_FailoverOnOpenAI429BeforeReturn` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T015 [P] [US4] Write failing test `TestOpenAISubscriptionRouting_FailoverOnOpenAI5xxBeforeReturn` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T016 [P] [US4] Write failing test `TestOpenAISubscriptionRouting_NoRetryAfterStreamingBodyStarted` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T017 [P] Write regression test `TestResolveOpenAIAPIKeyForParams_APIKeyPathUnchanged` in `internal/proxy/handler/openai_subscription_resolution_test.go`
- [X] T018 [P] Write regression test `TestValidateOpenAISubscriptionCredentialIDs_CustomAPIBaseStillRejected` in `internal/config/validate_test.go`
- [X] T019 Run targeted tests and confirm expected failures come from missing multi-candidate routing/current `ids[0]` behavior, not environment or network

**Checkpoint**: Failing tests define all HO-1179 behavior before production changes.

## Phase 2: Candidate Resolution and Error Model

**Goal**: Build usable/unusable candidate state from all configured IDs.

- [X] T020 [US1] Add `internal/proxy/handler/openai_subscription_routing.go` with candidate and aggregate error types
- [X] T021 [US1] Implement candidate construction that evaluates every configured credential ID in order
- [X] T022 [US1] Reuse `resolveUsableOpenAISubscriptionBundle` for load/refresh and preserve existing typed credential error codes
- [X] T023 [US1] Exclude disabled credentials before selection while preserving sanitized reason for all-unusable errors
- [X] T024 [US1] Update `resolveOpenAISubscriptionCredential` and `resolveOpenAIAPIKeyForParams` to use routed candidate selection for direct HTTP transport
- [X] T025 [US1] Ensure all-unusable credential errors do not return or send fallback `api_key`
- [X] T026 [US1] Run US1 resolver tests and confirm pass

**Checkpoint**: Multiple configured IDs are visible to routing and all-unusable errors are explicit/sanitized.

## Phase 3: Strategy Selection and Org-wide Sticky

**Goal**: Apply native strategy semantics with OpenAI subscription identity and org scope.

- [X] T027 [US2] Add OpenAI subscription route key helper using provider namespace, org scope, and model/route class
- [X] T028 [US2] Add deterministic master/no-org sticky scope for requests without `ContextKeyOrgID`
- [X] T029 [US2] Add round-robin selection over available OpenAI subscription candidates
- [X] T030 [US2] Add sticky selection that stores credential ID and reuses it only while available
- [X] T031 [US2] Add sticky re-evaluation when selected credential is disabled, refresh-failed, or rate-limit gated
- [X] T032 [US2] Add lowest-utilization ordering using OpenAI rate-limit remaining/reset state when known, with deterministic fallback when unknown
- [X] T033 [US2] Run US2 strategy tests and confirm pass

**Checkpoint**: Sticky is stable per org and strategy behavior does not leak across orgs.

## Phase 4: OpenAI Rate-limit State

**Goal**: Parse official OpenAI response headers into provider-specific routing gates.

- [X] T034 [US3] Add `internal/proxy/handler/openai_subscription_ratelimit.go` with OpenAI rate-limit state and parser
- [X] T035 [US3] Parse request/token limit, remaining, and reset headers using duration/reset semantics from official docs
- [X] T036 [US3] Gate candidates when remaining requests or tokens is exhausted until reset
- [X] T037 [US3] Treat absent or malformed headers as no state update, not automatic exhaustion
- [X] T038 [US3] Store OpenAI rate-limit state by credential ID/account ID, never raw bearer token
- [X] T039 [US3] Update state on both successful and non-success official OpenAI HTTP responses
- [X] T040 [US3] Run US3 parser/state tests and confirm pass

**Checkpoint**: OpenAI rate-limit gate is driven only by OpenAI header data.

## Phase 5: Endpoint Failover Integration

**Goal**: Retry another configured credential before returning failure when safe.

- [X] T041 [US4] Wrap direct official OpenAI HTTP endpoint dispatch with bounded credential-attempt loop
- [X] T042 [US4] Fail over after pre-dispatch credential load/refresh failures
- [X] T043 [US4] Fail over after 429 responses before body relay and update exhausted credential rate-limit state
- [X] T044 [US4] Fail over after retryable 5xx responses before body relay
- [X] T045 [US4] Preserve non-retry behavior for non-rate-limit 4xx responses
- [X] T046 [US4] Preserve no-retry behavior after streaming body relay has started
- [X] T047 [US4] Ensure attempt count is capped at available candidate count
- [X] T048 [US4] Run US4 endpoint failover tests and confirm pass

**Checkpoint**: One unusable selected credential no longer fails the request while another configured credential can serve it.

## Phase 6: Regression and Final Verification

**Purpose**: Prove HO-1179 did not regress existing OpenAI API-key and config behavior.

- [X] T049 Run all targeted HO-1179 tests: `go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscription|TestOpenAISubscriptionRouting|TestParseOpenAIRateLimit|TestOpenAISubscriptionEndpoints' -count=1 -v`
- [X] T050 Run config validation regression: `go test ./internal/config/... -run 'TestValidateOpenAISubscription' -count=1 -v`
- [X] T051 Run touched package tests: `go test ./internal/proxy/handler/... ./internal/config/... -count=1`
- [X] T052 Run `git diff --check origin/main...HEAD`
- [X] T053 Verify PR diff includes only HO-1179 implementation files and `specs/1179-anthropic-like-sticky-failover-openai-subscriptions/`
- [X] T054 Update SpecKit tasks to `[X]` as implementation completes, commit, push, and move through In Review only after Linear is In Progress and implementation gates pass

## Dependencies & Parallelism

- T001-T018 can be written in parallel by file grouping.
- T020-T026 must land before strategy and endpoint failover integration.
- T027-T033 and T034-T040 can proceed in parallel after candidate identity is defined.
- T041-T048 depends on candidate resolution and rate-limit state.
- T049-T054 are final verification.

## Scope Stop

This Todo planning PR stops at SpecKit artifacts and draft PR review. Production implementation starts only after Linear HO-1179 moves to **In Progress**.
