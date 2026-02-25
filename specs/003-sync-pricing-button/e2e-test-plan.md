# E2E Test Plan: Sync Model Pricing Button

**Branch**: `003-sync-pricing-button`  
**Date**: 2026-02-25  
**Author**: 魏徵 (QA)

---

## A. 環境準備

### A1. OrbStack K8s Port-Forward

```bash
# App (namespace: tianji)
kubectl port-forward -n tianji svc/tianji 4000:4000 &

# DB (namespace: postgresql)
kubectl port-forward -n postgresql svc/postgresql 5432:5432 &
```

### A2. DB 連線驗證

```bash
# 透過 port-forward 連線
psql "postgresql://postgres:23J%3Ax%3Fq6tAX5ckbHTzip@localhost:5432/tianji" \
  -c "SELECT 1 AS ok;"
```

### A3. 確認 Migration 011 已跑

```bash
psql "postgresql://postgres:23J%3Ax%3Fq6tAX5ckbHTzip@localhost:5432/tianji" \
  -c "SELECT column_name FROM information_schema.columns WHERE table_name='model_pricing' ORDER BY ordinal_position;"
```

預期欄位：`model_name`, `input_cost_per_token`, `output_cost_per_token`, `max_input_tokens`, `max_output_tokens`, `max_tokens`, `mode`, `provider`, `synced_at`, `source_url`

### A4. 清理測試資料（測試前）

```bash
psql "postgresql://postgres:23J%3Ax%3Fq6tAX5ckbHTzip@localhost:5432/tianji" \
  -c "DELETE FROM model_pricing;"
```

---

## B. 手動 E2E 測試案例

### E2E-001: Happy Path — Sync Pricing 成功

| 項目 | 內容 |
|------|------|
| **ID** | E2E-001 |
| **描述** | 正常 sync pricing，資料寫入 DB，Calculator 更新 |
| **前置條件** | App 啟動（DB 模式）、DB 連線正常、`model_pricing` 表為空或已知狀態 |

**步驟**：

1. 開啟瀏覽器 → `http://localhost:4000/ui/models`
2. 確認頁面有 **Sync Pricing** 按鈕
3. 點擊 **Sync Pricing**
4. 觀察按鈕變成 loading/disabled 狀態
5. 等待完成（≤60 秒）

```bash
# 或用 curl 測試 API
curl -X POST http://localhost:4000/ui/models/sync-pricing -v
```

6. 驗證 DB：

```bash
psql "postgresql://postgres:23J%3Ax%3Fq6tAX5ckbHTzip@localhost:5432/tianji" \
  -c "SELECT COUNT(*) FROM model_pricing;"
```

**預期結果**：
- 回應 HTTP 200，body 包含 "Synced" 及模型數量
- UI 顯示成功 toast（含 sync 數量）
- DB `model_pricing` 表有 ≥1000 筆資料
- 按鈕恢復可點擊狀態
- `synced_at` 為剛才的時間

---

### E2E-002: Upstream 不可達 — Error Toast

| 項目 | 內容 |
|------|------|
| **ID** | E2E-002 |
| **描述** | upstream URL 不可達時顯示錯誤，資料不受影響 |
| **前置條件** | App 啟動、先執行一次成功 sync（DB 有資料） |

**步驟**：

1. 記錄當前 DB 資料數量：

```bash
psql ... -c "SELECT COUNT(*) FROM model_pricing;"
```

2. 修改 app 環境變數（或重新部署）：

```bash
kubectl set env -n tianji deployment/tianji PRICING_UPSTREAM_URL=http://192.0.2.1:9999/nonexistent
kubectl rollout status -n tianji deployment/tianji
kubectl port-forward -n tianji svc/tianji 4000:4000 &
```

3. 點擊 Sync Pricing（或 curl）：

```bash
curl -X POST http://localhost:4000/ui/models/sync-pricing -v
```

4. 驗證 DB 資料未變：

```bash
psql ... -c "SELECT COUNT(*) FROM model_pricing;"
```

5. 恢復正確的 upstream URL：

```bash
kubectl set env -n tianji deployment/tianji PRICING_UPSTREAM_URL-
```

**預期結果**：
- 回應含錯誤訊息（timeout 或 connection refused）
- UI 顯示 error toast
- DB 資料筆數不變（transaction rollback）
- 35 秒內完成（含 30s timeout）

---

### E2E-003: Concurrent Sync — 409 Conflict

