# Research: Request Log Details

**Feature**: 200-request-log-details
**Date**: 2026-03-27

## Decision 1: Payload Storage Architecture

**Decision**: Store request/response payloads in a separate `RequestPayloads` table, not in SpendLogs.

**Rationale**: PostgreSQL TOAST causes 5-10x query performance degradation when JSONB values exceed 2KB. A single Claude Opus request easily exceeds 100KB of prompt content. Keeping payloads separate ensures SpendLogs remains fast for list views, aggregations, and filtering.

**Alternatives considered**:
- **Same table (LiteLLM's approach)**: Simpler schema but causes TOAST performance cliffs. LiteLLM users frequently report slow log queries. Their UI shows "Request/Response Data Not Available" because many operators disable it.
- **External object storage (S3)**: Overkill for this use case; adds latency and operational complexity for what is essentially a debugging feature.

**Sources**:
- [PostgreSQL Large JSON Performance](https://www.evanjones.ca/postgres-large-json-performance.html)
- [pganalyze: JSONB TOAST Performance Cliffs](https://pganalyze.com/blog/5mins-postgres-jsonb-toast)

## Decision 2: Payload Truncation Strategy

**Decision**: Truncate payloads at storage time using `MAX_STRING_LENGTH_PROMPT_IN_DB` env var (default 2048 chars). Same as LiteLLM. Use a front/back split: keep 35% from beginning + 65% from end, insert a truncation marker.

**Rationale**: Follow LiteLLM exactly. They use `MAX_STRING_LENGTH_PROMPT_IN_DB` (default 2048 chars) with a 35/65 split strategy. This preserves the system prompt (beginning) and the most recent assistant response (end). Users can increase via env var.

**Alternatives considered**:
- **No truncation, truncate only at display**: Risks unbounded DB growth. A single 1MB conversation would consume 1MB per request.
- **YAML config instead of env var**: LiteLLM uses env var; follow their convention for consistency.

## Decision 3: Detail Drawer UI Pattern

**Decision**: Split view layout — same as LiteLLM. Table shrinks to ~50% left, detail panel appears on the right. NOT a Sheet/drawer overlay.

**Rationale**: LiteLLM uses this pattern and it works well for log investigation — users keep the list visible for context while viewing details. The existing Sheet component (`max-w-sm` = 384px) is too narrow for log detail content. A split view with CSS flex/grid gives full control over widths. HTMX `hx-get` loads the detail partial on row click; closing restores the table to full width.

**Alternatives considered**:
- **Sheet/drawer overlay**: Existing Sheet component too narrow (384px). Would need CSS overrides and still obscures the table.
- **Full detail page**: Loses list context; user must navigate back and forth. Poor UX for log investigation.
- **Modal dialog**: Too confined for the amount of detail content (metrics, request/response, metadata).

## Decision 4: Request Metrics Logging (stdout)

**Decision**: Add structured log output in the spend tracker callback for every completed request.

**Rationale**: Operators need real-time visibility into per-request token consumption to diagnose usage spikes (e.g., session context growing over time). The callback already has all necessary data (`LogData` struct contains tokens, cost, model, timing). The existing tracker only logs warnings for zero-token records.

**Format**: `log.Printf("request-log: request_id=%s model=%s prompt_tokens=%d completion_tokens=%d total_tokens=%d cost=%.6f duration_ms=%d status=%s key=%s", ...)`

**Alternatives considered**:
- **Structured JSON logging**: Better for log aggregation but inconsistent with the codebase's existing `log.Printf` style.
- **Metrics-only (Prometheus)**: Loses per-request granularity; counters show totals, not individual request sizes.

## Decision 5: Duration Storage

**Decision**: Add `request_duration_ms` INTEGER column to SpendLogs. Compute from `endtime - starttime` at insert time in the spend tracker.

**Rationale**: Currently duration is derived at display time. Storing it enables efficient sorting and filtering in SQL queries without function-based indexes.

**Alternatives considered**:
- **Computed column / generated column**: PostgreSQL supports `GENERATED ALWAYS AS` but not for interval arithmetic on timestamps. Would need a function-based expression.
- **Keep computing at display time**: Works but prevents server-side sorting by duration.

## Decision 6: Error Detail Fetching

**Decision**: LEFT JOIN SpendLogs with ErrorLogs by request_id in the detail query. No schema changes needed for ErrorLogs.

**Rationale**: ErrorLogs already contains `status_code`, `error_type`, `error_message`, `traceback`, and `request_id`. A LEFT JOIN covers both success (no error record) and failure (one error record) cases cleanly.

## Decision 7: Payload Storage Configuration

**Decision**: Add `store_prompts_in_logs` boolean to proxy config (`general_settings`). Default: `false`.

**Rationale**: Follows LiteLLM's `store_prompts_in_spend_logs` pattern. Storing prompts has privacy and storage implications; it should be opt-in. When disabled, the callback simply skips the RequestPayloads INSERT, and the UI shows a guidance notice.
