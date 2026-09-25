# Specification: HO-1321 Codex Responses WebSocket prewarm/state/large frames

## Scope

讓 Codex CLI 對 Tianji `/v1/responses` 使用 OpenAI-compatible Responses WebSocket primary transport 時，能處理真實 Codex 0.130.0 的 prewarm、state chaining、與大型 `response.create` frame，不再靠 WebSocket 失敗後 HTTP fallback 完成。

本 issue owns Tianji server-side adapter behavior。它不修改 Codex CLI，不重新設計 OpenAI subscription OAuth，也不把 HTTP fallback success 當完成條件。

## Problem

HO-1319 已讓 Codex CLI 對 Tianji 完成 `OK`，但 live evidence 顯示完成路徑仍是 WebSocket primary transport 失敗後的 HTTP fallback：

- Codex 0.130.0 會先送 WebSocket prewarm `response.create`，帶 `generate:false` 與 `tools: []`。
- Tianji 目前把 raw payload 交給 `chatgptcodex.Transport.BuildResponsesRequest`，`internal/provider/chatgptcodex/transport.go` 的 `normalizeResponsesPayload` 會保留未知欄位，因此 `generate:false` 被送進 ChatGPT Codex backend，live path 因 `Unsupported parameter: generate` 關閉 WebSocket。
- Codex 後續 request 可只帶 `previous_response_id=<warmup id>` 與 `input: []`；若 Tianji 只 strip `generate` 而沒有保存或重建 warmup state，backend 仍會收到空/不完整 request。
- `internal/proxy/handler/responses.go` 的 WebSocket accept/read path 沒有 `SetReadLimit`；`github.com/coder/websocket` 預設讀取限制讓真實 Codex frame 在 32769 bytes 附近被 `1009` 關閉。

Root cause chain: Tianji 只把 `/v1/responses` WebSocket 當作逐 frame HTTP/SSE bridge，沒有實作 OpenAI Responses WebSocket adapter 需要的 prewarm no-op 消化、connection-local response state、與大型 frame capacity；因此 Codex CLI 會看到 repeated reconnect noise 並改走 HTTP fallback。

## User Stories

### US1 - Codex prewarm is accepted without backend `generate` leakage (P1)

作為 Tianji operator，我要 Codex 的 `generate:false` prewarm frame 在 Tianji boundary 被消化或轉譯，不能把 ChatGPT Codex backend 不支援的 `generate` 直接轉發。

**Independent Test**: WebSocket handshake 後送 `{"type":"response.create","model":"openai/gpt-5.5","generate":false,"tools":[],"input":[...]}`，mock backend 不應收到 `generate`，client 應收到可供後續 chain 的 warmup `response.created` / `response.completed` 或等價 adapter-generated response id。

### US2 - Follow-up `previous_response_id` can use warmup state (P1)

作為 Codex CLI 使用者，我要 prewarm 後的 generation frame 可以只帶 `previous_response_id` 與 incremental input，Tianji 仍能送出 backend-valid payload。

**Independent Test**: 同一 WebSocket 先送 prewarm，取得 `resp_prewarm`；再送 `{"type":"response.create","model":"openai/gpt-5.5","previous_response_id":"resp_prewarm","input":[]}`。測試需證明 Tianji 沒有把空 `input: []` 直接轉成不完整 backend request，而是從 connection-local state 補齊或安全重建有效 request。

### US3 - Realistic Codex large frames are accepted (P1)

作為 Codex CLI 使用者，我要帶完整 instructions、tools、metadata 的大型 frame 不會因 Tianji WebSocket read limit 被關閉。

**Independent Test**: 用 >32KiB 的 Codex-like `response.create` frame 連到 `/v1/responses`，連線不應以 `1009 read limited at 32769 bytes` 關閉，backend 應收到 normalized request。

### US4 - True Codex CLI has no reconnect noise (P1)

