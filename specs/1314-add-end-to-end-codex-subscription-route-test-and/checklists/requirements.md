# Requirements Checklist: HO-1314

## Scope

- [x] Captures live failure after HO-1306 streaming support.
- [x] Covers full `openai/*` subscription Codex chain.
- [x] Includes credential lookup/read integration, not only provider mocks.
- [x] Preserves API-key-backed `openai/*` behavior.
- [x] Defines secret-safety requirements.

## Testability

- [x] Primary RED test has a clear failing condition.
- [x] Missing-row and lookup/read failure are separate test cases.
- [x] Codex backend is locally mocked.
- [x] Platform wrong-route sentinel is specified.
- [x] Verification commands are explicit.

## Governance

- [x] Todo phase contains docs only.
- [x] Production/test code is blocked until Linear `In Progress`.
- [x] Draft PR is for scope review.
- [x] Recovery checkpoint is documented.

## Open Questions

- [x] No owner input required before Waiting.
