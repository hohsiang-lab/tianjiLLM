# Feature Specification: Request Log Details

**Feature Branch**: `200-request-log-details`
**Created**: 2026-03-27
**Status**: Draft
**Input**: User description: "跟 LiteLLM 一样，要记录 log details"
**Reference**: LiteLLM SpendLogs UI (screenshots in `specs/` directory)

## Clarifications

### Session 2026-03-27

- Q: Should request/response payloads be stored in the same SpendLogs table (Option A) or a separate table (Option C)? → A: Separate table (Option C) — store payloads in an independent `RequestPayloads` table joined by request_id, to avoid TOAST performance cliffs on the main SpendLogs table.
- Q: Should the proxy log each request's token counts to stdout for real-time observability? → A: Yes — log prompt_tokens, completion_tokens, total_tokens, model, and cost per request to stdout, so operators can track per-request token consumption and diagnose usage spikes (e.g., session context growing over time).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Log Detail Panel (Priority: P1)

As a proxy admin, I want to click on any log entry in the Request Logs table and see a detail panel (split view) showing the full request information, so I can understand exactly what happened with each API call.

**Why this priority**: The core value — without a detail view, the list table only provides a summary. Admins need to drill into individual requests for debugging, cost analysis, and auditing.

**Independent Test**: Click any row in the logs table; the page splits into table (left ~50%) and detail panel (right ~50%) showing request details, metrics, and metadata for that specific request.

**Acceptance Scenarios**:

1. **Given** the logs table is displayed with at least one row, **When** the user clicks a row, **Then** the table shrinks to the left and a detail panel appears on the right showing the full log information
2. **Given** the detail panel is open, **When** the user clicks the close button or presses Escape, **Then** the panel closes and the table is restored to full width
3. **Given** the detail panel is open, **When** the user clicks a different row in the table, **Then** the panel updates to show the newly selected log's details

---

### User Story 2 - View Success Request Details (Priority: P1)

As a proxy admin, I want to see comprehensive details for successful requests including model info, token usage, cost, duration, and cache status.

**Why this priority**: Equally important as the detail panel itself — successful requests are the majority of traffic and the primary use case for cost analysis and performance monitoring.

**Independent Test**: Click a successful log entry; the detail panel displays all relevant metrics and request metadata.

**Acceptance Scenarios**:

1. **Given** a successful log entry is selected, **When** the detail panel opens, **Then** it displays:
   - Status badge (Success) with status color
   - Request ID (copyable)
   - Timestamp (human-readable)
   - Request Details section: Model, Provider, Call Type, API Base
   - Metrics section: Prompt Tokens, Completion Tokens, Total Tokens, Cost ($), Duration (seconds), Cache Hit status
   - Start Time and End Time
2. **Given** a successful log entry with cache hit, **When** the detail panel opens, **Then** the cache status is clearly indicated (e.g., "Hit" in green, "Miss" in gray)

---

### User Story 3 - View Failed Request Details (Priority: P1)

As a proxy admin, I want to see error information for failed requests including error code, error type, error message, and traceback, so I can diagnose and fix issues.

**Why this priority**: Error debugging is the primary reason admins look at individual log entries. Without error details, the logs page has limited diagnostic value.

**Independent Test**: Click a failed log entry; the detail panel displays a prominent error alert with error code, type, message, and traceback.

**Acceptance Scenarios**:

1. **Given** a failed log entry is selected, **When** the detail panel opens, **Then** it displays:
   - Status badge (Failed) with red/error styling
   - An error alert section showing: Error Code (HTTP status), Error Type, Error Message
   - Traceback text (if available) in a scrollable code block
2. **Given** a failed log entry without a traceback, **When** the detail panel opens, **Then** the traceback section is omitted (not shown as empty)

---

### User Story 4 - View Request and Response Payloads (Priority: P2)

As a proxy admin, I want to optionally see the request messages and response data for a log entry, so I can debug prompt/response issues.

**Why this priority**: Important for debugging but requires storing additional data (messages/response). The core detail view (metrics, errors) delivers value without this. Can be enabled via configuration.

**Independent Test**: When prompt storage is enabled and a log entry is selected, the detail panel shows request messages and response content.

**Acceptance Scenarios**:

1. **Given** prompt storage is enabled in config, **When** the detail panel opens for a log with stored messages, **Then** a "Request & Response" section shows the request messages and response content fetched from the RequestPayloads table
2. **Given** prompt storage is NOT enabled, **When** the detail panel opens, **Then** a notice indicates that request/response data is not available, with guidance on how to enable it
3. **Given** the request/response section is displayed, **When** toggling between Pretty and JSON view modes, **Then** the content re-renders in the selected format

---

### User Story 5 - View Metadata (Priority: P2)

As a proxy admin, I want to see the full metadata JSON for any log entry, so I can inspect custom metadata fields passed with requests.

**Why this priority**: Metadata contains arbitrary key-value pairs that vary per deployment. Showing it as expandable JSON covers all use cases without needing to anticipate every field.

**Independent Test**: Click a log entry that has metadata; an expandable Metadata section shows the full JSON.

**Acceptance Scenarios**:

