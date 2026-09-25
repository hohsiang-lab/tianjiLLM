# Quickstart: Enforce Per-Key Rate Limits & Budget Controls

## 修改清單

### 1. 擴展 TokenInfo（db_validator.go）

TokenInfo struct 加入 limit 欄位。ValidateToken() 從 VerificationToken map 到 TokenInfo 時，加入 RpmLimit、TpmLimit、MaxBudget、Spend、MaxParallelRequests、Models、Expires 的 mapping。

如果 key 的 limit 為 nil，條件性查 Team/Org 做 fallback：
- teamID != nil → GetTeam() → 取 team 的 limit
- orgID != nil → GetOrganization() → 取 org 的 limit
- budgetID != nil → GetBudget() → 取 max_parallel_requests

### 2. Auth middleware — context injection + body buffering + checks（auth.go）

**Context injection** — Virtual key 路徑加入 context.WithValue：
- `rpmLimitKey` = info.RpmLimit（deref to int64, or 0 if nil）
- `tpmLimitKey` = info.TpmLimit
- `maxParallelLimitKey` = info.MaxParallelRequests
- `maxBudgetKey` = info.MaxBudget
- `spendKey` = info.Spend

**Expiry 檢查**（在注入 context 之前）：
- if info.Expires.Valid && time.Now().After(info.Expires.Time) → 403

**Models 檢查 — body buffering**（參考 LiteLLM `get_model_from_request`）：
```go
// 1. Buffer body
bodyBytes, _ := io.ReadAll(r.Body)
r.Body = io.NopCloser(bytes.NewReader(bodyBytes))  // reset for handler

// 2. Extract model (partial decode)
var partial struct { Model string `json:"model"` }
json.Unmarshal(bodyBytes, &partial)

// 3. Fallback: extract from URL path (/openai/deployments/gpt-4)
if partial.Model == "" {
    partial.Model = extractModelFromPath(r.URL.Path)
}

// 4. Check against allowed models (supports wildcard via wildcard.Match)
if len(info.Models) > 0 && partial.Model != "" {
    if !isModelAllowed(partial.Model, info.Models) {
        → 403 model not allowed
    }
}
```

Body buffering 只在 info.Models 非空時執行，避免不必要的 buffer。

### 3. 掛載 Budget middleware（server.go）

在 middleware chain 中加入 budget middleware：
```
AuthMiddleware → BudgetMW → ParallelMW → DynamicRateMW → CacheControlMW
```

### 4. Budget middleware 改讀 context（budget.go）

改為從 context 讀 spend 和 max_budget，不再獨立查 DB。同時檢查 Redis spend cache（`tianji:spend:{keyHash}`）取更新值：
```go
dbSpend := context.Value(spendKey).(float64)
cacheSpend := rdb.Get("tianji:spend:" + keyHash)  // may be more recent
spend := max(dbSpend, cacheSpend)  // 取較大值
if maxBudget != nil && spend >= *maxBudget → 429
```

### 5. Post-call 更新 Redis（tracker.go）

在 Record() 中新增：
- `INCRBYFLOAT tianji:spend:{keyHash} cost` — spend cache for budget check
- `INCRBY tianji:dynamic_tpm:{keyHash} totalTokens` + Expire 60s — TPM counter for rate limit

需要在 Tracker struct 注入 `redis.UniversalClient`。

### 6. 新增 context keys（helpers.go）

新增 maxBudgetKey、spendKey context key 定義。

## 驗證流程

```bash
# 1. 寫 failing tests
go test ./internal/proxy/middleware/... -run "TestAuthMiddleware_Injects" -v
# Expected: FAIL (tests don't exist yet)

# 2. 實作後驗證
make test

# 3. 全量 check
make check
```
