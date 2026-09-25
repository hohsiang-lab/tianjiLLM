# SpecKit Analyze: Credential Test/Refresh/Disable/Delete actions UI

**Issue**: HO-1188
**Date**: 2026-05-08
**State**: Todo planning

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Linear Scope Alignment

| Linear requirement | Spec/plan/tasks coverage |
| --- | --- |
| Test action | `spec.md` FR-003, SC-001; `tasks.md` T001-T004 |
| Refresh action | `spec.md` FR-004, SC-002; `tasks.md` T001/T004 |
| Disable action | `spec.md` FR-005, SC-003; `tasks.md` T005-T006 |
| Delete action | `spec.md` FR-006, SC-004; `tasks.md` T007-T008/T028-T030 |
| Existing confirm + toast pattern | `spec.md` User Story 3, FR-007/FR-012/FR-015; `plan.md` UI Layout Plan |
| Action results update safe status metadata | `spec.md` FR-009/FR-010/FR-011; `plan.md` Data Flow |
| UI E2E for all actions | `tasks.md` Phase 1 |
| Confirm/toast behavior matches current UI pattern | `research.md`, `plan.md`, `tasks.md` T021-T025 |
| Action error states render safely | `spec.md` User Story 4, FR-008/FR-014/FR-017; `tasks.md` T009-T010 |

## Repo Reality Alignment

- Existing credentials UI is server-rendered under `internal/ui` and protected via `sessionAuth`; plan keeps this boundary.
- Existing backend lifecycle endpoints already exist under auth middleware; plan consumes them rather than redefining backend semantics.
- Existing toast patterns use `internal/ui/components/toast` with HTMX partial/OOB responses; plan requires reuse.
- Existing HO-1186 credentials E2E already cover list/detail/quota/redaction; plan extends, not replaces, those tests.

## Mockscreen Scope Review

HO-1188 is FE/UI and includes owner-facing actions, so mockscreen evidence is required before Waiting.

Mockscreen evidence:

- Desktop PNG: `/Users/n0rmanc/.openclaw/media/HO-1188-desktop.png` (`1440x1000`), posted to Discord thread message `1502285105408770120`.
- Mobile PNG: `/Users/n0rmanc/.openclaw/media/HO-1188-mobile.png` (`780x1688`), posted to Discord thread message `1502285120541818973`.
- Visual sanity: desktop shows credentials list row actions, detail action toolbar, success toast, and delete confirmation panel; mobile shows action controls wrapping inside list content without overlap.

Mockscreen shows:

- list-row action controls
- detail-page action toolbar
- success/error toast region
- disabled and delete-confirm/destructive affordance
- responsive wrapping for action controls

## Risk Review

### Secret egress

Risk: lifecycle failures can carry upstream/token text into toast or metadata.

Mitigation: stable reason-code UI copy, existing redaction helper, DOM/toast/attribute E2E secret assertions.

### Backend/UI contract drift

Risk: UI reimplements lifecycle behavior and diverges from HO-1185.

Mitigation: UI wrapper must call existing lifecycle/delete behavior or shared helper; no OpenAI token/test logic in UI layer.

### Stale UI after mutation

Risk: action response updates only toast and leaves status stale.

Mitigation: reload DB-safe metadata and refresh affected row/detail after success.

### Delete from detail

Risk: stale detail page remains after delete.

Mitigation: explicit safe deleted-state or redirect-to-list task.

## Plan Review Evidence

- Context7 htmx evidence confirms `hx-confirm` for request confirmation.
- GitHub grep-app evidence confirms real HTMX admin UIs use `hx-confirm` + `hx-post` + partial target/swap for destructive actions.
- Official OpenAI API reference confirms model-list test remains bearer-authenticated backend concern; UI shows only safe summary.

## Gate Decision

Todo SpecKit artifacts are internally consistent and have no open owner question. Draft PR can be opened as docs-only planning PR after mockscreen artifact is generated and posted.
