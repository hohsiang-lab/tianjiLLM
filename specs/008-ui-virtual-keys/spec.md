# Feature Specification: UI Virtual Keys Management

**Feature Branch**: `008-ui-virtual-keys`
**Created**: 2026-02-20
**Status**: Draft
**Input**: User description: "目前 UI 上面沒有辦法管理 virtual keys"
**Reference**: 對齊 LiteLLM Python UI 的 virtual keys 管理功能

## Clarifications

### Session 2026-02-20

- Q: 詳情頁導航方式？ → A: 獨立 URL 路由（`/ui/keys/{id}`），整頁導航到詳情頁
- Q: 操作失敗時的錯誤回饋方式？ → A: Toast 通知（頁面角落彈出錯誤提示，幾秒後自動消失）
- Q: 過濾器如果現有 API 不完全支援，UI 第一版應如何處理？ → A: 先擴充 API 支援所有過濾參數，再實作 UI 過濾器（保證伺服器端過濾）

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 查看 Virtual Keys 列表（Priority: P1）

管理員登入後，在 Keys 頁面看到所有 virtual keys 的完整列表。列表預設按建立時間倒序排列。管理員可以透過多維度過濾器（Team ID、Key Alias、User ID、Key Hash）篩選 keys，支援伺服器端分頁和欄位排序。

**Why this priority**: 查看是所有管理操作的前提——不知道有哪些 key、什麼狀態，後續操作無從談起。

**Independent Test**: 登入管理面板，進入 Keys 頁面，能看到 key 列表含完整欄位，過濾能正確縮小結果，分頁和排序正常運作。

**Acceptance Scenarios**:

1. **Given** 管理員已登入且資料庫有 100 筆 keys，**When** 進入 Keys 頁面，**Then** 看到第一頁 50 筆 keys（按建立時間倒序），顯示以下欄位：Key ID、Key Alias、Secret Key（脫敏）、Team Alias、Team ID、User ID、Created At、Expires、Spend、Budget、Budget Reset、Models、Rate Limits（TPM/RPM）
2. **Given** 管理員在 Keys 頁面，**When** 使用 Key Alias 過濾器輸入關鍵字，**Then** 列表重新從伺服器查詢，只顯示別名匹配的 keys，分頁重置到第 1 頁
3. **Given** 管理員在 Keys 頁面，**When** 使用 Team ID 過濾器選擇特定 team，**Then** 列表只顯示該 team 的 keys
4. **Given** 有多頁 keys，**When** 點擊下一頁，**Then** 顯示下一頁的 keys，頁碼指示器顯示「Page N of M」和「Showing X - Y of Z results」
5. **Given** 管理員在 Keys 頁面，**When** 點擊 Spend 或 Budget 欄位標題，**Then** 列表按該欄位重新排序
6. **Given** 列表中的 key expires 為空，**When** 查看該行，**Then** Expires 欄顯示「Never」
7. **Given** 列表中的 key max_budget 為空，**When** 查看該行，**Then** Budget 欄顯示「Unlimited」
8. **Given** 列表中的 key models 超過 3 個，**When** 查看該行，**Then** 顯示前 3 個模型 + 折疊指示，可展開查看全部

---

### User Story 2 - 查看 Key 詳情（Priority: P1）

管理員點擊列表中的 Key ID，進入該 key 的詳情頁面。詳情頁分為「Overview」和「Settings」兩個 tab。Overview 以卡片形式展示花費、速率限制、模型等摘要資訊。Settings 以列表形式展示所有屬性的當前值。

**Why this priority**: 列表只顯示關鍵欄位，詳情頁是查看完整配置的入口，也是編輯和刪除的操作起點。

**Independent Test**: 在列表中點擊任一 Key ID，進入詳情頁，Overview 和 Settings 兩個 tab 都能正確顯示所有屬性。

**Acceptance Scenarios**:

