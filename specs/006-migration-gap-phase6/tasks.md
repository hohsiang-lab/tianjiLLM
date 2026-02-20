# Tasks: Phase 6 Migration Gap Closure — Full Parity

**Input**: Design documents from `/specs/006-migration-gap-phase6/`
**Prerequisites**: spec.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Wave 1: Setup (Shared Infrastructure)

**Purpose**: Database migrations and shared types for Phase 6 features

- [X] T001 Create `internal/db/schema/007_audit.sql` — tables: `audit_log` (id UUID PK, updated_at, changed_by, changed_by_api_key, action, table_name, object_id, before_value JSONB, updated_values JSONB); `deleted_verification_token` (mirrors verification_token + deleted_at, deleted_by, deleted_by_api_key, tianji_changed_by); `deleted_team_table` (mirrors team_table + deleted_at, deleted_by, deleted_by_api_key, tianji_changed_by); indexes on deleted_at, token, team_id, user_id, organization_id
- [X] T002 [P] Create `internal/db/schema/008_skills_agents.sql` — tables: `agents_table` (agent_id UUID PK, agent_name UNIQUE, tianji_params JSONB, agent_card_params JSONB, agent_access_groups TEXT[], created_at, created_by, updated_at, updated_by); `daily_agent_spend` (id UUID PK, agent_id, date, api_key, model, model_group, custom_llm_provider, mcp_namespaced_tool_name, endpoint, prompt_tokens BIGINT, completion_tokens BIGINT, cache_read_input_tokens BIGINT, cache_creation_input_tokens BIGINT, spend FLOAT, api_requests BIGINT, successful_requests BIGINT, failed_requests BIGINT, created_at, updated_at; UNIQUE on (agent_id, date, api_key, model, custom_llm_provider, mcp_namespaced_tool_name, endpoint)); `skills_table` (skill_id UUID PK, display_title, description, instructions, source DEFAULT 'custom', latest_version, file_content BYTEA, file_name, file_type, metadata JSONB, created_at, created_by, updated_at, updated_by); `claude_code_plugin_table` (id UUID PK, name UNIQUE, version, description, manifest_json JSONB, files_json JSONB, enabled BOOLEAN DEFAULT true, source, source_url, created_by, created_at, updated_at)
- [X] T003 [P] Create `internal/db/schema/009_extensions.sql` — tables: `health_check_table` (id UUID PK, model_name, status, response_time_ms FLOAT, error_message, checked_at); `error_logs` (id UUID PK, request_id, api_key_hash, model, provider, status_code INT, error_type, error_message, traceback TEXT, created_at); `daily_organization_spend` (id UUID PK, organization_id, date, model, spend FLOAT, prompt_tokens BIGINT, completion_tokens BIGINT, api_requests BIGINT, created_at, updated_at; UNIQUE on (organization_id, date, model)); `daily_end_user_spend` (id UUID PK, end_user_id, date, model, api_key, spend FLOAT, prompt_tokens BIGINT, completion_tokens BIGINT, api_requests BIGINT, created_at, updated_at; UNIQUE on (end_user_id, date, model, api_key))
- [X] T004 Create sqlc queries for audit in `internal/db/queries/audit.sql` — `InsertAuditLog`, `ListAuditLogs` (with filters: changed_by, action, table_name, object_id, date range; pagination), `GetAuditLog`, `InsertDeletedVerificationToken`, `InsertDeletedTeam`; run `make generate`
- [X] T005 [P] Create sqlc queries for agents in `internal/db/queries/agent.sql` — `CreateAgent`, `GetAgent`, `GetAgentByName`, `ListAgents`, `UpdateAgent`, `PatchAgent`, `DeleteAgent`, `ListAgentsByAccessGroups`; run `make generate`
- [X] T006 [P] Create sqlc queries for skills in `internal/db/queries/skill.sql` — `CreateSkill`, `GetSkill`, `ListSkills` (with pagination), `DeleteSkill`; run `make generate`
- [X] T007 [P] Create sqlc queries for marketplace in `internal/db/queries/marketplace.sql` — `CreatePlugin`, `GetPlugin`, `ListPlugins`, `ListEnabledPlugins`, `EnablePlugin`, `DisablePlugin`, `DeletePlugin`; run `make generate`
- [X] T008 [P] Create sqlc queries for extensions in `internal/db/queries/health_check.sql` — `InsertHealthCheck`, `ListHealthChecks`; `internal/db/queries/error_log.sql` — `InsertErrorLog`, `ListErrorLogs`; run `make generate`
- [X] T009 [P] Create sqlc queries for global spend in `internal/db/queries/spend_global.sql` — `GetGlobalActivity` (daily request counts + token totals), `GetGlobalActivityByModel`, `GetGlobalSpendReport` (group by team/customer/key), `ResetAllKeySpend`, `ResetAllTeamSpend`, `GetProviderBudgets`, `GetCacheHitStats`, `ResetKeySpend` (single key); run `make generate`

