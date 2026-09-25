# Requirements Checklist: HO-1335

## Completeness

- [x] Scope names the affected route and pipeline.
- [x] Problem statement includes the full root cause chain.
- [x] HTTP success logging is covered.
- [x] HTTP failure logging is covered.
- [x] WebSocket generation success logging is covered.
- [x] WebSocket generation failure logging is covered.
- [x] WebSocket prewarm no-double-count behavior is covered.
- [x] UI source of truth is specified as DB request-log queries.
- [x] Existing chat-completions behavior is protected.
- [x] Live operator verification is required.

## Testability

- [x] Each P1 user story has an independent test.
- [x] Regression tests are expected to fail on current main before implementation.
- [x] Token usage/cost behavior is testable with usage-present and usage-absent responses.
- [x] Prewarm behavior is testable without backend calls.
- [x] Live verification does not rely on raw pod logs.

## Safety

- [x] Redaction requirements are explicit.
- [x] No credential/token material is required in artifacts.
- [x] UI query redesign is out of scope.
- [x] Stdout/access-log tailing is out of scope.
- [x] Codex CLI changes are out of scope.

## Ambiguity

- [x] No owner-input blocker remains for Todo scope.
- [x] Implementation has freedom to choose helper structure while preserving behavior.
- [x] Unknown usage extraction details are handled with best-effort plus visible zero-token rows.
