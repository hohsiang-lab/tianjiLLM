# Tasks: HO-1343 shared sticky routing strategy and Codex usage snapshots

**Input**: Design documents from `/specs/1343-refactor-sticky-routing-to-share-core-strategy/`

**Tests**: Failing tests are mandatory before production changes. All tasks remain unchecked in Todo state.

## Phase 1 - Shared core RED tests

- [ ] T001 Add shared sticky core test for sticky reuse when selected candidate remains available and policy allows reuse.
- [ ] T002 Add shared sticky core test for reselect when selected candidate is absent from the available list.
- [ ] T003 Add shared sticky core test for policy-triggered re-evaluation.
- [ ] T004 Add shared sticky core test for deterministic fallback when scores are unknown or tied.
- [ ] T005 Add shared sticky core test proving selected IDs are stable non-secret candidate IDs.

## Phase 2 - Claude parity RED tests

- [ ] T006 Add/extend Claude adapter test proving sonnet and non-sonnet tracks stay separate.
- [ ] T007 Add/extend Claude adapter test proving 5h reset timestamp change forces re-evaluation.
- [ ] T008 Add/extend Claude adapter test proving earliest known 7d reset still wins after refactor.
- [ ] T009 Add/extend Claude adapter test proving unknown 7d reset still loses to known reset and all-unknown remains deterministic.
- [ ] T010 Run existing `sticky_upstream_test.go` and model-aware gate tests before production refactor and record baseline failures/success.

## Phase 3 - Codex cached-snapshot RED tests

- [ ] T011 Add Codex routing test with fresh cached usage snapshots proving lower primary 5h usage wins when candidates are otherwise available.
- [ ] T012 Add Codex routing test proving weekly usage/reset breaks ties or triggers re-evaluation according to policy.
- [ ] T013 Add Codex routing test proving selected credential re-evaluates when cached selected snapshot becomes exhausted or reset fingerprint changes.
- [ ] T014 Add Codex routing test proving missing snapshots fall back to current deterministic sticky/round-robin behavior.
- [ ] T015 Add Codex routing test proving stale, unavailable, or backoff cache entries do not block routing and do not trigger fetch.

## Phase 4 - No hot-path fetch RED tests

- [ ] T016 Add chat completions test with fail-on-call `CodexUsageFetcher` proving request routing does not fetch usage.
- [ ] T017 Add Responses HTTP test with fail-on-call `CodexUsageFetcher` proving request routing does not fetch usage.
- [ ] T018 Add Responses WebSocket test with fail-on-call `CodexUsageFetcher` proving request routing does not fetch usage.

## Phase 5 - Implementation

- [ ] T019 Add provider-neutral shared sticky core with locked track map, selected ID reuse, reselect, policy re-evaluation, scoring, and deterministic fallback.
- [ ] T020 Refactor Claude sticky selection to use shared core while preserving provider-specific gates, DB fallback, 5h reset metadata, and 7d scoring.
- [ ] T021 Refactor OpenAI subscription sticky selection to use shared core while preserving org/model route keys, disabled/rate-limited filtering, and deterministic fallback.
- [ ] T022 Add Codex cached-snapshot policy scoring using fresh `OpenAISubscriptionCodexUsageCache` entries only.
- [ ] T023 Add no-fetch guard wiring so routing reads cache directly and never calls the usage snapshot fetch path.
- [ ] T024 Ensure shared core and logs/errors never store or emit raw API key, bearer token, refresh token, or encrypted credential material.

## Phase 6 - Verification

- [ ] T025 Run `go test ./internal/proxy/handler -run 'Sticky|OpenAISubscriptionRouting|CodexUsage|Responses|ChatGPTCodex' -count=1`.
- [ ] T026 Run `go test ./internal/callback -run 'RateLimit|OpenAIQuota' -count=1`.
- [ ] T027 Run `go test ./internal/proxy/handler -count=1`.
- [ ] T028 Run `git diff --check origin/main...HEAD`.
- [ ] T029 Verify PR diff contains only HO-1343 implementation files plus this SpecKit directory.

## Scope Stop

This Todo planning PR stops at SpecKit artifacts, draft PR, and scope confirmation. Production implementation starts only after Linear moves to an implementation state.