**Checkpoint**: Run `make generate && make build` to verify all queries compile.

---

## Wave 1, Phase A: US1 — Parallel Request Limiting + Dynamic Rate Limiting (P0)

**Goal**: Enforce max_parallel_requests, dynamic rate limiting, and middleware gaps

- [X] T010 [US1] Create `internal/proxy/middleware/parallel.go` — `ParallelRequestLimiter` struct with Redis client; `Check(ctx, keyHash, limit)` using Redis INCR with TTL (60s window); `Release(ctx, keyHash)` using DECR; `NewParallelRequestMiddleware(redis, db)` — read `max_parallel_requests` from VerificationToken via context, Check on entry, Release on response complete (via `defer` + `http.ResponseWriter` wrapper)
- [X] T011 [P] [US1] Create `internal/proxy/middleware/dynamic_ratelimit.go` — `DynamicRateLimiter` struct; saturation check via Redis counters; three-phase logic: (1) read-only check all limits, (2) decide based on saturation threshold, (3) atomic increment if allowed; priority-based throttling using key metadata `priority` field; `NewDynamicRateLimitMiddleware(redis)` factory
- [X] T012 [P] [US1] Create `internal/proxy/middleware/cache_control.go` — `CacheControlCheck` middleware; read `allowed_cache_controls` from context (loaded by auth middleware from VerificationToken); if request has `cache` param, verify each control is in allowed list; reject with 403 if not
- [X] T013 [P] [US1] Create `internal/proxy/middleware/response_security.go` — `ResponseIDSecurity` hook; encrypt response IDs with `tianji_proxy:responses_api:response_id:{id};user_id:{uid};team_id:{tid}` format using HMAC-SHA256 + base64; decrypt on input; verify user/team ownership; skip for admin users
- [X] T014 [P] [US1] Create `internal/proxy/middleware/batch_ratelimit.go` — `BatchRateLimiter` struct; on batch submission, read input file, count requests + estimate tokens; check against key limits (RPM/TPM); atomic increment to reserve capacity; on completion, adjust for actuals
- [X] T015 [US1] Mount `ModelBudgetLimiter` in `internal/proxy/server.go` HTTP chain — wire existing `ModelBudgetLimiter.Check()` as middleware after auth, before handler dispatch
- [X] T016 [US1] Wire all new middleware in `internal/proxy/server.go` — add `ParallelRequestMiddleware`, `DynamicRateLimitMiddleware`, `CacheControlCheck` to the middleware chain (after auth, before handlers)
- [X] T017 [US1] Write contract test `test/contract/parallel_ratelimit_test.go` — test max_parallel_requests enforcement with concurrent goroutines; test release on response; test Redis unavailable fallback
- [X] T018 [P] [US1] Write contract test `test/contract/cache_control_test.go` — test allowed/rejected cache controls
- [X] T019 [P] [US1] Write contract test `test/contract/response_security_test.go` — test ID encryption/decryption, cross-user access rejection

**Checkpoint**: Run `go test ./test/contract/... -run TestParallel -v && go test ./test/contract/... -run TestCacheControl -v`.

