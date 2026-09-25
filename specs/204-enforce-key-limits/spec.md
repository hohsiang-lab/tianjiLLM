# Feature Specification: Enforce Per-Key Rate Limits & Budget Controls

**Feature Branch**: `204-enforce-key-limits`
**Created**: 2026-04-01
**Status**: Draft
**Input**: User description: "修復 per-key 限制功能 — UI 上可設定 RPM/TPM Limit、Max Budget、Models 限制、Max Parallel Requests、Key Expiry，但後端 auth middleware 從未將這些欄位注入 request context，導致所有限制形同虛設。"

## Background

管理員透過 UI 為每把 virtual key 設定使用限制（RPM、TPM、Budget、Models、Parallel Requests、Key Expiry），但後端完全沒有 enforce 這些設定。根因：auth middleware 從 DB 讀出完整的 `VerificationToken`，卻只注入 tokenHash、userID、teamID、orgID、guardrails 到 request context。下游的 rate limit、budget、parallel request middleware 從 context 讀不到任何 limit 值，永遠默認為 0 直接放行。

### 現狀問題清單

| 功能           | DB 有值？ | Auth 注入 Context？ | Middleware Enforce？           |
| -------------- | --------- | ------------------- | ----------------------------- |
| RPM Limit      | Yes       | No                  | No（讀到 0，放行）           |
| TPM Limit      | Yes       | No                  | No（讀到 0，放行）           |
| Max Budget     | Yes       | No                  | Budget middleware 未掛載到 chain |
| Max Parallel   | Yes       | No                  | No（讀到 0，放行）           |
| Models 限制    | Yes       | No                  | 從未檢查 per-key models      |
| Key Expiry     | Yes       | No                  | 從未檢查過期時間             |
| Key Blocked    | Yes       | Yes                 | Yes（唯一有效的）            |

## Clarifications

### Session 2026-04-01

- Q: Team/Org 層級的 limit 與 Key 層級的 limit 如何交互？ → A: Key limit 優先，key 沒設才 fallback 到 team，再 fallback 到 org（繼承鏈）
- Q: TPM 限制應該在請求的哪個時間點檢查？ → A: 混合策略 — Pre-check 用已累計消耗量（超限即拒），Post-update 用實際 token 數更新計數器。TPM enforce 是 best-effort，可能略超。此為業界共識（OpenAI、LiteLLM 皆如此）
- Q: Budget check 是讀 DB 還是讀 cache？ → A: B+C 混合 — Auth 讀 DB 值作為 baseline，Redis cache 做 hot-path check，post-call 異步更新 cache + 批量寫 DB。參考 LiteLLM DualCache 架構
- Q: 127 次 429 是天機 retry 還是 client retry？ → A: 經日誌確認是 **client retry**。每個 429 都有獨立的 request.received event，天機自己沒有打 upstream（所有 token 被 5h_gate 跳過），直接回 429。天機已有 Retry-After header（native_format.go:37），但 client 無視了
- Q: Native passthrough 是否受 rate limit 保護？ → A: `/v1/messages`（Anthropic）走 `/v1` route group，**有完整 middleware**。但 `/v1beta/models/*`（Gemini）和 `/anthropic/v1/*`（Batches）是獨立 route group，**只有 Auth，缺少 Parallel/DynamicRate/CacheControl/Budget**

## User Scenarios & Testing

### User Story 1 - RPM/TPM Rate Limiting 生效 (Priority: P1)

管理員為某把 key 設定 RPM=10、TPM=200000，當該 key 的請求超過每分鐘 10 次或每分鐘 200K tokens 時，系統拒絕請求並回傳 429 狀態碼及標準的 rate_limit_exceeded 錯誤。

**Why this priority**: 這是最直接的成本控制手段。真實案例：一把 key 在 17 分鐘內打了 81 次 opus 請求（181K tokens/次），花了 $12.58。RPM/TPM 限制能直接掐住這種暴衝行為。

**Independent Test**: 設定 RPM=5 的 key，連續發 6 個請求，第 6 個應回傳 429。設定 TPM=1000 的 key，發一個 2000 token 的請求後再發一個，第二個應回傳 429。

**Acceptance Scenarios**:

1. **Given** key 設定 RPM=10，**When** 同一分鐘內發送第 11 個請求，**Then** 回傳 HTTP 429 + `{"error": {"type": "rate_limit_exceeded"}}` + `X-RateLimit-*` response headers
2. **Given** key 設定 TPM=200000，**When** 該 key 本分鐘已消耗 200000 tokens 後再次發送請求，**Then** 回傳 HTTP 429
3. **Given** key 沒有設定 RPM/TPM limit（值為 null），**When** 發送任意數量的請求，**Then** 全部放行（向後相容）
4. **Given** key 設定 RPM=10，**When** 過了 60 秒窗口後，**Then** 計數器重置，新請求被允許
5. **Given** key 沒設 TPM limit 但所屬 team 設了 TPM=500000，**When** 該 key 發送請求，**Then** 使用 team 的 TPM limit（繼承鏈：key → team → org）
6. **Given** TPM limit=100000，請求完成後實際消耗 tokens 記錄到 Redis，**When** 累計超過 100000，**Then** 下一個請求被拒（post-update 機制）

