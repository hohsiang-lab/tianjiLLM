# Research: HO-1290 Normalize Codex Responses output and errors

## Repo Reality

### Current success response contract

- `internal/model/response.go` defines `ModelResponse` as an OpenAI-compatible chat completion response.
- Required caller-facing fields include `id`, `object`, `created`, `model`, `choices`, and `usage`.
- `Choice.Message` carries the assistant `role` and `content`.

### Current error contract

- `internal/model/errors.go` defines `ErrorResponse` and `ErrorDetail`.
- `ErrorDetail` already has `message`, `type`, `param`, `code`, `llm_provider`, and `model`.
- `TianjiError` carries status, message, type, provider, model, and wrapped sentinel error, but not upstream `code`.

### Current OpenAI provider behavior

- `internal/provider/openai/openai.go` uses Platform `/v1/chat/completions` and parses successful chat completion JSON directly into `model.ModelResponse`.
- For non-200 responses, `parseErrorResponse` reads upstream `error.message`, `error.type`, and `error.code`, but only preserves message/type in `TianjiError`.
- Existing tests in `internal/provider/openai/openai_test.go` cover normal success/error parsing and must remain stable.

### Current handler bug chain

- `internal/proxy/handler/chat.go` calls `p.TransformResponse(...)`.
- On transform error, the handler only preserves one special 401 case containing `OpenAI subscription reauthorization required`.
- All other transform errors currently become HTTP 502 with `type: "internal_error"` and message prefix `transform response:`.
- Therefore an upstream 401/403/429 from a Codex backend adapter would be misleadingly surfaced as a generic proxy/transform failure unless HO-1290 changes the handler propagation path.

## Official OpenAI Evidence

Official docs lookup was restricted to OpenAI domains on 2026-05-10.

- Responses API reference: `POST /v1/responses` creates model responses, and the response object has `object: "response"`, `output[]`, `error`, and `usage`.
- Responses API example shows assistant text inside `output[].content[]` items with `type: "output_text"` and token usage fields `input_tokens`, `output_tokens`, and `total_tokens`.
- Chat Completions API reference defines `POST /v1/chat/completions` and the chat completion object with `object: "chat.completion"` and `choices[].message.content`.
- API overview documents request IDs and rate-limit headers as useful debugging/rate-limit diagnostics.

Conclusion: public OpenAI API shapes support a clear adapter boundary. Codex backend Responses-style success should be normalized to chat completion for `/v1/chat/completions` callers, while upstream error status/details should stay actionable.

## Context7 Evidence

Command:

```bash
npx -y ctx7@latest library openai "Responses API response object output error Chat Completions error shape"
npx -y ctx7@latest docs /openai/openai-go "Responses API Response output items error type ChatCompletion object"
```

Result:

- `/openai/openai-go` documents Chat Completions callers reading `completion.Choices[0].Message.Content`.
- `/openai/openai-go` documents Responses callers using `resp.OutputText()` to aggregate text output.
- `/openai/openai-go` API error examples expose `StatusCode`, `Type`, and `Message`.

Decision: Tianji should implement an explicit Responses-to-chat adapter and preserve structured status/type/message diagnostics.

## GitHub / grep.app Evidence

`grep-app-cli` was not available in this runtime (`command -v grep-app-cli` returned no path), so no external GitHub code samples were used. Repo reality plus official OpenAI docs and Context7 evidence are sufficient for this planning slice.

## Decisions

### D1 - Keep raw Responses JSON out of `/v1/chat/completions`

Returning raw Codex Responses JSON would break OpenAI-compatible chat clients. Normalize success into `model.ModelResponse`.

### D2 - Preserve upstream 401/403/429

Auth, missing-scope, and quota errors are caller-actionable. Rewriting them to 502 hides the fix and makes Tianji look broken when upstream credentials/scopes/quota are the actual issue.

### D3 - Add code preservation

`error.code` is needed for diagnostics such as `missing_scope`, `invalid_token`, and `insufficient_quota`. Add a code-carrying path rather than dropping it after parsing.

### D4 - Keep Platform OpenAI regression separate

HO-1290 should not rewrite `internal/provider/openai` broadly. If implementation must touch shared handler helpers, tests must prove existing OpenAI API-key behavior remains unchanged.

### D5 - Streaming remains out of this slice

The issue acceptance names non-streaming success mapping. Streaming can be covered by HO-1288 or a follow-up after the non-streaming/error surface is stable.
