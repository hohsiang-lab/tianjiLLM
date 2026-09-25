# Specification Analysis Report

**Feature**: HO-2091 Proactive OpenAI Subscription Credential Refresh
**Date**: 2026-06-18
**Artifacts**: `spec.md`, `plan.md`, `tasks.md`

## Findings

| ID   | Category | Severity | Location(s)   | Summary                                                                           | Recommendation                                |
| ---- | -------- | -------- | ------------- | --------------------------------------------------------------------------------- | --------------------------------------------- |
| None | Coverage | None     | All artifacts | No fatal, critical, high, medium, or low consistency issues found for Todo scope. | Proceed to docs-only draft PR / Waiting gate. |

## Coverage Summary

| Requirement Key          | Has Task? | Task IDs               | Notes                                                      |
| ------------------------ | --------- | ---------------------- | ---------------------------------------------------------- |
| proactive-job-every-5m   | Yes       | T011, T014, T015       | Scheduler registration covered.                            |
| active-within-30m-only   | Yes       | T009, T010, T013       | Separate proactive buffer covered.                         |
| keep-5m-request-buffer   | Yes       | T005, T013, T016       | Existing request freshness preserved.                      |
| reuse-refresh-path       | Yes       | T013, T020             | Existing helper reuse covered.                             |
| no-stampede              | Yes       | T017-T023              | Same-process and distributed lock covered.                 |
| update-success-metadata  | Yes       | T026, T030             | Success cleanup covered.                                   |
| persist-failure-metadata | Yes       | T024, T025, T028, T029 | First/latest failure state covered.                        |
| reconnect-required       | Yes       | T024, T029, T031       | Exact operator message covered.                            |
| routing-skip-failed      | Yes       | T033, T035, T038, T042 | Failed/deleted candidate skip covered.                     |
| stale-route-visible      | Yes       | T034, T039-T041        | UI visibility covered.                                     |
| api-key-unchanged        | Yes       | T038, T044             | Regression surface covered through affected handler tests. |
| secret-redaction         | Yes       | T036, T042, T049       | Redaction proof covered.                                   |

## Constitution Alignment Issues

None.

## UI / Alignment Gate

- This issue has operator-facing UI text/status surfacing, but does not define a new screen, layout, navigation path, form, dialog, or responsive design.
- Alignment source is the existing Tianji credential/model administration surfaces already represented by `internal/ui/handler_credentials.go`, `internal/ui/handler_models.go`, and `internal/ui/pages/models.templ`.
- The required UI behavior is exact status/diagnostic wording and sanitized stale-reference visibility, not visual redesign: `OpenAI session ended; reconnect required` plus stale route references without secrets.
- Desktop/mobile mockscreen attachments are not required for Todo because no new UI layout is being designed. In Progress must validate the existing UI surfaces with handler/template tests and templ generation if templates change.

## Unmapped Tasks

None. Setup/generation/polish tasks support covered requirements and Tianji constitution gates.

## Metrics

- Total Requirements: 16
- Total Tasks: 49
- Coverage: 100%
- Ambiguity Count: 0
- Duplication Count: 0
- Fatal Issues Count: 0
- Critical Issues Count: 0

## Todo Gate Status

- `fatal=0`
- `critical=0`
- `Missing part: none for Todo scope`
- `UI alignment: existing credentials/models operator surfaces; no new layout mockscreen required for Todo`
- `實作細節：已釐清 — repo ownership, scheduler host, refresh helper, lock strategy, metadata shape, routing exclusion, UI status source, and verification are backed by repo/external evidence.`

## Implementation Addendum

- `fatal=0`
- `critical=0`
- Implemented backend scheduler path, 30-minute proactive refresh eligibility, existing `singleflight` reuse, Redis scheduler lock wrapping, reconnect-required metadata, routing exclusion for failed credentials, and existing models/credentials UI status surfacing.
- Focused and affected-package Go tests pass for proactive refresh, transient vs reconnect-required metadata, redaction, routing exclusion, stale model references, scheduler registration, and `cmd/tianji` compile.
- UI alignment remains existing credentials/models operator surfaces; no new layout, dialog, or template interaction was introduced, so mockscreen and templ regeneration are not required.
- TDD red-run evidence was not reliably preserved for every test-writing task because implementation and tests landed in the same chunk; green regression evidence is recorded in PR/Linear/thread evidence instead.