---

### User Story 2 - Max Budget 生效 (Priority: P1)

管理員為某把 key 設定 Max Budget=$100，當該 key 的累計花費達到 $100 時，系統拒絕後續所有請求。

**Why this priority**: 與 RPM/TPM 並列最高優先級。Budget 是最後一道防線，防止 key 的總花費失控。

**Independent Test**: 為 key 設定 Max Budget=$1，使用該 key 發送請求直到 spend 超過 $1，驗證後續請求被拒。

**Acceptance Scenarios**:

1. **Given** key 設定 max_budget=$100 且 spend=$99.50，**When** 發送請求，**Then** 允許通過
2. **Given** key 設定 max_budget=$100 且 spend=$100.01，**When** 發送請求，**Then** 回傳 HTTP 429 + budget exceeded 錯誤訊息
3. **Given** key 沒有設定 max_budget（值為 null），**When** 發送任意請求，**Then** 全部放行
4. **Given** key 沒設 max_budget 但所屬 team 設了 max_budget=$500，**When** 該 key 的 team 累計 spend 超過 $500，**Then** 拒絕請求（繼承鏈）
5. **Given** budget check 使用 Redis cache 中的 spend 值，**When** 請求完成後，**Then** 實際 cost 異步更新到 Redis cache + 批量寫 DB

---

### User Story 3 - Models 限制生效 (Priority: P1)

管理員為某把 key 設定只能使用特定模型（如 `["qwen3-30b", "qwen3-coder-next"]`），當該 key 嘗試使用未授權的模型時，系統拒絕請求。

**Why this priority**: 與 Budget 同為 P1。限制可用模型是最有效的成本控制 — 不讓用 opus 就從根本上消滅高成本請求。

**Independent Test**: 為 key 設定 models=["qwen3-30b"]，用該 key 請求 claude-sonnet-4-5，驗證被拒；請求 qwen3-30b，驗證通過。

**Acceptance Scenarios**:

1. **Given** key 設定 models=["qwen3-30b", "qwen3-coder-next"]，**When** 請求 model="claude-opus-4-5"，**Then** 回傳 HTTP 403 + model not allowed 錯誤
2. **Given** key 設定 models=["qwen3-30b"]，**When** 請求 model="qwen3-30b"，**Then** 正常通過
3. **Given** key 的 models 為空陣列或 null，**When** 請求任意 model，**Then** 全部放行（表示不限制）
4. **Given** key 設定 models 包含 wildcard 如 "qwen3-*"，**When** 請求 model="qwen3-30b"，**Then** 正常通過（利用現有 wildcard matching）

---

### User Story 4 - Max Parallel Requests 生效 (Priority: P2)

管理員為某把 key 設定 Max Parallel Requests=3，當該 key 同時有 3 個進行中的請求時，第 4 個請求被拒絕。

**Why this priority**: P2 因為在典型使用場景中請求多為串行（間隔 3-11 秒），但能防止極端並發場景（如開多個 Claude Code 同時跑）。

**Independent Test**: 為 key 設定 max_parallel=2，同時發 3 個 streaming 請求，第 3 個應被拒。

**Acceptance Scenarios**:

1. **Given** key 設定 max_parallel_requests=3 且當前有 3 個進行中的請求，**When** 第 4 個請求到達，**Then** 回傳 HTTP 429 + parallel limit exceeded
2. **Given** key 設定 max_parallel_requests=3 且一個請求完成後只剩 2 個，**When** 新請求到達，**Then** 正常通過
3. **Given** key 沒有設定 max_parallel_requests，**When** 發送任意數量的並發請求，**Then** 全部放行

---

### User Story 5 - Key Expiry 生效 (Priority: P2)

管理員為某把 key 設定到期時間，到期後該 key 的所有請求被拒。

**Why this priority**: P2，因為現有的 blocked 狀態可以手動停用 key 作為替代。但自動到期是必要的安全功能。

**Independent Test**: 為 key 設定 expires 為過去時間，驗證請求被拒。

**Acceptance Scenarios**:

1. **Given** key 設定 expires=2026-03-01（已過期），**When** 發送請求，**Then** 回傳 HTTP 403 + key expired 錯誤
2. **Given** key 設定 expires=2026-12-31（未過期），**When** 發送請求，**Then** 正常通過
3. **Given** key 沒有設定 expires（值為 null），**When** 發送請求，**Then** 正常通過（永不過期）

