# Spec: HO-490 — Support extra usage (allowed_warning) bypass in upstream token throttle

## Overview

When an Anthropic OAuth token enters `allowed_warning` status, it means the token's
normal quota (5h/7d) is exhausted but Anthropic's extra usage (overage/credit) is active
and the token can still serve requests. Currently, Tianji's upstream selector throttles
such tokens due to high utilization values, causing them to be incorrectly skipped even
though Anthropic would accept requests.

## Research Summary

Confirmed from Anthropic GitHub issues (#29604, #29300, #41185) — `anthropic-ratelimit-unified-status`
header values (as of 2026-04):

| Value | Meaning |
|---|---|
| `allowed` | Normal quota available |
| `allowed_warning` | Quota exhausted but extra usage (overage credit) active; requests still accepted |
| `rejected` | Request rejected; quota exhausted and no extra usage |

Previous constants `rate_limited` / `overage` are NOT official Anthropic header values
and have been removed.

## Goals

1. Tokens with `UnifiedStatus == "allowed_warning"` bypass utilization gates in
   `selectUpstreamWithThrottle` and remain in the available pool.
2. Replace non-existent `rate_limited`/`overage` consts with canonical `rejected`.
3. Discord alert fires a non-blocking warning when a token enters `allowed_warning`.
4. `rejected` tokens are still skipped (quota exhausted, no extra usage).
5. `allowed` tokens with high utilization still get throttled (no regression).

## Non-Goals

- Parsing extra usage headers / filling `ExtraUsage*` fields in `OAuthUsageState` (separate task).
- Modifying `lowestUtilizationSelect` — that function operates post-filter, bypass is in pre-filter.

## Affected Files

| File | Change |
|------|--------|
| `internal/callback/ratelimit_store.go` | Replace old consts; add `UnifiedStatusRejected`, `UnifiedStatusAllowedWarning` |
| `internal/proxy/handler/native_upstream.go` | Skip on `rejected`; bypass gate for `allowed_warning` |
| `internal/callback/discord_ratelimit.go` | Alert for `rejected`; non-blocking alert for `allowed_warning` |
| `internal/proxy/handler/native_upstream_test.go` | New/updated test cases |
| `internal/callback/discord_ratelimit_test.go` | Update for new consts |
| `internal/callback/discord_ratelimit_oauth_test.go` | New test case |
| `internal/callback/ratelimit_store_test.go` | Update for new consts |
| `internal/callback/ratelimit_flusher_test.go` | Update for new consts |

## Behaviour Specification

### Constants (ratelimit_store.go)

```go
const (
    UnifiedStatusAllowed        = "allowed"
    UnifiedStatusAllowedWarning = "allowed_warning" // quota exhausted, extra usage active
    UnifiedStatusRejected       = "rejected"        // quota exhausted, no extra usage
)
```

### `selectUpstreamWithThrottle` (native_upstream.go)

```
for each token:
  if status == rejected → skip (track reset)
  if status == allowed_warning → add to available; continue  ← bypass gate
  if 5h util >= threshold → skip
  if 7d util >= gate7d   → skip
  available << token
```

### `CheckAndAlert` (discord_ratelimit.go)

- `rejected` → 🚨 alert + early return
- `allowed_warning` → ⚠️ alert (non-blocking, continues to util checks)

## Acceptance Criteria

- [ ] Token status=`allowed_warning`, 5h_util=0.92 → included in `available`
- [ ] Token status=`rejected` → skipped
- [ ] Token status=`allowed`, 5h_util=0.92 → still throttled (no regression)
- [ ] Discord alert fires for `rejected`
- [ ] Discord alert fires for `allowed_warning` (non-blocking)
- [ ] `go test ./internal/...` passes
- [ ] `go vet ./...` passes
