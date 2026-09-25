# Research: HO-1289 chat to Codex Responses payload

## Repo Reality

### Existing chat request model

`internal/model/request.go` defines `ChatCompletionRequest`，包含 OpenAI-compatible fields：`model`、`messages`、generation parameters、tools、response format、streaming options、metadata、modalities、prompt-template fields，以及 unknown JSON fields 的 `ExtraParams`。

**Decision**: implementation 要用 allowlist mapper，不直接把 `ExtraParams` forward 到 Codex backend payload。

### Existing OpenAI provider transform

`internal/provider/openai/openai.go` builds normal Platform chat body with `model` and `messages`，並 post to `baseURL + "/chat/completions"`。

**Decision**: 不 reuse `transformRequestBody` for Codex backend，因為它刻意輸出 legacy chat-completions shape。

### Existing Responses handlers

`internal/proxy/handler/responses.go` and `responses_ext.go` proxy `/v1/responses` route family to OpenAI-compatible upstream endpoints。

**Decision**: HO-1289 不改 `/v1/responses` proxying；它只替 explicit HO-1288 backend transport map chat-completions input。

### HO-1288 merged dependency

HO-1288 is merged as PR #162. Current `origin/main` includes:

- `internal/provider/chatgptcodex/transport.go`
- `internal/proxy/handler/chatgpt_codex_backend.go`
- `internal/proxy/handler/chatgpt_codex_backend_test.go`

`chatgptcodex.Transport.BuildRequest` already creates the HTTP POST to `chatgpt.com/backend-api/codex/responses` with bearer/account/originator headers, but its body still marshals `input: req.Messages` and `stream: req.IsStreaming()` directly. The handler currently rejects streaming with `unsupported_streaming` before upstream.

**Decision**: HO-1289 should replace the `BuildRequest` body construction with a mapper callable inside `internal/provider/chatgptcodex`；不 independently own HTTP headers、credential resolution、upstream retries、or response adaptation。

## External Evidence

### OpenAI docs / Context7

Context7 `/openai/openai-go` documentation shows Responses creation uses `Responses.New` with `Input` and `Model`。這支持 mapper target `input`，而不是 legacy chat `messages`。

Official OpenAI docs search found public Responses API surface, but did not find official documentation for `chatgpt.com/backend-api/codex/responses`、`ChatGPT-Account-Id`、or `originator`。所以 private backend body contract 必須 isolated behind tests。

### GitHub real-world evidence

`grep-app-cli` against `openai/codex` found:

- `ResponseInputItem::Message { role, content, phase }`
- `ContentItem::InputText { text }`
- `ContentItem::InputImage { image_url, detail }`
- Codex tests asserting requests to `/api/codex/responses`
- Codex bearer auth provider adding `Authorization` and `ChatGPT-Account-ID`

**Decision**: use typed content items (`input_text`, `input_image`) and exact JSON fixture tests。

## Decisions

### D1 - Allowlist, not passthrough

Unknown chat fields and unsupported known fields must not be forwarded to the backend。它們應該 produce structured diagnostic or explicit error。

### D2 - Instructions strategy

Leading `system` and `developer` messages map to a top-level `instructions` string。Preserve original order and role-label each segment。

### D3 - Conversation input strategy

User and assistant messages map to `input` message items。Text content becomes `input_text`；supported image content becomes `input_image`；unsupported content returns diagnostics。

### D4 - Model normalization

Model mapping should be table-driven and tested。Mapper should accept the already-resolved model from Tianji route selection and normalize only route prefixes that are not part of backend model ID。

### D5 - No transport side effects

Mapper returns payload plus diagnostics。HTTP URL、headers、credential ordering、rate-limit failover、response adaptation stay in merged HO-1288 transport scope。
