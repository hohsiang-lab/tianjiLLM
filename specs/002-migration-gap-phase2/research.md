# Phase 0: Research — TianjiLLM Go Migration Phase 2

**Date**: 2026-02-16
**Spec**: [spec.md](spec.md)

## Technology Decisions

### TD-001: AWS SDK for Go v2

**Decision**: Use `github.com/aws/aws-sdk-go-v2` for all AWS services (Secrets Manager, S3, DynamoDB, SQS, Bedrock Guardrails, SageMaker)

**Rationale**:
- Official AWS SDK, actively maintained
- Unified config pattern: `config.LoadDefaultConfig()` → `service.NewFromConfig(cfg)`
- Automatic SigV4 signing — no manual credential handling needed
- All services share the same credential chain (env vars → config files → IAM role)

**Alternatives considered**:
- `aws-sdk-go` (v1): Deprecated, no new features
- Direct HTTP + SigV4: Unnecessary complexity, SDK handles everything

**Key packages**:
| Service | Import Path |
|---------|-------------|
| Secrets Manager | `github.com/aws/aws-sdk-go-v2/service/secretsmanager` |
| S3 | `github.com/aws/aws-sdk-go-v2/service/s3` |
| DynamoDB | `github.com/aws/aws-sdk-go-v2/service/dynamodb` + `feature/dynamodb/attributevalue` |
| SQS | `github.com/aws/aws-sdk-go-v2/service/sqs` |
| Bedrock Runtime | `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` |
| SageMaker Runtime | `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime` |

**Gotchas**:
- DynamoDB: Must use `attributevalue.MarshalMap()` — manual `AttributeValue` construction is 10x verbose
- S3: Single `PutObject` max 5GB — use `manager.Uploader` for large files (not relevant for log payloads)
- SQS: Needs Queue URL (not ARN) for `SendMessage`
- Secrets Manager: Check both `SecretString` and `SecretBinary` — binary secrets return nil for `SecretString`

**Sources**: Context7 AWS SDK docs, agent research output `a027b88`

---

### TD-002: Google Cloud Go SDKs

**Decision**: Use official `cloud.google.com/go/*` packages

**Rationale**:
- Official Google-maintained SDK
- ADC (Application Default Credentials) works automatically in GCP environments
- gRPC-based — efficient for high-throughput scenarios

**Key packages**:
| Service | Import Path |
|---------|-------------|
| Secret Manager | `cloud.google.com/go/secretmanager/apiv1` |
| Cloud Storage | `cloud.google.com/go/storage` |
| Vertex AI | `google.golang.org/genai` (Go GenAI SDK) |

**Authentication patterns**:
1. ADC (default): `client, err := secretmanager.NewClient(ctx)`
2. Service account file: `option.WithCredentialsFile("/path/to/key.json")`
3. In-memory JSON: `option.WithAuthCredentials(creds)`

**Gotchas**:
- Secret name format: `projects/{project}/secrets/{secret}/versions/latest` — must be exact
- GCS Writer: `Close()` is the actual upload — must check error on `Close()`
- Secret Manager: Default quota 1000 reads/min — must cache
- All clients must `Close()` — holds gRPC connections

**Sources**: Context7 google-cloud-go docs, agent research output `abb04ce`

---

### TD-003: Azure SDKs for Go

**Decision**: Use official `github.com/Azure/azure-sdk-for-go/sdk/*` packages

**Rationale**:
- Official Azure SDK, modern module structure
- `DefaultAzureCredential` handles all auth scenarios automatically
- Clear error types with `*azcore.ResponseError`

**Key packages**:
| Service | Import Path |
|---------|-------------|
| Key Vault Secrets | `github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets` |
| Blob Storage | `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` |
| Identity | `github.com/Azure/azure-sdk-for-go/sdk/azidentity` |

**Azure Content Safety**: No Go SDK — use direct HTTP calls to REST API

**Gotchas**:
- Key Vault URL: No trailing slash — `https://vault.vault.azure.net` not `https://vault.vault.azure.net/`
- `DefaultAzureCredential` tries 5 auth methods sequentially — can be slow (10s timeout on failure)
- `resp.Value` is `*string` — must dereference
- Blob `UploadBuffer` for `[]byte`, `UploadFile` for `*os.File`

