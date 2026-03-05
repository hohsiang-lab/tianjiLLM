# Implementation Plan: OAuth Token 智能限流

**Branch**: `079-oauth-token-throttle` | **Date**: 2026-03-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/079-oauth-token-throttle/spec.md`

## Summary

当多把 Anthropic OAuth token 配置为 upstream 时，在 `selectUpstream` 阶段根据 `RateLimitStore` 中的 5h/7d 利用率数据过滤掉超阈值 token（默认 80%），从剩余健康 token 中 round-robin 选择。所有 token 耗尽时返回 429 + Retry-After。恢复被 revert 的 `CheckAndAlertOAuth` Discord 告警方法。

## Technical Context

**Language/Version**: Go 1.24.4
**Primary Dependencies**: chi/v5 (router), testify (assertions)
**Storage**: N/A（内存 `InMemoryRateLimitStore`，已存在）
**Testing**: `go test` + `testify`
**Target Platform**: Linux server (Docker/K8s)
**Project Type**: Single Go project
**Performance Goals**: 100ms 内返回 429（SC-003），无额外上游延迟
**Constraints**: 限流检查在内存中完成，不引入 DB/Redis 依赖
**Scale/Scope**: 1-10 OAuth tokens 并发使用

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | 此功能为 tianjiLLM 原创，Python 版无对应实现 |
| II. Feature Parity | N/A | 同上 |
| III. Research Before Build | PASS | 无新外部依赖，所有代码基于现有 internal 包 |
| IV. Failing-Tests-First | PASS | Failing Tests section 已设计（见下） |
| V. Go Best Practices | PASS | 方法接收者、error 返回、接口复用 |
| VI. No Stale Knowledge | PASS | 无新 library；现有代码已读取确认 |
| VII. sqlc-First DB Access | N/A | 无数据库操作 |

## Project Structure

### Documentation (this feature)

```text
specs/079-oauth-token-throttle/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
internal/
├── proxy/handler/
│   ├── native_upstream.go       # MODIFY: rename selectUpstream → roundRobinSelect, add selectUpstreamWithThrottle
│   ├── native_upstream_test.go  # NEW: throttle selection tests
│   └── native_format.go         # MODIFY: call site + 429 handling + Discord alert wiring
├── callback/
│   ├── discord_ratelimit.go     # MODIFY: restore CheckAndAlertOAuth + sendOAuthAlertIfNotCooling
│   └── discord_ratelimit_test.go # MODIFY: add OAuth alert test cases
└── provider/anthropic/
    └── oauth.go                 # READ ONLY: IsOAuthToken()
```

**Structure Decision**: 完全在现有目录结构内修改，无新 package。

## Failing Tests

### User Story 1 Tests — 自动跳过高利用率 Token

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestSelectUpstreamThrottle_Skips5hOverThreshold` | `internal/proxy/handler/native_upstream_test.go` | 3 tokens, token A 5h=0.85: selected token is NOT A | AS-1.1 |
| `TestSelectUpstreamThrottle_Skips7dOverThreshold` | `internal/proxy/handler/native_upstream_test.go` | Token B 7d=0.90: selected token is NOT B | AS-1.2 |
| `TestSelectUpstreamThrottle_RecoversBelowThreshold` | `internal/proxy/handler/native_upstream_test.go` | Token A 5h=0.60 (below threshold): A is selectable | AS-1.3 |
| `TestSelectUpstreamThrottle_SkipsRateLimitedStatus` | `internal/proxy/handler/native_upstream_test.go` | Token A UnifiedStatus="rate_limited": A is skipped regardless of utilization | AS-1.4 |
| `TestSelectUpstreamThrottle_UnknownStateIsAvailable` | `internal/proxy/handler/native_upstream_test.go` | Token not in store: treated as available | Edge: 首次请求 |
| `TestSelectUpstreamThrottle_SentinelNeg1IsAvailable` | `internal/proxy/handler/native_upstream_test.go` | Token with utilization=-1: treated as available | Edge: 数据缺失 |
| `TestSelectUpstreamThrottle_NonOAuthNotThrottled` | `internal/proxy/handler/native_upstream_test.go` | Non-OAuth API key: never throttled | Edge: 混合配置 |
| `TestSelectUpstreamThrottle_DeduplicatesByAPIKey` | `internal/proxy/handler/native_upstream_test.go` | 3 upstreams with same API key: deduplicated to 1 | Edge: 重复 key |

