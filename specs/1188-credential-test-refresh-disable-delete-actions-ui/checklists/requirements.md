# Requirements Checklist: Credential actions UI

## Spec Quality

- [x] No implementation code in Todo planning.
- [x] User stories are independently testable.
- [x] Functional requirements are numbered and verifiable.
- [x] Edge cases are listed.
- [x] Out-of-scope boundaries are explicit.
- [x] Dependencies on HO-1184/HO-1185/HO-1186/HO-1187 are recorded.

## Security

- [x] Raw credential value is forbidden from UI output.
- [x] Token material is forbidden from DOM/toast/attributes.
- [x] Backend lifecycle response is treated as safe summary only.
- [x] Action errors use stable reason codes/safe copy.
- [x] E2E secret non-rendering assertions are required.

## UI Scope

- [x] List-page action controls are required.
- [x] Detail-page action controls are required.
- [x] Confirm is required for Disable/Delete.
- [x] Toast is required for success/error.
- [x] No new frontend framework/package.
- [x] Existing HO-1186 quota/list/detail behavior remains compatible.

## Tests

- [x] Test action E2E required.
- [x] Refresh action E2E required.
- [x] Disable action E2E required.
- [x] Delete action E2E required.
- [x] Confirm cancel/accept E2E required.
- [x] Error-state E2E required.
- [x] Offline/mock OpenAI behavior required.

## Owner Input

- [x] No unresolved owner input. Linear scope is explicit.
