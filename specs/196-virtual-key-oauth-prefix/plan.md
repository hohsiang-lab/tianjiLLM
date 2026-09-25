# Implementation Plan: Virtual Key OAuth Prefix

**Branch**: `196-virtual-key-oauth-prefix` | **Date**: 2026-03-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/196-virtual-key-oauth-prefix/spec.md`

## Summary

TianjiLLM virtual keys 目前使用 `sk-<uuid>` 或 `sk-<hex>` 格式，無法被 OpenClaw 的 `isAnthropicOAuthApiKey()` 辨識為 OAuth token，導致 OpenClaw 不注入 OAuth beta headers（`oauth-2025-04-20`, `claude-code-20250219`），Anthropic 因此拒絕請求。需要將 key 格式改為 `sk-ant-oat01-tianji-<60 hex chars>`（總長度 80），統一 API 和 UI 的生成邏輯，並更新相關 E2E 測試和 UI 遮罩顯示。

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: chi/v5, pgx/v5, crypto/rand (stdlib), encoding/hex (stdlib)
**Storage**: PostgreSQL (SHA256 hash of key stored in `verification_tokens.token` column)
**Testing**: go test + testify
**Target Platform**: Linux server (Kubernetes)
**Project Type**: web (Go backend + templ/HTMX UI)
**Performance Goals**: N/A — key generation is a low-frequency admin operation
**Constraints**: 產生的 key 必須通過 OpenClaw 的兩個檢查: `includes("sk-ant-oat")` 和 `startsWith("sk-ant-oat01-") && length >= 80`
**Scale/Scope**: 5 個 production 檔案修改 + 4 個 test 檔案修改/新增

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | 此功能為 TianjiLLM Go 專屬需求，Python 版無對應 |
| II. Feature Parity | N/A | 新功能，不涉及 Python 行為複製 |
| III. Research Before Build | PASS | 僅使用 stdlib（crypto/rand, encoding/hex），無需外部研究 |
| IV. Failing-Tests-First | PASS | 見下方 Failing Tests section |
| V. Go Best Practices | PASS | 統一重複邏輯到共用函式，符合 DRY |
| VI. No Stale Knowledge | PASS | 僅使用 Go stdlib，無版本相依問題 |
| VII. sqlc-First DB Access | N/A | 不涉及新的 DB query |

## Project Structure

### Documentation (this feature)

```text
specs/196-virtual-key-oauth-prefix/
├── plan.md              # This file
├── research.md          # Phase 0 output (minimal — stdlib only)
├── spec.md              # Feature specification
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
internal/
├── proxy/handler/
│   ├── key.go           # MODIFY: add generateVirtualKey(), update KeyGenerateHandler()
│   ├── key_ext.go       # MODIFY: use generateVirtualKey() in KeyRegenerate()
│   └── key_test.go      # NEW: unit tests for generateVirtualKey()
├── ui/
│   ├── handler_keys.go      # MODIFY: update generateAPIKey() to new format
│   ├── handler_keys_test.go  # NEW: unit tests for UI generateAPIKey()
│   ├── handler_models.go     # MODIFY: update maskAPIKey() display format
│   └── handler_models_test.go # NEW: unit tests for maskAPIKey()
└── provider/anthropic/
    └── oauth_test.go    # MODIFY: add test case for tianji-prefixed key

test/
├── contract/
│   └── key_management_test.go  # ADD: key format validation tests
└── e2e/
    ├── helpers_test.go         # MODIFY: update generateTestKey() prefix
    ├── keys_create_test.go     # MODIFY: update key format assertions
    └── keys_regen_test.go      # MODIFY: update key format assertions
```

**Structure Decision**: 在 `internal/proxy/handler/key.go` 新增 `generateVirtualKey()` 函式，`key_ext.go` 的 `KeyRegenerate()` 呼叫它。`internal/ui/handler_keys.go` 的 `generateAPIKey()` 各自更新為相同格式 `sk-ant-oat01-tianji-<60 hex chars>`。兩個 package 各自維護生成函式（與既有的 `hashKey()` 重複模式一致），這是 Go package 隔離的正常代價。不建新檔案。

## Failing Tests

### User Story 1 Tests — API 產生新格式 Virtual Key

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestGenerateAPIKey_HasOAuthPrefix` | `internal/proxy/handler/key_test.go` | `strings.HasPrefix(key, "sk-ant-oat01-tianji-")` == true | AS-1.1 |
| `TestGenerateAPIKey_MinLength80` | `internal/proxy/handler/key_test.go` | `len(key) >= 80` | AS-1.1, FR-002 |
| `TestGenerateAPIKey_PassesOAuthCheck` | `internal/proxy/handler/key_test.go` | `anthropic.IsOAuthToken(key)` == true | AS-1.2 |
| `TestGenerateAPIKey_CryptoRandom` | `internal/proxy/handler/key_test.go` | 連續呼叫 1000 次無重複 key | FR-007 |
| `TestGenerateAPIKey_ExactLength80` | `internal/proxy/handler/key_test.go` | `len(key) == 80`（prefix 20 + hex 60） | Edge: 長度邊界 |

