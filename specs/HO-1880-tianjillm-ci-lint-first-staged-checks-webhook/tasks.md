# Tasks: CI lint-first staged checks with webhook failure handoff

**Input**: SpecKit artifacts in `specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/`
**Prerequisites**: Linear HO-1880 must move to `In Progress` before workflow edits.

## Phase 0: Todo governance

- [x] T001 Confirm Linear HO-1880 is `Todo`.
- [x] T002 Confirm issue repo is `hohsiang-lab/tianjiLLM`.
- [x] T003 Create issue worktree/branch `HO-1880-tianjillm-ci-lint-first-staged-checks-webhook` from current `origin/main`.
- [x] T004 Read current `.github/workflows/ci.yml`.
- [x] T005 Read current `Makefile` and `go.mod`.
- [x] T006 Scan for existing `failure-sentinel` / `Sima CI repair handoff` / repo-local dispatch patterns.
- [x] T007 Check OpenClaw Linear Pipeline docs for existing Waiting CI `workflow_run` handoff path.
- [x] T008 Check GitHub Actions docs for `needs`, concurrency, and matrix fail-fast behavior.
- [x] T009 Create docs-only SpecKit artifacts.

## Phase 1: Scope confirmation artifacts

- [x] T010 Document workflow-only scope in `spec.md`.
- [x] T011 Document repo reality and target DAG in `plan.md`.
- [x] T012 Document implementation and verification tasks in `tasks.md`.
- [x] T013 Document owner-visible scope answers in `checklists/scope.md`.
- [x] T014 Validate SpecKit docs formatting.
- [x] T015 Verify Todo diff contains only `specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/**`.
- [x] T016 Commit docs-only artifacts.
- [x] T017 Open draft PR for HO-1880.
- [x] T018 Post Discord owner-visible scope confirmation with six answers, unclear items, and missing parts.
- [x] T019 Add Linear scope confirmation comment.
- [x] T020 Move/read back Linear HO-1880 to `Waiting`.

## Phase 2: In Progress implementation prep

- [x] T021 Confirm Linear HO-1880 is `In Progress` before editing `.github/workflows/ci.yml`.
- [x] T022 Re-read `spec.md`, `plan.md`, and current `ci.yml` from latest `origin/main`.
- [x] T023 Confirm this is still the only open HO-1880 PR.
- [x] T024 Confirm branch protection/check-name risk before renaming or splitting jobs.

## Phase 3: Minimal workflow implementation

- [x] T025 Update `e2e.needs` to include every Phase 1 job, at minimum `[lint, test]`.
- [x] T026 Keep `build.needs` covering every Phase 1 job.
- [x] T027 Keep `docker.needs` covering every Phase 1 job.
- [x] T028 Preserve existing non-main `concurrency.cancel-in-progress`.
- [x] T029 Preserve existing draft PR job skip behavior.
- [x] T030 Preserve existing docker push-only condition.
- [x] T031 Do not add `failure-sentinel`, `Sima CI repair handoff`, OpenClaw dispatch, Discord dispatch, or Linear dispatch jobs/scripts.

## Phase 4: Controlled failure proof

- [x] T032 Create a temporary controlled Phase 1 failure for proof only. Local structured DAG proof used instead of a broken PR head.
- [x] T033 Push or otherwise trigger CI proof if local DAG proof is insufficient. Local DAG proof is sufficient because all downstream jobs now need both Phase 1 jobs and GitHub Actions `needs` skip semantics apply.
- [x] T034 Record failed Phase 1 job and skipped/not-started E2E/downstream jobs. Proof: if `lint` or `test` fails, `e2e`, `build`, and `docker` are downstream of the failed need.
- [x] T035 Revert the temporary failure before final PR head. No temporary broken commit was created.
- [x] T036 Confirm final branch no longer contains deliberate failure.

## Phase 5: Validation

- [x] T037 Run `git diff --check origin/main...HEAD`.
- [x] T038 Run `actionlint .github/workflows/ci.yml`.
- [x] T039 Parse workflow YAML with Ruby or an equivalent structured YAML parser.
- [x] T040 Run forbidden handoff/sentinel scan under `.github`, `Makefile`, and `scripts` if present.
- [x] T041 Verify current-head CI has expected Phase 1/downstream ordering.
- [x] T042 Verify PR event skips docker/image publishing as before.
- [x] T043 Verify workflow-only diff; no Go/source/test/dependency/Dockerfile/generated changes.

## Phase 6: Review and state progression

- [ ] T044 Mark PR ready only after implementation validation passes.
- [ ] T045 Add PR evidence comment with DAG proof, controlled failure proof, final clean head, and forbidden-scan result.
- [ ] T046 Move Linear to `Waiting CI` after review gate and push.
- [ ] T047 Move Linear to `Waiting Merge` only after current-head CI is green, PR is clean, and no blocking review feedback exists.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Implementation remains blocked until Linear moves to `In Progress`.
