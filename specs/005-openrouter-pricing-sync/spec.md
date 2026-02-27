# Feature Specification: OpenRouter Pricing Sync

**Feature Branch**: `005-openrouter-pricing-sync`  
**Created**: 2025-06-28  
**Status**: Draft  
**Input**: User description: "Add OpenRouter API as secondary upstream pricing source for model sync. LiteLLM remains primary, OpenRouter fills gaps. Graceful degradation if OpenRouter fails."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Zero-Cost Model Fix (Priority: P1)

As an admin, when I click "Sync Pricing", models that exist in OpenRouter but not in LiteLLM (e.g., `gemini-2.5-pro-preview`) should have their cost data populated from OpenRouter.

**Why this priority**: This is the core bug that triggered HO-21. Users see $0.00 cost for models that have known pricing.

**Independent Test**: After sync, query the DB for `gemini-2.5-pro-preview` and verify non-zero input/output cost.

**Acceptance Scenarios**:

1. **Given** LiteLLM upstream lacks `gemini-2.5-pro-preview`, **When** admin clicks Sync Pricing, **Then** the model's cost is populated from OpenRouter ($1.25/$10.00 per 1M tokens).
2. **Given** a model exists in both LiteLLM and OpenRouter with different prices, **When** admin clicks Sync Pricing, **Then** the LiteLLM price is used (primary source wins).

---

### User Story 2 - Dual Key Storage (Priority: P1)

As a system, when syncing from OpenRouter, each model should be stored under both its provider-prefixed name (`google/gemini-2.5-pro-preview`) and its bare name (`gemini-2.5-pro-preview`), so lookups work regardless of which format the proxy uses.

**Why this priority**: Without dual keys, the lookup may still miss depending on how the model name arrives from the LLM proxy.

**Independent Test**: After sync, query DB for both `google/gemini-2.5-pro-preview` and `gemini-2.5-pro-preview` — both should have identical pricing.

**Acceptance Scenarios**:

1. **Given** OpenRouter returns model id `google/gemini-2.5-pro-preview`, **When** sync completes, **Then** DB contains entries for both `google/gemini-2.5-pro-preview` and `gemini-2.5-pro-preview`.
2. **Given** an OpenRouter model id has no `/` (no provider prefix), **When** sync completes, **Then** only one entry is created (no duplicate).

---

### User Story 3 - Graceful Degradation (Priority: P2)

As an admin, if OpenRouter API is unreachable or returns an error, the sync should still complete successfully with LiteLLM data. The failure should be logged but not block the operation.

**Why this priority**: Reliability — adding a second upstream must not make sync less reliable than before.

**Independent Test**: Block OpenRouter URL, click Sync Pricing, verify LiteLLM models still sync and a warning is logged.

**Acceptance Scenarios**:

1. **Given** OpenRouter API returns HTTP 500, **When** admin clicks Sync Pricing, **Then** LiteLLM models sync normally and a warning toast/log indicates OpenRouter failed.
2. **Given** OpenRouter API times out after 30s, **When** admin clicks Sync Pricing, **Then** LiteLLM sync proceeds without waiting indefinitely.

---

### User Story 4 - Configurable OpenRouter URL (Priority: P3)

As a deployer, I can override the OpenRouter API URL via the `OPENROUTER_PRICING_URL` environment variable for testing or self-hosted mirrors.

**Why this priority**: Standard operability — useful for testing and air-gapped environments.

**Independent Test**: Set env var to a mock server URL, sync, verify data comes from mock.

**Acceptance Scenarios**:

1. **Given** `OPENROUTER_PRICING_URL` is set to `http://localhost:9999/models`, **When** sync runs, **Then** it fetches from that URL instead of the default.
2. **Given** `OPENROUTER_PRICING_URL` is not set, **When** sync runs, **Then** it uses `https://openrouter.ai/api/v1/models`.

---

### Edge Cases

- What happens when OpenRouter returns an empty `data` array? → Treat as 0 supplemental models, log warning, proceed.
- What happens when an OpenRouter model has `null` or `"0"` pricing? → Skip that model (don't overwrite existing data with zero).
- What happens when OpenRouter model id has multiple `/` (e.g., `openrouter/google/gemini`)? → Split on first `/` only for provider extraction; bare name = everything after first `/`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST fetch model pricing from OpenRouter API (`GET https://openrouter.ai/api/v1/models`) after fetching from LiteLLM upstream during sync.
- **FR-002**: System MUST use LiteLLM pricing when a model exists in both sources (LiteLLM is primary).
- **FR-003**: System MUST create two DB entries per OpenRouter-only model: provider-prefixed (`provider/model`) and bare (`model`).
- **FR-004**: System MUST parse OpenRouter's `data[].pricing.prompt` and `data[].pricing.completion` as per-token costs (string → float64).
- **FR-005**: System MUST NOT block or fail the overall sync if OpenRouter fetch fails (graceful degradation).
- **FR-006**: System MUST log a warning when OpenRouter fetch fails, including the error reason.
- **FR-007**: System MUST support `OPENROUTER_PRICING_URL` env var to override the default OpenRouter API URL.
- **FR-008**: System MUST skip OpenRouter models with zero or null pricing values.

### Key Entities

- **OpenRouter Model**: Identified by `data[].id` (e.g., `google/gemini-2.5-pro-preview`). Pricing in `data[].pricing.prompt` / `.completion` as string-encoded per-token costs.
- **PricingRecord (DB)**: Existing `ModelPricing` table row. `source_url` field distinguishes origin (LiteLLM URL vs OpenRouter URL).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After sync, 100% of models available in OpenRouter but missing from LiteLLM have non-zero cost data in the DB.
- **SC-002**: After sync, no model that exists in LiteLLM has its pricing overwritten by OpenRouter data.
- **SC-003**: When OpenRouter API is unavailable, sync completes successfully with all LiteLLM models — zero regression from current behavior.
- **SC-004**: Both `google/gemini-2.5-pro-preview` and `gemini-2.5-pro-preview` return identical pricing after sync.
- **SC-005**: Sync operation completes within 60 seconds (combined LiteLLM + OpenRouter fetch + DB upsert).