1. **Given** a log entry with metadata, **When** the detail panel opens, **Then** a collapsible "Metadata" section shows the JSON content with syntax highlighting
2. **Given** a log entry with empty metadata, **When** the detail panel opens, **Then** the Metadata section is omitted

---

### User Story 6 - Store Request Duration (Priority: P2)

As a proxy admin, I want request duration to be persisted in the database, so I can sort and filter logs by response time.

**Why this priority**: Duration is currently computed at display time from startTime/endTime. Storing it as a dedicated column enables efficient sorting and filtering in the logs table.

**Independent Test**: After a request completes, the duration in milliseconds is stored in the SpendLogs table; the logs table can sort by duration.

**Acceptance Scenarios**:

1. **Given** a request completes (success or failure), **When** the spend log is persisted, **Then** the `request_duration_ms` column contains the elapsed time in milliseconds
2. **Given** the logs table is displayed, **When** the user sorts by Duration column, **Then** logs are ordered by persisted duration value

---

### Edge Cases

- What happens when a log entry's associated ErrorLog record is missing? — Show the "Failed" status from metadata but omit the error detail section.
- What happens when the detail panel is open and the log list refreshes (live tail)? — The panel stays open with the currently selected log; if that log is still visible, highlight it.
- What happens when request/response payloads are very large? — Payloads are truncated at storage time to `MAX_STRING_LENGTH_PROMPT_IN_DB` characters (default 2048, configurable via env var) using a 35% front + 65% back split with a truncation marker. Same as LiteLLM.
- What happens when metadata JSON is deeply nested? — Render with a collapsible JSON viewer, collapsed by default beyond the first level.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST display a detail panel when a user clicks on any row in the Request Logs table
- **FR-002**: The detail panel MUST show Request Details: Model, Provider, Call Type, API Base
- **FR-003**: The detail panel MUST show Metrics: Prompt Tokens, Completion Tokens, Total Tokens, Cost, Duration, Cache Hit status, Start Time, End Time
- **FR-004**: For failed requests, the detail panel MUST display an error alert showing Error Code, Error Type, and Error Message from the associated ErrorLogs record
- **FR-005**: For failed requests with a traceback, the detail panel MUST show the traceback in a scrollable code block
- **FR-006**: The panel MUST show a "Request & Response" section when prompt storage is enabled; otherwise show a guidance notice
- **FR-007**: The panel MUST show a collapsible "Metadata" section with syntax-highlighted JSON when metadata is non-empty
- **FR-008**: System MUST store `request_duration_ms` in the SpendLogs table for each completed request
- **FR-009**: The detail panel MUST join SpendLogs with ErrorLogs (by request_id) to display error details for failed requests
- **FR-010**: The panel MUST be dismissible via close button or Escape key
- **FR-011**: Clicking a different row while the detail panel is open MUST update the detail panel to show the new log's details
- **FR-012**: The Request ID in the detail panel MUST be copyable (click-to-copy)
- **FR-013**: System MUST store request/response payloads in a separate RequestPayloads table (not in SpendLogs), joined by request_id, to avoid TOAST performance degradation on the main table
- **FR-014**: Payload storage MUST be controlled by a configuration flag; when disabled, no payload data is written
- **FR-015**: System MUST log each completed request to stdout with: request_id, model, prompt_tokens, completion_tokens, total_tokens, cost, duration_ms, status (success/failure), and key_hash — enabling operators to diagnose token usage spikes in real time

### Key Entities

- **SpendLog**: A record of a proxied API request — contains request_id, model, provider, tokens, cost, timestamps, metadata, cache info, and team/user associations. Primary entity for successful requests. Kept lean (no large payloads).
- **ErrorLog**: A record of a failed API request — contains request_id, status_code, error_type, error_message, traceback. Joined with SpendLog by request_id to enrich the detail view for failures.
- **RequestPayload**: Stores the request messages and response content for a proxied API call. Stored in a separate table to avoid TOAST performance cliffs on SpendLogs. Joined by request_id. Only populated when payload storage is enabled in config.
- **LogDetailView**: A composite view combining SpendLog + ErrorLog + RequestPayload data for the detail panel. Not a persisted entity — assembled at query time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view complete details for any log entry within 1 second of clicking
- **SC-002**: Failed request details show error code, type, and message for 100% of entries that have associated error records
- **SC-003**: All existing logs page functionality (filtering, pagination, live tail) continues to work without degradation
- **SC-004**: Request duration is accurately stored and sortable for all new requests after deployment
- **SC-005**: Enabling payload storage does not degrade SpendLogs table query performance (list view, aggregations)

## Assumptions

- The existing `SpendLogs` and `ErrorLogs` tables already capture sufficient data for the detail view; a new `RequestPayloads` table is needed for payload storage, and `request_duration_ms` is added to SpendLogs
- Request/response payload storage is an optional configuration; the detail view gracefully degrades when disabled
- The panel UI follows the existing templ + HTMX + templUI component patterns used throughout the tianjiLLM admin UI
- Error details are fetched by joining ErrorLogs on request_id; a SpendLog entry for a failed request may have zero or one associated ErrorLog
- Payload data is stored as JSONB in the separate RequestPayloads table; large payloads are truncated at storage time to `MAX_STRING_LENGTH_PROMPT_IN_DB` characters (default 2048, env var override)
