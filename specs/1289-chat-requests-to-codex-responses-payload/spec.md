# Feature Specification: OpenAI-compatible chat request 轉 Codex Responses payload

**Branch**: `HO-1289-chat-requests-to-codex-responses`
**Linear**: HO-1289
**Created**: 2026-05-10
**Status**: Todo planning only；本階段不修改 production code

## 摘要

TianjiLLM 需要一個 deterministic mapper，把 `/v1/chat/completions` 形式的 `ChatCompletionRequest` 轉成已 merged 的 HO-1288 `chatgpt_codex_backend` transport 要送到 `chatgpt.com/backend-api/codex/responses` 的 Codex Responses payload。HO-1288 目前已提供 `internal/provider/chatgptcodex.Transport` 與 handler route，但 `BuildRequest` 仍暫時把 legacy `req.Messages` 直接放進 `input`；本 issue owns replacing that payload construction、`model` normalization、`messages` 轉 `instructions` / `input`、相容 option allowlist，以及 unsupported field 的明確處理。

## Scope

In scope:

- 新增純 chat-to-Codex Responses payload transformation boundary。
- 將 Tianji/OpenAI-compatible model name normalize 成 backend Codex model string。
- 轉換 `system`、`developer`、`user`、`assistant` 與可安全表示的 multimodal content。
- 保留相容的 non-streaming basics，例如 token limit、`temperature`、`top_p`、`metadata`、`user` correlation。
- 對 `tools`、`tool_calls`、`n > 1`、`logprobs`、unsupported `response_format`、unknown `ExtraParams` 等欄位明確 reject 或 diagnostic ignore。
- Implementation 必須先寫 RED unit tests，涵蓋 `developer` / `system` / `user` conversion 與 unsupported-field diagnostics。
- Mapper 必須接到 HO-1288 已 merged 的 `chatgptcodex.Transport.BuildRequest` boundary，取代目前 direct `input: req.Messages` 的暫時 payload。

Out of scope:

- 實作 HO-1288 backend HTTP transport、credential resolution、retry 或 response adaptation。
- 改變一般 OpenAI API-key `/v1/chat/completions` behavior。
- 在測試中呼叫真實 `chatgpt.com` 或 `api.openai.com`。
- 新增 model transport selection UI。
- 實作完整 tool-call execution semantics。
- 實作 streaming response adapter；HO-1288 目前 route 對 streaming 回 `unsupported_streaming`，本 issue 只 owns deterministic request-side diagnostic / no-upstream behavior。

## User Stories and Tests

### User Story 1 - 基本 chat request 轉成 Codex Responses request（P1）

作為 Tianji client，我要用一般 `/v1/chat/completions` request 觸發 Codex backend transport，讓 backend 收到合法的 Responses-style body。

**Independent Test**: Unit test 給 `model` + 一個 `user` message，assert output 有 normalized `model`、`input` message item、`content` 為 `input_text`，且沒有 legacy `messages` key。

### User Story 2 - `developer` 與 `system` message 語意被保留（P1）

作為 Codex caller，我需要 `developer` / `system` instruction 不被當成普通 user text。

**Independent Test**: Unit tests 覆蓋 `system`、`developer`、mixed leading instruction messages。Leading instruction messages 依原順序帶 role label 合併到 top-level `instructions`；non-leading instruction message 必須被明確 preserve 成 `input` item 或 deterministic reject。

### User Story 3 - 相容 caller options 安全保留（P1）

作為 client，我希望 backend 支援的基本 generation controls 不被 mapper 靜默丟失或改語意。

**Independent Test**: Unit tests 覆蓋 `max_tokens` / `max_completion_tokens`、`temperature`、`top_p`、`stream`、`metadata`、`user`，assert exact output key 或 explicit diagnostic。

### User Story 4 - Unsupported fields 不可 silent corrupt upstream request（P1）

作為 operator，我需要 unsupported chat-completion fields 明確 fail 或 diagnostic ignore，而不是盲目 forward 到 Codex backend。

**Independent Test**: Unit tests 覆蓋 `tools`、`tool_choice`、assistant `tool_calls`、`tool` messages、`n > 1`、`logprobs`、`top_logprobs`、`stop`、unsupported `response_format`、unknown `ExtraParams`。

## Functional Requirements

- **FR-001**: System MUST expose pure function 或 narrow package boundary，將 `model.ChatCompletionRequest` 轉為 Codex backend Responses payload。
- **FR-002**: System MUST normalize configured model IDs，避免 backend payload `model` 帶入 Tianji-only routing prefix。
- **FR-003**: System MUST convert `messages` into Responses-compatible `input` items and/or top-level `instructions`。
- **FR-004**: System MUST 用 unit tests 覆蓋 `system`、`developer`、`user` message conversion。
- **FR-005**: System MUST map simple text content to `content: [{ "type": "input_text", "text": ... }]`。
- **FR-006**: System MUST 只在可安全表示時把 image content map to `input_image`。
- **FR-007**: System MUST preserve compatible non-streaming basics，包括 token limit、`temperature`、`top_p`。
- **FR-008**: System MUST deterministic handle `stream`：沿用 HO-1288 current handler behavior，streaming request 不得送 upstream，必須回 explicit unsupported diagnostic until a separate streaming adapter exists。
- **FR-009**: System MUST reject `n > 1`。
- **FR-010**: System MUST reject 或 diagnostic ignore unsupported fields，不可 silent forward。
- **FR-011**: System MUST NOT include access token、refresh token、credential ID、raw credential JSON in payload or diagnostics。
- **FR-012**: System MUST preserve normal OpenAI provider behavior for routes not selecting Codex backend transport。
- **FR-013**: Tests MUST use local fixtures；不得真實呼叫 OpenAI/ChatGPT network。

## Key Entities

- **ChatCompletionRequest**: `internal/model/request.go` 既有 OpenAI-compatible chat request model。
- **Codex Responses Payload**: HO-1288 `chatgptcodex.Transport` 送往 `chatgpt.com/backend-api/codex/responses` 的 internal JSON body。
- **Instruction Block**: Leading `system` / `developer` messages 依序合併出的 top-level `instructions`。
- **Input Message Item**: Responses-compatible item，包含 `type: "message"`、role、typed content parts。
- **Unsupported Field Diagnostic**: 說明 rejected / omitted fields 的 structured validation result。

## Success Criteria

- **SC-001**: Unit tests 證明 `developer`、`system`、`user` messages 正確 mapping。
- **SC-002**: Unit tests 證明 backend payload 不 forward legacy `messages`。
- **SC-003**: Unit tests 證明 compatible options 被保留或明確 diagnostic。
- **SC-004**: Unsupported fields 有 deterministic error 或 diagnostics。
- **SC-005**: Regression tests 證明 API-key OpenAI chat transform 不變。
- **SC-006**: Todo branch 只包含 SpecKit artifacts。

## Dependencies

- HO-1288 已 merged backend transport selection and HTTP execution：`internal/provider/chatgptcodex/transport.go`、`internal/proxy/handler/chatgpt_codex_backend.go`。
- `internal/model/request.go` 的 `ChatCompletionRequest` / `Message` shapes。
- `internal/provider/openai/openai.go` 的 existing OpenAI provider transform 作為 regression baseline。
- `internal/proxy/handler/responses*.go` 的 existing Responses proxy behavior。
