# Feature Specification: Reduce Virtual Key Auth DB Query

**Feature Branch**: `003-reduce-auth-db-query`
**Created**: 2026-02-28
**Status**: Draft
**Input**: HO-10: Reduce double DB query on virtual key auth. Both ValidateToken() and GetGuardrails() in db_validator.go call DB.GetVerificationToken() separately, causing 2 identical DB queries per request. ValidateToken should return the full VerificationToken struct, and GetGuardrails should reuse the already-fetched result.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - API Request Completes With Less Database Load (Priority: P1)

Every time an API consumer sends a request using a virtual key, the proxy system must validate the key and check its associated guardrail policies before forwarding the request. Currently, these two checks each independently query the database for the same record, doubling the database load on every authenticated request. After this change, both checks share a single database query result.

**Why this priority**: Every authenticated API request triggers this code path. Reducing the query count from 2 to 1 directly lowers database load and latency for every request in the system.

**Independent Test**: Can be fully tested by sending an API request with a virtual key and confirming the proxy validates the key, enforces guardrails, and forwards the request — all with a single database lookup recorded.

**Acceptance Scenarios**:

1. **Given** a valid virtual key with guardrail policies attached, **When** an API request is made, **Then** the system validates the key, retrieves guardrail policies, and forwards the request using a single database lookup — not two separate lookups.
2. **Given** a valid virtual key with no guardrail policies, **When** an API request is made, **Then** the system validates the key with a single database lookup and proceeds without a second lookup.
3. **Given** an invalid or non-existent virtual key, **When** an API request is made, **Then** the system returns an authentication error after a single database lookup and does not attempt a second lookup.
4. **Given** a blocked virtual key, **When** an API request is made, **Then** the system rejects the request with a single database lookup and does not attempt a second lookup.

---

### User Story 2 - Operator Observes Lower Database Query Rate (Priority: P2)

System operators and administrators who monitor database metrics should see a measurable reduction in the rate of `GetVerificationToken` queries after this change is deployed, without any change in API behavior.

**Why this priority**: This is the primary measurable business outcome of the optimization. Operators need to verify the change worked as intended.

**Independent Test**: Can be fully tested by monitoring database query logs before and after deployment while maintaining the same API request volume.

**Acceptance Scenarios**:

1. **Given** a steady load of virtual-key-authenticated API requests, **When** this change is deployed, **Then** the rate of database verification token lookups per request drops from 2 to 1.
2. **Given** the change is deployed, **When** an operator examines request traces, **Then** each authenticated request shows exactly one database token lookup rather than two.

---

### Edge Cases

- What happens when the database returns an error mid-request? The single lookup result must propagate the error correctly so neither the validation nor the guardrail check silently succeeds.
- What happens when guardrails are configured but the database lookup fails? The system should reject the request rather than proceeding without guardrail enforcement.
- What happens when a token has no policies? The system must not make a second lookup to confirm the absence of policies.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST perform exactly one database lookup per virtual-key-authenticated request, combining the results used by both token validation and guardrail policy retrieval.
- **FR-002**: The system MUST continue to validate virtual key existence, blocked status, user ID, and team ID with the same correctness as before.
- **FR-003**: The system MUST continue to retrieve and enforce all guardrail policies associated with a virtual key.
- **FR-004**: The system MUST propagate database errors to both the authentication and guardrail checks — a failed lookup must result in request rejection, not partial success.
- **FR-005**: The system MUST NOT change the external API behavior — authenticated requests must succeed or fail under the same conditions as before.
- **FR-006**: The system MUST remain backward-compatible with any existing tests or callers that depend on the current token validation and guardrail interfaces.

### Key Entities

- **VerificationToken**: The full record retrieved from the database for a virtual key. Contains user ID, team ID, blocked status, and associated guardrail policy names. After this change, a single fetch of this record satisfies both validation and guardrail lookups within the same request.
- **Virtual Key**: An API key issued to an API consumer. Identified by its SHA256 hash. Maps to a VerificationToken record.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Each virtual-key-authenticated request triggers exactly one database token lookup, reducing the previous count of two identical lookups to one.
- **SC-002**: Database query throughput for token lookups is reduced by 50% relative to authenticated request volume (from 2× to 1× requests).
- **SC-003**: All existing authentication and guardrail behaviors pass their current test suites without modification to test expectations.
- **SC-004**: Response latency for authenticated requests does not increase compared to the baseline; typical cases show equal or lower latency.

## Assumptions

- The `GetVerificationToken` database query is the only shared lookup between token validation and guardrail retrieval; no additional queries are being duplicated.
- The guardrail check is always performed immediately after token validation within the same request lifecycle, making it safe to reuse the in-memory result without staleness concerns.
- There is no caching layer between the middleware and the database for this lookup; the optimization is purely about sharing the in-memory query result within a single request.
- The change is scoped to the `db_validator.go` file and its callers; no schema changes or database-side modifications are required.
