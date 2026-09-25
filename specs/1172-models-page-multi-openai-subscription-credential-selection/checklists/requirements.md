# Requirements Checklist: HO-1172

## Content Quality

- [x] Spec is written in Traditional Chinese with technical identifiers preserved。
- [x] Requirements are user-facing and testable。
- [x] Scope and out-of-scope boundaries are explicit。
- [x] No production implementation is included in Todo planning。

## Requirement Completeness

- [x] Create Model multi-select is covered。
- [x] Edit Model prefill/add/remove/save is covered。
- [x] Invalid subscription IDs + custom `api_base` combination is covered。
- [x] Existing API-key behavior regression is covered。
- [x] Safe metadata/no-secret rendering is covered。
- [x] Missing/deleted selected credential IDs are covered。
- [x] Empty credential list is covered。

## Testability

- [x] E2E coverage is specified for create, edit, invalid combination, API-key regression, and DOM secret boundary。
- [x] Unit/handler coverage is specified for parsing and summary helpers。
- [x] Verification commands are listed。

## Gate Readiness

- [x] Fatal issues: 0。
- [x] Critical issues: 0。
- [x] Owner input required: 0。
- [x] UI mockscreen gate is required before Todo -> Waiting。