### User Story 2 Tests — UI 產生新格式 Virtual Key

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestUIGenerateAPIKey_HasOAuthPrefix` | `internal/ui/handler_keys_test.go` | `strings.HasPrefix(key, "sk-ant-oat01-tianji-")` == true | AS-2.1 |
| `TestUIGenerateAPIKey_MinLength80` | `internal/ui/handler_keys_test.go` | `len(key) >= 80` | AS-2.1 |
| `TestUIGenerateAPIKey_MatchesAPIFormat` | `internal/ui/handler_keys_test.go` | UI 和 API 產生的 key 格式 regex 相同 | SC-004 |

### User Story 3 Tests — 舊 Key 向後相容

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestIsOAuthToken_TianjiVirtualKey` | `internal/provider/anthropic/oauth_test.go` | `IsOAuthToken("sk-ant-oat01-tianji-abc...")` == true | AS-1.2 |
| `TestAuthMiddleware_OldFormatKey` | `test/contract/key_management_test.go` | 舊格式 `sk-<uuid>` key 仍然通過認證 | AS-3.1 |
| `TestAuthMiddleware_NewFormatKey` | `test/contract/key_management_test.go` | 新格式 `sk-ant-oat01-tianji-<hex>` key 通過認證 | AS-3.2 |

### Edge Case Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestGenerateAPIKey_HexOnly` | `internal/proxy/handler/key_test.go` | random 部分只包含 `[0-9a-f]` 字元 | Edge: 格式正確性 |

### E2E Tests — 既有測試更新

| Test / Helper | File | 改動 | 原因 |
|---------------|------|------|------|
| `generateTestKey()` | `test/e2e/helpers_test.go:816` | 前綴改為 `sk-ant-oat01-tianji-`，隨機部分 30 bytes | E2E helper 需要產生新格式 key |
| key create assertion | `test/e2e/keys_create_test.go:34` | `assert.Contains` → `assert.True(HasPrefix(..., "sk-ant-oat01-tianji-"))` + 長度檢查 | 精確驗證新格式 |
| key regen assertion | `test/e2e/keys_regen_test.go:29,88` | 同上 | 精確驗證新格式 |

### UI Display Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestMaskAPIKey_NewFormat` | `internal/ui/handler_models_test.go` | `maskAPIKey("sk-ant-oat01-tianji-abc...xyz")` 顯示前幾字元 + `...` + 後4字元 | maskAPIKey 改動 |
| `TestMaskAPIKey_OldFormat` | `internal/ui/handler_models_test.go` | `maskAPIKey("sk-abc123")` 向後相容顯示 | 舊格式不壞 |

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./internal/proxy/handler/... -run "TestGenerateAPIKey_|TestAuthMiddleware_" -v
go test ./internal/ui/... -run "TestUIGenerateAPIKey_|TestMaskAPIKey_" -v
go test ./internal/provider/anthropic/... -run "TestIsOAuthToken_TianjiVirtualKey" -v
go test ./test/contract/... -run "TestAuthMiddleware_" -v

