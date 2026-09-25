# Research: Persist RateLimitStore to Database

**Feature**: 199-persist-ratelimit-store
**Date**: 2026-03-26

## Decision 1: Flush 策略 — Periodic Flush + Graceful Shutdown

**Decision**: 用 `time.Ticker` 每 30 秒 flush dirty keys 到 DB。Pod shutdown 時做最後一次 flush。參考 Grafana Alerting (`persister_async.go`) 和 sing-box (`service_usage.go`)。

**Rationale**:
- 業界六種主流模式（periodic flush, debounce, threshold+timer, VSA, channel batcher, rate-limited save），periodic flush 最簡單且適合 rate limit 狀態場景。
- Grafana Alerting 用 1 分鐘間隔 + graceful shutdown flush，我們用 30 秒（rate limit 數據更時效性）。
- Crash 最多丟 30 秒數據 — 下一個 response 就會重新填充。可接受。
- LiteLLM 完全不持久化 cooldown 到 DB（只用 memory + Redis）。我們做得更好。

**Alternatives Considered**:
1. **Debounce per-key**: 每個 token 一個 timer，靜默後寫入。Rejected — per-key timer 管理複雜，比 periodic flush 沒有實質好處。
2. **每次 response 都寫 DB**: 簡單但浪費。10 次/秒 × 2 tokens = 20 次/秒 upsert。Rejected。
3. **Threshold-based**: utilization 變化超過 1% 才寫。Rejected — 需要額外的 diff tracking 邏輯，且低頻更新場景下數據可能長時間不寫入。

## Decision 2: DB Schema — 新增 Table vs 擴展 OAuthTokenMetadata

**Decision**: 新增 `OAuthTokenRateLimitState` 表，不擴展 `OAuthTokenMetadata`。

**Rationale**:
- `OAuthTokenMetadata` 只存 `token_key` + `org_id`，語義是 token 的身份信息，很少更新。
- Rate limit state 是高頻更新的暫時性數據（5h/7d utilization），語義完全不同。
- 分表遵循單一職責：metadata = 身份，rate_limit_state = 即時狀態。
- 分表也方便獨立設定 retention policy（rate limit state 可以更積極地清理）。

**Alternatives Considered**:
1. **擴展 OAuthTokenMetadata 加 utilization 欄位**: 更少 migration，但混合了兩種更新頻率完全不同的數據。Rejected。
2. **用 Redis**: 比 PostgreSQL 更快，但增加了基礎設施依賴。目前 Redis 是 optional（cache），不想讓 rate limit 功能依賴 Redis。Rejected。

## Decision 3: Dirty Tracking — Set Flag on Write

**Decision**: 在 `InMemoryRateLimitStore.Set()` 時標記 key 為 dirty。Flush 時讀取 dirty set，寫入 DB，清除 dirty flags。

**Rationale**:
- 最簡單的 dirty tracking：一個 `map[string]bool` 加 mutex。
- Set 時 `dirty[key] = true`。Flush 時 snapshot dirty keys，批量 upsert，清空 dirty map。
- 不需要比較新舊值（diff），因為 rate limit state 的每次更新都應該被持久化。

**Alternatives Considered**:
1. **比較新舊值只寫變化的欄位**: 更精細但複雜度不值得。Upsert 整個 row 很便宜。Rejected。
2. **全量 flush（不 track dirty）**: 每次 flush 寫入所有 keys。Token 數量通常 < 10，全量也可以。Rejected — dirty tracking 多了幾行 code 但在 token 數量增長時更好。

## Decision 4: 啟動時載入 — 一次性 SELECT

**Decision**: 啟動時 `SELECT * FROM OAuthTokenRateLimitState WHERE updated_at > now() - interval '30 minutes'`，結果轉為 `AnthropicOAuthRateLimitState` 寫入 in-memory store。

**Rationale**:
- 一次性載入，不在 hot path。
- 30 分鐘 TTL 過濾掉過期數據。
- Token 數量通常 < 10，SELECT 結果集很小。

**Alternatives Considered**:
1. **Lazy load（第一次 Get 時從 DB 查）**: 在 hot path 上加了 DB query。Rejected。
2. **不過濾直接全量載入**: 可能載入過期數據導致錯誤路由。Rejected。
