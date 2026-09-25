# Requirements Checklist: HO-1186

## Issue Alignment

- [x] Linear title `[FE] Credentials sidebar list/detail quota UI` is represented.
- [x] Parent HO-1171 recorded in spec context via dependencies.
- [x] Goal "Credentials UI surface for viewing OpenAI subscription credentials and quota/status" is represented.
- [x] Scope includes sidebar `Credentials` page.
- [x] Scope includes credential list page.
- [x] Scope includes credential detail page.
- [x] Detail includes account email.
- [x] Detail includes status.
- [x] Detail includes utilization/quota usage.
- [x] Detail includes reset time.
- [x] Detail includes `last_refresh_at`.
- [x] Detail includes `last_error`.
- [x] Detail includes `disabled_reason`.
- [x] Quota UI follows cards/table/progress bar/status badge/reset time patterns.
- [x] Tests include UI E2E for credentials list/detail.
- [x] Tests include quota/status rendering.
- [x] Tests include safe metadata only and no token display.

## Quality Gates

- [x] No production code planned in Todo.
- [x] Implementation gated on Linear `In Progress`.
- [x] First implementation task is failing E2E.
- [x] Unknown quota is distinct from zero quota.
- [x] Secret fields are excluded from view models and rendered DOM.
- [x] Existing UI component patterns are reused.
- [x] No new frontend framework is planned.

## Scope Questions

- [x] Owner input required: 0.