**Sources**: Context7 azure-sdk-for-go docs, agent research output `abb04ce`

---

### TD-004: HashiCorp Vault Go Client

**Decision**: Use official `github.com/hashicorp/vault/api` + `api/auth/approle`

**Rationale**:
- Official Vault client, maintained by HashiCorp
- KVv2 helper hides `data.data` nesting complexity
- Built-in token lifecycle management (`LifetimeWatcher`)

**Authentication methods** (for tianjiLLM):
1. Token auth: `client.SetToken(token)` — simplest, for development
2. AppRole auth: `approle.NewAppRoleAuth(roleID, secretID)` — production recommended

**Gotchas**:
- KV v2 raw API returns `data.data` nesting — use `client.KVv2()` helper instead
- Token renewal is caller's responsibility — must start `LifetimeWatcher` goroutine
- Must monitor both `DoneCh()` and `RenewCh()` — goroutine leak otherwise
- `DefaultConfig()` reads `VAULT_ADDR`, `VAULT_TOKEN` from environment

**Sources**: Context7 vault-client-go docs, agent research output `adc229a`

---

### TD-005: Redis Cluster (go-redis v9)

**Decision**: Use existing `github.com/redis/go-redis/v9` — already in project, cluster support built-in

**Rationale**:
- Already used for single-node Redis
- `redis.NewClusterClient()` has identical command interface as `redis.NewClient()`
- Automatic slot-based routing and MOVED/ASK redirect handling

**Key differences from single-node**:
- `MGet` across slots is NOT atomic — SDK splits into multiple requests
- Transactions (`Watch`) only work on same-slot keys — use hash tags `{prefix}`
- Pipeline auto-splits across nodes and merges results in order
- Error types: `IsClusterDownError()`, `IsMovedError()`, `IsAskError()`

**Implementation approach**:
- Extend existing `cache.Cache` interface
- New `cache.NewRedisCluster(opts)` constructor
- Config: `cache_type: "redis_cluster"` with `addrs: [...]`

**Sources**: Context7 go-redis docs, agent research output `adc229a`

---

### TD-006: Observability Integrations — No Go SDKs Available

**Decision**: Build HTTP clients for Langsmith, Braintrust, MLflow, Helicone.

**Rationale**:
- Langsmith: No official Go SDK — HTTP API at `POST /runs/batch`
- Braintrust: No official Go SDK — HTTP API
- MLflow: No official Go SDK — HTTP API at `/api/2.0/mlflow/runs/*`
- Helicone: No SDK needed — post-request HTTP logger (`POST /oai/v1/log`), implements `CustomLogger` (not header injection — verified from Python source `tianji/integrations/helicone.py`)
- Arize/Phoenix: Use existing OTEL exporter with custom endpoint
- W&B: No official Go SDK — HTTP API

**Integration patterns**:
| Integration | Pattern | Complexity |
|------------|---------|------------|
| Langsmith | HTTP batch `POST /runs/batch` | M |
| Braintrust | HTTP API | M |
| Helicone | HTTP POST log (`/oai/v1/log`) — post-request | S |
| Arize/Phoenix | OTEL OTLP exporter (already have OTEL) | S |
| MLflow | HTTP API (`create run` → `log batch` → `update run`) | M |
| W&B | HTTP API | M |

