# Implementation Plan: OpenAI Subscription Cross-Feature Regression Matrix

**Branch**: `HO-1191-openai-subscription-cross-feature-regression`
**Spec**: `specs/1191-openai-subscription-cross-feature-regression/spec.md`

## Technical Approach

Use the existing TianjiLLM test stack instead of introducing a new QA framework:

- Matrix artifact: commit `contracts/openai-subscription-regression-matrix.md` under this spec directory.
- Backend/API tests: Go `_test.go` files under `internal/...` and `test/integration/...`.
- UI E2E tests: existing Playwright-Go suite under `test/e2e/...`.
- Mocking: reuse `internal/testutil/openaitest` and `httptest.NewServer()`; keep the no-real-OpenAI guard active for OAuth/upstream paths.
- Validation: run targeted OpenAI subscription tests locally before full CI; CI remains the final matrix representation.

This issue is QA/test scope. Production code edits are allowed only if a test exposes that a required acceptance path is untestable or broken; otherwise implementation should be test/artifact-only.

## Repo Reality

- Existing matrix-relevant unit/API tests already include:
  - `internal/config/loader_test.go`
  - `internal/config/validate_test.go`
  - `internal/config/openai_oauth_test.go`
  - `internal/proxy/handler/openai_subscription_resolution_test.go`
  - `internal/proxy/handler/openai_subscription_refresh_test.go`
  - `internal/proxy/handler/openai_subscription_routing_test.go`
  - `internal/proxy/handler/openai_subscription_ratelimit_test.go`
  - `internal/proxy/handler/openai_subscription_lifecycle_test.go`
  - `internal/proxy/handler/openai_subscription_endpoints_test.go`
  - `internal/proxy/handler/openai_subscription_attribution_test.go`
  - `internal/callback/openai_quota_store_test.go`
  - `internal/callback/openai_subscription_attribution_test.go`
  - `internal/spend/openai_subscription_attribution_test.go`
  - `internal/testutil/openaitest/*_test.go`
- Existing matrix-relevant UI E2E tests already include:
  - `test/e2e/openai_connect_flow_test.go`
  - `test/e2e/credentials_test.go`
  - `test/e2e/models_openai_subscription_test.go`
  - `test/e2e/models_create_test.go`
  - `test/e2e/models_edit_test.go`
- `Makefile` exposes `test`, `lint`, `build`, and `e2e`; E2E uses Go build tag `e2e` and `test/e2e/...`.
- HO-1172 has landed on `origin/main` and adds Models page multi OpenAI subscription credential selection tests.

## External / Prior-Art Review

- Context7 lookup for Go docs resolved the Go API docs and confirmed `net/http/httptest` provides `NewServer`, `NewTLSServer`, `NewRecorder`, and server `Close()` semantics suitable for local HTTP tests.
- Official Go docs found through web search:
  - `go.dev/pkg/net/http/httptest` documents `httptest.NewServer` as a local test HTTP server.
  - `go.dev/blog/subtests` and `go.dev/wiki/TableDrivenTests` support table-driven subtests for broad scenario matrices.
- grep.app attempt with `/Users/n0rmanc/.cargo/bin/grep-app-cli --json --language Go 'httptest.NewServer'` returned `[]`; this does not change the plan because the current repo already has direct `httptest.NewServer` and `openaitest` prior art that is more specific.

## Architecture

### Matrix Artifact

`contracts/openai-subscription-regression-matrix.md` will be the source of truth for cross-feature coverage. Each row must include:

- Acceptance path
- Owning issue/group
- Layer: unit, integration, UI E2E, CI guard
- Current test file and test name
- Required command
- Status: covered, partial, gap fixed
- Notes/gap action

### Test Additions

Implementation should first fill the matrix with current coverage, then add tests only for partial/gap rows. Expected likely additions:

- API-key regression row and tests around omitted `openai_subscription_credential_ids`.
- Models page regression ensuring custom `api_base` + subscription IDs remains blocked and existing API-key create/edit remains unchanged.
- Coverage-map tests or assertions that no real OpenAI hosts are used in OpenAI subscription paths.
- Any missing lifecycle idempotency or redaction rows revealed by the initial matrix.

### CI Representation

The PR must prove the matrix is represented in CI by mapping rows to commands:

```bash
go test ./internal/config ./internal/provider/openai ./internal/proxy/handler ./internal/callback ./internal/spend ./internal/testutil/openaitest
go test ./test/integration/...
go test -tags e2e -count=1 ./test/e2e/...
make lint
make build
```

If full local E2E is too slow or requires unavailable local DB services, targeted local E2E may be used before push, but CI must provide the final full E2E evidence.

## Constitution Check

| Principle | Status | Rationale |
| --- | --- | --- |
| Test-first QA | PASS | Implementation begins with matrix/gap audit and adds regression tests before any production change. |
| Repo reality first | PASS | Plan uses existing `openaitest`, Go tests, and Playwright-Go E2E. |
| No real OpenAI in CI | PASS | All OAuth/token/upstream paths use `httptest.NewServer` and guarded clients. |
| Feature parity | PASS | Existing `api_key` path is a required regression row. |
| Minimal scope | PASS | No UI redesign, no docs main body, no new mock service. |

## Project Structure

```text
specs/1191-openai-subscription-cross-feature-regression/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── tasks.md
├── analyze.md
├── checklists/
│   └── coverage-quality.md
└── contracts/
    └── openai-subscription-regression-matrix.md
```

Implementation-phase files may include only matrix-driven test edits under:

```text
internal/**/*
test/integration/**/*
test/e2e/**/*
```

Production runtime files should remain unchanged unless a matrix row proves a real acceptance gap that cannot be tested without a minimal fix.

## Risks

- Matrix can become stale if it only documents tests without enforcing them in CI. Mitigation: every row must name a command and CI evidence.
- UI E2E can overfit toast text. Mitigation: critical flows must also assert DB/config state or DOM redaction.
- Local E2E may depend on database/browser setup. Mitigation: use targeted local compile/run plus CI full gate, and record any local blocker honestly.
