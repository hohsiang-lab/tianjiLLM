# Implementation Plan: Chat requests to Codex Responses payload

**Branch**: `HO-1289-chat-requests-to-codex-responses`
**Linear**: HO-1289
**Spec**: [spec.md](spec.md)
**Phase**: Todo planning only；implementation starts only after Linear moves to `In Progress`

## 摘要

新增小而可測的 mapper，將 Tianji `ChatCompletionRequest` 轉成已 merged 的 HO-1288 ChatGPT Codex backend transport 的 Responses request body。Mapper 不處理 HTTP transport、credential resolution、rate-limit retry，也不改一般 OpenAI provider；它取代 HO-1288 目前 `Transport.BuildRequest` 裡 direct `input: req.Messages` 的暫時 payload。

## Technical Context

- Language/runtime: Go。
- Existing chat request model: `internal/model/request.go`。
- Existing OpenAI chat transform: `internal/provider/openai/openai.go`。
- Existing Responses route proxy: `internal/proxy/handler/responses.go` and `responses_ext.go`。
- HO-1288 已 merged：`internal/provider/chatgptcodex/transport.go` builds ChatGPT Codex backend HTTP request；`internal/proxy/handler/chatgpt_codex_backend.go` selects the transport and rejects streaming。
- Testing: unit-level, offline, no real OpenAI/ChatGPT network。

## Constitution Check

| Principle | Status | Notes |
|---|---:|---|
| Spec-first | PASS | Todo phase is docs-only。 |
| Repo reality first | PASS | 已查 `ChatCompletionRequest`、OpenAI provider transform、Responses handlers、HO-1288 merged `chatgptcodex` transport/handler。 |
| RED tests first | PASS | Tasks 先寫 failing mapper tests。 |
| Narrow ownership | PASS | HO-1289 owns payload mapping only；HO-1288 owns transport。 |
| Security | PASS | Diagnostics 不得 leak credential material。 |

## Architecture Decision

### D1 - Mapper package，不 mutate OpenAI provider

Implementation 建議新增窄邊界，例如：

```text
internal/provider/chatgptcodex/payload.go
internal/provider/chatgptcodex/payload_test.go
```

HO-1288 已建立 `internal/provider/chatgptcodex`，所以 implementation 優先把 mapper 放在該 package，並由 `Transport.BuildRequest` 使用 mapper output。不要把 `internal/provider/openai.transformRequestBody` 改成 Codex shape；那個 function owns Platform `/v1/chat/completions`。

### D2 - Output shape

Mapper output 應等價於：

```json
{
  "model": "gpt-5.2-codex",
  "instructions": "system: ...\ndeveloper: ...",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "hello" }]
    }
  ],
  "stream": false
}
```

Struct name 可依 repo convention 調整，但 tests 必須 assert wire JSON。

### D3 - Instruction handling

Leading `system` and `developer` messages become top-level `instructions`，依原順序保留並加 role label。若 `system` / `developer` 出現在 user/assistant turns 後面，implementation 必須選擇並測試其中一種行為：

- preserve as explicit `input` message item with same role；或
- reject with deterministic diagnostic。

### D4 - Supported content

Support:

- string content
- array content with text parts
- image parts only when current `ContentPart` / `ImagePart` shape can safely map to `input_image`

Unsupported arbitrary JSON object 不得 stringify；必須 diagnostic。

### D5 - Option policy

Allowed/pass-through with tests:

- `max_completion_tokens` / `max_tokens` normalized to backend token limit key
- `temperature`
- `top_p`
- `stream` as explicit unsupported diagnostic/no-upstream behavior, matching HO-1288 current handler
- `metadata`
- `user` as safe metadata/client correlation, not auth

Rejected or diagnostic-only by default:

- `tools`, `tool_choice`, assistant `tool_calls`, `tool` messages
- `n > 1`
- `logprobs`, `top_logprobs`
- unsupported `response_format`
- `stop` unless verified compatible
- unknown `ExtraParams` unless explicitly allowlisted

## Repo Evidence

- `internal/model/request.go` captures known chat fields and unknown JSON fields in `ExtraParams`。
- `internal/provider/openai/openai.go` forwards legacy `messages` to `/chat/completions`；HO-1289 不能 reuse that body for Codex backend。
- `internal/proxy/handler/responses.go` proxies `/v1/responses` route family；it does not convert chat messages。
- HO-1288 merged `internal/provider/chatgptcodex/transport.go` calls `https://chatgpt.com/backend-api/codex/responses` but currently marshals `input: req.Messages`；HO-1289 replaces that payload with typed Responses `instructions` / `input` output。
- HO-1288 merged `internal/proxy/handler/chatgpt_codex_backend.go` returns `unsupported_streaming` before upstream for streaming requests；HO-1289 must preserve that no-upstream behavior unless a separate streaming adapter issue changes it。

## Plan Review Evidence

- Context7 `/openai/openai-go` docs show `Responses.New` uses `Input` and `Model`，支持 mapper target `input`。
- Official OpenAI docs search found Responses API as public documented surface；no official doc was found for private `chatgpt.com/backend-api/codex/responses`，所以此 mapper 必須 isolated and fixture-tested。
- `grep-app-cli` against `openai/codex` found `ResponseInputItem::Message { role, content, phase }` and `ContentItem::InputText`，支持 typed `input` item。
- `openai/codex` tests assert Codex requests hit `/api/codex/responses` and include `stream` / `reasoning.encrypted_content`，所以 implementation 要 assert exact wire payload，不可靠 generic chat body forwarding。

## Risk Register

| Risk | Mitigation |
|---|---|
| Mapper silently forwards unsupported fields | Explicit allowlist + diagnostics tests。 |
| Instruction role semantics 被 flatten 錯 | Tests for `system`、`developer`、mixed ordering、non-leading behavior。 |
| Existing OpenAI chat behavior regresses | Add regression test around `internal/provider/openai` or route selection。 |
| Private Codex backend shape drifts | Keep mapper isolated and fixture-tested。 |
| Token/credential details leak | Mapper 不接收 credential object；add sentinel leakage test。 |

## Verification Commands

```bash
go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Test.*Codex.*Payload|Test.*ChatGPTCodex.*|Test.*OpenAI.*Chat' -count=1
go test ./internal/model/... ./internal/provider/... ./internal/proxy/handler/... -count=1
go tool golangci-lint run ./internal/provider/... ./internal/proxy/handler/...
git diff --check origin/main...HEAD
```

## Todo Gate Status

Todo planning complete 的條件：`spec.md`、`plan.md`、`tasks.md`、`research.md`、`data-model.md`、`quickstart.md`、`contracts/`、`checklists/`、`analyze.md` committed to docs-only draft PR。Implementation blocked until Linear moves to `In Progress`。