| 項目 | 內容 |
|------|------|
| **ID** | E2E-003 |
| **描述** | 同時發兩個 sync 請求，第二個被拒（409） |
| **前置條件** | App 啟動、DB 正常 |

**步驟**：

```bash
# 同時發兩個請求
curl -X POST http://localhost:4000/ui/models/sync-pricing -v &
curl -X POST http://localhost:4000/ui/models/sync-pricing -v &
wait
```

**預期結果**：
- 一個請求返回 200（成功 sync）
- 另一個返回 409 Conflict
- DB 資料一致（無 corruption）

---

### E2E-004: No-DB Mode — Button Disabled

| 項目 | 內容 |
|------|------|
| **ID** | E2E-004 |
| **描述** | 不配置 DB 時，Sync Pricing 按鈕不可用 |
| **前置條件** | 部署 app 不帶 DB 連線（移除 DATABASE_URL） |

**步驟**：

1. 部署無 DB 的 app：

```bash
kubectl set env -n tianji deployment/tianji DATABASE_URL-
kubectl rollout status -n tianji deployment/tianji
kubectl port-forward -n tianji svc/tianji 4000:4000 &
```

2. 開啟 `http://localhost:4000/ui/models`
3. 檢查 Sync Pricing 按鈕狀態

```bash
# API 驗證
curl -X POST http://localhost:4000/ui/models/sync-pricing -v
```

4. 恢復 DB 連線：

```bash
kubectl set env -n tianji deployment/tianji DATABASE_URL="postgresql://postgres:23J%3Ax%3Fq6tAX5ckbHTzip@postgresql.postgresql.svc.cluster.local:5432/tianji"
```

**預期結果**：
- UI：按鈕 hidden 或 disabled（帶 tooltip 說明需要 DB）
- API：回應包含 "Database not configured" 錯誤
- App 正常運作，無 panic/crash

---

### E2E-005: Restart Persistence — DB Pricing 自動載入

| 項目 | 內容 |
|------|------|
| **ID** | E2E-005 |
| **描述** | Sync 後重啟 app，pricing 從 DB 自動載入 |
| **前置條件** | 已成功執行一次 sync，DB 有資料 |

**步驟**：

1. 先 sync 並記錄某個 model 的 pricing：

```bash
psql ... -c "SELECT model_name, input_cost_per_token FROM model_pricing LIMIT 3;"
```

2. 重啟 app：

```bash
kubectl rollout restart -n tianji deployment/tianji
kubectl rollout status -n tianji deployment/tianji
kubectl port-forward -n tianji svc/tianji 4000:4000 &
```

3. 檢查 app log 確認從 DB 載入：

```bash
kubectl logs -n tianji deployment/tianji --tail=50 | grep -i "pricing\|reload\|loaded"
```

4. 驗證 pricing 計算仍使用 DB 資料（透過 UI 或 cost calculation API）

**預期結果**：
- App 啟動時 log 顯示從 DB 載入 pricing
- Calculator 使用 DB pricing（覆蓋 embedded data）
- 載入 ≤5 秒（NFR）

---

### E2E-006: Validation — Upstream < 50 Models 被拒

| 項目 | 內容 |
|------|------|
| **ID** | E2E-006 |
| **描述** | Upstream 回傳資料量太少時，sync 應被拒絕 |
| **前置條件** | 先做一次成功 sync（DB 有 ≥1000 筆） |

**步驟**：

1. 啟動一個 mock upstream（僅 10 筆資料）：

```bash
# 在本機跑一個簡單 HTTP server
python3 -c "
import json, http.server
data = {f'model-{i}': {'input_cost_per_token': 0.001, 'output_cost_per_token': 0.002, 'max_input_tokens': 4096, 'max_output_tokens': 2048, 'max_tokens': 6144, 'mode': 'chat', 'litellm_provider': 'test'} for i in range(10)}
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type','application/json')
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())
H.log_message = lambda *a: None
http.server.HTTPServer(('0.0.0.0', 9876), H).serve_forever()
" &
MOCK_PID=$!
```

2. 設定 upstream 指向 mock：

```bash
kubectl set env -n tianji deployment/tianji PRICING_UPSTREAM_URL=http://host.orb.internal:9876/
kubectl rollout status -n tianji deployment/tianji
kubectl port-forward -n tianji svc/tianji 4000:4000 &
```

3. 嘗試 sync：

```bash
curl -X POST http://localhost:4000/ui/models/sync-pricing -v
```

4. 驗證 DB 資料未被覆蓋：

```bash
psql ... -c "SELECT COUNT(*) FROM model_pricing;"
```

