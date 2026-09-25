# Quickstart: HO-1191 QA Matrix

## Implementation Flow

1. Fill `contracts/openai-subscription-regression-matrix.md` from current repo tests.
2. Mark every row `covered`, `partial`, or `blocked`.
3. Add tests for every `partial` row that is in HO-1191 scope.
4. Keep OAuth/token/upstream tests on `httptest.NewServer()` or `internal/testutil/openaitest`.
5. Run targeted commands locally.
6. Push, let CI run full lint/test/e2e/build, and update matrix/analyze with CI evidence before Waiting Merge.

## Useful Local Commands

```bash
go test ./internal/config ./internal/provider/openai ./internal/proxy/handler ./internal/callback ./internal/spend ./internal/testutil/openaitest
go test ./test/integration/...
go test -tags e2e -count=1 ./test/e2e/... -run 'Test(OpenAI|Credential|ModelOpenAISubscription)'
make lint
make build
git diff --check origin/main...HEAD
```

## Completion Gate

HO-1191 is not complete because a markdown matrix exists. It is complete only when:

- every matrix row has CI-represented coverage,
- every gap row has a test or a justified blocker,
- no real OpenAI host is required in CI,
- existing `api_key` path and custom `api_base` behavior are explicitly protected,
- UI E2E covers Connect/Test/Refresh/Disable/Delete and Models page multi-select.
