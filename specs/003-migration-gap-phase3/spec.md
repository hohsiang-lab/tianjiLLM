# Feature Specification: TianjiLLM Go Migration Phase 3 — Enterprise Features & Full Parity

**Feature Branch**: `003-migration-gap-phase3`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "Phase 3 migration gap analysis: enterprise features, missing management APIs, additional providers/callbacks/guardrails"

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

Phase 2 (002-migration-gap-phase2) completed 73 tasks and delivered:
- 4 secret managers (AWS, GCP, Azure Key Vault, HashiCorp Vault)
- 7 enterprise guardrails (Bedrock, Azure Content Safety, Lakera, generic API, content filter, tool permission, fail-open/fail-closed policy)
- Cloud storage logging (S3, GCS, Azure Blob, DynamoDB, SQS) + email alerting
- 4 additional providers (Vertex AI, SageMaker, AI21, WatsonX)
- 8 observability integrations (Langsmith, Braintrust, Helicone, Arize/Phoenix, MLflow, W&B)
- Redis Cluster + semantic cache + disk cache
- Prompt management (Langfuse + generic interface)
- 2 router strategies (least-busy, budget-limited)
- Advanced spend analytics + FinOps FOCUS export

This Phase 3 spec addresses the remaining gaps to achieve enterprise-grade production parity. Scope: ~130 tasks across P0/P1/P2 priorities.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Policy Engine for Conditional Routing and Guardrail Assignment (Priority: P0)

As a platform architect managing a multi-team LLM deployment, I want a policy engine that conditionally assigns guardrails and routing rules based on request attributes (model, team, key) so that different teams get different safety and routing policies without per-key manual configuration.

**Why this priority**: Without a policy engine, admins must manually configure guardrails and routing for each key/team individually. This does not scale for organizations with 50+ teams and complex compliance requirements. Policy-based automation is the single largest enterprise governance gap.

**Independent Test**: Can be fully tested by creating policies with conditions, attaching them to teams/keys, and verifying that matching requests have the correct guardrails applied.

**Acceptance Scenarios**:

1. **Given** a policy with condition `model: "gpt-4.*"` and guardrails `[pii-detection, content-filter]`, **When** a request for model `gpt-4o` is sent, **Then** both guardrails are applied to the request
2. **Given** a policy attached to `team-finance` with guardrails `[budget-check, audit-log]`, **When** a request from `team-finance` is sent, **Then** the policy's guardrails are applied regardless of the specific key used
3. **Given** a child policy that inherits from a parent policy and removes `content-filter`, **When** a request matches the child policy, **Then** the resolved guardrail list includes the parent's guardrails minus `content-filter`
4. **Given** a policy pipeline with 3 steps and `on_fail: block` on step 2, **When** step 2 fails, **Then** the request is blocked and steps 3 onward are skipped
5. **Given** a policy CRUD API, **When** an admin creates, reads, updates, and deletes policies and their attachments, **Then** all operations succeed and the changes take effect on subsequent requests

---

### User Story 2 - SCIM 2.0 Protocol for Enterprise IDP Provisioning (Priority: P0)

As an IT administrator using Okta or Azure AD for identity management, I want tianjiLLM to support SCIM 2.0 so that user and group (team) provisioning is automatic when I manage identities in my IDP.

**Why this priority**: Enterprise customers with 100+ users cannot manually create and manage proxy users. SCIM 2.0 is a hard requirement for SOC2-compliant organizations using centralized identity management. Without SCIM, every user change requires manual proxy configuration.

**Independent Test**: Can be fully tested by configuring an IDP (or SCIM test client) to provision users and groups, then verifying the corresponding proxy users and teams are created.

**Acceptance Scenarios**:

1. **Given** SCIM is enabled, **When** an IDP sends a POST to `/scim/v2/Users` with user attributes, **Then** the proxy creates a corresponding internal user with the correct email, alias, and metadata
2. **Given** SCIM is enabled, **When** an IDP sends a POST to `/scim/v2/Groups` with a group name and member list, **Then** the proxy creates a corresponding team with the listed members
3. **Given** a SCIM user exists, **When** the IDP sends a PATCH to update the user's active status to false, **Then** the proxy deactivates the user and their keys stop working
4. **Given** SCIM is enabled, **When** the IDP queries `/scim/v2/Users` with a filter, **Then** the proxy returns matching users in RFC 7644 compliant format
5. **Given** `scim_upsert_user: true` (default), **When** a Group references a user that doesn't exist, **Then** the proxy auto-creates the user; when `scim_upsert_user: false`, the request is rejected

---

### User Story 3 - Assistants API Pass-through (Priority: P0)

