# Spec: Prometheus Metrics Exporter

**Issue:** HO-15
**Status:** Draft
**Author:** 諸葛亮 (PM)
**Date:** 2026-03-01

## Overview

tianjiLLM 已有完整的 Prometheus metrics callback（`internal/callback/prometheus.go`），但 `/metrics` endpoint 未註冊到 chi router，外部 Prometheus scraper 無法抓取指標。本 spec 補齊最後一哩路：註冊 route、擴充 labels、新增 error metric。

## Goals

1. 暴露 `/metrics` HTTP endpoint（Prometheus exposition format）
2. 所有現有 metrics 加入 `api_key` label（可透過 config 開關）
3. 新增 `tianji_errors_total` counter metric
4. 提供 config flag 控制 `/metrics` 是否需要 auth

## Non-Goals

- 自訂 metrics registry（繼續用 `prometheus.DefaultRegisterer`）
- Grafana dashboard 或 alerting rules（後續 issue）
- Push gateway 支援

---

## Plan

### 1. 註冊 `/metrics` route

**檔案：** `internal/proxy/server.go` — `setupRoutes()`

在 `/health` route group 下方（auth middleware 之前）加入：

```go
// Prometheus metrics endpoint (no auth by default)
if s.Config.Metrics.Enabled {
    if s.Config.Metrics.RequireAuth {
        r.Group(func(r chi.Router) {
            r.Use(s.AuthMiddleware)
            r.Handle("/metrics", callback.Handler())
        })
    } else {
        r.Handle("/metrics", callback.Handler())
    }
}
```

`callback.Handler()` 已存在於 `internal/callback/prometheus.go`，直接呼叫即可。

### 2. 擴充 config

**檔案：** `internal/config/config.go`（或對應 config struct 檔案）

新增 `Metrics` section：

```go
type MetricsConfig struct {
    Enabled       bool `yaml:"enabled" envconfig:"METRICS_ENABLED" default:"true"`
    RequireAuth   bool `yaml:"require_auth" envconfig:"METRICS_REQUIRE_AUTH" default:"false"`
    PerKeyMetrics bool `yaml:"per_key_metrics" envconfig:"METRICS_PER_KEY" default:"false"`
}
```

並在主 `Config` struct 加入 `Metrics MetricsConfig`。

**環境變數對照：**
| Config | Env Var | Default | 說明 |
|--------|---------|---------|------|
| `metrics.enabled` | `METRICS_ENABLED` | `true` | 是否啟用 `/metrics` endpoint |
| `metrics.require_auth` | `METRICS_REQUIRE_AUTH` | `false` | `/metrics` 是否需要 Bearer token auth |
| `metrics.per_key_metrics` | `METRICS_PER_KEY` | `false` | 是否在 metrics label 中包含 `api_key` |

### 3. 加入 `api_key` label

**檔案：** `internal/callback/prometheus.go`

修改所有 metric 的 label 定義，條件式加入 `api_key`：

**推薦方案：** 統一帶 `api_key` label，當 `PerKeyMetrics=false` 時填固定值 `"_all"`。這樣 metric schema 一致，Prometheus relabeling 也容易處理。

具體改動：

```go
func NewPrometheusCallback(perKey bool) *PrometheusCallback {
    // 修改所有 label slices 加入 "api_key":
    // []string{"model", "provider"}           → []string{"model", "provider", "api_key"}
    // []string{"model", "provider", "status"} → []string{"model", "provider", "status", "api_key"}
    // []string{"model", "provider", "type"}   → []string{"model", "provider", "type", "api_key"}
}
```

`LogSuccess` / `LogFailure` 方法中：

```go
func (p *PrometheusCallback) apiKeyLabel(raw string) string {
    if !p.perKey {
        return "_all"
    }
    // 只取 SHA-256 前 8 字元 hex，避免洩漏完整 key
    h := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(h[:4])
}
```

> **安全考量：** `api_key` label 不應使用原始 key 值。使用 SHA-256 前 8 字元 hex 作為 label value，既可區分不同 key 又不洩漏敏感資訊。

### 4. 新增 `tianji_errors_total` metric

