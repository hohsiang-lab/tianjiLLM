# Implementation Plan: Move ccDBReader from UIHandler struct to function-level injection

**Spec**: `spec.md`
**Created**: 2026-04-12

## Architecture

Pure refactoring — no new packages, no new files, no behavior change.

### Change summary

1. Add private helper `func (h *UIHandler) dbReader() rateLimitStateReader` — returns `callback.NewRateLimitDB(h.DB)` when `h.DB != nil`, else `nil`.
2. Change `loadClaudeCodeData` signature to accept `rateLimitStateReader` parameter.
3. Update 5 production callers to pass `h.dbReader()`.
4. Remove `ccDBReader` field from `UIHandler` struct.
5. Update 6 test cases to pass stub via function parameter.

### Files touched

| File | Change |
|------|--------|
| `internal/ui/handler.go` | Remove `ccDBReader` field (line 33) |
| `internal/ui/handler_claude_code.go` | Add `dbReader()` helper; change `loadClaudeCodeData` signature; update 3 callers |
| `internal/ui/handler_usage.go` | Update 2 callers |
| `internal/ui/handler_claude_code_test.go` | Update 6 tests to pass stub as parameter; remove `h.ccDBReader = ...` assignments |

### Design decisions

- **Why `dbReader()` helper?** The nil-check on `h.DB` is repeated at every call site without it. The helper centralizes this logic and keeps production callers clean: `h.loadClaudeCodeData(r, h.dbReader())`.
- **Why parameter, not option?** Single-purpose injection for a single interface — function parameter is the simplest Go idiom. Options pattern is overkill.

### Risk assessment

- **Risk**: Zero — pure refactoring, all existing tests verify behavior.
- **Rollback**: Revert single commit.

## Dependencies

None. No new packages, no schema changes, no CI config changes.
