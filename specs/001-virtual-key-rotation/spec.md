# Feature Specification: Virtual Key Rotation

**Feature Branch**: `001-virtual-key-rotation`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add virtual key rotation support: allow virtual keys to be automatically rotated on a configurable schedule to reduce key leakage risk. Include rotation policy configuration, graceful key transition (old key still valid during grace period), rotation history/audit log, and manual rotation trigger."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Rotation Policy (Priority: P1)

As a security-conscious administrator, I want to configure an automatic rotation schedule for a virtual key so that the key value changes regularly without requiring manual intervention, reducing the blast radius of any potential key leakage.

**Why this priority**: This is the core feature — without a configurable rotation policy, none of the downstream capabilities (automatic rotation, grace period, history) make sense. It must exist first and delivers immediate value by itself.

**Independent Test**: Can be fully tested by navigating to a virtual key's settings, enabling rotation, setting a 7-day interval and 24-hour grace period, saving the policy, and verifying the policy is stored and displayed correctly.

**Acceptance Scenarios**:

1. **Given** an administrator is viewing a virtual key, **When** they enable rotation with a 30-day interval and 48-hour grace period, **Then** the policy is saved and displayed on the key detail page with the next scheduled rotation date.
2. **Given** an active rotation policy exists, **When** the administrator changes the interval from 30 days to 14 days, **Then** the policy is updated and the next rotation date recalculates accordingly.
3. **Given** an active rotation policy exists, **When** the administrator disables the policy, **Then** no further automatic rotations occur and the key retains its current value.

---

### User Story 2 - Automatic Key Rotation with Grace Period (Priority: P2)

As an administrator, I want the system to automatically issue a new key value on the configured schedule while keeping the old key valid for the grace period, so that my clients have time to update their configurations without experiencing downtime.

**Why this priority**: The grace period is the critical safety mechanism that makes rotation practical. Without it, rotation would cause immediate outages for any client that hasn't yet updated to the new key.

**Independent Test**: Can be fully tested by setting a short rotation interval (e.g., 1 minute in a test environment), observing that: (a) a new key is generated, (b) both old and new keys are accepted for authentication, (c) after the grace period the old key is rejected.

**Acceptance Scenarios**:

1. **Given** a virtual key has a rotation policy with a 24-hour grace period, **When** the scheduled rotation time arrives, **Then** a new key value is generated, the old value remains valid for exactly 24 hours, and both values authenticate successfully during that window.
2. **Given** a key is in its grace period (old and new key both valid), **When** the grace period expires, **Then** the old key is rejected with an authentication error and only the new key authenticates successfully.
3. **Given** a rotation policy is active, **When** the system performs automatic rotation, **Then** a rotation event is recorded in the audit log with timestamp, trigger type "automatic", and both old and new key identifiers.

---

### User Story 3 - Manual Rotation Trigger (Priority: P3)

As an administrator who suspects a virtual key has been compromised, I want to immediately trigger a key rotation on demand, so that the potentially leaked key is replaced as quickly as possible.

**Why this priority**: Incident response requires immediate action. Manual rotation is the emergency brake — it must work independently of any scheduled policy and deliver immediate security value.

**Independent Test**: Can be fully tested by clicking "Rotate Now" on any virtual key (with or without a rotation policy), verifying a new key value is issued immediately, and confirming the old key remains valid only during the configured grace period (or is immediately invalidated if grace period is 0).

**Acceptance Scenarios**:

1. **Given** a virtual key exists, **When** an administrator clicks "Rotate Now", **Then** a new key value is generated within 5 seconds and displayed to the administrator exactly once.
2. **Given** a manual rotation is triggered with a grace period of 0, **When** the rotation completes, **Then** the old key is immediately invalid and any request using it receives an authentication error.
3. **Given** a manual rotation is triggered while the key is already in a grace period from a previous rotation, **When** the new rotation completes, **Then** only the newly generated key and the most recent previous key (still within its grace period) are valid; any older keys are immediately invalidated.
4. **Given** a manual rotation is triggered, **When** the rotation completes, **Then** a rotation event is recorded in the audit log with trigger type "manual" and the administrator's identity.

---

### User Story 4 - View Rotation History and Audit Log (Priority: P4)

As a security auditor or compliance officer, I want to see the complete rotation history for any virtual key, so that I can verify the key rotation policy is being followed and investigate any anomalies.

**Why this priority**: Audit and compliance requirements exist even if rotation is not yet happening frequently. This story independently delivers visibility and accountability value.

**Independent Test**: Can be fully tested by triggering several rotations (manual and automatic) and then viewing the rotation history page for the key, verifying each event appears with correct timestamp, trigger type, and outcome.

**Acceptance Scenarios**:

