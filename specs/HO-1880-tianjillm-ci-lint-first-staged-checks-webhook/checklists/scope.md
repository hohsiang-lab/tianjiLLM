# Scope Checklist: CI lint-first staged checks with webhook failure handoff

## Six Scope Answers

- [x] **Target repo**: `hohsiang-lab/tianjiLLM`.
- [x] **Allowed files during Todo**: only `specs/HO-1880-tianjillm-ci-lint-first-staged-checks-webhook/**`.
- [x] **Implementation surface after In Progress**: `.github/workflows/ci.yml` only unless implementation-time evidence proves a docs-only support file is required.
- [x] **Phase 1 checks**: existing `lint` and `test` at minimum; any added setup/static validation job must also be Phase 1.
- [x] **Downstream checks**: `e2e`, `build`, and `docker` must require all Phase 1 jobs.
- [x] **Failure handoff**: existing GitHub Actions `workflow_run` webhook to OpenClaw/Sima; no repo-local sentinel/handoff job.

## Explicit Non-Scope

- [x] No Go application code.
- [x] No Go tests.
- [x] No migrations or database schema.
- [x] No Dockerfile or image tag changes.
- [x] No dependency or Go version changes.
- [x] No runner label or secret changes.
- [x] No OpenClaw plugin or Linear pipeline code changes.
- [x] No branch protection/settings changes.
- [x] No React Doctor addition.

## Unclear Items

- [x] Branch protection required checks are not readable from local repo evidence; implementation should avoid renaming existing jobs unless GitHub settings evidence is obtained.
- [x] Controlled Phase 1 failure proof method is intentionally deferred to `In Progress`; final PR head must be clean.

## Missing Parts To Surface Before Implementation

- [x] Exact controlled failure proof run ID cannot exist during Todo because workflow code changes are blocked.
- [x] Final proof that OpenClaw receives a failed current-head workflow event is outside repo scope; existing pipeline route evidence is enough for Todo, while live handoff remains Waiting CI behavior.

## Todo Gate

- [x] Spec created.
- [x] Plan created.
- [x] Tasks created.
- [x] Scope checklist created.
- [x] Docs formatting passed.
- [x] Docs-only diff confirmed.
- [x] Draft PR opened.
- [x] Discord owner-visible scope report posted.
- [x] Linear scope report posted.
- [x] Linear moved/read back to `Waiting`.