As a developer using OpenAI Assistants in my application, I want tianjiLLM to proxy Assistants, Threads, and Runs endpoints so that I can use the same proxy for both chat completions and assistant workflows.

**Why this priority**: The Assistants API is widely adopted for agent-based workflows. Applications using both chat completions and assistants need a single proxy entry point. Without this, developers must maintain separate connection configurations.

**Independent Test**: Can be fully tested by sending Assistants API requests through the proxy and verifying they reach the upstream provider (OpenAI or Azure) and return valid responses.

**Acceptance Scenarios**:

1. **Given** an assistants config with `custom_llm_provider: "openai"`, **When** a POST to `/v1/assistants` is sent, **Then** the proxy creates an assistant on OpenAI and returns the response
2. **Given** an existing assistant, **When** a POST to `/v1/threads` creates a thread, then POST to `/v1/threads/{id}/messages` adds a message, then POST to `/v1/threads/{id}/runs` starts a run, **Then** each operation succeeds and returns valid responses
3. **Given** `custom_llm_provider: "azure"`, **When** assistants requests are sent, **Then** the proxy routes to Azure OpenAI with correct authentication
4. **Given** an unsupported provider, **When** an assistants request is sent, **Then** the proxy returns a clear error indicating the provider does not support the Assistants API

---

### User Story 4 - Background Scheduler for Automated Maintenance (Priority: P0)

As a platform operator, I want tianjiLLM to run background tasks (budget reset, spend log cleanup, deployment hot-reload, health checks) so that the proxy maintains itself without manual intervention or external cron jobs.

**Why this priority**: Without automated budget resets, keys and teams accumulate spend indefinitely and budgets never reset. Without log cleanup, the database grows unbounded. Without hot-reload, config changes require proxy restarts. These are basic operational requirements for any production deployment.

**Independent Test**: Can be fully tested by configuring budget reset intervals and verifying that key/team spend resets on schedule, and that expired spend logs are cleaned up.

**Acceptance Scenarios**:

1. **Given** a key with a daily budget and `budget_reset_at` set, **When** the reset interval elapses, **Then** the key's spend is reset to zero and `budget_reset_at` is updated to the next reset time
2. **Given** `maximum_spend_logs_retention_days: 30`, **When** the cleanup job runs, **Then** spend log entries older than 30 days are deleted from the database
3. **Given** a new model deployment added to the database, **When** the hot-reload job runs (every 30 seconds), **Then** the new deployment is available for routing without proxy restart
4. **Given** `health_check_interval: 60`, **When** the health check job runs, **Then** each deployment is probed and unhealthy deployments are marked with updated failure counts
5. **Given** the scheduler is running, **When** the proxy shuts down, **Then** all background jobs are gracefully stopped and in-flight operations complete

---

### User Story 5 - High-Value Callback Integrations (Priority: P0)

As a platform engineer, I want tianjiLLM to support Lunary, Traceloop, PostHog, Opik, TianjiLLM-native Datadog LLM Observability, GCS Pub/Sub, OpenMeter, Greenscale, and PromptLayer so that I can plug the proxy into my existing analytics and observability stack.

**Why this priority**: These 9 callbacks represent the most-requested integrations from enterprise customers. Lunary and Traceloop are popular open-source LLM analytics tools. PostHog is the leading product analytics platform. OpenMeter and Greenscale enable usage metering and carbon tracking respectively.

**Independent Test**: Can be fully tested by configuring each callback and verifying that structured log/metric data is sent to the target service after requests complete.

**Acceptance Scenarios**:

1. **Given** Lunary logging is configured, **When** a request completes, **Then** a trace with model, tokens, cost, and latency is sent to Lunary
2. **Given** Traceloop is configured, **When** a request completes, **Then** OpenTelemetry-compatible trace data is exported to the Traceloop endpoint
3. **Given** PostHog is configured, **When** a request completes, **Then** an event with LLM usage properties is sent to PostHog
4. **Given** Datadog LLM Observability is configured, **When** a request completes, **Then** LLM-specific metrics and traces are sent using Datadog's LLM Observability schema
5. **Given** any of the 9 callbacks encounters a transient error, **When** the send fails, **Then** the error is logged and request processing is not blocked

---

### User Story 6 - Missing Management Endpoints (Priority: P1)

As an admin managing a large tianjiLLM deployment, I want complete CRUD for models, tags, customers (end_users), fallback configs, and proxy config so that I can manage all aspects of the proxy through the API without manual config file edits.

**Why this priority**: Phase 1+2 delivered core management APIs but several important CRUD operations are missing. Without model management API, adding/removing models requires config file changes and restarts. Without tag management, cost attribution and filtering are limited.

