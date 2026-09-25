# Feature Specification: HO-2370 Mirror Codex subscription model catalog

**Feature Branch**: `HO-2370-mirror-and-cache-openai-codex-subscription-model`
**Created**: 2026-07-10
**Status**: Draft
**Input**: Linear HO-2370, "Mirror and cache OpenAI Codex subscription model catalog metadata"

> Informed by memory: memory_search unavailable on 2026-07-10 due provider timeout. Scope is instead grounded in live Linear HO-2370, repo scan, HO-1331 artifacts, and official/stock Codex evidence captured in `research.md`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Subscription aliases use upstream Codex metadata (Priority: P1)

As a Tianji operator, I want subscription-backed Sol, Terra, and Luna aliases to advertise the actual Codex catalog metadata for the resolved subscription entitlement, so downstream Codex/OpenClaw clients do not receive fabricated 128k/90%/text-only values.

**Why this priority**: This fixes the user-visible broken metadata that causes premature compaction, hidden image support, and wrong reasoning levels.

**Independent Test**: Seed a fixture matching the observed upstream Sol/Terra/Luna catalog, configure matching Tianji runtime aliases, call `GET /models`, and assert the Codex `models[]` entries reflect upstream values while `data[]` remains OpenAI-compatible.

**Acceptance Scenarios**:

1. **Given** a fresh upstream catalog entry for `gpt-5.6-sol` with `context_window: 372000`, `max_context_window: 372000`, `effective_context_window_percent: 95`, `auto_compact_token_limit: null`, `truncation_policy: {"mode":"tokens","limit":10000}`, text/image modalities, upstream reasoning levels/default, and speed/service tiers, **When** Tianji lists a routable Sol alias backed by that subscription model, **Then** the alias Codex metadata uses those upstream values and does not synthesize a 90% auto-compact limit.
2. **Given** upstream catalog entries for Terra and Luna with model-specific defaults, **When** matching Tianji runtime routes exist, **Then** each alias receives its own upstream reasoning, modality, context, truncation, and tier metadata.
3. **Given** an upstream catalog entry exists but Tianji has no matching runtime route, **When** `GET /models` is requested, **Then** Tianji does not advertise that upstream-only model as callable.

---

### User Story 2 - Catalog refresh is cached and credential-aware (Priority: P1)

As a Tianji operator, I want Codex catalog refreshes cached per credential or entitlement boundary, so `/models` is fast, resilient, and cannot leak one credential's entitlements as another credential's guarantees.

**Why this priority**: The catalog must not add an upstream request to every model-list call, and entitlements are not globally interchangeable.

**Independent Test**: Configure two subscription credentials with different upstream fixtures, call `GET /models` repeatedly, and assert Tianji uses fresh cache entries without refetching while keeping catalog results separated by credential or explicit aggregate semantics.

**Acceptance Scenarios**:

1. **Given** a fresh cache entry for a resolved subscription credential, **When** `/models` is called multiple times within the TTL, **Then** Tianji serves from cache and does not call upstream once per downstream request.
2. **Given** two subscription credentials with different model capabilities, **When** Tianji builds alias metadata, **Then** it never presents one credential's catalog as another credential's guaranteed capability set; any aggregation or selection is explicit and tested.
3. **Given** a subscription credential is connected, refreshed, disabled, or replaced, **When** catalog metadata may have changed, **Then** Tianji invalidates or refreshes the affected credential-aware cache entries.

---

### User Story 3 - Last-known-good fallback is safe (Priority: P2)

As a Tianji maintainer, I want transient upstream failures to serve the last known good catalog with degraded-state evidence, so clients remain usable without silently reverting to fabricated metadata.

**Why this priority**: Resilience is required, but hiding degraded refreshes would make entitlement drift hard to diagnose.

**Independent Test**: Seed a cached catalog, make refresh fail, call `GET /models`, and assert Tianji serves the cached metadata, records a degraded signal, and leaks no tokens or raw credential bundles.

**Acceptance Scenarios**:

