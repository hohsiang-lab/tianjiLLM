# Feature Specification: Guardrail 管理 UI

**Feature Branch**: `006-guardrail-mgmt-ui`
**Created**: 2026-02-27
**Status**: Draft
**Ticket**: HO-22

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 查看與管理 Guardrail 列表 (Priority: P1)

管理員進入 Guardrail 管理頁面，能夠一覽所有已設定的 guardrail rules，了解每條規則的名稱、類型、狀態（啟用/停用），並可即時切換啟用狀態，無需額外頁面跳轉。搜尋與分頁功能讓管理員能在 guardrail 數量龐大時快速定位。

**Why this priority**: 這是整個功能的入口。管理員首先需要能看到並控制所有 guardrail 規則，才有辦法進行後續的管理操作。沒有列表頁，其餘功能無從使用。

**Independent Test**: 部署後訪問 `/ui/guardrails`，可看到 guardrail 列表，並能在不離開頁面的情況下切換某條規則的啟用/停用狀態，列表即時更新反映新狀態。

**Acceptance Scenarios**:

1. **Given** 系統中已有多條 guardrail configs，**When** 管理員訪問 guardrail 管理頁面，**Then** 應顯示所有 guardrail 的名稱、類型、failure policy、啟用狀態與建立時間
2. **Given** guardrail 列表已顯示，**When** 管理員在搜尋框輸入關鍵字，**Then** 列表即時篩選出名稱包含該關鍵字的 guardrail
3. **Given** 系統中有超過預設每頁顯示數量的 guardrail，**When** 管理員到達列表底部，**Then** 可透過分頁控制瀏覽其他 guardrail
4. **Given** 某 guardrail 當前為啟用狀態，**When** 管理員點擊其啟用/停用 toggle，**Then** 該 guardrail 立即切換為停用狀態，頁面無需完整重載
5. **Given** 系統中尚無任何 guardrail config，**When** 管理員訪問 guardrail 管理頁面，**Then** 應顯示空狀態提示，並提供新增引導入口

---

### User Story 2 - 新增、編輯與刪除 Guardrail Config (Priority: P2)

管理員能夠透過表單介面新增 guardrail 規則，填寫名稱、選擇類型（30+ 種 provider 類型）、設定 JSON 格式的 config、選擇 failure policy、以及設定初始啟用狀態。同樣地，可以修改現有規則的所有欄位，或刪除不再需要的規則。

**Why this priority**: 列表頁提供了可見性，但管理員必須能夠實際建立與維護規則，才能讓整個 guardrail 系統發揮作用。CRUD 是 guardrail 管理功能的核心操作。

**Independent Test**: 從列表頁點擊「新增」，填寫所有必要欄位並儲存，新 guardrail 出現在列表中；點擊某 guardrail 的「編輯」，修改名稱後儲存，列表中該規則名稱更新；點擊「刪除」並確認，該規則從列表中移除。

**Acceptance Scenarios**:

1. **Given** 管理員在 guardrail 列表頁，**When** 點擊「新增 Guardrail」，**Then** 顯示新增表單，包含名稱、類型選擇、config JSON 輸入區、failure policy 選擇與啟用狀態
2. **Given** 管理員填寫完整的新增表單，**When** 點擊儲存，**Then** 新 guardrail 建立成功並出現在列表中，並顯示成功提示
3. **Given** 管理員提交 config JSON 格式有誤的表單，**When** 嘗試儲存，**Then** 顯示格式錯誤提示，不執行儲存，表單保留已輸入資料
4. **Given** guardrail 列表中有現有規則，**When** 管理員點擊某規則的「編輯」，**Then** 開啟預填現有資料的編輯表單
5. **Given** 管理員修改編輯表單中的任何欄位並儲存，**When** 操作成功，**Then** 列表中該規則的資料即時更新
6. **Given** 管理員點擊某 guardrail 的「刪除」，**When** 出現確認提示並點擊確認，**Then** 該規則從列表中移除，並顯示成功提示
7. **Given** guardrail 名稱與現有規則重複，**When** 嘗試建立，**Then** 顯示名稱已存在的錯誤訊息

---

### User Story 3 - 查看與管理 Policy Binding (Priority: P3)

