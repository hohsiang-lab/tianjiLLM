# Feature Specification: TianjiLLM Go Migration Phase 2 — Enterprise Production Readiness

**Feature Branch**: `002-migration-gap-phase2`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "Phase 2 migration gap analysis: identify all remaining Python TianjiLLM features not yet ported to tianjiLLM"

## Context

Phase 1 (001-migration-gap-analysis) completed 99 tasks and delivered:
- 20 LLM providers + openaicompat factory
- Core LLM APIs (chat, completion, embedding, images, audio, moderation, rerank, responses)
- Pass-through for files/batches/fine-tuning
- Management APIs (key, team, user, org, budget, credentials, access groups)
- 7 callbacks (Prometheus, OTEL, Datadog, Langfuse, webhook, Slack, callback interface)
- 3 guardrails (OpenAI moderation, Presidio PII, prompt injection)
- 3 cache backends (memory, Redis, dual)
- 5 router strategies (shuffle, latency, cost, usage, tag)
- SSO (Google, Microsoft, Generic OIDC), RBAC, JWT auth

This Phase 2 spec addresses the remaining gaps that block enterprise production adoption.

## Clarifications

### Session 2026-02-16

- Q: Cloud logger buffer overflow strategy — bounded queue or unbounded list? → A: Follow Python TianjiLLM — unbounded `List` with `batch_size` (default 512) triggering flush + periodic flush (default 5s). On flush failure, discard entire batch and log error. No retry, no max queue size. Env-configurable via `DEFAULT_BATCH_SIZE` and `DEFAULT_FLUSH_INTERVAL_SECONDS`.
- Q: Cloud logger payload content — include full request/response body or metadata only? → A: Follow Python TianjiLLM — `StandardLoggingPayload` includes complete request/response body. PII risk mitigated by existing Presidio guardrail applied before logging. No separate redaction at logger level.
- Q: Secret manager cache synchronization across multiple proxy instances? → A: Follow Python TianjiLLM — each instance maintains independent InMemoryCache with configurable TTL (default 86400s for Google/Vault). No cross-instance cache sync. During secret rotation, instances may serve stale credentials for up to one TTL window.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Secret Management for Enterprise Deployment (Priority: P1)

As a platform engineer deploying tianjiLLM in a regulated enterprise environment, I want the proxy to retrieve API keys and credentials from my organization's secret management service (AWS Secrets Manager, Google Secret Manager, Azure Key Vault, or HashiCorp Vault) so that I never need to put secrets in environment variables or config files.

**Why this priority**: This is the single largest blocker for enterprise adoption. No enterprise security team will approve a production deployment that requires API keys in environment variables or plaintext config files. Secret rotation, audit trails, and centralized secret management are hard requirements for SOC2/ISO27001/HIPAA compliance.

**Independent Test**: Can be fully tested by configuring a secret manager backend and verifying that the proxy resolves API keys at startup and runtime without any secrets in the config file.

**Acceptance Scenarios**:

1. **Given** a proxy config referencing secrets via `os.environ/SECRET_NAME` syntax and a configured secret manager, **When** the proxy starts, **Then** it resolves all secrets from the configured secret manager before establishing provider connections
2. **Given** a secret is rotated in the secret manager, **When** the proxy refreshes its credential cache, **Then** subsequent requests use the new secret without restart
3. **Given** the configured secret manager is unreachable at startup, **When** the proxy starts, **Then** it fails with a clear error message identifying which secrets could not be resolved
4. **Given** multiple secret managers are configured (e.g., AWS for provider keys, Vault for internal keys), **When** the proxy resolves secrets, **Then** it routes each secret reference to the correct manager based on prefix

---

### User Story 2 - Enterprise Guardrails for Compliance (Priority: P1)

As a compliance officer at a regulated organization, I want tianjiLLM to integrate with enterprise guardrail services (AWS Bedrock Guardrails, Azure Content Safety, Lakera AI) and support a generic guardrail API so that I can enforce content policies mandated by our industry regulations.

**Why this priority**: Regulated industries (finance, healthcare, government) require auditable content safety enforcement. The 3 existing guardrails (OpenAI mod, Presidio, prompt injection) are insufficient for organizations that mandate specific vendor guardrail services.

**Independent Test**: Can be fully tested by configuring each guardrail integration and verifying that violating content is blocked or redacted according to the configured policy.

**Acceptance Scenarios**:

