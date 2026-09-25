# Implementation Plan: Enforce Per-Key Rate Limits & Budget Controls

**Branch**: `204-enforce-key-limits` | **Date**: 2026-04-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/204-enforce-key-limits/spec.md`

## Summary

修復 per-key 限制功能。UI 上可設定 RPM/TPM/Budget/Models/Parallel/Expiry，但後端 auth middleware 從未將這些欄位注入 request context，且 budget middleware 未掛載到 chain。修復方式：擴展 TokenInfo 結構、auth middleware 注入 limit 值到 context（含 key→team→org 繼承鏈）、auth middleware 做 body buffering + model extraction + models 檢查（參考 LiteLLM `get_model_from_request` + `can_key_call_model` 模式）、掛載 budget middleware、加入 expiry 檢查、post-call 異步更新 Redis spend cache + TPM counter。

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: chi/v5 (router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), testify (testing)
**Storage**: PostgreSQL (key/team/org data), Redis (rate counters, spend cache)
**Testing**: `go test` + `testify`, `httptest.NewServer` / `httptest.NewRecorder`
**Target Platform**: Linux server (Kubernetes)
**Project Type**: web (single Go binary)
**Performance Goals**: Budget/rate limit check 增加 < 1ms p99 延遲（Redis round-trip）
**Constraints**: fail-open on Redis failure, 零回歸（null limits = 不限制）
**Scale/Scope**: 修改 ~8 個 source files + 5 個 test files，新增 ~350 行 code，新增 ~600 行 tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | 這是 bug fix，非 Python 移植功能 |
| II. Feature Parity | PASS | 修復後將與 LiteLLM 的 per-key enforce 行為對齊 |
| III. Research Before Build | PASS | 已完成業界調研（OpenAI、LiteLLM TPM/Budget 策略） |
| IV. Failing-Tests-First | PASS | Failing Tests section 已列出所有 test functions |
| V. Go Best Practices | PASS | 使用 context.WithValue、interface injection、error wrapping |
| VI. No Stale Knowledge | PASS | 已透過 Context7 + GitHub 驗證 LiteLLM 架構 |
| VII. sqlc-First DB Access | PASS | 不需新增 DB query — 現有 GetVerificationToken/GetTeam/GetOrganization 已足夠 |

## Project Structure

### Documentation (this feature)

```text
specs/204-enforce-key-limits/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A — no new API endpoints)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/proxy/middleware/
├── auth.go              # MODIFY: body buffering + model extraction + models check +
│                        #          expiry check + context injection（limits）
├── db_validator.go      # MODIFY: TokenInfo 加入 limit 欄位 + 繼承鏈解析
├── helpers.go           # MODIFY: 新增 context keys（maxBudgetKey, spendKey）
├── budget.go            # MODIFY: 改讀 context（不再獨立查 DB）+ Redis spend cache check
├── parallel.go          # EXISTING: 已有 enforce 邏輯，context key 已定義（不需改）
├── dynamic_ratelimit.go # EXISTING: 已有 enforce 邏輯，context key 已定義（不需改）
└── auth_test.go         # NEW: auth limit injection + models + expiry tests

internal/proxy/
└── server.go            # MODIFY: 掛載 budget middleware 到 chain +
                         #          補齊 Gemini/Anthropic Batches route 的 middleware chain

internal/router/
├── router.go            # MODIFY: retry loop 加入 exponential backoff + Retry-After
├── fallback.go          # MODIFY: fallback chain 加入短暫延遲
└── router_test.go       # MODIFY: 新增 backoff tests

internal/spend/
└── tracker.go           # MODIFY: post-call INCRBYFLOAT Redis spend cache + TPM counter

