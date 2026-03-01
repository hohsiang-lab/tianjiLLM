# Feature Specification: ErrorLog Team ID for Request Log Filtering

**Feature Branch**: `001-errorlog-team-filter`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "HO-73: Add team_id to ErrorLogs table to support team filter in ListRequestLogs."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Team Admin Filters Request Logs by Team (Priority: P1)

A team admin opens the Request Logs dashboard and selects their team from the team filter dropdown. They expect to see **all** requests associated with their team — including both successful requests and failed requests that never produced a spend log entry. Currently, failed requests that have no corresponding spend entry (pure error requests) are always excluded from the team-filtered view because the system has no team context stored on error records.

**Why this priority**: This is the root bug. Team-scoped views are incomplete when error-only requests are silently omitted from filtered results, leading to misleading data for team admins and operators.

**Independent Test**: Can be fully tested by submitting a request with a team API key that triggers an upstream error (e.g., invalid model name), then filtering the Request Logs UI by that team's ID and verifying the failed request appears in the results.

**Acceptance Scenarios**:

1. **Given** a request was made using a team-scoped API key and the request failed before generating a spend record, **When** an admin filters Request Logs by that team, **Then** the failed request appears in the filtered results with its error status.

2. **Given** a request was made with no team context and it failed, **When** an admin filters Request Logs by any specific team, **Then** the teamless failed request does NOT appear in the filtered results.

3. **Given** a team filter is active, **When** the results page loads, **Then** the total request count (pagination) also reflects only team-scoped records, including error-only records.

---

### User Story 2 - Error Logs Carry Team Context at Write Time (Priority: P2)

When a proxied request fails, the system records an error log. If the request was made through a team-scoped API key, the team identifier must be captured and stored alongside the error record at the time of the failure.

**Why this priority**: The team filter fix depends entirely on this data being present. Without writing team_id on error record creation, the filtering in Story 1 cannot work.

**Independent Test**: Can be tested by inspecting the database directly after a failed team-scoped request — the ErrorLogs record for that request must have a non-null team_id matching the team of the API key used.

**Acceptance Scenarios**:

1. **Given** a request is authenticated with a team-scoped API key, **When** the request fails and an error log is created, **Then** the error log record contains the team identifier from the request context.

2. **Given** a request has no team context (e.g., master key or non-team key), **When** the request fails, **Then** the error log record's team_id field is null.

3. **Given** an existing ErrorLogs record that was created before this change, **When** it is queried through ListRequestLogs with no team filter, **Then** it appears as normal (backward compatibility).

---

### Edge Cases

- What happens when a request has both a spend log (with team_id) and an error log? The listing query returns only the spend log segment for that request — existing behavior is preserved.
- What happens when team_id is available in context but is an empty string? It should be treated as null (no team assigned), so the empty string does not match any team filter.
- What happens when the migration runs on a database with existing ErrorLogs rows? Existing rows retain null team_id — they appear in unfiltered views but are excluded from any team-specific filter.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The error record storage system MUST accept and persist an optional team identifier alongside each error record at the time the error is logged.
- **FR-002**: When a proxied request fails, the system MUST capture the team identifier from the authenticated request context and include it in the error record if present.
- **FR-003**: The Request Logs listing query MUST include error-only records (those without a corresponding spend entry) in team-filtered results when the error record carries a matching team identifier.
- **FR-004**: The Request Logs listing query MUST exclude error-only records from team-filtered results when those records have no team identifier.
- **FR-005**: The total request count query used for pagination MUST apply the same team filter logic to error-only records as the listing query.
- **FR-006**: All existing Request Logs behavior for unfiltered queries (no team filter) MUST remain unchanged — zero regression on unfiltered views.
- **FR-007**: The database schema change MUST be applied via an additive migration — existing error log records MUST NOT be deleted or modified.

### Key Entities

- **ErrorLog**: Represents a failed proxy request. Attributes: request_id, api_key_hash, model, provider, status_code, error_type, error_message, traceback, created_at. **New attribute**: team_id (optional — identifies the team whose API key was used for the request).
- **RequestLogRow**: A unified view row combining data from SpendLogs and ErrorLogs. Includes team_id. After this change, error-only rows surface the stored team_id from ErrorLogs instead of always returning null.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When filtering Request Logs by team, 100% of failed requests associated with that team (via their API key) appear in results — both those with spend records and those without.
- **SC-002**: Team-filtered request counts (pagination totals) match the actual number of visible rows, including error-only records with a matching team identifier.
- **SC-003**: All existing Request Logs filters (unfiltered, model filter, API key filter, status filter, date range filter) return identical result sets before and after this change.
- **SC-004**: Failed requests made with non-team API keys continue to be excluded from all team-filtered views — no cross-team data leakage.

## Assumptions

- The authenticated request context already carries team identifier information when a team-scoped API key is used; no changes to the authentication or context-population middleware are required.
- The `CountRequestLogs` pagination query mirrors the same dual-segment pattern as `ListRequestLogs` and also requires the same team filter fix for error-only records.
- The nullable team_id field uses the same text type as the existing team_id in SpendLogs.
- No additional index on team_id in ErrorLogs is needed for initial delivery given current data volumes.