# After implementation, run full suite including E2E:
make test
# E2E (requires PostgreSQL):
# make e2e
```

## Complexity Tracking

No constitution violations — no complexity justification needed.

## Implementation Notes

### Key Format Calculation

```
Prefix:  sk-ant-oat01-tianji-  (20 chars)
Random:  30 bytes → hex.EncodeToString → 60 chars
Total:   80 chars (exactly meets >= 80 requirement)
```

### Changes Summary

**Production code (4 files)**:

1. **`internal/proxy/handler/key.go`**: 新增 `generateVirtualKey()` 函式，line 47 改為呼叫它
2. **`internal/proxy/handler/key_ext.go`** line 32: `"sk-" + uuid.New().String()` → `generateVirtualKey()`
3. **`internal/ui/handler_keys.go`** line 713-717: 修改 `generateAPIKey()` 為相同格式
4. **`internal/ui/handler_models.go`** line 395-402: 更新 `maskAPIKey()` 顯示格式，改為顯示前幾個字元 + `...` + 後4字元，而非硬編碼 `"sk-..."`
5. **可移除 `uuid` import**: `key.go` 和 `key_ext.go` 僅在 key 生成處使用 uuid，確認可安全移除

**Test code (4 files)**:

6. **`internal/provider/anthropic/oauth_test.go`**: 新增 tianji virtual key 測試案例
7. **`test/e2e/helpers_test.go`** line 816: `generateTestKey()` 前綴從 `"sk-"` 更新為 `"sk-ant-oat01-tianji-"`，隨機部分改為 30 bytes
8. **`test/e2e/keys_create_test.go`** line 34: `assert.Contains` 斷言更新為 `assert.True(strings.HasPrefix(rawKey, "sk-ant-oat01-tianji-"))` 並驗證長度 >= 80
9. **`test/e2e/keys_regen_test.go`** line 29, 88: 同上，更新為精確的前綴和長度斷言

### What Does NOT Change

- 認證中間件 (`auth.go`): SHA256 hash 比對不受 key 前綴影響，零改動
- 資料庫 schema: 只存 hash，不受影響
- 既有 key: 已存在的 `sk-` 前綴 key 的 hash 仍在 DB 中，認證照常
- 既有 OAuth 測試: `oauth_test.go`、`native_upstream_test.go`、`handler_ratelimit_seed_test.go`、`anthropic_messages_test.go` 使用真正的 OAuth token 格式（`sk-ant-oat01-abc123`），不是 virtual key，不受影響
- 整合測試 `test/integration/virtual_key_auth_test.go`: 使用硬編碼測試 key（`sk-virtual-valid` 等）搭配 `memValidator`，只做 hash 比對，不依賴前綴格式
- Master key 測試: 所有 `"sk-master"` 測試值與 virtual key 格式無關
- `ServiceAccountKeyGenerate()`: 委託給 `KeyGenerateHandler()`，不自己生成 key，自動覆蓋

### IsOAuthToken() 副作用分析

新格式 `sk-ant-oat01-tianji-<hex>` 會讓 `anthropic.IsOAuthToken()` 回傳 `true`（因為 `strings.HasPrefix(key, "sk-ant-oat")`）。Codebase 中有 11 個 callsite 依賴此函式：

| Caller | File | 影響評估 |
|--------|------|----------|
| `SetupHeaders()` | `anthropic/anthropic.go:106` | 不影響 — 處理 upstream API key，非 virtual key |
| `setProviderAuth()` | `passthrough/router.go:149` | 不影響 — 同上 |
| `Director` | `passthrough/handler.go:59` | 不影響 — 同上 |
| `nativeProxy()` Director | `handler/native_format.go:86` | 不影響 — 同上 |
| `nativeProxy()` ModifyResponse | `handler/native_format.go:138,154` | 不影響 — Discord alerter 僅判斷 upstream key |
| `selectUpstreamWithThrottle()` | `handler/native_upstream.go:121` | 不影響 — throttle 邏輯處理 upstream deployment key |
| `buildRateLimitWidgetData()` | `ui/handler_ratelimit.go:79,140` | 不影響 — 處理配置中的 upstream key |

**結論**: 所有 callsite 處理的是 **upstream provider API key**（配置在 `proxy_config.yaml` 中的真實 Anthropic key），不是 client 提供的 virtual key。認證中間件在 `auth.go` 層就已經把 virtual key 解析為 DB token 並設入 context，後續 provider 層使用的是配置中的 upstream key。兩條代碼路徑完全隔離，不存在交叉風險。

**邊界假設**: 沒有人會把 `sk-ant-oat01-tianji-*` 格式的 key 配置在 `proxy_config.yaml` 的 `api_key` 欄位作為 upstream key。這不是合理的使用場景。

### OpenClaw 源碼驗證（GitHub 交叉比對 2026-03-25）

透過 GitHub 搜索 `openclaw/openclaw` 源碼，驗證 plan 中所有 OpenClaw 假設：

| 假設 | 源碼位置 | 驗證結果 |
|------|---------|---------|
| `isAnthropicOAuthApiKey()` 使用 `includes("sk-ant-oat")` | `src/agents/pi-embedded-runner/anthropic-stream-wrappers.ts:58` | 完全吻合 |
| `validateAnthropicSetupToken()` 使用 `startsWith("sk-ant-oat01-")` + `length >= 80` | `src/plugins/provider-auth-token.ts:3-4,26-38` | 完全吻合 |
| OAuth 檢測觸發 `PI_AI_OAUTH_ANTHROPIC_BETAS` 注入 | `src/agents/pi-embedded-runner/anthropic-stream-wrappers.ts:20-24` | 完全吻合 |
| OAuth 檢測觸發 billing header 注入 | OpenClaw 源碼中無 billing header 邏輯 | **不正確** |

**Billing header 修正**: `x-anthropic-billing-header` 由 `pi-ai` library（`oh-my-pi/packages/ai/src/providers/anthropic.ts`）的 `createClaudeBillingHeader()` 注入，當 `includeClaudeCodeInstruction` 為 true 時觸發。此行為不受 API key prefix 控制，與本次改動無關。Linear issue HO-451 中的相關描述不精確。

**OAuth 檢測觸發的實際行為**（已驗證）:
1. Beta headers: 使用 `PI_AI_OAUTH_ANTHROPIC_BETAS`（含 `claude-code-20250219`, `oauth-2025-04-20`）而非 `PI_AI_DEFAULT_ANTHROPIC_BETAS`
2. Context-1m 過濾: OAuth 模式下自動移除 `context-1m-2025-08-07` beta（Anthropic 不支援 OAuth + context-1m）
3. Fast mode bypass: OAuth key 跳過 `createAnthropicFastModeWrapper` 的 service tier patching
