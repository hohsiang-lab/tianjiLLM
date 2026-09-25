# Remove Unused Tianji Admin Domains — Design Specification

**Date:** 2026-09-04
**Repository:** `github.com/praxisllmlab/tianjiLLM`
**Status:** Written design for user review; the design sections were approved in chat before implementation.

## Goal

Remove the unused Tianji admin domains represented by these UI routes:

- `/ui/access-groups`
- `/ui/guardrails`
- `/ui/orgs`
- `/ui/teams`

Remove their dedicated UI, routes, navigation entries, permissions, handlers, services, API endpoints, and tests when those artifacts have no remaining consumer. Preserve unrelated pages and shared runtime behavior. Do not alter existing database schema or data.

## Approved decisions and hard boundaries

1. **Breaking removal is intentional.** The management UI and the dedicated operational API/runtime for the four domains are removed rather than hidden, redirected, or kept behind an obsolete compatibility switch.
2. **Database is preserved.** Existing tables, rows, migrations, and historical generated model types remain. This work does not delete, rewrite, backfill, or migrate organization, team, access-group, guardrail, or policy data.
3. **No synthetic compatibility behavior.** The change adds no new legacy-data writes, migration path, fallback lookup, or special-case branch merely to preserve the removed management features. If a field or helper is still consumed by an unrelated live feature, it remains and is classified as shared rather than removed by name.
4. **Scope ambiguity is a stop condition.** If removing a shared organization/team scope requires choosing new fail-open versus fail-closed behavior for a live key/model/API path, implementation stops at that boundary for explicit product direction; it must not silently invent a policy.
5. **No unnecessary E2E additions.** Do not add E2E tests solely to assert that the four removed URLs return 404 or that the sidebar has no corresponding entries. Absence is proven by the route source/list and residual-reference audit. Existing E2E tests that exercise still-supported pages and flows remain in the suite.
6. **No destructive operational work.** Do not run production data operations, destructive migrations, SQL bypasses, external-service changes, test skipping, coverage-threshold reductions, or CI bypasses.

## Current architecture impact

The four domains are not UI-only. The baseline audit found references in UI routing and permissions, proxy route registration and handlers, generated DB queries/interfaces/models, key and model flows, auth/access-control code, spend/callback attribution, A2A agent access filtering, and the policy/guardrail pipeline. Therefore removal is organized as three dependent, independently reviewable slices instead of deleting page files only.

### PR1 — Organizations and Teams

Base: `origin/main`
Branch: `remove-unused-admin-domains/pr1`

Remove the dedicated organization/team management surface:

- UI route registrations and permission constants/role grants for `/orgs*` and `/teams*`.
- Organization/team list, detail, membership, model-assignment, block/unblock, callback, and delete handlers and their templates/tests when they have no remaining consumer.
- Proxy route blocks and dedicated organization/team handlers/tests.
- DB query sources, Store interface methods, and generated query output that are proven to serve only the removed management endpoints.
- Navigation entries and page-specific assets/components that are not used by retained pages.
- Cross-cutting callers that exist only to support removed organization/team management.

Before deleting a shared symbol, search all callers and classify it as one of:

- management-only and removable;
- shared by a retained key/model/credential/usage/auth flow and preservable;
- historical schema/generated artifact that is preserved because the schema is unchanged.

The PR must not remove organization/team data, schema migrations, or a shared field solely because its name contains `Organization` or `Team`.

### PR2 — Access Groups

Base: the verified PR1 head
Branch: `remove-unused-admin-domains/pr2`

Remove the dedicated access-group surface:

- UI routes, permission constant/role grants, handlers, templates, generated templates, and management-only tests.
- Proxy access-group route/handler and its tests.
- Access-group query sources, Store methods, and generated query output that have no remaining consumer.
- Access-group-only fields, filtering, and registry/permission plumbing in keys and A2A agents, while preserving the retained Agents and Skills APIs and any genuinely shared authorization behavior.

`ModelAccessGroup` and schema history remain when generated from the unchanged schema or required for historical representation. No access-group rows are deleted or rewritten.

### PR3 — Guardrails and Policy

Base: the verified PR2 head
Branch: `remove-unused-admin-domains/pr3`

Remove the dedicated guardrail/policy surface:

- Guardrail UI routes, permission constant/role grants, handlers, templates, generated templates, and management-only tests.
- Dedicated proxy policy/guardrail routes and handlers.
- `internal/guardrail/`, `internal/policy/`, policy-specific Store/query/model code, and router initialization/wiring only after complete caller audit proves they are exclusive to this removed feature.
- Tianji policy/guardrail evaluation from the chat pipeline.

Keep provider-owned content-policy fallback behavior in the router/provider path. A provider error must continue to use the existing generic fallback semantics; removing Tianji policy configuration must not remove unrelated provider routing.

Policy/guardrail schema history and data remain untouched. Historical generated types may remain when required by the unchanged schema or other generated output.

## Component and error boundaries

### UI and navigation

`internal/ui/routes.go` is the source of truth for registered UI routes. Remove only the four domain route blocks and their dedicated middleware permission references. Remove corresponding navigation items from the layout source, then regenerate templ output using repository tooling; do not hand-edit generated `_templ.go` files.