1. **Given** AWS Bedrock Guardrails are configured with a guardrail ID, **When** a request contains content that violates the guardrail policy, **Then** the proxy blocks the request and returns a structured error with the guardrail's violation details
2. **Given** a generic guardrail API endpoint is configured, **When** a request is processed, **Then** the proxy calls the external guardrail service with the request/response payload and respects the service's allow/block decision
3. **Given** the built-in content filter is configured with custom categories, **When** a request contains content matching a category pattern, **Then** the proxy blocks the request with a category-specific error message
4. **Given** a guardrail service is unavailable, **When** a request is processed, **Then** the proxy follows the configured failure policy (fail-open or fail-closed) and logs the guardrail failure

---

### User Story 3 - Cloud Storage Logging and Email Alerts (Priority: P2)

As a platform operations engineer, I want tianjiLLM to export structured logs to cloud storage (S3, GCS, Azure Blob) and send email alerts for budget thresholds so that I have durable audit trails and proactive cost controls.

**Why this priority**: Production deployments need durable log storage beyond in-memory callbacks. Cloud storage logging enables compliance audit trails, post-incident analysis, and cost attribution. Email alerts are table-stakes for budget governance.

**Independent Test**: Can be fully tested by configuring a cloud storage logger and verifying that structured log entries appear in the target bucket after requests complete.

**Acceptance Scenarios**:

1. **Given** S3 logging is configured with a bucket and prefix, **When** a request completes, **Then** a structured JSON log entry is written to the configured S3 path within 60 seconds
2. **Given** email alerting is configured with SMTP settings and a budget threshold, **When** a key/team/user spend exceeds the threshold, **Then** an email is sent to the configured recipients with the spend details
3. **Given** the cloud storage service is temporarily unavailable, **When** logs fail to write, **Then** the proxy buffers failed logs and retries according to the configured retry policy without blocking request processing

---

### User Story 4 - Vertex AI and SageMaker Provider Support (Priority: P2)

As a platform engineer with workloads on Google Cloud or AWS managed AI services, I want tianjiLLM to natively support Vertex AI and SageMaker so that I can route requests to these services through the same unified proxy.

**Why this priority**: Vertex AI and SageMaker are the primary managed LLM services for Google Cloud and AWS enterprise customers respectively. These cannot be handled by the openaicompat factory because they require platform-specific authentication (GCP service account, AWS IAM role signing).

**Independent Test**: Can be fully tested by configuring a Vertex AI or SageMaker deployment and sending a chat completion request that is correctly transformed and authenticated.

**Acceptance Scenarios**:

1. **Given** a Vertex AI model is configured with a GCP project and region, **When** a chat completion request is sent, **Then** the proxy authenticates using the configured service account, transforms the request to Gemini format, and returns an OpenAI-compatible response
2. **Given** a SageMaker endpoint is configured, **When** a request is sent, **Then** the proxy signs the request using AWS Sigv4 and routes it to the SageMaker endpoint
3. **Given** invalid credentials for Vertex AI, **When** a request is sent, **Then** the proxy returns a clear authentication error identifying the provider and credential issue

---

### User Story 5 - Additional Observability Integrations (Priority: P2)

As a platform engineer, I want tianjiLLM to support popular LLM observability platforms (Langsmith, Braintrust, Helicone, Arize) so that I can plug the proxy into my existing ML monitoring stack.

**Why this priority**: Teams already invested in specific observability platforms need native integration. While the webhook callback provides a generic fallback, native integrations reduce setup friction and provide richer data.

**Independent Test**: Can be fully tested by configuring each integration and verifying that structured trace/log data appears in the target platform after requests complete.

**Acceptance Scenarios**:

1. **Given** Langsmith logging is configured with an API key and project, **When** a request completes, **Then** a trace with model, tokens, cost, and latency is sent to Langsmith
2. **Given** Helicone is configured, **When** a request is sent to a provider, **Then** the proxy injects the correct Helicone headers for transparent logging
3. **Given** a callback integration encounters a transient error, **When** the log send fails, **Then** the proxy retries according to the callback's retry policy without blocking the request response

---

### User Story 6 - Redis Cluster and Advanced Cache (Priority: P3)

As a platform engineer operating tianjiLLM at scale, I want the proxy to support Redis Cluster for cache and rate limiting so that my cache infrastructure is highly available and horizontally scalable.