1. **Given** a virtual key has had 5 rotations, **When** an administrator views the rotation history for that key, **Then** all 5 events are listed in reverse chronological order with: timestamp, trigger type (automatic/manual), outcome (success/failure), and administrator name for manual triggers.
2. **Given** a rotation history exists, **When** an administrator filters by date range, **Then** only rotation events within that range are displayed.
3. **Given** no rotations have occurred for a key, **When** an administrator views the rotation history, **Then** an empty state message is shown with guidance on how to configure rotation.

---

### Edge Cases

- What happens when a scheduled rotation fails (e.g., database unavailable)? The system retries the rotation up to 3 times within the same hour before recording a "failed" rotation event. The old key remains valid until a successful rotation occurs.
- What if a key reaches its spending budget limit when rotated? The new key inherits the same budget policy limits but resets the current spend counter. Budget limits follow the policy, not the key version.
- What if the rotation policy is deleted while an old key version is still within its grace period? The grace period for the already-rotating key is honored until it expires; no further automatic rotations occur.
- What if a grace period is set to 0 hours? The old key is invalidated immediately upon rotation. This is valid and expected behavior for high-security scenarios.
- What if the system restarts during a rotation? The rotation operation must be idempotent — a partial rotation is either completed or rolled back on restart; no key value is lost.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow administrators to configure a rotation policy on any virtual key, specifying a rotation interval expressed in days (minimum 1 day).
- **FR-002**: System MUST allow administrators to set a grace period per rotation policy, expressed in hours, during which the previous key value remains valid (minimum 0 hours, maximum 72 hours).
- **FR-003**: System MUST automatically rotate virtual keys according to their configured rotation policies without administrator intervention.
- **FR-004**: System MUST keep the previous key value valid for authentication for exactly the configured grace period after a rotation event.
- **FR-005**: System MUST invalidate the previous key value immediately when its grace period expires.
- **FR-006**: Administrators MUST be able to trigger an immediate manual rotation of any virtual key, regardless of whether an automatic policy is configured.
- **FR-007**: System MUST display the newly generated key value to the triggering administrator exactly once after a rotation (manual or scheduled), with explicit instruction to save it.
- **FR-008**: System MUST record every rotation event (automatic and manual) in an immutable audit log, capturing: timestamp, trigger type, outcome, and for manual triggers — the identity of the initiating administrator.
- **FR-009**: Administrators MUST be able to view the rotation history for any virtual key, with events listed in reverse chronological order.
- **FR-010**: System MUST allow administrators to disable a rotation policy without deleting the key or its history.
- **FR-011**: System MUST display the next scheduled rotation date/time on the key detail page when a rotation policy is active.
- **FR-012**: When a manual rotation is triggered while a key is already in a grace period, the system MUST immediately invalidate any key versions older than the immediately preceding one, so that at most two key values are simultaneously valid at any time.
- **FR-013**: System MUST retry failed automatic rotations up to 3 times within a configurable window before recording a failed event and alerting via audit log.

### Key Entities

- **Rotation Policy**: Defines the automatic rotation schedule for a single virtual key. Attributes: rotation interval in days, grace period in hours, enabled/disabled status, next scheduled rotation timestamp. A key has at most one active rotation policy.
- **Key Version**: A specific key value for a virtual key at a point in time. Attributes: key value (hashed for storage), created at timestamp, expires at timestamp (grace period end), status (active, grace-period, expired). Multiple versions may coexist only during grace periods.
- **Rotation Event**: An immutable audit record for a single rotation occurrence. Attributes: key identifier, timestamp, trigger type (automatic/manual), triggering administrator (for manual), outcome (success/failure), failure reason (if applicable), previous key version reference, new key version reference.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Administrators can configure and activate a rotation policy for a virtual key in under 60 seconds from the key detail page.
- **SC-002**: Automatic key rotation completes without service interruption — authenticated requests using the old key continue to succeed throughout the entire configured grace period.
- **SC-003**: 100% of rotation events (automatic and manual, success and failure) are recorded in the audit log with accurate timestamps and complete metadata.
- **SC-004**: Manual rotation requests complete and return the new key value within 5 seconds of the administrator's confirmation.
- **SC-005**: Scheduled automatic rotations execute within a 5-minute window of the configured rotation time.
- **SC-006**: Administrators can locate and view the full rotation history for any key within 3 clicks from the key listing page.
- **SC-007**: Failed automatic rotations are retried and the failure is surfaced in the audit log within 1 hour, without the existing key being invalidated.

## Assumptions

- Rotation policies apply at the individual virtual key level, not at a team or organization level. Global default policies are out of scope for this feature.
- The system has an existing scheduler or background job mechanism that can be configured to run key rotation tasks at defined intervals.
- Administrators will be responsible for distributing newly rotated key values to their clients; proactive notification (email/webhook) of rotation events is out of scope for this initial version.
- The rotation history is retained indefinitely (no automatic purge). Configurable retention limits are out of scope for this feature.
- A virtual key undergoing rotation retains all other attributes (name, team, model restrictions, budget policy) on both the old and new versions.
- The new key value generated on rotation follows the same format and length as the original virtual key format.
