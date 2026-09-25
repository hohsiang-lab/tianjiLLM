# Data Model: HO-2370 Mirror Codex subscription model catalog

## RuntimeAlias

- **Source**: `config.ModelConfig` from `RuntimeModelSource`.
- **Key fields**:
  - `model_name`: Tianji alias exposed to clients.
  - `tianji_params.model`: Upstream model slug or wildcard target.
  - `openai_subscription_credential_ids`: Eligible subscription credential IDs.
  - `openai_subscription_transport`: Must be `chatgpt_codex_backend` for catalog enrichment.
  - `model_info`: Existing fallback metadata.
- **Rules**:
  - Alias remains the callable availability boundary.
  - Hidden aliases are filtered before both `data[]` and `models[]`.

## SubscriptionCredential

- **Source**: Existing credential store and resolver.
- **Key fields**:
  - `credential_id`: Safe identifier for cache keying.
  - access token/account ID: used only for upstream request, never persisted in catalog cache.
  - credential status: connected/refreshed/disabled/replaced events affect cache invalidation.
- **Rules**:
  - Secrets never appear in catalog response, persisted cache body, logs, or degraded status.

## UpstreamCodexCatalogEntry

- **Source**: Stock Codex provider-owned `/models` response.
- **Key fields**:
  - `slug`, `display_name`, `description`
  - `context_window`, `max_context_window`, `effective_context_window_percent`
  - `auto_compact_token_limit` including explicit null
  - `truncation_policy`
  - `default_reasoning_level`, `supported_reasoning_levels`
  - `input_modalities`
  - `additional_speed_tiers`, `service_tiers`, `default_service_tier`
  - remaining HO-1331 Codex `ModelInfo` fields
- **Rules**:
  - Preserve future non-empty reasoning effort values.
  - Preserve explicit upstream nulls where the response contract allows them.
  - Reject malformed entries that cannot identify a model slug.

## CatalogCacheEntry

- **Scope**: Credential ID or explicit entitlement grouping.
- **Key fields**:
  - `cache_key`
  - sanitized `models[]`
  - `fetched_at`
  - `expires_at`
  - `last_error_reason`
  - `degraded`
  - `backoff_until`
- **Rules**:
  - Fresh entry wins.
  - On refresh failure, last-known-good may still serve with degraded status.
  - No secret-bearing fields are stored.

## EnrichedCodexModelInfo

- **Source**: Runtime alias overlaid with upstream catalog entry.
- **Key fields**:
  - `slug`: Tianji alias.
  - `display_name`: Upstream display name unless alias-specific display should be retained.
  - metadata fields from upstream catalog entry.
  - Tianji defaults only for missing optional fields.
- **Rules**:
  - Must remain compatible with HO-1331 `models[]`.
  - Must keep `data[]` generated from the same runtime alias list.
