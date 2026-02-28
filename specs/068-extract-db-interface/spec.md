# HO-68: Extract DB Interface for Handler Testability

**Parent**: HO-66 (CI/CD test coverage gate + coverage report)
**Status**: Draft
**Author**: 諸葛亮 (PM)
**Date**: 2026-02-28

---

## Problem

`internal/proxy/handler/Handlers.DB` 是 concrete type `*db.Queries`，無法 mock。Handler unit test 只能測 nil-DB early return，coverage 貢獻極低。

## Goal

抽出 `db.Store` interface，讓 handler tests 可用 mock DB 測完整邏輯，filtered coverage 從目前水準推到 ≥ 40%。

---

## Functional Requirements

### FR-1: DB Interface 定義

- 在 `internal/db/interface.go` 定義 `type Store interface { ... }`
- Interface 包含 handler 實際使用的 **154 個 DB methods**（已用 grep 驗證）
- 包含 2 個 extension methods：
  - `Pool() *pgxpool.Pool`（被 `analytics.go` 使用）
  - `Ping(ctx context.Context) error`（被 `health.go` 使用）
- Compile-time assertion：`var _ Store = (*Queries)(nil)`

### FR-2: Handler 結構改用 Interface

- `Handlers.DB` field type 從 `*db.Queries` 改為 `db.Store`
- 所有 handler 調用點不需改動（`*Queries` 已 satisfy interface）
- 零行為改變：`go build ./...` + `go test ./...` 全過

### FR-3: Mock 實作

- 新增 `internal/db/dbmock/store.go`
- 建議使用 `testify/mock` 或 `mockery` 自動生成
- Mock 需實作完整 `db.Store` interface

### FR-4: Handler Tests

以下 5 個 handler file 新增 mock-based tests，優先順序：

| File | Functions | 預估 Coverage +% |
|------|-----------|------------------|
| `key.go` | 7 | +1.5% |
| `team.go` | 6 | +1.0% |
| `spend.go` | 8 | +1.5% |
| `user.go` | 3 | +0.5% |
| `organization.go` | 7 | +1.0% |

每個 test 模式：mock DB return → call handler → assert HTTP status + response body。

### FR-5: CI Threshold 更新

- `.github/workflows/ci.yml` `COVERAGE_THRESHOLD` 從 `"30"` 改為 `"40"`

---

## Success Criteria

- [ ] **SC-1**: `Handlers.DB` type 是 `db.Store`（interface）
- [ ] **SC-2**: `var _ Store = (*Queries)(nil)` compile-time assertion 存在且通過
- [ ] **SC-3**: 現有 test 全部 PASS（零 regression）
- [ ] **SC-4**: 新增至少 5 個 handler file 的 mock-based test
- [ ] **SC-5**: Filtered coverage（排除 UI）≥ 40%
- [ ] **SC-6**: CI 全綠（lint + test + build + coverage gate）
- [ ] **SC-7**: `COVERAGE_THRESHOLD` 已改為 `"40"`

---

## Implementation Phases

### Phase 1: Extract Interface（不改行為）

1. 建立 `internal/db/interface.go`，定義 `Store` interface（154 sqlc methods + `Pool` + `Ping`）
2. 加 compile-time assertion
3. `Handlers.DB` type 改為 `db.Store`
4. 驗證：`go build ./...` && `go test ./...`

### Phase 2: Mock + Tests

1. 建立 `internal/db/dbmock/store.go`
2. 依優先順序補 handler tests：key → team → spend → user → organization
3. 每個 handler function 至少覆蓋 happy path + error path

### Phase 3: CI Threshold

1. 改 `COVERAGE_THRESHOLD` → `"40"`
2. 確認 CI 全綠

---

## Technical Notes

- 154 個 DB methods 是 sqlc 自動生成的，interface 很大但維護成本低（sqlc 更新 → 同步更新 interface 即可）
- 不需要拆 per-domain interface。先用大 interface，未來可按需拆分
- `Pool()` 回傳 `*pgxpool.Pool`，mock 時可回傳 nil（analytics test 需注意）
- `Ping()` 用在 health check，mock 直接回 nil/error 即可
- Issue 描述說 coverage 28.4%，CI threshold 實際是 30%（非 38%）。目標是拉到 40%

---

## Risks

| Risk | Mitigation |
|------|-----------|
| Interface 遺漏 method → compile error | grep 已驗證 154 個，加 compile-time assertion 防漏 |
| Mock 維護成本 | 用 mockery/moq 自動生成，sqlc 更新時重跑 |
| Coverage 40% 達不到 | Phase 2 的 5 個 file 預估 +5.5%，從 28.4% 到 ~34% 仍不夠，可能需要擴大 test 範圍到更多 handler files |

---

## Open Decisions

1. **Mock 工具選擇**：`testify/mock` vs `mockery` vs `moq` — 建議 `mockery`（自動生成，省手工）
2. **Coverage gap**：28.4% + 5.5% ≈ 34%，要到 40% 可能需要覆蓋更多 handler files（如 credentials.go, budget.go, analytics.go 等）。建議 Phase 2 結束後評估，必要時擴大範圍