**檔案：** `internal/callback/prometheus.go`

```go
errorCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "tianji_errors_total",
    Help: "Total number of errors by type",
}, []string{"model", "provider", "error_type", "api_key"}),
```

在 `LogFailure` 中 increment，`error_type` 從 HTTP status code 或 error category 衍生：
- `"timeout"` — context deadline exceeded
- `"rate_limit"` — 429
- `"upstream_error"` — 5xx from provider
- `"client_error"` — 4xx
- `"unknown"` — fallback

### 5. 初始化接線

**檔案：** `cmd/tianji/main.go`（或 callback 初始化位置）

確保 `NewPrometheusCallback(cfg.Metrics.PerKeyMetrics)` 傳入 config。

**檔案：** `internal/proxy/server.go`

確保 `Server` struct 能存取 `Config.Metrics`。

---

## 變更摘要

| 檔案 | 改動 |
|------|------|
| `internal/config/config.go` | 新增 `MetricsConfig` struct |
| `internal/callback/prometheus.go` | 加 `api_key` label、加 `tianji_errors_total`、`NewPrometheusCallback` 加參數 |
| `internal/proxy/server.go` | `setupRoutes()` 註冊 `/metrics` |
| `cmd/tianji/main.go` | 傳 config 到 `NewPrometheusCallback` |

---

## Acceptance Criteria

### AC-1: `/metrics` endpoint 可存取
- `curl http://localhost:<port>/metrics` 回傳 HTTP 200
- Response Content-Type 為 `text/plain; version=0.0.4` 或 `application/openmetrics-text`
- Response body 包含 `tianji_requests_total` metric

### AC-2: 預設不需 auth
- 不帶 `Authorization` header 請求 `/metrics` → 200 OK
- 設定 `METRICS_REQUIRE_AUTH=true` 後，不帶 token → 401/403

### AC-3: 請求數 by model
- 發送一筆 chat completion 請求（model=gpt-4）
- `/metrics` 中 `tianji_requests_total{model="gpt-4",...}` counter ≥ 1

### AC-4: 請求數 by api_key
- 設定 `METRICS_PER_KEY=true`
- 用不同 API key 各發一筆請求
- `/metrics` 中 `tianji_requests_total` 出現兩個不同的 `api_key` label value
- `api_key` label value 為 hash prefix（非原始 key）

### AC-5: Per-key 預設關閉
- 預設（`METRICS_PER_KEY` 未設定）時，所有 metrics 的 `api_key` label 值為 `"_all"`

### AC-6: 延遲 histogram
- `/metrics` 中包含 `tianji_request_total_latency_seconds_bucket` histogram buckets
- 發送請求後 histogram count > 0

### AC-7: Token usage counter
- 發送一筆請求後，`tianji_tokens_total{type="prompt",...}` 和 `tianji_tokens_total{type="completion",...}` > 0

### AC-8: Error counter
- 觸發一筆錯誤請求（e.g., invalid model）
- `/metrics` 中 `tianji_errors_total` counter ≥ 1，包含 `error_type` label

### AC-9: Config flag 控制 endpoint 啟用
- 設定 `METRICS_ENABLED=false` → `/metrics` 回傳 404

---

## 注意事項

### Cardinality 風險
`api_key` label 會依 key 數量線性增長 metric cardinality。若系統有 1000 個 key × 50 個 model × 6 個 metric = 300K time series。因此：
- **預設關閉** `per_key_metrics`
- 使用 hash prefix 而非原始 key
- 文件中標註 cardinality 警告

### Auth 考量
- Prometheus scraper 通常不帶 Bearer token，故預設不加 auth
- 生產環境建議透過 network policy（firewall / internal network）限制 `/metrics` 存取
- 提供 `METRICS_REQUIRE_AUTH` 作為額外防線

### 向後相容
- 現有 metrics 加入 `api_key` label 會改變 time series identity，Prometheus 會視為新 series
- 升級時舊 series 會自然過期（依 retention），不影響功能
- 建議在 CHANGELOG 中標註 breaking metric schema change

### 預估工作量
~1-2 小時（含測試）