---

## Wave 1, Phase B: US2 — Audit Logging + Soft-Delete (P0)

**Goal**: Full audit trail for management operations + soft-delete for keys/teams

- [X] T020 [US2] Create `internal/proxy/handler/audit.go` — `AuditLogList` handler for `GET /audit` (paginated, filtered by changed_by, action, table_name, object_id, start_date, end_date); `AuditLogGet` handler for `GET /audit/{id}`
- [X] T021 [US2] Create audit helper function in `internal/proxy/handler/audit_helper.go` — `createAuditLog(ctx, db, action, tableName, objectID, changedBy, changedByAPIKey, beforeValue, updatedValues)` — checks `store_audit_logs` config flag, inserts audit record
- [X] T022 [US2] Integrate audit logging into `internal/proxy/handler/key.go` — on KeyGenerate: audit "created"; on KeyUpdate: read before value, audit "updated" with diff; on KeyDelete: copy to DeletedVerificationToken, audit "deleted"
- [X] T023 [P] [US2] Integrate audit logging into `internal/proxy/handler/team.go` — on TeamCreate: audit "created"; on TeamUpdate: audit "updated"; on TeamDelete: copy to DeletedTeamTable, audit "deleted"
- [X] T024 [P] [US2] Integrate audit logging into `internal/proxy/handler/user.go` — on UserCreate: audit "created"; on UserUpdate: audit "updated"; on UserDelete: audit "deleted"
- [X] T025 [US2] Wire audit routes in `internal/proxy/server.go` — add `GET /audit` and `GET /audit/{id}` in management group
- [X] T026 [US2] Write contract test `test/contract/audit_test.go` — test audit log creation on key/team/user CRUD; test soft-delete writes to deleted tables; test audit list with filters + pagination
- [X] T027 [US2] Add `store_audit_logs` boolean field to `internal/config/config.go` general settings

**Checkpoint**: Run `go test ./test/contract/... -run TestAudit -v`.

---

## Wave 1, Phase C: US3 — Missing Management Endpoints (P0)

**Goal**: Complete key/team/spend management API surface

### Key Management Extensions

- [X] T028 [US3] Add `ServiceAccountKeyGenerate` handler in `internal/proxy/handler/key_ext.go` — `POST /key/service-account/generate`: create key with team_id but no user_id, apply team limits
- [X] T029 [P] [US3] Add `ResetKeySpend` handler in `internal/proxy/handler/key_ext.go` — `POST /key/{key}/reset_spend`: reset spend to 0 (or specified value) for a single key
- [X] T030 [P] [US3] Add `KeyAliases` handler in `internal/proxy/handler/key_ext.go` — `GET /key/aliases`: return distinct non-null key_alias values sorted alphabetically
- [X] T031 [P] [US3] Add `KeyInfoV2` handler in `internal/proxy/handler/key_ext.go` — `POST /v2/key/info`: accept array of key hashes, return batch info

### Team Management Extensions

- [X] T032 [US3] Add `TeamMemberUpdate` handler in `internal/proxy/handler/team_ext.go` — `POST /team/member_update`: update member role/budget within team
- [X] T033 [P] [US3] Add `TeamAvailable` handler in `internal/proxy/handler/team_ext.go` — `GET /team/available`: return teams the requesting user hasn't joined
- [X] T034 [P] [US3] Add `TeamPermissionsList` and `TeamPermissionsUpdate` handlers in `internal/proxy/handler/team_ext.go` — `GET /team/permissions_list` and `POST /team/permissions_update`

### Global Spend Endpoints

