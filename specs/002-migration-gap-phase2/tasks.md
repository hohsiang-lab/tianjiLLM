# Tasks: TianjiLLM Go Migration Phase 2 — Enterprise Production Readiness

**Input**: Design documents from `/specs/002-migration-gap-phase2/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/interfaces.go, quickstart.md

**Organization**: Tasks grouped by user story. Each story is independently implementable and testable after Phase 2 (Foundational) completes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create new package directories and shared base types

- [x] T001 Create `internal/secretmanager/` package directory
- [x] T002 [P] Create `internal/prompt/` package directory
- [x] T003 [P] Create `test/contract/secretmanager/` test directory
- [x] T004 [P] Create `test/contract/guardrail/` test directory
- [x] T005 [P] Create `test/contract/callback/` test directory
- [x] T006 [P] Create `test/fixtures/secretmanager/` fixtures directory
- [x] T007 [P] Create `test/fixtures/guardrail/` fixtures directory
- [x] T008 [P] Create `test/fixtures/callback/` fixtures directory

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared infrastructure that MUST complete before any user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T009 Implement `BatchLogger` base struct with queue, mutex, flush ticker, and `flushFn` callback in `internal/callback/batchlogger.go`. Fields: `queue []LogData`, `mu sync.Mutex`, `batchSize int` (default 512), `flushTicker *time.Ticker` (default 5s), `stopCh chan struct{}`. Methods: `Start()`, `LogSuccess(data)`, `LogFailure(data)`, `flush()`, `Stop()`. On flush error: discard batch + log error. Env-configurable via `DEFAULT_BATCH_SIZE` and `DEFAULT_FLUSH_INTERVAL_SECONDS`
- [x] T010 [P] Add all Phase 2 config structs to `internal/config/config.go` in a single task: (a) `SecretManagerConfig` struct (Type, Region, ProjectID, VaultURL, CacheTTL) + add `SecretManager *SecretManagerConfig` to `GeneralSettings`. (b) `PromptManagementConfig` struct (Type, PublicKey, SecretKey, BaseURL) + add to `GeneralSettings`. (c) `CallbackConfig` struct (Type, Bucket, Prefix, FlushInterval, BatchSize, APIKey, Project, Entity) + extend `TianjiLLMSettings` to support structured callback config alongside existing string list. (d) Extend `CacheParams` with `Addrs []string` for Redis Cluster and `EmbeddingModel string` for semantic cache. (e) Add `FailurePolicy string` field (`fail_open`/`fail_closed`) to `GuardrailConfig` struct
- [x] T014 Write unit tests for `BatchLogger` in `internal/callback/batchlogger_test.go`: test flush on batch size, flush on ticker, discard on error, Stop() drains queue, concurrent LogSuccess/LogFailure

**Checkpoint**: Foundation ready — user story implementation can begin

---

## Phase 3: User Story 1 — Secret Management for Enterprise Deployment (Priority: P1) 🎯 MVP

**Goal**: Proxy retrieves all credentials from external secret managers. Zero secrets in env vars or config files.

**Independent Test**: Configure secret manager → start proxy → verify secrets resolved from vault, not env vars.

**FRs covered**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007

### Implementation for User Story 1

- [x] T015 [US1] Implement `SecretManager` interface + `Registry` + `CachedSecretManager` wrapper in `internal/secretmanager/secretmanager.go`. Interface: `Name() string`, `Get(ctx, path) (string, error)`, `Health(ctx) error`. Registry: `Register(name, factory)`, `Get(name) (SecretManager, error)`. CachedSecretManager: wraps any SecretManager with `map[string]cachedEntry`, `sync.RWMutex`, configurable TTL (default 86400s). Per constitution: follow Python's per-instance InMemoryCache, no cross-instance sync
- [x] T016 [P] [US1] Implement AWS Secrets Manager in `internal/secretmanager/aws.go`. Use `aws-sdk-go-v2/service/secretsmanager`. Auth: `config.LoadDefaultConfig()` with optional region override. `Get()`: call `GetSecretValue`, check both `SecretString` and `SecretBinary`. Self-register via `init()` → `Register("aws_secrets_manager", factory)`. Ref: Python `tianji/secret_managers/aws_secret_manager_v2.py`
- [x] T017 [P] [US1] Implement Google Secret Manager in `internal/secretmanager/google.go`. Use `cloud.google.com/go/secretmanager/apiv1`. Auth: ADC or `option.WithCredentialsFile()`. Path format: `projects/{project}/secrets/{name}/versions/latest`. Must `Close()` client. Self-register via `init()`. Ref: Python `tianji/secret_managers/google_secret_manager.py`
- [x] T018 [P] [US1] Implement Azure Key Vault in `internal/secretmanager/azure.go`. Use `azsecrets.NewClient()` with `azidentity.NewDefaultAzureCredential()`. VaultURL must not have trailing slash. Dereference `resp.Value` (*string). Self-register via `init()`. Ref: Python `tianji/secret_managers/get_azure_ad_token_provider.py`
- [x] T019 [P] [US1] Implement HashiCorp Vault in `internal/secretmanager/vault.go`. Use `github.com/hashicorp/vault/api`. Auth: token (`client.SetToken()`) or AppRole (`approle.NewAppRoleAuth()`). Use `client.KVv2()` helper to avoid `data.data` nesting. Self-register via `init()`. Ref: Python `tianji/secret_managers/hashicorp_secret_manager.py`
- [x] T020 [US1] Integrate SecretManager with config loader in `internal/config/loader.go`. Add `resolveSecrets(cfg *ProxyConfig, sm SecretManager)` function. Detect `os.environ/SECRET_NAME` syntax in all string fields (API keys, database URL, master key, cache password). Call `sm.Get(ctx, path)` for each. Call after `resolveEnvVars()` in `Load()`. If SecretManager configured but unreachable → return error with list of unresolved secrets
- [x] T021 [US1] Write contract tests for all 4 secret managers in `internal/secretmanager/secretmanager_test.go`. Use `httptest.NewServer` to mock AWS/Google/Azure/Vault HTTP APIs. Test: Get success, Get not found, Get with caching (second call hits cache), Health check, empty path rejection. Test CachedSecretManager TTL expiry

**Checkpoint**: Secret Management fully functional. Proxy can start with zero secrets in config.

---

## Phase 4: User Story 2 — Enterprise Guardrails for Compliance (Priority: P1)

**Goal**: Enterprise guardrail services integrated. Configurable fail-open/fail-closed policy.

**Independent Test**: Configure Bedrock guardrail → send violating content → verify blocked with violation details.

**FRs covered**: FR-008, FR-009, FR-010, FR-011, FR-012, FR-013, FR-014

### Implementation for User Story 2

- [x] T022 [P] [US2] Implement AWS Bedrock Guardrails in `internal/guardrail/bedrock.go`. Use `aws-sdk-go-v2/service/bedrockruntime` `ApplyGuardrail()`. Config: guardrailID, guardrailVersion, failOpen bool. Check `action == "GUARDRAIL_INTERVENED"` → block. Support PII redaction via ModifiedRequest. Self-register via `init()`. Ref: Python `tianji/proxy/guardrails/guardrail_hooks/bedrock_guardrails.py`
- [x] T023 [P] [US2] Implement Azure Text Moderation in `internal/guardrail/azure_text_mod.go`. Direct HTTP to `{endpoint}/contentsafety/text:analyze`. Auth: `Ocp-Apim-Subscription-Key` header. Parse `categoriesAnalysis[{category, severity}]`. Block if any severity > configured threshold. failOpen bool. Self-register via `init()`. Ref: Python `tianji/proxy/guardrails/guardrail_hooks/azure_content_safety.py`
- [x] T024 [P] [US2] Implement Azure Prompt Shield in `internal/guardrail/azure_prompt_shield.go`. Direct HTTP to `{endpoint}/contentsafety/text:shieldPrompt`. Check `userPromptAnalysis.attackDetected`. failOpen bool. Self-register via `init()`. Ref: same Python file as T023
- [x] T025 [P] [US2] Implement Lakera AI v2 in `internal/guardrail/lakera.go`. HTTP POST to `https://api.lakera.ai/v2/guard`. Auth: Bearer token. Check `flagged` field. Support `onFlagged: "block"|"monitor"`. failOpen bool. Self-register via `init()`. Ref: Python `tianji/proxy/guardrails/guardrail_hooks/lakera_v2.py`
- [x] T026 [P] [US2] Implement Generic Guardrail API in `internal/guardrail/generic.go`. Configurable endpoint URL + headers. Request: `{prompt, response, metadata}`. Response: `{action: "allow"|"block", message, modified_content}`. failOpen bool. Self-register via `init()`. Ref: Python `tianji/proxy/guardrails/guardrail_hooks/custom_guardrail.py`
- [x] T027 [P] [US2] Implement built-in Content Filter in `internal/guardrail/contentfilter.go`. Categories: violence, hate, self-harm, sexual. Each category has `*regexp.Regexp` patterns. Configurable severity threshold (0-3). No external API call. Self-register via `init()`
- [x] T028 [P] [US2] Implement Tool Permission guardrail in `internal/guardrail/toolpermission.go`. Config: `allowedTools map[string][]string` (key/team → tool names). Pre-call hook only: check `request.Tools` against allowed list. Block if unauthorized tool found. Self-register via `init()`
- [x] T029 [US2] Add fail-open/fail-closed support to `internal/guardrail/guardrail.go` `RunPreCall()` and `RunPostCall()`. DO NOT modify the existing `Guardrail` interface. Instead: (a) add a `GuardrailWithPolicy` wrapper struct that pairs a `Guardrail` with its `failOpen bool` from config. (b) In `RunPreCall()`/`RunPostCall()`, when `Run()` returns a non-BlockedError: if `failOpen=true` → log warning + continue; if `failOpen=false` → return error. (c) Registry stores `GuardrailWithPolicy` entries keyed by name. The `Guardrail` interface remains unchanged — fail-open/fail-closed is a config concern, not a guardrail implementation concern
- [x] T030 [US2] Write contract tests for all new guardrails in `internal/guardrail/guardrail_test.go`. Use `httptest.NewServer` to mock Bedrock/Azure/Lakera/Generic APIs. Test: block on violation, pass clean content, fail-open behavior, fail-closed behavior, PII redaction (Bedrock), tool permission deny/allow