管理員能夠在 guardrail 詳細頁面中，查看該 guardrail 透過哪些 Policy 被套用到哪些 API key 或 team 上。同時可以查看哪些 key/team 未綁定任何 guardrail。管理員能夠建立或移除 policy 與 guardrail 的綁定關係。

**Why this priority**: Policy binding 讓 guardrail 能精確套用到特定用戶或團隊，是 guardrail 系統精細化管理的關鍵。但這在列表和 CRUD 之後，因為必須先有 guardrail 規則才能進行綁定。

**Independent Test**: 進入某 guardrail 的詳細頁，能看到目前透過哪些 Policy 套用到哪些 key/team，並能成功新增一個新 binding（選擇某 key 或 team），頁面即時更新顯示新增的 binding。

**Acceptance Scenarios**:

1. **Given** 某 guardrail 已透過 Policy 綁定至若干 key 或 team，**When** 管理員進入該 guardrail 的詳細頁，**Then** 應顯示目前所有相關 policy 及其所套用的 key/team 清單
2. **Given** 某 guardrail 尚未被任何 policy 使用，**When** 管理員查看其 binding 區塊，**Then** 顯示「尚無 policy 使用此 guardrail」的提示
3. **Given** guardrail 詳細頁顯示 binding 清單，**When** 管理員點擊「新增 Binding」，**Then** 可選擇 policy、scope（key 或 team）並新增關聯
4. **Given** 現有 binding 不再需要，**When** 管理員點擊某 binding 的「移除」並確認，**Then** 該 binding 從清單中移除

---

### User Story 4 - 即時測試 Guardrail 效果 (Priority: P4)

管理員在 guardrail 詳細頁面中可以輸入一段 prompt 文字，對選定的 guardrail 執行即時測試，並立即看到測試結果：該 guardrail 是判定 passed（通過）還是 blocked（攔截），以及對應的理由或訊息。

**Why this priority**: 測試功能讓管理員在部署 guardrail 前能夠驗證其行為是否符合預期，避免設定錯誤導致誤攔截或漏放。這是輔助功能，不影響主要管理流程的可用性。

**Independent Test**: 在某 guardrail 的詳細頁，輸入一段已知應被攔截的 prompt，點擊「測試」，頁面顯示「blocked」結果及攔截理由；再輸入一段應通過的 prompt，點擊「測試」，頁面顯示「passed」結果。

**Acceptance Scenarios**:

1. **Given** 管理員在 guardrail 詳細頁的測試區塊，**When** 輸入 prompt 文字並點擊「執行測試」，**Then** 頁面顯示測試結果（passed 或 blocked）及說明訊息
2. **Given** 測試對象 guardrail 評估結果為 blocked，**When** 結果顯示，**Then** 明確標示「Blocked」狀態及攔截原因
3. **Given** 測試對象 guardrail 評估結果為 passed，**When** 結果顯示，**Then** 明確標示「Passed」狀態
4. **Given** guardrail 所需的外部服務（如第三方 API）無法連線，**When** 執行測試，**Then** 顯示服務不可用的錯誤提示，不顯示假結果
5. **Given** 測試輸入為空，**When** 點擊「執行測試」，**Then** 顯示「請輸入測試 prompt」提示，不執行測試

---

### Edge Cases

