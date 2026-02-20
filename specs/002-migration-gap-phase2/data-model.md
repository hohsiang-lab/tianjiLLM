# Data Model — TianjiLLM Go Migration Phase 2

**Date**: 2026-02-16
**Spec**: [spec.md](spec.md)

## Entities

### 1. SecretManager (Interface)

```go
// internal/secretmanager/secretmanager.go

type SecretManager interface {
    // Name returns the manager identifier (e.g., "aws_secrets_manager")
    Name() string
    // Get retrieves a secret value by path
    Get(ctx context.Context, path string) (string, error)
    // Health checks if the manager is reachable
    Health(ctx context.Context) error
}

type CachedSecretManager struct {
    manager  SecretManager
    cache    map[string]cachedEntry
    mu       sync.RWMutex
    ttl      time.Duration // default 86400s
}

type cachedEntry struct {
    value   string
    expires time.Time
}
```

**Implementations**:
| Name | Auth Config | Package |
|------|-------------|---------|
| `AWSSecretsManager` | Region, AccessKeyID/SecretKey or IAM role | `internal/secretmanager/aws.go` |
| `GoogleSecretManager` | ProjectID, CredentialsFile or ADC | `internal/secretmanager/google.go` |
| `AzureKeyVault` | VaultURL, TenantID/ClientID/ClientSecret or DefaultAzureCredential | `internal/secretmanager/azure.go` |
| `VaultSecretManager` | Address, Token or AppRole (RoleID + SecretID), MountPath | `internal/secretmanager/vault.go` |

**Validation rules**:
- `path` must not be empty
- `ttl` must be > 0 (default 86400s)
- Auth config validated at construction time

**Registration**: Self-register via `secretmanager.Register(name, factory)` — same pattern as providers

---

### 2. Guardrail (Extended)

Existing interface in `internal/guardrail/guardrail.go` — no changes needed. New implementations:

```go
// New guardrail implementations

type BedrockGuardrail struct {
    client         *bedrockruntime.Client
    guardrailID    string
    guardrailVer   string
    failOpen       bool
}

type AzureTextModeration struct {
    endpoint string
    apiKey   string
    failOpen bool
}

type AzurePromptShield struct {
    endpoint string
    apiKey   string
    failOpen bool
}

type LakeraGuardrail struct {
    apiURL   string
    apiKey   string
    failOpen bool
    onFlagged string // "block" or "monitor"
}

type GenericGuardrail struct {
    name     string
    apiURL   string
    headers  map[string]string
    hooks    []Hook
    failOpen bool
}

type ContentFilter struct {
    categories map[string]*regexp.Regexp
    severity   int // 0=off, 1=low, 2=medium, 3=high
}

type ToolPermissionGuardrail struct {
    allowedTools map[string][]string // key/team → allowed tool names
}
```

**Failure policy**: Each guardrail has `failOpen bool`:
- `true` = service unavailable → pass request through
- `false` = service unavailable → block request

---

### 3. BatchLogger (Base)

```go
// internal/callback/batchlogger.go

type BatchLogger struct {
    queue       []LogData
    mu          sync.Mutex
    batchSize   int           // default 512
    flushTicker *time.Ticker  // default 5s
    flushFn     func(batch []LogData) error
    stopCh      chan struct{}
}
```

**Lifecycle**:
1. `Start()` — begins ticker goroutine
2. `LogSuccess(data)` / `LogFailure(data)` — append to queue, flush if full
3. `flush()` — take all items, call `flushFn`, on error: discard + log
4. `Stop()` — flush remaining, stop ticker

**Cloud logger implementations**:

```go
// internal/callback/s3.go
type S3Logger struct {
    BatchLogger
    client *s3.Client
    bucket string
    prefix string
}

// internal/callback/gcs.go
type GCSLogger struct {
    BatchLogger
    client *storage.Client
    bucket string
    prefix string
}

// internal/callback/azureblob.go
type AzureBlobLogger struct {
    BatchLogger
    client    *azblob.Client
    container string
    prefix    string
}

// internal/callback/dynamodb.go
type DynamoDBLogger struct {
    BatchLogger
    client    *dynamodb.Client
    tableName string
}

// internal/callback/sqs.go
type SQSLogger struct {
    BatchLogger
    client   *sqs.Client
    queueURL string
}
```

