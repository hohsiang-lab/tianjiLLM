# Specification: HO-1335 `/v1/responses` request-log pipeline

## Scope

Make Codex traffic served through Tianji `/v1/responses` visible in the same UI request-log pipeline used by existing proxied model calls.

This issue owns request/error logging integration for the Codex Responses HTTP and WebSocket paths. It does not redesign `/ui/logs`, raw pod logging, OpenAI subscription credential routing, Codex WebSocket state handling, or model catalog behavior.

## Problem

Operator evidence from 2026-05-13 shows `codex` traffic against `https://tianji.hohsiang.com.tw/v1` succeeds with `model = "openai/gpt-5.5"`, but `/ui/logs?page=1&time_range=24h&live_tail=true` remains empty for those calls.

Root cause chain:

1. `/ui/logs` reads request-log views from DB through `CountRequestLogs` and `ListRequestLogs`; it does not tail stdout or access logs.
2. Successful rows come from the callback/spend pipeline, ultimately creating `SpendLogs` rows.
3. Failure rows become visible through the error/request-log pipeline and `ErrorLogs` rows included by `ListRequestLogs`.
4. Existing `/v1/chat/completions` Codex backend paths call `logSuccess`, `logFailure`, and `logStreamSuccess`.
5. `/v1/responses` Codex HTTP path currently forwards the upstream response with `copyHTTPResponse(w, resp)` and the WebSocket path forwards SSE events as WebSocket messages, but neither path records the real upstream model work in the request-log pipeline.
6. Therefore Codex `/v1/responses` traffic can complete successfully while `/ui/logs` shows no row.

## User Stories

### US1 - Successful HTTP Responses calls appear in UI logs (P1)

As a Tianji operator, I want successful `POST /v1/responses` Codex calls to appear in `/ui/logs`, so I can confirm traffic reached Tianji without reading pod logs.

**Independent Test**: Send a successful Codex-shaped HTTP `POST /v1/responses` through a handler test with a request-log capture callback or DB-backed logger, then assert one success log record is produced with model, request id, duration, upstream subscription attribution, and token usage when available.

### US2 - Failed HTTP Responses calls appear in UI logs (P1)

As a Tianji operator, I want failed `POST /v1/responses` Codex calls to appear through the same failed-log path that `/ui/logs` reads, so backend errors are diagnosable.

**Independent Test**: Make the mocked ChatGPT Codex backend return a non-2xx or transport error for HTTP `POST /v1/responses`, then assert the request-log/error-log pipeline receives exactly one failure record and no success record.

### US3 - Successful primary WebSocket generation appears in UI logs (P1)

As a Codex CLI user, I want successful WebSocket primary-path generation frames to create request-log rows, so clean WebSocket success is operationally visible.

**Independent Test**: Open `GET /v1/responses` WebSocket, send a real generation `response.create` frame that reaches the mocked backend, stream `response.completed`, and assert one success log record is produced.

### US4 - WebSocket prewarm is not double-counted (P1)

As an operator, I do not want local protocol housekeeping to create misleading request rows.

**Independent Test**: Send only a `response.create` prewarm frame with `generate:false`; Tianji may synthesize WebSocket events, but the backend is not called and no request-log success/failure row is produced.

### US5 - Existing chat-completions logging remains unchanged (P2)

As an existing client, I want `/v1/chat/completions` Codex request logging to keep its current behavior.

**Independent Test**: Existing chat-completions Codex logging tests stay green, including streaming success, upstream failure, and token/cost propagation behavior.

## Functional Requirements

- **FR-001**: Successful Codex HTTP `POST /v1/responses` model work MUST call the same success request-log pipeline used by existing proxied model endpoints.
- **FR-002**: Failed Codex HTTP `POST /v1/responses` model work MUST call the same failure/error request-log pipeline read by `/ui/logs`.
- **FR-003**: Successful Codex WebSocket generation frames that reach upstream model work MUST call the same success request-log pipeline.
- **FR-004**: Failed Codex WebSocket generation frames that reach or attempt upstream model work MUST call the failure/error request-log pipeline with a redacted, actionable error.
- **FR-005**: WebSocket prewarm or synthetic `generate:false` handling MUST NOT create request-log rows when no real upstream model work occurs.
- **FR-006**: A single real upstream Responses request MUST create at most one request-log row.
- **FR-007**: Logged rows MUST include normal operational fields when available: Tianji request id, model, status, duration, upstream token key/subscription attribution, prompt/completion/total token usage, and cost.
- **FR-008**: Token usage and cost extraction MAY be best-effort when the Responses event stream lacks usage data, but a successful request MUST still be visible even with zero or unknown usage.
- **FR-009**: Logging MUST preserve existing redaction rules; no access token, refresh token, account secret, raw bearer value, or encrypted credential blob may be written to docs, tests, DB logs, PR body, or thread.
- **FR-010**: `/ui/logs` behavior MUST remain DB-backed through `CountRequestLogs` and `ListRequestLogs`; this issue must not implement stdout/access-log tailing.
- **FR-011**: Existing `/v1/chat/completions` Codex logging behavior MUST remain unchanged.
- **FR-012**: Regression coverage MUST prove `/v1/responses` traffic becomes visible through the same request-log pipeline the UI reads.
- **FR-013**: Live operator verification MUST prove a real Codex request through Tianji appears in `/ui/logs` without relying on raw pod logs.

## Non-Goals

- Do not implement raw pod/access-log tailing in `/ui/logs`.
- Do not change the `/ui/logs` data source away from `CountRequestLogs` / `ListRequestLogs`.
- Do not double-count WebSocket prewarm or synthetic adapter frames.
- Do not change Codex CLI, OpenAI subscription credential CRUD, model catalog schema, or WebSocket state chaining behavior beyond what logging requires.
- Do not weaken credential/log redaction.

## Success Criteria

- **SC-001**: A successful HTTP `POST /v1/responses` regression test fails on current main because no request-log callback/row is produced, then passes after implementation.
- **SC-002**: A failed HTTP `POST /v1/responses` regression test fails on current main because no failure/error log is produced, then passes after implementation.
- **SC-003**: A successful Codex WebSocket generation regression test fails on current main because no request-log row is produced, then passes after implementation.
- **SC-004**: A WebSocket `generate:false` prewarm test proves zero request-log rows are produced.
- **SC-005**: Existing `/v1/chat/completions` Codex logging tests remain green.
- **SC-006**: Live `codex` traffic against Tianji appears in `/ui/logs` within the selected 24h window.

## Open Questions

None for Todo scope. Implementation may choose exact helper structure, but it must preserve the row-count and prewarm constraints above.