1. **Given** 管理員在 Keys 列表頁，**When** 點擊某個 Key ID，**Then** 瀏覽器導航到獨立 URL（`/ui/keys/{id}`）的詳情頁，頁面頂部顯示 key alias（或「Virtual Key」）、Key ID（帶複製按鈕）、建立/更新時間
2. **Given** 管理員在 key 詳情頁，**When** 查看 Overview tab，**Then** 看到花費（含預算進度）、速率限制（TPM/RPM）、允許模型列表的摘要卡片
3. **Given** 管理員在 key 詳情頁，**When** 切換到 Settings tab，**Then** 看到所有屬性的當前值：Key ID、Key Alias、Secret Key（脫敏）、Team ID、Created、Expires、Spend、Budget、Tags、Models、Rate Limits、Metadata
4. **Given** 管理員在 key 詳情頁，**When** 點擊「Back to Keys」，**Then** 返回列表頁

---

### User Story 3 - 建立新 Virtual Key（Priority: P1）

管理員需要為使用者或服務建立 API key。點擊「Create New Key」打開建立對話框。表單分為必填區域和可選設定（折疊）。建立成功後，彈出獨立的對話框顯示一次性的完整 API key 明文值，附帶複製按鈕和安全警告。

**Why this priority**: 建立 key 是管理面板最核心的寫入操作。

**Independent Test**: 填寫建立表單、提交、在獨立對話框中看到並複製新 key 的明文值、確認 key 出現在列表中。

**Acceptance Scenarios**:

1. **Given** 管理員在 Keys 頁面，**When** 點擊「Create New Key」，**Then** 打開建立對話框，顯示以下必填欄位和可選設定

   **必填欄位**：
   - Key Alias（必填，作為 key 的可讀名稱）

   **可選設定**（折疊區域）：
   - Max Budget（數值，精度 0.01）
   - Budget Duration（預算重置週期：daily / weekly / monthly）
   - TPM Limit（正整數）
   - RPM Limit（正整數）
   - Models（多選）
   - Team ID（下拉選擇）
   - User ID（下拉選擇）
   - Duration（有效期限：30s/30m/30h/30d 格式，或留空永不過期）
   - Metadata（JSON 格式文字）
   - Tags（多選標籤）

2. **Given** 管理員填寫完表單並提交，**When** 建立成功，**Then** 關閉建立對話框，彈出「Save your Key」對話框，顯示安全警告（「此密鑰只顯示一次，無法再次查看」）和完整的 API key 明文值，附帶「Copy Virtual Key」按鈕
3. **Given**「Save your Key」對話框顯示中，**When** 點擊複製按鈕，**Then** key 值複製到剪貼簿，顯示複製成功提示
4. **Given** 管理員關閉「Save your Key」對話框，**When** 查看列表，**Then** 新 key 出現在列表中
5. **Given** 管理員在建立表單中，**When** 未填寫 Key Alias（必填），**Then** 表單不允許提交

---

### User Story 4 - 編輯 Virtual Key（Priority: P2）

管理員在 key 詳情頁的 Settings tab 點擊「Edit Settings」，進入編輯模式。表單預填當前所有屬性值，管理員修改後點擊「Save Changes」提交。

**Why this priority**: 隨著業務變化，key 的配置需要持續調整，但比建立和查看的優先級低。

**Independent Test**: 進入 key 詳情頁，切換到編輯模式，修改屬性，確認修改後的值正確顯示。

**Acceptance Scenarios**:

1. **Given** 管理員在 key 詳情頁 Settings tab，**When** 點擊「Edit Settings」，**Then** 切換為編輯表單，預填以下可編輯欄位：
   - Key Alias
   - Models（多選）
   - Max Budget
   - Budget Duration
   - TPM Limit / RPM Limit
   - Team ID
   - Tags
   - Metadata

2. **Given** 管理員在編輯模式中，**When** 修改 max_budget 並點擊「Save Changes」，**Then** 返回查看模式，顯示更新後的值
3. **Given** 管理員在編輯模式中，**When** 點擊「Cancel」，**Then** 放棄所有修改，返回查看模式

