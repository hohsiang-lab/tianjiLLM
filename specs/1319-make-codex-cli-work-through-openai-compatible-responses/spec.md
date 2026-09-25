# Specification: HO-1319 Codex CLI through OpenAI-compatible `/v1/responses`

## Scope

讓 Codex CLI 使用 Tianji 的 OpenAI-compatible `/v1` gateway 時，可以用 `openai/gpt-5.5` 穩定完成簡單 prompt。

本 issue 不處理 Codex CLI client config 教學。Linear 已記錄 client-side path 經 live verification 可把 built-in OpenAI traffic 導到 Tianji `/v1/responses`。本 issue owns Tianji server-side compatibility。

## Problem

Codex CLI 指向 `https://tianji.hohsiang.com.tw/v1` 後，實際鏈路是：

`Codex CLI openai_base_url -> GET /v1/responses websocket handshake -> first websocket response.create frame -> openai/* wildcard resolution -> subscription credential health/refresh -> model response`

目前 live evidence 顯示這條鏈沒有 end-to-end 打通：

- `GET /v1/responses` with websocket upgrade headers 回 `405 Method Not Allowed`，`Allow: POST`。
- Codex Responses WebSocket 不只是「GET 不 405」：client 會在 handshake 後送 `response.create` frame，包含 `type`, `model`, `input` 等 Responses payload 欄位；`stream` 是 HTTP transport-specific 欄位，WebSocket frame 不可依賴它存在。
- `POST /v1/responses` with `{"model":"openai/gpt-5.5","input":"say OK"}` 回 subscription auth/refresh 類錯誤。
- owner 明確決策：不接受「websocket probe 持續失敗，但 HTTP fallback 剛好可用」作為完成條件。

## User Stories

### US1 - Codex CLI primary transport works without fallback loop (P1)

作為 Tianji operator，我要 Codex CLI 對 Tianji `/v1/responses` 的 primary transport 行為能直接成功，避免 repeated fallback/reconnect noise。

**Independent Test**: 模擬 Codex CLI 對 `GET /v1/responses` 發 websocket upgrade request，帶入 `OpenAI-Beta: responses_websockets=2026-02-06`、`x-client-request-id`、`session_id`、`thread_id`、`Authorization` 等 handshake headers；連線成功後送出不含 HTTP `stream` 欄位的 `{"type":"response.create","model":"openai/gpt-5.5","input":[...]}` frame，驗證 Tianji 能產生 Responses-compatible streaming events，且同一連線可處理後續 `response.create`，不是只做到 route registered / non-405。

### US2 - `/v1/responses` routes `openai/gpt-5.5` through the intended subscription path (P1)

作為 Tianji operator，我要 `/v1/responses` 對 DB-managed `openai/*` wildcard + `openai/gpt-5.5` 使用 intended subscription transport，不誤走未覆蓋的 Platform OpenAI direct HTTP path。

**Independent Test**: 建立 `openai/*` wildcard + subscription credential fixture，對 `/v1/responses` 發 `openai/gpt-5.5` request，驗證 outbound target、auth header、account metadata、response shape 符合 intended Codex-compatible subscription route。

### US3 - Credential health/refresh errors are resolved or surfaced at the correct boundary (P1)

作為 operator，我要 intended active subscription credential 可支撐 `/v1/responses`，並且 refresh/auth failure 不會被誤診為 client config 問題。

**Independent Test**: 對 stale-but-refreshable credential 驗證 `/v1/responses` forced refresh 後成功；對 disabled/unrefreshable credential 驗證回傳可診斷、已 redacted 的 error。

### US4 - Existing chat-completions Codex route stays green (P2)

作為既有 OpenAI-compatible client，我要 HO-1306/HO-1314 已打通的 `/v1/chat/completions` streaming 行為不被 `/v1/responses` 改動破壞。

**Independent Test**: 既有 DB-managed `openai/*` + `stream:true` chat-completions E2E 繼續通過，且 non-streaming deterministic failure contract 不變。

## Functional Requirements

