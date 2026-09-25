# Contract: Codex subscription catalog enrichment

## Downstream `GET /models` and `GET /v1/models`

Tianji continues returning the HO-1331 dual shape:

```json
{
  "object": "list",
  "data": [
    {
      "id": "openai/gpt-5.6-sol",
      "object": "model",
      "owned_by": "tianji"
    }
  ],
  "models": [
    {
      "slug": "openai/gpt-5.6-sol",
      "display_name": "GPT-5.6 Sol",
      "context_window": 372000,
      "max_context_window": 372000,
      "effective_context_window_percent": 95,
      "auto_compact_token_limit": null,
      "truncation_policy": {
        "mode": "tokens",
        "limit": 10000
      },
      "input_modalities": ["text", "image"],
      "default_reasoning_level": "max",
      "supported_reasoning_levels": [
        {"effort": "medium", "description": "Balanced reasoning."},
        {"effort": "max", "description": "Maximum reasoning."}
      ],
      "additional_speed_tiers": ["fast"],
      "service_tiers": [
        {"id": "priority", "name": "Priority", "description": "Priority capacity"}
      ]
    }
  ]
}
```

## Upstream catalog request

- **Method**: `GET`
- **Path**: provider-owned `/models` resolved through the ChatGPT Codex backend base URL.
- **Auth**: resolved subscription access token and account ID headers, using existing credential resolution patterns.
- **Timeout**: bounded; implementation should not let downstream `/models` hang indefinitely.

## Cache behavior contract

- Fresh credential-aware cache entry serves without upstream call.
- Stale entry attempts refresh.
- Refresh failure with last-known-good entry serves cached metadata and records degraded status.
- Refresh failure without last-known-good entry falls back to static Tianji defaults.
- Cache/log payloads must not contain access tokens, refresh tokens, bearer headers, raw credential values, or raw credential bundles.

## Matching contract

- Match a runtime alias to upstream catalog by its configured upstream model slug.
- Emit the Tianji alias as the advertised `slug`.
- Do not emit upstream entries without routable Tianji aliases.
- If multiple credentials are configured for an alias, selection or aggregation semantics must be explicit in tests.
