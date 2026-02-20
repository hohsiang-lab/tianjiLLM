# Research: Phase 3 — Enterprise Features & Full Parity

**Branch**: `003-migration-gap-phase3` | **Date**: 2026-02-17
**Status**: Complete — all NEEDS CLARIFICATION resolved

## Research Summary

6 parallel research agents were dispatched to investigate:
1. SCIM 2.0 Go libraries
2. Go scheduler patterns (existing codebase analysis)
3. Go policy engine patterns
4. Callback SDK availability for 19 integrations
5. Provider API format analysis for 12 native providers
6. CyberArk Go integration + object storage cache patterns

---

## Decision 1: SCIM 2.0 Implementation

**Decision**: Use `github.com/elimity-com/scim` library + `github.com/scim2/filter-parser/v2` for filter parsing

**Rationale**:
- `elimity-com/scim` is an actively maintained Go SCIM 2.0 server library (MIT license) that provides:
  - `scim.NewServer()` — complete RFC 7644 HTTP server (routing, serialization, error handling)
  - `ResourceHandler` interface — we only implement 6 methods (Create/Get/GetAll/Patch/Replace/Delete)
  - `schema.CoreUserSchema()` / `schema.CoreGroupSchema()` — built-in RFC 7643 schemas
  - `scim/errors` — RFC 7644 standard error format
  - `scim/optional` — optional field handling
  - Built-in Okta + Azure AD IDP compatibility tests
- Production usage confirmed in major projects:
  - **Casdoor** (9k+ stars) — SSO/IAM platform, full SCIM provisioning
  - **Zitadel** (14k+ stars) — enterprise IAM, SCIM endpoints
  - **Getprobo** — compliance platform, uses elimity-com/scim + scim2/filter-parser
  - **Teleport** (18k+ stars) — uses scim2/filter-parser/v2
- `scim2/filter-parser/v2` provides full RFC 7644 filter parsing (eq, ne, co, sw, ew, gt, lt, etc.) — much more robust than our original plan's regex-based parser or Python's string split approach
- We only need to write `ResourceHandler` implementations (~150 lines for User + Group mapping to existing tables), the library handles all RFC compliance

**Alternatives Considered**:
- **Self-implement from scratch** (original plan) — ~400 lines, but must handle RFC 7644 edge cases (pagination, ETags, PATCH operations, error format). Library handles all of this
- `imulab/go-scim` v2 (Go) — Go 1.13, last commit 2020-12-25, stale. Depends on archived `satori/go.uuid`
- OPA/external SCIM server — overkill; adds deployment complexity

**Implementation Pattern**:
```go
// Server setup — elimity-com/scim handles routing, serialization, errors
server, _ := scim.NewServer(&scim.ServerArgs{
    ServiceProviderConfig: &scim.ServiceProviderConfig{},
    ResourceTypes: []scim.ResourceType{
        {Name: "User", Endpoint: "/Users", Schema: schema.CoreUserSchema(), Handler: userHandler{}},
        {Name: "Group", Endpoint: "/Groups", Schema: schema.CoreGroupSchema(), Handler: groupHandler{}},
    },
})
// Mount: chi.Mount("/scim/v2", server)

// We implement ResourceHandler interface for User/Group → existing User/Team table mapping
type userHandler struct{ db *pgxpool.Pool }
func (h userHandler) Create(r *http.Request, attrs scim.ResourceAttributes) (scim.Resource, error) { ... }
func (h userHandler) Get(r *http.Request, id string) (scim.Resource, error) { ... }
func (h userHandler) GetAll(r *http.Request, params scim.ListRequestParams) (scim.Page, error) { ... }
func (h userHandler) Patch(r *http.Request, id string, ops []scim.PatchOperation) (scim.Resource, error) { ... }
func (h userHandler) Replace(r *http.Request, id string, attrs scim.ResourceAttributes) (scim.Resource, error) { ... }
func (h userHandler) Delete(r *http.Request, id string) error { ... }
```
- SCIM User → internal User table mapping via `ResourceHandler` (userName→user_id, active→metadata["scim_active"])
- SCIM Group → internal Team table mapping via `ResourceHandler`
- Bearer token auth handled by existing middleware (master key or dedicated SCIM token)

---

## Decision 2: Background Scheduler

**Decision**: Use stdlib `time.Ticker` + `context.Context` + `sync.WaitGroup` — 60 lines