**Independent Test**: Can be fully tested by calling each CRUD endpoint and verifying the changes are persisted and take effect.

**Acceptance Scenarios**:

1. **Given** an admin user, **When** they call POST `/model/new` with model configuration, **Then** the model is added to the router and available for requests without restart
2. **Given** tags exist in the system, **When** an admin calls GET `/tag/list` and GET `/tag/summary`, **Then** all tags with their spend summaries are returned
3. **Given** an end_user/customer management API, **When** CRUD operations (create, read, update, delete, block, unblock) are performed, **Then** end_user records are persisted and blocking takes effect on subsequent requests
4. **Given** a config management API, **When** an admin updates callback or pass-through endpoint settings via API, **Then** changes take effect without restart
5. **Given** key management extensions, **When** an admin calls `/key/{key}/regenerate`, **Then** a new key is issued with the same permissions and the old key is invalidated

---

### User Story 7 - Medium-Value Callback Integrations (Priority: P1)

As a platform engineer with specific observability and compliance requirements, I want tianjiLLM to support Argilla, Lago, Azure Sentinel, Supabase, CloudZero, Logfire, Athina, DeepEval, Galileo, and Literal AI so that I can meet my organization's specific monitoring and billing integration needs.

**Why this priority**: These 10 callbacks serve specific enterprise niches. Argilla is essential for data labeling teams. Lago and CloudZero serve billing/cost teams. Azure Sentinel is required by Azure-centric security teams. Each unlocks a specific customer segment.

**Independent Test**: Can be fully tested by configuring each callback and verifying that log/metric data is sent to the target service.

**Acceptance Scenarios**:

1. **Given** Argilla is configured, **When** a request completes, **Then** the request/response pair is logged to Argilla as an annotation candidate
2. **Given** Lago is configured with billing events, **When** a request completes, **Then** a usage event is sent to Lago for billing purposes
3. **Given** Azure Sentinel is configured, **When** a request completes, **Then** a security log entry is sent to the Azure Sentinel workspace
4. **Given** any of the 10 callbacks, **When** the target service is unavailable, **Then** the failure is logged and request processing continues normally

---

### User Story 8 - Additional API Endpoints (Priority: P1)

As a developer using advanced OpenAI features (vector stores, Responses API extensions, provider native formats), I want tianjiLLM to support these endpoints so that I can use the full range of LLM capabilities through a single proxy.

**Why this priority**: Vector stores are increasingly used for RAG workflows. The Responses API needs GET/cancel/input_items for production use. Provider pass-through namespaces (e.g., `/anthropic/`, `/vertex_ai/`) enable direct provider API access through the proxy.

**Independent Test**: Can be fully tested by sending requests to each endpoint and verifying correct upstream forwarding and response format.

**Acceptance Scenarios**:

1. **Given** vector store endpoints are enabled, **When** a client calls POST `/v1/vector_stores/{id}/files` and POST `/v1/vector_stores/{id}/search`, **Then** the requests are forwarded to the upstream provider and results returned
2. **Given** an existing response, **When** GET `/v1/responses/{id}` is called, **Then** the response details are returned; when POST `/v1/responses/{id}/cancel` is called, the response generation is cancelled
3. **Given** provider pass-through namespaces are configured, **When** a request is sent to `/anthropic/v1/messages`, **Then** it is forwarded to Anthropic's API with correct auth and the raw response is returned
4. **Given** Anthropic native format is enabled, **When** a request is sent to `/v1/messages`, **Then** the proxy forwards it as an Anthropic Messages API request
5. **Given** Gemini native format endpoints, **When** a request to `/models/{name}:generateContent` is sent, **Then** the proxy forwards it to Gemini in its native format

---

### User Story 9 - Database Views and Cold Storage for Spend Analytics (Priority: P1)

As a FinOps analyst, I want tianjiLLM to provide database views for spend analysis and support archiving spend logs to cold storage (S3/GCS) so that I can perform historical cost analysis without impacting production database performance.

**Why this priority**: As deployments grow, the spend logs table becomes the largest table in the database. Without cold storage archival, queries slow down and storage costs increase. DB views provide pre-computed aggregations for common analytics queries.

**Independent Test**: Can be fully tested by generating spend data, querying the views for aggregated results, and verifying that archived logs are accessible in cold storage.

**Acceptance Scenarios**:

