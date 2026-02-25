# Data Model: Sync Model Pricing

## Migration: `011_model_pricing.up.sql`

```sql
CREATE TABLE IF NOT EXISTS "ModelPricing" (
    model_name          TEXT PRIMARY KEY,
    input_cost_per_token  DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_cost_per_token DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_input_tokens    INTEGER NOT NULL DEFAULT 0,
    max_output_tokens   INTEGER NOT NULL DEFAULT 0,
    max_tokens          INTEGER NOT NULL DEFAULT 0,
    mode                TEXT NOT NULL DEFAULT '',
    provider            TEXT NOT NULL DEFAULT '',
    source_url          TEXT NOT NULL DEFAULT '',
    synced_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_provider ON "ModelPricing" (provider);
CREATE INDEX IF NOT EXISTS idx_model_pricing_synced_at ON "ModelPricing" (synced_at);
```

## 欄位說明

| 欄位 | 型別 | 說明 |
|------|------|------|
| `model_name` | TEXT PK | LiteLLM model 名稱，如 `gpt-4`, `claude-3-opus` |
| `input_cost_per_token` | DOUBLE PRECISION | 每 input token 成本 (USD) |
| `output_cost_per_token` | DOUBLE PRECISION | 每 output token 成本 (USD) |
| `max_input_tokens` | INTEGER | 最大 input token 數 |
| `max_output_tokens` | INTEGER | 最大 output token 數 |
| `max_tokens` | INTEGER | 最大總 token 數 |
| `mode` | TEXT | 模型模式：chat, embedding, completion 等 |
| `provider` | TEXT | LiteLLM provider 名稱 |
| `source_url` | TEXT | 資料來源 URL |
| `synced_at` | TIMESTAMPTZ | 最後一次 sync 的時間 |
| `created_at` | TIMESTAMPTZ | 首次建立時間 |
| `updated_at` | TIMESTAMPTZ | 最後更新時間 |

## 設計決策

1. **不使用 `created_by`/`updated_by`**: 這是系統自動同步，非使用者操作，省略 actor 欄位
2. **`synced_at` vs `updated_at`**: `synced_at` 記錄 upstream sync 時間，`updated_at` 記錄 row 更新時間。多次 sync 同樣資料時 `updated_at` 會變但 `synced_at` 也會變（代表重新確認）
3. **DOUBLE PRECISION**: 與現有 `BudgetTable` 保持一致（不用 NUMERIC），且與 Go `float64` 直接對應
4. **無 FK**: 此表獨立，不與其他表關聯
5. **Provider index**: 未來可能需要按 provider 查詢（P2 scope）
