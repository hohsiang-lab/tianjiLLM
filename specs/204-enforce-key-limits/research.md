# Research: Enforce Per-Key Rate Limits & Budget Controls

## R1: TPM Enforcement Strategy

**Decision**: Pre-check 已累計消耗量 + Post-update 實際 token 數（mixed strategy）

**Rationale**:
- OpenAI 使用 estimated_tokens（character_count/4 + max_tokens）做 pre-check，完成後用實際值更新
- LiteLLM 明確文件：「TPM enforcement is best-effort and may slightly exceed the limit, because token count is unknown until the LLM responds」
- LiteLLM `parallel_request_limiter.py` 在 pre-call hook 中檢查 `current_tpm < tpm_limit`，post-call 更新 `current_tpm`

**Alternatives considered**:
- Pre-check only（input token 估算）：output tokens 未知，估算不準
- Post-check only：超限請求已經完成，浪費資源
- **Mixed** 是業界共識

**Sources**:
- [OpenAI Rate Limits](https://developers.openai.com/api/docs/guides/rate-limits)
- [LiteLLM parallel_request_limiter.py](https://github.com/BerriAI/litellm/blob/main/litellm/proxy/hooks/parallel_request_limiter.py)
- [LiteLLM Load Balancing Docs](https://github.com/berriai/litellm/blob/main/docs/my-website/docs/proxy/load_balancing.md)

## R2: Budget Check Architecture

**Decision**: Auth 讀 DB baseline + Redis cache hot-path check + Post-call 異步更新

**Rationale**:
- LiteLLM 使用 DualCache（in-memory + Redis），首次 auth 查 DB/Cache，後續讀 cache
- `async_increment` 比 `async_set_cache` 快 2x，in-memory 每 0.01s sync 到 Redis
- 100 RPS × 3 instances 測試，drift 最多 10 個請求
- 天機已有 spend tracker（`internal/spend/tracker.go`）做 post-call DB 更新，只需加 Redis cache 更新

**Alternatives considered**:
- 每次查 DB：精確但增加 1 DB round-trip（~5ms）
- 純 Redis cache：需要額外 cache warm-up 邏輯
- **混合**：auth 已查 DB（TokenInfo），budget middleware 讀 context 中的值做 pre-check，post-call 更新 Redis

**Sources**:
- [LiteLLM Architecture](https://deepwiki.com/BerriAI/litellm/3.5.1-api-key-management)
- [LiteLLM db_spend_update_writer.py](https://github.com/BerriAI/litellm/blob/main/litellm/proxy/db/db_spend_update_writer.py)

## R3: Limit 繼承鏈（Key → Team → Org）

**Decision**: Key limit 優先，null 時 fallback Team，再 fallback Org

**Rationale**:
- LiteLLM 的 budget 也有 team/user 層級，team budget 覆蓋 key
- 繼承鏈需要額外 DB query（GetTeam, GetOrganization），但 TeamID/OrgID 已在 VerificationToken 中
- auth middleware 已注入 teamID/orgID，只需條件性查 team/org 當 key limit 為 null

**Implementation approach**:
- auth middleware 中：讀 VerificationToken → 如果某 limit 為 nil 且 teamID != nil → GetTeam → 取 team limit
- 如果 team limit 也為 nil 且 orgID != nil → GetOrganization → 取 org limit
- 用 resolved limit 注入 context

**Concern**: 額外 DB queries 增加延遲
- GetTeam/GetOrganization 都是 simple primary key lookup，~1ms
- 只在 limit 為 nil 時觸發（大部分 key 會直接設 limit）
- 可考慮未來加 cache，但 v1 先直接查

## R4: 現有 Middleware 修改分析

**Decision**: 最小化修改，複用現有邏輯

**Findings**:

1. **auth.go** — TokenInfo 只有 UserID/TeamID/OrgID/Blocked/Guardrails。需擴展加入：RpmLimit, TpmLimit, MaxBudget, Spend, MaxParallelRequests, Models, Expires

2. **db_validator.go** — ValidateToken() 已讀 VerificationToken 全部欄位，只是沒 map 到 TokenInfo。改動：增加 mapping

3. **server.go** — middleware chain 缺 budget。改動：在 parallel 和 dynamicRate 之間插入 budget middleware

4. **budget.go** — 現有邏輯直接查 DB。改動：改為讀 context（spend, max_budget），不再獨立查 DB

5. **parallel.go** — 邏輯完整，只是 context key 從沒被設。auth 注入後即可工作

6. **dynamic_ratelimit.go** — 邏輯完整，同 parallel

7. **helpers.go** — 需新增 context keys：maxBudgetKey, spendKey, modelsKey, expiresKey

## R5: max_parallel_requests 來源

**Decision**: 從 BudgetTable 經 BudgetID join 取得

**Rationale**:
- VerificationToken 有 `BudgetID` 但沒有 `max_parallel_requests` 欄位
- `max_parallel_requests` 存在 BudgetTable 中
- 需要額外 query GetBudget(budgetID) 來取得

**Alternative**: 直接在 VerificationToken 加欄位（需 migration）— 過度，用 join 即可

## R6: Models Check — Auth 層 Body Buffering

**Decision**: Auth middleware 做 body buffering + model extraction + models check（與 LiteLLM 對齊）

**Rationale**:
- LiteLLM 在 auth 層（`user_api_key_auth.py`）就讀 request body，呼叫 `get_model_from_request(request_data, route)` 提取 model name，再呼叫 `can_key_call_model()` 做 access check
- Python（FastAPI）自動 cache body，Go 的 `http.Request.Body` 是 `io.ReadCloser`，讀一次就消耗
- Go 的標準做法：`io.ReadAll` + `bytes.NewReader` 重置 body，在 reverse proxy / API gateway 中非常常見
- LLM 請求 body 通常 < 1MB（text messages），記憶體消耗可忽略

**Implementation**:
```go
// auth.go — 在 models check 時
bodyBytes, err := io.ReadAll(r.Body)
r.Body = io.NopCloser(bytes.NewReader(bodyBytes))  // reset for handler

// partial decode — 只需 model 欄位
var partial struct { Model string `json:"model"` }
json.Unmarshal(bodyBytes, &partial)

// 也從 URL path 提取（如 /openai/deployments/gpt-4）
if partial.Model == "" {
    partial.Model = extractModelFromPath(r.URL.Path)
}
```

**Alternatives considered**:
1. Auth 注入 allowedModels 到 context，handler 層做 check — 分散限制邏輯，不統一
2. 新增 middleware 專門做 model check — 過度抽象，增加複雜度
3. **Auth 層做 body buffering** — 與 LiteLLM 一致，所有 per-key 限制集中在一處

**Sources**:
- [LiteLLM auth_utils.py — get_model_from_request](https://github.com/BerriAI/litellm/blob/main/litellm/proxy/auth/auth_utils.py)
- [LiteLLM auth_checks.py — can_key_call_model](https://github.com/BerriAI/litellm/blob/main/litellm/proxy/auth/auth_checks.py)
- [LiteLLM user_api_key_auth.py](https://github.com/BerriAI/litellm/blob/main/litellm/proxy/auth/user_api_key_auth.py)

## R7: Post-Call Redis Spend Cache 更新

**Decision**: spend/tracker.go 的 Record() 中新增 `INCRBYFLOAT tianji:spend:{keyHash} cost`

**Rationale**:
- 目前 tracker.go 只做：① Redis buffer push（spend log）② DB UpdateVerificationTokenSpend
- 沒有更新 Redis 中的 spend cache key — budget middleware 的 hot-path check 需要這個
- 新增一行 `rdb.IncrByFloat(ctx, "tianji:spend:"+keyHash, cost)` 即可
- 同時需要在 Record() 中加 TPM counter 更新：`rdb.IncrBy(ctx, "tianji:dynamic_tpm:"+keyHash, totalTokens)` + TTL 60s

**Concern**: tracker.go 目前沒有 Redis client 引用
- tracker 有 `buffer *SpendBuffer` which wraps Redis — 可以透過 buffer 暴露 Redis client
- 或在 Tracker struct 中直接注入 `redis.UniversalClient`
