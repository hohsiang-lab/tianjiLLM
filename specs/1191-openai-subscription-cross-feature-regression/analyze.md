# Analyze: HO-1191 Todo Planning

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Scope Alignment

| Source | Requirement | Artifact Coverage |
| --- | --- | --- |
| HO-1191 Goal | Ensure child acceptance paths are covered by unit, integration, and UI E2E tests. | `spec.md` FR-001..FR-015, `tasks.md` T001-T019 |
| HO-1191 Scope | PKCE, callback, refresh, routing, quota, redaction, API lifecycle, UI actions, Models page selection. | Matrix groups in `data-model.md`; task phases map each area. |
| HO-1191 Scope | Add existing `api_key` path regression. | `spec.md` FR-012, `tasks.md` T008/T013 |
| HO-1191 Scope | Add config validation for subscription IDs + custom `api_base`. | `spec.md` FR-013, `tasks.md` T008/T013 |
| HO-1191 Scope | UI E2E covers Connect/Test/Refresh/Disable/Delete and multi-credential selection. | `spec.md` US3/FR-011, `tasks.md` T012/T013/T016 |
| HO-1173 / HO-1165 | CI must not call real OpenAI; use `httptest.NewServer`; no WireMock. | `spec.md` FR-014, `plan.md` mock strategy, `tasks.md` T009/T014 |

## Repo Reality Check

- Existing test files for most matrix rows already exist under `internal/config`, `internal/proxy/handler`, `internal/callback`, `internal/spend`, `internal/testutil/openaitest`, and `test/e2e`.
- HO-1172 Models page selection tests are present in `test/e2e/models_openai_subscription_test.go`.
- The implementation phase should begin by filling the matrix and only then add missing tests.

## Risk Review

- **Fatal 0**: No artifact tells implementation to write production code during Todo; no real OpenAI dependency is planned.
- **Critical 0**: Matrix-first tasks prevent claiming completion from test names alone; existing `api_key` and custom `api_base` regressions are explicit required rows.
- **Owner input 0**: Linear scope is concrete enough; no business decision is needed before Waiting.

## E2E / CI Coverage Map

- PR layer: QA matrix + matrix-driven tests.
- Runtime boundary: OpenAI OAuth/token/upstream mocked with local servers; UI flows use Playwright-Go against local `httptest.Server`.
- Test command: targeted backend/API, integration, targeted UI E2E, full CI lint/test/e2e/build.
- CI evidence: to be filled during implementation after Linear moves to In Progress.
- Gap/follow-up: HO-1192 owns operator docs, not this PR.

## Implementation Update - 2026-05-09

### Gap Closure

- Existing `api_key` path regression is now direct UI E2E coverage: `TestModelCreate_WithOptionalFields` and `TestModelEdit_APIKeyPreservation` assert persisted `tianji_params` keeps `api_key` and has no `openai_subscription_credential_ids`.
- The matrix row for 401 refresh retry / failover now points to the existing direct endpoint tests in `internal/proxy/handler/openai_subscription_endpoints_test.go`, including same-credential retry success, refresh-failure failover, retry-still-401 disable/failover, all-unusable reauth error, and API-key path non-refresh behavior.

### Local Gate Evidence

- Passed: `go test ./internal/config ./internal/proxy ./internal/proxy/handler ./internal/callback ./internal/spend ./internal/testutil/openaitest`.
- Passed: `go test -c -tags e2e -o /tmp/ho1191-e2e.test ./test/e2e`.
- Passed: `make lint` with `0 issues`.
- Passed: CI-equivalent build command after installing CI-pinned `templ@v0.3.977`: `templ generate ./internal/ui/... && go build -o bin/tianji ./cmd/tianji`.
- Passed: `git diff --check`.
- Local-only limitation: `go test ./test/integration/...` failed because local Postgres on `localhost:5433` was unavailable. CI `test` job starts the required Postgres service and remains the final DB-backed integration gate.

### Final Risk

- Fatal: 0
- Critical: 0
- Owner input required: 0
- Residual risk: Waiting Merge still depends on PR CI `lint`, `test`, `e2e`, and `build` passing on the pushed head.
