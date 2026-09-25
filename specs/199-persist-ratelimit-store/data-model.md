# Data Model: Persist RateLimitStore to Database

**Feature**: 199-persist-ratelimit-store
**Date**: 2026-03-26

## New Table: OAuthTokenRateLimitState

| Column | Type | Description |
|--------|------|-------------|
| token_key | TEXT PRIMARY KEY | SHA256[:6] hex of OAuth token（與 OAuthTokenMetadata.token_key 相同格式） |
| unified_status | TEXT | "allowed", "rate_limited", "overage" |
| unified_5h_status | TEXT | 5h window status |
| unified_5h_utilization | DOUBLE PRECISION | [0,1] or -1 (missing) |
| unified_5h_reset | TEXT | Unix timestamp string |
| unified_7d_status | TEXT | 7d window status |
| unified_7d_utilization | DOUBLE PRECISION | [0,1] or -1 (missing) |
| unified_7d_reset | TEXT | Unix timestamp string |
| unified_7d_sonnet_status | TEXT | 7d sonnet window status |
| unified_7d_sonnet_utilization | DOUBLE PRECISION | [0,1] or -1 (missing) |
| unified_7d_sonnet_reset | TEXT | Unix timestamp string |
| org_id | TEXT | Organization ID |
| updated_at | TIMESTAMPTZ NOT NULL DEFAULT now() | Last flush time |

**設計決策**:
- 只持久化 `lowestUtilizationSelect` 和 `selectUpstreamWithThrottle` 需要的欄位（unified 系列 + org_id）
- 不持久化 legacy 欄位（RequestsLimit, TokensLimit 等）— 這些只用於非 OAuth token，不在 native handler 路徑上
- 不持久化 RepresentativeClaim, FallbackPercentage, OverageDisabledReason — 這些只用於 UI 顯示，不影響路由決策
- `updated_at` 用於啟動時的 TTL 過濾（只載入 30 分鐘內的記錄）

## Modified Entity: InMemoryRateLimitStore

新增 dirty tracking：

| Field | Type | Description |
|-------|------|-------------|
| dirty | map[string]bool | 自上次 flush 以來有變更的 token keys |
| dirtyMu | sync.Mutex | 保護 dirty map 的 mutex（獨立於 entries 的 RWMutex） |

## New Component: RateLimitFlusher

| Field | Type | Description |
|-------|------|-------------|
| store | *InMemoryRateLimitStore | 讀取 dirty keys 和 state |
| db | db.Store | 寫入 DB |
| interval | time.Duration | Flush 間隔（30 秒） |
| ticker | *time.Ticker | 定時觸發 |

**Lifecycle**:
```
Start() → goroutine: for { select { case <-ticker.C: flush(); case <-ctx.Done(): flush(); return } }
flush() → lock dirty → snapshot dirty keys → unlock → batch upsert → clear flushed keys from dirty
```

## Relationships

```
OAuthTokenRateLimitState (DB)
    ↑ flush every 30s
InMemoryRateLimitStore (memory)
    ↑ Set() on every response
native_format.go recordOAuthUsage()

Startup:
  DB → SELECT → InMemoryRateLimitStore.Set() (pre-populate)
```