---

### User Story 5 - 封鎖/解封 Virtual Key（Priority: P2）

管理員發現某個 key 被濫用或需要暫停使用，需要快速封鎖它。封鎖操作可在列表頁直接執行（快捷操作）。解封同理。

**Why this priority**: 封鎖是緊急安全操作，需要快速存取。

**Independent Test**: 封鎖一個 key 後確認其狀態變更，解封後恢復正常。

**Acceptance Scenarios**:

1. **Given** 管理員在 Keys 列表看到一個未封鎖的 key，**When** 點擊該行的封鎖按鈕，**Then** key 狀態立即變為「已封鎖」，按鈕切換為「解封」
2. **Given** 管理員在 Keys 列表看到一個已封鎖的 key，**When** 點擊該行的解封按鈕，**Then** key 狀態恢復為正常

---

### User Story 6 - 刪除 Virtual Key（Priority: P3）

管理員需要永久刪除不再使用的 key。刪除操作在 key 詳情頁觸發。確認對話框顯示 key 的關鍵資訊（alias、Key ID、Team ID、Spend），且需要管理員輸入 key alias 才能確認刪除。

**Why this priority**: 刪除是不可逆操作，需要最嚴格的確認機制。

**Independent Test**: 刪除一個 key 後確認它從列表中消失。

**Acceptance Scenarios**:

1. **Given** 管理員在 key 詳情頁，**When** 點擊「Delete Key」，**Then** 彈出確認對話框，顯示不可逆警告、key 的關鍵資訊（Key Alias、Key ID、Team ID、Spend），以及一個確認輸入框
2. **Given** 確認對話框已顯示，**When** 管理員在輸入框中正確輸入 key alias，**Then** 刪除按鈕變為可點擊
3. **Given** 管理員已輸入正確的 key alias 並點擊刪除，**When** 操作成功，**Then** 返回列表頁，該 key 已移除
4. **Given** 確認對話框已顯示，**When** 管理員輸入的 alias 不匹配，**Then** 刪除按鈕保持禁用

---

### User Story 7 - 重新產生 Key（Priority: P3）

管理員需要輪換 key 的密鑰值。在詳情頁觸發重新產生操作，彈出對話框允許同時調整 max_budget、TPM/RPM limit 和 duration。成功後顯示新的明文 key 值。

**Why this priority**: 密鑰輪換是安全最佳實踐，但不如基本 CRUD 緊急。

**Independent Test**: 對一個 key 執行 regenerate，獲得新的明文 key 值，原有配置不變（除了可選的調整項）。

**Acceptance Scenarios**:

1. **Given** 管理員在 key 詳情頁，**When** 點擊「Regenerate Key」，**Then** 彈出對話框，預填 Key Alias（唯讀）、Max Budget、TPM Limit、RPM Limit、Duration（可修改），並顯示當前過期時間
2. **Given** 管理員在 regenerate 對話框中，**When** 修改 duration 值，**Then** 即時顯示計算後的新過期時間預覽
3. **Given** 管理員確認 regenerate，**When** 操作成功，**Then** 對話框切換為「Regenerated Key」模式，顯示安全警告和新的 API key 明文值，附帶複製按鈕
4. **Given** key 已重新產生，**When** 查看詳情頁，**Then** 顯示「Regenerated」標記，所有未修改的配置保持不變

---

### Edge Cases

- 搜尋/過濾不到任何 key 時，頁面顯示空狀態提示
- key 的 max_budget 設為 0 與設為空（無限制）的區別需清楚顯示——0 表示零預算，空表示 Unlimited
- key 已過期時，在列表和詳情頁都需要有明確的視覺提示
- 建立 key 時輸入非法值（負數預算、非正整數的 TPM/RPM）時，表單驗證應阻止提交
- 多個管理員同時操作同一個 key——後寫入覆蓋
- 一次性顯示的 key 明文值，使用者關閉對話框後無法再次取得
- models 列表為空表示允許所有模型，列表中需有明確標示（如「All Models」badge）
- key alias 在同 team 內應唯一，建立時需檢查重複
- 列表 Key ID 欄位是可點擊的連結，進入詳情頁
- budget_reset_at 欄位在沒有 budget_duration 時不顯示
- 操作失敗（網路錯誤、伺服器錯誤）時，以 Toast 通知顯示錯誤訊息，不阻塞頁面操作
- 詳情頁 URL 中的 key ID 不存在或已刪除時，顯示「Key not found」錯誤並提供返回列表的連結