- [X] T035 [US3] Create `internal/proxy/handler/spend_global.go` — `GlobalActivity` handler for `GET /global/activity`: daily request counts + token totals aggregated from SpendLogs
- [X] T036 [P] [US3] Add `GlobalActivityByModel` handler in `spend_global.go` — `GET /global/activity/model`: activity grouped by model
- [X] T037 [P] [US3] Add `GlobalSpendReport` handler in `spend_global.go` — `GET /global/spend/report`: daily spend by team/customer/key (with group_by param)
- [X] T038 [P] [US3] Add `GlobalSpendReset` handler in `spend_global.go` — `POST /global/spend/reset`: reset all key and team spend to 0 (admin only)
- [X] T039 [P] [US3] Add `ProviderBudgets` handler in `spend_global.go` — `GET /provider/budgets`: provider budget limits + current spend
- [X] T040 [P] [US3] Add `CacheHitStats` handler in `spend_global.go` — `GET /global/activity/cache_hits`: cache hit/miss statistics

### Route Wiring + Tests

- [X] T041 [US3] Wire all new management routes in `internal/proxy/server.go` — key service-account, reset_spend, aliases, v2 info; team member_update, available, permissions; global activity, spend report, spend reset, provider budgets, cache hits
- [X] T042 [US3] Write contract test `test/contract/management_ext_test.go` — test each new endpoint with mock DB; verify response formats match Python

**Checkpoint**: Run `go test ./test/contract/... -run TestManagement -v`.

---

## Wave 2, Phase D: US4 — Enterprise Hooks + Management Events (P1)

**Goal**: Content filtering hooks and management lifecycle events

- [X] T043 [US4] Create `internal/proxy/hook/banned_keywords.go` — `BannedKeywordsHook` implementing Hook interface; `PreCall` scans message content for banned keywords (case-insensitive substring match); returns 403 error if found; configurable keyword list from YAML
- [X] T044 [P] [US4] Create `internal/proxy/hook/blocked_users.go` — `BlockedUserListHook` implementing Hook interface; `PreCall` checks if request user_id is in blocked list; returns 403 if blocked; configurable user list from YAML
- [X] T045 [P] [US4] Create `internal/proxy/hook/key_events.go` — `KeyManagementEventHook` with methods: `OnKeyGenerated(key)`, `OnKeyUpdated(key, before)`, `OnKeyDeleted(key)`, `OnKeyRotated(key)`; creates audit log entries; fires webhook events (if webhook URL configured)
- [X] T046 [P] [US4] Create `internal/proxy/hook/user_events.go` — `UserManagementEventHook` with methods: `OnUserCreated(user)`, `OnUserUpdated(user, before)`, `OnUserDeleted(user)`; creates audit log entries; fires webhook events
- [X] T047 [US4] Register hook constructors in `internal/proxy/hook/factory.go` — add `banned_keywords`, `blocked_user_list` to constructor registry; wire from config
- [X] T048 [US4] Wire hooks in key/team/user handlers — call `OnKeyGenerated` from KeyGenerate, `OnUserCreated` from UserCreate, etc.
- [X] T049 [US4] Write contract test `test/contract/hooks_test.go` — test banned keyword rejection; test blocked user rejection; test event hook webhook delivery

**Checkpoint**: Run `go test ./test/contract/... -run TestHooks -v`.

---

## Wave 2, Phase E: US5 — A2A Protocol (P1)

**Goal**: Full A2A agent lifecycle + JSON-RPC protocol

### A2A Core

- [X] T050 [US5] Create `internal/a2a/registry.go` — `AgentRegistry` struct with in-memory map + DB backend; `RegisterAgent(config)`, `DeregisterAgent(name)`, `GetAgentByID(id)`, `GetAgentByName(name)`, `ListAgents()`, `LoadFromDB(ctx, db)`, `LoadFromConfig(configs)` methods
- [X] T051 [US5] Create `internal/a2a/protocol.go` — JSON-RPC 2.0 types: `JSONRPCRequest` (jsonrpc, method, params, id), `JSONRPCResponse` (jsonrpc, result, error, id), `JSONRPCError` (code, message, data); `HandleMessage(ctx, agentID, request)` — route `message/send` to LLM completion bridge; build agent card response
- [X] T052 [US5] Create `internal/a2a/permission.go` — `AgentPermissionHandler` with `GetAllowedAgents(keyModels, teamModels, agentAccessGroups)` — intersection logic: team+key both restricted = intersect; team restricted + key unrestricted = team; team unrestricted = key; both unrestricted = all; `IsAgentAllowed(agentID, allowedAgents)` check
- [X] T053 [US5] Create `internal/a2a/bridge.go` — `CompletionBridge` struct; `SendMessage(ctx, agent, userMessage)` — build ChatCompletionRequest from agent's tianji_params + user message, route through existing handler/router, extract response, wrap as A2A SendMessageResult

