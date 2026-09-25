# TianjiLLM 完整 Request Flow

## 概述

```mermaid
flowchart LR
    C([Client]) --> R[Chi Router]
    R --> MW[Middleware Chain]
    MW --> H[Handler]
    H --> PR[Provider Resolution]
    PR --> TX[TransformRequest]
    TX --> UP[(Upstream LLM)]
    UP --> TR[TransformResponse]
    TR --> CB[Callbacks]
    CB --> C
```

---

## 一、啟動初始化 (`cmd/tianji/main.go`)

```mermaid
flowchart TD
    A[main] --> B[config.Load]
    A --> C[pgxpool.New + DB migrations]
    A --> D[Cache\nMemory / Redis / Dual]
    A --> E[Router optional\nload balancer]
    A --> F[Guardrails Registry]
    A --> G[Callbacks Registry\nspend tracker + user callbacks]
    A --> H[Policy Engine optional]
    A --> I[Provider Registry\ninit side-effect auto-register]
    A --> J[Scheduler\nbudget reset / spend cleanup]
    A --> K[Pricing Calculator]
    A --> L[rateLimitStore\nInMemoryRateLimitStore]
    L --> L1[goroutine: Prune every 1min\nremove entries inactive >5min]
    A --> M[discordAlerter\nDiscordRateLimitAlerter]
    A --> N[proxy.NewServer\nchi router + all middleware]
```

**Provider 自注冊模式**：所有 provider 透過 `import _` 的 side-effect，在 `init()` 中呼叫 `provider.Register("name", instance)`，無需中央枚舉。

---

## 二、路由結構 (`internal/proxy/server.go`)

```mermaid
flowchart TD
    R[chi.Router] --> H1["/health/*\n無 auth"]
    R --> H2["/metrics\n無 auth - Prometheus"]
    R --> H3["/v1/chat/completions\n/v1/embeddings\n/chat/completions"]
    R --> H4["/v1/messages\nAnthropic native"]
    R --> H5["/key/* /team/*\n/user/* /budget/*\n管理 API - auth required"]
    R --> H6["/ui/*\nAdmin dashboard"]

    H3 --> MW[Middleware Chain\nAuth → Parallel → RateLimit → CacheControl]
    H4 --> MW
    MW --> CH[ChatCompletion / AnthropicMessages Handler]
```

---

## 三、Middleware Chain（執行順序）

```mermaid
flowchart TD
    REQ([Incoming Request]) --> MW0

    subgraph MW0[Chi 框架內建 - 全域]
        direction LR
        m0a[RequestID] --> m0b[RealIP] --> m0c[Recoverer] --> m0d[StructuredLogging]
    end

    MW0 --> MW1

    subgraph MW1[Stage 1: AuthMiddleware]
        direction TB
        a1[提取 token\nAuthorization: Bearer 或 api-key]
        a1 --> a2{驗證順序}
        a2 -->|master key| a3[SHA256 compare]
        a2 -->|JWT| a4[JWT verify]
        a2 -->|virtual key| a5[DB lookup\nVerificationToken]
        a3 & a4 & a5 --> a6[存入 context\nUserID / TeamID / TokenHash / Guardrails]
        a3 -->|失敗| E1[401 Unauthorized]
        a4 -->|失敗| E1
        a5 -->|失敗| E1
    end

    MW1 --> MW2

    subgraph MW2[Stage 2: ParallelRequestMiddleware]
        direction TB
        b1[讀取 max_parallel_requests] --> b2[Redis INCR\ntianji:parallel:keyHash]
        b2 -->|超限| E2[409 Too Many Requests]
        b2 -->|通過| b3[繼續\nResponse 時自動 DECR]
    end

    MW2 --> MW3

    subgraph MW3[Stage 3: DynamicRateLimitMiddleware]
        direction TB
        c1[RPM sliding window\nLua script] --> c3{超限?}
        c2[TPM per model+key] --> c3
        c3 -->|是| E3[429 Too Many Requests]
        c3 -->|否| c4[繼續]
    end

    MW3 --> MW4

    subgraph MW4[Stage 4: CacheControlMiddleware]
        direction TB
        d1[讀取 request body cache field] --> d2{directive 在\nAllowedCacheControls?}
        d2 -->|否| E4[403 Forbidden]
        d2 -->|是| d3[繼續]
    end

    MW4 --> HANDLER([Handler])
```

---

## 四、Handler 執行 (`internal/proxy/handler/chat.go`)

```mermaid
flowchart TD
    H([Handler]) --> D1[JSON decode\nChatCompletionRequest]
    D1 --> D2[解析 prompt template\n若 PromptName 有設定]
    D2 --> D3[resolveProvider\n見 Section V]
    D3 --> D4[LogProviderResolved]
    D4 --> D5[PolicyEngine.Evaluate\n合併 guardrails]
    D5 --> D6[Guardrails.RunPreCall]
    D6 --> D7{IsStreaming?}
    D7 -->|Yes| S[handleStreamingCompletion\nSSE - Section VI-B]
    D7 -->|No| N[handleNonStreamingCompletion\nJSON - Section VI-A]
```