Retained routes—keys, credentials, models, usage, logs, users, and other existing pages—keep their current paths, permissions, and handlers. No replacement landing page or redirect is introduced for a removed route.

### HTTP/API

Remove dedicated proxy route blocks and handlers for the four domains. With no route registered, chi's normal not-found behavior is the expected result for those removed management endpoints. Do not add a catch-all that turns their absence into an alternate API behavior.

Retained API routes and shared middleware remain unchanged unless a compile-verified caller audit identifies a management-only dependency. Authentication and authorization checks for retained endpoints must remain before storage or upstream access.

### DB and generated code

Edit only query source files when a query is management-only, then run the repository's SQLC generation command. Do not hand-edit generated query files. Preserve migration files and schema definitions. Generated structs may remain when they represent preserved schema history or a retained shared contract.

The DB Store interface is reduced only for methods with no remaining caller after the corresponding UI/API/runtime removal. A method used by a retained feature is not removed merely because it accepts an organization, team, access-group, policy, or guardrail identifier.

### Legacy data and scope

Existing rows are not migrated, marked deleted, or read through a new compatibility path. Existing shared scope fields are handled only by the retained code that demonstrably still owns them. If removing a caller would expose a live semantic choice about legacy team/org-scoped keys or models, stop and request that choice instead of silently broadening access or manufacturing rejection behavior.

## Generation and source-of-truth rules

- Edit `.templ` source, then run the repository UI generation/build command (`make ui` or the narrower repository-supported templ command).
- Edit SQL query sources, then run `make generate`.
- Never hand-edit generated templ or SQLC files.
- Do not modify schema migrations, CI coverage thresholds, or unrelated build configuration.
- Keep generated diffs semantically limited to the removed queries/templates and required compile updates; investigate unrelated generator churn instead of normalizing it blindly.

## Testing and verification strategy

### TDD for retained behavior

For each retained behavior whose interface changes, write the smallest focused regression test first, run it to observe the intended failure, implement the minimum change, and rerun it green before broader regression checks. Pure deletion with no new behavior does not justify an artificial test that only asserts absence; existing route/source audits and retained-flow tests are sufficient.

### Per-PR checks

Run the smallest touched-package tests first, followed by repository-native checks as available:

- `go test -race` for touched packages and their direct callers;
- `make lint`;
- `make test` (including DB-backed portions when the environment provides PostgreSQL);
- `make build`;
- repository E2E suite only for existing retained flows, without adding four URL-absence/sidebar-absence cases.

Local evidence must distinguish the non-DB tests that ran from DB-backed tests that could not run because the local PostgreSQL service is unavailable. Hosted CI remains the authoritative full integration/E2E gate when it supplies the required service containers.

### Static residual audit

At every final PR head:

1. Enumerate registered routes from the actual UI/proxy route sources.
2. Search exact URL fragments, route names, navigation labels, handler names, permission constants, and dedicated package/file names with `git grep`.
3. Classify remaining matches as retained shared usage, unchanged schema/migration history, required generated historical output, or an unexplained residual.
4. Require zero unexplained management UI/API/runtime residuals for the four domains.
5. Confirm retained route/package tests still compile and pass.

This audit is not a reason to delete shared organization/team authorization, key, model, credential, spend, callback, A2A, or provider fallback code that still has a live caller.

## Review and delivery workflow

The implementation uses stacked PRs so each slice has an exact base and head:

1. PR1: Organizations/Teams, base `origin/main`.
2. PR2: Access Groups, base the verified PR1 head.
3. PR3: Guardrails/Policy, base the verified PR2 head.

For each PR, record the exact `base_sha`, `head_sha`, changed-path allowlist, focused test output, and residual audit. Run the requested review sequence against the exact current head:

1. code review A;
2. code review B;
3. Ponytail review;
4. requirement review;
5. E2E coverage assessment.

Every finding is first verified against the current source and approved scope. Blocking or important findings are fixed in a new TDD cycle, then the affected exact range is re-reviewed. Any new commit invalidates prior head-bound review and CI evidence. Continue until the review ledger has no unresolved findings and the hosted CI pipeline is terminal-successful for the exact final head.

No merge, deploy, or production operation is implied by this design. The integration decision remains a human gate after review and CI evidence are complete.

## Acceptance criteria

- The four requested UI route blocks, navigation entries, dedicated permissions, handlers, templates, and management-only tests are absent from the final source.
- Dedicated API/service/runtime code is absent only where the complete caller audit proves it is no longer used.
- Retained pages, shared authorization, provider fallback, keys/models/credentials/usage, and other unrelated flows remain available and tested.
- Database schema, migrations, existing rows, and required historical generated representations are preserved.
- No new 404/sidebar E2E tests were added solely for removed behavior.
- UI and SQL generated artifacts are produced through repository tooling and have no unexplained drift.
- Local and hosted verification evidence is separate, exact-head bound, and honest about unavailable local PostgreSQL.
- All requested review seats are clean after fix-and-re-review loops, and the final exact-head CI pipeline is green before any merge decision.