## Requirements *(mandatory)*

### Functional Requirements

**列表頁**

- **FR-001**: 系統必須在 Keys 頁面以表格形式顯示所有 virtual keys，包含以下欄位：Key ID（可點擊）、Key Alias、Secret Key（脫敏）、Team Alias、Team ID、User ID、Created At、Expires、Spend（USD）、Budget（USD）、Budget Reset、Models、Rate Limits（TPM + RPM）
- **FR-002**: 列表預設按 Created At 倒序排列，支援對 Key ID、Key Alias、Created At、Updated At、Spend、Budget 欄位排序
- **FR-003**: 系統必須支援伺服器端分頁，每頁 50 筆，顯示當前頁碼、總頁數和結果範圍
- **FR-004**: 系統必須提供過濾器，支援按 Team ID、Key Alias、User ID、Key Hash 過濾，過濾時重置到第 1 頁
- **FR-005**: 列表中 Expires 為空時顯示「Never」，Budget 為空時顯示「Unlimited」
- **FR-006**: 列表中 Models 超過 3 個時折疊顯示，可展開查看全部；Models 為空時顯示「All Models」標示
- **FR-007**: 已封鎖的 key 必須在列表中有明確的視覺區分
- **FR-008**: 已過期的 key 必須在列表中有明確的視覺區分
- **FR-009**: 空列表狀態必須顯示友好提示文字
- **FR-010**: 列表每行提供封鎖/解封快捷操作按鈕

**詳情頁**

- **FR-011**: 點擊列表中的 Key ID 必須導航到獨立 URL（`/ui/keys/{id}`）的詳情頁，頁面頂部顯示 key alias（或「Virtual Key」）、Key ID（帶一鍵複製）、時間戳
- **FR-012**: 詳情頁必須包含 Overview tab，以卡片形式展示：花費/預算、速率限制、允許模型
- **FR-013**: 詳情頁必須包含 Settings tab，以 key-value 形式展示所有屬性的當前值
- **FR-014**: 詳情頁必須提供「Back to Keys」返回列表的導航

**建立**

- **FR-015**: 系統必須提供「Create New Key」按鈕，打開建立對話框
- **FR-016**: 建立表單必須包含：Key Alias（必填）以及可折疊的可選設定區域（Max Budget、Budget Duration、TPM Limit、RPM Limit、Models、Team ID、User ID、Duration、Metadata、Tags）
- **FR-017**: Key Alias 在同 team 內必須唯一，建立時需驗證
- **FR-018**: 建立成功後必須彈出獨立的「Save your Key」對話框，顯示安全警告和完整 API key 明文值，附帶一鍵複製按鈕
- **FR-019**: 複製成功後必須顯示確認提示

**編輯**

- **FR-020**: 詳情頁 Settings tab 必須提供「Edit Settings」按鈕，切換為編輯表單
- **FR-021**: 編輯表單必須預填所有當前屬性值，可修改：Key Alias、Models、Max Budget、Budget Duration、TPM/RPM Limit、Team ID、Tags、Metadata
- **FR-022**: 編輯表單必須提供「Save Changes」和「Cancel」按鈕

**封鎖/解封**

- **FR-023**: 系統必須支援從列表頁直接封鎖/解封 key，操作後立即更新該行狀態

**刪除**

