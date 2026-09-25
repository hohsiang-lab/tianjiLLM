# Contract: Codex-compatible model catalog

## Endpoint

```http
GET /models
GET /v1/models
```

Both endpoints are handled by Tianji `ListModels`.

## Response Shape

```json
{
  "object": "list",
  "data": [
    {
      "id": "openai/*",
      "object": "model",
      "owned_by": "tianji"
    }
  ],
  "models": [
    {
      "slug": "openai/*",
      "display_name": "openai/*",
      "description": "Tianji runtime model openai/*",
      "default_reasoning_level": "medium",
      "supported_reasoning_levels": [
        { "effort": "low", "description": "Low reasoning effort" },
        { "effort": "medium", "description": "Medium reasoning effort" },
        { "effort": "high", "description": "High reasoning effort" }
      ],
      "shell_type": "default",
      "visibility": "list",
      "supported_in_api": true,
      "priority": 0,
      "additional_speed_tiers": [],
      "service_tiers": [],
      "availability_nux": null,
      "upgrade": null,
      "base_instructions": "",
      "model_messages": null,
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
  ]
}
```

The exact default values may be tuned during implementation, but the field presence and type compatibility are part of the contract.

## Source-of-Truth Invariant

For every visible runtime model:

- one `data[]` entry exists with `id == ModelConfig.ModelName`;
- one `models[]` entry exists with `slug == ModelConfig.ModelName`;
- both entries are created from the same filtered `ModelConfig`.

## Hidden Alias Invariant

If router alias metadata marks a model alias hidden:

- it is absent from `data[]`;
- it is absent from `models[]`.

## Security Invariant

The response must not include:

- API keys;
- OpenAI subscription credential IDs;
- bearer tokens;
- refresh tokens;
- raw `tianji_params`;
- raw `model_info`;
- upstream authorization headers.
