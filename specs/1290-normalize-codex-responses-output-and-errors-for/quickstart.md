# Quickstart: HO-1290 Codex response/error normalization

## Preconditions

- Linear HO-1290 is `In Progress` before production edits.
- Worktree: `worktrees/tianjiLLM/HO-1290-normalize-codex-responses-output-and-errors`.
- HO-1288 transport branch/PR context has been rechecked before implementation.

## RED Test Targets

Add failing tests first:

```bash
go test ./internal/provider/... ./internal/proxy/handler/... -run 'Test.*Codex.*Response|Test.*Codex.*Error' -count=1
```

Expected RED failures before implementation:

- Codex Responses success is not mapped to `chat.completion`.
- 401/403/429 transform errors are still wrapped as 502.
- Upstream `error.code` is not preserved.

## Implementation Checkpoints

1. Add Codex response fixture/test for successful non-streaming Responses JSON.
2. Add error fixtures/tests for 401, 403 missing scope, and 429 quota.
3. Implement Codex adapter.
4. Add/adjust handler helper to preserve actionable `TianjiError` status/code.
5. Add leakage tests with sentinel token strings.
6. Run OpenAI provider regression tests.

## Verification

```bash
go test ./internal/model/... ./internal/provider/openai/... -count=1
go test ./internal/provider/... ./internal/proxy/handler/... -run 'Test.*Codex.*Response|Test.*Codex.*Error|Test.*TransformResponse|Test.*OpenAIProvider' -count=1
go test ./internal/provider/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Expected Caller Output

Successful Codex backend response through `/v1/chat/completions` returns:

```json
{
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "..."
      },
      "finish_reason": "stop"
    }
  ]
}
```

Actionable upstream errors keep status:

```json
{
  "error": {
    "message": "missing required scope: model.request",
    "type": "permission_error",
    "code": "missing_scope"
  }
}
```