**Rationale**:
- Existing codebase already uses this pattern in 3 places:
  - `internal/cache/memory.go:72-85` — pure ticker for cache cleanup
  - `internal/spend/redis_buffer.go:55-68` — ticker + context for spend flush
  - `internal/callback/batchlogger.go:50-62` — ticker + stopCh + final flush
- All 3 patterns work in production; unifying them into a single Scheduler struct is trivial
- External libraries (robfig/cron, gocron) add unnecessary dependencies:
  - Cron syntax parsing (not needed — all jobs use fixed intervals)
  - 5+ transitive dependencies (gocron)
  - Loss of control over graceful shutdown
  - API learning curve

**Alternatives Considered**:
- `robfig/cron` v3 — cron syntax overkill, v1 unmaintained
- `go-co-op/gocron` — 5+ transitive deps, 15+ API methods for 3-method need
- APScheduler (Python equivalent) — no Go port

**Implementation Pattern**:
```go
type Scheduler struct {
    jobs   []jobEntry
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

type Job interface {
    Run(ctx context.Context) error
    Name() string
}

func (s *Scheduler) Add(job Job, interval time.Duration)
func (s *Scheduler) AddWithStartupRun(job Job, interval time.Duration)  // catch-up
func (s *Scheduler) Start()
func (s *Scheduler) Stop()  // cancel + wg.Wait
```