5. 清理：

```bash
kill $MOCK_PID
kubectl set env -n tianji deployment/tianji PRICING_UPSTREAM_URL-
```

**預期結果**：
- Sync 失敗，回應含 validation 錯誤（模型數量太少）
- DB 保持原有資料（≥1000 筆）
- 無 partial write

---

### E2E-007: Idempotent Sync — 重複 Sync 無副作用

| 項目 | 內容 |
|------|------|
| **ID** | E2E-007 |
| **描述** | 連續兩次 sync 結果一致，無重複資料 |
| **前置條件** | App 啟動、DB 正常 |

**步驟**：

```bash
curl -X POST http://localhost:4000/ui/models/sync-pricing
psql ... -c "SELECT COUNT(*) AS count1 FROM model_pricing;"

curl -X POST http://localhost:4000/ui/models/sync-pricing
psql ... -c "SELECT COUNT(*) AS count2 FROM model_pricing;"
```

**預期結果**：
- count1 = count2（upsert，無重複）
- 第二次 `synced_at` 更新

---

## C. 自動化建議

### C1. 現有 Integration Test 覆蓋分析

| 測試場景 | `sync_integration_test.go` | `handler_models_integration_test.go` | 缺口 |
|----------|:---:|:---:|------|
| Happy path sync → DB | ✅ `TestIntegration_SyncWritesToDB` | ✅ `TestIntegrationUI_SyncPricingSuccess` | — |
| Calculator 使用 DB 價格 | ✅ `TestIntegration_CalcLookupUsesDBAfterSync` | — | — |
| 重啟後從 DB 載入 | ✅ `TestIntegration_RestartReloadFromDB` | — | — |
| Sync 失敗 rollback | ✅ `TestIntegration_SyncFailureRollback` | — | — |
| Embedded fallback | ✅ `TestIntegration_EmbeddedFallbackWhenDBEmpty` | — | — |
| Concurrent 409 | — | ✅ `TestIntegrationUI_SyncPricingConcurrent409` | — |
| DB nil → disabled | — | ✅ `TestIntegrationUI_SyncPricingDBNil` | — |
| Race safety | — | ✅ `TestIntegrationUI_ConcurrentSafeRace` | — |
| **Validation (<50 models reject)** | — | — | ⚠️ **需補** |
| **Upstream timeout** | — | — | ⚠️ **需補** |
| **Idempotent upsert** | — | — | ⚠️ **需補** |
| **Malformed JSON** | — | — | ⚠️ **需補** |

### C2. 建議補充的 Integration Tests

#### 1. `TestIntegration_SyncRejectsFewerThan50Models`（in `sync_integration_test.go`）

```go
func TestIntegration_SyncRejectsFewerThan50Models(t *testing.T) {
    pool, queries := setupTestDB(t)
    srv := buildIntegrationUpstream(t, 10) // only 10 models
    calc := &Calculator{...}
    _, err := SyncFromUpstream(ctx, pool, queries, calc, srv.URL)
    if err == nil {
        t.Fatal("expected validation error for <50 models")
    }
}
```

#### 2. `TestIntegration_SyncTimeoutHandling`（in `sync_integration_test.go`）

Mock upstream 用 `time.Sleep` 延遲超過 timeout → 預期 error。

#### 3. `TestIntegration_SyncIdempotent`（in `sync_integration_test.go`）

兩次 sync 同樣資料 → row count 不變，`synced_at` 更新。

#### 4. `TestIntegration_SyncMalformedJSON`（in `sync_integration_test.go`）

Upstream 回傳 `{invalid` → 預期 parse error，DB 不變。

### C3. CI Integration Test 配置

```yaml
# .github/workflows/integration.yml
name: Integration Tests
on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: tianji
          POSTGRES_PASSWORD: tianji
          POSTGRES_DB: tianji_e2e
        ports:
          - 5433:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run migrations
        env:
          DATABASE_URL: postgres://tianji:tianji@localhost:5433/tianji_e2e
        run: |
          go run ./cmd/migrate up

      - name: Run integration tests
        env:
          TEST_DATABASE_URL: postgres://tianji:tianji@localhost:5433/tianji_e2e
        run: |
          go test -tags integration -race -v ./internal/pricing/ ./internal/ui/
```

**重點**：
- 用 GitHub Actions `services` 跑 PostgreSQL container
- 先跑 migration 確保 schema 存在
- `-tags integration` 啟用 integration test
- `-race` 偵測 data race
