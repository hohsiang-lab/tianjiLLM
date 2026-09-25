# Feature Specification: Normalize Expired Windows in DB Fallback Path

**Feature Branch**: `HO-568-progress-bar-reset`  
**Created**: 2026-04-12  
**Status**: Implemented  
**Linear**: HO-568

> Informed by memory: no prior memory found — first encounter of this pattern.

## Root Cause

`loadClaudeCodeData` (handler_claude_code.go) reads rate-limit state from two paths:

1. **In-memory store** (`h.RateLimitStore.Get(key)`) — `InMemoryRateLimitStore.Get()` already calls `NormalizeExpiredWindows(e.state, time.Now())` before returning. ✅ SAFE.
2. **DB fallback** (post-restart / TTL eviction) — called `callback.DBRowToState(row)` without wrapping with `NormalizeExpiredWindows`. ❌ BUG.

Result: after a server restart or cache eviction, progress bars show stale pre-reset utilization even though `ccResetCountdown` correctly shows "Reset now."

The identical fix was already applied to `native_upstream.go:311` in a prior commit (`9a46a0c`).

## Systematic Audit (Scope B)

All `DBRowToState` / `dbRowToState` call sites:

| Site | Normalize? | Safe? |
|------|-----------|-------|
| `handler_claude_code.go:74` | ❌ missing | **BUG (this fix)** |
| `native_upstream.go:311` | ✅ wrapped | Safe |
| `ratelimit_flusher.go:78` (unexported) | — | Stores to InMemoryRateLimitStore; `Get()` normalizes on read → Safe |
| `ratelimit_flusher.go:230` (unexported) | — | Stores to `OAuthUsageStore`; field comment: "not read by loadClaudeCodeData" → Not a UI bug |

**Conclusion**: only one production bug site.

## Alternatives Considered

| Option | Decision |
|--------|----------|
| Normalize inside `DBRowToState` itself | Rejected — DBRowToState is a pure DB→struct mapping; normalization is time-dependent and belongs at the call site (same pattern as native_upstream.go). |
| Add normalization to `OAuthUsageStore.Get()` | Rejected for this issue — OAuthUsageStore is not in the loadClaudeCodeData read path; separate concern. |
| Fix = wrap at call site | **Selected** — mirrors existing correct pattern in native_upstream.go. |

## User Scenarios

### User Story 1 — Progress bar resets correctly after session window expires (P1)

**Acceptance Scenarios**:

1. **Given** a token whose 5h window expired 1 hour ago, **When** `loadClaudeCodeData` reads from DB fallback, **Then** `SessionUsage` is 0 (not the stale pre-reset value).
2. **Given** a token whose 7d window is still active, **When** `loadClaudeCodeData` reads from DB fallback, **Then** `WeeklyUsage` preserves the stored value.
3. **Given** in-memory store has a live entry, **When** `loadClaudeCodeData` reads it, **Then** behavior is unchanged (in-memory path already normalized).

### Edge Cases

- **Zero reset time** (no header received yet): `ParseResetTime("")` returns zero → `NormalizeExpiredWindows` treats as NOT expired (conservative). Behavior unchanged.
- **Both windows expired**: both utilization fields zeroed.
- **Server restart with valid cached data**: data survives in DB; normalization applies on read.

## Functional Requirements

- **FR-001**: DB fallback path MUST apply `NormalizeExpiredWindows` before using the state for UI rendering.
- **FR-002**: In-memory store path MUST remain unchanged (already normalizes via `Get()`).
- **FR-003**: A regression test MUST exist asserting that an expired 5h window loaded from DB renders as 0 utilization.

## Success Criteria

- **SC-001**: `TestLoadClaudeCodeData_DBFallback_ExpiredWindowZerosUsage` passes.
- **SC-002**: All existing `TestLoadClaudeCodeData_*` tests pass.
- **SC-003**: `go vet ./...` returns no errors.
- **SC-004**: Progress bar shows 0% (not stale %) when countdown label shows "Reset now."

## Out of Scope

- `OAuthUsageStore` normalization (field not in UI read path; separate issue if needed).
- Normalization inside `DBRowToState` / `dbRowToState` (time-coupling concern).
- Any changes to `ratelimit_flusher.go`.