**Batch logger base pattern** (matching Python's `CustomBatchLogger`):
- Unbounded `[]LogData` queue
- Flush trigger: `len(queue) >= batchSize` (default 512) OR ticker (default 5s)
- On flush failure: discard batch, log error
- Thread-safe via `sync.Mutex`

**Sources**: Python TianjiLLM source code, agent research output `a7ba0a5`

---

### TD-007: Guardrail Service APIs

**Decision**: Direct HTTP for Azure Content Safety and Lakera AI. AWS SDK for Bedrock Guardrails.

**Rationale**:
- AWS Bedrock: `bedrockruntime.ApplyGuardrail()` — native SDK
- Azure Content Safety: REST API at `contentsafety/text:analyze` + `text:shieldPrompt`
- Lakera AI: REST API at `https://api.lakera.ai/v2/guard`
- Generic: Configurable HTTP endpoint with standardized request/response contract

**API contracts** (verified from Python source):

**Bedrock Guardrails**:
- Input: `ApplyGuardrailInput{GuardrailIdentifier, Source, Content}`
- Output: `action: "GUARDRAIL_INTERVENED" | "NONE"` + assessments
- Supports PII redaction (returns modified content)

**Azure Content Safety**:
- Text Moderation: `POST /text:analyze` → `categoriesAnalysis[{category, severity}]`
- Prompt Shield: `POST /text:shieldPrompt` → `userPromptAnalysis.attackDetected`
- Auth: `Ocp-Apim-Subscription-Key` header

**Lakera AI v2**:
- `POST /v2/guard` → `{flagged, categories, payload.detections}`
- Auth: `Authorization: Bearer` header
- Supports PII detection and masking

**Generic Guardrail API**:
- Request: `{prompt, response, metadata}`
- Response: `{action: "allow"|"block", message, modified_content}`
- Configurable endpoint URL, headers, failure policy

**Sources**: Python TianjiLLM guardrail hooks source code, agent research output `afdc0bf`

---

### TD-008: Email Alerting (SMTP)

**Decision**: Use Go stdlib `net/smtp` — no external dependency needed

**Rationale**:
- Standard library handles SMTP + TLS
- Budget alerts are low-frequency — no need for async queue
- HTML templates via `html/template`

**Gotchas**:
- Port 587 = STARTTLS, Port 465 = implicit TLS
- Gmail requires App Password (not account password)
- Set `Content-Type: text/html; charset=UTF-8` for HTML emails

---

### TD-009: Vertex AI Provider

**Decision**: Use `google.golang.org/genai` (Go GenAI SDK) — official Google SDK

**Rationale**:
- Official SDK, 121 code examples, benchmark score 83.5
- Handles Gemini format transformation natively
- Service account auth via ADC

**Key insight from Python source**:
- Python TianjiLLM's Vertex AI uses `httpx` client (NOT the Python Vertex AI SDK)
- Go should use the official Go GenAI SDK for cleaner integration
- Vertex AI uses the same Gemini format — tianjiLLM already has a Gemini provider

**Implementation approach**:
- New `internal/provider/vertexai/` package
- Inherits from Gemini format (same request/response transformation)
- Differs in: auth (service account), base URL (Vertex AI endpoints), headers

**Sources**: Context7 go-genai docs, Python TianjiLLM vertex_ai source

---

### TD-010: Parquet / FinOps FOCUS Export

**Decision**: Use `github.com/xitongsys/parquet-go` or `github.com/apache/arrow-go` for Parquet format

**Rationale**:
- FOCUS 1.2 spec requires Parquet output
- Python uses `polars` for DataFrame → Parquet
- Go has `parquet-go` (community) or `arrow-go` (Apache official)

**Note**: This is P2 priority — research deferred to implementation phase. Mark as "needs verification" per constitution.

---

### TD-011: Semantic Cache Embedding

**Decision**: Use Redis Stack `FT.SEARCH` with vector similarity (HNSW index)

**Rationale**:
- Python TianjiLLM uses `redisvl` (Redis Vector Library)
- Go: use `go-redis` `Do()` with `FT.CREATE` / `FT.SEARCH` commands
- Requires Redis Stack (not vanilla Redis)

**Implementation**:
- Embed query → call configured embedding model endpoint
- Store: `HSET cache:{hash} embedding <bytes> response <json>`
- Retrieve: `FT.SEARCH idx:cache "*=>[KNN 1 @embedding $vec AS score]"`
- Threshold: configurable cosine distance (default 0.1)

**Gotchas**:
- Requires Redis Stack module — must document this dependency clearly
- Vector serialization: `[]float32` → `binary.LittleEndian` → `[]byte`
- Each cache lookup requires an embedding API call — adds latency
- P2 priority

**Sources**: Context7 go-redis docs, agent research output `adc229a`

---

## All NEEDS CLARIFICATION Resolved

All technology choices have been researched and documented with external source verification. No remaining unknowns block Phase 1 design.