test/contract/
└── key_limits_test.go   # NEW: 全流程 contract tests
```

**Structure Decision**: 修改現有檔案為主，不引入新 package。測試放在 test/contract/ 和各 package 的 _test.go。

### Body Buffering 策略（Models Check）

Auth middleware 需要在 handler 之前讀取 request body 提取 model name。參考 LiteLLM 的
`get_model_from_request(request_data, route)` 模式：

1. `io.ReadAll(r.Body)` 讀取 body
2. `r.Body = io.NopCloser(bytes.NewReader(bodyBytes))` 重置 body 供 handler 使用
3. 只解析 `{"model": "xxx"}` 一個欄位（partial JSON decode）
4. 也從 URL path 提取 model（如 `/openai/deployments/gpt-4` → `gpt-4`）
5. 記憶體消耗可忽略 — LLM 請求 body 通常 < 1MB

這與 LiteLLM 的 `user_api_key_auth.py` → `get_model_from_request()` → `can_key_call_model()` 完全對齊。

## Failing Tests

### User Story 1 Tests (RPM/TPM)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAuthMiddleware_InjectsRPMLimit` | `internal/proxy/middleware/auth_test.go` | context 包含 rpmLimitKey=10 when key has rpm_limit=10 | FR-001 |
| `TestAuthMiddleware_InjectsTPMLimit` | `internal/proxy/middleware/auth_test.go` | context 包含 tpmLimitKey=200000 when key has tpm_limit=200000 | FR-001 |
| `TestAuthMiddleware_FallbackToTeamRPM` | `internal/proxy/middleware/auth_test.go` | key rpm_limit=nil, team rpm_limit=20 → context has 20 | FR-002, AS-1.5 |
| `TestAuthMiddleware_FallbackToOrgTPM` | `internal/proxy/middleware/auth_test.go` | key=nil, team=nil, org tpm_limit=500000 → context has 500000 | FR-002 |
| `TestAuthMiddleware_NullLimitsNoInjection` | `internal/proxy/middleware/auth_test.go` | all null → rpmLimitKey=0 (no limit), passes through | FR-010, AS-1.3 |
| `TestDynamicRateLimit_RPMExceeded` | `internal/proxy/middleware/dynamic_ratelimit_test.go` | 11th request returns 429 when RPM=10 | AS-1.1 |
| `TestDynamicRateLimit_TPMExceeded` | `internal/proxy/middleware/dynamic_ratelimit_test.go` | request rejected when TPM counter >= limit | AS-1.2 |
| `TestDynamicRateLimit_WindowReset` | `internal/proxy/middleware/dynamic_ratelimit_test.go` | after 60s TTL, new request allowed | AS-1.4 |
| `TestDynamicRateLimit_SetsRateLimitHeaders` | `internal/proxy/middleware/dynamic_ratelimit_test.go` | response has X-RateLimit-Limit-Requests, X-RateLimit-Remaining-Requests headers | FR-011 |

### User Story 2 Tests (Budget)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestBudgetMiddleware_SpendBelowBudget` | `internal/proxy/middleware/budget_test.go` | spend=$99.50, max_budget=$100 → 200 OK | AS-2.1 |
| `TestBudgetMiddleware_SpendExceedsBudget` | `internal/proxy/middleware/budget_test.go` | spend=$100.01, max_budget=$100 → 429 | AS-2.2 |
| `TestBudgetMiddleware_NullBudgetPassThrough` | `internal/proxy/middleware/budget_test.go` | max_budget=nil → 200 OK | AS-2.3 |
| `TestBudgetMiddleware_FallbackToTeamBudget` | `internal/proxy/middleware/budget_test.go` | key budget=nil, team budget=$500, team spend=$501 → 429 | AS-2.4 |
| `TestBudgetMiddleware_ReadsFromContext` | `internal/proxy/middleware/budget_test.go` | middleware reads spend/max_budget from context, not DB | FR-006 |
| `TestSpendTracker_UpdatesRedisSpendCache` | `internal/spend/tracker_test.go` | Record() increments Redis key `tianji:spend:{keyHash}` by cost | FR-006 |

### User Story 3 Tests (Models)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAuthMiddleware_ModelNotAllowed` | `internal/proxy/middleware/auth_test.go` | models=["qwen3-30b"], request body model="claude-opus-4-5" → 403 | AS-3.1 |
| `TestAuthMiddleware_ModelAllowed` | `internal/proxy/middleware/auth_test.go` | models=["qwen3-30b"], request body model="qwen3-30b" → pass | AS-3.2 |
| `TestAuthMiddleware_EmptyModelsAllowAll` | `internal/proxy/middleware/auth_test.go` | models=[] → any model allowed | AS-3.3 |
| `TestAuthMiddleware_ModelWildcardMatch` | `internal/proxy/middleware/auth_test.go` | models=["qwen3-*"], request body "qwen3-30b" → pass | AS-3.4 |
| `TestAuthMiddleware_BodyBufferingPreservesBody` | `internal/proxy/middleware/auth_test.go` | auth reads body for model, handler can still read full body | Body buffering |
| `TestAuthMiddleware_ModelFromURLPath` | `internal/proxy/middleware/auth_test.go` | /openai/deployments/gpt-4 → extracts model "gpt-4" from path | Body buffering fallback |

### User Story 4 Tests (Parallel)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAuthMiddleware_InjectsMaxParallel` | `internal/proxy/middleware/auth_test.go` | context has maxParallelLimitKey when budget has value | FR-009 |
| `TestParallel_ExceedsLimit` | `internal/proxy/middleware/parallel_test.go` | 4th concurrent request returns 429 when limit=3 | AS-4.1 |
| `TestParallel_NullLimitPassThrough` | `internal/proxy/middleware/parallel_test.go` | no limit → all pass | AS-4.3 |

