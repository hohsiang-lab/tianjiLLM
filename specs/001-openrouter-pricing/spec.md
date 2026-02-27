# Feature Specification: Add OpenRouter as Secondary Pricing Source

**Feature Branch**: `001-openrouter-pricing`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "HO-21: Add OpenRouter as secondary upstream for model pricing sync. Problem: pricing sync only uses LiteLLM GitHub, missing models like gemini-2.5-pro-preview get cost=0. Solution: add OpenRouter API as second source."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Accurate Cost Tracking for Previously-Zero-Cost Models (Priority: P1)

An operator routes traffic to `gemini-2.5-pro-preview` or other newer models not yet listed in LiteLLM's pricing data. Today, all requests report cost = 0 in spend dashboards, making budget tracking and alerting useless for these models. After this feature, the pricing sync pipeline consults OpenRouter as a fallback source, so those models display real per-token pricing.

**Why this priority**: Cost accuracy is the core business value of a proxy. Operators rely on cost data to enforce budgets, chargebacks, and model selection decisions. Silent zero-cost reporting creates invisible overspending.

**Independent Test**: Can be fully tested by triggering a pricing sync and then querying the stored price for `gemini/gemini-2.5-pro-preview` or `gemini-2.5-pro-preview` — the value must be non-zero and match the OpenRouter catalog.

**Acceptance Scenarios**:

1. **Given** a fresh pricing sync runs, **When** the LiteLLM source does not contain `gemini-2.5-pro-preview`, **Then** the system fetches OpenRouter data and stores a non-zero cost per token for that model.
2. **Given** pricing sync completes, **When** a request is processed for `gemini-2.5-pro-preview`, **Then** the spend log records the actual cost instead of zero.
3. **Given** OpenRouter lists a model under the id `google/gemini-2.5-pro-preview`, **When** sync completes, **Then** the model is accessible under both `google/gemini-2.5-pro-preview` and `gemini-2.5-pro-preview` keys.

---

### User Story 2 - LiteLLM Prices Remain Authoritative (Priority: P2)

A model appears in both LiteLLM and OpenRouter with slightly different pricing. The operator trusts LiteLLM's carefully curated data. After pricing sync, LiteLLM values must win for any model present in both sources, with OpenRouter only filling in the gaps.

**Why this priority**: Merge priority protects against OpenRouter data quality issues silently overwriting curated LiteLLM prices, which could corrupt all cost calculations for high-volume models.

**Independent Test**: Set up mock sources where model X has price 0.001 in LiteLLM and 0.002 in OpenRouter; verify stored price is 0.001.

**Acceptance Scenarios**:

1. **Given** a model exists in both LiteLLM and OpenRouter data, **When** sync completes, **Then** the stored price matches the LiteLLM value exactly.
2. **Given** a model exists only in LiteLLM, **When** OpenRouter is unavailable, **Then** the LiteLLM price is stored unchanged.
3. **Given** a model exists only in OpenRouter, **When** sync completes, **Then** the OpenRouter price is stored.

---

### User Story 3 - Graceful Degradation When OpenRouter Is Unreachable (Priority: P3)

The OpenRouter API is temporarily down or the network is blocked. The pricing sync should complete successfully using only LiteLLM data, logging a warning rather than failing the entire sync job. Models already priced from LiteLLM continue to function correctly.

**Why this priority**: Resilience ensures a transient external dependency never blocks the proxy from starting or updating prices for the majority of well-known models.

**Independent Test**: Block OpenRouter URL during a sync run and verify: sync completes without error exit, LiteLLM prices are persisted, a warning log entry appears.

**Acceptance Scenarios**:

1. **Given** the OpenRouter endpoint is unreachable, **When** pricing sync runs, **Then** the sync completes successfully using LiteLLM data only.
2. **Given** OpenRouter returns an unexpected response format, **When** sync runs, **Then** a warning is logged and the sync continues without crashing.
3. **Given** OpenRouter is unreachable and a model only exists in OpenRouter, **When** sync completes, **Then** that model retains its previous stored price (or zero if no previous price exists).

---

### User Story 4 - Configurable OpenRouter Endpoint (Priority: P4)

An operator runs in an air-gapped environment or wants to use a mirror of the OpenRouter catalog. They set the `OPENROUTER_PRICING_URL` environment variable to point to an internal endpoint. The pricing sync uses the configured URL without requiring a code change.

**Why this priority**: Configurability is a lower-priority operational concern — the default works for the vast majority of deployments, but env-var override is needed for regulated environments.

**Independent Test**: Set `OPENROUTER_PRICING_URL` to a local test server URL and verify that server receives the pricing fetch request during sync.

**Acceptance Scenarios**:

1. **Given** `OPENROUTER_PRICING_URL` is set, **When** sync runs, **Then** requests go to the configured URL, not the default.
2. **Given** `OPENROUTER_PRICING_URL` is not set, **When** sync runs, **Then** requests go to `https://openrouter.ai/api/v1/models`.

---

### User Story 5 - Remove Manual JSON Pricing Workarounds (Priority: P5)

