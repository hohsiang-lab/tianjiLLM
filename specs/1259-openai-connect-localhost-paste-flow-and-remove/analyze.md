# Analyze: HO-1259 OpenAI localhost paste flow

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
|---|---:|---:|---:|---|
| Remove `public_base_url` OpenAI OAuth dependency, including `proxy.yml` / proxy YAML artifacts | FR-001, FR-002, FR-015, SC-003 | Decisions 1, cleanup | T013-T016, T034 | Covered |
| Default localhost callback | FR-004, SC-001 | Target flow, Decision 1 | T001, T021 | Covered |
| New-tab authorize launch keeps Tianji credentials page available | FR-005, SC-007 | Target flow, UI Scope | T005, T030 | Covered |
| Credentials-page paste modal | FR-010, SC-004 | Decision 4, UI Scope | T006, T029 | Covered |
| Explicit redirect override | FR-003, FR-006 | Decision 1 | T013 | Covered |
| Persist exact redirect URI in state | FR-007, SC-002 | Decision 2 | T003, T017-T020 | Covered |
| Token exchange uses stored redirect URI | FR-008, SC-002 | Target flow | T007, T024-T026 | Covered |
| Server-side pasted URL parsing | FR-011, FR-012 | Data flow, Risk Register | T008, T031 | Covered |
| Direct callback preserved/shared | FR-013 | Decision 3 | T024-T026 | Covered |
| Credential save success | FR-014 | Target flow | T009, T025-T027 | Covered |
| Fixture updates | FR-015 | Test Plan | T023, T034 | Covered |
| Secret redaction | FR-016, SC-006 | Security notes, Risk Register | T010, T033 | Covered |
| Regression tests | FR-017 | Test Plan | T001-T012, T035-T036 | Covered |

## Consistency Checks

- `tasks.md` begins with RED tests before production implementation, satisfying bug/TDD discipline.
- `plan.md` and `spec.md` both reject `public_base_url` fallback for OpenAI OAuth.
- `plan.md`, `spec.md`, `tasks.md`, and `quickstart.md` now include Norman's explicit `proxy.yml` cleanup request.
- `plan.md`, `spec.md`, `tasks.md`, `contracts/`, and `quickstart.md` now make the OpenAI authorize launch a new-tab/window UX contract.
- `plan.md`, `spec.md`, `tasks.md`, `contracts/`, and `quickstart.md` now make `/ui/credentials` paste modal the primary paste UX.
- Direct callback and pasted callback share one helper in both architecture and tasks.
- UI route is protected, but callback completion remains state-authenticated and does not trust UI/form org values.
- External OpenAI evidence is scoped correctly: Codex docs/source support localhost callback; Apps SDK auth docs do not justify hosted Tianji callback.

## Scope Confirmation

Default scope is sufficient and owner input required is 0:

1. Remove OpenAI OAuth runtime dependency on `public_base_url`, including `proxy.yml` / proxy YAML config artifacts.
2. Default OpenAI authorize redirect to `http://localhost:1455/auth/callback`.
3. Open OpenAI authorize in a new tab/window so the original `/ui/credentials` page remains available.
4. Persist exact redirect URI in OAuth state and use it for exchange.
5. Add protected Tianji paste modal on `/ui/credentials` for full localhost callback URL.
6. Preserve direct callback through shared completion logic.
7. Cover redirect identity, new-tab launch, credentials modal, paste parsing, replay, credential persistence, and leakage with RED-first tests.
