# Implementation Plan: HO-490 — allowed_warning bypass

**Branch**: `490-allowed-warning-bypass` | **Date**: 2026-04-04 | **Spec**: [spec.md](./spec.md)

## Summary

Replace non-existent `rate_limited`/`overage` consts with canonical Anthropic value `rejected`.
Add `allowed_warning` bypass in `selectUpstreamWithThrottle` so tokens with active extra usage
are not incorrectly skipped due to high utilization. Add Discord alerts for both new statuses.

## Research Findings (verified 2026-04-04)

**Sources**: Anthropic GitHub issues #29300, #29604, #41185

`anthropic-ratelimit-unified-status` confirmed values:

| Value | Meaning | Requests accepted? |
|-------|---------|-------------------|
| `allowed` | Normal quota available | ✅ |
| `allowed_warning` | Quota exhausted; extra usage (overage credit) active | ✅ (deducts from credit) |
| `rejected` | Quota exhausted; no extra usage | ❌ |

`rate_limited` / `overage` are **NOT** official `unified-status` values (removed from codebase).

**Additional headers found** (out of scope this PR):
- `anthropic-ratelimit-unified-fallback` — overage availability signal
- `anthropic-ratelimit-unified-overage-status` — whether extra usage is currently active

**Official docs note**: `platform.claude.com/docs/api/rate-limits` only covers standard API key headers (`x-ratelimit-*`). The `anthropic-ratelimit-unified-*` headers are OAuth/Claude Code plan specific and not documented there.

## Technical Context

**Language/Version**: Go (latest stable — 1.24.x)
**Primary Dependencies**: stdlib only (no new libraries)
**Storage**: N/A (in-memory `RateLimitStore` + existing DB layer, no schema changes)
**Testing**: `go test` + `testify`
**Target Platform**: Linux server (tianjiLLM proxy)
**Project Type**: Single Go project under `internal/`
**Performance Goals**: No hot-path changes; gate check is O(n) over tokens, same as before
**Constraints**: No new allocations in the skip path; goroutine-safe (existing mutex pattern)
**Scale/Scope**: Affects only Anthropic OAuth token upstream selection

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First | ✅ N/A | Feature is Go-only; no Python equivalent exists |
| II. Feature Parity | ✅ N/A | New Go capability, not a port |
| III. Research Before Build | ✅ | `research.md` complete; header values verified from Anthropic GitHub issues |
| IV. Failing-Tests-First | ✅ | `## Failing Tests` section below; tests written before implementation |
| V. Go Best Practices | ✅ | Const addition, guard clause, goroutine via `go a.sendAlertIfNotCooling` |
| VI. No Stale Knowledge | ✅ | Header values verified from live Anthropic GitHub issues |
| VII. sqlc-First | ✅ N/A | No DB queries added |

**Gate**: PASS — no violations.

## Project Structure

```text
specs/490-allowed-warning-bypass/
├── spec.md       ✅
├── research.md   ✅
├── plan.md       ← this file
└── tasks.md      (next phase)

internal/callback/
├── ratelimit_store.go            — const additions
├── discord_ratelimit.go          — alert logic
├── ratelimit_store_test.go       — update for new consts
├── discord_ratelimit_test.go     — update for new consts
├── discord_ratelimit_oauth_test.go — new test

internal/proxy/handler/
├── native_upstream.go            — bypass + skip logic
└── native_upstream_test.go       — new + updated tests
```

## Failing Tests

*All tests below must be written first and confirmed to fail before implementation.*

### US-1: allowed_warning token bypasses utilization gate

| Test Function | File | Assertion | Covers |
|---|---|---|---|
| `TestSelectUpstreamThrottle_AllowedWarning_BypassesUtilizationGate` | `internal/proxy/handler/native_upstream_test.go` | Token with `UnifiedStatus=allowed_warning`, `5h_util=0.92` is in `available` (selected ≥1 time in 20 trials) | AC-1 |
| `TestSelectUpstreamThrottle_Allowed_HighUtil_StillThrottled_NoRegression` | `internal/proxy/handler/native_upstream_test.go` | Token with `UnifiedStatus=allowed`, `5h_util=0.92` → only other token selected | AC-3 |

