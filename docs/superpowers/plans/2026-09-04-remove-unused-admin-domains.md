# Remove Unused Tianji Admin Domains Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the unused Organizations, Teams, Access Groups, and Guardrails/Policy admin domains and their exclusive runtime callers while preserving retained features, schema, and data.

**Architecture:** Deliver three serial stacked PR slices: Organizations/Teams, Access Groups, and Guardrails/Policy. Each slice removes UI/API/DB operations only after a complete caller audit, regenerates generated artifacts from source, and leaves shared authorization, spend, credential, model, provider, and schema contracts intact.

**Tech Stack:** Go 1.27, chi/v5, pgx/v5, sqlc, templ, HTMX, Tailwind CSS, PostgreSQL, Playwright E2E, golangci-lint, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-04-remove-unused-admin-domains-design.md`

## Global Constraints

- Do not modify PostgreSQL schema, migrations, existing rows, or production data.
- Do not add compatibility writes, migrations, fallback lookups, or new legacy-scope policy.
- Remove a symbol only after `git grep` proves no retained caller; shared `TeamID`, `OrganizationID`, spend, credential, auth, A2A, and provider-fallback code is preserved when still used.
- If removing a shared scope requires choosing fail-open versus fail-closed behavior, stop and request product direction.
- Do not add E2E tests for removed URLs returning 404 or for sidebar absence.
- Do not skip tests, lower the 50% CI coverage gate, change CI triggers, or bypass generated-code checks.
- Edit `.templ` and `.sql` sources; use `make ui` and `make generate`; never hand-edit generated output.
- Do not push, merge, deploy, or mutate external systems until the explicit PR/review/CI gates in this plan are reached.

---

## Task 1: Create the durable execution ledger and PR1 baseline

**Files:**
- Create: `.superpowers/sdd/remove-unused-admin-domains/progress.md` with the durable task ledger.
- Create: `.superpowers/sdd/remove-unused-admin-domains/pr1-ui-brief.md`, `.superpowers/sdd/remove-unused-admin-domains/pr1-api-brief.md`, and `.superpowers/sdd/remove-unused-admin-domains/pr1-shared-callers-brief.md`.

**Inputs:**
- Worktree: `/root/.hermes/profiles/coder/worktrees/tianji-remove-unused-admin-domains-pr1`
- Current base: `bfb2080a6f64abb8301d7d450259ed590d593685`
- Approved spec: `docs/superpowers/specs/2026-09-04-remove-unused-admin-domains-design.md`

- [ ] **Step 1: Anchor the exact repository and worktree.**

Run:

```bash
git rev-parse --show-toplevel
git remote get-url origin
git branch --show-current
git rev-parse HEAD
git rev-parse origin/main
git status --short --branch
git worktree list --porcelain
```

Expected: the worktree is the `remove-unused-admin-domains/pr1` branch, `HEAD` is the approved spec commit, and there are no unexplained modifications.

- [ ] **Step 2: Create the SDD ledger and task briefs.**

Create the durable task state with the file tools, not an untracked shell helper:

```text
.superpowers/sdd/remove-unused-admin-domains/progress.md
.superpowers/sdd/remove-unused-admin-domains/pr1-ui-brief.md
.superpowers/sdd/remove-unused-admin-domains/pr1-api-brief.md
.superpowers/sdd/remove-unused-admin-domains/pr1-shared-callers-brief.md
```

The ledger must contain the exact base SHA, allowed paths, forbidden paths, current task, verification commands, and the known local PostgreSQL limitation. Each brief must quote only the relevant task section from this plan and the approved spec. Do not put credentials in the ledger or briefs.

- [ ] **Step 3: Freeze a read-only domain inventory.**

Run:

```bash
git grep -n -E 'access-groups|guardrails|/orgs|/teams|/team|/organization|AccessGroup|Guardrail|Organization|Team|PolicyEng|PolicyEngine' -- '*.go' '*.templ' '*.sql' > .superpowers/sdd/remove-unused-admin-domains/inventory.txt
git diff --check origin/main...HEAD
```

Use the inventory as the review baseline; do not treat a name match as proof that a symbol is removable.

- [ ] **Step 4: Commit only the durable plan/ledger files if the repository convention tracks them.**

Run:

```bash
git status --short
git diff --check
```

Stage only the plan/ledger paths, not build artifacts or generated files, and commit with:

```bash
git add -- docs/superpowers/plans/2026-09-04-remove-unused-admin-domains.md .superpowers/sdd/remove-unused-admin-domains
git commit -m "docs: add admin domain removal plan"
```

If the ledger paths are ignored by repository convention, preserve them as ignored task state and do not force-add them.

## Task 2: Remove PR1 UI and navigation ownership

**Files:**
- Modify: `internal/ui/routes.go`
- Modify: `internal/ui/permissions.go`
- Modify: `internal/ui/pages/layout.templ`
- Delete only after caller audit: `internal/ui/handler_orgs.go`, `internal/ui/handler_orgs_detail.go`, `internal/ui/handler_teams.go`, `internal/ui/handler_teams_detail.go`
- Delete only after caller audit: `internal/ui/pages/orgs.templ`, `internal/ui/pages/orgs_templ.go`, `internal/ui/pages/teams.templ`, `internal/ui/pages/teams_templ.go`
- Delete only after caller audit: `internal/ui/handler_orgs_test.go`, `internal/ui/handler_teams_test.go`
- Modify the retained Users detail only to remove dead links: `internal/ui/pages/users.templ`, `internal/ui/pages/users_templ.go`
- Modify if a retained page consumes a removed helper: `internal/ui/handler_keys.go`, `internal/ui/handler_users_detail.go`
- Regenerate: `internal/ui/pages/*_templ.go` through `make ui`

- [ ] **Step 1: Identify the exact UI-only symbols before editing.**

Run:

```bash
git grep -n -E 'handleOrgs|handleOrg|handleTeams|handleTeam|PermissionTeamsManage|PermissionOrganizationsManage|/ui/(orgs|teams)' -- internal/ui
```

Classify each match as a route, navigation entry, management-only handler/template/test, or retained caller. A `Team`/`Organization` field in a retained page is not a deletion target by itself.

- [ ] **Step 2: Remove the four UI route blocks and dedicated permission grants.**

Delete the `/teams*` and `/orgs*` registrations in `internal/ui/routes.go`. Remove only `PermissionTeamsManage` and `PermissionOrganizationsManage` plus their role-map entries in `internal/ui/permissions.go` when no retained route references them. Leave `PermissionUsersManage`, keys, credentials, models, usage, logs, and dashboard permissions unchanged.

- [ ] **Step 3: Remove the four navigation entries from the templ source.**

Delete only these `navItem` calls from `internal/ui/pages/layout.templ`:

```templ
@navItem("/ui/teams", "Teams", "users", activePath)
@navItem("/ui/orgs", "Organizations", "building-2", activePath)
@navItem("/ui/access-groups", "Access Groups", "layers", activePath)
@navItem("/ui/guardrails", "Guardrails", "shield", activePath)
```

PR1 removes the first two; PR2 removes the latter two. Keep all other navigation entries and the shared `navItem` component.

- [ ] **Step 4: Delete management-only UI source/tests and regenerate.**

Delete only the PR1 handler/template/test files proven to have no retained import or caller. Run:

```bash
make ui
# Run gofmt only on the modified Go paths printed by: git diff --name-only -- '*.go'
```

Expected: generated templ files change only because the removed templates/navigation disappeared; no manual edits to generated files are made.

- [ ] **Step 5: Run the focused UI package test.**

Run:

```bash
go test -race ./internal/ui/...
git diff --check
```

If compilation reveals a retained caller such as key/team selection, do not add a compatibility stub. Trace that caller in Task 4 and either preserve the shared contract or stop at the scope decision boundary.

- [ ] **Step 6: Commit the PR1 UI slice.**

Run:

```bash
git diff --name-status
# After reviewing the list, stage each changed path with: git add -- path/to/file
git commit -m "refactor(ui): remove organization and team admin pages"
```

Parent verification must read back `HEAD`, its parent, the changed-path list, and `git diff --check origin/main...HEAD` before the next task.

## Task 3: Remove PR1 dedicated HTTP/API and database management operations

**Files:**
- Modify: `internal/proxy/server.go`
- Delete after caller audit: `internal/proxy/handler/organization.go`, `internal/proxy/handler/team.go`, `internal/proxy/handler/team_ext.go`
- Delete after caller audit: `internal/proxy/handler/organization_test.go`, `internal/proxy/handler/team_test.go`
- Modify: `internal/db/interface.go`
- Modify source only when management-only: `internal/db/queries/org_membership.sql`, `internal/db/queries/organization.sql`, `internal/db/queries/team.sql`
- Regenerate: `internal/db/org_membership.sql.go`, `internal/db/organization.sql.go`, `internal/db/team.sql.go`, and other SQLC output through `make generate`
- Modify test doubles only for removed interface methods: `internal/proxy/handler/mock_store_test.go`, `internal/proxy/middleware/auth_test.go`, `internal/proxy/middleware/db_validator_test.go`
- Modify test surfaces only where they exercise removed management operations: `internal/proxy/handler/nodb_extra_test.go`, `internal/proxy/handler/nodb_test.go`, `test/contract/budget_test.go`, `test/integration/phase6_management_test.go`
- Audit `internal/auth/rbac.go` for route/permission literals; remove only management-only entries and preserve retained roles
- Delete only management-only contract/integration/E2E tests: `test/contract/organization_test.go`, `test/contract/teams_orgs_test.go`, `test/e2e/orgs_create_test.go`, `test/e2e/orgs_delete_test.go`, `test/e2e/orgs_detail_test.go`, `test/e2e/orgs_list_test.go`, `test/e2e/orgs_members_test.go`, `test/e2e/teams_block_test.go`, `test/e2e/teams_create_test.go`, `test/e2e/teams_delete_test.go`, `test/e2e/teams_detail_test.go`, `test/e2e/teams_list_test.go`, `test/e2e/teams_members_test.go`, `test/e2e/teams_models_test.go`

- [ ] **Step 1: Freeze API caller ownership.**

Run:

```bash
git grep -n -E 'Handlers\.(Org|Team)|Org(New|Info|Update|Delete|Member)|Team(New|Info|List|Delete|Update|Block|Unblock|Member|Model|Available|Permissions|Callback)|/(organization|team)' -- '*.go'
```

Remove the `/team` and `/organization` route blocks in `internal/proxy/server.go` only after classifying every handler reference as API-management-only or retained shared behavior. Keep `/spend/teams` and global/team spend queries when the retained spend UI/API uses them.

- [ ] **Step 2: Remove dedicated handlers and tests.**

Delete the organization/team management handler files and tests only after the caller inventory has no retained import. Do not replace removed endpoints with a new handler; chi not-found behavior is the contract.

- [ ] **Step 3: Remove only unreferenced Store methods and query sources.**

Search each method before deleting it:

```bash
for symbol in AddOrgMember CreateOrganization DeleteOrgMember DeleteOrganization GetOrganization UpdateOrgMember UpdateOrganization AddTeamMember AddTeamModel BlockTeam CreateTeam DeleteTeam GetTeam GetTeamCallback GetTeamDailyActivity GetTeamPermissions ListAvailableTeams ListTeams RemoveTeamMember RemoveTeamModel SetTeamCallback SetTeamPermissions UnblockTeam UpdateTeam UpdateTeamMemberRole; do
  printf '\n## %s\n' "$symbol"
  git grep -n -F "$symbol" -- '*.go' '*.sql'
done
```

Remove a method from `internal/db/interface.go` and its SQL source only when the output contains no retained caller. Preserve methods used by `internal/db/spend*`, `verification_token`, `credential`, `error_log`, or other retained paths. Run:

```bash
make generate
git diff --check
```

- [ ] **Step 4: Reconcile test doubles and generated output.**

Update mocks only to match the reduced `db.Store` interface. Do not add no-op methods to keep deleted management APIs compiling. Run:

```bash
go test -race ./internal/db/... ./internal/proxy/... ./test/contract/...
```

- [ ] **Step 5: Remove only management E2E tests.**

Delete E2E files that exercise organization/team management endpoints or pages. Do not add replacement 404 tests, sidebar-absence tests, or new fixtures. Retain E2E tests for keys, models, credentials, access-control behavior, spend, and other supported flows.

- [ ] **Step 6: Commit the PR1 API/DB slice.**

Run:

```bash
git diff --name-status
# After reviewing the list, stage each changed path with: git add -- path/to/file
git commit -m "refactor(api): remove organization and team management endpoints"
```

Parent verification must confirm that the commit parent is the previously verified PR1 UI commit and that no schema/migration path changed.

## Task 4: Reconcile PR1 shared callers without inventing scope semantics

**Files:**
- Audit and modify only proven management-only callers: `internal/proxy/middleware/db_validator.go`, `internal/proxy/middleware/auth.go`, `internal/auth/jwt.go`, `internal/proxy/handler/key.go`, `internal/proxy/handler/key_ext.go`, `internal/proxy/handler/runtime_model_source.go`, `internal/proxy/handler/handler.go`, `internal/ui/handler_keys.go`, `internal/ui/handler_models.go`, `internal/router/router.go`, `internal/router/access_control.go`, `internal/spend/tracker.go`, `internal/callback/callback.go`, `cmd/tianji/main.go`
- Tests to update only when a retained contract changes: `internal/proxy/middleware/auth_test.go`, `internal/proxy/middleware/db_validator_test.go`, `internal/router/access_control_test.go`, `internal/config/access_control_test.go`, `test/integration/virtual_key_auth_test.go`, `test/e2e/api_access_control_test.go`, `test/e2e/models_access_control_test.go`

- [ ] **Step 1: Trace every live scope field and method.**

Run:

```bash
git grep -n -E 'AllowedOrgs|AllowedTeams|ContextKey(TeamID|OrgID)|TeamID|OrganizationID|ListTeams|ListOrganizations|GetTeam|GetOrganization|IsAllowed|AccessControl' -- internal/auth internal/config internal/proxy internal/router internal/spend internal/callback internal/ui test
```

For each match, record whether it serves a removed management endpoint, retained key/model/credential/auth/usage/spend behavior, or historical/schema representation.

- [ ] **Step 2: Apply the minimal retained-flow change.**

Remove only code whose sole purpose was to call the deleted organization/team management operations. Preserve context fields, JWT claims, spend attribution, error-log fields, credential organization scoping, retained model/key access control, and shared test coverage for non-management endpoints when they still have a caller.

If a legacy scoped key/model now requires a new decision between rejecting it and treating it as global, stop and record the exact caller and decision needed. Do not implement either fail-open or fail-closed by assumption.

- [ ] **Step 3: Use TDD for any changed retained behavior.**

Before changing a retained predicate or resolver, add one focused assertion to the existing package test, run its exact test to observe the failure, implement the minimum change, and rerun it. For pure removal with no retained behavior change, do not manufacture a new test.

Run the smallest relevant checks after each vertical slice:

```bash
go test -race ./internal/proxy/middleware/... ./internal/router/... ./internal/proxy/handler/... ./internal/ui/...
go test -race ./test/contract/... -run 'Auth|Access|Model|Key|Spend'
```

- [ ] **Step 4: Commit the shared-caller reconciliation.**

Run:

```bash
git diff --name-status
# After reviewing the list, stage each changed path with: git add -- path/to/file
git commit -m "refactor: detach retained flows from team organization management"
```

## Task 5: Verify and review PR1 before branching PR2

**Files:**
- Modify only files already owned by Tasks 2–4 when a verification finding is confirmed.
- Create review ledger entries under `.superpowers/sdd/remove-unused-admin-domains/`.

- [ ] **Step 1: Run source and route residual checks.**

Run:

```bash
git grep -n -E '/ui/(orgs|teams)|handle(Org|Team)|Permission(Teams|Organizations)Manage|/(organization|team)/(new|info|list|delete|update)' -- internal cmd test
```

Classify every match. Allowed matches are retained shared fields/callers explicitly recorded in the ledger and unchanged schema/history references. Unexplained management route/handler/navigation matches block PR1.

- [ ] **Step 2: Run PR1 focused verification.**

Run:

```bash
go test -race ./internal/ui/... ./internal/proxy/... ./internal/router/... ./internal/db/... ./test/contract/...
make lint
make build
git diff --check origin/main...HEAD
```

Run `make test` only when PostgreSQL is available locally; otherwise record the exact `localhost:5433` connection failure and leave the full DB-backed result to hosted CI.

- [ ] **Step 3: Parent-read the exact PR1 range.**

Run:

```bash
git rev-parse HEAD
git rev-parse origin/main
git status --short --branch
git diff --name-status origin/main...HEAD
git diff --check origin/main...HEAD
git show --stat --oneline HEAD
```

Generate an exact `origin/main..HEAD` review package and obtain two explicit reviewer verdicts: `spec_verdict` and `quality_verdict`, with findings and limitations. A timeout or summary-only report is `UNVERIFIED`.

- [ ] **Step 4: Fix PR1 findings through the same TDD/review gate.**

For each blocking/important finding, verify it against the current source and spec, make one bounded fix commit, rerun the smallest relevant test, then re-review the exact fix range. Any new commit invalidates earlier head-bound review and CI evidence.

- [ ] **Step 5: Push PR1 only after local gate.**

Run:

```bash
git push -u origin remove-unused-admin-domains/pr1
```

Create PR1 against `main` with exact `base_sha=origin/main` and current `head_sha`. Do not merge it. Keep the worktree for CI/review iteration.

## Task 6: Implement PR2 Access Groups on the verified PR1 head

**Files:**
- Modify: `internal/ui/pages/layout.templ`
- Delete after audit: `internal/ui/handler_access_groups.go`, `internal/ui/pages/access_groups.templ`, `internal/ui/pages/access_groups_helpers.go`, `internal/ui/pages/access_groups_templ.go`
- Delete management-only tests: `test/contract/accessgroup_test.go`, `test/integration/accessgroup_test.go`, `test/integration/accessgroup_batch_test.go`
- Modify: `internal/proxy/server.go`
- Delete after audit: `internal/proxy/handler/accessgroup.go`
- Modify: `internal/db/interface.go`
- Modify source only when unreferenced: `internal/db/queries/access_group.sql`
- Regenerate: `internal/db/access_group.sql.go` and dependent SQLC output through `make generate`
- Audit before modifying: `internal/a2a/permission.go`, `internal/a2a/registry.go`, `internal/proxy/handler/agent.go`, `internal/proxy/handler/key.go`, `internal/proxy/handler/key_ext.go`, `internal/db/queries/agent.sql`, `internal/db/models.go`, `internal/db/verification_token.sql.go`, `internal/db/spend_global.sql.go`
- Tests to reconcile: `internal/a2a/a2a_test.go`, `internal/proxy/handler/mock_store_test.go`, `test/contract/a2a_test.go`, `test/e2e/api_access_control_test.go`, `test/e2e/models_access_control_test.go`

- [ ] **Step 1: Create PR2 from the verified PR1 head.**

Read back PR1's remote head and create the next linked worktree/branch without rebasing or rewriting PR1:

```bash
git fetch origin remove-unused-admin-domains/pr1
git worktree add -b remove-unused-admin-domains/pr2 /root/.hermes/profiles/coder/worktrees/tianji-remove-unused-admin-domains-pr2 origin/remove-unused-admin-domains/pr1
```

Record `base_sha` as the exact PR1 head.

- [ ] **Step 2: Remove access-group UI/API ownership.**

Remove the access-group route blocks and the Access Groups navigation entry. Delete the management handler/template/helper and proxy handler only after exact caller search. Do not replace the route with an absence test or fallback endpoint.

- [ ] **Step 3: Remove access-group-only DB operations and A2A/key plumbing.**

Search:

```bash
git grep -n -E 'AccessGroup|agent_access_groups|ListAgentsByAccessGroups|access_group_ids|model_access_group' -- '*.go' '*.sql'
```

Delete query methods and fields only when their only consumer is the removed management feature. If agent or key behavior uses access groups as an active authorization contract, record that retained use and remove only management CRUD/filtering; do not broaden access by dropping a live predicate without an approved replacement.

- [ ] **Step 4: Regenerate and run focused tests.**

Run:

```bash
make generate
make ui
go test -race ./internal/a2a/... ./internal/db/... ./internal/proxy/handler/... ./internal/ui/... ./test/contract/...
git diff --check
```

- [ ] **Step 5: Delete only access-group management E2E coverage.**

Remove tests that exercise the deleted management page/API. Keep existing retained agent/key/model access-control E2E tests. Do not add URL-404 or sidebar-absence tests.

- [ ] **Step 6: Commit and exact-head review PR2.**

Run:

```bash
git diff --name-status
# After reviewing the list, stage each changed path with: git add -- path/to/file
git commit -m "refactor: remove access group management"
```

Parent-read the exact PR1-head-to-PR2-head range, run the Task 5 residual/targeted checks, obtain code review A and B, and fix/re-review findings before pushing PR2 against PR1's exact head.

## Task 7: Implement PR3 Guardrails and Policy on the verified PR2 head

**Files:**
- Modify: `internal/ui/pages/layout.templ`
- Delete after audit: `internal/ui/handler_guardrails.go`, `internal/ui/pages/guardrails.templ`, `internal/ui/pages/guardrails_templ.go`, `internal/ui/handler_guardrails_test.go`
- Modify: `internal/proxy/server.go`
- Delete after audit: `internal/proxy/handler/guardrail_mgmt.go`, `internal/proxy/handler/guardrail_mgmt_test.go`, `internal/proxy/handler/policy.go`
- Modify: `internal/db/interface.go`
- Modify source only when unreferenced: `internal/db/queries/guardrail_mgmt.sql`, `internal/db/queries/policy.sql`
- Regenerate: `internal/db/guardrail_mgmt.sql.go`, `internal/db/policy.sql.go`, and dependent SQLC output through `make generate`
- Audit and modify only dedicated runtime wiring: `cmd/tianji/main.go`, `internal/proxy/handler/handler.go`, `internal/proxy/handler/chat.go`, `internal/scheduler/jobs.go`, `internal/model/policy.go`, `internal/router/policy.go`, `internal/policy/*.go`, `internal/guardrail/*.go`, `internal/config/config.go`, `internal/config/validate.go`
- Preserve and test: provider-independent content-policy fallback in `internal/router/fallback.go` and its existing callers/tests
- Delete management-only tests: `test/contract/policy_test.go`, `test/integration/guardrail_test.go`, `test/integration/policy_integration_test.go`, and policy/guardrail management portions of `test/integration/router_test.go` only after caller audit

- [ ] **Step 1: Create PR3 from the verified PR2 head.**

Read back PR2's remote head and create the linked branch/worktree:

```bash
git fetch origin remove-unused-admin-domains/pr2
git worktree add -b remove-unused-admin-domains/pr3 /root/.hermes/profiles/coder/worktrees/tianji-remove-unused-admin-domains-pr3 origin/remove-unused-admin-domains/pr2
```

Record `base_sha` as the exact PR2 head.

- [ ] **Step 2: Trace the policy/guardrail request path before deletion.**

Run:

```bash
git grep -n -E 'Guardrails|PolicyEngine|PolicyEng|NewRegistry|NewEngine|PolicyHotReloadJob|Evaluate\(|RunPreCall|RunPostCall|/policy|/guardrails|ContentPolicyFallback' -- cmd internal test
```

Separate Tianji DB-backed policy/guardrail management/evaluation from provider-owned content-policy fallback. The latter remains; the former is removed only when no retained caller remains.

- [ ] **Step 3: Remove UI/API/DB management ownership.**

Remove guardrail UI routes/permission/navigation, `/policy` and `/guardrails` API route blocks, management handlers, and management-only Store/query methods. Delete the package code and generated queries only after the full caller map proves exclusive ownership. Preserve schema/migrations/data and required historical generated types.

- [ ] **Step 4: Remove dedicated initialization and chat evaluation.**

In `cmd/tianji/main.go`, remove imports and initialization for the deleted registries/engines and the scheduler hot-reload job only when the caller audit confirms they have no other consumer. In `internal/proxy/handler/handler.go` and `chat.go`, remove only deleted dependency fields and the Tianji policy/guardrail evaluation branch. Leave provider routing, `ContentPolicyFallback`, auth, budgets, caching, spend tracking, and streaming behavior intact.

- [ ] **Step 5: Use TDD for changed provider/chat fallback behavior.**

If removing the branch changes a retained chat error path, first add or adjust one focused test in `internal/proxy/handler` or `internal/router`, run it to observe the expected failure, implement the minimum change, and rerun it. If the code is pure deletion and existing provider fallback tests already cover the retained behavior, do not add a duplicate test.

Run:

```bash
make generate
make ui
go test -race ./internal/router/... ./internal/proxy/handler/... ./internal/db/... ./internal/config/... ./test/contract/...
git diff --check
```

- [ ] **Step 6: Delete only management E2E/integration tests and commit PR3.**

Do not add absence E2E. Commit:

```bash
git diff --name-status
# After reviewing the list, stage each changed path with: git add -- path/to/file
git commit -m "refactor: remove guardrail and policy management"
```

Parent-read the exact PR2-head-to-PR3-head range and confirm no schema/migration files changed.

## Task 8: Run the full requested review sequence on each exact PR head

**Files:**
- Review packages and ledger entries under `.superpowers/sdd/remove-unused-admin-domains/`.
- PR descriptions/comments through GitHub tooling only after the branch has been pushed.

- [ ] **Step 1: Code review A.**

Give a fresh reviewer only the exact PR base SHA, head SHA, spec, changed-path list, and review package. Require:

```text
spec_verdict: PASS|FAIL|UNVERIFIED
quality_verdict: PASS|FAIL|UNVERIFIED
findings: []
checks: ...
limitations: ...
```

Review correctness, shared-caller preservation, auth boundaries, error behavior, generated-file provenance, and secret safety.

- [ ] **Step 2: Code review B.**

Use a different fresh reviewer and the same exact range. Do not treat reviewer A's result as reviewer B's result. Fix every blocking/important finding, then rerun both reviews against the new head.

- [ ] **Step 3: Ponytail review.**

Run a full Ponytail review focused on YAGNI: every retained addition must have a current caller; every deleted file must be management-only; no new abstraction, compatibility layer, test fixture, or dependency was introduced. Findings that request extra 404/sidebar E2E are rejected as outside the approved scope and recorded as non-applicable.

- [ ] **Step 4: Requirement review.**

Check each acceptance criterion in the spec line by line: four UI routes/nav entries removed, exclusive runtime removed, shared retained behavior preserved, schema/data untouched, no unnecessary E2E added, generated output sourced correctly, and exact PR bases/heads recorded.

- [ ] **Step 5: E2E coverage assessment.**

Inspect the current implementation diff and existing tests. Verdict must be `sufficient for this fix` only if retained browser flows covered by the change remain represented. Explicitly record that the four removed URL/sidebar absence cases are intentionally not added and are covered by route/source residual audit instead. Do not claim a complete test matrix.

- [ ] **Step 6: Fix loop.**

For each finding:

```text
read complete finding → trace current callers → classify in-scope or non-applicable →
write failing focused test when retained behavior changes → implement minimum fix →
run focused check → read exact diff → re-review current exact head
```

Do not close a finding from a reviewer summary alone. A new commit invalidates prior review approvals and CI results.

## Task 9: Exact-head local and hosted verification

**Files:**
- No new source files.
- Verification ledger only under `.superpowers/sdd/remove-unused-admin-domains/`.

- [ ] **Step 1: Run local static and package checks on PR3.**

Run:

```bash
make ui
make generate
# Run gofmt only on the modified Go paths printed by: git diff --name-only -- '*.go'
git diff --check origin/remove-unused-admin-domains/pr2...HEAD
go test -race ./internal/... ./test/contract/...
make lint
make build
```

Do not report `make test` as green unless its DB-backed command exits zero with PostgreSQL available. If local PostgreSQL remains unavailable, record the exact failure and rely on the hosted test job for that gate.

- [ ] **Step 2: Run existing E2E only when its database/browser service is available.**

Run the repository command without adding tests:

```bash
make e2e
```

If the local service is unavailable, record E2E as not run locally; the hosted CI E2E job is authoritative. Never turn a skipped/unavailable run into PASS.

- [ ] **Step 3: Run the final residual audit.**

Run:

```bash
git grep -n -E '/ui/(access-groups|guardrails|orgs|teams)|Permission(AccessGroups|Guardrails|Organizations|Teams)Manage|handle(AccessGroups|Guardrails|Org|Team)|/(model_access_group|policy|guardrails|organization|team)' -- cmd internal test
```

Use the route source files and the full current matches to classify retained shared fields, unchanged schema/history, required generated output, or unexplained residuals. The final verdict requires zero unexplained removed-management route/nav/handler/service matches.

- [ ] **Step 4: Read hosted CI by exact SHA.**

After pushing each PR, read PR metadata and checks, then require every applicable CI job (`lint`, `test`, `codex-compatibility`, `e2e`, `build`) to be terminal-successful with `head_sha` equal to the current PR head. For failures, read the raw failed job log, fix the root cause without lowering gates, push a new commit, and repeat all exact-head reviews/checks.

## Task 10: Create and maintain the stacked PRs

- [ ] **Step 1: Create PR1, PR2, and PR3 with exact bases.**

Use `gh` only after authentication is verified without printing credentials:

```bash
gh pr create --base main --head remove-unused-admin-domains/pr1 --title "refactor: remove organization and team admin domains"
gh pr create --base remove-unused-admin-domains/pr1 --head remove-unused-admin-domains/pr2 --title "refactor: remove access group management"
gh pr create --base remove-unused-admin-domains/pr2 --head remove-unused-admin-domains/pr3 --title "refactor: remove guardrail and policy management"
```

If the forge rejects a stacked base, keep the PR branch and report the exact provider error rather than force-pushing or changing the base implicitly.

- [ ] **Step 2: Update each PR description with evidence.**

Include the spec path, exact base/head SHAs, changed-path scope, generated-command provenance, local test limitations, review ledger verdicts, and the explicit statement that no 404/sidebar absence E2E was added. Never include tokens, passwords, connection strings, or raw environment values.

- [ ] **Step 3: Fix CI failures until exact final head is green.**

Use `gh pr checks <number>` and `gh run view <run-id> --log-failed` for diagnosis. Fix source/test/generation issues in the task worktree, run focused checks, commit, push, and rerun the full review sequence for the new head. Do not modify `.github/workflows/ci.yml` merely to suppress a failure.

- [ ] **Step 4: Stop before merge.**

The requested deliverable is PRs with clean reviews and green exact-head CI. Do not merge or deploy without a separate explicit user gate.

## Final handoff checklist

- [ ] PR1/PR2/PR3 URLs, repositories, bases, heads, and branch ancestry read back from GitHub.
- [ ] All review seats have exact-head `PASS`/clean verdicts with no unresolved blocking or important findings.
- [ ] Existing E2E coverage assessment is recorded as sufficient or as a concrete blocker; no unnecessary absence tests were added.
- [ ] `make lint`, full hosted test, hosted E2E, build, and applicable checks are terminal green for the exact final heads.
- [ ] Final `git grep` residual audit has no unexplained removed-domain management route/nav/handler/service matches.
- [ ] Schema/migration/data files are unchanged and no production/external mutation was performed.
- [ ] Task-owned worktrees, branches, processes, and artifacts are inventory-clean only when cleanup is explicitly requested; preserve the PR worktrees by default for follow-up review.