---

### User Story 6 - Router Backoff + Middleware 覆蓋補齊 (Priority: P1)

兩個子問題：

**6a — Router retry backoff**：當 Router.Route() 嘗試多個 deployment 時，目前零延遲立即重試。需加入 exponential backoff + jitter，且尊重 upstream 的 Retry-After header。此外 RetryPolicy.RetryAfterSeconds 和 TimeoutSeconds 已被解析但從未被使用。

**6b — Middleware chain gap**：Gemini native passthrough（`/v1beta/models/*`）和 Anthropic Batches（`/anthropic/v1/*`）的 route group 只掛了 AuthMiddleware，缺少 Parallel/DynamicRate/CacheControl middleware。這意味著走這些路徑的請求不受任何 rate limit 保護。

**Why this priority**: P1。
- 6a：真實案例中 127 次 429 是 client 無 backoff 重試 + 天機所有 upstream token 被 throttle。雖然直接原因是 client，但 Router retry loop 也需要 backoff 防止非 passthrough 路徑的 thundering herd。
- 6b：Gemini passthrough 完全繞過 rate limit，是安全漏洞。

**Independent Test**:
- 6a: Mock upstream 回傳 429 + Retry-After: 5，驗證 Router 等待至少 5 秒後才重試。
- 6b: 對 `/v1beta/models/gemini-pro:generateContent` 發送超限請求，驗證被 RPM limit 拒絕。

**Acceptance Scenarios**:

1. **Given** upstream 回傳 429，**When** Router 準備重試下一個 deployment，**Then** 等待 exponential backoff 延遲（base=1s, factor=2, max=30s）+ random jitter（0-25%）
2. **Given** upstream 429 response 包含 `Retry-After: N` header，**When** Router 準備重試，**Then** 至少等待 N 秒
3. **Given** 所有 deployments 都在 cooldown，**When** 回傳 429 給客戶端，**Then** response 包含 `Retry-After` header 告知客戶端何時重試
4. **Given** upstream 回傳 429，**When** Router 已用盡所有 retries，**Then** 將 429 傳回客戶端（不無限重試）
5. **Given** RetryPolicy 已設定 RetryAfterSeconds，**When** upstream 返回 429，**Then** 使用該設定值作為 backoff 基準（當前已解析但未使用）
6. **Given** key 設定 RPM=10，**When** 透過 `/v1beta/models/gemini-pro:generateContent` 發送第 11 個請求，**Then** 回傳 429（Gemini route 也受 rate limit 保護）
7. **Given** key 設定 max_budget，**When** 透過 `/anthropic/v1/messages/batches` 發送請求且 budget 已超限，**Then** 回傳 429

---

### Edge Cases

- Limit 值為 0（明確設為 0）vs null：0 = 禁止所有請求，null = 不限制
- Budget 剛好等於 max_budget（spend == max_budget）：視為超限，拒絕（>=）
- Models 陣列包含不存在的模型名稱：忽略，只對請求的模型做匹配
- 多個 limit 同時觸發（RPM 超限 + Budget 超限）：回傳第一個觸發的錯誤
- Redis 不可用時的 RPM/TPM/Parallel 檢查：fail-open（允許通過），避免因 Redis 故障阻斷所有請求
- Streaming 請求的 parallel request 計數：請求開始時 +1，response 結束時 -1
- 繼承鏈解析：key limit 優先；key 沒設時 fallback team；team 沒設時 fallback org；全沒設 = 不限制
- Budget cache 與 DB 不一致：cache 中的 spend 可能略低於 DB 實際值（秒級延遲），budget enforce 是 best-effort，可能略超
- Backoff 溢出：連續 10+ 次 429 時 backoff 不應超過 30 秒上限
- Retry-After header 值異常大（如 3600s）：設合理上限（如 300s），超過則直接拒絕不重試
- Gemini native passthrough（`/v1beta/models/*`）和 Anthropic Batches（`/anthropic/v1/*`）缺少 Parallel/DynamicRate/CacheControl middleware — 需補齊

## Requirements

### Functional Requirements

