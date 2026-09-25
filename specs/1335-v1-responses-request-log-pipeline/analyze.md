# Analysis: HO-1335 `/v1/responses` request-log pipeline

## Gate Result

- Fatal issues: 0
- Critical issues: 0
- Owner-input blockers: 0

## Requirement Coverage

- Successful HTTP `/v1/responses` visibility: covered by FR-001, US1, T002, SC-001.
- Failed HTTP `/v1/responses` visibility: covered by FR-002, US2, T003, SC-002.
- Successful WebSocket primary-path visibility: covered by FR-003, US3, T004, SC-003.
- WebSocket warmup/synthetic no double-count: covered by FR-005, US4, T005, SC-004.
- Normal operational fields: covered by FR-007, data model, T008, T019.
- Token/cost best-effort: covered by FR-008, T016-T018.
- Existing chat-completions behavior unchanged: covered by FR-011, US5, T006, SC-005.
- Regression tests: covered by T001-T006 and T020-T022.
- Live operator verification: covered by FR-013, T024, quickstart live verification.

## Root Cause Validation

The documented root cause is internally consistent with repo evidence:

- `/ui/logs` reads DB request-log views.
- Existing chat Codex paths invoke the callback logging helpers.
- Responses HTTP/WebSocket Codex paths perform upstream model work without invoking those helpers.
- Therefore successful model work can bypass UI-visible request-log persistence.

## Scope Risks

- **Risk**: Implementation fakes a chat-completion response only to reuse `logSuccess`.
  - Mitigation: Plan allows a Responses-specific adapter/helper as long as it feeds the same callback/spend pipeline.
- **Risk**: WebSocket prewarm is logged because it emits synthetic completion events.
  - Mitigation: Explicit `realUpstreamWork=false` rule and T005.
- **Risk**: Stream usage extraction is incomplete.
  - Mitigation: FR-008 requires visible zero-token rows when usage is absent.
- **Risk**: Failure logging leaks upstream secret material.
  - Mitigation: FR-009 and data-model redaction rules.

## Out Of Scope Confirmed

- No UI query/source redesign.
- No stdout-tail logging feature.
- No Codex CLI changes.
- No OpenAI subscription CRUD/routing redesign.
- No WebSocket state-chain behavior change beyond not logging synthetic prewarm.

## Scope Confirmation

This Todo docs-only scope is ready to move to Waiting for implementation confirmation. No owner decision is required before writing code in a later state.
