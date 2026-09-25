# Feature Specification: CI lint-first staged checks with webhook failure handoff

**Branch**: `HO-1880-tianjillm-ci-lint-first-staged-checks-webhook`
**Linear**: HO-1880
**Created**: 2026-06-10
**Phase**: Todo planning only; workflow implementation starts only after Linear moves to `In Progress`.

## Summary

tianjiLLM's GitHub Actions CI already cancels stale non-main runs, but expensive downstream jobs are not fully blocked by all Phase 1 checks. HO-1880 defines a workflow-only change that makes lint/static checks, dependency/setup validation, and unit coverage finish before E2E, build, and image publishing can start. CI failure handoff must remain the existing GitHub Actions webhook to OpenClaw/Sima path, with no repo-local sentinel or repair handoff job.

## Scope

In scope:

- Update `.github/workflows/ci.yml` DAG so expensive downstream jobs require every Phase 1 gate.
- Keep stale PR/branch run cancellation through workflow concurrency.
- Treat `lint`, dependency/setup validation, static/build generation checks, and unit coverage as Phase 1.
- Treat E2E, production binary build, and GHCR image build/push as downstream/expensive.
- Prove a controlled Phase 1 failure skips or does not start E2E/downstream jobs.
- Preserve webhook-based CI failure handoff through OpenClaw's existing `workflow_run` pipeline.
- Keep exactly one GitHub PR for HO-1880.

Out of scope:

- Adding `failure-sentinel`, `Sima CI repair handoff`, repository-local OpenClaw dispatch, or webhook-calling jobs/scripts.
- Changing Go application code, tests, database schema, migrations, UI, Dockerfile, or dependency versions.
- Changing GitHub branch protection, repository settings, OpenClaw gateway/plugin code, or Linear pipeline code.
- Adding React Doctor; tianjiLLM is a Go/templ backend repo and has no current React Doctor workflow.
- Changing CI runner labels or secrets.

## User Stories and Tests

### User Story 1 - Fast failure blocks expensive work (P1)

As a maintainer, I need lint/static/setup/unit failures to stop E2E, build, and image-push work, so broken PR heads fail quickly without spending runner time on expensive jobs.

**Independent Test**: Introduce a controlled temporary Phase 1 failure during implementation validation and verify E2E/downstream jobs are skipped or never start, then remove the failure before final PR head.

**Acceptance Scenarios**:

1. Given `lint` fails, when the CI DAG evaluates downstream jobs, then E2E, build, and docker do not run.
2. Given `test` fails, when the CI DAG evaluates downstream jobs, then E2E, build, and docker do not run.
3. Given all Phase 1 jobs pass, when the workflow reaches downstream jobs, then E2E/build/docker may run according to their existing event conditions.

### User Story 2 - Stale PR work is cancelled (P1)

As a maintainer, I need older commits on the same PR/branch to cancel when a newer commit arrives, so Sima and reviewers focus on the latest head only.

**Independent Test**: Read the workflow and current PR run behavior to confirm non-main `cancel-in-progress` remains enabled.

**Acceptance Scenarios**:

1. Given a new commit is pushed to the same PR branch, when GitHub starts the new run, then the prior non-main run is eligible for cancellation.
2. Given the run is on `main`, when CI starts, then main branch runs are not cancelled by the workflow expression.

### User Story 3 - Failure handoff remains webhook-based (P1)

As an OpenClaw operator, I need CI failures to continue through the existing GitHub webhook to OpenClaw/Sima path, so the repo does not grow duplicated repair orchestration.

**Independent Test**: Scan workflow, scripts, and package files to prove no `failure-sentinel`, `Sima CI repair handoff`, or repo-local OpenClaw dispatch was added.

**Acceptance Scenarios**:

1. Given a current-head CI run fails while the Linear issue is in `Waiting CI`, when GitHub sends a `workflow_run completed` webhook, then OpenClaw's existing `github-ci-sima-dispatch` route owns the Sima handoff.
2. Given workflow YAML is inspected, then there is no repo-local sentinel/handoff job.
3. Given the PR diff is inspected, then no OpenClaw webhook token or API endpoint is introduced into tianjiLLM.