**Why this priority**: Single-node Redis is a single point of failure. Production deployments at scale require Redis Cluster for HA and partitioning. Semantic caching can significantly reduce costs for repetitive workloads.

**Independent Test**: Can be fully tested by configuring a Redis Cluster endpoint and verifying that cache operations distribute across cluster nodes.

**Acceptance Scenarios**:

1. **Given** a Redis Cluster endpoint is configured, **When** the proxy performs cache operations, **Then** operations distribute across cluster nodes and survive single-node failures
2. **Given** semantic caching is enabled with an embedding model, **When** a semantically similar request is received, **Then** the cached response is returned if the similarity score exceeds the configured threshold

---

### User Story 7 - Prompt Management Integration (Priority: P3)

As a product engineer, I want tianjiLLM to fetch prompt templates from Langfuse or a generic prompt management service so that I can version and manage prompts independently from application code.

**Why this priority**: Separating prompts from code enables non-engineers to iterate on prompts without deployments. This is increasingly important for teams using the proxy as a shared AI platform.

**Independent Test**: Can be fully tested by configuring a Langfuse prompt endpoint and verifying that the proxy resolves prompt templates before forwarding requests.

**Acceptance Scenarios**:

1. **Given** a Langfuse prompt is configured for a model, **When** a request is sent with a prompt reference, **Then** the proxy resolves the prompt from Langfuse, applies template variables, and forwards the resolved content
2. **Given** the prompt management service is unavailable, **When** a request references a prompt, **Then** the proxy falls back to the inline prompt content (if provided) and logs a warning

---

### User Story 8 - Advanced Routing Strategies (Priority: P3)

As a platform engineer managing costs across multiple providers, I want tianjiLLM to support least-busy routing and budget-limited routing so that I can optimize for both performance and cost simultaneously.

**Why this priority**: Current routing strategies cover most use cases, but high-throughput deployments benefit from queue-depth-aware routing (least-busy) and teams with strict provider budgets need budget-limited routing.

**Independent Test**: Can be fully tested by configuring a least-busy strategy and verifying that under load, deployments with fewer in-flight requests receive more traffic.

**Acceptance Scenarios**:

1. **Given** least-busy routing is configured, **When** multiple deployments are available with different queue depths, **Then** the proxy routes to the deployment with the fewest in-flight requests
2. **Given** a provider budget limit is configured, **When** the budget is exhausted, **Then** the proxy excludes that provider's deployments from routing decisions until the budget resets

---

### User Story 9 - Advanced Spend Analytics and FinOps Export (Priority: P3)

As a finance/FinOps lead, I want tianjiLLM to provide advanced spend analytics (trend analysis, top-N queries by spend) and export cost data in FinOps FOCUS format so that I can perform cost attribution, chargeback, and financial reporting.

**Why this priority**: Basic spend tracking already exists. Advanced analytics and FOCUS export enable financial governance for large organizations with multiple teams and providers.

**Independent Test**: Query spend analytics API with group-by and date range → verify aggregated results match expected output.

**Acceptance Scenarios**:

1. **Given** spend data exists for multiple teams and models, **When** an analytics query with group-by=team is submitted, **Then** the proxy returns aggregated spend per team sorted by spend descending
2. **Given** a top-N query for top 5 models by spend, **When** the query is submitted with a date range, **Then** the proxy returns exactly the top 5 models with their spend and request count
3. **Given** a FOCUS export is requested, **When** the export completes, **Then** the output file conforms to FinOps FOCUS 1.2 schema with correct field mappings (BilledCost, Provider, Service, ResourceID)

---

### Edge Cases

- What happens when a secret manager returns an empty or malformed secret? The proxy should reject the secret and log a clear error with the secret path
- What happens when a guardrail service returns an unexpected response format? The proxy should treat it as a guardrail failure and follow the configured fail-open/fail-closed policy
- What happens when cloud storage logging falls permanently behind (e.g., S3 outage > 1 hour)? Following Python TianjiLLM behavior: on each failed flush the entire batch is discarded and logged. The unbounded list continues accumulating until the next successful flush. No memory cap is enforced
- What happens when a Redis Cluster undergoes failover during a cache operation? The proxy should retry the operation once after a brief delay; if it fails again, treat as cache miss
- What happens when a prompt management service returns a prompt with variables that don't match the request? The proxy should reject the request with a clear error listing the missing variables

## Requirements *(mandatory)*

### Functional Requirements

**Work Stream A: Secret Managers**

