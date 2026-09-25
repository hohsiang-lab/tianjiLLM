# Tasks: OpenAI Subscription Cross-Feature Regression Matrix

**Input**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `quickstart.md`

## Phase 1 - Matrix Baseline

- [x] T001 Create `contracts/openai-subscription-regression-matrix.md` with all required matrix groups and initial rows from HO-1191 / HO-1173 / HO-1165.
- [x] T002 Map existing config/API-key tests from `internal/config/*_test.go` and mark coverage for subscription IDs, custom `api_base`, omitted IDs, and existing API-key behavior.
- [x] T003 Map existing OAuth/connect/callback tests from `internal/config/openai_oauth_test.go`, `internal/proxy/openai_oauth_routes_test.go`, and `test/e2e/openai_connect_flow_test.go`.
- [x] T004 Map existing refresh/routing/failover tests from `internal/proxy/handler/openai_subscription_*_test.go`.
- [x] T005 Map existing quota/spend/audit/redaction tests from `internal/callback`, `internal/spend`, and handler lifecycle tests.
- [x] T006 Map existing credential lifecycle and UI action tests from `internal/proxy/handler/openai_subscription_lifecycle_test.go` and `test/e2e/credentials_test.go`.
- [x] T007 Map existing Models page selection coverage from `test/e2e/models_openai_subscription_test.go` and related model create/edit tests.

## Phase 2 - Gap Closure Tests

- [x] T008 Add or strengthen config/API-key regression tests for omitted subscription IDs, explicit empty IDs, custom provider/custom `api_base`, and `$OPENAI_API_KEY` interpolation where matrix rows are partial.
- [x] T009 Add or strengthen OAuth/callback tests for PKCE verifier, state one-time consume, callback success/failure redaction, and no-real-OpenAI guard where matrix rows are partial.
- [x] T010 Add or strengthen refresh/routing tests for refresh rotation, per-credential lock, 401 retry then failover, disabled exclusion, all-unusable error, and no API-key fallback where matrix rows are partial.
- [x] T011 Add or strengthen quota/spend/audit/redaction tests for OpenAI quota headers, store consumption, lifecycle audit metadata, and secret redaction where matrix rows are partial.
- [x] T012 Add or strengthen credential API/UI lifecycle tests for Test/Refresh/Disable/Delete, idempotency, confirm-cancel, and redacted failure toasts where matrix rows are partial.
- [x] T013 Add or strengthen Models page E2E for create/edit multi-select, safe metadata rendering, custom `api_base` rejection, DB round-trip, and existing API-key create/edit regression where matrix rows are partial.

## Phase 3 - Verification

- [x] T014 Run targeted backend/API tests: `go test ./internal/config ./internal/proxy ./internal/proxy/handler ./internal/callback ./internal/spend ./internal/testutil/openaitest`.
- [x] T015 Run integration tests: `go test ./test/integration/...`; local attempt failed because local Postgres on `localhost:5433` was unavailable, so the CI `test` job with Postgres service is the final DB-backed gate.
- [x] T016 Run targeted UI E2E compile gate: `go test -c -tags e2e -o /tmp/ho1191-e2e.test ./test/e2e`; full UI E2E is owned by CI per Sima implementation policy.
- [x] T017 Run full project gates or record CI equivalent: `make lint` and CI-equivalent `templ generate ./internal/ui/... && go build -o bin/tianji ./cmd/tianji`; PR CI lint/test/e2e/build is the final Waiting Merge gate.
- [x] T018 Update matrix rows with final status and CI evidence path.
- [x] T019 Update `analyze.md` with fatal/critical count, E2E coverage map, and residual risk.

## Phase 4 - PR / State Gate

- [ ] T020 Verify PR diff contains only HO-1191 artifacts and matrix-driven tests.
- [ ] T021 Push branch and update draft PR body with matrix summary.
- [ ] T022 Move Linear to In Review only after implementation tasks are complete and local gates pass.