**Checkpoint**: 9+ guardrails available. Enterprise compliance requirements met.

---

## Phase 5: User Story 3 — Cloud Storage Logging and Email Alerts (Priority: P2)

**Goal**: Durable log storage to S3/GCS/Azure Blob/DynamoDB/SQS. Email alerts for budget thresholds.

**Independent Test**: Configure S3 logger → send requests → verify JSON logs appear in S3 bucket within 60s.

**FRs covered**: FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, FR-021

### Implementation for User Story 3

- [x] T031 [P] [US3] Implement S3 Logger in `internal/callback/s3.go`. Embed `BatchLogger`. Use `aws-sdk-go-v2/service/s3` `PutObject()`. Key format: `{prefix}/{date}/{timestamp}-{uuid}.json`. Batch → JSON array → single S3 object. Self-register via `init()`. Ref: Python `tianji/integrations/s3.py`
- [x] T032 [P] [US3] Implement GCS Logger in `internal/callback/gcs.go`. Embed `BatchLogger`. Use `cloud.google.com/go/storage`. Must check error on `Writer.Close()` (that's the actual upload). Key format same as S3. Self-register via `init()`. Ref: Python `tianji/integrations/gcs_bucket.py`
- [x] T033 [P] [US3] Implement Azure Blob Logger in `internal/callback/azureblob.go`. Embed `BatchLogger`. Use `azblob.UploadBuffer()`. Self-register via `init()`. Ref: Python `tianji/integrations/azure_storage.py`
- [x] T034 [P] [US3] Implement DynamoDB Logger in `internal/callback/dynamodb.go`. Embed `BatchLogger`. Use `aws-sdk-go-v2/service/dynamodb` + `feature/dynamodb/attributevalue.MarshalMap()`. Use `BatchWriteItem` for batch flush (max 25 items per batch → split if needed). Self-register via `init()`. Ref: Python `tianji/integrations/dynamodb.py`
- [x] T035 [P] [US3] Implement SQS Logger in `internal/callback/sqs.go`. Embed `BatchLogger`. Use `aws-sdk-go-v2/service/sqs` `SendMessageBatch()` (max 10 messages per batch → split if needed). Needs Queue URL not ARN. Self-register via `init()`. Ref: Python `tianji/integrations/custom_batch_logger.py`
- [x] T036 [P] [US3] Implement Email Alerting in `internal/callback/email.go`. Use `net/smtp` + `html/template`. Config: SMTP host/port, from/to, TLS mode. Trigger on budget threshold breach events (not regular log data). Port 587 = STARTTLS, Port 465 = implicit TLS. Self-register via `init()`. Ref: Python `tianji/integrations/email_alerting.py`
- [x] T037 [US3] Write contract tests for cloud loggers in `internal/callback/callback_test.go`. Use `httptest.NewServer` to mock S3/GCS/Azure/DynamoDB/SQS APIs. Test: batch flush on size, batch flush on ticker, flush failure → discard + log, Stop() drains, email send on budget event

**Checkpoint**: Durable log storage operational. Audit trail requirements met.

---

## Phase 6: User Story 4 — Vertex AI and SageMaker Provider Support (Priority: P2)

**Goal**: Native Vertex AI and SageMaker support with platform-specific auth.

**Independent Test**: Configure Vertex AI model → send chat completion → verify Gemini format transformation + service account auth.

**FRs covered**: FR-022, FR-023, FR-024, FR-025 (FR-026 already implemented in Phase 1)

### Implementation for User Story 4

- [x] T038 [P] [US4] Implement Vertex AI provider in `internal/provider/vertexai/vertexai.go`. Use composition: embed `*gemini.Provider` as a field and delegate `TransformRequest`/`TransformResponse`/`TransformStreamChunk`/`GetSupportedParams`/`MapParams` to it. Override `GetRequestURL()` (Vertex AI regional endpoint `{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/google/models/{model}:generateContent`) and `SetupHeaders()` (service account OAuth2 token via ADC). Implement `Provider` interface (7 methods). Self-register `"vertex_ai"` via `init()`. Ref: Python `tianji/llms/vertex_ai/`
- [x] T039 [P] [US4] Implement Vertex AI streaming (delegated to Gemini — same SSE format) in `internal/provider/vertexai/stream.go`. Reuse Gemini stream chunk transformation. Vertex AI uses same SSE format as Gemini
- [x] T040 [P] [US4] Implement SageMaker provider in `internal/provider/sagemaker/sagemaker.go`. Use `aws-sdk-go-v2/service/sagemakerruntime` `InvokeEndpoint()`. Config: endpoint name, region. Auth: `config.LoadDefaultConfig()` with SigV4 signing (handled by SDK). Transform request to SageMaker JSON format, transform response back to OpenAI format. Self-register `"sagemaker"` via `init()`. Ref: Python `tianji/llms/sagemaker/`
- [x] T041 [P] [US4] Implement AI21 provider in `internal/provider/ai21/ai21.go`. Native API format. Self-register `"ai21"` via `init()`. Ref: Python `tianji/llms/ai21/`
- [x] T042 [P] [US4] Implement WatsonX provider in `internal/provider/watsonx/watsonx.go`. IBM Cloud IAM auth. Self-register `"watsonx"` via `init()`. Ref: Python `tianji/llms/watsonx/`
- [x] T043 [US4] Write contract tests for Vertex AI and SageMaker in `internal/provider/vertexai/vertexai_test.go` and `internal/provider/sagemaker/sagemaker_test.go`. Use `httptest.NewServer` to mock upstream. Test: request transform, response transform, stream chunks, auth header presence, error handling

**Checkpoint**: 24+ providers available. GCP and AWS managed AI services supported.

---

## Phase 7: User Story 5 — Additional Observability Integrations (Priority: P2)

**Goal**: Native integrations with popular LLM observability platforms.

**Independent Test**: Configure Langsmith → send request → verify trace data posted to Langsmith API.

**FRs covered**: FR-027, FR-028, FR-029, FR-030, FR-031, FR-032

### Implementation for User Story 5

- [x] T044 [P] [US5] Implement Langsmith callback in `internal/callback/langsmith.go`. Embed `BatchLogger`. HTTP POST to `{baseURL}/runs/batch` (default `https://api.smith.langchain.com`). Auth: `x-api-key` header. Payload: runs array with model, tokens, cost, latency, metadata. Self-register via `init()`. Ref: Python `tianji/integrations/langsmith.py`
- [x] T045 [P] [US5] Implement Helicone callback in `internal/callback/helicone.go`. Implements `CustomLogger` interface (`LogSuccess`/`LogFailure`). Following Python: on `LogSuccess`, HTTP POST to `{api_base}/oai/v1/log` (or `/anthropic/v1/log` for Claude models) with `providerRequest` (url, json, meta with `Helicone-Auth` header), `providerResponse` (json, status), and `timing` (startTime, endTime in seconds+milliseconds). Auth: `Authorization: Bearer {HELICONE_API_KEY}`. Forward `helicone_*` metadata from proxy request headers. Self-register via `init()`. Ref: Python `tianji/integrations/helicone.py`
- [x] T046 [P] [US5] Implement Braintrust callback in `internal/callback/braintrust.go`. Embed `BatchLogger`. HTTP API to Braintrust. Self-register via `init()`. Ref: Python `tianji/integrations/braintrust_logging.py`
- [x] T047 [P] [US5] Implement Arize/Phoenix callback in `internal/callback/arize.go`. Use existing OTEL exporter (`go.opentelemetry.io/otel`) with custom endpoint configuration pointing to Arize collector. Self-register via `init()`. Ref: Python `tianji/integrations/arize_ai.py`
- [x] T048 [P] [US5] Implement MLflow callback in `internal/callback/mlflow.go`. HTTP API: `POST /api/2.0/mlflow/runs/create` → `POST /api/2.0/mlflow/runs/log-batch` → `POST /api/2.0/mlflow/runs/update`. Config: tracking URI, experiment ID. Self-register via `init()`. Ref: Python `tianji/integrations/mlflow.py`
- [x] T049 [P] [US5] Implement Weights & Biases callback in `internal/callback/wandb.go`. Embed `BatchLogger`. HTTP API. Config: API key, project, entity. Self-register via `init()`. Ref: Python `tianji/integrations/weights_biases.py`
- [x] T050 [US5] Write contract tests for observability callbacks in `internal/callback/observability_test.go`. Use `httptest.NewServer` to mock each API. Test: Langsmith batch post, Helicone log post (verify providerRequest/providerResponse/timing payload structure), MLflow 3-step lifecycle, W&B/Braintrust batch send

**Checkpoint**: 15+ callbacks available. Major observability platforms integrated.

---

## Phase 8: User Story 6 — Redis Cluster and Advanced Cache (Priority: P3)

**Goal**: HA cache via Redis Cluster. Semantic cache for cost reduction.

**Independent Test**: Configure Redis Cluster → verify cache ops distribute across nodes.

**FRs covered**: FR-033, FR-034, FR-035

### Implementation for User Story 6

- [x] T051 [P] [US6] Implement Redis Cluster cache in `internal/cache/redis_cluster.go`. Use `redis.NewClusterClient()` from existing `go-redis/v9`. Same `Cache` interface (Get/Set/Delete/MGet). Config: `addrs []string`, password, read-only replicas. Note: MGet across slots is NOT atomic — SDK splits automatically. Self-register in cache factory. Ref: research.md TD-005
- [x] T052 [P] [US6] Implement Semantic Cache in `internal/cache/semantic.go`. Requires Redis Stack with `FT.SEARCH`. Flow: embed query → `HSET cache:{hash} embedding <bytes> response <json>` → `FT.SEARCH idx:cache "*=>[KNN 1 @embedding $vec AS score]"`. Config: `embedding_model` (user-configured model name, routed through the existing embedding handler to generate vectors — e.g., `"text-embedding-3-small"`), cosine distance threshold (default 0.1). Vector serialization: `[]float32` → `binary.LittleEndian` → `[]byte`. Each cache lookup requires one embedding API call — this adds latency proportional to embedding model response time. Ref: research.md TD-011
- [x] T053 [P] [US6] Implement Disk Cache in `internal/cache/disk.go`. File-based cache for local development. Key → filename (SHA256 hash), value → file contents. TTL via file mtime check. Same `Cache` interface
- [x] T054 [US6] Write tests for Redis Cluster and Semantic Cache in `internal/cache/cache_test.go`. Test: cluster Get/Set/Delete, cluster MGet across slots, semantic cache embed+store+retrieve, disk cache CRUD + TTL expiry

**Checkpoint**: 5+ cache backends. HA and semantic cache operational.

---

## Phase 9: User Story 7 — Prompt Management Integration (Priority: P3)

**Goal**: Fetch versioned prompt templates from external services.

**Independent Test**: Configure Langfuse prompt → send request with prompt reference → verify template resolved.

**FRs covered**: FR-036, FR-037

### Implementation for User Story 7

- [x] T055 [US7] Implement `PromptSource` interface + `Registry` in `internal/prompt/prompt.go`. Interface: `Name() string`, `GetPrompt(ctx, promptID, opts) (*ResolvedPrompt, error)`. PromptOptions: `Version *int`, `Label *string`, `Variables map[string]string`. ResolvedPrompt: `Messages []model.Message`, `Metadata map[string]string`. Registry: `Register(name, source)`, `Get(name) (PromptSource, error)`
- [x] T056 [US7] Implement Langfuse prompt source in `internal/prompt/langfuse.go`. HTTP client to Langfuse API. Auth: Basic (publicKey:secretKey). Fetch prompt by ID + version/label. Cache with configurable TTL. Template variable substitution (`{{variable}}`). Self-register via `init()`. Ref: Python `tianji/integrations/langfuse/langfuse_prompt_management.py`
- [x] T057 [US7] Write contract tests for prompt management in `internal/prompt/prompt_test.go`. Mock Langfuse HTTP API. Test: fetch by ID, fetch by version, fetch by label, variable substitution, cache hit, service unavailable fallback

**Checkpoint**: Prompt management operational. Non-engineers can iterate prompts without deployments.

---

## Phase 10: User Story 8 — Advanced Routing Strategies (Priority: P3)

**Goal**: Least-busy and budget-limited routing for high-throughput deployments.

**Independent Test**: Configure least-busy strategy → send concurrent requests → verify lower-queue deployments get more traffic.

**FRs covered**: FR-038, FR-039

### Implementation for User Story 8

- [x] T058 [P] [US8] Implement Least Busy strategy in `internal/router/strategy/leastbusy.go`. Satisfies existing `Strategy` interface (`Pick(deployments) *Deployment`) — no interface change. Track in-flight requests per deployment via `map[string]*atomic.Int64`. `Pick()`: return deployment with lowest in-flight count. Additional methods `Acquire(deploymentID string)` and `Release(deploymentID string)` are called by the router via type assertion (`if lb, ok := strategy.(*LeastBusy); ok { lb.Acquire(id) }`). Router calls `Acquire()` before sending to provider and `Release()` in defer after response. This follows Python TianjiLLM's `get_available_deployment` pattern where least-busy is tracked at router level
- [x] T059 [P] [US8] Implement Budget Limiter strategy in `internal/router/strategy/budgetlimiter.go`. Satisfies existing `Strategy` interface — no interface change. Config: `budgets map[string]float64` (provider → max budget), wraps an inner `Strategy` for selection. `Pick()`: query current spend from `internal/spend/tracker.go` `GetProviderSpend()`, filter out deployments whose provider spend >= budget, delegate remaining to inner strategy. If all exhausted → return nil. Budget period resets per config (daily/monthly). This follows Python TianjiLLM's `router_budget_config` pattern where spend is read from the existing tracker, not maintained separately
- [x] T060 [US8] Write tests for routing strategies in `internal/router/strategy/leastbusy_test.go` and `internal/router/strategy/budgetlimiter_test.go`. Test: least-busy picks lowest count, concurrent acquire/release, budget limiter excludes exhausted providers, budget reset

**Checkpoint**: 7 routing strategies. Performance and cost optimization available.

---

## Phase 11: User Story 9 — Advanced Analytics (Priority: P3)

**Goal**: Spend trend analysis, top-N queries, FinOps FOCUS export.

**Independent Test**: Query spend analytics API → verify grouped results within 5s for large dataset.

**FRs covered**: FR-040, FR-041, FR-042

### Implementation for User Story 9

- [x] T061 [P] [US9] Implement advanced analytics queries in `internal/spend/analytics.go`. Types: `AnalyticsQuery{GroupBy, StartDate, EndDate, TopN}`, `AnalyticsResult{Groups, Total}`, `SpendGroup{Name, Spend, Count}`. Functions: `QueryByGroup(ctx, db, query)`, `QueryTopN(ctx, db, query)`, `QueryTrend(ctx, db, query)`. Use existing pgx pool for SQL queries. Group by: team, tag, model, user, key
- [x] T062 [P] [US9] Implement FinOps FOCUS export in `internal/spend/focus.go`. Export spend data in FOCUS 1.2 format to cloud storage. Map fields: BilledCost, EffectiveCost, Provider, Service, ResourceID, UsageQuantity. Output: JSON (Parquet deferred to implementation phase per TD-010)
- [x] T063 [US9] Write tests for analytics in `internal/spend/analytics_test.go`. Test: group by team, group by model, top-N query, date range filtering, empty result handling

**Checkpoint**: Advanced analytics operational. FinOps reporting available.

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Wire everything together, integration validation

- [x] T064 Wire all new secret manager, guardrail, callback, cache, prompt, and router registrations in `cmd/tianji/main.go` or appropriate startup code. Ensure config-driven initialization: read config → create instances → register
- [x] T065 [P] Validate all new config structs (from T010) are correctly wired: SecretManagerConfig, PromptManagementConfig, CallbackConfig, CacheParams extensions, GuardrailConfig.FailurePolicy. Write config parsing test in `internal/config/config_test.go` with a sample YAML containing all new sections
- [x] T066 [P] Add callback config factory in `internal/callback/registry.go` or new `internal/callback/factory.go` — create callback instances from `CallbackConfig` structs
- [x] T067 [P] Add cache factory support for `redis_cluster`, `semantic`, `disk` types in cache initialization code
- [x] T068 [P] Add router strategy registration for `least-busy` and `budget-limited` in router initialization
- [x] T069 [P] Add spend analytics HTTP handler endpoints in `internal/proxy/handler/` for trend, top-N, and group-by queries
- [x] T070 [P] Validate SC-002: benchmark guardrail latency — each guardrail must add <50ms. Write benchmark test in `internal/guardrail/benchmark_test.go` using `testing.B`. Test with mock upstream returning in <1ms, measure added latency per guardrail
- [x] T071 [P] Validate SC-007: benchmark prompt resolution — cached templates <200ms, uncached <2s. Write benchmark test in `internal/prompt/benchmark_test.go`
- [x] T072 [P] Validate SC-008: benchmark spend analytics — queries return <5s for 10M rows. Write benchmark test in `internal/spend/benchmark_test.go` with mock DB returning large result sets
- [x] T073 [P] Validate SC-006: verify all new integrations follow self-registration pattern — grep for `init()` + `Register()` in all new files, ensure zero changes needed to existing code when adding a new integration
- [x] T074 [P] Validate edge cases from spec.md: (a) secret manager returns empty/malformed secret → error with path, (b) guardrail returns unexpected format → follow fail-open/fail-closed, (c) Redis Cluster failover → retry once then cache miss, (d) prompt variables mismatch → reject with missing variable list. Write integration tests in `test/integration/edge_cases_test.go`
- [x] T075 Run `make check` (lint + test + build) — fix any issues
- [x] T076 Run quickstart.md validation: verify each round's test commands pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 Secret Managers (Phase 3)**: Depends on Phase 2 (config structs)
- **US2 Guardrails (Phase 4)**: Depends on Phase 2 only — parallel with US1
- **US3 Cloud Logging (Phase 5)**: Depends on Phase 2 (BatchLogger) — parallel with US1/US2
- **US4 Providers (Phase 6)**: Depends on Phase 2 only — parallel with US1-US3
- **US5 Observability (Phase 7)**: Depends on Phase 2 (BatchLogger) — parallel with US1-US4
- **US6 Cache (Phase 8)**: Depends on Phase 2 (config) — parallel with US1-US5
- **US7 Prompt Mgmt (Phase 9)**: Depends on Phase 2 (config) — parallel with US1-US6
- **US8 Routing (Phase 10)**: Depends on Phase 2 only — parallel with US1-US7
- **US9 Analytics (Phase 11)**: Depends on Phase 2 only — parallel with US1-US8
- **Polish (Phase 12)**: Depends on ALL user stories complete

### User Story Dependencies

```
Phase 1: Setup
    ↓
Phase 2: Foundational (BatchLogger, Config structs)
    ↓
    ├── US1: Secret Managers (P1) ─────────────────────┐
    ├── US2: Guardrails (P1)                           │
    ├── US3: Cloud Logging (P2) ← depends on BatchLogger│
    ├── US4: Providers (P2)                            │ All can run
    ├── US5: Observability (P2) ← depends on BatchLogger│ in parallel
    ├── US6: Cache (P3)                                │
    ├── US7: Prompt Mgmt (P3)                          │
    ├── US8: Routing (P3)                              │
    └── US9: Analytics (P3) ──────────────────────────┘
                                                       ↓
                                                  Phase 12: Polish
```

### Within Each User Story

- Interface/types before implementations
- Implementations (marked [P]) can run in parallel
- Integration/wiring after all implementations
- Tests after implementation (or TDD if preferred)

### Parallel Opportunities

**Maximum parallelism after Phase 2**: All 9 user stories can proceed simultaneously.

**Within each story**: All implementations marked [P] can run in parallel (different files, no dependencies).

Example for User Story 1:
```
T016 (AWS) ──┐
T017 (GCP) ──┤
T018 (Azure)─┤── all [P], different files
T019 (Vault)─┘
      ↓
T020 (config integration) ── depends on T015-T019
      ↓
T021 (tests) ── depends on T015-T020
```

Example for User Story 3:
```
T031 (S3)  ──────┐
T032 (GCS) ──────┤
T033 (Azure Blob)┤
T034 (DynamoDB) ─┤── all [P], different files
T035 (SQS) ──────┤
T036 (Email) ────┘
      ↓
T037 (tests) ── depends on T031-T036
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (BatchLogger, Config structs)
3. Complete Phase 3: US1 — Secret Managers
4. **STOP and VALIDATE**: `go test ./internal/secretmanager/... -v`
5. Enterprise customers can now deploy with secret manager integration

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (Secret Managers) → Deploy for enterprise security compliance (MVP!)
3. US2 (Guardrails) → Deploy for regulated industries
4. US3 (Cloud Logging) → Deploy for audit trail requirements
5. US4-US5 (Providers + Observability) → Broader platform support
6. US6-US9 (Cache + Prompt + Routing + Analytics) → Full feature parity
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Dev A: US1 (Secret Managers) + US2 (Guardrails) — P0 critical path
   - Dev B: US3 (Cloud Logging) + US5 (Observability) — both use BatchLogger
   - Dev C: US4 (Providers) — independent, no shared dependencies
   - Dev D: US6-US9 (Cache + Prompt + Routing + Analytics) — P3 features
3. Stories integrate independently via self-registration pattern

---

## Summary

| Metric | Count |
|--------|-------|
| Total tasks | 73 |
| Phase 1 (Setup) | 8 |
| Phase 2 (Foundational) | 3 (T009 BatchLogger, T010 all config structs, T014 tests) |
| US1 (Secret Managers) | 7 |
| US2 (Guardrails) | 9 |
| US3 (Cloud Logging) | 7 |
| US4 (Providers) | 6 |
| US5 (Observability) | 7 |
| US6 (Cache) | 4 |
| US7 (Prompt Mgmt) | 3 |
| US8 (Routing) | 3 |
| US9 (Analytics) | 3 |
| Phase 12 (Polish) | 13 (incl. SC validation benchmarks + edge case tests) |
| Parallel opportunities | 59 tasks marked [P] |
| User stories parallelizable | All 9 (after Phase 2) |
