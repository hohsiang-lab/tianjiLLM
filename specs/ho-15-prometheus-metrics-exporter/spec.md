# HO-15: Add Prometheus Metrics Exporter

## Status
Draft

## Problem Statement

tianjiLLM 已有 `PrometheusCallback`（`internal/callback/prometheus.go`）收集 metrics，但：

1. **沒有 `/metrics` endpoint** — `setupRoutes()` 未註冊任何 Prometheus scrape 路由，外部 Prometheus 無法抓取
2. **缺少 `api_key` label** — 無法做 per-key 用量分析（計費、quota 監控）
3. **Histogram bucket 未針對 LLM 調整** — 預設 `DefBuckets`（5ms–10s）對 LLM 回應（常見 1–60s）覆蓋不足
4. **Error 分類不夠細** — `LogFailure` 只區分 `error` / `500`，缺少 HTTP status code、error type

## Goal

讓 Prometheus 可以 scrape tianjiLLM，取得 per-model、per-key 的請求數、延遲、token 用量、錯誤率。

## Non-Goals

- Grafana dashboard provisioning（另案處理）
- Push gateway 支援
- Custom metric registration API

---

## Design

### 1. `/metrics` Endpoint Routing

在 `internal/proxy/server.go` `setupRoutes()` 中，**與 `/health` 同層**註冊：

```go
// Prometheus metrics (no auth by default)
r.Handle("/metrics", callback.Handler())
```

**位置**：`/health` route group 之後、LLM routes 之前。
**Auth**：預設無 auth（與 `/health` 一致）。可選透過 config 啟用 basic auth（見下方）。

### 2. Optional Basic Auth Protection

新增 config 欄位：

```yaml
general_settings:
  metrics_auth:
    enabled: false
    username: "prometheus"
    password: "changeme"
```

對應 `internal/config/config.go`：

