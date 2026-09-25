# Implementation Plan: HO-1335 `/v1/responses` request-log pipeline

## Goal

Hook Tianji Codex `/v1/responses` HTTP and WebSocket paths into the existing request-log/error-log pipeline so successful and failed model work appears in `/ui/logs`.

## Constraints

- Todo phase is docs-only. No production code or test implementation in this PR.
- Do not double-count WebSocket prewarm or synthetic `generate:false` frames.
- Keep `/ui/logs` DB-backed; do not add stdout/access-log tailing.
- Preserve existing `/v1/chat/completions` Codex logging behavior.
- Preserve credential redaction.

## Current Architecture

- UI logs: `internal/ui/handler_logs.go`
  - `loadLogsPageData` calls `CountRequestLogs` and `ListRequestLogs`.
- UI log SQL: `internal/db/queries/spend_views.sql`
  - `ListRequestLogs` unions `SpendLogs` and standalone `ErrorLogs`.
- Success request logs: `internal/spend/tracker.go`
  - `Tracker.LogSuccess` records `SpendLogs` and prints `request-log:` diagnostic lines.
- Chat Codex logging: `internal/proxy/handler/chatgpt_codex_backend.go`
  - Non-streaming success calls `logSuccess`.
  - Streaming success calls `logStreamSuccess`.
  - Upstream errors call `logFailure`.
- Responses Codex gap: `internal/proxy/handler/responses.go`
  - HTTP Codex path calls `doOpenAISubscriptionProviderRequest`, then `copyHTTPResponse(w, resp)`.
  - WebSocket path calls `doOpenAISubscriptionProviderRequest`, forwards SSE events, stores connection-local response state, and returns.
  - Neither path currently calls the success/failure logging helpers.

## Proposed Design

### 1. Add Responses-specific logging helpers

Add narrow helper(s) near existing handler logging code or in `responses.go` that can build callback `LogData` from a Responses request.

The helper should:

- Use `buildBaseLogData(ctx, model)` or equivalent existing auth-context extraction.
- Record `Model` from the Responses payload/frame.
- Record request start/end/duration and upstream latency when available.
- Include OpenAI subscription attribution returned by `doOpenAISubscriptionProviderRequest`.
- Extract usage from non-streaming Responses JSON or streaming `response.completed` events when available.
- Still emit success logging when usage is absent, with zero tokens and zero/unknown cost if pricing cannot be derived.
- Never persist raw request bodies containing secrets or raw backend errors containing token material.

### 2. HTTP `POST /v1/responses` success/failure

In `handleChatGPTCodexResponse`:

- Capture `startTime` before decode/upstream call.
- For client validation/decode errors, decide whether existing middleware error logging already records them; if not, record a redacted failure row only when the request was accepted as model work.
- On upstream transport error, call failure logging before returning `502`.
- On upstream non-2xx, call failure logging with status/error detail before copying or returning the upstream error.
- On upstream 2xx, buffer or tee enough response data to extract usage and then call success logging before returning the response to the client.
- Preserve response body, status, and headers.

### 3. WebSocket generation success/failure

In `handleResponsesWebSocketFrame`:

- Do not log frames handled entirely locally, especially `generate:false`.
- Start timing only for frames that are about to call upstream model work.
- On upstream transport error, non-2xx backend response, transform/stream parse error, or failed upstream generation, call failure logging once.
- On successful SSE relay completion, call success logging once.
- Use the real model from the `response.create` frame and include subscription attribution from the selected credential attempt.
- Avoid closing behavior changes except where needed to log before returning.

### 4. Token usage and cost extraction

Preferred extraction order:

1. Native Responses usage in non-streaming JSON response.
2. `response.completed` streaming event usage object.
3. Existing ChatGPT Codex transform usage helpers if they can map Responses events safely.
4. Zero-token success log when usage is unavailable.

Cost should use existing pricing helpers and model normalization conventions. If the model string is `openai/gpt-5.5`, keep the same model naming convention used by existing Codex logging so pricing and UI grouping remain consistent.

### 5. Regression tests

Add tests before implementation:

- HTTP success: mocked backend returns a Responses response with usage; assert one success log.
- HTTP failure: mocked backend returns non-2xx or transport error; assert one failure/error log and zero success logs.
- WebSocket generation success: mocked SSE emits `response.created`, delta, `response.completed` with usage; assert one success log.
- WebSocket prewarm: `generate:false` creates no backend call and no log.
- No-regression: existing chat-completions Codex logging tests still pass.

## Test Plan

- `go test ./internal/proxy/handler -run 'Responses|Codex|RequestLog|Log' -count=1`
- `go test ./internal/spend -count=1`
- `go test ./internal/ui -run 'Logs|Request' -count=1` if UI query mapping changes are touched.
- `git diff --check`
- Live verification after merge candidate:
  - Run true Codex request through `https://tianji.hohsiang.com.tw/v1`.
  - Open `/ui/logs?page=1&time_range=24h&live_tail=true`.
  - Verify a row for the request appears with model/status/duration/upstream token key and usage/cost when derivable.

## Plan Review

- Repo reality checked first: `/ui/logs` reads `CountRequestLogs` / `ListRequestLogs`; chat Codex paths call `logSuccess`, `logFailure`, and `logStreamSuccess`; Responses Codex paths do not.
- Official OpenAI API reference checked: Responses are created with `POST /v1/responses`; streaming Responses emit semantic lifecycle events such as `response.created`, `response.output_text.delta`, and `response.completed`.
- Official OpenAI API reference checked: request diagnostics include `x-request-id`, and OpenAI recommends logging request IDs in production deployments.
- This plan keeps the UI pipeline DB-backed and treats stdout `request-log:` output as a diagnostic side effect, not the UI source of truth.

## Rollout Notes

- Ship behind normal code review and CI gates.
- Live verification must use `/ui/logs` itself, not only `kubectl logs` or stdout.
- A prewarm-only WebSocket probe must remain invisible in `/ui/logs` unless it triggers real upstream model work.