- **FR-024**: 詳情頁必須提供「Delete Key」按鈕
- **FR-025**: 刪除確認對話框必須顯示不可逆警告、key 的關鍵資訊（Key Alias、Key ID、Team ID、Spend）
- **FR-026**: 刪除確認必須要求管理員輸入 key alias 才能解鎖刪除按鈕
- **FR-027**: 刪除成功後自動返回列表頁

**重新產生**

- **FR-028**: 詳情頁必須提供「Regenerate Key」按鈕
- **FR-029**: Regenerate 對話框必須預填 Key Alias（唯讀）、Max Budget、TPM/RPM Limit、Duration（可修改），顯示當前過期時間
- **FR-030**: 修改 Duration 時必須即時顯示新過期時間預覽
- **FR-031**: Regenerate 成功後必須顯示新的 API key 明文值，附帶複製按鈕

**表單驗證**

- **FR-032**: 所有表單中 Max Budget 不可為負數、TPM/RPM 必須為正整數或留空
- **FR-033**: Duration 欄位格式必須為數字 + 時間單位（s/m/h/d），或留空

**錯誤回饋**

- **FR-034**: 所有伺服器端操作失敗（建立、編輯、刪除、封鎖/解封、重新產生）時，必須以 Toast 通知顯示錯誤訊息
- **FR-035**: Toast 通知顯示在頁面角落，包含操作名稱和錯誤原因，幾秒後自動消失
- **FR-036**: 操作成功時亦以 Toast 通知確認（建立成功除外——建立成功使用獨立的「Save your Key」對話框）

**API 前置條件**

- **FR-037**: `/key/list` API 必須先擴充支援 team_id、key_alias、user_id、key_hash 過濾參數以及 page、size 分頁參數和 sort_by、sort_order 排序參數，作為 UI 過濾器的前置工作

### Key Entities

- **Virtual Key**：代表一個 API 存取金鑰。核心屬性：名稱（alias）、預算上限及重置週期、花費記錄、允許使用的模型列表、所屬 team、所屬 user、速率限制（TPM/RPM）、有效期限、封鎖狀態、標籤、metadata。系統以 token hash 作為唯一識別，明文 key 只在建立和重新產生時顯示一次。
- **Team**：key 可以關聯的組織單位，一個 team 下可以有多個 keys，team 有自己的預算和速率限制上限。
- **User**：key 可以關聯的使用者，一個 user 下可以有多個 keys。

## Assumptions

- 只有 master key 持有者（管理員）可以存取 UI 管理 virtual keys（現有認證機制已支持）
- key 的唯一識別由系統內部 token hash 決定
- 編輯 key 時不能修改 token 本身（token rotation 是獨立的 regenerate 操作）
- 建立 key 的表單欄位與現有 REST API `/key/generate` 的參數對齊
- 過濾和分頁在伺服器端執行（對齊 LiteLLM Python 的實作方式）
- Budget Duration 選項：daily、weekly、monthly
- Duration 使用自由格式文字輸入（30s/30m/30h/30d），不使用下拉選單（對齊 LiteLLM）
- 多管理員同時操作時，採用「後寫入覆蓋」策略
- 不實作 LiteLLM Premium 專屬功能（Guardrails、Policies、Prompts、MCP Settings、Agent Settings、Logging Settings、Auto-Rotation、Pass Through Routes）——這些留待後續 feature
- 不實作 Service Account 建立流程——留待後續 feature
- 不實作批量操作——LiteLLM Python UI 也尚未實作

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 管理員能在 30 秒內完成建立一個帶完整配置的 virtual key
- **SC-002**: 管理員能在 10 秒內透過過濾器找到目標 key 並進入詳情頁
- **SC-003**: 所有 key 管理操作（建立、查看詳情、編輯、封鎖/解封、刪除、重新產生）可在 UI 上完成，無需使用 REST API
- **SC-004**: key 列表頁面載入和分頁切換在 key 數量少於 1000 時不超過 2 秒
- **SC-005**: UI 的 key 管理功能與 LiteLLM Python UI 的核心功能對齊（列表欄位、過濾器、CRUD、regenerate、詳情頁）
