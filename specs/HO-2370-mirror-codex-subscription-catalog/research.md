# Research: HO-2370 Mirror Codex subscription model catalog

## Decision: Reuse stock Codex provider-owned `/models` path

**Decision**: Implement refresh against the provider-owned `/models` catalog path used by Codex model refresh, authenticated with a resolved subscription credential for the ChatGPT Codex backend.

**Rationale**: Stock Codex source defines `MODELS_ENDPOINT = "/models"` and calls `ModelsClient.list_models(...)` with provider auth. HO-1331 already made Tianji expose a compatible top-level `models[]` response because Codex decodes provider-owned `/models`.

**Alternatives considered**:

- OpenAI public API `/v1/models`: rejected because it does not expose Codex app-server fields like `effective_context_window_percent`, `auto_compact_token_limit`, service tiers, or Codex reasoning presets.
- Local static Tianji converter only: rejected because it caused this bug.

**Sources**:

- `internal/proxy/handler/list_models_catalog.go`
- `specs/1331-codex-model-catalog-schema/plan.md`
- `https://raw.githubusercontent.com/openai/codex/main/codex-rs/model-provider/src/models_endpoint.rs`

## Decision: Keep Tianji runtime aliases as availability boundary

**Decision**: Use upstream catalog only as metadata enrichment for configured Tianji runtime aliases. Do not expose upstream-only models.

**Rationale**: `RuntimeModelSource` already merges YAML and DB `ProxyModelTable` rows as Tianji routing source of truth. HO-1331 established `data[]` and `models[]` must come from the same filtered runtime list.

**Alternatives considered**:

- Mirror every upstream model into `/models`: rejected because it would advertise models Tianji cannot route.
- Store enriched metadata only in DB `model_info`: rejected because DB rows are currently empty for affected routes and manual metadata does not solve subscription rollout drift.

**Sources**:

- `internal/proxy/handler/runtime_model_source.go`
- `internal/proxy/handler/runtime_model_source_test.go`
- `specs/1331-codex-model-catalog-schema/spec.md`

## Decision: Model catalog cache should mirror usage-cache architecture

**Decision**: Add a credential-aware in-memory cache with persistent last-known-good read/write, TTL, backoff, and singleflight-style refresh behavior patterned after `OpenAISubscriptionCodexUsageCache`.

**Rationale**: Existing Codex usage code already scopes cache entries by credential ID, hydrates persisted values, supports TTL/backoff, and redacts sensitive credential data. Catalog refresh has the same operational shape.

**Alternatives considered**:

- Request upstream catalog on every `/models`: rejected by acceptance criteria and latency/reliability risk.
- One global catalog cache: rejected because subscription entitlements can differ by credential.

**Sources**:

- `internal/proxy/handler/openai_subscription_codex_usage.go`
- `internal/proxy/handler/openai_subscription_codex_usage_test.go`

## Decision: Preserve explicit upstream nulls and future enum values

**Decision**: Treat upstream metadata as authoritative when present, including `auto_compact_token_limit: null`, future reasoning effort strings such as `max` and `ultra`, text/image modalities, and service tiers.

**Rationale**: Current local synthesis hardcodes medium/low-high-xhigh/text-only/90% compaction. Stock Codex `ModelInfo` supports `ReasoningEffort::Max`, `ReasoningEffort::Ultra`, custom effort strings, text/image modalities, and service tier metadata.

**Alternatives considered**:

- Normalize reasoning to Tianji's old fixed set: rejected because it hides supported upstream capabilities.
- Recompute auto-compaction from context: rejected because upstream can explicitly report null or a different value.

**Sources**:

- `https://raw.githubusercontent.com/openai/codex/main/codex-rs/protocol/src/openai_models.rs`
- `https://developers.openai.com/api/docs/models`
- `https://github.com/openai/codex/issues/19319`
- `https://github.com/openai/codex/issues/30875`

## Decision: No new third-party Go library is required

**Decision**: Use existing Go standard library HTTP/JSON primitives and existing Tianji handler/store patterns. Add sqlc queries only if persistent catalog storage needs a new table or credential-info row access path.

**Rationale**: The repo already uses `net/http`, `encoding/json`, `singleflight`-style request coalescing for usage, and sqlc for database access. Adding a library for a simple authenticated JSON fetch/cache would violate the constitution's simplicity bias.

**Alternatives considered**:

- Introduce a cache library: rejected as unnecessary because the repo has a small in-memory cache pattern already.
- Hand-write SQL in Go: rejected by constitution Principle VII.

**Sources**:

- `.specify/memory/constitution.md`
- `internal/proxy/handler/openai_subscription_codex_usage.go`
- `internal/db/queries/`

## Open Questions Resolved

- **Patch or systemic fix?** Systemic fix. Linear explicitly requires "Required Long-Term Fix" and rejects hardcoding/DB backfill.
- **Should upstream catalog define availability?** No. Runtime routing remains boundary.
- **Can official public model docs be the source of exact Codex metadata?** No. Public docs confirm Sol/Terra/Luna and text/image support but not subscription app-server catalog fields; implementation must use stock Codex catalog response fixtures and live verification.