- **FR-001**: Auth middleware MUST 從 VerificationToken 中提取 rpm_limit、tpm_limit、max_budget、spend、models、expires，並透過 BudgetID 從 BudgetTable 提取 max_parallel_requests，注入 request context
- **FR-002**: 當 key 層級的 limit 為 null 時，MUST fallback 到 team 層級，再 fallback 到 org 層級（繼承鏈：key → team → org）
- **FR-003**: Rate limit middleware MUST 在 rpm_limit > 0 時 enforce RPM 限制
- **FR-004**: Rate limit middleware MUST 在 tpm_limit > 0 時 enforce TPM 限制，採用 pre-check（已累計量）+ post-update（實際值）混合策略
- **FR-005**: Budget middleware MUST 被掛載到 middleware chain，在 spend >= max_budget 時拒絕請求
- **FR-006**: Budget check MUST 使用 Redis cache 做 hot-path 檢查，auth 讀 DB 值作為 baseline，post-call 異步更新 cache + 批量寫 DB
- **FR-007**: Auth middleware MUST 在 models 非空時檢查請求的 model 是否在允許清單中，支援 wildcard matching
- **FR-008**: Auth middleware MUST 在 key 過期時（expires != null && now > expires）拒絕請求
- **FR-009**: Parallel request middleware MUST 在 max_parallel_requests > 0 時 enforce 並發限制
- **FR-010**: 所有 limit 值為 null（且繼承鏈上也無值）時 MUST 保持現有行為（不限制），確保向後相容
- **FR-011**: Rate limit response MUST 包含標準 X-RateLimit-* headers
- **FR-012**: Redis 不可用時，RPM/TPM/Parallel/Budget cache 檢查 MUST fail-open（允許通過）
- **FR-013**: Router retry loop MUST 在每次重試之間加入 exponential backoff（base=1s, factor=2, max=30s）+ random jitter（0-25%）
- **FR-014**: 當 upstream 429 response 包含 Retry-After header 時，Router MUST 至少等待該秒數後再重試
- **FR-015**: Router 已解析但未使用的 RetryPolicy.RetryAfterSeconds 和 RetryPolicy.TimeoutSeconds MUST 被啟用
- **FR-016**: Fallback chain（GeneralFallback, ContentPolicyFallback）MUST 在每次 fallback 之間加入短暫延遲（避免同時轟炸所有 fallback 目標）
- **FR-017**: 當所有 deployments 都被限流時，回傳給客戶端的 429 response MUST 包含 Retry-After header
- **FR-018**: 所有 native passthrough route（Gemini `/v1beta/models/*`、Anthropic Batches `/anthropic/v1/*`）MUST 使用與 `/v1/*` 相同的完整 middleware chain（Auth + Parallel + DynamicRate + CacheControl + Budget）

### Key Entities

- **VerificationToken**: 包含所有 per-key 限制欄位（rpm_limit, tpm_limit, max_budget, spend, models, expires, max_parallel_requests, blocked）
- **Team / Organization**: 包含同樣的 limit 欄位，作為 key 的 fallback 繼承來源
- **Request Context**: middleware 之間傳遞限制值的媒介，需新增注入的 context keys
- **Redis Counters**: RPM（sliding window）、TPM（counter with TTL + post-update）、Parallel（INCR/DECR）
- **Redis Spend Cache**: key spend 的 hot-path cache，post-call 異步更新

## Success Criteria

### Measurable Outcomes

- **SC-001**: 設定 RPM limit 的 key，超限請求被拒率 100%
- **SC-002**: 設定 TPM limit 的 key，超限請求被拒率 100%（best-effort，允許因 post-update 延遲略超）
- **SC-003**: 設定 Max Budget 的 key，spend >= max_budget 後拒絕率 100%（best-effort，允許因 cache 延遲略超）
- **SC-004**: 設定 Models 限制的 key，未授權 model 請求拒絕率 100%
- **SC-005**: 設定 Max Parallel Requests 的 key，超限並發請求拒絕率 100%
- **SC-006**: 過期 key 的請求拒絕率 100%
- **SC-007**: 未設定任何 limit 的 key，行為與修改前完全一致（零回歸）
- **SC-008**: Redis 故障期間，所有請求仍可正常通過（fail-open）
- **SC-009**: 繼承鏈正確運作：key 無 limit → 使用 team limit → team 無 limit → 使用 org limit
- **SC-010**: Upstream 429 後重試間隔符合 exponential backoff（1s, 2s, 4s...），不再出現每秒重試
- **SC-011**: Upstream 429 含 Retry-After header 時，重試延遲 >= Retry-After 值

## Assumptions

- Redis 已部署且可用（現有架構已依賴 Redis）
- VerificationToken DB schema 已包含所有必要欄位 — 從代碼確認已存在
- Team / Organization 表也包含 rpm_limit、tpm_limit、max_budget 欄位
- 現有的 DynamicRateLimiter 和 RateLimiter 邏輯正確，只需確保 limit 值被正確傳入
- Budget middleware 的 check 邏輯已實現，只需掛載到 middleware chain
- Parallel request middleware 的 Redis INCR/DECR 邏輯已實現，只需注入 limit 值
- Post-call spend 更新可複用現有 callback 系統
