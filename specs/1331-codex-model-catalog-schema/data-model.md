# Data Model: HO-1331 Codex model catalog schema

## Existing Source: `config.ModelConfig`

```go
type ModelConfig struct {
    ModelName    string
    TianjiParams TianjiParams
    ModelInfo    *ModelInfo
    Tags         []string
}
```

Role:

- canonical runtime alias and routing metadata;
- returned by `runtimeModelList()`;
- may come from YAML or DB `ProxyModelTable`.

Catalog usage:

- `ModelName` becomes OpenAI `data[].id` and Codex `models[].slug`;
- `ModelInfo` may supply display/context defaults;
- `TianjiParams` must not be serialized raw.
- Model-level access control is not part of the runtime contract; legacy
  `model_info.access_control` data is ignored and models are public.

## Existing Output: OpenAI-Compatible Model Entry

```json
{
  "id": "openai/*",
  "object": "model",
  "owned_by": "tianji"
}
```

Role:

- backward-compatible API shape for existing clients;
- remains in `data[]`.

## New Output: Combined Model List Response

```json
{
  "object": "list",
  "data": [],
  "models": []
}
```

Role:

- one handler response serving both OpenAI-compatible clients and Codex custom-provider refresh.

Invariant:

- `data[]` and `models[]` are built from the same filtered runtime model slice.

## New Output: Tianji Codex Model Info

Minimum required fields mirrored from current Codex schema:

```json
{
  "slug": "openai/*",
  "display_name": "openai/*",
  "description": "Tianji runtime model openai/*",
  "default_reasoning_level": "medium",
  "supported_reasoning_levels": [],
  "shell_type": "default",
  "visibility": "list",
  "supported_in_api": true,
  "priority": 0,
  "additional_speed_tiers": [],
  "service_tiers": [],
  "availability_nux": null,
  "upgrade": null,
  "base_instructions": "",
  "supports_reasoning_summaries": false,
  "default_reasoning_summary": "auto",
  "support_verbosity": false,
  "default_verbosity": null,
  "apply_patch_tool_type": null,
  "web_search_tool_type": "text",
  "truncation_policy": { "mode": "tokens", "limit": 128000 },
  "supports_parallel_tool_calls": true,
  "supports_image_detail_original": false,
  "context_window": 128000,
  "max_context_window": 128000,
  "auto_compact_token_limit": null,
  "effective_context_window_percent": 95,
  "experimental_supported_tools": [],
  "input_modalities": ["text"],
  "supports_search_tool": false
}
```

Exact defaults may be adjusted during implementation, but every required field must be explicit and covered by tests.

## Hidden Alias Filter

Source:

```go
h.Router.ModelGroupAlias()
```

Rule:

- if alias is hidden, exclude it from both `data[]` and `models[]`.

## Sensitive Fields

Never serialize:

- `TianjiParams.APIKey`
- `TianjiParams.OpenAISubscriptionCredentialIDs`
- bearer tokens
- refresh tokens
- raw `tianji_params`
- raw `model_info`
- upstream auth headers
