# Requirements Checklist: HO-1342

## Scope Clarity

- [x] Problem describes current irreversible disabled state.
- [x] Root cause maps UI, proxy route, RBAC, lifecycle handler, metadata, and resolver behavior.
- [x] In-scope enable lifecycle is explicit.
- [x] Out-of-scope OAuth reconnect and delete semantics are explicit.
- [x] Non-operator disabled reason semantics are explicitly out of scope for enable.

## Testability

- [x] Each user story has an independent test.
- [x] Functional requirements include UI list/detail coverage.
- [x] Functional requirements include proxy route/RBAC coverage.
- [x] Functional requirements include resolver/routable regression.
- [x] Functional requirements include redaction/audit coverage.

## Safety

- [x] Enable is metadata-only by default.
- [x] Token decrypt/rewrite is prohibited for enable.
- [x] Audit metadata forbids token/raw credential leakage.
- [x] Auth/refresh failure disabled reasons are not silently broadened without owner confirmation.

## Docs-Only Todo Gate

- [x] Spec exists.
- [x] Plan exists.
- [x] Tasks exist.
- [x] Research exists.
- [x] Data model exists.
- [x] Quickstart exists.
- [x] Contract exists.
- [x] Analyze exists.