---

## 五、Provider Resolution

```mermaid
flowchart TD
    RP[resolveProvider] --> Q1{Router configured?}

    Q1 -->|Yes| RR[Router.Route]
    RR --> R1[auto_router/ 語意路由解析]
    R1 --> R2[ModelGroupAlias 展開]
    R2 --> R3{精確 deployment?}
    R3 -->|No| R3W[wildcardMatch]
    R3 & R3W --> R4[healthyDeployments\nfailure count + cooldown]
    R4 --> R5[重試循環 numRetries+1]
    R5 --> R6[strategy.Pick\nshuffle / latency / cost]
    R6 --> R7[provider.GetWithBaseURL]
    R7 -->|成功| OK1[(Deployment, Provider)]
    R7 -->|失敗| R8[RecordFailure + 繼續]
    R8 --> R5

    RR -->|全部失敗| FB[GeneralFallback]
    FB --> FB1[settings.Fallbacks model-specific]
    FB --> FB2[settings.DefaultFallbacks]
    FB1 & FB2 -->|成功| OK1
    FB1 & FB2 -->|失敗| ERR[error]

    Q1 -->|No| DC[resolveProviderFromConfig]
    DC --> DC1[findModelConfig\n精確匹配 → 通配符匹配]
    DC1 --> DC2[provider.ParseModelName\nprovider/model]
    DC2 --> DC3[provider.GetWithBaseURL]
    DC3 --> OK1
```

---

## 六、Provider Transform + Upstream 呼叫

### VI-A 非流式（Non-Streaming）

```mermaid
flowchart TD
    NS[handleNonStreamingCompletion] --> C1{Cache HIT?}
    C1 -->|HIT| CR[X-Cache: HIT\n直接返回]
    C1 -->|MISS| T1[TransformRequest\nOpenAI → Provider native]
    T1 --> U1[http.DefaultClient.Do\n呼叫上游 LLM]
    U1 --> L1[LogUpstreamResponded\nstatusCode + latencyMs]
    L1 --> T2[TransformResponse\nProvider native → OpenAI]
    T2 --> CB1[go Callbacks.LogSuccess\n非同步]
    CB1 --> S1[spend.Tracker → SpendLogs]
    CB1 --> S2[user callbacks\nLangSmith / Helicone]
    T2 --> C2[Cache.Set TTL 5min]
    T2 --> RESP[writeJSON 200]
```

### VI-B 流式（Streaming / SSE）

```mermaid
flowchart TD
    ST[handleStreamingCompletion] --> F1{w implements\nhttp.Flusher?}
    F1 -->|No| ERR[500 streaming not supported]
    F1 -->|Yes| T1[TransformRequest]
    T1 --> U1[http.DefaultClient.Do]
    U1 --> H1[Set SSE Headers\nContent-Type: text/event-stream]
    H1 --> SCAN[bufio.Scanner\n逐行讀取 SSE body]
    SCAN --> LINE{line has\ndata: prefix?}
    LINE -->|No| SCAN
    LINE -->|Yes| TC[TransformStreamChunk]
    TC --> DONE{isDone?}
    DONE -->|Yes| WD[Write data: DONE + Flush]
    WD --> CB2[go LogStreamSuccess\n非同步]
    WD --> CACHE[cache assembled result]
    DONE -->|No| WC[Write chunk + Flush]
    WC --> TTFT[記錄 timeToFirstToken\n第一個 chunk]
    TTFT --> ACC[累積 accUsage\naccContent]
    ACC --> SCAN
```

---

## 七、Anthropic Native Proxy（OAuth Token 多 Token 管理）

`POST /v1/messages` 走獨立路徑，不經過 `ChatCompletion` handler。

```mermaid
flowchart TD
    AM[AnthropicMessages] --> NP[nativeProxy\nprovider=anthropic]

    NP --> RU[resolveAllNativeUpstreams\n遍歷 ModelList 篩選 prefix=anthropic]
    RU --> ST[selectUpstreamWithThrottle]

    ST --> Q1{IsOAuthToken?}
    Q1 -->|No| AV[available]
    Q1 -->|Yes| Q2{RateLimitStore\nhas record?}
    Q2 -->|No record\n首次/Prune後| AV
    Q2 -->|Yes| Q3{UnifiedStatus?}
    Q3 -->|rate_limited\noverage| SK[skip\ntrackNearestReset]
    Q3 -->|allowed| Q4{5h/7d util\n≥ threshold?}
    Q4 -->|Yes| SK
    Q4 -->|No| AV

    AV --> RR[roundRobinSelect\natomic counter per provider]
    SK --> ALLSK{全部 skip?}
    ALLSK -->|Yes| E429[429 + Retry-After\nnearestReset]
    ALLSK -->|No| RR

    RR --> RP[httputil.ReverseProxy]

    subgraph Director[Director - 設置 headers]
        D1{IsOAuthToken?}
        D1 -->|Yes| D2[Authorization: Bearer\nanthropic-beta: oauth-2025-04-20]
        D1 -->|No| D3[x-api-key header]
    end

    RP --> Director
    Director --> ANT[(Anthropic API)]

    ANT --> MR[ModifyResponse\n200 和非 200 都執行]
    MR --> P1[ParseAnthropicOAuthRateLimitHeaders\nanthropic-ratelimit-unified-*]
    P1 --> ST1[RateLimitStore.Set\nsha256 apiKey :12]
    ST1 --> Q5{IsOAuthToken?}
    Q5 -->|Yes| DA[go DiscordAlerter\nCheckAndAlertOAuth\n1h 冷卻去重]
    Q5 -->|No| DONE[done]
    DA --> DONE
```

