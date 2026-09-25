# Analysis: HO-1340 Codex usage snapshot

## Gate Result

- Fatal issues: 0
- Critical issues: 0
- Owner-input blockers: 0

## Requirement Coverage

- Provider helper and request headers: covered by FR-001 through FR-004 and T002, T016-T021.
- Existing credential resolution/refresh reuse: covered by FR-005, FR-006 and T006-T007, T022-T024.
- Cache/backoff: covered by FR-007 through FR-011 and T005, T008, T025-T027.
- Normalized safe fields: covered by FR-012, FR-013 and T003-T004, T015, T030, T047-T048.
- Credential detail UI: covered by FR-014, FR-017 and T011-T012, T031-T035.
- `/ui/usage` Codex tab: covered by FR-015, FR-016, FR-017 and T013-T014, T036-T043.
- Missing/disabled/malformed states: covered by FR-018 and T009, T028, T046.
- Optional/unknown buckets: covered by FR-019 and T004, T020.
- No per-proxy-request fetch: covered by FR-010, FR-020 and T010, T044.

## Root Cause Validation

The documented root cause is internally consistent with repo evidence:

- OpenAI subscription credential resolution already produces refreshed `BearerToken` and `AccountID`.
- ChatGPT Codex backend transport already uses those fields for Codex requests.
- Credential detail currently shows generic quota state from `RateLimitStore`, not a Codex-specific usage snapshot.
- `/ui/usage` currently has a Claude Code tab but no Codex/OpenAI subscription credential tab.
- Therefore Codex usage is available through credential-backed upstream probing, but no Tianji component fetches, normalizes, caches, or displays it.

## Scope Risks

- **Risk**: Implementation treats unofficial `wham/usage` as a stable billing API.
  - Mitigation: Spec and UI contract require best-effort operational snapshot language and safe stale/error states.
- **Risk**: A second token decrypt path leaks or drifts from refresh semantics.
  - Mitigation: FR-005 and T023 require existing resolution/refresh helpers.
- **Risk**: UI refresh calls spam `chatgpt.com`.
  - Mitigation: TTL >= 60s, active backoff, and manual refresh respecting cache/backoff.
- **Risk**: Raw response/token material is persisted for debugging.
  - Mitigation: FR-013 and redaction tasks explicitly forbid this across DB, JSON, HTML, logs, and audit metadata.
- **Risk**: Usage fetching accidentally lands on proxy hot path.
  - Mitigation: FR-010/FR-020 and no-proxy-fetch regression tests.

## Out Of Scope Confirmed

- No Codex CLI change.
- No proxy routing or payload change.
- No per-request usage endpoint fetch.
- No billing-source-of-truth claim.
- No raw response persistence.
- No cookie-based credential storage.
- No Claude Code tab redesign.

## Scope Confirmation

This Todo docs-only scope is ready to move to Waiting for implementation confirmation. No owner decision is required before writing code in a later state.
