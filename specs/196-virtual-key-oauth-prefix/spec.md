# Feature Specification: Virtual Key OAuth Prefix

**Feature Branch**: `196-virtual-key-oauth-prefix`
**Created**: 2026-03-25
**Status**: Draft
**Input**: User description: "feat: Virtual key 使用 sk-ant-oat prefix 以通過 OpenClaw OAuth 檢測"
**Linear Issue**: [HO-451](https://linear.app/hohsiang-lab/issue/HO-451)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - OpenClaw 透過 TianjiLLM 存取 Anthropic 模型 (Priority: P1)

OpenClaw 管理員設定 TianjiLLM 作為 Anthropic API 的代理端點。當 OpenClaw 發送請求時，需要根據 API key 前綴判斷是否使用 OAuth 模式。OpenClaw 的 `isAnthropicOAuthApiKey()` 檢查 key 是否包含 `sk-ant-oat`，若為 true 則：
- 注入 OAuth beta headers（`claude-code-20250219`, `oauth-2025-04-20`, `fine-grained-tool-streaming-2025-05-14`, `interleaved-thinking-2025-05-14`）
- 過濾不相容的 context-1m beta
- 跳過 fast mode service tier patching

注意：`x-anthropic-billing-header` 由底層的 `pi-ai` library 注入，不受 key prefix 控制。

目前 TianjiLLM 的 virtual key 使用 `sk-` 前綴，導致 OpenClaw 無法辨識為 OAuth token，進而不注入 OAuth beta headers，最終被 Anthropic 拒絕請求（403 或 400）。

**Why this priority**: 這是核心問題——沒有這個修改，OpenClaw 完全無法透過 TianjiLLM 存取 Anthropic 模型。

**Independent Test**: 產生一把新的 virtual key，驗證其格式為 `sk-ant-oat01-` 前綴且長度 >= 80 字元，然後透過 OpenClaw 發送 Anthropic 請求，確認 OpenClaw 注入了 OAuth beta headers（`oauth-2025-04-20` 等）。

**Acceptance Scenarios**:

1. **Given** TianjiLLM 已啟動, **When** 管理員透過 API 產生新的 virtual key, **Then** 產生的 key 以 `sk-ant-oat01-` 開頭且總長度 >= 80 字元
2. **Given** 一把新格式的 virtual key, **When** OpenClaw 的 `isAnthropicOAuthApiKey()` 判斷此 key, **Then** 回傳 `true`（因為 key 包含 `sk-ant-oat`）
3. **Given** 一把新格式的 virtual key, **When** OpenClaw 的 `validateAnthropicSetupToken()` 驗證此 key, **Then** 通過驗證（前綴為 `sk-ant-oat01-` 且長度 >= 80）

---

### User Story 2 - 透過 UI 產生新格式 Virtual Key (Priority: P1)

管理員透過 TianjiLLM Web UI 建立或重新產生 virtual key 時，產生的 key 必須同樣採用新的 `sk-ant-oat01-` 前綴格式。

**Why this priority**: UI 是管理員日常操作的主要介面，與 API 同等重要。

**Independent Test**: 在 UI 點擊「建立金鑰」，確認產生的 key 格式符合 `sk-ant-oat01-tianji-<hash>` 且長度 >= 80 字元。

**Acceptance Scenarios**:

1. **Given** 管理員在 UI 建立頁面, **When** 建立新的 virtual key, **Then** 顯示的 key 以 `sk-ant-oat01-` 開頭且長度 >= 80 字元
2. **Given** 管理員在 UI 重新產生 key, **When** 對既有 key 執行 regenerate, **Then** 新 key 同樣符合新格式

---

### User Story 3 - 既有 Virtual Key 持續運作 (Priority: P2)

已在使用中的舊格式 virtual key（`sk-` 前綴）不應因此變更而立即失效。既有 key 的認證和授權流程必須維持正常運作。

**Why this priority**: 確保升級不中斷現有服務，但優先級低於核心功能，因為可透過 regenerate 逐步遷移。

**Independent Test**: 使用一把舊格式的 `sk-` virtual key 發送請求至 TianjiLLM，確認認證通過並正常代理。

**Acceptance Scenarios**:

1. **Given** 一把既有的 `sk-` 前綴 virtual key, **When** 透過 TianjiLLM 發送請求, **Then** 認證正常通過，請求正常代理
2. **Given** 混合環境中同時存在新舊格式的 key, **When** 分別使用兩種格式發送請求, **Then** 兩者都能正常認證和代理

---

### Edge Cases

- 新 key 格式中的 `sk-ant-oat01-tianji-` 前綴是否可能與真正的 Anthropic OAuth token 混淆？答：不會。TianjiLLM 的認證是 SHA256 hash 比對，不會把 virtual key 直接轉發給 Anthropic——virtual key 僅用於 TianjiLLM 自身的認證，後端會使用配置中的真實 Anthropic API key 與上游通訊。
- 產生的 key 長度恰好等於 80 字元的邊界情況：必須確保 >= 80 字元的條件穩定滿足。
- Key regeneration 時 hash 碰撞：SHA256 碰撞機率極低，現有防護已足夠。
- API 與 UI 目前使用不同的 key 生成邏輯（UUID vs random bytes）：此次修改應統一為同一種生成方式。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系統 MUST 產生格式為 `sk-ant-oat01-tianji-<random>` 的 virtual key（含 `sk-ant-oat01-` 前綴和 `tianji-` 辨識段，以區別於真正的 Anthropic OAuth token）
- **FR-002**: 產生的 virtual key 總長度 MUST >= 80 字元
- **FR-003**: API 端點 (`/key/generate`) 和 UI 建立流程 MUST 使用相同的 key 生成邏輯
- **FR-004**: API 端點 (`/key/regenerate`) 和 UI 重新產生流程 MUST 使用相同的 key 生成邏輯
- **FR-005**: 既有的 `sk-` 前綴 virtual key MUST 繼續被認證中間件接受
- **FR-006**: Key 的隨機部分 MUST 使用密碼學安全的隨機數產生器

### Key Entities

- **Virtual Key**: 用戶用於存取 TianjiLLM 代理服務的憑證。格式為 `sk-ant-oat01-tianji-<random>`，總長度 >= 80 字元。資料庫儲存其 SHA256 hash。
- **Key Generation**: 產生 virtual key 的邏輯單元，被 API handler 和 UI handler 共用。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 新產生的每一把 virtual key 都以 `sk-ant-oat01-` 開頭且長度 >= 80 字元
- **SC-002**: OpenClaw 客戶端使用新格式 key 時，`isAnthropicOAuthApiKey()` 回傳 true，觸發 OAuth beta headers 注入（`oauth-2025-04-20`, `claude-code-20250219`）
- **SC-003**: 既有的 `sk-` 前綴 virtual key 在升級後仍可正常認證
- **SC-004**: API 和 UI 產生的 key 格式完全一致（前綴、長度、隨機性）

## Clarifications

### Session 2026-03-25

- Q: Virtual key 辨識段的精確格式？ → A: `tianji-`，完整格式為 `sk-ant-oat01-tianji-<random>`
- Q: Billing header 是否由 key prefix 控制？ → A: 否。`x-anthropic-billing-header` 由 `pi-ai` library 注入，不受 key prefix 控制。Key prefix 僅控制 OAuth beta headers 注入。（GitHub 源碼驗證 2026-03-25）

## Assumptions

- OpenClaw 的 `isAnthropicOAuthApiKey()` 判斷邏輯（`apiKey.includes("sk-ant-oat")`）和 `validateAnthropicSetupToken()` 驗證邏輯（`startsWith("sk-ant-oat01-")` + `length >= 80`）在可預見的未來不會改變。（已透過 GitHub 源碼驗證: `openclaw/openclaw` @ `anthropic-stream-wrappers.ts:58` 和 `provider-auth-token.ts:3-4`）
- `x-anthropic-billing-header` 由 `pi-ai` library（`oh-my-pi/packages/ai`）的 `createClaudeBillingHeader()` 注入，不受 API key prefix 控制。Linear issue HO-451 中 "OpenClaw 不注入 billing system prompt" 的描述不精確——billing header 與 key prefix 無關。
- TianjiLLM 的認證機制（SHA256 hash 比對）不受 key 前綴格式影響。
- 既有 key 的 migration 不在此次 scope 內——使用者可透過 regenerate 自行更新至新格式。