詳見 [oauth-token-strategy.md](./oauth-token-strategy.md)。

---

## 八、錯誤處理

| 位置 | HTTP 狀態碼 | 觸發條件 |
|------|------------|---------|
| AuthMiddleware | 401 | token 缺失或無效 |
| ParallelMiddleware | 409 | max_parallel_requests 超限 |
| RateLimitMiddleware | 429 | RPM/TPM 超限 |
| CacheControlMiddleware | 403 | cache directive 未授權 |
| resolveProvider | 404 | model 找不到 |
| resolveProvider | 403 | access denied |
| TransformRequest | 500 | provider 轉換失敗 |
| Upstream HTTP | pass-through | 上游回傳錯誤 |
| TransformResponse | 502 | response 解析失敗 |
| selectUpstreamWithThrottle | 429 | 全部 OAuth token throttled |

所有失敗都會觸發：
```go
h.logFailure(ctx, req, p, startTime, err)
  ├─ recordErrorLog(ctx, ...) → INSERT ErrorLogs（同步）
  └─ go h.Callbacks.LogFailure(data)（非同步）
```

---

## 九、完整流程時間軸

```mermaid
sequenceDiagram
    participant C as Client
    participant CHI as Chi Router
    participant AUTH as AuthMiddleware
    participant PAR as ParallelMW
    participant RL as RateLimitMW
    participant CC as CacheControlMW
    participant H as Handler
    participant PR as Provider Resolution
    participant P as Provider
    participant UP as Upstream LLM
    participant CB as Callbacks

    C->>CHI: POST /v1/chat/completions
    CHI->>AUTH: forward
    AUTH-->>C: 401 (token invalid)
    AUTH->>PAR: token valid
    PAR-->>C: 409 (parallel limit)
    PAR->>RL: ok
    RL-->>C: 429 (RPM/TPM exceeded)
    RL->>CC: ok
    CC-->>C: 403 (cache directive denied)
    CC->>H: ok

    H->>PR: resolveProvider(modelName)
    PR-->>H: (Provider, apiKey, model)

    alt Non-Streaming
        H->>P: TransformRequest
        P->>UP: HTTP Request
        UP-->>P: HTTP Response
        P->>H: TransformResponse
        H-->>C: JSON 200
        H->>CB: go LogSuccess (async)
    else Streaming
        H->>P: TransformRequest
        P->>UP: HTTP Request
        loop SSE chunks
            UP-->>P: chunk
            P->>H: TransformStreamChunk
            H-->>C: data: {...}\n\n (Flush)
        end
        UP-->>P: [DONE]
        H-->>C: data: [DONE]\n\n
        H->>CB: go LogStreamSuccess (async)
    end

    CB->>CB: spend.Tracker → SpendLogs
    CB->>CB: user callbacks (optional)
```

---

## 十、關鍵檔案索引

| 檔案 | 職責 |
|------|------|
| `cmd/tianji/main.go` | 啟動、依賴注入、wiring |
| `internal/proxy/server.go` | chi router、middleware 註冊 |
| `internal/proxy/handler/chat.go` | ChatCompletion handler、流程編排 |
| `internal/proxy/handler/handler.go` | Provider resolution、`resolveProviderFromConfig` |
| `internal/proxy/handler/native_format.go` | Anthropic native proxy、`ModifyResponse` |
| `internal/proxy/handler/native_upstream.go` | `selectUpstreamWithThrottle`、round-robin |
| `internal/proxy/middleware/auth.go` | Authentication、虛擬 key |
| `internal/proxy/middleware/parallel.go` | Parallel request limiting |
| `internal/proxy/middleware/ratelimit.go` | RPM/TPM limiting（Redis Lua） |
| `internal/proxy/middleware/cache_control.go` | Cache directive 驗證 |
| `internal/router/router.go` | 多 deployment 路由、重試、健康檢查 |
| `internal/router/fallback.go` | GeneralFallback / ContextWindowFallback / ContentPolicyFallback |
| `internal/provider/provider.go` | Provider interface 定義 |
| `internal/callback/ratelimit_store.go` | `InMemoryRateLimitStore`、OAuth token 狀態 |
| `internal/callback/discord_ratelimit.go` | Discord 告警、1h 冷卻去重 |
