# Specification: HO-1331 Codex model catalog schema for `/models`

## Scope

讓 Tianji 的 `GET /models` 與 `GET /v1/models` 在維持 OpenAI-compatible `object:list` + `data[]` 回應的同時，也提供 Codex CLI custom provider model refresh 可 decode 的 top-level `models: []` catalog。

本 issue owns Tianji server-side model-list response schema 與 runtime metadata conversion。它不修改 Codex CLI，不新增第二份 static catalog，不改 OpenAI subscription credential refresh，也不要求 operator 繼續使用 temporary `model_catalog_json` workaround。

## Problem

目前 true `codex exec` 對 Tianji `/v1/responses` 主流程已可完成，但 Codex custom provider refresh model list 時會先打同一個 provider `base_url` 的 `GET /models`，並用 Codex `ModelsResponse { models: Vec<ModelInfo> }` decode。

Tianji 現況：

- `internal/proxy/server.go` routes both `/models` and `/v1/models` to `Handlers.ListModels`。
- `internal/proxy/handler/handler.go` `ListModels` 只輸出 OpenAI-compatible shape：`{"object":"list","data":[{"id":...,"object":"model","owned_by":"tianji"}]}`。
- `internal/proxy/handler/runtime_model_source.go` 已提供正確 runtime source of truth：YAML `ModelList` + DB `ProxyModelTable` 合併成 `[]config.ModelConfig`。
- `config.ModelConfig.ModelInfo` 目前只保留 Tianji/LiteLLM pricing/capability metadata 的小集合，沒有 Codex `ModelInfo` full schema。

Root cause chain: Codex custom provider refresh expects top-level `models` with full Codex `ModelInfo` fields, but Tianji `/models` only returns OpenAI-compatible `data[]`; therefore Codex decodes response body 時缺少 required `models` field，refresh 在 route/credential 之前就失敗。

## User Stories

### US1 - Codex refresh can decode Tianji `/models` (P1)

作為 Codex custom provider 使用者，我要 Codex 打 Tianji `GET /models` 時能 decode `ModelsResponse.models[]`，不再看到 `missing field models`。

**Independent Test**: handler test call `GET /models`，decode response into a local mirror of Codex `ModelsResponse` required fields，current main 應因沒有 `models` 失敗，實作後通過。

### US2 - Existing OpenAI-compatible clients keep working (P1)

作為現有 OpenAI-compatible client，我要 `GET /models` 與 `GET /v1/models` 仍保留 `object:"list"` 與 `data[]` model entries。

**Independent Test**: existing and new regression tests assert response still contains `object:"list"` and `data[].id/object/owned_by` for YAML and DB-managed models.

### US3 - Runtime source is single and DB-managed models are included (P1)

作為 operator，我要 Codex catalog 顯示與 Tianji routing/runtime 一致的 model list，包括 DB-managed `openai/*` refresh 後的 rows。

**Independent Test**: test uses `runtimeModelSourceTestHandlers` with DB `ProxyModelTable` row, then asserts both `data[]` and `models[]` include the same visible model alias.

### US4 - Hidden aliases stay hidden from both shapes (P2)

作為 router maintainer，我要 `Router.ModelGroupAlias().Hidden` filtered aliases 不會只從 OpenAI shape 隱藏，卻洩漏到 Codex `models[]`。

**Independent Test**: hidden alias test asserts neither `data[]` nor `models[]` exposes hidden alias.

### US5 - Unsupported Tianji metadata has explicit defaults (P2)

作為 maintainer，我要 Tianji 缺少 Codex-only metadata 時有明確 converter defaults，而不是 hardcode per-model catalog 或 output thin `slug` only payload。

**Independent Test**: converter unit test asserts generated Codex model has required fields such as `slug`, `display_name`, `supported_reasoning_levels`, `shell_type`, `visibility`, `supported_in_api`, `priority`, `base_instructions`, `truncation_policy`, and tool capability defaults.

## Functional Requirements

- **FR-001**: `GET /models` MUST return top-level `models: []` compatible with Codex `ModelsResponse`.
- **FR-002**: `GET /v1/models` MUST return the same dual-shape response unless a future explicit compatibility split is approved.
- **FR-003**: Response MUST continue including OpenAI-compatible `object:"list"` and `data[]`.
- **FR-004**: `data[]` and `models[]` MUST be built from the same filtered `[]config.ModelConfig` returned by `runtimeModelList()`.
- **FR-005**: DB-managed models loaded through `RuntimeModelSource` MUST appear in both shapes after model refresh.
- **FR-006**: Router hidden aliases MUST be excluded from both shapes.
- **FR-007**: Codex converter MUST be a single conversion layer from `config.ModelConfig` to Tianji-owned Codex model metadata, not a second static catalog.
- **FR-008**: Converter MUST populate all required Codex `ModelInfo` fields needed by current Codex decode.
- **FR-009**: Unsupported or missing Codex-only capabilities MUST use explicit Tianji defaults in code and tests.
- **FR-010**: Converter MUST use `model_info.max_input_tokens`, `max_tokens`, or safe fallback values for context/truncation metadata where available.
- **FR-011**: Converter MUST keep pricing and sensitive credential data out of `/models` response.
- **FR-012**: Regression coverage MUST prove both OpenAI-compatible shape preservation and Codex `ModelsResponse` decode.
- **FR-013**: Live verification after implementation MUST prove true Codex model refresh no longer logs `missing field models`.

## Non-Goals

- 不修改 Codex CLI source。
- 不新增 independent static Codex catalog。
- 不只回 `models:[{"slug":"..."}]`。
- 不改 `/v1/responses` payload or WebSocket behavior from HO-1319/HO-1321。
- 不改 OpenAI subscription credential CRUD、OAuth refresh、或 Tianji model update UI。
- 不暴露 API key、subscription credential id、bearer token、refresh token 到 catalog。

## Success Criteria

- **SC-001**: A RED-first test proves current main lacks Codex `models` and fails decode.
- **SC-002**: After implementation, `GET /models` and `GET /v1/models` include both OpenAI-compatible `data[]` and Codex-compatible `models[]`.
- **SC-003**: DB-managed `openai/*` appears in both shapes using `runtimeModelList()` source.
- **SC-004**: Codex model list decode test passes without temporary `model_catalog_json`.
- **SC-005**: Existing `/models` OpenAI-compatible behavior remains green.
- **SC-006**: Live `codex exec` model refresh against Tianji no longer reports `missing field models`.

## Open Questions

None for Todo scope. Implementation phase may tune exact Tianji defaults for Codex-only capability fields, but must keep the defaults explicit and covered by tests.