### User Story 5 Tests (Expiry)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAuthMiddleware_ExpiredKey` | `internal/proxy/middleware/auth_test.go` | expires=past → 403 | AS-5.1 |
| `TestAuthMiddleware_ValidExpiry` | `internal/proxy/middleware/auth_test.go` | expires=future → pass | AS-5.2 |
| `TestAuthMiddleware_NullExpiryNeverExpires` | `internal/proxy/middleware/auth_test.go` | expires=null → pass | AS-5.3 |

### User Story 6 Tests (Upstream 429 Backoff + Middleware Gap)

**注意**：經日誌分析確認，127 次 429 是 client 無 backoff 重試 + 天機所有 upstream token 被 5h_gate 跳過，天機直接回 429。不是天機 retry 造成的。但 Router retry loop 仍需 backoff（防止非 passthrough 路徑的 thundering herd）。

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestRouter_RetryWithExponentialBackoff` | `internal/router/router_test.go` | 第 1 次重試延遲 ~1s，第 2 次 ~2s，第 3 次 ~4s | AS-6.1, FR-013 |
| `TestRouter_RespectsRetryAfterHeader` | `internal/router/router_test.go` | upstream 429 + Retry-After: 5 → 至少等 5 秒 | AS-6.2, FR-014 |
| `TestRouter_RetryAfterCapAt300s` | `internal/router/router_test.go` | Retry-After: 3600 → cap 到 300s，直接拒絕不重試 | Edge: 異常大 Retry-After |
| `TestRouter_AllDeploymentsCooldown_ReturnsRetryAfter` | `internal/router/router_test.go` | 所有 deployment cooldown → 429 + Retry-After header | AS-6.3, FR-017 |
| `TestRouter_UsesRetryPolicySettings` | `internal/router/router_test.go` | RetryPolicy.RetryAfterSeconds 被使用作為 backoff 基準 | AS-6.5, FR-015 |
| `TestFallback_DelayBetweenAttempts` | `internal/router/fallback_test.go` | fallback 間有 > 0 延遲 | FR-016 |
| `TestRouter_BackoffWithJitter` | `internal/router/router_test.go` | 多次重試的延遲有隨機 jitter（不完全相同） | FR-013 jitter |
| `TestRouter_MaxBackoffCap` | `internal/router/router_test.go` | 連續 10+ 次失敗，backoff 不超過 30s | Edge: backoff 溢出 |
| `TestRouter_ExhaustsRetriesReturns429` | `internal/router/router_test.go` | all retries exhausted → 429 to client, no infinite loop | AS-6.4 |
| `TestGeminiRoute_HasFullMiddlewareChain` | `internal/proxy/server_test.go` | /v1beta/models/* route 有 Parallel + DynamicRate + CacheControl middleware | FR-018 |
| `TestAnthropicBatchRoute_HasFullMiddlewareChain` | `internal/proxy/server_test.go` | /anthropic/v1/* route 有完整 middleware chain | FR-018 |

### Edge Case Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAuthMiddleware_ZeroRPMBlocksAll` | `internal/proxy/middleware/auth_test.go` | rpm_limit=0 → all blocked | Edge: 0 vs null |
| `TestBudgetMiddleware_SpendEqualsMax` | `internal/proxy/middleware/budget_test.go` | spend == max_budget → 429 | Edge: >= check |
| `TestDynamicRateLimit_RedisUnavailableFailOpen` | `internal/proxy/middleware/dynamic_ratelimit_test.go` | Redis down → request allowed | FR-012 |
| `TestBudgetMiddleware_RedisUnavailableFailOpen` | `internal/proxy/middleware/budget_test.go` | Redis down → request allowed | FR-012 |
| `TestMiddlewareChain_MultipleLimitsTrigger` | `internal/proxy/middleware/budget_test.go` | RPM + budget both exceeded → first in chain returns 429 | Edge: multi-limit |
| `TestParallel_DecrementOnStreamEnd` | `internal/proxy/middleware/parallel_test.go` | streaming request: counter +1 on start, -1 on Close | Edge: streaming parallel |

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./internal/proxy/middleware/... ./internal/proxy/... ./internal/router/... -run "TestAuthMiddleware_Injects|TestAuthMiddleware_Fallback|TestAuthMiddleware_Null|TestAuthMiddleware_Model|TestAuthMiddleware_Expired|TestAuthMiddleware_Zero|TestAuthMiddleware_Body|TestBudgetMiddleware_|TestDynamicRateLimit_RPM|TestDynamicRateLimit_TPM|TestDynamicRateLimit_Window|TestDynamicRateLimit_Redis|TestParallel_|TestRouter_Retry|TestRouter_Backoff|TestRouter_AllDeployments|TestRouter_Uses|TestRouter_Max|TestFallback_Delay|TestGeminiRoute_|TestAnthropicBatchRoute_" -v
```

## Complexity Tracking

No violations. All changes stay within existing packages and patterns. No new abstractions or projects introduced.
