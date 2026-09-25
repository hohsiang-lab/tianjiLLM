# Feature Specification: Persist RateLimitStore to Database

**Feature Branch**: `199-persist-ratelimit-store`
**Created**: 2026-03-26
**Status**: Draft
**Input**: User description: "是不是應該讓 usage 的資料存在 database 裡？"

## Clarifications

### Session 2026-03-26

- Q: DB 寫入頻率控制 — 每次 response 都寫 DB，還是有節流機制？ → A: Periodic flush 每 30 秒 + graceful shutdown flush。參考 Grafana Alerting 和 sing-box 的生產模式。In-memory store 仍然每次 response 即時更新。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Pod 重啟後保留 Token Utilization 數據 (Priority: P1)

運維人員部署新版本或 pod 被 Kubernetes 重新調度後，系統重啟。第一個 native Anthropic 請求進來時，系統從資料庫載入各 OAuth token 的最近 utilization 數據，而不是把所有 token 都當成空閒。`lowest_utilization` 策略能正確選擇 utilization 最低的 token，避免把流量打到已經接近限額的 token。

**Why this priority**: 這是 persistence 的核心價值。目前每次 pod 重啟，RateLimitStore 歸零，所有 token 被當成 idle（composite=0），導致已滿的 token 被優先選中（因為 0 < 任何正數）。用戶會看到 429 或 rejected 錯誤。

**Independent Test**: 啟動服務，確認 RateLimitStore 從 DB 預載了之前的 utilization 數據，第一個請求選擇了 utilization 最低的 token。

**Acceptance Scenarios**:

1. **Given** 兩個 OAuth token（A: 5h=4%, B: 5h=60%）的 utilization 數據已存在資料庫，**When** pod 重啟並收到第一個 `/v1/messages` 請求，**Then** 系統選擇 token A（4%），不選 token B。
2. **Given** 資料庫裡有 utilization 數據但已超過保鮮期（例如 1 小時前），**When** pod 重啟，**Then** 過期數據被忽略，系統 fallback 到 round-robin。
3. **Given** 資料庫裡沒有任何 utilization 數據（全新部署），**When** pod 重啟並收到請求，**Then** 系統走 round-robin，行為與目前相同。

---

### User Story 2 - Periodic Flush 將 Utilization 數據寫入 DB (Priority: P1)

系統每 30 秒將 in-memory RateLimitStore 中有變更的 token 狀態批量寫入資料庫。In-memory store 仍然每次 response 即時更新（現有行為不變）。Pod 正常關閉時做最後一次 flush，確保零數據丟失。

**Why this priority**: 沒有寫入就沒有數據可讀，這是 persistence 的另一半。Periodic flush 相比每次 response 寫 DB，減少 90%+ 的 DB 寫入量。

**Independent Test**: 發送多個 `/v1/messages` 請求，等待 30 秒，確認資料庫裡有對應 token 的 utilization 記錄。

**Acceptance Scenarios**:

1. **Given** 10 個連續 Anthropic 請求在 5 秒內完成，**When** 30 秒 flush 週期到達，**Then** 資料庫只寫入 1 次（包含最新狀態），而非 10 次。
2. **Given** 一個 429 response 更新了 in-memory store 的 5h_util=1.00，**When** 30 秒 flush 到達，**Then** 資料庫記錄更新為 5h_util=1.00。
3. **Given** 資料庫不可用（連線錯誤），**When** flush 嘗試寫入，**Then** in-memory store 不受影響，flush 失敗被記錄到 log，下次 flush 重試。
4. **Given** pod 收到 SIGTERM 準備關閉，**When** graceful shutdown 開始，**Then** 系統做最後一次 flush 將所有 dirty keys 寫入 DB。

---

### User Story 3 - 多 Pod 間共享 Token 狀態 (Priority: P2)

當多個 pod 同時運行（水平擴展），每個 pod 的 in-memory store 只有自己處理過的請求的數據。透過 DB 持久化，新啟動的 pod 能看到其他 pod 寫入的最新 token 狀態，避免多個 pod 同時把流量打到同一個低利用率 token。

**Why this priority**: 單 pod 時不需要，但多 pod 部署時 DB 共享是避免 thundering herd 的關鍵。

**Independent Test**: 啟動兩個 pod，Pod A 處理請求更新 token 狀態到 DB，Pod B 重啟後從 DB 載入 Pod A 寫的數據。

**Acceptance Scenarios**:

1. **Given** Pod A 處理了 10 個請求，token X 的 5h_util 更新到 50%，**When** Pod B 啟動，**Then** Pod B 的 store 裡 token X 的 5h_util 反映 DB 中的 50%。
2. **Given** Pod A 和 Pod B 同時 flush 同一個 token 的狀態，**When** 兩筆寫入幾乎同時發生，**Then** 資料庫保留較新的那筆（last-write-wins），不會報錯。