### User Story 2 Tests — 所有 Token 耗尽返回 429

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestSelectUpstreamThrottle_AllThrottled_ReturnsError` | `internal/proxy/handler/native_upstream_test.go` | All tokens over threshold: returns `allTokensThrottledError` | AS-2.1 |
| `TestSelectUpstreamThrottle_AllThrottled_NearestReset` | `internal/proxy/handler/native_upstream_test.go` | Error contains nearest reset time (Token A reset in 30min) | AS-2.2 |
| `TestSelectUpstreamThrottle_SingleTokenThrottled_Returns429` | `internal/proxy/handler/native_upstream_test.go` | Only 1 OAuth token configured, over threshold: returns error | Edge: 单 token |
| `TestSelectUpstreamThrottle_ConfigurableThreshold` | `internal/proxy/handler/native_upstream_test.go` | Threshold=0.5, token at 0.6: throttled; at 0.4: available | FR-011 |

### User Story 3 Tests — Discord 告警

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCheckAndAlertOAuth_5hOverThreshold` | `internal/callback/discord_ratelimit_test.go` | 5h utilization >= threshold: webhook called with correct message | AS-3.1 |
| `TestCheckAndAlertOAuth_Cooldown` | `internal/callback/discord_ratelimit_test.go` | Same token alert within 1h: webhook NOT called again | AS-3.2 |
| `TestCheckAndAlertOAuth_RateLimitedStatus` | `internal/callback/discord_ratelimit_test.go` | UnifiedStatus="rate_limited": webhook called | AS-3.3 |
| `TestCheckAndAlertOAuth_NilAlerter` | `internal/callback/discord_ratelimit_test.go` | Nil alerter (no webhook URL): no panic, no error | AS-3.4 |
| `TestCheckAndAlertOAuth_7dOverThreshold` | `internal/callback/discord_ratelimit_test.go` | 7d utilization >= threshold: webhook called | FR-005 supplement |

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle" -v
go test ./internal/callback/... -run "TestCheckAndAlertOAuth" -v
```

## Implementation Details

### Step 1: Rename `selectUpstream` → `roundRobinSelect`

**File**: `internal/proxy/handler/native_upstream.go`

Rename the existing package-level function from `selectUpstream` to `roundRobinSelect`. Same signature, same logic. This is purely a rename to make room for the new method.

### Step 2: Add `selectUpstreamWithThrottle` method

**File**: `internal/proxy/handler/native_upstream.go`

New method on `*Handlers`:

```go
func (h *Handlers) selectUpstreamWithThrottle(
    providerName string, upstreams []nativeUpstream,
) (nativeUpstream, error)
```

**Logic**:
1. Non-anthropic OR `h.RateLimitStore == nil` → delegate to `roundRobinSelect`
2. Get threshold from `h.Config.RatelimitAlertThreshold` (default 0.8 if 0)
3. Deduplicate upstreams by APIKey (`seen` map)
4. For each unique upstream:
   - `!anthropic.IsOAuthToken(apiKey)` → always available
   - Not in store → available (FR-009)
   - `UnifiedStatus == "rate_limited"` → skip, track reset
   - `Unified5hUtilization >= 0 && >= threshold` → skip, track reset
   - `Unified7dUtilization >= 0 && >= threshold` → skip, track reset
   - `-1` utilization → available (FR-010)
5. Empty available list → return `allTokensThrottledError{resetAt: nearestReset}`
6. Otherwise → `roundRobinSelect(providerName, available)`

**New types**:
- `allTokensThrottledError` struct: `resetAt time.Time`, implements `error`
- `parseUnixResetTime(s string) time.Time` helper

### Step 3: Update `nativeProxy` call site

**File**: `internal/proxy/handler/native_format.go` (lines 28-35)

Change line 29 from:
```go
upstream := selectUpstream(providerName, upstreams)
```
To:
```go
upstream, err := h.selectUpstreamWithThrottle(providerName, upstreams)
if err != nil {
    if ate, ok := err.(*allTokensThrottledError); ok {
        retryAfter := int(time.Until(ate.resetAt).Seconds())
        if retryAfter < 1 { retryAfter = 60 }
        w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
        writeJSON(w, http.StatusTooManyRequests, model.ErrorResponse{
            Error: model.ErrorDetail{
                Message: "all OAuth tokens throttled",
                Type:    "rate_limit_error",
            },
        })
        return
    }
}
```

### Step 4: Restore `CheckAndAlertOAuth`

**File**: `internal/callback/discord_ratelimit.go`

Restore from reverted commit `eec5ca2`:
- `CheckAndAlertOAuth(state AnthropicOAuthRateLimitState)` method
- `sendOAuthAlertIfNotCooling(key, reason string, state AnthropicOAuthRateLimitState)` method

Add 7d utilization check (not in original reverted code):
```go
if state.Unified7dUtilization >= a.threshold {
    key := fmt.Sprintf("ratelimit:oauth:7d_util:%s", state.TokenKey)
    go a.sendOAuthAlertIfNotCooling(key, "⚠️ 7d utilization ...", state)
}
```

### Step 5: Wire Discord alert in ModifyResponse

**File**: `internal/proxy/handler/native_format.go`

After line 134 (where `h.RateLimitStore.Set` is called for 200 responses), add:
```go
if h.DiscordAlerter != nil && anthropic.IsOAuthToken(apiKey) {
    h.DiscordAlerter.CheckAndAlertOAuth(rlState)
}
```

Same for the non-200 path (after line 121):
```go
if h.DiscordAlerter != nil && anthropic.IsOAuthToken(apiKey) {
    h.DiscordAlerter.CheckAndAlertOAuth(rlState)
}
```

## Reused Existing Code

| Function/Type | File | Usage |
|---------------|------|-------|
| `callback.RateLimitStore` interface | `internal/callback/ratelimit_store.go:108` | Query token utilization |
| `callback.RateLimitCacheKey()` | `internal/callback/ratelimit_store.go:45` | Hash API key → store key |
| `callback.AnthropicOAuthRateLimitState` | `internal/callback/ratelimit_store.go:16` | Token state struct |
| `anthropic.IsOAuthToken()` | `internal/provider/anthropic/oauth.go` | Detect OAuth tokens |
| `DiscordRateLimitAlerter` struct | `internal/callback/discord_ratelimit.go:73` | Alert + cooldown |
| `roundRobinCounters` (atomic) | `internal/proxy/handler/native_upstream.go:17` | Round-robin within available pool |
| `h.Config.RatelimitAlertThreshold` | `internal/config/config.go` | Configurable threshold |
| `model.ErrorResponse` | `internal/model/errors.go` | 429 response format |

## Complexity Tracking

No violations. All changes use existing interfaces and patterns.
