# Research: OpenAI Subscription Cross-Feature Regression Matrix

## Repo Evidence

- Linear HO-1191 is `Todo`, labels `Feature` and `testing`, with empty comments at dispatch time.
- Parent HO-1173 requires CI-safe OpenAI subscription test matrix and documentation; this issue owns QA matrix, while HO-1192 owns docs.
- Epic HO-1165 requires preserving existing `api_key` / `OPENAI_API_KEY` behavior, explicit `openai_subscription_credential_ids`, no custom `api_base` for subscription credentials, org-scoped credentials, no real OpenAI in CI, and `httptest.NewServer()` mocks.
- `origin/main` includes HO-1172 (`aef7c94`) and current E2E coverage for Models page multi OpenAI subscription selection in `test/e2e/models_openai_subscription_test.go`.
- Existing test helper prior art:
  - `internal/testutil/openaitest/oauth_server_test.go`
  - `internal/testutil/openaitest/upstream_server_test.go`
  - `internal/testutil/openaitest/guard_transport_test.go`
- Existing UI E2E prior art:
  - `test/e2e/openai_connect_flow_test.go`
  - `test/e2e/credentials_test.go`
  - `test/e2e/models_openai_subscription_test.go`

## Decisions

### Decision 1: Use a committed markdown matrix, not a generated spreadsheet

**Chosen**: `contracts/openai-subscription-regression-matrix.md`.

**Reason**: The repo already uses SpecKit markdown artifacts, PR review can diff rows, and the implementation can update rows together with test additions. A generated spreadsheet would not be naturally reviewed in GitHub.

### Decision 2: Reuse `internal/testutil/openaitest`

**Chosen**: Extend or reuse existing OpenAI OAuth/upstream mock helpers.

**Reason**: HO-1190 already provides the reusable local `httptest` harness and no-real-OpenAI guard. A second mock abstraction would increase drift and weaken the guard.

### Decision 3: Treat indirect coverage as partial until assertions match acceptance

**Chosen**: Matrix rows may be `partial`; implementation must close partial rows before Waiting Merge.

**Reason**: HO-1191 is about acceptance paths, not file existence. A test that exercises a flow without asserting the redaction, DB persistence, or no-fallback invariant does not fully protect the path.

### Decision 4: Keep docs work out of HO-1191

**Chosen**: Link HO-1192 as follow-up for admin/troubleshooting docs and keep this issue focused on tests/matrix.

**Reason**: Linear splits QA and docs. Mixing docs into this PR would make review noisy and blur the completion gate.

## External Evidence

- Context7 docs for `/golang/go` confirmed Go `net/http/httptest` utilities include local test servers and recorders.
- Official Go package docs for `net/http/httptest` document `httptest.NewServer` for local HTTP tests.
- Official Go subtests/table-driven docs support using `t.Run` and table entries for broad scenario coverage.
- grep.app returned no useful public-code result for the literal `httptest.NewServer` query in this runtime; local repo prior art is stronger and directly applicable.

## Open Questions

None for Todo planning. Implementation should discover concrete partial/gap rows from the initial matrix fill.
