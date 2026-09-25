# Feature Specification: Move ccDBReader from UIHandler struct to function-level injection

**Feature Branch**: `HO-570-move-ccdbreader`
**Created**: 2026-04-12
**Status**: Draft
**Input**: Linear HO-570 — Remove test-only `ccDBReader` field from production `UIHandler` struct; inject via function parameter instead.
**Linear Issue**: HO-570

## Context

`ccDBReader` is a `rateLimitStateReader` field on `UIHandler` that exists solely for test injection. It is always `nil` in production — the fallback chain goes RateLimitStore → `callback.NewRateLimitDB(h.DB)`. This test seam pollutes the production struct and was flagged during HO-568 PR review (#128).

## User Scenarios & Testing

### User Story 1 - Clean production struct (Priority: P1)

As a developer reading `UIHandler`, I should not see test-only fields that are always nil in production.

**Why this priority**: The struct is the public API of the package; a test-only field is confusing and a maintenance burden.

**Independent Test**: All existing tests pass with zero behavior change. `ccDBReader` field is gone from `UIHandler`.

**Acceptance Scenarios**:

1. **Given** the current `UIHandler` struct, **When** I remove `ccDBReader`, **Then** the struct compiles without it and no production code references it.
2. **Given** the refactored code, **When** all existing tests run, **Then** every test passes with identical assertions.

---

### User Story 2 - Tests still inject DB stub (Priority: P1)

As a test author, I need a way to inject a `rateLimitStateReader` stub without a real DB connection.

**Why this priority**: Without test injection, DB fallback tests become untestable without PostgreSQL.

**Independent Test**: `TestLoadClaudeCodeData_DBFallback_*` tests pass using the new injection mechanism.

**Acceptance Scenarios**:

1. **Given** a test that needs a DB stub, **When** I call `loadClaudeCodeData` with a stub reader, **Then** it uses the stub instead of `h.DB`.
2. **Given** production code (no stub), **When** `loadClaudeCodeData` is called, **Then** it falls back to `callback.NewRateLimitDB(h.DB)` when RateLimitStore misses.

---

### Edge Cases

- **h.DB is nil AND no stub**: `loadClaudeCodeData` should produce cards with no DB data (same as today). Verified by existing `TestLoadClaudeCodeData_NoRateLimitData`.
- **h.DB is nil in production**: Reader helper returns nil; the nil-check in `loadClaudeCodeData` already handles this — no reader means skip DB lookup.

## Requirements

### Functional Requirements

- **FR-001**: System MUST remove the `ccDBReader` field from the `UIHandler` struct.
- **FR-002**: `loadClaudeCodeData` MUST accept a `rateLimitStateReader` parameter (or use a private helper) for DB fallback.
- **FR-003**: A private `dbReader()` helper method on `UIHandler` MUST centralize the `if h.DB != nil { callback.NewRateLimitDB(h.DB) }` logic so production callers don't duplicate it.
- **FR-004**: All 5 production callers MUST continue to work identically (zero behavior change).
- **FR-005**: All 6 existing DB fallback tests MUST pass by injecting the stub via function parameter.

### Key Entities

- **`rateLimitStateReader`**: Interface with single method `GetOAuthTokenRateLimitState`. Stays in `handler_claude_code.go`.
- **`UIHandler`**: Production struct — `ccDBReader` field removed.
- **`stubRateLimitDB`**: Test stub — injected via `loadClaudeCodeData` parameter instead of struct field.

## Alternatives Considered

| Approach | Pros | Cons | Decision |
|----------|------|------|----------|
| **Function parameter** (chosen) | Clean, explicit, no struct change needed beyond removal | Signature change touches 5 callers + tests | Best: explicit injection, Go-idiomatic |
| **Functional-option constructor** | Constructor-level injection | Over-engineered for single test seam; adds complexity | Rejected: too much ceremony |
| **Build tag for test seam** | Zero production impact | Complicates build; hard to maintain | Rejected |

Design detail: A private `func (h *UIHandler) dbReader() rateLimitStateReader` helper avoids repeating the nil-check in 5 production callers. Each caller passes `h.dbReader()` to `loadClaudeCodeData`.

## Success Criteria

- **SC-001**: `ccDBReader` field does not exist in `UIHandler` struct.
- **SC-002**: `go build ./...` succeeds.
- **SC-003**: `go test ./internal/ui/...` passes — all existing tests green, no new test failures.
- **SC-004**: No behavior change in production code paths (same fallback chain: RateLimitStore → DB → skip).

## Out of Scope

- Changing the `rateLimitStateReader` interface itself.
- Modifying `callback.NewRateLimitDB` or `callback.RateLimitDB`.
- Adding new tests beyond adapting existing ones.
- Refactoring the RateLimitStore lookup path.
