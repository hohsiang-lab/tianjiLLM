# Contract: Runtime Model Source

## Purpose

All runtime/discovery routes must consume one merged model source rather than splitting between DB UI models and YAML runtime models.

## Source Inputs

- YAML `[]config.ModelConfig` from `cfg.ModelList`.
- DB `[]db.ProxyModelTable` from `ListProxyModels(ctx)` when DB is available.

## Required Consumers

- `/v1/models` and bare `/models`
- `/v1/chat/completions` and bare `/chat/completions`
- `/v1/completions`, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/transcriptions`, `/v1/audio/speech`, `/v1/moderations`, `/v1/rerank`
- Azure-style `/engines/{model}/...` and `/openai/deployments/{model}/...`
- OpenAI-compatible resource helpers using `resolveProviderBaseURL`: files, batches, fine-tuning, rerank, and any same-helper endpoint
- OpenAI endpoint helpers using `resolveAssistantsUpstream` or `resolveOpenAIEndpointRouteForRequest`: assistants, threads, vector stores, responses, image edits, image variations, OCR, videos, containers, and related OpenAI pass-through routes
- Native routes `/v1/messages`, `/v1/messages/count_tokens`, and Gemini `/v1beta/models/{model}:...` where they rely on configured upstreams
- Anthropic batch pass-through routes
- `/model_group/info`

Each consumer must be covered by either code wiring to this source, an explicit unaffected classification backed by route code, or a follow-up issue if the path needs a separate implementation boundary.

## Behavior

### `ListModelConfigs(ctx)`

Returns merged model configs with DB-over-YAML exact-name precedence.

### `FindModelConfig(ctx, modelName)`

Exact match first, then wildcard match according to existing specificity rules.

Return values:

- matched `config.ModelConfig`
- resolved provider model string after wildcard substitution
- not-found error or nil result consistent with current helper style

### `Router(ctx)`

Returns a router built from the same merged model list. The router must not have a different model universe than `ListModelConfigs`.

### `Refresh(ctx)`

Reloads DB rows and rebuilds the snapshot/router. Must be called after successful model create/update/delete if hot reload is implemented.

## Error Contract

- Invalid DB JSON produces sanitized errors.
- Missing provider model (`tianji_params.model`) makes the DB row invalid.
- DB unavailable at startup falls back to YAML config and logs a warning.
- DB unavailable during refresh returns an error and keeps the previous valid snapshot.
- Error responses must not expose API keys, bearer tokens, encrypted credential blobs, or full raw JSON.

## Test Contract

- DB-only model appears in list.
- DB-only model routes chat completion.
- DB-only model routes one non-chat route.
- Duplicate DB/YAML name chooses DB.
- YAML-only model remains routable.
- Refresh or restart-required behavior is visible after create/update/delete.
- Route audit covers every path family listed in Required Consumers.