### US-2: rejected token is skipped

| Test Function | File | Assertion | Covers |
|---|---|---|---|
| `TestSelectUpstreamThrottle_SkipsRejectedStatus` | `internal/proxy/handler/native_upstream_test.go` | Token with `UnifiedStatus=rejected` → only other token selected | AC-2 |
| `TestSelectUpstreamThrottle_RejectedStatusIgnoredAfterReset` | `internal/proxy/handler/native_upstream_test.go` | Token with `UnifiedStatus=rejected` + `UnifiedReset` in the past → token included (PruneExpired cleared status) | Edge: stale state |
| `TestSelectUpstreamThrottle_Rejected_StillSkipped_NoRegression` | `internal/proxy/handler/native_upstream_test.go` | Same as SkipsRejectedStatus — regression guard | AC-2 regression |

### US-3: Discord alerts

| Test Function | File | Assertion | Covers |
|---|---|---|---|
| `TestCheckAndAlertOAuth_StatusRejected_TriggersAlert` | `internal/callback/discord_ratelimit_oauth_test.go` | `rejected` status → exactly 1 alert fires | AC-4 |
| `TestCheckAndAlertOAuth_AllowedWarning_TriggersAlert` | `internal/callback/discord_ratelimit_oauth_test.go` | `allowed_warning` status → ≥1 alert fires, message contains "allowed_warning" | AC-5 |
| `TestCheckAndAlert_OAuth_Rejected_AlertFiresAndReturns` | `internal/callback/discord_ratelimit_test.go` | Updated: `rejected` → 1 alert, message contains "rejected" | AC-4 |

### Verification command

```bash
go test ./internal/proxy/handler/... ./internal/callback/... \
  -run "TestSelectUpstreamThrottle_AllowedWarning|TestSelectUpstreamThrottle_SkipsRejected|TestSelectUpstreamThrottle_Rejected|TestSelectUpstreamThrottle_Allowed_HighUtil|TestCheckAndAlertOAuth_Status|TestCheckAndAlertOAuth_AllowedWarning|TestCheckAndAlert_OAuth_RateLimited" \
  -v -count=1
```

## Implementation Steps

### Step 1 — Write failing tests (T001)
Write all test functions from `## Failing Tests` above.
Run verification command → confirm tests compile but fail.

### Step 2 — Add/replace consts (T002)

`internal/callback/ratelimit_store.go`:
```go
const (
    UnifiedStatusAllowed        = "allowed"
    UnifiedStatusAllowedWarning = "allowed_warning"
    UnifiedStatusRejected       = "rejected"
    // rate_limited and overage removed — not official Anthropic values
)
```

### Step 3 — Update skip + bypass logic (T003)

`internal/proxy/handler/native_upstream.go` — `selectUpstreamWithThrottle`:
```go
if state.UnifiedStatus == callback.UnifiedStatusRejected {
    trackNearestReset(...)
    continue
}
if state.UnifiedStatus == callback.UnifiedStatusAllowedWarning {
    available = append(available, u)
    continue  // bypass 5h/7d gate
}
// existing 5h/7d gate below unchanged
```

### Step 4 — Discord alerts (T004)

`internal/callback/discord_ratelimit.go` — `CheckAndAlert`:
```go
if state.UnifiedStatus == UnifiedStatusRejected {
    go a.sendAlertIfNotCooling(...)
    return  // early return
}
if state.UnifiedStatus == UnifiedStatusAllowedWarning {
    go a.sendAlertIfNotCooling(...)
    // NO return — continue to utilization checks
}
```

### Step 5 — Update existing tests (T005)

Replace all references to `UnifiedStatusRateLimited` / `UnifiedStatusOverage`
in `ratelimit_store_test.go`, `discord_ratelimit_test.go`, `ratelimit_flusher_test.go`,
`native_upstream_test.go`.

### Step 6 — Full test run (T006)

```bash
go build ./...
go test ./internal/...
go vet ./...
```

### Step 7 — Commit + push + open PR (T007)
