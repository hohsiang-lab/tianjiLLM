# Implementation Plan: HO-1331 Codex model catalog schema

## Goal

Extend Tianji `ListModels` so one runtime model source emits two compatible response shapes:

- OpenAI-compatible: `object:"list"` + `data[]`
- Codex-compatible: top-level `models: []` matching Codex `ModelsResponse`

The implementation must eliminate Codex custom-provider `missing field models` refresh failures without requiring operator-side `model_catalog_json`.

## Constraints

- Todo phase is docs-only. No production code in this PR.
- Implementation must be RED-test-first because the issue is labeled Bug and the broken behavior is reproducible at schema decode.
- Keep `runtimeModelList()` as source of truth; no second catalog.
- Keep `/models` credential-free and safe for existing clients.
- Preserve current `/models` and `/v1/models` route compatibility.

## Repo Reality

- `internal/proxy/handler/handler.go`
  - `ListModels` currently builds hidden alias set from `h.Router.ModelGroupAlias()`.
  - It calls `h.runtimeModelList(r.Context())`.
  - It returns only `object` and `data`.
- `internal/proxy/server.go`
  - Routes both `/models` and `/v1/models` to the same handler.
- `internal/proxy/handler/runtime_model_source.go`
  - `RuntimeModelSource` merges YAML `config.ProxyConfig.ModelList` and DB `ProxyModelTable` rows.
  - DB rows are converted to `config.ModelConfig` by `proxyModelToConfig`.
  - `applyDBModelInfo` preserves limited `model_info` fields: `id`, `mode`, token limits, and token costs.
- `internal/config/config.go`
  - `ModelConfig` has `ModelName`, `TianjiParams`, optional `ModelInfo`, tags, and access control.
  - `ModelInfo` does not mirror Codex `ModelInfo`; a Tianji-owned converter type is needed.
- `internal/proxy/handler/runtime_model_source_test.go`
  - Already covers `ListModelsIncludesDBManagedModel` for OpenAI-compatible body.
  - It is the right home for runtime-source list-model regression tests.

## External Protocol Evidence

- Official Codex advanced configuration docs describe custom `model_providers.<id>` with `base_url` and `wire_api = "responses"`; the same provider base URL is used by Codex provider-owned model refresh.
- `openai/codex` `model-provider/src/models_endpoint.rs` uses `MODELS_ENDPOINT = "/models"` for provider-owned model refresh.
- `openai/codex` `protocol/src/openai_models.rs` defines `ModelsResponse { models: Vec<ModelInfo> }`.
- Current Codex `ModelInfo` requires many fields beyond `slug`, including display name, reasoning levels, shell/tool capability fields, visibility, API support, priority, base instructions, truncation policy, and parallel-tool flags.
- OpenAI API docs still document model listing as an OpenAI-compatible model list surface for general API clients, so Tianji must preserve `object/list + data[]`.

Sources:

- <https://developers.openai.com/codex/config-advanced>
- <https://developers.openai.com/api/docs/models>
- <https://raw.githubusercontent.com/openai/codex/main/codex-rs/model-provider/src/models_endpoint.rs>
- <https://raw.githubusercontent.com/openai/codex/main/codex-rs/protocol/src/openai_models.rs>

## Proposed Design

### 1. Add Tianji-owned response structs

Create explicit response structs near `ListModels` or in a small helper file under `internal/proxy/handler`:

- `openAIModelListEntry`
- `codexModelInfo`
- `codexReasoningEffortPreset`
- small enum/string fields for Codex values Tianji emits
- `listModelsResponse` with `Object`, `Data`, and `Models`

Use JSON tags that match current Codex schema. Keep the type local to Tianji; do not import Codex crates or generated schemas.

### 2. Extract filtered runtime model list helper

Refactor `ListModels` into:

- collect hidden alias map
- filter runtime models once
- build OpenAI entries from the filtered list
- build Codex entries from the same filtered list

This avoids drift between `data[]` and `models[]`.

### 3. Convert `config.ModelConfig` to Codex model defaults

Add `codexModelInfoFromConfig(model config.ModelConfig, priority int) codexModelInfo`.

Default strategy:

- `slug`: `ModelName`
- `display_name`: `ModelInfo.ID` if meaningful, else `ModelName`
- `description`: mention Tianji runtime model alias, with upstream model when non-sensitive
- `default_reasoning_level`: `medium` or nil based on conservative decode behavior
- `supported_reasoning_levels`: include standard `low`, `medium`, `high`, `xhigh` descriptions unless future metadata narrows support
- `shell_type`: `default`
- `visibility`: `list`
- `supported_in_api`: true
- `priority`: stable index order from filtered runtime model list
- `base_instructions`: empty string or Tianji generic default
- `truncation_policy`: derive from `ModelInfo.MaxInputTokens`, `MaxTokens`, or fallback byte/token policy
- `context_window` / `max_context_window`: derive from available token limit fields when present
- `supports_parallel_tool_calls`: true as a Tianji default unless provider metadata later says otherwise
- `input_modalities`: text by default; include image only if Tianji later has source metadata

Keep all unsupported Codex-only fields explicit and tested: empty arrays, nil objects, false booleans, or documented defaults.

### 4. Guard sensitive fields

Do not include:

- `TianjiParams.APIKey`
- `OpenAISubscriptionCredentialIDs`
- bearer tokens
- refresh tokens
- raw `tianji_params`
- raw `model_info` JSON

Only expose alias/capability metadata.

### 5. Tests

Add RED-first tests before implementation:

- `TestListModelsIncludesCodexModelsResponse`
- `TestListModelsCodexResponseUsesRuntimeDBManagedModels`
- `TestListModelsPreservesOpenAICompatibleShape`
- `TestListModelsHiddenAliasesExcludedFromCodexModels`
- `TestCodexModelInfoFromConfigUsesExplicitDefaults`

Expected commands:

```bash
go test ./internal/proxy/handler -run 'ListModels|CodexModel' -count=1
go test ./internal/proxy/handler -run 'RuntimeModelSource|ListModels' -count=1
git diff --check
```

Live verification after implementation:

```bash
codex exec --json 'say OK'
```

Configured against `https://tianji.hohsiang.com.tw/v1` and `openai/gpt-5.5`; acceptance requires model refresh logs not to contain `missing field models`.

## Plan Review

- Repo reality checked first: current `ListModels` lacks `models[]`; `runtimeModelList()` already owns merged YAML/DB source.
- Official OpenAI docs checked: Codex custom provider config uses provider `base_url`; API model list remains an OpenAI-compatible surface.
- `openai/codex` source checked: provider-owned refresh fetches `/models` and decodes `ModelsResponse { models }`.
- No owner input needed for Todo scope.
