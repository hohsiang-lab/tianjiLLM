# Research: HO-1335 `/v1/responses` request-log pipeline

## Repo Findings

### `/ui/logs` source of truth

Decision: Treat `SpendLogs` and `ErrorLogs` as the source of truth for UI request logs.

Evidence:

- `internal/ui/handler_logs.go` builds logs page data through `CountRequestLogs` and `ListRequestLogs`.
- `internal/db/queries/spend_views.sql` unions successful spend rows and standalone error rows.

Rationale: The operator-visible bug is not that stdout lacks access lines; it is that the DB-backed UI pipeline has no row for successful `/v1/responses` model work.

### Existing Codex chat logging

Decision: Reuse the same callback helpers or callback data shape used by chat-completions Codex traffic.

Evidence:

- `internal/proxy/handler/chatgpt_codex_backend.go` calls `logSuccess`, `logFailure`, and `logStreamSuccess`.
- `internal/proxy/handler/chat.go` maps those helpers to callback `LogSuccess` / `LogFailure`.
- `internal/spend/tracker.go` persists callback success data to `SpendLogs`.

Rationale: HO-1335 is a parity bug. A separate logging mechanism would risk UI drift and duplicate operational semantics.

### `/v1/responses` gap

Decision: Add logging at the `responses.go` model-work boundary, not in `/ui/logs`.

Evidence:

- `handleChatGPTCodexResponse` forwards successful HTTP upstream response via `copyHTTPResponse(w, resp)`.
- `handleResponsesWebSocketFrame` forwards upstream SSE events over WebSocket and stores response state.
- Neither path currently calls the existing success/failure logging helpers.

Rationale: The UI already queries the right DB-backed views; the missing behavior is request-log persistence at the Responses handler boundary.

### WebSocket prewarm

Decision: Do not log `generate:false` synthetic/prewarm frames.

Evidence:

- `responses.go` recognizes `generate:false`, deletes the field, stores synthetic response state, and writes synthetic `response.created` / `response.completed` events without calling upstream.

Rationale: Those frames are protocol housekeeping. Logging them as requests would create misleading rows and violate the issue scope constraint.

## Official OpenAI Findings

### Responses endpoint and streaming

Decision: Treat successful Responses streaming completion as the success boundary for stream/WebSocket logging.

Evidence:

- Official OpenAI API reference documents `POST https://api.openai.com/v1/responses` for creating model responses.
- Official streaming guide says Responses streaming emits semantic lifecycle events including `response.created`, `response.output_text.delta`, `response.completed`, and `error`.

Rationale: A WebSocket generation should be logged after real upstream lifecycle completion, not when Tianji merely accepts a local frame.

### Request diagnostics

Decision: Preserve request-id logging when available.

Evidence:

- Official OpenAI API reference documents `x-request-id` as a unique request identifier for troubleshooting and recommends logging request IDs in production deployments.

Rationale: Request id is an operator field already expected by `/ui/logs`; Responses parity should keep it.

## Rejected Approaches

- **Tail stdout/access logs in `/ui/logs`**: rejected because current UI intentionally queries DB request-log views.
- **Log WebSocket prewarm as success**: rejected because no upstream model work happened.
- **Only add stdout `request-log:` print**: rejected because `/ui/logs` does not read stdout.
- **Create a Responses-only log table**: rejected because existing UI already reads `SpendLogs` / `ErrorLogs` and the issue asks for the same pipeline.

## Unknowns For Implementation

- Exact usage shape in ChatGPT Codex backend Responses events may require best-effort extraction and zero-token fallback.
- Existing `logSuccess` helpers currently take chat-completion response types; implementation may need a small Responses-specific adapter instead of forcing fake chat response structs.
