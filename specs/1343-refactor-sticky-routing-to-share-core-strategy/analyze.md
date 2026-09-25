# Analyze: HO-1343 shared sticky routing strategy and Codex usage snapshots

## Gate Result

- Fatal issues: 0
- Critical issues: 0
- Owner-input blockers: 0

## Requirement Coverage

| Requirement area | Spec | Plan | Tasks | Status |
| --- | --- | --- | --- | --- |
| Shared sticky core | FR-001 to FR-003, US1 | Shared sticky core | T001-T005, T019 | Covered |
| Claude unchanged behavior | FR-004 to FR-007, US2 | Claude adapter | T006-T010, T020, T025-T027 | Covered |
| OpenAI subscription route behavior | FR-008, FR-009, FR-015 | OpenAI subscription/Codex adapter | T021, T025-T027 | Covered |
| Codex fresh cached usage snapshots | FR-010 to FR-012, US3, US4 | Codex adapter | T011-T015, T022 | Covered |
| No hot-path usage fetch | FR-013, FR-014, US5 | No hot-path usage fetch | T016-T018, T023 | Covered |
| Secret safety | FR-002, FR-016 | Risk register, data model | T005, T024, T029 | Covered |

## Repo Reality Alignment

- Claude sticky currently lives in `internal/proxy/handler/native_upstream.go`.
- OpenAI subscription sticky currently lives in `internal/proxy/handler/openai_subscription_routing.go`.
- OpenAI quota response-header state currently lives in `internal/callback/openai_quota_store.go` and `internal/proxy/handler/openai_subscription_ratelimit.go`.
- Codex usage snapshot cache currently lives in `internal/proxy/handler/openai_subscription_codex_usage.go`.
- Codex usage network fetch currently lives in `internal/provider/chatgptcodex/usage.go` and must stay outside request routing.

## Root Cause Validation

The documented root cause is internally consistent with repo evidence:

- Claude and OpenAI subscription sticky have separate handler-local state maps and reuse/reselect loops.
- Claude policy is quota-window aware via Anthropic OAuth rate-limit state.
- OpenAI subscription sticky is credential-affinity oriented, with direct OpenAI response-header quota scoring available but no Codex usage snapshot policy hook.
- HO-1340 added cached Codex usage snapshots, but the cache is currently UI/status oriented and not part of route selection.

## Scope Risks

- **Risk**: Shared core becomes too generic and hides provider behavior.
  - Mitigation: Keep quota semantics in explicit provider adapters and test adapter parity.
- **Risk**: Claude selection subtly changes.
  - Mitigation: Add parity tests around 5h reset, 7d reset, sonnet/all tracks, and run existing sticky tests.
- **Risk**: Codex snapshot scoring adds hot-path fetch.
  - Mitigation: Fail-on-fetch tests across chat completions, Responses HTTP, and WebSocket.
- **Risk**: Snapshot absence creates nondeterministic routing.
  - Mitigation: Treat unknown snapshots as fallback and test missing/stale/backoff cases.

## Out Of Scope Confirmed

- No UI change.
- No Codex CLI change.
- No OAuth connect/callback/token refresh change.
- No synchronous `wham/usage` fetch in proxy request routing.
- No sticky DB persistence.

## Scope Confirmation

Todo docs-only scope is ready to move to Waiting for implementation confirmation. No owner decision is required before writing code in a later state.