- 當 guardrail config JSON 欄位格式錯誤時，系統如何回應並引導修正？
- 刪除一個仍被現有 Policy 參照的 guardrail 時，系統應如何處理（警告、拒絕或級聯移除 binding）？
- 同時有多個管理員操作同一 guardrail 時，後儲存者是否覆蓋前者，或需要樂觀鎖定提示？
- guardrail 類型為需要外部服務金鑰的 provider 時，config 欄位是否需要遮蔽敏感資訊顯示？
- 測試功能在 guardrail 為停用狀態時，是否仍可執行測試（基於當前 config 評估，不受 enabled 狀態影響）？

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系統 MUST 提供 guardrail 管理頁面，顯示所有 guardrail configs 的名稱、類型、failure policy、啟用狀態與建立時間
- **FR-002**: 系統 MUST 支援依名稱關鍵字搜尋 guardrail，搜尋結果即時更新
- **FR-003**: 系統 MUST 支援 guardrail 列表分頁顯示，每頁數量合理（預設 20 筆）
- **FR-004**: 管理員 MUST 能夠在列表頁直接切換任一 guardrail 的啟用/停用狀態，無需頁面跳轉
- **FR-005**: 管理員 MUST 能夠新增 guardrail config，必填欄位包含：guardrail 名稱（唯一）、guardrail 類型（從 30+ 支援類型中選擇）、config JSON、failure policy
- **FR-006**: 系統 MUST 驗證 config JSON 格式的有效性，並在格式錯誤時阻止儲存並顯示明確錯誤訊息
- **FR-007**: 管理員 MUST 能夠編輯現有 guardrail config 的所有欄位
- **FR-008**: 管理員 MUST 能夠刪除 guardrail config，刪除前須顯示確認提示
- **FR-009**: 系統 MUST 在嘗試以重複名稱建立 guardrail 時拒絕操作並提示名稱已存在
- **FR-010**: 管理員 MUST 能夠在 guardrail 詳細頁查看目前所有透過 Policy 套用此 guardrail 的 key/team 清單
- **FR-011**: 管理員 MUST 能夠在 guardrail 詳細頁新增 policy binding，選擇 policy 與適用範圍（key 或 team）
- **FR-012**: 管理員 MUST 能夠在 guardrail 詳細頁移除現有的 policy binding
- **FR-013**: 管理員 MUST 能夠在 guardrail 詳細頁輸入 prompt 文字並執行即時測試
- **FR-014**: 系統 MUST 對測試請求回傳明確的 passed 或 blocked 結果，blocked 時包含攔截原因說明
- **FR-015**: 系統 MUST 在執行測試失敗（如外部服務不可用）時顯示明確錯誤，不回傳假結果
- **FR-016**: guardrail 管理 UI MUST 整合至現有管理後台的側邊導覽列，讓管理員能從其他頁面直接跳轉

### Key Entities

- **Guardrail Config**：一條 guardrail 規則，具有唯一名稱、類型（決定使用哪種評估引擎）、JSON 格式的 provider 特定設定、failure policy（決定評估失敗時的行為），以及啟用/停用狀態
- **Guardrail Type**：guardrail 評估引擎的種類，目前支援 30+ 種（openai_moderation、presidio、lakera_guard 等），每種類型對應不同的外部或內建評估服務
- **Failure Policy**：guardrail 評估過程中發生錯誤時的處理策略（如 fail_open 允許通過 vs fail_closed 拒絕通過）
- **Policy**：一組將 guardrail 規則套用到特定範圍的規則集合，包含條件、要啟用的 guardrails 列表及執行管線設定
- **Policy Attachment**：將 Policy 關聯到特定 scope（API keys、teams、models 或 tags）的綁定記錄
- **Guardrail Test Result**：即時測試的輸出，包含評估結論（passed/blocked）及說明訊息

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 管理員可在 60 秒內找到並切換任一 guardrail 的啟用狀態（包含列表載入、搜尋定位、狀態切換全流程）
- **SC-002**: 管理員可在 3 分鐘內完成一條新 guardrail 的建立（包含選擇類型、填寫 config、設定 failure policy、儲存）
- **SC-003**: guardrail 測試功能在正常網路條件下，從提交到顯示結果不超過 10 秒
- **SC-004**: guardrail 列表在 50 條以上規則時，搜尋篩選結果在輸入後 1 秒內更新
- **SC-005**: 新管理員無需培訓即可完成 guardrail 的基本 CRUD 操作（基於 UI 引導的自助完成率 ≥ 90%）
- **SC-006**: 所有 guardrail CRUD 操作的成功/失敗狀態均有明確的視覺回饋，管理員不需要重新整理頁面才能確認操作結果

## Assumptions

- 管理後台已存在身份驗證機制，只有已登入的管理員才能訪問 guardrail 管理頁面（無需在此 feature 中新建 auth 邏輯）
- guardrail 類型清單（30+ 種）為固定清單，暫不需要從後端動態獲取，可在 UI 中靜態列出
- Policy binding 的新增操作假設 Policy 和 key/team 已存在於系統中，本 feature 不包含建立新 Policy 或 key/team 的功能
- 刪除一個被 Policy 參照的 guardrail 時，採用警告並拒絕刪除的策略（需先移除所有 binding 才能刪除）
- config JSON 的格式驗證只做基本 JSON 語法檢查，不做各 provider 特定的 schema 驗證（因各類型 schema 差異大）
- 測試功能針對停用狀態的 guardrail 仍可執行（測試基於當前 config，不受 enabled 狀態影響）
