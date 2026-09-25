# Analyze: Codex subscription route E2E and credential lookup recovery

## Scope Alignment

- Linear requires one issue-owned test and fix for `wildcard route -> subscription credential lookup/refresh -> Codex payload -> Codex SSE response`.
- `spec.md` FR-001 and `plan.md` primary RED route test cover the full chain with `openai/*`, DB-backed credential lookup, Codex payload, and SSE response.
- Linear requires preserving API-key-backed `openai/*`; `spec.md` FR-011, `plan.md` API-key no-regression, and `tasks.md` T011/T018 cover it.
- Linear requires secret safety; `spec.md` FR-012, `quickstart.md`, and `tasks.md` G003 cover it.
- Linear says current row exists and active while lookup fails; `plan.md` focuses the first RED on generated-query/DB-backed lookup instead of another mock-store-only wildcard test.
- Wiki/memory review added three E2E guards from prior Tianji lessons: DB-managed `ProxyModelTable` route source must be part of the primary test, stale-token refresh must be proven through the route, and subscription non-streaming behavior must be deterministic instead of looking like credential lookup failure.

## Consistency Check

| Check | Result | Evidence |
| --- | --- | --- |
| Spec requires full route coverage | Pass | FR-001/FR-002 and User Story 1 |
| Plan starts with RED tests | Pass | Phase 1 and T003-T010 |
| Tasks avoid production code in Todo | Pass | G001 and Todo phase |
| API-key route protected | Pass | User Story 4, FR-011, T011 |
| Secret safety explicit | Pass | FR-012, quickstart redaction, G003 |
| Existing prior issues respected | Pass | Dependencies list and repo reality |
| Non-streaming contract covered | Pass after review update | FR-013 and `TestCodexSubscriptionRoute_NonStreamingContractIsDeterministic` |
| Refresh boundary covered in route | Pass after review update | `TestCodexSubscriptionRouteE2E_StaleCredentialRefreshesBeforeStreaming` |

## Risk Audit

- Fatal: 0
- Critical: 0
- Owner input required before Waiting: 0

## Implementation Gate

Implementation is blocked until Linear HO-1314 moves from `Todo` to `In Progress`.

## Recovery Audit

- Required live sources checked: Linear issue/comments, Git worktree status/head/upstream, PR existence, Discord thread, pipeline ledger.
- Git checkpoint result: no dirty files and no unpushed commits existed before writing these SpecKit artifacts, so no recovery commit was needed before continuing.
- Side effects already present in live state were no-opped: existing thread start/resume messages and current executing orchestration.