1. **Given** spend data exists, **When** the daily spend view is queried with group-by team, **Then** pre-aggregated daily spend per team is returned within 2 seconds
2. **Given** a cold storage archival schedule (e.g., archive logs older than 90 days), **When** the archival job runs, **Then** matching spend logs are exported to the configured S3/GCS bucket and removed from the primary database
3. **Given** archived logs exist in cold storage, **When** a historical spend query spans archived dates, **Then** the system either queries cold storage directly or returns a clear indication that the data is archived with a pointer to the storage location
4. **Given** `global/spend/*` endpoints, **When** queried with various dimensions (keys, models, teams, tags, providers), **Then** aggregated global spend data is returned

---

### User Story 10 - Special Native Providers (Priority: P2)

As a platform engineer serving teams on GitHub Copilot, Snowflake Cortex, Oracle OCI, SAP AI Core, or Chinese cloud platforms (DashScope/Alibaba, Volcengine/ByteDance, MiniMax, Moonshot/Kimi), I want tianjiLLM to natively support these providers so that I can route to them through the unified proxy.

**Why this priority**: These providers require platform-specific authentication or API formats that cannot be handled by the openaicompat factory. GitHub Copilot and Snowflake Cortex are increasingly popular in enterprise. Chinese market providers (DashScope, Volcengine, MiniMax, Moonshot) serve a large market segment.

**Independent Test**: Can be fully tested by configuring each provider and sending a chat completion request, verifying correct auth and format transformation.

**Acceptance Scenarios**:

1. **Given** GitHub Copilot is configured with a token, **When** a chat completion request is sent, **Then** the proxy authenticates with GitHub's Copilot API and returns an OpenAI-compatible response
2. **Given** Snowflake Cortex is configured with account credentials, **When** a request is sent, **Then** the proxy authenticates via Snowflake's auth mechanism and transforms the request/response
3. **Given** DashScope (Alibaba) is configured, **When** a request is sent, **Then** the proxy transforms the request to DashScope's format and returns an OpenAI-compatible response
4. **Given** any of the special native providers has invalid credentials, **When** a request is sent, **Then** a clear authentication error identifying the provider is returned

---

### User Story 11 - Guardrail CRUD and Prompt Management Endpoints (Priority: P2)

As a platform admin, I want to manage guardrails and prompts through REST APIs so that I can create, update, and test guardrail configurations and prompt templates without editing config files.

**Why this priority**: Phase 1+2 guardrails are config-file driven only. Production teams need API-driven guardrail management for dynamic policy changes. Similarly, prompt management needs REST endpoints for versioning and testing.

**Independent Test**: Can be fully tested by calling CRUD endpoints for guardrails and prompts, then verifying changes take effect on subsequent requests.

**Acceptance Scenarios**:

1. **Given** a guardrail CRUD API, **When** an admin creates a guardrail with POST `/guardrails`, **Then** it is available for policy attachment and request processing
2. **Given** an existing guardrail, **When** an admin updates its configuration via PUT `/guardrails/{id}`, **Then** subsequent requests use the updated configuration
3. **Given** a prompt CRUD API, **When** an admin creates a prompt with POST `/prompts` and specifies version and variables, **Then** the prompt is stored and retrievable
4. **Given** a prompt with template variables, **When** a test request is sent via POST `/prompts/test`, **Then** the resolved prompt is returned without calling the LLM

---

### User Story 12 - Cache, Secret Manager, and Auth Enhancements (Priority: P2)

As a platform engineer with advanced infrastructure requirements, I want tianjiLLM to support S3/GCS/Azure Blob as cache backends, CyberArk as a secret manager, and IP whitelist-based access control so that I can meet specific enterprise infrastructure and security requirements.

**Why this priority**: Some enterprises mandate specific cache and secret management solutions. IP whitelisting is a common network-level security requirement. These are incremental additions to existing subsystems.

**Independent Test**: Can be fully tested by configuring each backend and verifying operations succeed (cache hit/miss, secret resolution, IP allow/deny).

**Acceptance Scenarios**:

1. **Given** S3 is configured as a cache backend, **When** a cache set is performed, **Then** the value is stored in S3; when a cache get is performed for the same key, **Then** the cached value is returned
2. **Given** CyberArk is configured as a secret manager, **When** the proxy starts with secret references, **Then** secrets are resolved from CyberArk's API
3. **Given** an IP whitelist is configured, **When** a request arrives from a whitelisted IP, **Then** it is allowed; when from a non-whitelisted IP, **Then** it is rejected with a 403 error
4. **Given** GCS or Azure Blob is configured as a cache backend, **When** cache operations are performed, **Then** they succeed with the same semantics as S3 cache

---

### Edge Cases