```go
type MetricsAuth struct {
    Enabled  bool   `yaml:"enabled"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
}
```

當 `enabled: true` 時，wrap `/metrics` handler 加 basic auth middleware。

### 3. Metric Names & Labels

所有 metric 遵循 [Prometheus naming conventions](https://prometheus.io/docs/practices/naming/)：`namespace_subsystem_name_unit`。

#### 3.1 Request Counter（改良現有）

```
tianji_requests_total{model, provider, api_key, status}
```

- `model`：requested model name（e.g. `gpt-4o`）
- `provider`：backend provider（e.g. `openai`）
- `api_key`：token hash 前 8 碼（避免洩漏完整 key）
- `status`：`success` | HTTP status code string（`400`, `429`, `500`, `502`, etc.）

#### 3.2 Latency Histograms（改良現有）

**Total latency**（含 proxy overhead）：
```
tianji_request_total_latency_seconds{model, provider, api_key}
```

**LLM API latency**（純 upstream 呼叫時間）：
```
tianji_llm_api_latency_seconds{model, provider, api_key}
```

**Time to first token**（streaming）：
```
tianji_time_to_first_token_seconds{model, provider, api_key}
```

**Bucket 設計**（針對 LLM workload）：

```go
var LLMLatencyBuckets = []float64{
    0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0,
}
```

理由：
- `0.05–0.5s`：cache hit、embedding、短 prompt
- `1.0–5.0s`：典型 chat completion
- `10–30s`：長 context、complex reasoning
- `60–120s`：batch / long-running generation

#### 3.3 Token Usage Counter（改良現有）

```
tianji_tokens_total{model, provider, api_key, type}
```

- `type`：`prompt` | `completion`

#### 3.4 Cost Counter（保留現有）

```
tianji_spend_total{model, provider}
```

不加 `api_key`（cost 是 provider 側概念，key 粒度用 token counter 推算即可）。

#### 3.5 Error Counter（新增）

```
tianji_errors_total{model, provider, api_key, error_type}
```

- `error_type`：`timeout` | `rate_limited` | `auth_error` | `upstream_error` | `invalid_request` | `unknown`

分類邏輯：
- HTTP 408 / context deadline → `timeout`
- HTTP 429 → `rate_limited`
- HTTP 401/403 → `auth_error`
- HTTP 5xx → `upstream_error`
- HTTP 400 → `invalid_request`
- 其他 → `unknown`

### 4. Instrumentation Points

| Handler / Middleware | 改動 |
|---|---|
| `internal/callback/prometheus.go` | 新增 `api_key` label、新增 `tianji_errors_total`、改 bucket |
| `internal/proxy/handler/chat.go` | `buildLogData` 已帶 `APIKey`（token hash），不需改 |
| `internal/proxy/handler/embedding.go` | 確認 `LogSuccess` 傳入 `APIKey` |
| `internal/proxy/handler/completion.go` | 同上 |
| `internal/proxy/handler/audio.go` | 同上 |
| `internal/proxy/handler/image.go` | 同上 |
| `internal/proxy/server.go` | 註冊 `/metrics` route |
| `internal/config/config.go` | 新增 `MetricsAuth` struct |

**不需改的**：
- `middleware/` 層不需 instrument（auth、rate limit 已在 handler 層 callback 處理）
- `callback/registry.go` 介面不變

### 5. Label Cardinality 控管

- `api_key` 使用 token hash 前 8 碼，非完整 hash（避免 cardinality 爆炸，且足以區分）
- `model` 以 requested model name 為準（不展開 router alias）
- `error_type` 固定 6 個 enum 值

預估 cardinality：~models(50) × providers(20) × keys(100) × status(10) ≈ 100K series（可接受）。

---

## Acceptance Criteria

### SC-001: `/metrics` endpoint 回傳 Prometheus 格式
- GET `/metrics` 回傳 HTTP 200
- Content-Type 為 `text/plain; version=0.0.4` 或 `application/openmetrics-text`
- Body 包含 `tianji_requests_total` metric

### SC-002: 請求數 by model & api_key
- 發送 chat completion 請求（帶 API key）
- GET `/metrics` 可見 `tianji_requests_total{model="<model>", api_key="<hash_prefix>", status="success"}` 值 >= 1

### SC-003: 延遲 histogram 使用 LLM-tuned bucket
- GET `/metrics` 中 `tianji_request_total_latency_seconds_bucket` 的 `le` 值包含 `0.05`, `0.1`, `0.25`, `0.5`, `1`, `2.5`, `5`, `10`, `30`, `60`, `120`

### SC-004: Token usage by api_key
- 發送 chat completion 請求
- GET `/metrics` 可見 `tianji_tokens_total{type="prompt", api_key="<hash_prefix>"}` 值 > 0

### SC-005: Error counter 正確分類
- 發送一個會 400 的 bad request
- GET `/metrics` 可見 `tianji_errors_total{error_type="invalid_request"}` 值 >= 1

### SC-006: Optional basic auth on `/metrics`
- 設定 `metrics_auth.enabled: true` + username/password
- 不帶 auth GET `/metrics` → 401
- 帶正確 basic auth → 200

### SC-007: `/metrics` 不影響現有 `/health` 和 LLM routes
- 所有現有 contract tests 通過
- `/health/readiness` 仍回傳 200

### SC-008: Label cardinality 安全
- `api_key` label 值為 8 字元 hex string（token hash 前 8 碼）
- `error_type` label 值限定為 6 個 enum 之一

---

## Migration / Rollout

1. **Breaking change**：無。新增 endpoint + 現有 metric label 新增 `api_key`（Prometheus 視為新 time series，不影響舊 query）
2. **Config backward compat**：`metrics_auth` 為 optional，預設 disabled
3. **Dependency**：無新 dependency（`prometheus/client_golang` 已在 go.mod）

## References

- 現有實作：`internal/callback/prometheus.go`
- Prometheus naming：https://prometheus.io/docs/practices/naming/
- Histogram best practices：https://prometheus.io/docs/practices/histograms/