### A2A Handlers

- [X] T054 [US5] Create `internal/proxy/handler/agent.go` — Agent CRUD handlers: `AgentCreate` (POST /v1/agents), `AgentList` (GET /v1/agents — permission-filtered), `AgentGet` (GET /v1/agents/{id}), `AgentUpdate` (PUT /v1/agents/{id}), `AgentPatch` (PATCH /v1/agents/{id}), `AgentDelete` (DELETE /v1/agents/{id}), `AgentMakePublic` (POST /v1/agents/{id}/make_public), `AgentDailyActivity` (GET /agent/daily/activity)
- [X] T055 [US5] Create `internal/proxy/handler/a2a.go` — A2A protocol handlers: `A2AAgentCard` (GET /a2a/{id}/.well-known/agent-card.json), `A2AMessage` (POST /a2a/{id} — JSON-RPC dispatch for message/send and message/stream)
- [X] T056 [US5] Wire A2A routes in `internal/proxy/server.go` — agent CRUD under /v1/agents, A2A protocol at /a2a/{id}/*
- [X] T057 [US5] Initialize AgentRegistry in `cmd/tianji/main.go` — load from config + DB on startup, pass to handlers

### A2A Tests

- [X] T058 [US5] Write contract test `test/contract/agent_test.go` — test agent CRUD lifecycle; test permission filtering on list; test A2A agent card response format
- [X] T059 [US5] Write contract test `test/contract/a2a_test.go` — test JSON-RPC message/send round-trip with mock LLM; test agent card discovery; test permission rejection

**Checkpoint**: Run `go test ./test/contract/... -run TestAgent -v && go test ./test/contract/... -run TestA2A -v`.

---

## Wave 2, Phase F: US6 — Anthropic Skills API (P1)

**Goal**: Skills CRUD + Anthropic API pass-through

- [X] T060 [US6] Create `internal/proxy/handler/skills.go` — `SkillCreate` (POST /v1/skills — multipart form with zip file, store in DB, forward to Anthropic), `SkillList` (GET /v1/skills — paginated from DB), `SkillGet` (GET /v1/skills/{id}), `SkillDelete` (DELETE /v1/skills/{id} — remove from DB + Anthropic); model routing via `x-tianji-model` header or `model` query param
- [X] T061 [US6] Wire skills routes in `internal/proxy/server.go` — mount under /v1/skills with auth
- [X] T062 [US6] Write contract test `test/contract/skills_test.go` — test skill CRUD with mock Anthropic upstream; test multipart upload; test model routing

**Checkpoint**: Run `go test ./test/contract/... -run TestSkills -v`.

---

## Wave 2, Phase G: US7 — Claude Code Marketplace (P1)

**Goal**: Plugin marketplace for Claude Code

- [X] T063 [US7] Create `internal/proxy/handler/marketplace.go` — `MarketplaceList` (GET /claude-code/marketplace.json — public, no auth, returns enabled plugins), `PluginCreate` (POST /claude-code/plugins), `PluginList` (GET /claude-code/plugins), `PluginGet` (GET /claude-code/plugins/{name}), `PluginEnable` (POST /claude-code/plugins/{name}/enable), `PluginDisable` (POST /claude-code/plugins/{name}/disable), `PluginDelete` (DELETE /claude-code/plugins/{name})
- [X] T064 [US7] Wire marketplace routes in `internal/proxy/server.go` — public route for marketplace.json, auth-protected for plugin CRUD
- [X] T065 [US7] Write contract test `test/contract/marketplace_test.go` — test plugin CRUD lifecycle; test public marketplace endpoint returns only enabled plugins

**Checkpoint**: Run `go test ./test/contract/... -run TestMarketplace -v`.

---

## Wave 2, Phase H: US8 — OCR / Video / Container Endpoints (P1)

**Goal**: Pass-through proxy for OCR, Video, Container APIs

- [X] T066 [US8] Create `internal/proxy/handler/ocr.go` — `OCR` handler for `POST /v1/ocr` and `POST /ocr`: pass-through to upstream provider (Mistral OCR format); resolve provider from model param, forward request, return response
- [X] T067 [P] [US8] Create `internal/proxy/handler/video.go` — `VideoCreate` (POST /v1/videos), `VideoList` (GET /v1/videos), `VideoGet` (GET /v1/videos/{id}), `VideoContent` (GET /v1/videos/{id}/content — binary response), `VideoRemix` (POST /v1/videos/{id}/remix): pass-through to upstream video provider
- [X] T068 [P] [US8] Create `internal/proxy/handler/container.go` — `ContainerCreate` (POST /v1/containers), `ContainerList` (GET /v1/containers), `ContainerGet` (GET /v1/containers/{id}), `ContainerDelete` (DELETE /v1/containers/{id}): pass-through to OpenAI Containers API
- [X] T069 [US8] Wire OCR/Video/Container routes in `internal/proxy/server.go`
- [X] T070 [US8] Write contract test `test/contract/media_test.go` — test OCR pass-through with mock Mistral; test video CRUD with mock upstream; test container CRUD with mock OpenAI

**Checkpoint**: Run `go test ./test/contract/... -run TestMedia -v`.

---

## Wave 2, Phase I: US9 — RAG Endpoints (P1)

**Goal**: RAG ingest and query pipelines

- [X] T071 [US9] Create `internal/rag/ingest.go` — `IngestPipeline` struct; `Ingest(ctx, file, config)` — parse file (text/PDF/markdown), chunk with configurable size/overlap, call embedding endpoint, write chunks to vector store; support multipart file upload and URL fetch
- [X] T072 [US9] Create `internal/rag/query.go` — `QueryPipeline` struct; `Query(ctx, question, config)` — search vector store for relevant chunks, optionally rerank results, inject retrieved context into system prompt, call chat completion endpoint
- [X] T073 [US9] Create `internal/proxy/handler/rag.go` — `RAGIngest` (POST /v1/rag/ingest — multipart form or JSON), `RAGQuery` (POST /v1/rag/query — JSON with question + optional config)
- [X] T074 [US9] Wire RAG routes in `internal/proxy/server.go`
- [X] T075 [US9] Write contract test `test/contract/rag_test.go` — test ingest with mock embedding + vector store; test query with mock search + LLM

**Checkpoint**: Run `go test ./test/contract/... -run TestRAG -v`.

---

## Wave 3, Phase J: US10 — Fallback/Health/TeamCB/Minor Endpoints (P2)

**Goal**: Complete remaining minor endpoints

- [X] T076 [US10] Create `internal/proxy/handler/fallback.go` — `FallbackCreate` (POST /fallback), `FallbackGet` (GET /fallback/{model}), `FallbackDelete` (DELETE /fallback/{model}): persist fallback config, integrate with router
- [X] T077 [P] [US10] Add `PromptTest` handler in `internal/proxy/handler/prompt_ext.go` — `POST /prompts/test`: render dotprompt template with variables, execute streamed LLM call
- [X] T078 [P] [US10] Add `RoutesList` handler in `internal/proxy/handler/misc.go` — `GET /routes`: return all registered chi routes
- [X] T079 [P] [US10] Add `TransformRequest` handler in `internal/proxy/handler/misc.go` — `POST /utils/transform_request`: resolve provider, transform request, return without executing
- [X] T080 [P] [US10] Add team callback support in `internal/proxy/handler/team_ext.go` — `POST /team/{team_id}/callback`: store callback config in team metadata; integrate with callback dispatch
- [X] T081 [P] [US10] Add Anthropic batches pass-through in `internal/proxy/handler/passthrough_ext.go` — `POST /anthropic/v1/messages/batches`, `GET /anthropic/v1/messages/batches/{id}`, `GET /anthropic/v1/messages/batches/{id}/results`
- [X] T082 [US10] Wire all minor routes in `internal/proxy/server.go`
- [X] T083 [US10] Write contract test `test/contract/minor_endpoints_test.go` — test fallback CRUD, routes list, transform request, prompt test

**Checkpoint**: Run `go test ./test/contract/... -run TestMinor -v`.

---

## Wave 3, Phase K: US11 — Provider Coverage Expansion (P2)

**Goal**: Expand provider coverage via JSON config and code

- [X] T084 [US11] Update `providers.json` with OpenAI-compatible providers — add entries for ollama, vllm, lm_studio, llamafile, xinference, triton, oobabooga, lemonade, docker_model_runner, heroku with their default base URLs and auth patterns
- [X] T085 [P] [US11] Create `internal/provider/datarobot/datarobot.go` — openaicompat subclass with DataRobot-specific auth (API key header), base URL override; register in `init()`
- [X] T086 [P] [US11] Create `internal/provider/novita/novita.go` — openaicompat subclass for Novita AI; register in `init()`
- [X] T087 [P] [US11] Create `internal/provider/hyperbolic/hyperbolic.go` — openaicompat subclass for Hyperbolic; register in `init()`
- [X] T088 [P] [US11] Create `internal/provider/featherless/featherless.go` — openaicompat subclass for Featherless AI; register in `init()`
- [X] T089 [P] [US11] Add search providers `internal/search/linkup.go` and `internal/search/firecrawl.go` — implement SearchProvider interface, register in `init()`
- [X] T090 [US11] Write unit tests for new providers — verify URL construction, auth headers, response parsing

**Checkpoint**: Run `go test ./internal/provider/datarobot/... -v && go test ./internal/provider/novita/... -v`.

---

## Wave 3, Phase L: US12 — Database Extensions (P2)

**Goal**: Additional operational tables

- [X] T091 [US12] Verify `009_extensions.sql` migration works with existing schema — run migration, verify table creation
- [X] T092 [US12] Add health check recording in health handler — after each health check, insert result into `HealthCheckTable`
- [X] T093 [US12] Add error log recording in error handling — on provider errors, insert into `ErrorLogs`
- [X] T094 [US12] Write contract test `test/contract/db_extensions_test.go` — test health check recording, error log recording

**Checkpoint**: Run `go test ./test/contract/... -run TestDBExt -v`.

---

## Wave 4: Integration Tests + Final Verification

**Goal**: End-to-end validation of all Phase 6 features

- [X] T095 Write `test/integration/phase6_middleware_test.go` — parallel request limiter + dynamic rate limiting + cache control middleware e2e
- [X] T096 [P] Write `test/integration/phase6_audit_test.go` — audit log write + query + soft-delete e2e
- [X] T097 [P] Write `test/integration/phase6_management_test.go` — new management endpoints e2e
- [X] T098 [P] Write `test/integration/phase6_a2a_test.go` — agent CRUD + A2A JSON-RPC flow e2e
- [X] T099 [P] Write `test/integration/phase6_skills_test.go` — skills CRUD e2e
- [X] T100 [P] Write `test/integration/phase6_marketplace_test.go` — Claude Code plugin CRUD e2e
- [X] T101 [P] Write `test/integration/phase6_media_test.go` — OCR/Video/Container pass-through e2e
- [X] T102 [P] Write `test/integration/phase6_rag_test.go` — RAG ingest + query pipeline e2e
- [X] T103 Run `make check` — verify lint + all tests + build pass with zero failures
- [X] T104 Update `CLAUDE.md` — add Phase 6 tech stack additions (A2A, Skills, Marketplace tables)

**Checkpoint**: `make check` green. All Phase 6 features verified.
