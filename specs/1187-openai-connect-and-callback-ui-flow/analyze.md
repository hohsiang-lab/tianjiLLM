# Analyze: OpenAI connect and callback UI flow

**Result**: fatal 0 / critical 0 / owner input 0

## Linear Alignment

- Linear Goal: "Implement the user-visible OpenAI connect flow and callback result pages."
  - Covered by `spec.md` Scope, User Story 1, User Story 2, User Story 3.
- Scope: "Add OpenAI Connect entry point."
  - Covered by FR-001 and tasks T012-T014.
- Scope: "Start connect from authenticated UI session."
  - Covered by FR-002, FR-013, T010, T015.
- Scope: "Render callback success state."
  - Covered by FR-005 to FR-007, T017-T020.
- Scope: "Render callback failure as readable UI error page."
  - Covered by FR-008 to FR-010, T007-T009, T020-T021.
- Scope: "Follow current toast/notification conventions where applicable."
  - Covered by plan architecture requiring existing Tianji UI style/assets/components. Toast is not required for public callback because callback is a full-page public terminal route outside `/ui` session.
- Tests: success with mocked OAuth/token upstream.
  - Covered by T001-T006 and quickstart.
- Tests: callback failure/error state.
  - Covered by T007-T009 and failure smoke cases.
- Tests: unauthenticated connect blocked/redirected.
  - Covered by T010 and FR-013.

## Repo Reality Alignment

- `/ui/openai/connect` already exists and is inside `sessionAuth`; spec avoids reimplementing OAuth start internals.
- `/oauth/openai/callback` already exists and is public; spec preserves state-authenticated public callback behavior.
- Existing callback result renderer is minimal HTML; spec targets the actual user-visible gap.
- `Credentials` UI already exists on `origin/main`; spec places the CTA there by default.
- Existing OpenAI mock harness exists; spec requires offline E2E through local mock endpoints.

## Cross-Artifact Consistency

- `spec.md` FRs map to `tasks.md` tests and implementation tasks.
- `plan.md` keeps implementation bounded to CTA + result pages + E2E.
- `data-model.md` defines only UI view/result/test models, not new DB schema.
- `contracts/openai-connect-ui-flow.md` matches existing route ownership.
- `quickstart.md` uses the same verification commands as `plan.md`.

## Out-of-Scope Challenge

- Credential CRUD/list/detail quota UI is not pulled into HO-1187; only CTA integration and post-success visibility are in scope.
- Backend state/PKCE/callback persistence is not rewritten; HO-1175 owns it.
- Token refresh/test/disable/delete lifecycle is excluded.
- Real OpenAI calls are explicitly forbidden.

## Owner Input

Owner input required: 0.

Default scope is actionable: `Credentials` CTA + safe styled callback success/failure pages + offline UI E2E.
