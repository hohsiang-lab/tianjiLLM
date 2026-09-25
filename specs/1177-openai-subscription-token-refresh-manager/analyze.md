# SpecKit Analyze: OpenAI Subscription Token Refresh Manager

**Date**: 2026-05-07
**Scope**: `spec.md`, `plan.md`, `tasks.md`, supporting docs

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Cross-Artifact Consistency

- Spec FR-002/FR-003 maps to plan fresh-token and expiry-buffer tests.
- Spec FR-004/FR-005 maps to provider refresh helper contract and request-shape tests.
- Spec FR-007/FR-008 maps to HO-1176 persistence helper reuse and rotation tests.
- Spec FR-010/FR-013 maps to HO-1183 redaction reuse and failure-metadata tests.
- Spec FR-011 maps to Context7 singleflight evidence and concurrent refresh test.
- Spec FR-012 maps to typed error data model and test coverage.
- Tasks are unchecked and implementation-scoped; no production task is marked complete in Todo state.

## Repo Reality Check

- Current resolver can return `credential expired`; plan changes this to refresh-before-error.
- Current provider OAuth helper only exchanges authorization codes; plan adds refresh helper.
- Current test harness already supports refresh-token fixtures; no real OpenAI call is needed.
- Existing DB helpers are enough for success/failure persistence; no schema migration is planned.

## External Evidence Check

- Context7 singleflight evidence supports per-key duplicate suppression.
- Context7 oauth2 evidence supports early-expiry buffer behavior.
- OpenAI Codex app-server protocol evidence supports client-managed refresh for external `chatgptAuthTokens`.
- grep-app failed and is recorded rather than treated as evidence.

## Open Questions

None. The only ambiguous phrase, "normal expiry buffer", is resolved by adopting a 5-minute default with a testable constant.