1. **Given** a last known good catalog exists and refresh fails transiently, **When** `/models` is requested, **Then** Tianji serves the cached catalog and records a degraded refresh signal.
2. **Given** no upstream result and no cached catalog exist, **When** `/models` is requested, **Then** Tianji falls back to conservative static metadata, including the existing `128000` context fallback.
3. **Given** refresh or cache logging occurs, **When** logs/metrics are inspected, **Then** they include credential-safe IDs/status/reasons only and never include access tokens, refresh tokens, bearer headers, or raw credential bundles.

---

### User Story 4 - HO-1331 response contract remains intact (Priority: P2)

As an existing client maintainer, I want Tianji to preserve the combined OpenAI-compatible `data[]` and Codex-compatible `models[]` response from HO-1331 while enriching metadata, so this fix does not regress model-list compatibility.

**Why this priority**: HO-1331 fixed the structural Codex decode failure; HO-2370 must only improve metadata source and cache behavior.

**Independent Test**: Run the existing list-models regression tests and add a fixture test proving enriched aliases still appear in both shapes from the same filtered runtime source.

**Acceptance Scenarios**:

1. **Given** any visible runtime model, **When** `/models` or `/v1/models` is called, **Then** the response includes `object:"list"`, OpenAI-compatible `data[]`, and Codex-compatible `models[]`.
2. **Given** a hidden alias configured through router alias settings, **When** `/models` is called, **Then** it is excluded from both `data[]` and `models[]`.
3. **Given** DB-managed runtime models exist, **When** runtime model refresh completes, **Then** model-list responses still use the merged runtime source and not an independent upstream-only catalog.

### Edge Cases

- Upstream catalog contains a model slug that matches no Tianji route: do not advertise it.
- Tianji alias uses a display name different from upstream model slug: retain the alias as `slug` but enrich from the matched upstream model.
- Upstream explicitly sends `auto_compact_token_limit: null`: preserve null/omission semantics; do not synthesize 90%.
- Upstream sends `max`, `ultra`, or future reasoning effort strings: pass through supported non-empty values; do not truncate to the old fixed set.
- Upstream omits optional speed/service tiers: use empty arrays without failing the response.
- Upstream fetch returns 401/403/429/5xx or malformed JSON: prefer last known good; otherwise conservative fallback.
- Multiple credentials have different entitlements: cache and selection must be credential- or entitlement-aware.
- Credential secrets appear in raw upstream requests/responses: cache persistence and logs must exclude them.
- Cache TTL expires during concurrent `/models` calls: coalesce refreshes or otherwise avoid request stampede.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tianji MUST fetch the OpenAI Codex subscription model catalog using a healthy resolved subscription credential through the same provider-owned `/models` catalog path used by stock Codex.
- **FR-002**: Tianji MUST cache upstream Codex catalog metadata so `/models` does not require an upstream request on every downstream call.
- **FR-003**: The catalog cache MUST be credential- or entitlement-aware, with explicit tested semantics for multiple subscription credentials.
- **FR-004**: Tianji MUST enrich only configured, routable runtime aliases from matching upstream catalog entries; upstream-only models MUST NOT become callable Tianji models.
- **FR-005**: Enriched aliases MUST map context window, max context window, effective context percentage, truncation policy, default/supported reasoning levels, input modalities, speed tiers, service tiers, and explicit upstream `auto_compact_token_limit` including null.
- **FR-006**: Tianji MUST stop synthesizing `auto_compact_token_limit = context * 90%` when upstream reports null or another explicit value.
- **FR-007**: Tianji MUST prefer fresh cached catalog entries, then last known good entries after refresh failure, then conservative static fallback only when neither upstream nor cached catalog is usable.
- **FR-008**: Tianji MUST record degraded refresh signals for failed refreshes without leaking secrets.
- **FR-009**: Tianji MUST invalidate or refresh affected cache entries when subscription credentials are connected, refreshed, disabled, or replaced.
- **FR-010**: Tianji MUST preserve the HO-1331 dual response contract: `object:"list"`, `data[]`, and `models[]` from the same filtered runtime source.
- **FR-011**: Regression tests MUST cover Sol/Terra/Luna fixture metadata, cache hit behavior, last-known-good fallback, no-cache static fallback, entitlement separation, route boundary, secret redaction, and HO-1331 shape preservation.
- **FR-012**: Live verification MUST show a Codex/OpenClaw model refresh through Tianji reports upstream-derived Sol/Terra/Luna metadata and no fabricated 128k/90% values.