- **FR-001**: Tianji MUST handle Codex CLI 的 `GET /v1/responses` websocket/upgrade request shape，不得回 plain `405 Method Not Allowed` 當作正常完成狀態。
- **FR-002**: Tianji MUST authenticate and accept Codex Responses WebSocket handshake headers, including `OpenAI-Beta: responses_websockets=2026-02-06`, `x-client-request-id`, `session_id`, `thread_id`, and `Authorization`.
- **FR-003**: Tianji MUST process websocket `response.create` frames as the primary transport contract, including `type=response.create`, `model=openai/gpt-5.5`, and `input`; WebSocket input MUST NOT depend on client-provided HTTP `stream=true`.
- **FR-003a**: Tianji MUST keep a successful Responses WebSocket connection open for sequential `response.create` frames until the client closes or an error occurs.
- **FR-004**: A route-only implementation that merely registers `GET /responses`, returns any non-405 status, or reuses the Realtime `/v1/realtime` protocol without proving Responses WebSocket `response.create` handling MUST NOT satisfy this issue.
- **FR-005**: 若 implementation 選擇不支援 websocket transport，必須透過明確 server/client integration 避免 Codex CLI 進入 repeated fallback loop；此方向屬 fallback-based solution，ship 前需要 owner explicit confirmation。
- **FR-006**: `/v1/responses` for model `openai/gpt-5.5` MUST resolve the DB-managed `openai/*` wildcard.
- **FR-007**: `/v1/responses` for `openai/gpt-5.5` MUST use the intended OpenAI subscription transport for Codex-compatible runtime, not silently default to `direct_openai_http` if that bypasses the Codex subscription route.
- **FR-008**: Healthy or refreshable subscription credentials MUST allow a simple `input:"say OK"` request to complete through Tianji.
- **FR-009**: Credential failure responses MUST stay redacted and actionable; they must not expose access tokens, refresh tokens, account secrets, or raw upstream bearer material.
- **FR-010**: The fix MUST preserve existing `/v1/chat/completions` streaming behavior for `openai/gpt-5.5`.
- **FR-011**: The issue MUST add issue-owned regression coverage for the actual Codex CLI `/v1/responses` path, including DB-managed `openai/*` wildcard and `openai/gpt-5.5`.
- **FR-012**: The issue MUST include live or local equivalent verification of the full chain after implementation.

## Non-Goals

- 不修改 Codex CLI source code。
- 不把 HTTP fallback success 當作 acceptance。
- 不把「`GET /responses` route exists」或「不是 405」當作 Responses WebSocket protocol support。
- 不以 Realtime `/v1/realtime` websocket relay 通過作為 `/v1/responses` WebSocket protocol 通過的證據。
- 不把 Tianji API key 或 OpenAI subscription token 寫入 docs、tests、PR body、thread。
- 不改 unrelated OpenAI-compatible endpoints such as embeddings/images/audio unless tests prove shared routing impact.
- 不重新設計 OpenAI subscription OAuth flow UI。

## Success Criteria

- **SC-001**: Codex CLI configured to use Tianji `/v1` and `openai/gpt-5.5` can complete `say OK` without repeated fallback/reconnect noise.
- **SC-002**: Codex-style `GET /v1/responses` WebSocket handshake succeeds with the expected Codex headers, and `response.create` frames without HTTP `stream` are accepted and processed sequentially.
- **SC-003**: `GET /v1/responses` websocket probe no longer returns the current 405 failure mode, but this alone is insufficient without `response.create` frame coverage.
- **SC-004**: `POST /v1/responses` for `openai/gpt-5.5` no longer returns `openai_subscription_reauthorization_required` or disabled-credential failure for the intended active credential path.
- **SC-005**: Regression tests fail before implementation because current `/v1/responses` path does not cover Codex CLI transport + `openai/*` wildcard.
- **SC-006**: Existing HO-1314 chat-completions E2E remains passing.

## Open Questions

- None for Todo scope. Owner already clarified fallback is not acceptable without explicit confirmation.