- **FR-001**: System MUST provide a pluggable secret manager interface with Get, List, and health check operations
- **FR-002**: System MUST support AWS Secrets Manager with IAM role and access key authentication
- **FR-003**: System MUST support Google Secret Manager with service account authentication
- **FR-004**: System MUST support Azure Key Vault with Azure AD token authentication
- **FR-005**: System MUST support HashiCorp Vault with token and AppRole authentication methods
- **FR-006**: System MUST resolve secret references in config files at startup using a configurable syntax (e.g., `os.environ/SECRET_NAME` or provider-prefixed paths)
- **FR-007**: System MUST support credential refresh without proxy restart, using per-instance InMemoryCache with configurable TTL (default 86400s). Instances do not synchronize secret caches; each independently expires and re-fetches on TTL expiry

**Work Stream B: Enterprise Guardrails**

- **FR-008**: System MUST support AWS Bedrock Guardrails with guardrail ID configuration
- **FR-009**: System MUST support Azure Content Safety (text moderation and prompt shield)
- **FR-010**: System MUST support Lakera AI for prompt injection and jailbreak detection
- **FR-011**: System MUST provide a generic guardrail API integration that calls any HTTP endpoint conforming to a defined request/response contract
- **FR-012**: System MUST provide a built-in content filter with configurable category patterns (violence, hate, self-harm, sexual content) supporting multiple languages
- **FR-013**: System MUST support a configurable fail-open or fail-closed policy when a guardrail service is unavailable
- **FR-014**: System MUST support tool permission guardrails that restrict which function call tools can be invoked, configurable per key or team

**Work Stream C: Cloud Storage Logging & Alerting**

- **FR-015**: System MUST support logging to Amazon S3 with configurable bucket, prefix, and flush interval
- **FR-016**: System MUST support logging to Google Cloud Storage with configurable bucket and prefix
- **FR-017**: System MUST support logging to Azure Blob Storage with configurable container and prefix
- **FR-018**: System MUST support email alerting via SMTP for budget threshold breaches
- **FR-019**: System MUST buffer log entries in an unbounded in-memory list, flush when batch_size (default 512, env-configurable) is reached or every flush_interval (default 5s, env-configurable), and on flush failure discard the batch and log the error without blocking request processing
- **FR-020**: System MUST support DynamoDB as a log storage backend
- **FR-021**: System MUST support SQS as an asynchronous log queue

**Work Stream D: Provider Expansion**

- **FR-022**: System MUST support Vertex AI with GCP service account authentication and Gemini format transformation
- **FR-023**: System MUST support SageMaker with AWS Sigv4 request signing
- **FR-024**: System MUST support AI21 with its native API format
- **FR-025**: System MUST support WatsonX with IBM Cloud IAM authentication
- **FR-026**: ~~System MUST provide a `providers.json` registry that enables zero-code registration of OpenAI-compatible providers with only a base URL and optional API key header~~ *[EXISTING — already implemented in Phase 1 via `internal/provider/openaicompat/loader.go`]*

**Work Stream E: Observability Integrations**

- **FR-027**: System MUST support Langsmith with trace-level logging including model, tokens, cost, latency, and metadata
- **FR-028**: System MUST support Braintrust with experiment-level logging
- **FR-029**: System MUST support Helicone via header injection for transparent logging
- **FR-030**: System MUST support Arize/Phoenix with ML observability payloads
- **FR-031**: System MUST support MLflow experiment tracking
- **FR-032**: System MUST support Weights & Biases logging

**Work Stream F: Cache Enhancements**

- **FR-033**: System MUST support Redis Cluster as a cache and rate-limiting backend
- **FR-034**: System MUST support semantic caching using an embedding model to match semantically similar requests
- **FR-035**: System MUST support disk-based caching for local development environments

**Work Stream G: Prompt Management**

- **FR-036**: System MUST support fetching prompt templates from Langfuse with version and label selection
- **FR-037**: System MUST provide a generic prompt management interface for custom prompt sources

**Work Stream H: Router Enhancements**

- **FR-038**: System MUST support a least-busy routing strategy based on in-flight request count per deployment
- **FR-039**: System MUST support a budget-limited routing strategy that excludes deployments whose provider budget is exhausted

**Work Stream I: Advanced Analytics**

- **FR-040**: System MUST support advanced spend analytics including trend analysis over configurable time periods
- **FR-041**: System MUST support top-N queries for keys, teams, users, and models by spend
- **FR-042**: System MUST support FinOps FOCUS-format export to cloud storage

