# Feature Specification: Discord OpenAI Credential Usage Report

**Feature Branch**: `1467-discord-openai-usage-report`

**Created**: 2026-08-14

**Status**: Draft

**Input**: User description: "Send each OpenAI credential's usage to a Discord channel every 30 minutes."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Receive a recurring credential-usage report (Priority: P1)

As a Tianji operator, I want a recurring Discord report that lists the safe
Codex usage state of every OpenAI subscription credential so I can compare
available capacity without opening the Tianji UI.

**Why this priority**: This is the requested operational outcome and provides
value on its own.

**Independent Test**: Seed multiple OpenAI subscription credentials with safe
usage snapshots, trigger one reporting period, and verify that the configured
Discord destination receives a readable report with one row per credential.

**Acceptance Scenarios**:

1. **Given** a Discord destination is configured and several OpenAI
   subscription credentials have current usage snapshots, **When** a reporting
   period runs, **Then** one report for that period lists every credential by
   its safe credential identifier with plan when known, primary short-window
   usage, weekly usage, reset information, and status.
2. **Given** a credential has optional additional usage buckets, **When** its
   report row is rendered, **Then** the row includes the known additional
   bucket information without omitting its primary or weekly usage.
3. **Given** the report is longer than one Discord message, **When** it is
   delivered, **Then** operators receive an ordered, readable set of messages
   for the same reporting period.
4. **Given** no OpenAI subscription credentials exist, **When** a reporting
   period runs, **Then** the configured destination receives a clearly labeled
   zero-credential operational report rather than a silent no-work skip.

---

### User Story 2 - See safe unavailable and stale credential states (Priority: P1)

As a Tianji operator, I want the report to identify credentials whose usage is
stale, unavailable, disabled, or requires reconnecting, so I can distinguish
capacity from an account that needs attention.

**Why this priority**: A report that silently excludes unhealthy credentials
would make the credential inventory misleading.

**Independent Test**: Seed active, disabled, reconnect-required, stale, and
usage-fetch-failed credentials; trigger one reporting period; verify every
credential has a safe row and that no secret or raw upstream data appears.

**Acceptance Scenarios**:

1. **Given** a credential is disabled or requires reconnecting, **When** a
   report runs, **Then** it appears as a safe status row without attempting to
   revive or use that credential.
2. **Given** a credential has a last-known snapshot that is stale or a safe
   fetch-failure state, **When** a report runs, **Then** its row identifies
   that condition and preserves only safe last-known information.
3. **Given** an upstream response or credential metadata contains secret
   material, **When** report content is produced, **Then** the Discord
   messages, logs, and delivery failures contain no token, credential bundle,
   webhook URL, or raw upstream payload.

---

### User Story 3 - Keep periodic reporting operationally isolated (Priority: P2)

As a Tianji operator, I want reporting failures or multiple service replicas
to avoid creating misleading report storms or affecting proxy traffic.

**Why this priority**: The report is observability, not a reason to degrade
normal model requests.

**Independent Test**: Exercise missing destination configuration, an
unreachable destination, and two concurrent report runners; verify that
normal proxy traffic remains unaffected, failures are safe, and distributed
coordination permits only one runner to start delivery for one reporting
period.

**Acceptance Scenarios**:

1. **Given** no Discord destination is configured, **When** a reporting period
   occurs, **Then** Tianji skips delivery without failing normal proxy
   operation.
2. **Given** Discord delivery fails, **When** the report runner handles the
   failure, **Then** it records a sanitized operational result and a later
   reporting period remains eligible to deliver a new report.
3. **Given** multiple Tianji replicas run the same reporting period with
   distributed coordination available, **When** they coordinate report
   delivery, **Then** only one runner starts delivery for that period.

### Edge Cases

- No OpenAI subscription credentials exist when a reporting period runs; send
  a clearly labeled zero-credential operational report when a destination is
  configured.
- A credential is added, deleted, disabled, or re-enabled while a report is
  being assembled.
- Snapshot data is partially available, stale, in backoff, or missing.
- A report needs more than one Discord message.
- The Discord destination rejects, rate-limits, or times out a delivery.
- The service starts between expected half-hour boundaries; the first report
  must not imply that an earlier reporting period was delivered.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST evaluate all OpenAI subscription credentials
  during each reporting period and render one safe report row per credential.
