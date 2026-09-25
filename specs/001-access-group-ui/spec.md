# Feature Specification: Access Group 管理 UI

**Feature Branch**: `001-access-group-ui`
**Created**: 2026-02-28
**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 查看 Access Group 列表 (Priority: P1)

管理員進入 Access Group 管理頁面，能夠一覽所有已設定的存取群組，了解每個群組的名稱（alias）、所屬組織、允許的模型數量與建立時間，並可透過搜尋快速定位特定群組。

**Why this priority**: 這是整個功能的入口。管理員需要先能看到所有 Access Group，才能進行後續的建立、編輯或刪除操作。沒有列表頁，其餘功能無從使用。

**Independent Test**: 部署後訪問 `/ui/access-groups`，可看到 Access Group 列表，包含群組名稱、組織、允許模型數量與建立時間；搜尋框輸入關鍵字後列表即時篩選。

**Acceptance Scenarios**:

1. **Given** 系統中已有多個 Access Group，**When** 管理員訪問 Access Group 管理頁面，**Then** 應顯示所有群組的 alias、所屬組織、允許模型數量與建立時間
2. **Given** Access Group 列表已顯示，**When** 管理員在搜尋框輸入關鍵字，**Then** 列表即時篩選出 alias 包含該關鍵字的群組
3. **Given** 系統中有超過預設每頁顯示數量的 Access Group，**When** 管理員到達列表底部，**Then** 可透過分頁控制瀏覽其他群組
4. **Given** 系統中尚無任何 Access Group，**When** 管理員訪問 Access Group 管理頁面，**Then** 應顯示空狀態提示，並提供新增群組的引導入口

---

### User Story 2 - 建立與管理 Access Group（CRUD）(Priority: P2)

管理員能夠建立新的 Access Group，填寫群組名稱（alias）、選擇所屬組織（可選）、設定初始允許存取的模型清單（可選）。同樣可以修改現有群組的設定，或刪除不再需要的群組。

**Why this priority**: 列表提供了可見性，但管理員必須能夠實際建立和維護 Access Group，才能讓存取控制機制發揮作用。CRUD 是本功能的核心操作。

**Independent Test**: 點擊「新增群組」，填寫 alias 後儲存，新群組出現在列表中；點擊「編輯」修改 alias 後儲存，列表即時更新；點擊「刪除」並確認，群組從列表移除。

**Acceptance Scenarios**:

1. **Given** 管理員在列表頁，**When** 點擊「新增 Access Group」，**Then** 顯示包含 alias（必填）、organization（選填）、models（選填）欄位的新增表單
2. **Given** 管理員填寫完整的新增表單，**When** 點擊儲存，**Then** 新 Access Group 建立成功並出現在列表，顯示成功提示
3. **Given** Access Group alias 與現有群組重複，**When** 嘗試建立，**Then** 顯示名稱已存在的錯誤提示，不執行新增，表單保留已輸入資料
4. **Given** 管理員提交必填欄位（alias）為空的表單，**When** 嘗試儲存，**Then** 顯示欄位必填的驗證錯誤，不執行儲存
5. **Given** 列表中已有 Access Group，**When** 管理員點擊某群組的「編輯」，**Then** 開啟預填現有資料的編輯表單
6. **Given** 管理員修改編輯表單並儲存，**When** 操作成功，**Then** 列表即時更新該群組的資料，顯示成功提示
7. **Given** 管理員點擊某群組的「刪除」，**When** 確認提示後點擊確認，**Then** 該群組從列表移除，顯示成功提示

---

### User Story 3 - 查看群組詳細資訊與管理允許模型 (Priority: P3)

管理員進入 Access Group 詳細頁面，可查看該群組完整的設定，並能動態新增或移除允許存取的模型，群組成員（被分配此 Access Group 的 API keys 和 Teams）也一目了然。

**Why this priority**: 詳細頁提供比列表更完整的管理能力，特別是允許模型的精細控制，是 CRUD 之後的深度管理功能。

**Independent Test**: 進入某 Access Group 的詳細頁，能看到允許模型清單與 key/team 成員清單，能成功新增一個模型至允許清單並即時反映，能成功移除清單中的模型。

**Acceptance Scenarios**:

1. **Given** 某 Access Group 已設定允許模型清單，**When** 管理員進入詳細頁，**Then** 應顯示所有允許的模型名稱，每個模型旁有移除按鈕
2. **Given** Access Group 的允許模型清單為空，**When** 管理員查看詳細頁，**Then** 顯示「允許所有模型（未設限）」的說明
3. **Given** 管理員在詳細頁選擇模型並點擊新增，**When** 操作成功，**Then** 模型立即出現在允許清單，無需完整頁面重載
4. **Given** 允許模型清單中有某模型，**When** 管理員點擊該模型旁的移除按鈕，**Then** 該模型從清單中立即移除
5. **Given** 某 Access Group 被若干 API keys 或 Teams 參照，**When** 管理員進入詳細頁，**Then** 顯示所有參照此群組的 key/team 清單（唯讀，供參考）