### Key Entities *(include if feature involves data)*

- **Runtime Alias**: A Tianji-configured model route exposed to clients; remains the availability boundary.
- **Subscription Credential**: A stored OpenAI subscription identity that can be resolved into access token/account metadata for upstream Codex backend calls.
- **Upstream Codex Catalog Entry**: The model metadata returned by stock Codex's provider-owned `/models` path for a subscription entitlement.
- **Credential-Aware Catalog Cache Entry**: A cached sanitized catalog snapshot scoped to a credential or explicit entitlement grouping, with TTL, last-known-good metadata, status, and degraded reason.
- **Enriched Codex Model Info**: The HO-1331 Codex `models[]` entry after overlaying upstream catalog metadata onto a Tianji runtime alias.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Fixture-based Sol/Terra/Luna tests assert `context_window = 372000`, `max_context_window = 372000`, `effective_context_window_percent = 95`, `auto_compact_token_limit = null`, `truncation_policy.limit = 10000`, text/image modalities, upstream reasoning defaults/levels, and upstream speed/service tiers where present.
- **SC-002**: Repeated `/models` calls within cache TTL perform at most one upstream catalog refresh per affected credential or entitlement key.
- **SC-003**: A transient refresh failure serves last known good metadata and records a degraded state without secret material.
- **SC-004**: With no upstream result and no cached result, `/models` still returns conservative static metadata including `128000` fallback context.
- **SC-005**: Two credentials with different fixtures cannot leak one credential's catalog as another credential's guaranteed capability set.
- **SC-006**: Existing and new HO-1331 regression tests pass for both `data[]` and `models[]`.
- **SC-007**: Live model refresh through Tianji shows upstream-derived Sol/Terra/Luna metadata and no fabricated `128000` context or `context * 90%` auto-compaction values.

## Alternatives Considered

- **Hardcode Sol/Terra/Luna metadata in Go**: Rejected because rollout and entitlement metadata drift, and Linear explicitly marks hardcoding as a non-goal.
- **Fill DB `model_info` manually**: Rejected because it is operationally fragile and does not solve refresh, entitlement, or cache invalidation.
- **Expose the entire upstream subscription catalog**: Rejected because Tianji runtime routing remains the availability boundary.
- **Keep HO-1331 static defaults only**: Rejected because it preserves fabricated values and hides image/reasoning/tier capabilities.

## Out of Scope

- Changing inference routing or credential selection except where needed to select a safe catalog credential.
- Changing Codex CLI or OpenClaw client behavior.
- Permanently fixing metadata by manual DB edits.
- Advertising non-routable upstream models.

## Repo Reality Notes

- Repo confirmation used `rg` and direct reads of `internal/proxy/handler/list_models_catalog.go`, `internal/proxy/handler/runtime_model_source.go`, `internal/proxy/handler/list_models_codex_test.go`, `internal/proxy/handler/runtime_model_source_test.go`, and `internal/proxy/handler/openai_subscription_codex_usage.go`.
- HO-1331 prior artifacts in `specs/1331-codex-model-catalog-schema/` confirm `/models` already owns the combined OpenAI-compatible `data[]` and Codex-compatible `models[]` response shape.
- Current production code synthesizes Codex metadata locally; this Todo scope is docs-only and does not change production code.

## UI Alignment

UI N/A. This is a backend/API/catalog metadata issue in `tianjiLLM`; no UI route, component, viewport, or mockscreen is owned by this issue.

## Clarification And Missing Parts

- Clarification: resolved for Todo. The Linear issue explicitly requires a systemic long-term fix, not a patch/hardcode.
- No clarification markers remain.
- unclear: none for Todo owner input; implementation must still choose and test explicit multi-credential selection or aggregation semantics.
- No clarification markers remain.
- Missing part: none for Todo planning. Repo reality, stock Codex source, OpenAI docs, HO-1331 artifacts, and the task/test plan identify the implementation surfaces and external contract evidence needed for In Progress.
