# Implementation Plan: HO-2370 Mirror Codex subscription model catalog

**Branch**: `HO-2370-mirror-and-cache-openai-codex-subscription-model` | **Date**: 2026-07-10 | **Spec**: `specs/HO-2370-mirror-codex-subscription-catalog/spec.md`
**Input**: Linear HO-2370 and repo reality from `internal/proxy/handler/list_models_catalog.go`, `runtime_model_source.go`, and HO-1331 artifacts.

## Summary

Replace HO-1331's purely local Codex model metadata synthesis for subscription-backed runtime aliases with a credential-aware mirror/cache of the upstream Codex subscription catalog. `/models` remains built from Tianji runtime routes, but each routable subscription alias is enriched from the matching upstream catalog entry when a fresh or last-known-good catalog exists.

## Technical Context

**Language/Version**: Go, existing module `github.com/praxisllmlab/tianjiLLM`
**Primary Dependencies**: existing `net/http`, `encoding/json`, `context`, `time`, existing handler/cache/store patterns; no new third-party dependency planned
**Storage**: PostgreSQL through sqlc if persistent catalog storage is needed; in-memory cache in handler process
**Testing**: `go test` with existing `testify/require` and handler harnesses
**Target Platform**: Tianji server process on Linux/container deployments
**Project Type**: Go backend service
**Performance Goals**: `/models` must serve from fresh cache without upstream call per request; refresh timeout bounded similarly to Codex usage refresh
**Constraints**: Do not leak access/refresh tokens; preserve HO-1331 dual response shape; keep runtime aliases as availability boundary; follow failing-tests-first workflow
**Scale/Scope**: Subscription-backed Sol/Terra/Luna aliases and future Codex subscription models exposed through configured Tianji routes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Python-first reference**: No equivalent Python Tianji behavior found in this repo scan for subscription Codex catalog mirroring; document this as a Go-side extension driven by Codex/OpenAI upstream behavior.
- **Feature parity**: Preserve existing Tianji `/models` OpenAI-compatible behavior and HO-1331 Codex-compatible schema.
- **Research before build**: Repo reality, HO-1331 artifacts, official OpenAI docs, and stock Codex source were checked; decisions are in `research.md`.
- **Failing-tests-first**: Required tests are listed below and must be implemented/fail before production code.
- **Go best practices**: Plan uses existing handler interfaces and context-aware HTTP calls; no unnecessary new library.
- **No stale knowledge**: External Codex/OpenAI claims are sourced in `research.md`.
- **sqlc-first DB access**: Any persistent catalog query/table must be added through SQL files and `make generate`; no raw SQL in Go.

## Project Structure

### Documentation (this feature)

```text
specs/HO-2370-mirror-codex-subscription-catalog/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── analyze.md
├── contracts/
│   └── codex-subscription-catalog.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/provider/chatgptcodex/
├── catalog.go
└── catalog_test.go

internal/proxy/handler/
├── list_models_catalog.go
├── list_models_codex_test.go
├── openai_subscription_codex_catalog.go
├── openai_subscription_codex_catalog_test.go
├── openai_subscription_codex_usage.go
└── runtime_model_source_test.go

internal/db/
├── queries/
│   └── credential.sql or codex_catalog.sql
└── schema/
```

**Structure Decision**: Keep upstream HTTP parsing in `internal/provider/chatgptcodex` alongside usage fetching. Keep handler-owned credential resolution, cache selection, runtime alias enrichment, and model-list response assembly in `internal/proxy/handler`.

## Failing Tests

### User Story 1 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestListModelsSubscriptionCatalogEnrichesSolTerraLuna` | `internal/proxy/handler/list_models_codex_test.go` | Fixture-backed Sol/Terra/Luna aliases expose upstream `372000`, `95`, null auto-compact, `tokens/10000`, text/image modalities, reasoning defaults/levels, and service/speed tiers | US1 AS-1, AS-2 |
| `TestListModelsSubscriptionCatalogKeepsRuntimeRouteBoundary` | `internal/proxy/handler/list_models_codex_test.go` | Upstream-only catalog model is absent from `data[]` and `models[]` without a Tianji runtime alias | US1 AS-3 |
| `TestCodexModelInfoFromConfigPreservesExplicitUpstreamNullAutoCompact` | `internal/proxy/handler/list_models_codex_test.go` | Explicit upstream null does not become `context*90%` | Edge: null auto-compact |
| `TestCodexCatalogClientParsesReasoningModalitiesAndTiers` | `internal/provider/chatgptcodex/catalog_test.go` | Catalog parser accepts `max`, `ultra`, custom efforts, text/image modalities, and service tiers | Edge: future values |

### User Story 2 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestOpenAISubscriptionCodexCatalogUsesFreshCacheWithoutRefetch` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Repeated calls within TTL produce one upstream fetch and subsequent cache hits | US2 AS-1 |
| `TestOpenAISubscriptionCodexCatalogSeparatesCredentialEntitlements` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Two credentials with different fixtures do not share catalog guarantees unless explicit aggregate semantics are selected | US2 AS-2 |
| `TestOpenAISubscriptionCodexCatalogInvalidatesOnCredentialRefresh` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Credential refresh/disable/replacement causes affected catalog cache to refresh or invalidate | US2 AS-3 |
| `TestOpenAISubscriptionCodexCatalogCoalescesConcurrentRefreshes` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Concurrent stale-cache `/models` requests do not stampede upstream | Edge: concurrent TTL expiry |