- What happens when a policy has circular inheritance (Policy A inherits from Policy B which inherits from Policy A)? The policy resolver should detect cycles and reject the policy with a clear error at creation time
- What happens when SCIM provisions a user that already exists (same email)? The system should update the existing user's metadata rather than creating a duplicate
- What happens when the scheduler misses a budget reset (e.g., proxy was down)? On startup, the scheduler should check all overdue budget resets and execute them immediately
- What happens when a cold storage archival job is interrupted mid-export? The job should be idempotent — re-running it should not create duplicate entries in cold storage
- What happens when a provider pass-through namespace receives a request for an unconfigured provider? Return 404 with a message indicating the pass-through provider is not configured
- What happens when a callback integration changes its API format? The proxy should handle unexpected response formats gracefully, logging the error without crashing

## Requirements *(mandatory)*

### Functional Requirements

**Work Stream A: Policy Engine (US1)**

- **FR-001**: System MUST provide a policy data model with name, optional parent policy for inheritance, guardrails-add list, guardrails-remove list, and optional condition
- **FR-002**: System MUST support policy conditions based on model name (exact match, regexp patterns, and list of patterns)
- **FR-003**: System MUST support policy attachments that bind a policy to multi-dimensional scopes (global, or any combination of teams[], keys[], models[], tags[] — supports wildcards)
- **FR-004**: System MUST resolve policy inheritance chains, merging guardrails-add and applying guardrails-remove from child policies
- **FR-005**: System MUST execute guardrail pipelines in defined step order, with configurable on_pass/on_fail actions (next, allow, block, modify_response) and optional data forwarding between steps (pass_data)
- **FR-006**: System MUST provide REST endpoints for policy CRUD (create, read, update, delete, list)
- **FR-007**: System MUST provide REST endpoints for policy attachment CRUD (create, read, delete, list)
- **FR-008**: System MUST provide a test-pipeline endpoint that evaluates a policy pipeline against sample input without calling providers
- **FR-009**: System MUST detect circular policy inheritance at creation time and reject with a clear error
- **FR-010**: System MUST support policy resolution that matches request context (model, team, key) to applicable policies and returns merged guardrail lists

**Work Stream B: SCIM 2.0 Protocol (US2)**

