# Tasks: HO-1335 `/v1/responses` request-log pipeline

## Phase 1 - RED tests

- [X] T001 Add a request-log capture test helper for handler tests if existing capture helpers are insufficient.
- [X] T002 Add HTTP `POST /v1/responses` success regression test proving one success log is emitted for a mocked successful Codex backend response.
- [X] T003 Add HTTP `POST /v1/responses` failure regression test proving one failure/error log is emitted for upstream transport or non-2xx backend failure.
- [X] T004 Add WebSocket `response.create` generation success regression test proving one success log is emitted after `response.completed`.
- [X] T005 Add WebSocket `generate:false` prewarm regression test proving zero request-log rows/callbacks and zero backend model calls.
- [X] T006 Add no-regression assertion that existing `/v1/chat/completions` Codex logging tests still pass.

## Phase 2 - HTTP implementation

- [X] T007 Capture start/end timing and upstream latency in `handleChatGPTCodexResponse`.
- [X] T008 Add Responses HTTP success logging with model, request id, duration, subscription attribution, and usage when available.
- [X] T009 Add Responses HTTP failure/error logging for upstream transport errors and non-2xx backend responses.
- [X] T010 Preserve HTTP response status, headers, and body while extracting logging metadata.

## Phase 3 - WebSocket implementation

- [X] T011 Add generation-only logging boundary in `handleResponsesWebSocketFrame`.
- [X] T012 Ensure `generate:false` synthetic/prewarm frames bypass request-log logging.
- [X] T013 Add WebSocket upstream transport/non-2xx/stream failure logging exactly once.
- [X] T014 Add WebSocket success logging after a completed real upstream SSE relay.
- [X] T015 Preserve existing connection-local previous-response state behavior.

## Phase 4 - Usage/cost details

- [X] T016 Extract Responses usage from non-streaming JSON responses when present.
- [X] T017 Extract Responses usage from `response.completed` streaming events when present.
- [X] T018 Fall back to visible zero-token success rows when usage is absent.
- [X] T019 Preserve existing upstream token key/subscription attribution propagation.

## Phase 5 - Verification

- [X] T020 Run `go test ./internal/proxy/handler -run 'Responses|Codex|RequestLog|Log' -count=1`.
- [X] T021 Run `go test ./internal/spend -count=1`.
- [X] T022 Run broader relevant package tests if shared logging helpers are touched.
- [X] T023 Run `git diff --check`.
- [ ] T024 Verify true Codex traffic through live Tianji appears in `/ui/logs?page=1&time_range=24h&live_tail=true`.

## Phase 6 - Review artifacts

- [ ] T025 Update PR description with tests, live-verification result, and prewarm no-double-count evidence.
- [ ] T026 Add Linear/thread evidence with request-log row visibility and no secret material.
