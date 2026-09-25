# Data Model: Runtime routes consume UI DB-managed models

## Existing Table: `ProxyModelTable`

| Field | Current Type | Runtime Meaning |
| --- | --- | --- |
| `model_id` | string | DB primary key and management identifier |
| `model_name` | string | Runtime model alias requested by clients |
| `tianji_params` | JSONB bytes | Provider config equivalent to `config.TianjiParams` |
| `model_info` | JSONB bytes | Optional metadata equivalent to `config.ModelInfo`; legacy access-control keys are ignored |
| `created_by` / `updated_by` | string | Audit metadata, not runtime routing input |
| `created_at` / `updated_at` | timestamptz | Audit and cache invalidation evidence |

## Existing Config Types

### `config.ModelConfig`

Runtime-compatible shape:

- `ModelName`
- `TianjiParams`
- `ModelInfo`
- `Tags`

### `config.TianjiParams`

Current runtime-relevant fields include:

- `Model`
- `APIKey`
- `APIBase`
- `APIVersion`
- `OpenAISubscriptionCredentialIDs`
- `TPM`
- `RPM`
- `Timeout`
- `Region`
- auto-router fields
- `Overflow`

## New Concept: Runtime Model Source

Implementation should introduce a small boundary with responsibilities equivalent to:

```go
type RuntimeModelSource interface {
    ListModelConfigs(ctx context.Context) []config.ModelConfig
    FindModelConfig(ctx context.Context, modelName string) (*config.ModelConfig, string)
    Router(ctx context.Context) *router.Router
    Refresh(ctx context.Context) error
}
```

The exact interface can differ, but the source must be singular: listing, routing, provider resolution, and discovery must not each invent their own DB/config merge.

## DB Row Conversion

### `tianji_params`

Decode `ProxyModelTable.TianjiParams` into `config.TianjiParams` using structured JSON handling. Requirements:

- `model` is required for runtime routing.
- `api_key`, `api_base`, `api_version`, `openai_subscription_credential_ids`, rate limits, and overflow fields must retain current semantics.
- Malformed JSON must make the DB row invalid for runtime and produce a sanitized error/log.

### `model_info`

Decode `ProxyModelTable.ModelInfo` into:

- optional `config.ModelInfo` values where present;
- ignore legacy `access_control` data. Model-level access control is removed,
  so every runtime model is public.

## Merge Snapshot

Recommended immutable snapshot:

```go
type RuntimeModelSnapshot struct {
    Models []config.ModelConfig
    Router *router.Router
    LoadedAt time.Time
}
```

Snapshot rules:

- `Models` is never mutated after publication.
- `Router` is built from the same `Models`.
- Exact lookup and wildcard lookup read from the same `Models`.
- Refresh swaps the whole snapshot atomically or under one write lock.

## Duplicate Names

Rules:

1. Index DB rows by `model_name`.
2. Include valid DB rows first.
3. Add YAML config rows only when their `model_name` is not present in DB.
4. Return one model per exact name in `/v1/models`.

## Wildcards

Preferred rule:

- Exact matches always win.
- Wildcard candidates from both DB and YAML participate in the existing specificity ordering.
- Duplicate wildcard pattern strings follow DB-over-YAML precedence.

Minimum fallback:

- DB create/update rejects `model_name` containing `*`.
- YAML wildcard behavior remains unchanged.
- Tests assert the rejection.

## Refresh Semantics

Preferred behavior:

- Initial startup loads YAML plus DB rows if DB exists.
- `/model/new`, `/model/update`, `/model/delete`, and corresponding UI handlers call `Refresh(ctx)` after successful DB mutation.
- Refresh failure returns or displays an explicit warning while preserving DB mutation result semantics chosen by implementation.

Allowed fallback only if hot refresh is unsafe:

- Runtime behavior explicitly documents restart-required behavior in UI/API response.
- Tests assert that the response exposes restart-required status.