---

### Edge Cases

- 刪除一個仍被 API key 或 Team 參照的 Access Group 時，系統如何處理（警告並拒絕，還是強制刪除並解除關聯）？
- 允許模型清單為空（models = []）在後端的語意：代表「允許所有模型」還是「禁止所有模型」？
- 同一 Access Group 被多個管理員同時編輯時，後儲存者的變更是否直接覆蓋前者？

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系統 MUST 提供 Access Group 管理頁面，以列表形式顯示所有群組的 alias、所屬組織、允許模型數量與建立時間
- **FR-002**: 系統 MUST 支援依 alias 關鍵字搜尋 Access Group，搜尋結果在輸入後 500ms 內即時更新
- **FR-003**: 系統 MUST 支援 Access Group 列表分頁顯示，預設每頁 20 筆
- **FR-004**: 管理員 MUST 能夠建立新的 Access Group，alias 為必填欄位，organization 與初始 models 為選填
- **FR-005**: 系統 MUST 在 alias 重複時拒絕建立並顯示明確錯誤訊息
- **FR-006**: 系統 MUST 在 alias 為空時拒絕建立並顯示必填驗證提示
- **FR-007**: 管理員 MUST 能夠編輯現有 Access Group 的 alias、organization 欄位
- **FR-008**: 管理員 MUST 能夠刪除 Access Group，刪除前須顯示確認提示
- **FR-009**: 管理員 MUST 能夠在 Access Group 詳細頁查看完整的允許模型清單
- **FR-010**: 管理員 MUST 能夠在詳細頁新增模型至允許清單，操作後即時反映，無需頁面重載
- **FR-011**: 管理員 MUST 能夠在詳細頁移除允許清單中的模型，操作後即時反映，無需頁面重載
- **FR-012**: 管理員 MUST 能夠在詳細頁查看目前參照此群組的 API keys 和 Teams 清單（唯讀）
- **FR-013**: Access Group 管理 UI MUST 整合至現有管理後台的側邊導覽列，讓管理員能從其他頁面直接跳轉
- **FR-014**: 所有 CRUD 操作的成功或失敗均 MUST 以即時的視覺回饋（toast 通知）告知管理員，無需手動重新整理頁面

### Key Entities

- **Access Group**：存取控制群組，具有唯一識別碼（group_id）、顯示名稱（group_alias）、允許存取的模型清單（models[]）、所屬組織（organization_id），以及建立與更新的時間戳記與操作者資訊
- **允許模型清單（models[]）**：Access Group 中允許使用的模型名稱列表；空清單語意視為允許所有模型（與 Teams 功能慣例一致）
- **API Key / Team**：可被分配到特定 Access Group 的使用者憑證或團隊；分配關係由 key 或 team 端維護，Access Group 本身不直接儲存成員列表，詳細頁的成員清單為查詢反向參照的唯讀視圖

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 管理員可在 60 秒內找到並查看任一 Access Group 的詳細資訊（包含列表載入、搜尋定位、進入詳細頁全流程）
- **SC-002**: 管理員可在 2 分鐘內完成一個新 Access Group 的建立（包含填寫 alias、選擇允許模型、儲存）
- **SC-003**: 允許模型的新增與移除操作，從點擊到列表更新的回應時間不超過 3 秒（正常網路條件）
- **SC-004**: Access Group 列表在 50 筆以上資料時，搜尋篩選結果在輸入後 1 秒內更新
- **SC-005**: 新管理員無需培訓即可完成 Access Group 基本 CRUD 操作（基於 UI 引導的自助完成率 ≥ 90%）
- **SC-006**: 所有 CRUD 操作的成功或失敗均有明確的視覺回饋，管理員不需要重新整理頁面才能確認操作結果

## Assumptions

- 管理後台已存在身份驗證機制，只有已登入的管理員才能訪問 Access Group 管理頁面（無需在此功能中新建 auth 邏輯）
- 允許模型清單的可選項目從系統中已設定的可用模型動態取得（與 Teams/Keys 管理 UI 一致）
- 將 API key 或 Team 指定到某 Access Group 的操作，由各自的管理頁面（Keys/Teams UI）負責；本功能的詳細頁僅提供唯讀的反向查詢視圖
- 允許模型清單為空（models = []）代表允許所有模型，與 Teams 功能的慣例一致
- 刪除仍被 key/team 參照的 Access Group 時，採用警告並拒絕刪除策略（需先在 key/team 端解除關聯才能刪除）
- 組織列表（organizations）從系統已存在的組織動態取得，供建立/編輯時選擇
- 詳細頁中的 key/team 成員清單為唯讀，本功能不提供直接從 Access Group 詳細頁新增/移除 key/team 的操作
