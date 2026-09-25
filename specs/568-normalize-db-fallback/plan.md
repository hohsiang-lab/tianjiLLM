# Implementation Plan: HO-568 Normalize Expired Windows in DB Fallback

**Branch**: `HO-568-progress-bar-reset`  
**Scope**: B) 系統性 — full audit of all DBRowToState call sites

## Technical Design

Single call-site fix in `internal/ui/handler_claude_code.go:74`:

```go
// Before
rlState = callback.DBRowToState(row)

// After
rlState = callback.NormalizeExpiredWindows(callback.DBRowToState(row), time.Now())
```

Rationale: mirrors the correct pattern already used in `native_upstream.go:311`.

## Audit Results

Full systematic review of all `DBRowToState`/`dbRowToState` call sites confirms only one bug site (see spec.md Systematic Audit table).

## Test Strategy

Unit test in `internal/ui/handler_claude_code_test.go`:
- `TestLoadClaudeCodeData_DBFallback_ExpiredWindowZerosUsage` — expired 5h window → 0; active 7d window → preserved.
- No E2E required: the normalization logic is already covered by `TestNormalizeExpiredWindows_*` in `ratelimit_store_test.go`.

## Risk Assessment

- **Low risk**: 1-line change, wrapping an existing function with `NormalizeExpiredWindows` that is already called in 3 other read paths.
- **No migration**: DB schema unchanged.
- **No config change**: behavior change is purely in-process normalization.
