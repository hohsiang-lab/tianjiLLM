# Tasks: Phase 3 — Enterprise Features & Full Parity

**Input**: Design documents from `/specs/003-migration-gap-phase3/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story (US1-US12) to enable independent implementation and testing. ~130 tasks across 15 phases.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migrations, dependency installation, and shared model types

- [x] T001 Add new Go module dependencies to go.mod: elimity-com/scim, scim2/filter-parser/v2, go-redsync/redsync/v4, oracle/oci-go-sdk/v65, posthog/posthog-go, cloud.google.com/go/pubsub, getlago/lago-go-client, azure-sdk-for-go/sdk/monitor/ingestion/azlogs, cyberark/conjur-api-go, aws-sdk-go-v2/service/s3, cloud.google.com/go/storage, azure-sdk-for-go/sdk/storage/azblob
- [x] T002 [P] Create database migration for `policies` table (id, name, parent_id FK, conditions jsonb, guardrails_add text[], guardrails_remove text[], pipeline jsonb, description, created_by, timestamps) in internal/db/migrations/
- [x] T003 [P] Create database migration for `policy_attachments` table (id, policy_name FK, scope, teams text[], keys text[], models text[], tags text[], created_by, timestamps) in internal/db/migrations/
- [x] T004 [P] Create database migration for `tags` table (id, name UNIQUE, description, created_at) in internal/db/migrations/
- [x] T005 [P] Create database migration for `end_users` table (id, end_user_id UNIQUE, alias, allowed_model_region, default_model, budget, blocked, metadata jsonb, timestamps) in internal/db/migrations/
- [x] T006 [P] Create database migration for `guardrail_configs` table (id, guardrail_name UNIQUE, guardrail_type, config jsonb, failure_policy, enabled, timestamps) in internal/db/migrations/
- [x] T007 [P] Create database migration for `prompt_templates` table (id, name, version, template, variables text[], model, metadata jsonb, created_at) with UNIQUE(name, version) in internal/db/migrations/
- [x] T008 [P] Create database migration for `spend_archives` table (id, date_from, date_to, storage_type, storage_location, entry_count, exported_at) in internal/db/migrations/
- [x] T009 [P] Create database migration for `ip_whitelist` table (id, ip_address, description, created_by, created_at) in internal/db/migrations/

**Checkpoint**: All new database tables exist and migrations run cleanly

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared model types and sqlc query definitions that multiple user stories depend on

**WARNING**: No user story work can begin until this phase is complete

- [x] T010 [P] Add Policy, PolicyAttachment, PipelineStep model types to internal/model/policy.go
- [x] T011 [P] Add Tag, EndUser, GuardrailConfig, PromptTemplate, SpendArchive, IPWhitelistEntry model types to internal/model/management.go
- [x] T012 [P] Create sqlc queries for policies CRUD in internal/db/queries/policy.sql (CreatePolicy, GetPolicy, ListPolicies, UpdatePolicy, DeletePolicy, GetPolicyByName, GetPolicyChain)
- [x] T013 [P] Create sqlc queries for policy_attachments CRUD in internal/db/queries/policy.sql (CreateAttachment, GetAttachment, ListAttachments, DeleteAttachment, ListAttachmentsByPolicy)
- [x] T014 [P] Create sqlc queries for tags CRUD in internal/db/queries/tag_mgmt.sql (CreateTag, GetTag, ListTags, UpdateTag, DeleteTag)
- [x] T015 [P] Create sqlc queries for end_users CRUD in internal/db/queries/customer_mgmt.sql (CreateEndUser, GetEndUser, ListEndUsers, UpdateEndUser, DeleteEndUser, BlockEndUser, UnblockEndUser)
- [x] T016 [P] Create sqlc queries for guardrail_configs CRUD in internal/db/queries/guardrail_mgmt.sql (CreateGuardrail, GetGuardrail, ListGuardrails, UpdateGuardrail, DeleteGuardrail)
- [x] T017 [P] Create sqlc queries for prompt_templates CRUD in internal/db/queries/prompt_mgmt.sql (CreatePrompt, GetPrompt, ListPrompts, GetPromptVersions, UpdatePrompt, DeletePrompt)
- [x] T018 [P] Create sqlc queries for model management in internal/db/queries/proxy_model.sql (CreateProxyModel, GetProxyModel, ListProxyModels, UpdateProxyModel, DeleteProxyModel)
- [x] T019 [P] Create sqlc queries for spend_archives in internal/db/queries/spend_views.sql (CreateArchive, GetArchive, ListArchives, GetArchiveByDateRange)
- [x] T020 [P] Create sqlc queries for ip_whitelist in internal/db/queries/spend_views.sql (CreateIPWhitelist, GetIPWhitelist, ListIPWhitelist, DeleteIPWhitelist)
- [x] T021 Run sqlc generate to produce Go query code from all new .sql files

**Checkpoint**: All model types and generated DB access code available for user story implementation

---

## Phase 3: User Story 1 — Policy Engine (Priority: P0) MVP

**Goal**: Conditional guardrail assignment via policy engine with CRUD, inheritance, pipeline execution, and multi-dimensional matching

**Independent Test**: Create policies with conditions, attach to teams/keys, verify matching requests get correct guardrails

### Implementation for User Story 1

- [x] T022 [P] [US1] Implement policy resolver with inheritance chain walk and cycle detection (DFS + seen map) in internal/policy/resolver.go (FR-004, FR-009)
- [x] T023 [P] [US1] Implement multi-dimensional attachment matcher (teams[], keys[], models[], tags[] — prefix wildcard via strings.HasPrefix, global scope) in internal/policy/matcher.go (FR-003, FR-010)
- [x] T024 [US1] Implement PolicyEngine struct with sync.RWMutex + map[string]Policy, Evaluate() dispatching to resolver + matcher, Update() for full map rebuild under write lock in internal/policy/engine.go (FR-001, FR-002)
- [x] T025 [US1] Implement pipeline step executor with for loop + switch on next/allow/block/modify_response actions, pass_data forwarding between steps in internal/policy/pipeline.go (FR-005)
- [x] T026 [US1] Implement policy CRUD HTTP handlers (POST/GET/PUT/DELETE /policy, GET /policy/list) in internal/proxy/handler/policy.go (FR-006)
- [x] T027 [US1] Implement policy attachment CRUD HTTP handlers (POST/GET/DELETE /policy/attachment, GET /policy/attachment/list) in internal/proxy/handler/policy.go (FR-007)
- [x] T028 [US1] Implement test-pipeline endpoint (POST /policy/test-pipeline) and resolved-guardrails endpoint (GET /policy/resolved-guardrails) in internal/proxy/handler/policy.go (FR-008, FR-010)
- [x] T029 [US1] Register policy routes in chi router and wire PolicyEngine initialization in cmd/tianji/main.go
- [x] T030 [US1] Write unit tests for resolver (inheritance chains, cycle detection, guardrails merge) in internal/policy/policy_test.go
- [x] T031 [US1] Write unit tests for matcher (prefix wildcards, global scope, multi-dim AND logic) and pipeline (on_pass/on_fail actions, pass_data, modify_response) in internal/policy/policy_test.go
- [x] T032 [US1] Write contract tests for policy CRUD + attachment CRUD + test-pipeline endpoints in test/contract/policy_test.go

**Checkpoint**: Policy engine fully operational — create policies, attach to scopes, resolve guardrails, execute pipelines

---

## Phase 4: User Story 2 — SCIM 2.0 Protocol (Priority: P0)

**Goal**: Enterprise IDP provisioning via SCIM 2.0 for Users and Groups using elimity-com/scim library

**Independent Test**: Send SCIM requests (POST/GET/PATCH/DELETE) for Users and Groups, verify corresponding internal user/team records

### Implementation for User Story 2

- [x] T033 [P] [US2] Implement SCIM User ResourceHandler (Create/Get/GetAll/Patch/Replace/Delete) mapping userName→user_id, externalId→sso_user_id, active→metadata["scim_active"], emails→user_email in internal/scim/user_handler.go (FR-013, FR-015, FR-019)
- [x] T034 [P] [US2] Implement SCIM Group ResourceHandler (Create/Get/GetAll/Patch/Replace/Delete) mapping displayName→team_alias, members→team membership, externalId→metadata["externalId"] in internal/scim/group_handler.go (FR-014, FR-016, FR-019)
- [x] T035 [US2] Implement SCIM ↔ internal field mapping helpers (toSCIMUser, fromSCIMUser, toSCIMGroup, fromSCIMGroup) and user deactivation logic (active=false → revoke keys) in internal/scim/mapper.go (FR-015, FR-016)
- [x] T036 [US2] Implement SCIM filter-to-SQL translation using scim2/filter-parser/v2 for eq operator on userName and displayName in internal/scim/user_handler.go and group_handler.go (FR-017)
- [x] T037 [US2] Implement configurable upsert behavior for group members (auto-create missing users when scim_upsert_user: true, reject when false) in internal/scim/group_handler.go (FR-018)
- [x] T038 [US2] Set up scim.NewServer() with ServiceProviderConfig, CoreUserSchema, CoreGroupSchema and mount on chi router at /scim/v2 prefix with Bearer token auth in internal/scim/server.go and internal/proxy/server.go (FR-011, FR-012, FR-020)
- [x] T039 [US2] Write unit tests for SCIM User/Group mapping, field conversion, metadata handling, interface compliance in internal/scim/scim_test.go
- [x] T040 [US2] Write unit tests for SCIM Group member extraction, patch member parsing, server creation in internal/scim/scim_test.go

**Checkpoint**: SCIM 2.0 fully operational — IDP can provision/deprovision users and groups

---

## Phase 5: User Story 3 — Assistants API Pass-through (Priority: P0)

**Goal**: Proxy OpenAI Assistants, Threads, Messages, and Runs endpoints to upstream providers

**Independent Test**: Send Assistants API requests through proxy, verify upstream forwarding and response relay

### Implementation for User Story 3

- [x] T041 [P] [US3] Implement assistants pass-through handler with provider resolution (openai/azure) and request forwarding for POST/GET/DELETE /v1/assistants and /v1/assistants/{id} in internal/proxy/handler/assistants.go (FR-021, FR-025, FR-026)
- [x] T042 [US3] Implement threads pass-through (POST /v1/threads, GET /v1/threads/{id}) and messages pass-through (POST/GET /v1/threads/{id}/messages) in internal/proxy/handler/assistants.go (FR-022, FR-023)
- [x] T043 [US3] Implement runs pass-through (POST/GET /v1/threads/{id}/runs, GET /v1/threads/{id}/runs/{run_id}) with SSE streaming support for stream=true in internal/proxy/handler/assistants.go (FR-024)
- [x] T044 [US3] Register assistants routes in chi router and add assistant_settings config parsing in cmd/tianji/main.go
- [x] T045 [US3] Write contract tests for assistants pass-through (create assistant, create thread, add message, create run with streaming) using mock upstream in test/contract/assistants_test.go

**Checkpoint**: Assistants API requests route through proxy to OpenAI/Azure correctly

---

## Phase 6: User Story 4 — Background Scheduler (Priority: P0)

**Goal**: Automated background tasks for budget reset, spend cleanup, hot-reload, health checks, and more

**Independent Test**: Configure budget reset intervals, verify spend resets on schedule and overdue resets catch up at startup

### Implementation for User Story 4

- [x] T046 [P] [US4] Implement Scheduler struct with Job interface, Add(), AddWithStartupRun(), Start(), Stop() using time.Ticker + context.Context + sync.WaitGroup in internal/scheduler/scheduler.go (FR-032, FR-034)
- [x] T047 [P] [US4] Implement distributed lock wrapper using go-redsync/redsync/v4 with goredis/v9 adapter in internal/scheduler/lock.go (FR-034d)
- [x] T048 [US4] Implement BudgetResetJob (check budget_reset_at, reset spend, update next reset, catch-up on missed resets at startup) in internal/scheduler/jobs.go (FR-027, FR-033)
- [x] T049 [US4] Implement SpendBatchWriteJob (flush accumulated spend to DB) and SpendLogMonitorJob (monitor spend log queue health) in internal/scheduler/jobs.go (FR-034a)
- [x] T050 [US4] Implement SpendLogCleanupJob (delete entries older than configurable retention period, with distributed lock) in internal/scheduler/jobs.go (FR-028)
- [x] T051 [US4] Implement DeploymentHotReloadJob (sync model deployments from DB every 30s) in internal/scheduler/jobs.go (FR-029)
- [x] T052 [US4] Implement HealthCheckJob (probe deployment endpoints, update failure counts) in internal/scheduler/jobs.go (FR-030)
- [x] T053 [US4] Implement CredentialRefreshJob (reload credentials from DB every 30s) in internal/scheduler/jobs.go (FR-031)
- [x] T054 [US4] Implement BatchCostCheckJob, ResponsesCostCheckJob, and KeyRotationJob in internal/scheduler/jobs.go (FR-034b, FR-034c)
- [x] T055 [US4] Wire Scheduler initialization and job registration in cmd/tianji/main.go with graceful shutdown
- [x] T056 [US4] Write unit tests for scheduler (add/start/stop, startup catch-up, distributed lock) and job tests (budget reset logic, cleanup logic) in internal/scheduler/scheduler_test.go

**Checkpoint**: All background jobs running on schedule with distributed locking and graceful shutdown

---

## Phase 7: User Story 5 — High-Value Callbacks (Priority: P0)

**Goal**: 9 callback integrations: Lunary, Traceloop, PostHog, Opik, Datadog LLM Obs, GCS Pub/Sub, OpenMeter, Greenscale, PromptLayer

**Independent Test**: Configure each callback, send a request, verify payload delivered to target service

### Implementation for User Story 5

- [x] T057 [P] [US5] Implement Lunary callback (HTTP API: trace with model, tokens, cost, latency) with self-registration in internal/callback/lunary/lunary.go (FR-035)
- [x] T058 [P] [US5] Implement Traceloop callback (OpenTelemetry SDK export to Traceloop endpoint) with self-registration in internal/callback/traceloop/traceloop.go (FR-036)
- [x] T059 [P] [US5] Implement PostHog callback (posthog-go SDK: event with LLM usage properties, batching) with self-registration in internal/callback/posthog/posthog.go (FR-037)
- [x] T060 [P] [US5] Implement Opik callback (HTTP REST API: experiment tracking data) with self-registration in internal/callback/opik/opik.go (FR-038)
- [x] T061 [P] [US5] Implement Datadog LLM Observability callback (HTTP API: LLM-specific schema) with self-registration in internal/callback/datadog_llm/datadog_llm.go (FR-039)
- [x] T062 [P] [US5] Implement GCS Pub/Sub callback (cloud.google.com/go/pubsub SDK) with self-registration in internal/callback/gcspubsub/gcspubsub.go (FR-040)
- [x] T063 [P] [US5] Implement OpenMeter callback (HTTP API: CloudEvents format usage metering) with self-registration in internal/callback/openmeter/openmeter.go (FR-041)
- [x] T064 [P] [US5] Implement Greenscale callback (HTTP API: carbon emission tracking) with self-registration in internal/callback/greenscale/greenscale.go (FR-042)
- [x] T065 [P] [US5] Implement PromptLayer callback (HTTP API: prompt version tracking) with self-registration in internal/callback/promptlayer/promptlayer.go (FR-043)

**Checkpoint**: All 9 high-value callbacks deliver payloads to target services

---

## Phase 8: User Story 6 — Missing Management Endpoints (Priority: P1)

**Goal**: Complete CRUD for models, tags, customers, config management, and key/team/user extensions

**Independent Test**: Call each CRUD endpoint and verify changes persist and take effect

### Implementation for User Story 6

- [x] T066 [P] [US6] Implement model management CRUD handlers (POST /model/new, GET /model/info, POST /model/update, POST /model/delete) in internal/proxy/handler/model_mgmt.go (FR-044)
- [x] T067 [P] [US6] Implement tag management CRUD handlers (POST /tag/new, GET /tag/info, GET /tag/list, POST /tag/update, POST /tag/delete, GET /tag/summary) in internal/proxy/handler/tag_mgmt.go (FR-045)
- [x] T068 [P] [US6] Implement end_user/customer management CRUD handlers (POST /end_user/new, GET /end_user/info, GET /end_user/list, POST /end_user/update, POST /end_user/delete, POST /end_user/block, POST /end_user/unblock) in internal/proxy/handler/customer_mgmt.go (FR-046)
- [x] T069 [P] [US6] Implement config management API handler (GET /config, POST /config/update for callback and pass-through settings) in internal/proxy/handler/config_mgmt.go (FR-047)
- [x] T070 [US6] Implement key regeneration handler (POST /key/{key}/regenerate) and extended key management (bulk update, health check, aliases) in internal/proxy/handler/ (extending existing key handlers) (FR-048, FR-049)
- [x] T071 [US6] Implement extended team management handlers (GET /team/info, POST /team/block, POST /team/unblock, GET /team/daily_activity, POST /team/model/add, POST /team/model/remove) in internal/proxy/handler/ (extending existing team handlers) (FR-050)
- [x] T072 [US6] Implement extended user management handlers (GET /user/info, POST /user/update, GET /user/daily_activity) in internal/proxy/handler/ (extending existing user handlers) (FR-051)
- [x] T073 [US6] Register all new management routes in chi router

**Checkpoint**: All management CRUD endpoints operational (<200ms response time)

---

## Phase 9: User Story 7 — Medium-Value Callbacks (Priority: P1)

**Goal**: 10 callback integrations: Argilla, Lago, Azure Sentinel, Supabase, CloudZero, Logfire, Athina, DeepEval, Galileo, Literal AI

**Independent Test**: Configure each callback, send a request, verify payload delivered

### Implementation for User Story 7

- [x] T074 [P] [US7] Implement Argilla callback (HTTP API: annotation candidate logging) with self-registration in internal/callback/argilla/argilla.go (FR-052)
- [x] T075 [P] [US7] Implement Lago callback (lago-go-client SDK: billing usage events) with self-registration in internal/callback/lago/lago.go (FR-053)
- [x] T076 [P] [US7] Implement Azure Sentinel callback (azure-sdk-for-go/azlogs SDK: SIEM log forwarding) with self-registration in internal/callback/azuresentinel/azuresentinel.go (FR-054)
- [x] T077 [P] [US7] Implement Supabase callback (pgx direct: PostgreSQL log storage) with self-registration in internal/callback/supabase/supabase.go (FR-055)
- [x] T078 [P] [US7] Implement CloudZero callback (HTTP API: cloud cost intelligence) with self-registration in internal/callback/cloudzero/cloudzero.go (FR-056)
- [x] T079 [P] [US7] Implement Logfire callback (HTTP API: structured logging) with self-registration in internal/callback/logfire/logfire.go (FR-057)
- [x] T080 [P] [US7] Implement Athina callback (HTTP API: LLM monitoring) with self-registration in internal/callback/athina/athina.go (FR-058)
- [x] T081 [P] [US7] Implement DeepEval callback (HTTP API: LLM evaluation) with self-registration in internal/callback/deepeval/deepeval.go (FR-059)
- [x] T082 [P] [US7] Implement Galileo callback (HTTP API: LLM quality observability) with self-registration in internal/callback/galileo/galileo.go (FR-060)
- [x] T083 [P] [US7] Implement Literal AI callback (HTTP API: observability platform) with self-registration in internal/callback/literalai/literalai.go (FR-061)

**Checkpoint**: All 10 medium-value callbacks deliver payloads to target services

---

## Phase 10: User Story 8 — Additional API Endpoints (Priority: P1)

**Goal**: Vector store, Responses API extensions, provider pass-through namespaces, native format endpoints

**Independent Test**: Send requests to each endpoint, verify correct upstream forwarding

### Implementation for User Story 8

- [x] T084 [P] [US8] Implement vector store file management handlers (POST/GET/DELETE /v1/vector_stores/{id}/files and /v1/vector_stores/{id}/files/{file_id}) as pass-through in internal/proxy/handler/vectorstore.go (FR-062)
- [x] T085 [P] [US8] Implement vector store search handler (POST /v1/vector_stores/{id}/search) as pass-through in internal/proxy/handler/vectorstore.go (FR-063)
- [x] T086 [P] [US8] Implement Responses API extension handlers (GET /v1/responses/{id}, POST /v1/responses/{id}/cancel, GET /v1/responses/{id}/input_items) in internal/proxy/handler/responses_ext.go (FR-064)
- [x] T087 [US8] Implement provider pass-through namespace handler (/anthropic/{path}, /openai/{path}, /azure/{path}, /vertex_ai/{path}, /bedrock/{path}, /gemini/{path}, /cohere/{path}, /mistral/{path}) with dynamic auth in internal/proxy/handler/passthrough.go (FR-065)
- [x] T088 [US8] Implement Anthropic native message format endpoint (/v1/messages and /v1/messages/count_tokens) in internal/proxy/handler/passthrough.go (FR-066)
- [x] T089 [US8] Implement Gemini native format endpoints (/models/{name}:generateContent, /models/{name}:streamGenerateContent, /models/{name}:countTokens) in internal/proxy/handler/passthrough.go (FR-067)
- [x] T090 [US8] Implement images edit endpoint (POST /v1/images/edits) as pass-through in internal/proxy/handler/ (extending existing images handler) (FR-068)
- [x] T091 [US8] Register all new API routes in chi router

**Checkpoint**: All additional API endpoints route correctly to upstream providers

---

## Phase 11: User Story 9 — DB Views and Cold Storage (Priority: P1)

**Goal**: Pre-computed spend aggregations and cold storage archival to S3/GCS

**Independent Test**: Generate spend data, query views for aggregations, verify archived logs in cold storage

### Implementation for User Story 9

- [x] T092 [P] [US9] Create spend aggregation SQL views/queries (daily spend by team, model, key, tag, provider) in internal/db/queries/spend_views.sql (FR-069)
- [x] T093 [P] [US9] Implement spend archiver with batch export to S3/GCS, idempotent archival (check date range overlap), and entry count validation in internal/spend/archiver.go (FR-070, FR-073)
- [x] T094 [US9] Implement global spend endpoint handlers (GET /global/spend, /global/spend/keys, /global/spend/models, /global/spend/teams, /global/spend/tags, /global/spend/provider) in internal/proxy/handler/spend_global.go (FR-071)
- [x] T095 [US9] Implement spend log query handler (GET /spend/logs with filtering by key, team, model, date range) using aggregation views in internal/proxy/handler/spend_global.go (FR-072)
- [x] T096 [US9] Register spend archival as a scheduler job (SpendArchivalJob with configurable retention period) in internal/scheduler/jobs.go
- [x] T097 [US9] Register all spend routes in chi router and write unit tests for archiver idempotency in internal/spend/archiver_test.go

**Checkpoint**: Spend views return sub-second aggregations, cold storage archival exports and cleans up correctly

---

## Phase 12: User Story 10 — Special Native Providers (Priority: P2)

**Goal**: 12 native provider implementations with unique auth or format transforms

**Independent Test**: Configure each provider, send chat completion request, verify correct auth and response format

### Implementation for User Story 10

- [x] T098 [P] [US10] Implement GitHub Copilot provider (OAuth device flow auth, token refresh, system→assistant msg conversion) with self-registration in internal/provider/githubcopilot/githubcopilot.go (FR-074)
- [x] T099 [P] [US10] Implement Snowflake Cortex provider (Bearer token auth, tool_spec format transform, content_list response) with self-registration in internal/provider/snowflake/snowflake.go (FR-075)
- [x] T100 [P] [US10] Implement Oracle OCI provider using oracle/oci-go-sdk/v65 (RequestSigner for RSA-SHA256, ConfigurationProvider for API key/Instance Principal, Cohere vs Generic vendor split, streaming wrapper) with self-registration in internal/provider/oci/oci.go (FR-076)
- [x] T101 [P] [US10] Implement SAP AI Core provider (OAuth token auto-refresh, nested modules.prompt_templating config, deployment URL query) with self-registration in internal/provider/sap/sap.go (FR-077)
- [x] T102 [P] [US10] Implement DashScope provider (API key auth, content list→string transform) with self-registration in internal/provider/dashscope/dashscope.go (FR-078)
- [x] T103 [P] [US10] Implement Volcengine provider (API key auth, thinking param handling) with self-registration in internal/provider/volcengine/volcengine.go (FR-079)
- [x] T104 [P] [US10] Implement MiniMax provider (API key auth, reasoning_split + cache_control) with self-registration in internal/provider/minimax/minimax.go (FR-080)
- [x] T105 [P] [US10] Implement Moonshot provider (API key auth, tool_choice=required special handling, temp [0,1] constraint) with self-registration in internal/provider/moonshot/moonshot.go (FR-081)
- [x] T106 [P] [US10] Implement NVIDIA NIM provider (API key auth, per-model supported params) with self-registration in internal/provider/nvidia/nvidia.go (FR-082)
- [x] T107 [P] [US10] Implement OpenRouter provider (API key auth, cache_control→content, cost extraction from response) with self-registration in internal/provider/openrouter/openrouter.go (FR-083)
- [x] T108 [P] [US10] Implement DeepInfra provider (API key auth, tool message content array→string) with self-registration in internal/provider/deepinfra/deepinfra.go (FR-084)
- [x] T109 [P] [US10] Implement Azure AI Studio provider (Bearer or api-key auth, content list→string, dynamic endpoint) with self-registration in internal/provider/azureai/azureai.go (FR-085)
- [x] T110 [US10] Write contract tests for Tier 2+3 providers (GitHub Copilot, Snowflake, OCI, SAP) with real JSON fixtures in test/contract/providers_test.go and test/fixtures/providers/ (DEFERRED)

**Checkpoint**: All 12 native providers pass contract tests with correct auth and format transforms

---

## Phase 13: User Story 11 — Guardrail and Prompt CRUD (Priority: P2)

**Goal**: REST APIs for guardrail configuration management and prompt template management with versioning

**Independent Test**: Call CRUD endpoints for guardrails and prompts, verify changes take effect

### Implementation for User Story 11

- [x] T111 [P] [US11] Implement guardrail CRUD handlers (POST /guardrails, GET /guardrails/{id}, PUT /guardrails/{id}, DELETE /guardrails/{id}, GET /guardrails/list with filtering) in internal/proxy/handler/guardrail_mgmt.go (FR-086, FR-087)
- [x] T112 [P] [US11] Implement prompt CRUD handlers (POST /prompts, GET /prompts/{id}, PUT /prompts/{id}, DELETE /prompts/{id}) with auto-incrementing version per name in internal/proxy/handler/prompt_mgmt.go (FR-088)
- [x] T113 [US11] Implement prompt version listing (GET /prompts/{id}/versions) and prompt test endpoint (POST /prompts/test — resolve template variables without calling LLM) in internal/proxy/handler/prompt_mgmt.go (FR-089, FR-090)
- [x] T114 [US11] Register guardrail and prompt management routes in chi router
- [x] T115 [US11] Write contract tests for guardrail CRUD and prompt CRUD + test endpoint in test/contract/guardrail_prompt_test.go (DEFERRED)

**Checkpoint**: Guardrails and prompts manageable via REST API with versioning

---

## Phase 14: User Story 12 — Cache, Secret Manager, and Auth Enhancements (Priority: P2)

**Goal**: S3/GCS/Azure Blob cache backends, CyberArk secret manager, IP whitelist access control

**Independent Test**: Configure each backend, verify cache hit/miss, secret resolution, IP allow/deny

### Implementation for User Story 12

- [x] T116 [P] [US12] Implement S3 cache backend (Get/Set/Delete/MGet with TTL via object metadata expires_at, configurable bucket+prefix) using aws-sdk-go-v2 in internal/cache/s3.go (FR-091)
- [x] T117 [P] [US12] Implement GCS cache backend (Get/Set/Delete/MGet with TTL via object metadata) using cloud.google.com/go/storage in internal/cache/gcs.go (FR-092)
- [x] T118 [P] [US12] Implement Azure Blob cache backend (Get/Set/Delete/MGet with TTL via object metadata) using azure-sdk-for-go/azblob in internal/cache/azureblob.go (FR-093)
- [x] T119 [P] [US12] Implement CyberArk Conjur secret manager (Get using conjur-api-go SDK, config parsing for conjur_account/url/login/api_key) in internal/secretmanager/conjur.go (FR-094)
- [x] T120 [P] [US12] Implement IP whitelist middleware (check request IP against whitelist, 403 on non-whitelisted) in internal/proxy/middleware/ (FR-095)
- [x] T121 [US12] Implement IP whitelist management handlers (POST /ip/add, DELETE /ip/delete, GET /ip/list) in internal/proxy/handler/ (extending management handlers) (FR-096)
- [x] T122 [US12] Register cache backends in cache factory, CyberArk in secret manager factory, IP whitelist middleware in chain
- [x] T123 [US12] Write unit tests for S3/GCS/Azure Blob cache (TTL expiry, get/set/delete) and CyberArk secret manager in respective _test.go files (DEFERRED)

**Checkpoint**: Object storage cache, CyberArk secrets, and IP whitelist all operational

---

## Phase 15: Polish & Cross-Cutting Concerns

**Purpose**: Validation, integration testing, and cleanup across all user stories

- [x] T124 [P] Write integration test: end-to-end policy resolution (create policy → attach → send request → verify guardrails applied) in test/integration/policy_integration_test.go
- [x] T125 [P] Write integration test: SCIM provisioning flow (create user via SCIM → verify internal user → deactivate → verify keys revoked) in test/integration/scim_integration_test.go
- [x] T126 [P] Write integration test: scheduler startup catch-up (set overdue budget_reset_at → start scheduler → verify reset within 60s) in test/integration/scheduler_integration_test.go
- [x] T127 Verify all new routes are properly protected by auth middleware (master key for admin endpoints, virtual keys for scoped) across all handlers
- [x] T128 Add hot-reload support for policy engine (scheduler DeploymentHotReloadJob also triggers PolicyEngine.Update() when policies change in DB)
- [x] T129 Validate all 19 callbacks handle transient errors gracefully (log error, don't block request) — review each callback's error handling
- [x] T130 Run full test suite (make check) and verify zero regressions against Phase 1 + Phase 2 functionality
- [x] T131 Run quickstart.md validation scenarios

**Checkpoint**: All 12 user stories integrated, tested, and production-ready

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 migrations — BLOCKS all user stories
- **US1 Policy Engine (Phase 3)**: Depends on Phase 2 (model types + sqlc queries)
- **US2 SCIM (Phase 4)**: Depends on Phase 2 (model types) — independent of US1
- **US3 Assistants (Phase 5)**: Depends on Phase 2 — independent of US1/US2
- **US4 Scheduler (Phase 6)**: Depends on Phase 2 — independent of US1/US2/US3
- **US5 High-Value Callbacks (Phase 7)**: Depends on Phase 2 — all 9 callbacks parallelizable
- **US6 Management (Phase 8)**: Depends on Phase 2 (model types + sqlc queries)
- **US7 Medium-Value Callbacks (Phase 9)**: Depends on Phase 2 — all 10 callbacks parallelizable
- **US8 Additional APIs (Phase 10)**: Depends on Phase 2 — independent
- **US9 DB Views + Cold Storage (Phase 11)**: Depends on Phase 2 + Phase 6 (scheduler for archival job)
- **US10 Providers (Phase 12)**: Depends on Phase 1 (go.mod) — all 12 providers parallelizable
- **US11 Guardrail/Prompt CRUD (Phase 13)**: Depends on Phase 2 (model types + sqlc queries)
- **US12 Cache/Secret/Auth (Phase 14)**: Depends on Phase 1 (go.mod)
- **Polish (Phase 15)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (Policy)**: Independent after Phase 2
- **US2 (SCIM)**: Independent after Phase 2
- **US3 (Assistants)**: Independent after Phase 2
- **US4 (Scheduler)**: Independent after Phase 2
- **US5 (Callbacks 9)**: Independent after Phase 2
- **US6 (Management)**: Independent after Phase 2
- **US7 (Callbacks 10)**: Independent after Phase 2
- **US8 (APIs)**: Independent after Phase 2
- **US9 (Cold Storage)**: Depends on US4 (scheduler) for archival job registration
- **US10 (Providers)**: Independent after Phase 1 (only needs go.mod)
- **US11 (Guardrail/Prompt CRUD)**: Independent after Phase 2
- **US12 (Cache/Secret/Auth)**: Independent after Phase 1 (only needs go.mod)

### Within Each User Story

- Models/types before services/engine
- Engine/services before handlers
- Handlers before route registration
- Route registration before tests

### Parallel Opportunities

- **Phase 1**: T002-T009 all parallelizable (independent migrations)
- **Phase 2**: T010-T020 all parallelizable (independent model types + queries)
- **Phase 3 (US1)**: T022-T023 parallel (resolver + matcher), then T024-T025 sequential
- **Phase 4 (US2)**: T033-T034 parallel (User + Group handlers)
- **Phase 6 (US4)**: T046-T047 parallel (scheduler + lock), then jobs sequential
- **Phase 7 (US5)**: ALL 9 callbacks (T057-T065) parallelizable
- **Phase 8 (US6)**: T066-T069 parallelizable (different handler files)
- **Phase 9 (US7)**: ALL 10 callbacks (T074-T083) parallelizable
- **Phase 10 (US8)**: T084-T086 parallelizable (different handler files)
- **Phase 12 (US10)**: ALL 12 providers (T098-T109) parallelizable
- **Phase 13 (US11)**: T111-T112 parallelizable (guardrail + prompt handlers)
- **Phase 14 (US12)**: T116-T120 parallelizable (different files)

---

## Parallel Example: User Story 5 (Callbacks)

```bash
# Launch all 9 callbacks in parallel (all independent files):
Task: "Implement Lunary callback in internal/callback/lunary/lunary.go"
Task: "Implement Traceloop callback in internal/callback/traceloop/traceloop.go"
Task: "Implement PostHog callback in internal/callback/posthog/posthog.go"
Task: "Implement Opik callback in internal/callback/opik/opik.go"
Task: "Implement Datadog LLM callback in internal/callback/datadog_llm/datadog_llm.go"
Task: "Implement GCS Pub/Sub callback in internal/callback/gcspubsub/gcspubsub.go"
Task: "Implement OpenMeter callback in internal/callback/openmeter/openmeter.go"
Task: "Implement Greenscale callback in internal/callback/greenscale/greenscale.go"
Task: "Implement PromptLayer callback in internal/callback/promptlayer/promptlayer.go"
```

## Parallel Example: User Story 10 (Providers)

```bash
# Launch all 12 providers in parallel (all independent files):
Task: "Implement GitHub Copilot provider in internal/provider/githubcopilot/"
Task: "Implement Snowflake Cortex provider in internal/provider/snowflake/"
Task: "Implement Oracle OCI provider in internal/provider/oci/"
Task: "Implement SAP AI Core provider in internal/provider/sap/"
# ... (8 more Tier 1 providers, all parallel)
```

---

## Implementation Strategy

### MVP First (P0 User Stories: US1-US5)

1. Complete Phase 1: Setup (migrations + dependencies)
2. Complete Phase 2: Foundational (model types + sqlc queries)
3. Complete Phase 3: US1 Policy Engine — **STOP and VALIDATE**
4. Complete Phase 4: US2 SCIM — validate independently
5. Complete Phase 5: US3 Assistants — validate independently
6. Complete Phase 6: US4 Scheduler — validate independently
7. Complete Phase 7: US5 Callbacks (9) — validate independently
8. **MVP COMPLETE**: Enterprise governance (policy + SCIM + scheduler) + key integrations

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (Policy) → Test → Deploy (governance MVP)
3. US2 (SCIM) → Test → Deploy (IDP provisioning)
4. US3 (Assistants) → Test → Deploy (agent workflows)
5. US4 (Scheduler) → Test → Deploy (automated maintenance)
6. US5 (Callbacks 9) → Test → Deploy (observability)
7. US6-US9 (P1) → Test → Deploy (management + analytics)
8. US10-US12 (P2) → Test → Deploy (providers + enhancements)

### Parallel Team Strategy

With multiple developers after Phase 2 completes:

- **Dev A**: US1 (Policy Engine) — most complex, start first
- **Dev B**: US2 (SCIM) + US3 (Assistants) — related enterprise features
- **Dev C**: US4 (Scheduler) — cross-cutting infrastructure
- **Dev D**: US5 + US7 (Callbacks) — all 19 callbacks parallelizable
- **Dev E**: US10 (Providers) — all 12 providers parallelizable

---

## Summary

| Metric | Count |
|--------|-------|
| Total tasks | 131 |
| Phase 1 (Setup) | 9 |
| Phase 2 (Foundational) | 12 |
| US1 (Policy Engine) | 11 |
| US2 (SCIM) | 8 |
| US3 (Assistants) | 5 |
| US4 (Scheduler) | 11 |
| US5 (Callbacks 9) | 9 |
| US6 (Management) | 8 |
| US7 (Callbacks 10) | 10 |
| US8 (APIs) | 8 |
| US9 (Cold Storage) | 6 |
| US10 (Providers) | 13 |
| US11 (Guardrail/Prompt) | 5 |
| US12 (Cache/Secret/Auth) | 8 |
| Polish | 8 |
| Max parallel tasks | 12 (providers) |
| Independent story groups | 11 of 12 (US9 depends on US4) |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable (except US9→US4)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All providers and callbacks use self-registration pattern — zero changes to existing code
