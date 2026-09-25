# Implementation Plan: CI lint-first staged checks with webhook failure handoff

**Branch**: `HO-1880-tianjillm-ci-lint-first-staged-checks-webhook`
**Spec**: `specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/spec.md`
**Linear**: HO-1880
**Phase**: Todo planning only; workflow implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Repo: `hohsiang-lab/tianjiLLM`.
- Runtime: Go module `github.com/praxisllmlab/tianjiLLM`, Go `1.26.0`.
- Workflow file: `.github/workflows/ci.yml`.
- Existing CI jobs: `lint`, `test`, `e2e`, `build`, `docker`.
- Existing local commands: `make lint`, `make test`, `make build`, `make e2e`.
- Existing E2E command in CI: `go test -tags e2e -count=1 -v -timeout 5m ./test/e2e/...`.

## Governance

- Linear HO-1880 is `Todo`; only SpecKit docs are allowed in this phase.
- Do not edit `.github/workflows/ci.yml`, Makefile, Go code, tests, Dockerfile, generated files, or dependencies until Linear moves to `In Progress`.
- The implementation PR must stay single-issue and single-repo.
- No repo-local handoff/sentinel mechanism is allowed.

## Repo Reality

- `.github/workflows/ci.yml` already has non-main stale-run cancellation:
  - `group: ${{ github.workflow }}-${{ github.ref }}`
  - `cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}`
- The workflow already skips jobs for draft pull requests through job-level `if`.
- `lint` runs `golangci/golangci-lint-action@v9`.
- `test` starts Postgres, applies SQL migrations best-effort, and runs race/coverage tests with a 50% coverage threshold.
- `e2e` runs in a Playwright container with Postgres and currently needs only `[test]`.
- `build` installs pinned `templ@v0.3.977`, generates templ, and builds `./cmd/tianji`; it currently needs `[lint, test]`.
- `docker` builds/pushes GHCR images only on push and currently needs `[lint, test]`.
- There is no matrix job in current CI, so no `fail-fast: false` is present.
- Repo scan found no `failure-sentinel`, `Sima CI repair handoff`, or repo-local OpenClaw dispatch job/script in `.github`, `Makefile`, or current workflow.

## External Evidence

- GitHub Actions `jobs.<job_id>.needs` is the native mechanism for preventing downstream jobs after a dependency fails or is skipped.
- GitHub Actions concurrency supports `cancel-in-progress` expressions and is already configured here for non-main refs.
- GitHub Actions matrix `fail-fast` defaults to true; this repo has no matrix jobs today.
- OpenClaw Linear Pipeline documentation records `github-ci-sima-dispatch-route` as the Waiting CI `workflow_run/completed` path, with cancelled workflow runs treated as skipped/no-op and success/failure actionable.

## Target DAG

Preferred minimal implementation after `In Progress`:

```yaml
jobs:
  lint:
    # existing lint job

  test:
    # existing unit/coverage job

  e2e:
    needs: [lint, test]
    # existing E2E job

  build:
    needs: [lint, test]
    # existing build job

  docker:
    needs: [lint, test]
    if: github.event_name == 'push'
    # existing docker job
```

This is intentionally conservative: only `e2e` is currently under-gated. `build` and `docker` already need both existing Phase 1 jobs.

Optional implementation if repo evidence shows setup/static validation should become first-class:

```yaml
jobs:
  lint:
  test:
  ci-setup:
  static-build-check:

  e2e:
    needs: [lint, test, ci-setup, static-build-check]

  build:
    needs: [lint, test, ci-setup, static-build-check]

  docker:
    needs: [lint, test, ci-setup, static-build-check]
```

Do not add optional jobs unless implementation-time evidence proves real value. Extra jobs increase CI complexity and can create noisy branch-protection names.

## Controlled Failure Proof

During implementation only:

1. Create a temporary branch/head that makes Phase 1 fail in a reversible way, preferably lint or a YAML-controlled check.
2. Push only if needed to obtain GitHub Actions DAG evidence.
3. Verify E2E/build/docker are skipped or not started.
4. Revert the temporary failure before final PR head.
5. Record run ID, failed Phase 1 job, skipped downstream jobs, and final clean head in the PR/Linear evidence.

The final PR head must not contain a deliberately failing commit.

## Validation Plan

Todo docs-only validation:

```bash
npx --yes prettier@3.7.1 --no-config --check \
  specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/*.md \
  specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/checklists/*.md
git diff --check origin/main...HEAD
```

Implementation validation after `In Progress`:

```bash
git diff --check origin/main...HEAD
actionlint .github/workflows/ci.yml
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/ci.yml"); puts "yaml ok"'
rg -n "failure-sentinel|Sima CI repair handoff|OpenClaw|github-ci-sima-dispatch|workflow_run" .github Makefile scripts || true
```

CI-owned validation after push:

- Current-head CI run starts from the final clean PR head.
- `lint`, `test`, and any added Phase 1 jobs pass.
- `e2e` and `build` run only after Phase 1 passes.
- `docker` remains push-only and skipped for PR events.
- A controlled Phase 1 failure proof exists from an earlier non-final head or controlled validation run.

## Risk Register

| Risk                                              | Mitigation                                                                                             |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| E2E still runs when lint fails                    | Add `lint` to `e2e.needs`; prove through static DAG and controlled failure evidence.                   |
| Branch protection check names change unexpectedly | Prefer editing existing job dependencies over renaming jobs or splitting jobs.                         |
| Repo accidentally duplicates OpenClaw handoff     | Scan workflow/scripts for sentinel, handoff, OpenClaw, Gateway, Discord, and Linear dispatch terms.    |
| Final PR head stays broken after proof            | Revert controlled failure before final push; final current-head CI must be green before Waiting Merge. |
| Main branch publish behavior changes              | Keep docker `if: github.event_name == 'push'` and main cancellation exemption intact.                  |
| Draft PRs start expensive jobs                    | Preserve existing draft-PR job-level `if` expressions unless issue scope explicitly changes.           |

## Todo Gate Status

Spec/plan/tasks/checklist can proceed to a docs-only draft PR and Waiting state. Workflow implementation remains blocked until Linear `In Progress`.
