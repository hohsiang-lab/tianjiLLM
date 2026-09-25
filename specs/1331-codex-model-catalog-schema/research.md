# Research: HO-1331 Codex model catalog schema

## Question 1: What endpoint does Codex custom provider use for model refresh?

**Finding**: `openai/codex` provider-owned refresh uses a fixed `/models` endpoint relative to the configured provider base URL. The issue is therefore not solved by changing Tianji `/v1/responses` only.

**Evidence**:

- `openai/codex` `codex-rs/model-provider/src/models_endpoint.rs` defines `MODELS_ENDPOINT = "/models"` and calls `ModelsClient.list_models`.
- Official Codex configuration docs define custom providers with `base_url` and `wire_api`; there is no separate `models_base_url` in the fetched advanced config page.

**Decision**: Tianji must make its existing `/models` route Codex-decodable.

## Question 2: What schema does Codex decode?

**Finding**: Codex decodes top-level `ModelsResponse { models: Vec<ModelInfo> }`. Current Tianji response lacks `models`, so decode fails before model routing or credential selection.

**Evidence**:

- `openai/codex` `codex-rs/protocol/src/openai_models.rs` defines `ModelsResponse` with `models: Vec<ModelInfo>`.
- Current Codex `ModelInfo` includes required fields beyond `slug`, including `display_name`, `supported_reasoning_levels`, `shell_type`, `visibility`, `supported_in_api`, `priority`, `base_instructions`, `truncation_policy`, and `supports_parallel_tool_calls`.

**Decision**: Do not emit a thin `models:[{slug:...}]`; create a complete-enough Tianji converter.

## Question 3: Should Tianji replace OpenAI-compatible `data[]`?

**Finding**: No. Existing clients and current tests expect OpenAI-compatible model list shape. OpenAI API model docs still present model listing as an API model surface, and Tianji already exposes `/models` and `/v1/models` as compatible endpoints.

**Decision**: Emit a dual-shape response: keep `object/list + data[]`, add `models[]`.

## Question 4: What is the source of truth?

**Finding**: `runtimeModelList()` already merges YAML config and DB-managed `ProxyModelTable` rows through `RuntimeModelSource`. This is also the source used by routing and existing model list output.

**Decision**: Both `data[]` and `models[]` must be generated from the same filtered runtime `[]config.ModelConfig` slice.

## Question 5: How should missing Codex-only metadata be handled?

**Finding**: Tianji `config.ModelInfo` contains limited metadata, mostly id/mode/pricing/token limits. It cannot supply every Codex UI capability field.

**Decision**: Fill missing Codex fields with explicit Tianji defaults in the converter and test those defaults. Prefer conservative text/API/tool defaults over pretending Tianji has richer per-model metadata.

## Question 6: What must not be exposed?

**Finding**: `config.ModelConfig.TianjiParams` may include API keys, subscription credential IDs, upstream base URLs, and provider routing details. Codex catalog only needs model metadata.

**Decision**: Catalog response must never expose secrets, credential IDs, bearer/refresh tokens, or raw params.