### Key Entities

- **SecretManager**: A pluggable backend for resolving credential references at startup and runtime; attributes include provider type, auth config, cache TTL, and health status
- **GuardrailService**: An external content safety service with an API contract; attributes include endpoint URL, auth method, supported hooks (pre-call/post-call), and failure policy (fail-open/fail-closed)
- **ContentFilter**: A built-in category-based content filter; attributes include category patterns, language, and severity thresholds
- **CloudLogger**: A durable log storage backend; attributes include storage type, bucket/container, prefix, batch_size (default 512), flush_interval (default 5s). Log payload is the full StandardLoggingPayload including request/response bodies
- **PromptTemplate**: A versioned prompt template fetched from an external service; attributes include template ID, version/label, variable schema, and resolved content

## Scope & Boundaries

### Current State (after Phase 1)

| Dimension         | Count                      |
| ----------------- | -------------------------- |
| Providers         | 20 + openaicompat factory  |
| Core LLM APIs     | 10 (complete)              |
| Management APIs   | 15 (complete)              |
| Callbacks         | 7                          |
| Guardrails        | 3                          |
| Secret Managers   | 0                          |
| Cache backends    | 3                          |
| Router strategies | 5                          |

### Target State (after Phase 2)

| Dimension         | Count                          |
| ----------------- | ------------------------------ |
| Providers         | 24 + openaicompat factory      |
| Core LLM APIs     | 10 (no change)                 |
| Management APIs   | 15 (no change)                 |
| Callbacks         | 15+                            |
| Guardrails        | 9+                             |
| Secret Managers   | 4+                             |
| Cache backends    | 5+                             |
| Router strategies | 7                              |

### In Scope

- All functional requirements FR-001 through FR-042
- Maintaining backward compatibility with Phase 1 functionality
- Plugin-based architecture for all new subsystems (secret managers, guardrails, loggers, prompt sources)

### Out of Scope

- Assistants/Threads API (OpenAI-specific, low adoption, can be added as pass-through later)
- Realtime/WebSocket API (requires fundamentally different server architecture; separate spec recommended)
- Vector Stores API (can use external vector DB solutions)
- MCP Server endpoints (experimental in Python TianjiLLM, not production-ready)
- Agent/A2A protocol endpoints (experimental)
- UI/dashboard endpoints
- Video/OCR/Container/Skills endpoints (niche use cases)
- Auto Router ML-based routing (XL complexity, minimal ROI over existing strategies)
- Migrating all 117 Python providers (openaicompat factory covers 80%+ of OpenAI-compatible providers)
- Migrating all 44+ Python callbacks (target top 15, generic webhook covers the rest)
- Migrating all 30+ Python guardrails (generic guardrail API covers most third-party services)

## Assumptions

- The openaicompat factory pattern continues to handle the long tail of OpenAI-compatible providers via `providers.json` configuration, avoiding the need to port each one individually
- Enterprise customers prioritize secret management, guardrails, and durable logging over additional provider support
- Cloud storage logging is eventually consistent — logs may be delayed up to 60 seconds under normal conditions
- Secret manager integrations follow each cloud provider's standard SDK authentication patterns
- Semantic caching accuracy depends on the quality of the configured embedding model; the system provides the infrastructure but does not guarantee cache hit quality
- Budget-limited routing uses the existing spend tracking data; no separate provider billing integration is required

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Proxy retrieves all provider credentials from at least one configured secret manager at startup, with zero secrets in environment variables or config files
- **SC-002**: At least 5 guardrail integrations (including generic API) can block violating requests within the request lifecycle with less than 50ms added latency per guardrail
- **SC-003**: Cloud storage logs appear in the target bucket within 60 seconds of request completion under normal operating conditions
- **SC-004**: Redis Cluster cache operations survive single-node failures without request-visible errors
- **SC-005**: 90% of enterprise customers (those requiring secret management + guardrails + durable logging) can adopt tianjiLLM without feature gaps
- **SC-006**: All new integrations (secret managers, guardrails, loggers, prompt sources) follow the self-registration plugin pattern, requiring zero changes to existing code when adding a new integration
- **SC-007**: Prompt templates resolve from external services within 200ms for cached templates and 2 seconds for uncached templates
- **SC-008**: Advanced spend analytics queries return results within 5 seconds for datasets up to 10 million spend log entries