PR #23 added manual pricing entries in a JSON file as a workaround for missing models. These entries are now superseded by the OpenRouter integration. Removing them reduces maintenance burden and eliminates the risk of stale hardcoded prices diverging from actual provider pricing.

**Why this priority**: Cleanup reduces technical debt but has no user-facing impact since OpenRouter data replaces the manual entries with dynamic, up-to-date values.

**Independent Test**: After the revert, verify that the manually added models still receive non-zero pricing from OpenRouter, with no hardcoded overrides present in the codebase.

**Acceptance Scenarios**:

1. **Given** PR #23 manual JSON entries are removed, **When** sync runs, **Then** models previously covered by those entries still have non-zero pricing sourced from OpenRouter.
2. **Given** a model's price changes on OpenRouter, **When** the next sync runs, **Then** the stored price reflects the updated value (not a stale hardcoded value).

---

### Edge Cases

- What happens when OpenRouter returns an empty `data` array? System logs a warning and treats it as a fetch failure (graceful degradation).
- What happens when a model's `pricing.prompt` or `pricing.completion` field is missing or `"0"` in OpenRouter? The system stores zero for that field (consistent with source data).
- What happens when OpenRouter returns `data[].id` without a `/` separator (no provider prefix)? The system stores it under both the raw id and as-is (no artificial prefix added).
- What happens when the sync job is interrupted mid-run? LiteLLM data already written remains; OpenRouter enrichment may be partial — the next full sync corrects it.
- What happens when the OpenRouter URL returns a non-200 HTTP status? Treated as a fetch failure; warning logged, sync continues with LiteLLM-only data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The pricing sync system MUST fetch LiteLLM pricing data first, completing or timing out before fetching OpenRouter data.
- **FR-002**: The pricing sync system MUST fetch OpenRouter pricing data after LiteLLM, using the endpoint configured in `OPENROUTER_PRICING_URL` or the default `https://openrouter.ai/api/v1/models`.
- **FR-003**: When merging pricing data, the system MUST use LiteLLM values for any model present in both sources; OpenRouter values are used only for models absent from LiteLLM.
- **FR-004**: For each OpenRouter model entry, the system MUST store pricing under the full `provider/model` key (e.g., `google/gemini-2.5-pro-preview`) AND under the model-only key (e.g., `gemini-2.5-pro-preview`).
- **FR-005**: The system MUST treat OpenRouter fetch failures (network error, non-200 response, malformed JSON) as non-fatal; pricing sync MUST complete using LiteLLM data alone, with a warning logged.
- **FR-006**: The manual pricing JSON entries introduced in PR #23 MUST be removed; their coverage MUST be provided by OpenRouter data instead.
- **FR-007**: The system MUST read per-token pricing from `data[].pricing.prompt` (input) and `data[].pricing.completion` (output) fields in the OpenRouter response.
- **FR-008**: The `OPENROUTER_PRICING_URL` environment variable MUST be respected when set; when absent, the system MUST use `https://openrouter.ai/api/v1/models` as the default.

### Key Entities

- **PricingRecord**: A stored cost entry for a model, containing input cost per token, output cost per token, and the source that provided the data (LiteLLM or OpenRouter).
- **LiteLLM Pricing Source**: The existing primary source — fetched first, authoritative for all overlapping models.
- **OpenRouter Pricing Source**: The new secondary source — fetched second, used only to enrich models absent from LiteLLM; response structure is `{ data: [{ id: "provider/model", pricing: { prompt: "per-token-string", completion: "per-token-string" } }] }`.
- **Model Key**: The identifier under which a price is stored; OpenRouter models produce two keys: full `provider/model` and bare `model`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Models available in OpenRouter but absent from LiteLLM (e.g., `gemini-2.5-pro-preview`) report non-zero input and output costs after a pricing sync — zero-cost entries for known models are eliminated.
- **SC-002**: Pricing for all models that existed in LiteLLM before this change remains identical after the change — no regressions in LiteLLM-sourced prices.
- **SC-003**: A pricing sync completes successfully (exit without error) even when the OpenRouter endpoint is intentionally unreachable — availability of the OpenRouter service does not block the sync job.
- **SC-004**: Manual pricing JSON entries from PR #23 are fully removed from the codebase with no loss of pricing accuracy for the models they covered.
- **SC-005**: Spend logs for requests to previously zero-cost models reflect non-zero costs within one sync cycle after deploying this feature.

## Assumptions

- OpenRouter's `/api/v1/models` endpoint does not require authentication for accessing the public model catalog and pricing data.
- The per-token price strings in OpenRouter's `pricing.prompt` / `pricing.completion` fields are parseable as floating-point numbers representing cost per token (not per million tokens).
- The existing pricing sync job runs on a scheduled basis; no changes to the scheduling mechanism are needed.
- OpenRouter's model IDs consistently use the `provider/model` format (e.g., `google/gemini-2.5-pro-preview`), making the two-key storage strategy reliable.
- "Graceful degradation" means the sync continues with available data; it does not mean retrying OpenRouter indefinitely — a single attempt with standard timeout is sufficient.