- **FR-011**: System MUST implement SCIM 2.0 service provider configuration endpoint (`/scim/v2/ServiceProviderConfig`) returning supported features
- **FR-012**: System MUST implement SCIM 2.0 schema discovery endpoints (`/scim/v2/Schemas`, `/scim/v2/ResourceTypes`)
- **FR-013**: System MUST implement SCIM 2.0 User CRUD (POST, GET, PUT, PATCH, DELETE on `/scim/v2/Users` and `/scim/v2/Users/{id}`)
- **FR-014**: System MUST implement SCIM 2.0 Group CRUD (POST, GET, PUT, PATCH, DELETE on `/scim/v2/Groups` and `/scim/v2/Groups/{id}`)
- **FR-015**: System MUST map SCIM User attributes to internal user identity (userName maps to user identifier, externalId maps to SSO identity reference, active status controls user access state)
- **FR-016**: System MUST map SCIM Group attributes to internal team records (displayName maps to team identifier, members list maps to team membership)
- **FR-017**: System MUST support SCIM list filtering with at least `eq` operator on `userName` and `displayName`
- **FR-018**: System MUST support configurable upsert behavior for group member references (auto-create missing users when `scim_upsert_user: true`, reject when false)
- **FR-019**: System MUST support SCIM PATCH operations with `add`, `remove`, and `replace` operations per RFC 7644
- **FR-020**: System MUST authenticate SCIM requests using Bearer token (the proxy's master key or a dedicated SCIM token)

**Work Stream C: Assistants API Pass-through (US3)**

- **FR-021**: System MUST implement pass-through for POST and GET on `/v1/assistants` and `/v1/assistants/{id}`
- **FR-022**: System MUST implement pass-through for POST `/v1/threads`, GET `/v1/threads/{id}`
- **FR-023**: System MUST implement pass-through for POST and GET on `/v1/threads/{id}/messages`
- **FR-024**: System MUST implement pass-through for POST and GET on `/v1/threads/{id}/runs`
- **FR-025**: System MUST support OpenAI and Azure as assistants providers, selectable via `assistant_settings.custom_llm_provider`
- **FR-026**: System MUST return a clear unsupported-provider error for assistants requests targeting non-OpenAI/Azure providers

**Work Stream D: Background Scheduler (US4)**

- **FR-027**: System MUST run a periodic budget reset job that resets spend for keys, users, and teams whose `budget_reset_at` has elapsed
- **FR-028**: System MUST run a periodic spend log cleanup job that deletes log entries older than a configurable retention period
- **FR-029**: System MUST run a periodic deployment hot-reload job that syncs model deployments from the database without restart
- **FR-030**: System MUST run a periodic health check job that probes deployment endpoints and updates health status
- **FR-031**: System MUST run a periodic credential refresh job that reloads credentials from the database
- **FR-032**: System MUST support configurable intervals for all background jobs
- **FR-033**: System MUST catch up on missed job executions at startup (e.g., overdue budget resets)
- **FR-034**: System MUST gracefully stop all background jobs during shutdown, waiting for in-flight operations to complete
- **FR-034a**: System MUST run a periodic spend batch-write job that flushes accumulated spend data to the database
- **FR-034b**: System MUST run a periodic batch cost check and responses cost check for async API operations
- **FR-034c**: System MUST run a periodic key rotation check job
- **FR-034d**: System MUST support distributed locking (Redis-based) to prevent duplicate job execution in multi-pod deployments

**Work Stream E: High-Value Callbacks (US5)**

- **FR-035**: System MUST support Lunary callback with trace-level LLM usage data
- **FR-036**: System MUST support Traceloop callback with OpenTelemetry-compatible trace export
- **FR-037**: System MUST support PostHog callback with event properties for LLM usage analytics
- **FR-038**: System MUST support Opik callback with experiment tracking data
- **FR-039**: System MUST support Datadog LLM Observability callback using Datadog's LLM-specific schema
- **FR-040**: System MUST support GCS Pub/Sub callback for event-driven log processing
- **FR-041**: System MUST support OpenMeter callback for usage metering events
- **FR-042**: System MUST support Greenscale callback for carbon emission tracking
- **FR-043**: System MUST support PromptLayer callback for prompt version tracking

**Work Stream F: Missing Management Endpoints (US6)**

- **FR-044**: System MUST implement model management CRUD (create, read, update, delete via `/model/*` endpoints)
- **FR-045**: System MUST implement tag management CRUD (create, read, update, delete, list, summary via `/tag/*` endpoints)
- **FR-046**: System MUST implement end_user/customer management CRUD (create, read, update, delete, list, block, unblock via `/end_user/*` or `/customer/*` endpoints)
- **FR-047**: System MUST implement config management API for updating callback and pass-through endpoint settings without restart
- **FR-048**: System MUST implement key regeneration (POST `/key/{key}/regenerate`) that issues a new key with the same permissions
- **FR-049**: System MUST implement extended key management (bulk update, health check, aliases)
- **FR-050**: System MUST implement extended team management (info, block/unblock, daily activity, model add/remove)
- **FR-051**: System MUST implement extended user management (info, update, daily activity)

**Work Stream G: Medium-Value Callbacks (US7)**

- **FR-052**: System MUST support Argilla callback for data annotation logging
- **FR-053**: System MUST support Lago callback for billing event emission
- **FR-054**: System MUST support Azure Sentinel callback for SIEM log forwarding
- **FR-055**: System MUST support Supabase callback for PostgreSQL-based log storage
- **FR-056**: System MUST support CloudZero callback for cloud cost intelligence
- **FR-057**: System MUST support Logfire callback for structured logging
- **FR-058**: System MUST support Athina callback for LLM monitoring
- **FR-059**: System MUST support DeepEval callback for LLM evaluation
- **FR-060**: System MUST support Galileo callback for LLM quality observability
- **FR-061**: System MUST support Literal AI callback for observability platform integration

**Work Stream H: Additional API Endpoints (US8)**

- **FR-062**: System MUST implement vector store file management (POST, GET, DELETE on `/v1/vector_stores/{id}/files` and `/v1/vector_stores/{id}/files/{file_id}`)
- **FR-063**: System MUST implement vector store search (POST `/v1/vector_stores/{id}/search`)
- **FR-064**: System MUST implement Responses API extensions (GET `/v1/responses/{id}`, POST `/v1/responses/{id}/cancel`, GET `/v1/responses/{id}/input_items`)
- **FR-065**: System MUST implement provider pass-through namespaces (`/anthropic/{path}`, `/openai/{path}`, `/azure/{path}`, `/vertex_ai/{path}`, `/bedrock/{path}`, `/gemini/{path}`, `/cohere/{path}`, `/mistral/{path}`)
- **FR-066**: System MUST implement Anthropic native message format endpoint (`/v1/messages` and `/v1/messages/count_tokens`)
- **FR-067**: System MUST implement Gemini native format endpoints (`/models/{name}:generateContent`, `/models/{name}:streamGenerateContent`, `/models/{name}:countTokens`)
- **FR-068**: System MUST implement images edit endpoint (`/v1/images/edits`)

**Work Stream I: DB Views and Cold Storage (US9)**

- **FR-069**: System MUST provide pre-computed spend aggregations queryable with sub-second performance for daily spend by team, model, key, and tag
- **FR-070**: System MUST support archiving spend logs older than a configurable retention period to S3 or GCS
- **FR-071**: System MUST implement global spend endpoints (`/global/spend`, `/global/spend/keys`, `/global/spend/models`, `/global/spend/teams`, `/global/spend/tags`, `/global/spend/provider`)
- **FR-072**: System MUST implement spend log query endpoints (`/spend/logs` with filtering by key, team, model, and date range)
- **FR-073**: System MUST ensure archived logs do not reappear when the archival job re-runs (idempotent archival)

**Work Stream J: Special Native Providers (US10)**

- **FR-074**: System MUST support GitHub Copilot with its token-based authentication
- **FR-075**: System MUST support Snowflake Cortex with Snowflake account authentication
- **FR-076**: System MUST support Oracle OCI with OCI IAM authentication
- **FR-077**: System MUST support SAP AI Core with SAP authentication
- **FR-078**: System MUST support DashScope (Alibaba Cloud) with its native API format
- **FR-079**: System MUST support Volcengine (ByteDance) with its authentication and format
- **FR-080**: System MUST support MiniMax with its native API format
- **FR-081**: System MUST support Moonshot (Kimi) with its native API format
- **FR-082**: System MUST support NVIDIA NIM with its inference API
- **FR-083**: System MUST support OpenRouter with its aggregation API and model routing
- **FR-084**: System MUST support DeepInfra with its inference API
- **FR-085**: System MUST support Azure AI Studio (non-OpenAI Azure AI models) with Azure AD authentication

**Work Stream K: Guardrail and Prompt CRUD Endpoints (US11)**

- **FR-086**: System MUST implement guardrail CRUD endpoints (POST, GET, PUT, DELETE on `/guardrails` and `/guardrails/{id}`)
- **FR-087**: System MUST implement guardrail list endpoint with filtering capabilities
- **FR-088**: System MUST implement prompt CRUD endpoints (POST, GET, PUT, DELETE on `/prompts` and `/prompts/{id}`)
- **FR-089**: System MUST implement prompt version listing (GET `/prompts/{id}/versions`)
- **FR-090**: System MUST implement prompt test endpoint (POST `/prompts/test`) that resolves template variables without calling LLM

**Work Stream L: Cache, Secret, and Auth Enhancements (US12)**

- **FR-091**: System MUST support S3 as a cache backend with configurable bucket and prefix
- **FR-092**: System MUST support GCS as a cache backend with configurable bucket and prefix
- **FR-093**: System MUST support Azure Blob Storage as a cache backend with configurable container and prefix
- **FR-094**: System MUST support CyberArk as a secret manager with CyberArk Conjur or CyberArk Vault API authentication
- **FR-095**: System MUST support IP whitelist-based access control with configurable allowed IP addresses/ranges
- **FR-096**: System MUST provide API endpoints for managing IP whitelists (add, delete, list allowed IPs)

### Key Entities

- **Policy**: A named set of guardrail assignments and conditions; supports inheritance from a parent policy; can be attached to teams, keys, models, or globally
- **PolicyAttachment**: A binding between a policy and a scope (global, team, key, model) that determines when the policy is evaluated
- **PolicyPipeline**: An ordered sequence of guardrail steps with on_pass/on_fail actions; executed when a matching policy is triggered
- **SCIMUser**: An RFC 7644 User resource that reuses the existing internal user table (no dedicated SCIM table); userName maps to user_id, externalId maps to sso_user_id, active status stored in metadata
- **SCIMGroup**: An RFC 7644 Group resource that reuses the existing internal team table (no dedicated SCIM table); displayName maps to team_alias, members maps to team membership
- **BackgroundJob**: A periodic task with configurable interval, last run time, and status; types include budget-reset, log-cleanup, hot-reload, health-check, credential-refresh
- **ModelDeployment**: A managed model configuration that can be added/updated/removed via API; hot-reloaded by the scheduler
- **SpendArchive**: A batch of spend log entries exported to cold storage; attributes include date range, storage location, entry count, export timestamp

## Scope & Boundaries

### Current State (after Phase 1 + Phase 2)

| Dimension           | Count                          |
| ------------------- | ------------------------------ |
| Providers           | 24 + openaicompat factory      |
| Core LLM APIs       | 10 (complete)                  |
| Management APIs     | 15 (basic CRUD)                |
| Callbacks           | 15                             |
| Guardrails          | 9                              |
| Secret Managers     | 4                              |
| Cache backends      | 5                              |
| Router strategies   | 7                              |
| Background jobs     | 0                              |
| Policy engine       | none                           |
| SCIM support        | none                           |

### Target State (after Phase 3)

| Dimension           | Count                          |
| ------------------- | ------------------------------ |
| Providers           | 36 + openaicompat factory      |
| Core LLM APIs       | 10 + assistants + vector store |
| Management APIs     | 25+ (full CRUD + extensions)   |
| Callbacks           | 34                             |
| Guardrails          | 9 + CRUD management            |
| Secret Managers     | 5                              |
| Cache backends      | 8                              |
| Router strategies   | 7                              |
| Background jobs     | 5+                             |
| Policy engine       | full (CRUD + resolver + pipeline) |
| SCIM support        | full (Users + Groups)          |

### In Scope

- All functional requirements FR-001 through FR-096
- Maintaining backward compatibility with Phase 1 and Phase 2 functionality
- Plugin-based architecture for all new callbacks and providers (self-registration pattern)
- Database migrations for new entities (policies, policy attachments, guardrail configs, prompt records) — SCIM reuses existing user/team tables with no new migrations

### Out of Scope — NEVER Migrate

- **UI/Dashboard endpoints** — Should be an independent frontend project; the Go proxy is an API server, not a web application
- **MCP Server endpoints** — Experimental in Python TianjiLLM; should be a standalone service with its own lifecycle
- **A2A Protocol endpoints** — Application-layer agent-to-agent protocol; not a proxy responsibility
- **Container/Kubernetes endpoints** — Infrastructure orchestration belongs in K8s operators/Helm charts, not the proxy
- **Auto Router ML-based routing** — Extreme complexity (requires ML model serving within the proxy) with minimal ROI over existing strategies (cost, latency, least-busy)
- **All 119 Python providers natively** — The openaicompat factory handles 80%+ of OpenAI-compatible providers via `providers.json`; only providers requiring unique auth or format transformation get native modules
- **All 45+ Python callbacks natively** — Target the top 34, generic webhook covers the rest
- **All 22+ Python guardrails natively** — The 9 existing + generic guardrail API covers enterprise needs
- **Skills/Plugin marketplace** — Application-layer feature, not a proxy responsibility
- **Claude Code integration endpoints** — IDE-specific, not a proxy responsibility
- **Video generation/OCR endpoints** — Niche use cases with low adoption; can be added via pass-through if needed

## Assumptions

- The policy engine follows Python TianjiLLM's design: policies contain guardrail lists (not routing rules), and policy conditions currently only support model matching
- SCIM implementation targets RFC 7644 compliance sufficient for Okta and Azure AD integration; advanced SCIM features (bulk operations, advanced filter operators beyond `eq`) are deferred
- Assistants API is a thin pass-through to OpenAI/Azure; the proxy does not interpret or modify assistant/thread/run state
- Background scheduler uses Go's built-in ticker/timer facilities; no external scheduler dependency (no cron, no APScheduler equivalent)
- Special native providers (GitHub Copilot, Snowflake, etc.) follow the existing provider self-registration pattern; each gets its own module
- Cold storage archival is a batch process that runs on a schedule; real-time log streaming to cold storage is out of scope
- Management endpoint extensions maintain the existing auth model (master key for admin operations, virtual keys for scoped operations)
- Callback integrations follow the existing callback interface with self-registration in `init()`; no changes to the callback dispatch mechanism

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Policy engine can resolve a 3-level inheritance chain and return merged guardrail lists within 10ms
- **SC-002**: SCIM provisioning of 100 users and 20 groups completes within 30 seconds via standard SCIM protocol
- **SC-003**: Assistants API requests pass through with less than 5ms added latency compared to direct provider access
- **SC-004**: All background jobs execute within their configured intervals with less than 10% jitter under normal load
- **SC-005**: All 19 new callback integrations (9 high-value + 10 medium-value) successfully deliver payloads to their target services
- **SC-006**: All management CRUD endpoints (model, tag, customer, config, guardrail, prompt) complete within 200ms
- **SC-007**: Spend log cold storage archival processes 1 million log entries per hour
- **SC-008**: 12 new native providers pass contract tests with real provider API responses
- **SC-009**: 95% of Python TianjiLLM enterprise features (policy engine, SCIM, scheduler, management APIs) have functional equivalents in tianjiLLM
- **SC-010**: All new functionality follows the self-registration plugin pattern, requiring zero changes to existing code when adding new integrations
- **SC-011**: Policy engine correctly rejects circular inheritance at creation time with a clear error message
- **SC-012**: SCIM duplicate user provisioning (same email) updates the existing user instead of creating a duplicate
- **SC-013**: Budget reset catches up on all missed resets within 60 seconds after proxy restart from downtime