## Functional Requirements

- **FR-001**: The final CI DAG MUST make E2E depend on every Phase 1 job, not just `test`.
- **FR-002**: The final CI DAG MUST make production build depend on every Phase 1 job.
- **FR-003**: The final CI DAG MUST make docker/image publishing depend on every Phase 1 job and keep existing push-only publishing behavior.
- **FR-004**: Phase 1 MUST include `lint` and `test` at minimum because current CI already has those jobs.
- **FR-005**: If implementation splits dependency/setup/static validation into separate jobs, downstream jobs MUST also need those jobs.
- **FR-006**: Non-main stale run cancellation MUST remain enabled through `concurrency.cancel-in-progress`.
- **FR-007**: Main branch runs MUST remain protected from cancellation by the workflow expression.
- **FR-008**: The workflow MUST NOT include a job, script, or step named or functioning as `failure-sentinel` or `Sima CI repair handoff`.
- **FR-009**: The repo MUST NOT call OpenClaw/Gateway/Discord/Linear directly from CI for this issue.
- **FR-010**: A controlled Phase 1 failure proof MUST be recorded before final handoff, and the final PR head MUST be clean.
- **FR-011**: Todo phase MUST remain docs-only under `specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/**`.
- **FR-012**: Implementation MUST preserve draft-PR skip behavior for draft pull requests unless explicit repo evidence proves it blocks the issue's goal.
- **FR-013**: CI YAML syntax MUST be validated before pushing implementation.
- **FR-014**: The PR MUST remain the only open HO-1880 PR for `hohsiang-lab/tianjiLLM`.

## Edge Cases

- Draft pull request: current jobs are skipped through job-level `if`; implementation must not accidentally run expensive jobs for drafts.
- Push to `main`: docker/image publishing remains allowed and should not be cancelled by stale-run concurrency.
- Pull request: docker/image publishing remains skipped by the existing push-only condition.
- Lint fails quickly while test is still queued/running: downstream jobs must still wait for all Phase 1 needs and skip once any required job fails.
- Test fails after lint passes: E2E/build/docker must still skip.
- Workflow run is cancelled by concurrency: OpenClaw pipeline behavior for cancelled runs is owned outside this repo.

## Success Criteria

- **SC-001**: Static DAG proof shows E2E needs every Phase 1 job.
- **SC-002**: Static DAG proof shows build and docker need every Phase 1 job.
- **SC-003**: Controlled Phase 1 failure evidence shows E2E/downstream skipped or not started.
- **SC-004**: Final branch scan finds no `failure-sentinel`, `Sima CI repair handoff`, or repo-local OpenClaw dispatch.
- **SC-005**: Final branch validates workflow YAML and has no non-doc changes during Todo.
- **SC-006**: Draft PR is open for HO-1880 and contains only docs during Todo.

## Repo Evidence

- Current `.github/workflows/ci.yml` has one workflow named `CI`.
- Current concurrency is `group: ${{ github.workflow }}-${{ github.ref }}` with `cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}`.
- Current jobs are `lint`, `test`, `e2e`, `build`, and `docker`.
- Current `e2e` needs only `[test]`, so lint failure does not explicitly gate E2E.
- Current `build` and `docker` need `[lint, test]`.
- Current `docker` is push-only through `if: github.event_name == 'push'`.
- Current repo scan found no repo-local `failure-sentinel` or `Sima CI repair handoff` workflow job/script.
- OpenClaw Linear Pipeline docs record `github-ci-sima-dispatch-route` as the existing `workflow_run/completed` route for Linear issues in `Waiting CI`.

## External Evidence

- GitHub Actions workflow syntax documents that jobs using `needs` are skipped when a needed job fails or is skipped, unless an override expression is used.
- GitHub Actions matrix strategy documents `fail-fast` as enabled by default unless set otherwise.
- This repo's current workflow has no matrix jobs, so the matrix `fail-fast` requirement is satisfied by avoiding an explicit `fail-fast: false`.

## State Gate

Todo planning may create SpecKit artifacts and a docs-only draft PR. Workflow YAML changes, controlled failure commits, and CI proof are blocked until Linear HO-1880 moves to `In Progress`.