---

### Edge Cases

- 全新 token（從沒被任何 pod 打過，DB 裡沒有記錄）→ store miss，走 round-robin 試一次後正常。這是預期行為。
- Flush 期間 DB 暫時不可用 → dirty keys 保留在 memory，下次 flush 重試。不丟數據。
- Pod crash（非 graceful shutdown）→ 最多丟失 30 秒內的變更。下一個 response 會重新填充。
- Token 被替換（env var 改了新 token）→ 舊 token 的 DB 記錄自然過期（保鮮期），新 token 從第一次請求開始累積。
- DB migration 失敗 → 系統退化為純 in-memory 模式（現有行為），不應該 crash。
- 5h session reset（Anthropic 的 5h window 到期）→ 下一次 response 帶回 reset 後的新 utilization，下次 flush 覆蓋 DB 中的舊值。
- Flush 期間同時有新 response 更新 in-memory store → flush 讀取的是 snapshot，新 response 的更新在下次 flush 寫入。不會互相干擾。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系統 MUST 每 30 秒將 in-memory RateLimitStore 中有變更的 token 狀態批量寫入資料庫（periodic flush）。
- **FR-002**: 系統 MUST 在啟動時從資料庫載入所有 token 的最近 utilization 數據到 in-memory RateLimitStore。
- **FR-003**: 系統 MUST 忽略資料庫中超過保鮮期的記錄（預設 30 分鐘），避免使用過時數據做路由決策。
- **FR-004**: 系統 MUST 在收到 SIGTERM/graceful shutdown 時做最後一次 flush，確保 dirty keys 全部寫入 DB。
- **FR-005**: 資料庫不可用時，系統 MUST 退化為純 in-memory 模式，功能不受影響。flush 失敗的 dirty keys 保留到下次 flush 重試。
- **FR-006**: 同一 token 的多次寫入 MUST 使用 upsert（insert-or-update），last-write-wins。
- **FR-007**: DB 記錄 MUST 以 token cache key（SHA256[:6] hex）為主鍵，與現有 `OAuthTokenMetadata` 表的 `token_key` 欄位格式一致。
- **FR-008**: 系統 MUST 保留現有 in-memory RateLimitStore 作為主要讀取源。DB 僅用於啟動時預載和跨 pod 同步。
- **FR-009**: Periodic flush MUST 只寫入自上次 flush 以來有變更的 token（dirty tracking），避免冗餘寫入。

### Key Entities

- **Token Rate Limit State**: 一個 OAuth token 在某個時間點的 utilization 快照。包含 token cache key、5h/7d/7d_sonnet utilization、unified status、reset times、organization ID、最後更新時間。
- **In-Memory RateLimitStore**: 現有的即時數據源，每次 response 更新。Pod 重啟時從 DB 預載。
- **Persisted Rate Limit State**: DB 中的持久化記錄。由 periodic flush 產生。啟動時載入到 in-memory store。
- **Dirty Set**: 追蹤自上次 flush 以來哪些 token 的狀態有變更，用於決定 flush 時寫入哪些 keys。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Pod 重啟後第一個請求不再盲選 — utilization 最低的 token 被正確選中（而非 round-robin 或 composite=0 的 store miss token）。
- **SC-002**: DB 寫入頻率 ≤ 每 30 秒一次 batch upsert，不隨請求量線性增長。
- **SC-003**: 資料庫不可用時，系統行為與目前完全相同（純 in-memory）。
- **SC-004**: 多 pod 部署時，新啟動的 pod 在第一個請求前就有其他 pod 寫入的 token 狀態。
- **SC-005**: Graceful shutdown 後 DB 中的數據反映 shutdown 前最新的 token 狀態。

## Assumptions

- 現有 `OAuthTokenMetadata` 表只存 `token_key` 和 `org_id`，不存 utilization 數據。需要新增表或擴展現有表。
- Periodic flush 每 30 秒，以 2 個 OAuth token 為例，每次 flush 最多 2 筆 upsert。DB 負載可忽略。
- 保鮮期 30 分鐘是基於 Anthropic 5h session window 的合理預設。5h window 裡的 utilization 變化很快，超過 30 分鐘的數據已不具參考價值。
- In-memory store 仍然是 request-path 上的唯一讀取源。DB 不在 hot path 上被查詢（只有啟動時一次性載入）。
- Pod crash（非 graceful）最多丟失 30 秒數據 — 可接受，因為下一個 response 就會重新填充 store。