**Log payload**: Full `LogData` struct serialized as JSON (matches Python `StandardLoggingPayload`)

---

### 4. ObservabilityLogger (HTTP-based callbacks)

```go
// internal/callback/langsmith.go
type LangsmithLogger struct {
    BatchLogger
    apiKey   string
    project  string
    baseURL  string // default "https://api.smith.langchain.com"
    tenantID string
}

// internal/callback/helicone.go — post-request logger (not BatchLogger)
type HeliconeLogger struct {
    apiKey  string
    apiBase string // default "https://api.hconeai.com"
}
// HeliconeLogger implements CustomLogger (LogSuccess/LogFailure).
// On LogSuccess: POST to {apiBase}/oai/v1/log with providerRequest/providerResponse/timing.
// Matches Python tianji/integrations/helicone.py behavior.

// internal/callback/mlflow.go
type MLflowLogger struct {
    client       *http.Client
    trackingURI  string
    experimentID string
}

// internal/callback/braintrust.go
type BraintrustLogger struct {
    BatchLogger
    apiKey  string
    baseURL string
}

// internal/callback/wandb.go
type WandBLogger struct {
    BatchLogger
    apiKey  string
    project string
    entity  string
}
```

---

### 5. PromptTemplate

```go
// internal/prompt/prompt.go

type PromptSource interface {
    Name() string
    GetPrompt(ctx context.Context, promptID string, opts PromptOptions) (*ResolvedPrompt, error)
}

type PromptOptions struct {
    Version   *int
    Label     *string
    Variables map[string]string
}

type ResolvedPrompt struct {
    Messages []model.Message
    Metadata map[string]string
}

// internal/prompt/langfuse.go
type LangfusePromptSource struct {
    client    *http.Client
    baseURL   string
    publicKey string
    secretKey string
    cache     map[string]*cachedPrompt
    mu        sync.RWMutex
    ttl       time.Duration
}
```

---

### 6. Router Strategy (Extended)

```go
// internal/router/strategy/leastbusy.go
type LeastBusy struct {
    inFlight map[string]*atomic.Int64 // deployment ID → count
    mu       sync.RWMutex
}

// internal/router/strategy/budgetlimiter.go
type BudgetLimiter struct {
    budgets map[string]float64 // provider → remaining budget
    mu      sync.RWMutex
}
```

---

### 7. SpendAnalytics (Extended)

```go
// internal/spend/analytics.go

type AnalyticsQuery struct {
    GroupBy   string    // "team", "tag", "model", "user", "key"
    StartDate time.Time
    EndDate   time.Time
    TopN     int       // for top-N queries
}

type AnalyticsResult struct {
    Groups []SpendGroup
    Total  float64
}

type SpendGroup struct {
    Name  string
    Spend float64
    Count int
}
```

---

## Relationships

```
Config
├── secret_manager_config → SecretManager (resolves $SECRET refs)
├── guardrail_config → Guardrail Registry (registers guardrails)
├── callback_config → Callback Registry (registers loggers)
├── cache_config → Cache (memory|redis|redis_cluster|dual)
├── router_config → Router Strategy (shuffle|latency|cost|usage|tag|leastbusy|budget)
└── prompt_config → PromptSource (langfuse|generic)

Request Flow:
  Client → Auth → [PromptSource] → [Pre-call Guardrails] → Provider → [Post-call Guardrails] → Response
                                                                    ↓
                                                              Callback Registry → [S3|GCS|Azure|DynamoDB|SQS|Langsmith|...]
```

## State Transitions

### SecretManager Cache Entry
```
Empty → Fetched (on first Get) → Expired (after TTL) → Fetched (on next Get)
```

### BatchLogger Queue
```
Empty → Accumulating → Full (≥ batchSize) → Flushing → Empty
                    → Ticker fires → Flushing → Empty
                    → Flush fails → Discard + Log → Empty
```

### Guardrail Result
```
Request → Run → Passed (continue) | Blocked (return error) | Modified (use modified request)
```