**Jobs to register** (matching Python's 10+ APScheduler jobs):
1. `BudgetResetJob` — check `budget_reset_at`, reset spend, update next reset (~10min, with jitter)
2. `SpendUpdateJob` — batch-write accumulated spend to DB (10s interval) — **critical, missing from original plan**
3. `SpendLogMonitorJob` — monitor spend log queue health (2s interval) — **critical, missing from original plan**
4. `SpendLogCleanupJob` — delete entries older than retention period (daily or cron)
5. `DeploymentHotReloadJob` — sync model deployments from DB (30s interval)
6. `HealthCheckJob` — probe deployment endpoints (configurable interval)
7. `CredentialRefreshJob` — reload credentials from DB (30s interval)
8. `BatchCostCheckJob` — check batch API costs (1h interval)
9. `ResponsesCostCheckJob` — check responses API costs (1h interval)
10. `KeyRotationJob` — process key rotations (24h interval)

**Distributed Lock** (for multi-pod deployments):
- Use `github.com/go-redsync/redsync/v4` (Redlock algorithm, BSD-3 license) for distributed mutex
- Proven in production: **Gitea** (global lock), **GitLab Workhorse** (workflow lock + circuit breaker), **SeaweedFS** (distributed storage), **Gitpod** (billing controller), **Harness** (CI/CD lock)
- Has `go-redsync/redsync/v4/redis/goredis/v9` adapter — seamless integration with our existing `redis/go-redis/v9`
- Handles clock drift, quorum, retry automatically — more reliable than hand-rolled `SETNX` + TTL
- Matches Python's `PodLockManager` pattern used by `SpendLogCleanup`
- Required for: budget reset, spend cleanup, key rotation (idempotent jobs can skip locking)

---

## Decision 3: Policy Engine

**Decision**: Self-implement in `internal/policy/` package — ~300 lines

**Rationale**:
- Requirements are narrow and well-defined:
  - Model regexp matching → `regexp.MatchString()` (stdlib, matching Python's `re.match()`)
  - Inheritance chain → tree traversal with cycle detection (DFS, ~30 lines)
  - Guardrail list merge → `append()` + `removeAll()` (~15 lines)
  - Pipeline execution → `for` loop + `switch` on action (~20 lines)
- External policy engines (OPA, Casbin) are massive overkill:
  - OPA: 42MB embedded binary, Rego DSL, bundle management
  - Casbin: RBAC/ABAC DSL, not designed for guardrail pipelines
- Python TianjiLLM's policy engine is simple: conditions match model names via `re.fullmatch()` (regexp), policies contain guardrail lists, attachments are multi-dimensional (teams[] + keys[] + models[] + tags[]) with **simple prefix-based wildcard matching** (endsWith `*` → startsWith prefix, NOT regexp)

**Alternatives Considered**:
- Open Policy Agent (OPA/Rego) — 42MB binary, Rego language learning curve, bundle management complexity
- Casbin — RBAC/ABAC model mismatch; designed for access control, not guardrail pipelines
- `hashicorp/hcl` — configuration language, not a policy engine

**Implementation Pattern**:
- `PolicyEngine` struct with `sync.RWMutex` + `map[string]Policy` (same as Router pattern)
- `Resolve(name string) → []string` — walk parent chain, merge add/remove lists
- `buildChain(name string) → []Policy` — iterative DFS with `seen` map for cycle detection
- `ExecutePipeline(ctx, []PipelineStep, req) → error` — for loop + switch on ActionNext/Allow/Block/ModifyResponse; supports `pass_data` for step-to-step data forwarding
- `Update(policies, attachments)` — full map rebuild under write lock (hot-reload)
- REST CRUD in `handler/policy.go` — copy existing key/team CRUD pattern

**File organization**: `internal/policy/` (not `internal/router/policy.go`) because policy is a distinct domain that handler imports separately from router.

---

## Decision 4: Callback SDK Selection

**Decision**: 5 use official SDKs, 14 use HTTP API

**Rationale**: Use SDK only when it provides substantial value (batching, retry, auth token refresh, connection pooling). Simple REST POST doesn't need a wrapper.

| # | Service | Approach | Package | Rationale |
|---|---------|----------|---------|-----------|
| **High-Value (9)** |
| 1 | Lunary | HTTP API | — | Python/JS only, simple REST POST |
| 2 | Traceloop | OTel SDK | `go.opentelemetry.io/otel` | Standard OTel, already in project |
| 3 | PostHog | SDK | `github.com/posthog/posthog-go` | Official Go SDK, handles batching + retry |
| 4 | Opik | HTTP API | — | Python/TS only, REST API available |
| 5 | Datadog LLM Obs | HTTP API | — | `datadog-go/v5` is DogStatsD (metrics only); Go LLM Obs exists in `dd-trace-go/v2/llmobs` but is experimental. Use HTTP API for stability, consistent with other HTTP callbacks. Future: migrate to `dd-trace-go/v2/llmobs` when stable |
| 6 | GCS Pub/Sub | SDK | `cloud.google.com/go/pubsub` | Google official, handles all complexity |
| 7 | OpenMeter | HTTP API | — | CloudEvents format, simple POST |
| 8 | Greenscale | HTTP API | — | Niche, simple webhook |
| 9 | PromptLayer | HTTP API | — | Python/JS only, simple POST |
| **Medium-Value (10)** |
| 10 | Argilla | HTTP API | — | Python-only ecosystem |
| 11 | Lago | SDK | `github.com/getlago/lago-go-client` | Official Go client, billing API |
| 12 | Azure Sentinel | SDK | `github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs` | Azure Monitor Log Ingestion module (modular SDK) |
| 13 | Supabase | pgx | `github.com/jackc/pgx/v5` | Supabase is Postgres — use existing driver |
| 14 | CloudZero | HTTP API | — | AWS cost API, simple REST |
| 15 | Logfire | HTTP API | — | Pydantic team, Python-first |
| 16 | Athina | HTTP API | — | Emerging platform, Python/TS only |
| 17 | DeepEval | HTTP API | — | LLM eval framework, Python-only |
| 18 | Galileo | HTTP API | — | Experiment platform, Python-first |
| 19 | Literal AI | HTTP API | — | TS/Python SDKs only |

**Source**: Context7 queries confirmed PostHog Go SDK (`posthog/posthog-go`), Lago Go client (`getlago/lago-go-client`), Opik no Go SDK (Python/TS only).

---

## Decision 5: Provider API Format Classification

**Decision**: Classify 12 providers into 3 tiers by complexity

**Rationale**: Python TianjiLLM source code analysis reveals clear complexity tiers. 8 of 12 providers are OpenAI-compatible requiring only auth + base URL configuration. Don't over-engineer simple cases.

### Tier 1: Pure OpenAI-Compatible (8 providers, ~150 lines total)

| Provider | Auth | Key Quirk |
|----------|------|-----------|
| DashScope | API Key | Content list → string |
| Volcengine | API Key | `thinking` param |
| MiniMax | API Key | `reasoning_split` + `cache_control` |
| Moonshot | API Key | `tool_choice=required` needs special message; temp [0,1] |
| NVIDIA NIM | API Key | Per-model supported params |
| DeepInfra | API Key | Tool message content: array → string |
| OpenRouter | API Key | `cache_control` moves to content; extract cost from response |
| Azure AI Studio | Bearer or `api-key` header | Content list → string; dynamic endpoint construction |

**Implementation**: Each gets its own provider module with `init()` self-registration. Can share `openaicompat` base patterns but need individual modules for custom transforms.

### Tier 2: Light Transform (2 providers, ~200 lines total)

| Provider | Auth | Transform |
|----------|------|-----------|
| GitHub Copilot | OAuth device flow + dynamic token refresh | Special headers, system→assistant msg conversion |
| Snowflake Cortex | Bearer token | `tool_spec` format (not `function`); `content_list` response |

### Tier 3: Heavy Transform (2 providers, ~300 lines total — reduced from ~500 by using OCI SDK)

| Provider | Auth | Transform |
|----------|------|-----------|
| Oracle OCI | `oracle/oci-go-sdk/v65` (official SDK — `common.RequestSigner()` handles RSA-SHA256 signing, `common.ConfigurationProvider` handles API key / Instance Principal / Resource Principal auth) | Custom message format, Cohere vs Generic vendor split, streaming wrapper. **Auth is SDK-handled**, reducing ~250 lines of manual signing to ~50 lines of config |
| SAP AI Core | OAuth token auto-refresh (no Go SDK — SAP only has Java/JS/Python SDKs) | Nested `modules.prompt_templating` config, deployment URL query |

**Source**: Python source code analysis of all 12 provider directories in `tianji/llms/`. OCI SDK verified via Context7 (`oracle/oci-go-sdk/v65`, 61 code snippets, High reputation).

---

## Decision 6: CyberArk Secret Manager

**Decision**: Use `cyberark/conjur-api-go` (Conjur SDK)

**Rationale**:
- CyberArk has two products: Conjur (open-source secrets) and Ark (enterprise PAM)
- Conjur SDK is appropriate for our use case (simple secret retrieval)
- Ark SDK (`cyberark/ark-sdk-golang`) requires audit `Reason` and `TicketID` per retrieval — overkill for automated proxy
- Context7 confirmed Ark SDK exists but docs show it's designed for interactive privileged access, not automated secret retrieval

**Alternatives Considered**:
- CyberArk Ark SDK (`ark-sdk-golang`) — requires audit reason per retrieval; designed for interactive PAM, not automation
- Direct HTTP API — Conjur API is simple enough, but SDK handles auth token refresh
- HashiCorp Vault (`vault/api`) — already covered by Phase 2; different product

**Implementation Pattern**:
```go
type ConjurSecretManager struct {
    client *conjurapi.Client
}

func (c *ConjurSecretManager) Get(ctx context.Context, key string) (string, error) {
    // key: "conjur://path/to/secret"
    secretBytes, err := c.client.RetrieveSecret(variableID)
    return string(secretBytes), err
}
```

**Config format**:
```yaml
secret_manager:
  type: conjur
  conjur_account: myorg
  conjur_url: https://conjur.example.com
  conjur_login: host/tianji-proxy
  conjur_api_key: $CONJUR_API_KEY
```

---

## Decision 7: Object Storage Cache Backends

**Decision**: Implement S3/GCS/Azure Blob cache backends as requested in spec, but document performance tradeoffs

**Rationale**:
- Spec FR-091, FR-092, FR-093 explicitly require these backends
- Object storage latency (50-200ms) is 50-100x slower than Redis (1-5ms)
- Appropriate for: large response caching, compliance-required persistence, environments without Redis
- Not appropriate for: hot request caching, rate limiting, distributed locks

**Implementation Pattern**:
- All three backends implement existing `cache.Cache` interface (Get, Set, Delete, MGet)
- TTL via object metadata (`expires_at` field) — no native TTL in object storage
- Use official cloud SDKs:
  - S3: `github.com/aws/aws-sdk-go-v2/service/s3`
  - GCS: `cloud.google.com/go/storage`
  - Azure: `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`

**Key caveat**: Object storage cache should be documented as "cold cache" for large/infrequent data. Redis remains the recommended default for production hot caching.

---

## Decision 8: SCIM vs Assistants vs Policy — Package Location

**Decision**: New top-level packages under `internal/`

| Feature | Package | Rationale |
|---------|---------|-----------|
| Policy Engine | `internal/policy/` | Distinct domain; imported by handler, not part of router |
| SCIM 2.0 | `internal/scim/` | ResourceHandler implementations for elimity-com/scim library; server routing handled by library |
| Scheduler | `internal/scheduler/` | Cross-cutting concern used by main.go |
| Assistants | `internal/proxy/handler/assistants.go` | Thin pass-through; just another handler file |

**Rationale**: Policy was considered for `internal/router/` but rejected because:
1. Policy is about guardrail assignment, not request routing
2. Putting it in router would create import cycles (handler → router → guardrail → handler)
3. Separate package enables independent testing
