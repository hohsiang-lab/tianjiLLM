# Feature Specification: Sync Model Pricing Button

**Feature Branch**: `003-sync-pricing-button`
**Created**: 2026-02-25
**Status**: Draft
**Input**: User description: "Sync Model Pricing button — fetch latest prices and persist to DB" (Issue #7)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sync Pricing from Upstream (Priority: P1)

An admin opens the Models page in the admin UI and clicks the "Sync Pricing"
button. The system fetches the latest `model_prices_and_context_window.json`
from the LiteLLM upstream source, writes the pricing data into the database,
and hot-reloads the in-memory pricing calculator. New API requests immediately
use the updated pricing. A success toast confirms the operation.

**Why this priority**: Core feature. Without this, pricing data is frozen at
build time and requires a redeploy to update.

**Independent Test**: Click "Sync Pricing" on the Models page. Verify the
database `model_pricing` table is populated, the in-memory calculator returns
updated prices for a known model, and a success toast is displayed.

**Acceptance Scenarios**:

1. **Given** the admin UI Models page is loaded, **When** the admin clicks
   "Sync Pricing", **Then** the system fetches pricing from the upstream URL,
   persists all model pricing entries to the database, and displays a success
   toast with the count of models synced.
2. **Given** a successful sync, **When** a new API request arrives for cost
   calculation, **Then** the in-memory pricing calculator uses the newly synced
   prices without requiring a service restart.
3. **Given** a successful sync, **When** the admin views the Models page,
   **Then** models display their updated pricing information.

---

### User Story 2 - Sync Failure Feedback (Priority: P1)

The upstream URL is unreachable or returns invalid data. The admin sees an
error toast with a clear message. Existing pricing data (embedded or previously
synced) remains intact and unchanged.

**Why this priority**: Failure must be graceful. Corrupting or clearing pricing
data on a failed sync would break cost tracking for all requests.

**Independent Test**: Block network access to the upstream URL (or configure
an invalid URL). Click "Sync Pricing". Verify an error toast appears, the
database pricing data is unchanged, and the in-memory calculator still works.

**Acceptance Scenarios**:

1. **Given** the upstream URL is unreachable, **When** the admin clicks
   "Sync Pricing", **Then** an error toast displays: "Failed to sync pricing:
   [reason]" and existing pricing data is unaffected.
2. **Given** the upstream returns malformed JSON, **When** the admin clicks
   "Sync Pricing", **Then** no data is written to the database and an error
   toast explains the parse failure.
3. **Given** a network timeout, **When** the admin clicks "Sync Pricing",
   **Then** the request completes within 30 seconds and shows a timeout error
   toast.

---

### User Story 3 - Pricing Lookup Fallback Chain (Priority: P1)

When calculating costs, the system checks DB-synced pricing first, then falls
back to the build-time embedded data. This ensures pricing always works even
if no sync has ever been performed.

**Why this priority**: The system must remain functional without a database or
before the first sync. The embedded data is the safety net.

**Independent Test**: Start the service fresh (no prior sync). Verify cost
calculations use embedded pricing. Perform a sync. Verify cost calculations
now use DB pricing. Restart the service without DB access. Verify embedded
pricing is used again.

**Acceptance Scenarios**:

1. **Given** no pricing has been synced to DB, **When** a cost calculation
   is requested, **Then** the embedded `model_prices.json` data is used.
2. **Given** pricing has been synced to DB, **When** the service starts,
   **Then** DB pricing is loaded into the in-memory calculator on startup,
   taking precedence over embedded data.
3. **Given** DB is unavailable (no-DB mode), **When** a cost calculation is
   requested, **Then** the embedded pricing is used and no errors are logged.

---

### User Story 4 - Button Loading State (Priority: P2)

While the sync is in progress, the button shows a loading spinner and is
disabled to prevent duplicate requests. The UI remains responsive.

**Why this priority**: UX polish. Prevents accidental double-syncs and gives
the admin visual feedback that the operation is in progress.

**Independent Test**: Click "Sync Pricing" and observe the button becomes
disabled with a spinner. After completion, the button returns to its normal
state.

**Acceptance Scenarios**:

1. **Given** the admin clicks "Sync Pricing", **When** the request is in
   flight, **Then** the button is disabled and shows a loading indicator.
2. **Given** the sync completes (success or failure), **When** the response
   is received, **Then** the button returns to its clickable state.

---

### Edge Cases

- What happens when the database is not configured (no-DB mode)?
  → The "Sync Pricing" button is hidden or disabled with a tooltip explaining
    that a database is required for pricing sync.
- What happens when a sync is triggered while another sync is already in progress?
  → The button is disabled during sync; concurrent requests from other sources
    are serialized or rejected with a 409 Conflict.
- What happens when the upstream JSON contains models not previously known?
  → New models are inserted; this is expected and desired behavior.
- What happens when the upstream JSON removes models that were previously synced?
  → Previously synced models that are absent from the new data are kept in DB
    (no deletion). A full replacement strategy would risk losing custom overrides.
    The sync is an upsert operation.
- What happens when the upstream JSON has a different schema than expected?
  → The sync validates the JSON structure before writing. If the top-level
    structure is valid but individual model entries have missing fields, those
    entries are skipped with a warning logged. The overall sync still succeeds
    for valid entries.
- What happens when the database write partially fails mid-sync?
  → The entire sync is wrapped in a database transaction. On any write failure,
    the transaction is rolled back, no partial data is persisted, and an error
    toast is shown.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The admin UI Models page MUST display a "Sync Pricing" button
  when the database is available.
- **FR-002**: Clicking the button MUST trigger an HTTP POST to a new endpoint
  (e.g., `POST /ui/models/sync-pricing`) that fetches the upstream pricing JSON.
- **FR-003**: The upstream URL MUST default to the LiteLLM
  `model_prices_and_context_window.json` GitHub raw URL and be configurable
  via environment variable (`PRICING_UPSTREAM_URL`).
- **FR-004**: The fetched pricing data MUST be validated (valid JSON, expected
  top-level structure) before any database writes.
- **FR-005**: The sync MUST persist pricing data to a `model_pricing` database
  table using an upsert strategy (insert or update on conflict).
- **FR-006**: All database writes for a single sync operation MUST be wrapped
  in a single transaction (all-or-nothing).
- **FR-007**: After successful DB write, the in-memory `Calculator` MUST be
  hot-reloaded with the new pricing data so subsequent cost calculations use
  updated prices immediately.
- **FR-008**: The endpoint MUST return an HTMX-compatible response that triggers
  a toast notification (success with model count, or failure with error message).
- **FR-009**: On service startup with DB available, the system MUST load
  DB-synced pricing into the in-memory calculator, with DB data taking
  precedence over embedded data.
- **FR-010**: The HTTP fetch to upstream MUST have a configurable timeout
  (default 30 seconds).
- **FR-011**: All DB queries for this feature MUST be defined as `.sql` files
  and code-generated via sqlc (per Constitution §VII).
- **FR-012**: The "Sync Pricing" button MUST be hidden or disabled when
  the database is not configured.

### Key Entities

- **model_pricing table**: Stores synced pricing data. Fields: `model_name`
  (text, PK), `input_cost_per_token` (numeric), `output_cost_per_token`
  (numeric), `max_input_tokens` (integer), `max_output_tokens` (integer),
  `max_tokens` (integer), `mode` (text), `provider` (text),
  `synced_at` (timestamptz), `source_url` (text).
- **Calculator**: Existing `internal/pricing.Calculator` — extended with a
  `ReloadFromDB` method to replace in-memory model data from DB records.
- **Upstream URL**: The remote JSON endpoint (LiteLLM GitHub raw URL by default).

### Non-Functional Requirements

- **NFR-001**: The sync operation (fetch + parse + DB write) MUST complete
  within 60 seconds under normal network conditions.
- **NFR-002**: The hot-reload of the in-memory calculator MUST be lock-safe
  and not block concurrent cost calculation requests for more than 100ms.
- **NFR-003**: The upstream fetch MUST use a dedicated HTTP client with
  appropriate timeouts (connect: 10s, total: 30s) — not the default Go client.
- **NFR-004**: The sync endpoint MUST be protected by the same admin
  authentication as other UI management endpoints.
- **NFR-005**: The feature MUST NOT break the existing no-DB deployment mode.
  All new DB-dependent code paths must check for DB availability first.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After clicking "Sync Pricing", the `model_pricing` table contains
  ≥1000 model entries (matching the upstream source count) within 60 seconds.
- **SC-002**: After a successful sync, `Calculator.Cost()` for a model whose
  price changed in the upstream returns the new price (verified by test).
- **SC-003**: A failed sync (network error) displays an error toast within
  35 seconds and leaves `model_pricing` table data unchanged.
- **SC-004**: Two consecutive syncs are idempotent — the second sync updates
  `synced_at` timestamps but produces no duplicate rows.
- **SC-005**: Service startup with existing DB pricing data loads it into
  memory in under 5 seconds for 10,000 model entries.
- **SC-006**: The in-memory hot-reload does not cause any concurrent
  `Cost()` call to return an error or zero for a previously-known model.

## Future Scope (P2/P3 — not in MVP)

### P2: Scheduled Auto-Sync

LiteLLM supports `POST /schedule/model_cost_map_reload?hours=6` for periodic
auto-sync. We should add a similar mechanism:

- **FR-013** (P2): The system SHOULD support a configurable periodic sync
  interval via environment variable (e.g., `PRICING_SYNC_INTERVAL_HOURS`).
- **FR-014** (P2): A `POST /api/v1/pricing/schedule?hours={hours}` endpoint
  SHOULD allow admins to start/stop periodic sync at runtime.
- **FR-015** (P2): A `GET /api/v1/pricing/schedule/status` endpoint SHOULD
  return the current sync schedule and last sync timestamp.

### P2: API Endpoint for CLI/Automation

The current spec only covers the UI button. For programmatic access:

- **FR-016** (P2): A `POST /api/v1/pricing/sync` REST endpoint SHOULD be
  available for CLI and automation tools (separate from the HTMX UI endpoint).
- **FR-017** (P2): The API endpoint SHOULD return JSON with sync result
  (count, duration, errors).

### P3: Local File Override

LiteLLM supports `LITELLM_LOCAL_MODEL_COST_MAP=True` to use a local file
instead of fetching from remote:

- **FR-018** (P3): The system SHOULD support loading pricing from a local
  JSON file path via `PRICING_LOCAL_FILE` environment variable.
- **FR-019** (P3): When `PRICING_LOCAL_FILE` is set, the "Sync Pricing"
  button SHOULD read from the local file instead of fetching remote.

## Assumptions

- The LiteLLM `model_prices_and_context_window.json` format is stable and
  matches the existing `ModelInfo` struct fields. The Python TianjiLLM uses
  the same upstream source.
- The database is PostgreSQL (consistent with existing schema migrations).
- The admin UI already has HTMX and toast infrastructure in place (confirmed
  in `internal/ui/components/toast/`).
- The `model_prices.json` embedded file continues to serve as the build-time
  fallback and is not modified by this feature.
