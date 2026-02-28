# Feature Specification: Reduce Auth DB Query (HO-10)

**Feature Branch**: `001-reduce-auth-db-query`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "HO-10: Reduce double DB query on virtual key auth. Currently every virtual key authentication triggers 2 identical GetVerificationToken DB queries: (1) ValidateToken() validates the key, (2) GetGuardrails() loads guardrail policies. Both call d.DB.GetVerificationToken() separately in internal/proxy/middleware/db_validator.go. Fix: merge into a single query by having ValidateToken return the full VerificationToken struct, then GetGuardrails reads from the already-fetched result instead of querying again. This is a pure refactor with no behavior change — only reducing redundant DB calls on the auth hot path."

## Overview

Every API request authenticated with a virtual key currently triggers two identical database lookups: one to validate the key and one to load its associated guardrail policies. Both lookups fetch the same record. This redundancy on the authentication hot path wastes database resources and adds unnecessary latency to every request.

The goal is to consolidate these two lookups into one, passing the already-fetched record to the guardrail step instead of repeating the query. The external behavior — authentication outcomes, error handling, guardrail enforcement — must remain entirely unchanged.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Single Lookup Per Auth (Priority: P1)

An API consumer sends a request authenticated with a virtual key. The system validates the key and loads its guardrail policies in a single database operation instead of two separate identical operations.

**Why this priority**: This is the core of the refactor. The authentication hot path is exercised on every virtual-key request. Eliminating the duplicate lookup reduces database load and latency for all API consumers.

**Independent Test**: Can be verified by observing database query counts during a virtual-key authenticated request — exactly one token lookup should occur, not two.

**Acceptance Scenarios**:

1. **Given** a valid virtual key, **When** a request is authenticated, **Then** the system validates the key and retrieves guardrail policies without issuing duplicate lookups to the token store.
2. **Given** an invalid or non-existent virtual key, **When** a request is authenticated, **Then** the system returns an authentication error identical to current behavior — no behavior change.
3. **Given** a blocked virtual key, **When** a request is authenticated, **Then** the system rejects the request with the same error as before — no behavior change.

---

### User Story 2 - Guardrail Policies Unaffected (Priority: P2)

An API consumer whose virtual key has associated guardrail policies sends a request. The system enforces those policies exactly as before.

**Why this priority**: Guardrail policy enforcement is a correctness requirement. Any regression here — missing policies, wrong policies — would be a security issue.

**Independent Test**: Can be verified by sending requests through a virtual key that has guardrail policies attached and confirming that policy enforcement behaves identically to before the change.

**Acceptance Scenarios**:

1. **Given** a virtual key with one or more guardrail policies, **When** the request is authenticated, **Then** all policies are applied in the same order and manner as before the change.
2. **Given** a virtual key with no guardrail policies, **When** the request is authenticated, **Then** no policies are applied — consistent with current behavior.

---

### Edge Cases

- What happens when the token store is unavailable (database down)? Authentication must fail with the same error type and message as before — no change in error handling behavior.
- What happens when the virtual key exists but has no guardrail policies? An empty policy list must be returned, identical to current behavior.
- What happens if the key record changes between the validation step and the guardrail step in the current implementation? After the refactor, this race window is eliminated — the single lookup guarantees consistency between validation and guardrail data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST perform exactly one token record lookup per virtual-key authentication request (currently two identical lookups occur).
- **FR-002**: System MUST return authentication success or failure with identical outcomes to the pre-change behavior for all key states (valid, invalid, blocked, missing).
- **FR-003**: System MUST supply guardrail policy names to the policy enforcement step using data obtained from the single token lookup.
- **FR-004**: System MUST NOT alter any externally observable behavior: authentication error codes, response formats, guardrail policy enforcement, and audit/spend-log attribution must remain unchanged.
- **FR-005**: System MUST maintain database-unavailability error handling: if the token store cannot be reached, requests must fail in the same manner as before.

### Key Entities

- **Virtual Key (VerificationToken)**: An opaque API key issued to a client. Carries identity attributes (user, team), a blocked flag, and a list of associated guardrail policy names. All fields are fetched in a single lookup after this change.
- **Guardrail Policy**: A named policy attached to a virtual key that controls request behavior (e.g., content filtering). After this change, policies are derived from the same record fetch as the key validation step.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every virtual-key authenticated request results in exactly one token store lookup — verified by database query count instrumentation or test assertions.
- **SC-002**: All existing authentication test cases pass without modification — zero regression in authentication outcomes (accept / reject / block).
- **SC-003**: All existing guardrail policy enforcement test cases pass without modification — policies applied correctly for keys with and without policies.
- **SC-004**: No change in error responses returned to API consumers for any key state — invalid, blocked, or missing keys produce identical error payloads.

## Assumptions

- The token record fetched during validation contains all fields needed by the guardrail step — no additional query is required for policy data.
- The refactor does not require changes to the database schema, query definitions, or any caller outside the authentication middleware component.
- Behavioral equivalence is the sole acceptance bar; performance benchmarks are not required as part of this change's acceptance criteria.