作為 operator，我要真實 `codex exec` 對 `https://tianji.hohsiang.com.tw/v1` 完成 `OK` 時沒有 `Reconnecting... 2/5` 到 `5/5`。

**Independent Test**: 實作後用 Codex CLI 0.130.x 或當前 repo-locked CLI 對 Tianji live route 執行簡單 prompt，驗證 `status=0`、final output `OK`、JSONL/stderr 不含 WebSocket reconnect fallback noise。

### US5 - HO-1319/HO-1314 behavior stays green (P2)

作為既有 client，我要小型 WebSocket frame、HTTP `/v1/responses`、與 `/v1/chat/completions` Codex subscription streaming 不被本次 adapter 改動破壞。

**Independent Test**: 既有 `responses_codex_test.go` sequential frame test、HO-1319 responses route tests、HO-1314 chat-completions E2E 繼續通過。

## Functional Requirements

- **FR-001**: Tianji MUST accept Codex Responses WebSocket handshake with `OpenAI-Beta: responses_websockets=2026-02-06` on `GET /v1/responses`.
- **FR-002**: Tianji MUST raise or otherwise handle WebSocket read limits so realistic Codex `response.create` frames larger than 32KiB are accepted.
- **FR-003**: Tianji MUST treat `generate:false` as OpenAI-compatible WebSocket adapter control data and MUST NOT forward unsupported `generate` to ChatGPT Codex backend.
- **FR-004**: Prewarm handling MUST return or synthesize a response id that can be used by a later `previous_response_id` on the same WebSocket connection.
- **FR-005**: Tianji MUST maintain connection-local previous-response state for at least the most recent successful warmup/generated response on a WebSocket connection.
- **FR-006**: A follow-up frame with `previous_response_id` and `input: []` MUST NOT be forwarded as an empty/incomplete backend request.
- **FR-007**: For `store:false` / ZDR-compatible behavior, Tianji MUST define behavior when referenced state is absent: return a redacted `previous_response_not_found`-style error or require full input context; do not silently invent unsafe context.
- **FR-008**: Tianji MUST continue processing multiple sequential `response.create` frames on one connection, one in-flight response at a time.
- **FR-009**: Tianji MUST preserve DB-managed `openai/*` + `openai/gpt-5.5` routing through `chatgpt_codex_backend`.
- **FR-010**: Tianji MUST preserve existing HTTP `/v1/responses` and `/v1/chat/completions` Codex subscription contracts.
- **FR-011**: Regression coverage MUST be RED before implementation for prewarm, state chaining, and >32KiB frame handling.
- **FR-012**: Live verification MUST use true `codex exec`, not only synthetic WebSocket probes.

## Non-Goals

- 不修改 Codex CLI source。
- 不把 `generate:false` 直接送到 ChatGPT Codex backend。
- 不只改 read limit 而忽略 prewarm/state chaining。
- 不只 strip `generate` 而讓 follow-up `previous_response_id + input: []` 失敗。
- 不新增 token/credential 到 docs、tests、PR body、thread。
- 不重新設計 OpenAI subscription credential CRUD / refresh UI。

## Success Criteria

- **SC-001**: Prewarm `response.create(generate:false)` regression test 會在 current main 失敗，實作後通過。
- **SC-002**: Follow-up `previous_response_id + input: []` regression test 會在 current main 失敗，實作後通過，且 backend request 是完整有效 payload。
- **SC-003**: >32KiB Codex-like WebSocket frame regression test 會在 current main 失敗於 read limit，實作後通過。
- **SC-004**: 真實 `codex exec` 對 Tianji live route 回 `OK` 且沒有 `Reconnecting...` noise。
- **SC-005**: 既有 HO-1319 `/v1/responses` 小 frame sequential handling 與 HO-1314 chat-completions streaming tests 維持 green。

## Open Questions

None for Todo scope. Implementation phase may choose exact adapter strategy, but it must satisfy prewarm response id and connection-local state semantics before shipping.
