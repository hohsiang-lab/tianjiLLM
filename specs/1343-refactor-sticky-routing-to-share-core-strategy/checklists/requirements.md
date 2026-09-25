# Requirements Checklist: HO-1343

## Completeness

- [x] Scope names both sticky implementations being refactored.
- [x] Problem statement includes the root cause chain.
- [x] Shared sticky core responsibilities are explicit.
- [x] Claude unchanged behavior is protected.
- [x] Codex fresh cached usage snapshot behavior is covered.
- [x] Missing/stale/backoff snapshot fallback is covered.
- [x] Proxy hot-path no-fetch rule is explicit.
- [x] Out-of-scope UI, token lifecycle, and Codex CLI changes are listed.

## Testability

- [x] Shared core has focused unit test requirements.
- [x] Claude parity tests are required.
- [x] Codex cached-snapshot selection tests are required.
- [x] Chat completions, Responses HTTP, and Responses WebSocket no-fetch tests are required.
- [x] Deterministic fallback is independently testable.

## Safety

- [x] Candidate identity forbids raw secrets.
- [x] No new synchronous upstream usage fetch is allowed in request routing.
- [x] Credential error behavior remains typed and sanitized.
- [x] Existing API-key path remains protected.

## Ambiguity

- [x] No owner-input blocker remains for Todo scope.
- [x] Implementation may choose exact helper/package shape while preserving provider-neutral core and provider-specific policies.