- **FR-002**: The system MUST publish a report every 30 minutes when a Discord
  destination is configured and reporting is available, including a clearly
  labeled zero-credential report when no OpenAI subscription credentials
  exist.
- **FR-003**: Each report row MUST distinguish current, stale, unavailable,
  disabled, and reconnect-required usage states.
- **FR-004**: Each current usage row MUST show the known primary short-window
  usage, weekly usage, reset information, plan when known, and known
  additional usage buckets.
- **FR-005**: The report MUST identify that its usage values are a best-effort
  operational snapshot and not a subscription invoice or billing statement.
- **FR-006**: Disabled, malformed, or reconnect-required credentials MUST
  appear only as safe status rows and MUST NOT be revived or used for normal
  proxy traffic because of reporting. A credential deleted before report
  enumeration is omitted; one deleted after enumeration is represented by a
  safe unavailable row.
- **FR-007**: Report delivery MUST use the existing configured Discord
  destination; absent configuration MUST result in a safe no-delivery outcome.
- **FR-008**: A long report MUST remain readable through an ordered delivery
  sequence that identifies the same reporting period.
- **FR-009**: Report content, delivery failures, and related logs MUST exclude
  access tokens, refresh tokens, bearer/JWT values, encrypted credential
  material, webhook URLs, raw upstream responses, raw upstream errors, email
  addresses, credential names, and organization identifiers.
- **FR-010**: A report-delivery failure MUST NOT block, delay, or alter normal
  model proxy traffic, credential routing, OAuth lifecycle, or credential
  management behavior.
- **FR-011**: Concurrent Tianji replicas MUST use existing distributed
  coordination so only one runner starts delivery for a reporting period.
  Delivery is best-effort; an uncertain external-network outcome MUST NOT be
  presented as an exactly-once delivery guarantee.
- **FR-012**: Automated coverage MUST exercise successful delivery, partial
  credential status reporting, long-report delivery, absent configuration,
  delivery failure, and multi-replica coordination without real credentials
  or a real Discord destination.

### Key Entities *(include if feature involves data)*

- **Credential Usage Report**: One operator-facing summary for a single
  half-hour reporting period.
- **Credential Report Row**: The safe identity, usage snapshot, reset
  information, and operational status for one OpenAI subscription credential.
- **Report Delivery Outcome**: The safe result of a report attempt, including
  delivered, skipped, or failed without secret-bearing diagnostic data.
- **Reporting Period**: The half-hour interval represented by a logical
  report, used to distinguish ordered message parts and prevent duplicate
  delivery.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In automated successful-delivery coverage, 100% of seeded OpenAI
  subscription credentials appear exactly once in the report rows for a
  reporting period.
- **SC-002**: In automated delivery coverage, a report with up to 50 seeded
  credential rows is emitted in readable ordered messages within 60 seconds
  of the reporting job starting.
- **SC-003**: Automated redaction coverage proves that 0 known secret-bearing
  fixture values appear in Discord content, logs, or recorded delivery
  failures.
- **SC-004**: Automated failure coverage proves that an absent or failed
  Discord destination causes 0 changes to normal proxy request outcomes.
- **SC-005**: Automated multi-replica coverage proves that one reporting
  period permits only one runner to start delivery.

## Assumptions

- “Usage” means Tianji's existing safe Codex subscription-usage snapshot, not
  Tianji request token totals, API-equivalent spend, an OpenAI invoice, or a
  fixed subscription quota.
- The first version reuses Tianji's existing configured Discord destination;
  separate channels, per-organization destinations, and a UI configuration
  surface are out of scope.
- The existing Discord destination is an instance-operator channel already
  authorized for global operational status. Reports send credential IDs only
  and do not disclose email, credential name, or organization data.
- Reports include every OpenAI subscription credential as a safe row; a
  non-selectable credential is represented by status rather than an upstream
  usage fetch.
- The reporting cadence is every 30 minutes after the scheduler starts. Exact
  wall-clock alignment to `HH:00` and `HH:30` is not required for the first
  version.
- A multi-replica rollout requires Tianji's existing distributed coordination;
  without it, reporting is limited to a single replica.
- Delivery may retry one confirmed Discord rate-limit response within the
  reporting budget. A network outcome whose delivery status is unknown is not
  retried and must not be presented as an exactly-once delivery guarantee.
- The first version provides operational observability only and does not
  change routing, billing, OAuth, credential lifecycle, or database schema.
