# Research: Runtime routes consume UI DB-managed models

## Repo Evidence

### Startup router is YAML-only

`cmd/tianji/main.go` initializes the runtime router with `router.New(cfg.ModelList, routeStrategy, settings)`. DB rows are not loaded into that constructor in the current code.

### Model listing is YAML-only

`internal/proxy/handler/handler.go` `ListModels` builds `models := make(... len(h.Config.ModelList))` and iterates only `h.Config.ModelList`.

### Runtime config lookup is YAML-only

`internal/proxy/handler/handler.go` `findModelConfig` checks exact names and wildcard candidates by iterating only `h.Config.ModelList`.

### Chat uses router or config fallback

`internal/proxy/handler/chat.go` `resolveProviderRoute` first uses `h.Router.Route(ctx, req.Model, req)`. That router was built from YAML `cfg.ModelList`. If no router exists, the fallback calls `resolveProviderFromConfigRouteWithContext`, which also uses `findModelConfig`.

### OpenAI-compatible resource routes can bypass router

`internal/proxy/handler/forward.go` `resolveProviderBaseURLWithContext` uses `findModelConfig(modelName)` for explicit model names, then scans `h.Config.ModelList` for the first OpenAI-compatible provider when no model is supplied.

### Discovery excludes DB by comment and behavior

`internal/proxy/handler/discovery.go` `ModelGroupInfo` calls `h.Router.ListModelGroups(r.Context())` and contains a comment that DB-managed models are not part of the router.

### UI and management APIs persist DB rows

`internal/ui/handler_models.go` reads/writes `ProxyModelTable` rows and appends YAML config models for display. `internal/proxy/handler/model_mgmt.go` exposes `/model/new|info|update|delete` against the same DB table.

### Key/team UI already sees DB model names

`internal/ui/handler_keys.go` `loadAvailableModelNames` deduplicates DB model names first, then YAML config model names. This proves UI restriction selectors can expose DB model names even though runtime routing ignores them.

### Existing schema can represent runtime params

`internal/db/models.go` `ProxyModelTable` contains `ModelName`, `TianjiParams`, and `ModelInfo`. `internal/config/config.go` already has a typed `config.TianjiParams` that includes `Model`, `APIKey`, `APIBase`, `OpenAISubscriptionCredentialIDs`, rate limits, and overflow fields.

## External / Official Evidence

### Go synchronization

`go doc sync/atomic.Value` confirms `Value` supports atomic `Load` and `Store` of a consistently typed value and zero-value `Load` returns nil. This is suitable if implementation chooses immutable snapshot refresh for merged model source.

`go doc sync.RWMutex` confirms reader/writer locking supports many readers or one writer, with the zero value unlocked. This is suitable if implementation chooses a mutable cache guarded by `RWMutex`.

### Context7 evidence

Context7 `/golang/go` confirms `sync/atomic` is the standard package for atomic shared-variable primitives and documents atomic load/store operations.

Context7 `/go-chi/docs` confirms `chi.URLParam` is the standard way to read route params such as `{model}` and that middleware can place request-scoped data on `context.Context`; this matters for Azure-compatible `/engines/{model}` and `/openai/deployments/{model}` paths.

### GitHub prior art

`grep.app` public code search for `atomic.Value` showed large Go projects using it for read-mostly runtime state snapshots, including Kubernetes peer-proxy caches and OpenTelemetry delegate pointers. This supports an immutable snapshot approach when the merged model source needs low-overhead reads and explicit refresh writes.

## Design Constraints

- Runtime model source reads happen on every model request; cache/snapshot reads should be cheap.
- DB refresh must not happen inside every route by default unless performance and failure semantics are explicit.
- `ProxyModelTable.tianji_params` is JSONB, while YAML config uses `yaml` tags; conversion must use structured JSON into a compatible local struct rather than ad-hoc string parsing.
- Existing router behavior handles wildcard specificity, access control, fallback, and strategy state. Any new merged source should reuse or adapt that behavior instead of duplicating routing logic.
- DB outage must not break YAML-only deployments.

## Chosen Direction for Planning

Implement a runtime model source boundary owned by handlers/server startup:

1. Convert DB rows into `config.ModelConfig`-compatible values using structured JSON unmarshalling of `tianji_params` and `model_info`.
2. Merge DB rows with YAML config models using deterministic precedence.
3. Build or refresh a router from the merged list.
4. Make listing, `findModelConfig`, `resolveProviderBaseURL`, and discovery read from the same source.
5. Trigger refresh after model create/update/delete; if hot refresh proves unsafe during implementation, document and surface restart-required behavior, but tests must lock the chosen behavior.

## Open Questions Resolved by Spec

- **Should DB or YAML win on duplicate names?** DB wins when DB is available, matching current UI behavior that treats DB rows as authoritative and appends YAML rows only for names missing from DB.
- **Should wildcard DB names be supported?** Preferred implementation supports them through existing wildcard specificity. If implementation discovers UI/API validation currently cannot safely support wildcard rows, it must reject wildcard DB names and update tests/docs before implementation proceeds.
- **Should resource-route fallback include DB models?** Yes, but the exact no-model fallback order must be tested and documented. Planning default: merged source order is DB-precedence by name with stable ordering, then first OpenAI-compatible candidate from that merged list.
