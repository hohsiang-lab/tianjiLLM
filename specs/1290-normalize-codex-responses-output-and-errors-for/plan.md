# Implementation Plan: Normalize Codex Responses output and errors

**Branch**: `HO-1290-normalize-codex-responses-output-and-errors`
**Linear**: HO-1290
**Spec**: [spec.md](spec.md)
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`.

## Summary

Add a narrow Codex backend response/error adapter and a handler error propagation fix for Codex-selected routes. The key production bug chain is: provider/adapter can return a structured `model.TianjiError` for upstream 401/403/429, but `handleNonStreamingCompletion` currently treats most `TransformResponse` errors as internal transform failures and writes HTTP 502. HO-1290 should make actionable upstream errors survive to the caller while keeping generic parse/shape failures as safe transform errors.

## Technical Context

- Language/runtime: Go.
- Existing OpenAI-compatible response type: `internal/model/response.go`.
- Existing OpenAI-compatible error body: `internal/model/errors.go`.
- Existing handler path: `internal/proxy/handler/chat.go`.
- Existing OpenAI provider: `internal/provider/openai/openai.go`.
- HO-1288 related planning PR: `https://github.com/hohsiang-lab/tianjiLLM/pull/162`.
- Official OpenAI Responses object has `object: "response"`, `output[]`, `error`, and `usage` fields; Chat Completions returns `object: "chat.completion"` with `choices[]`.

## Constitution Check

| Principle | Status | Notes |
|---|---:|---|
| Spec-first | PASS | Todo phase changes only this specs directory. |
| Repo reality first | PASS | Plan is based on current `chat.go`, `openai.go`, `model.Response`, and `model.ErrorResponse`. |
| Research before build | PASS | `research.md` records repo, official OpenAI docs, Context7, and grep-app availability. |
| Failing-tests-first | PASS | `tasks.md` starts with RED adapter and handler tests. |
| Security | PASS | Error preservation is bounded by redaction and no token leakage tests. |
| Narrow scope | PASS | No streaming, OAuth, UI, or default OpenAI provider rewrite in this issue. |

## Architecture Decision

### D1 - Codex adapter owns private backend shape

Create or extend a Codex-specific adapter near the HO-1288 transport boundary, for example:

```text
internal/provider/chatgptcodex/
  response.go
  response_test.go
```

The normal `internal/provider/openai.Provider` should not learn private `chatgpt.com/backend-api/codex/responses` response details.

### D2 - Success maps to existing `model.ModelResponse`

Map backend Responses JSON to:

```json
{
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {"role": "assistant", "content": "..."},
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  }
}
```

Use the backend `id`, `model`, and `created_at` where available. Use a deterministic safe fallback only when the backend omits optional fields and tests document that fallback.

### D3 - Handler preserves actionable `TianjiError`

Refactor `handleNonStreamingCompletion` transform-error handling so:

- `model.TianjiError.StatusCode` in 400-499 returns that status instead of 502.
- `error.type`, `error.message`, and `error.code` are preserved when present.
- Provider/model fields are carried when safe.
- Generic parse/shape failures remain safe `502 internal_error`.

This may become a small helper such as:

```go
writeTransformError(w, err, req.Model)
```

### D4 - Error body parser captures `code`

Existing `openai.parseErrorResponse` parses upstream `error.code` but does not store it on `model.TianjiError`. Implementation should either add a `Code` field to `TianjiError` or use a Codex-specific upstream error type that can populate `model.ErrorDetail.Code`.

### D5 - Redaction is part of the contract

All upstream error message passthrough must use existing redaction helpers before response/log emission when there is any chance the message came from the private backend or credential path.

## Target Flow

```text
Client /v1/chat/completions
  -> selected Codex backend transport sends request
  -> backend returns non-streaming Responses JSON or HTTP error
  -> Codex adapter:
       success: Responses JSON -> model.ModelResponse
       error: HTTP status + safe error fields -> model.TianjiError
  -> handler:
       model.ModelResponse -> 200 chat.completion JSON
       actionable TianjiError 401/403/429 -> same status OpenAI-compatible error JSON
       generic parse/shape error -> 502 internal_error
```

## Repo Evidence

- `internal/model/response.go` defines Tianji's current OpenAI-compatible `ModelResponse` with `choices[]` and `usage`.
- `internal/model/errors.go` defines `ErrorResponse` and `ErrorDetail` with `message`, `type`, `param`, `code`, `llm_provider`, and `model`.
- `internal/provider/openai/openai.go` maps non-200 upstream responses to `model.TianjiError`, but `TianjiError` currently has no `Code` field.
- `internal/proxy/handler/chat.go` only preserves a very narrow OpenAI subscription reauthorization 401; other `TransformResponse` errors become HTTP 502.
- `internal/provider/openai/openai_test.go` already covers normal OpenAI provider success/error behavior and should stay green.

## External Evidence

- Official OpenAI Responses API docs show Responses objects use `object: "response"`, `output[]`, `error`, and `usage.input_tokens` / `usage.output_tokens` / `usage.total_tokens`.
- Official OpenAI Chat Completions docs show chat completion callers expect `object: "chat.completion"` and `choices[].message.content`.
- Official OpenAI API overview documents `x-request-id` and rate-limit headers; implementation should preserve useful diagnostics where current abstractions allow it.
- Context7 `/openai/openai-go` examples show the Responses SDK exposes aggregated `OutputText()` while Chat Completions callers read `Choices[0].Message.Content`; this supports an explicit adapter rather than returning raw Responses JSON to chat clients.

## Risk Register

| Risk | Mitigation |
|---|---|
| Codex private backend shape differs from public Responses docs | Keep parser tolerant and test against known mocked fixture shapes; preserve raw unknown fields only in tests/debug, not caller body. |
| Upstream errors leak token material | Redaction + sentinel leakage tests for response body and log path. |
| Generic parse errors accidentally become 4xx | Only preserve status when error is structured upstream `TianjiError`; parse/shape failures stay 502. |
| API-key OpenAI provider behavior changes | Run existing `internal/provider/openai` tests and add a regression guard. |
| Missing `error.code` loses useful diagnostics | Add code-carrying field/type and tests for `missing_scope` / `insufficient_quota`. |

## Verification Commands

```bash
go test ./internal/provider/... ./internal/proxy/handler/... -run 'Test.*Codex.*Response|Test.*Codex.*Error|Test.*TransformResponse|Test.*OpenAIProvider' -count=1
go test ./internal/model/... ./internal/provider/openai/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Todo Gate Status

Spec/plan/tasks/analyze are complete when this docs-only branch is pushed and the draft PR exists. Implementation remains blocked until Linear moves to `In Progress`.
