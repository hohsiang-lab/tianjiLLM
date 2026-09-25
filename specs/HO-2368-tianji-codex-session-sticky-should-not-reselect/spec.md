# Feature Specification: HO-2368 Codex session sticky reuse across primary reset

**Feature Branch**: `HO-2368-tianji-codex-session-sticky-should-not-reselect`
**Created**: 2026-07-09
**Status**: Draft
**Input**: Linear HO-2368 "Tianji Codex session sticky should not reselect credential on primary reset change"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Session affinity survives primary reset changes (Priority: P1)

As a Codex client using a session-aware OpenAI subscription route, I want an existing session to keep using the same upstream credential when only the credential's primary reset timestamp changes, so a long-running Codex session does not unexpectedly switch tokens while the selected credential remains usable.

**Why this priority**: This is the reported production follow-up from HO-2360 verification. It is the smallest behavior change that fixes the observed `reason=primary_reset_changed` session switch without changing broader routing policy.

**Independent Test**: Seed a session-aware sticky route to a selectable credential, change only that credential's `PrimaryResetAt`, and verify the same session keeps the same selected credential.

**Acceptance Scenarios**:

1. **Given** a `codex-session` route is sticky to `cred-a` and fresh usage metadata for `cred-a` remains known and selectable, **When** `cred-a` receives a different primary window reset timestamp, **Then** the same session continues to route to `cred-a`.
2. **Given** a non-session fallback OpenAI subscription track is sticky to `cred-a`, **When** `cred-a` receives a different primary window reset timestamp, **Then** existing org/model-level primary-reset re-evaluation behavior is preserved.

---

### User Story 2 - Session stickiness still yields to unusable credentials (Priority: P1)

As an operator, I want session stickiness to release a credential when the selected credential is no longer safe to use, so affinity does not keep traffic on an exhausted, gated, disabled, missing, or unavailable credential.

**Why this priority**: Removing primary-reset equality from session-aware reuse must not bypass the existing Codex usage gates.

**Independent Test**: Seed a session-aware sticky route to a credential, then make the credential non-selectable through primary or secondary usage gates and verify the same session reselects a remaining selectable credential.

**Acceptance Scenarios**:

1. **Given** a `codex-session` route is sticky to `cred-a`, **When** `cred-a` exceeds the primary usage gate and `cred-b` remains selectable, **Then** the session reselects `cred-b`.
2. **Given** a `codex-session` route is sticky to `cred-a`, **When** `cred-a` exceeds the weekly/secondary usage gate and `cred-b` remains selectable, **Then** the session reselects `cred-b`.
3. **Given** a `codex-session` route is sticky to `cred-a`, **When** current usage metadata for `cred-a` is unknown in a state where reuse cannot be trusted, **Then** existing conservative unknown-handling behavior continues to decide whether to reselect.

---

### User Story 3 - Observability explains session-specific reuse decisions (Priority: P2)

As an operator investigating Codex routing, I want logs to distinguish session-aware primary-reset reuse from fallback-track primary-reset re-evaluation, so verification can prove the fix without exposing raw session IDs.

**Why this priority**: HO-2368 verification depends on logs no longer showing session-aware switches caused only by `primary_reset_changed`; existing log redaction must remain intact.

**Independent Test**: Enable Codex routing debug logs for a raw session id and verify session-aware tracks remain redacted while the primary-reset-only reuse path does not emit the old reselect reason for session tracks.

**Acceptance Scenarios**:

1. **Given** debug logging is enabled for a session-aware route, **When** the session reuses a selectable credential despite primary reset change, **Then** logs either stay quiet or use a reuse-specific reason such as `primary_reset_changed_reused`.
2. **Given** any Codex session-aware route is logged, **When** logs include the route key, **Then** the raw session id is not emitted and the redacted `codex-session:sha256:<digest>` shape is preserved.

### Edge Cases

- Selected credential exists in the configured route but has no known current usage metadata.
- Selected credential becomes known but `Selectable=false` because primary usage crosses `RatelimitAlertThreshold`.
- Selected credential becomes known but `Selectable=false` because weekly usage crosses `CodexUsageWeeklyThreshold`.
- Selected credential becomes unavailable/exhausted from the Codex usage snapshot status.
- Selected credential is disabled, refresh-failed, removed from `OpenAISubscriptionCredentialIDs`, or otherwise absent from the candidate pool before sticky reuse is evaluated.
- Session id is missing or blank, so routing falls back to the existing non-session route key and keeps the old primary-reset behavior.
- Raw Codex session ids appear in route keys before logging and must remain redacted in log output.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For OpenAI subscription route keys containing `:codex-session:`, the system MUST reuse the existing sticky credential when the candidate remains present, current usage metadata is known, and `Selectable=true`, regardless of `PrimaryResetAt` equality.
- **FR-002**: For OpenAI subscription route keys without `:codex-session:`, the system MUST preserve existing org/model-level sticky behavior where a changed `PrimaryResetAt` can trigger re-evaluation.
- **FR-003**: Session-aware sticky reuse MUST return `false` when current metadata is known and `Selectable=false`, preserving primary usage, weekly usage, unavailable/exhausted snapshot, and configured threshold gates.
- **FR-004**: Session-aware sticky reuse MUST preserve the existing conservative behavior for unknown current usage metadata unless implementation review proves a narrower safe rule from repo evidence.
- **FR-005**: Credential disabled/removed/auth-failed/rate-limited filtering MUST continue to happen before sticky reuse, so a session cannot reuse a credential absent from the selectable candidate pool.
- **FR-006**: Observability MUST not emit raw Codex session ids; any route key logged for a session-aware track MUST use the existing redacted `codex-session:sha256:<digest>` shape.
- **FR-007**: Verification MUST include failing-tests-first coverage for primary-reset-only session reuse, primary-gate reselect, secondary-gate reselect, and non-session fallback behavior.
- **FR-008**: The feature MUST NOT change rendezvous hashing, route-key construction, UI, database schema, config format, or non-Codex routing.

### Key Entities

- **OpenAI subscription route key**: Sticky track identifier. Session-aware keys contain `:codex-session:`; fallback org/model keys do not.
- **Sticky strategy entry**: Stored selected credential id plus metadata from the previous selection.
- **Sticky strategy candidate**: Current candidate credential plus current `codexStickyMetadata`.
- **Codex sticky metadata**: Current usage-derived metadata containing `Known`, `Available`, `Selectable`, `PrimaryResetAt`, `SecondaryResetScore`, and selection score.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Unit test proves a session-aware sticky track keeps the same credential when only `PrimaryResetAt` changes and the credential remains selectable.
- **SC-002**: Unit tests prove session-aware sticky routing still reselects when primary or secondary usage gates make the selected credential non-selectable.
- **SC-003**: Existing non-session primary-reset re-evaluation test continues to pass or is replaced with equivalent coverage showing fallback behavior is unchanged.
- **SC-004**: Log redaction coverage continues to prove raw session ids are absent from Codex routing logs.
- **SC-005**: Focused handler tests pass with `go test ./internal/proxy/handler -run 'Codex.*Sticky|OpenAISubscriptionRouting_StickyCodex|CodexSession' -count=1`.

## Clarifications

### Session 2026-07-09

- Q: Should non-session OpenAI subscription sticky behavior change too? -> A: No; Linear HO-2368 explicitly marks non-session fallback behavior as out of scope.
- Q: Is UI evidence required? -> A: No; issue scope, repo code confirmation, and Linear non-goals show this is backend routing policy only.