### User Story 3 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestOpenAISubscriptionCodexCatalogServesLastKnownGoodOnRefreshFailure` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Failed refresh returns cached metadata plus degraded reason | US3 AS-1 |
| `TestListModelsSubscriptionCatalogFallsBackToStaticMetadataWithoutCache` | `internal/proxy/handler/list_models_codex_test.go` | No upstream/no cache keeps conservative `128000` fallback | US3 AS-2 |
| `TestOpenAISubscriptionCodexCatalogRedactsSecretsFromCacheAndLogs` | `internal/proxy/handler/openai_subscription_codex_catalog_test.go` | Persisted cache/loggable result excludes bearer/access/refresh tokens and raw credential bundles | US3 AS-3 |

### User Story 4 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestListModelsPreservesOpenAICompatibleShape` | `internal/proxy/handler/list_models_codex_test.go` | Existing `object:"list"` and `data[]` remain present | US4 AS-1 |
| `TestListModelsHiddenAliasesExcludedFromBothShapes` | `internal/proxy/handler/list_models_codex_test.go` | Hidden aliases are absent from both response shapes | US4 AS-2 |
| `TestRuntimeModelSource_ListModelsIncludesDBManagedModel` | `internal/proxy/handler/runtime_model_source_test.go` | DB-managed runtime aliases still appear from merged runtime source | US4 AS-3 |

### Verification Command

```bash
go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|ListModels|RuntimeModelSource' -count=1
git diff --check
```

## Phase 0: Research

Research completed in `research.md`. Key findings:

- Stock Codex uses provider-owned `/models` and decodes Codex `ModelInfo`.
- Public OpenAI Models docs confirm Sol/Terra/Luna and text/image support but do not expose full app-server catalog metadata.
- Existing Codex usage cache gives the closest Tianji pattern for credential-scoped TTL/backoff/persist behavior.
- HO-1331 dual-shape response must remain intact.

## Phase 1: Design

## Implementation Approach

- Add a ChatGPT Codex catalog client/parser in `internal/provider/chatgptcodex` for the provider-owned `/models` response.
- Add a credential-aware catalog cache in `internal/proxy/handler`, modeled on the existing Codex usage cache, with fresh/last-known-good/static fallback ordering.
- Enrich only visible runtime aliases from `RuntimeModelSource`; preserve HO-1331 `data[]` and `models[]` from the same filtered runtime model list.
- Keep all production implementation for the later In Progress scene; this Todo PR only commits SpecKit planning artifacts.

- Add `chatgptcodex.CatalogClient` and catalog metadata structs that parse the upstream model list while preserving unknown/future values where safe.
- Add `OpenAISubscriptionCodexCatalogCache` in handler layer, patterned after usage cache but storing sanitized catalog entries.
- Resolve eligible subscription credentials from routable runtime models whose transport is `chatgpt_codex_backend`.
- Enrich each runtime alias by matching its upstream model name to a catalog entry selected from the eligible credential-aware cache.
- Keep fallback conversion from `config.ModelConfig` for non-subscription aliases or no-cache/no-upstream states.
- Preserve HO-1331 `data[]` + `models[]` response from one filtered runtime list.

## Scope Evidence

- Approach recorded: credential-aware upstream catalog mirror/cache, runtime alias enrichment, last-known-good fallback, HO-1331 response preservation.
- Surfaces recorded: `internal/provider/chatgptcodex`, `internal/proxy/handler`, optional sqlc-backed `internal/db` persistence, and tests under the same packages.
- Task groups recorded: RED tests, catalog parser, cache/refresh/invalidation, alias enrichment, fallback/secret safety, HO-1331 regression, live refresh verification.
- Behavior recorded: fresh cache wins; refresh failure serves last known good; no cache falls back static; upstream-only models remain hidden unless routable.
- UI alignment recorded: UI N/A because this is backend/API/catalog metadata scope.
- Clarification recorded: no owner input remains for Todo; systemic fix is required by Linear issue.
- Missing part recorded: none for Todo planning; implementation must still prove failing tests and live refresh in later states.
- Unclear items recorded: none; multi-credential semantics are intentionally captured as an implementation design/test gate.
- Pre-implementation missing parts recorded: none for Todo handoff.
- Repo/code confirmation recorded: `rg`/file reads confirmed current synthesized metadata and existing HO-1331 runtime/list-model surfaces.
- Implementation-detail clarity recorded: target packages, cache behavior, fallback order, tests, and live verification are specified before In Progress.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Persistent last-known-good catalog storage may need new sqlc queries | Last-known-good must survive transient upstream failures and process restarts if existing credential info storage is insufficient | In-memory-only cache weakens fallback semantics and live reliability |
