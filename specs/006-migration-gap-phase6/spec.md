# Feature Specification: Phase 6 Migration Gap Closure — Full Parity

**Feature Branch**: `006-migration-gap-phase6`
**Created**: 2026-02-19
**Status**: Draft
**Input**: Phase 6 migration gap closure — remaining ~5% features after Phases 1-5 (536 tasks completed). Focuses on parallel request limiting, audit logging, A2A protocol, Skills API, Claude Code Marketplace, OCR/RAG/Video/Container endpoints, missing management endpoints, enterprise hooks, and provider coverage expansion.

## Overview

Phases 1-5 brought the Go rewrite to ~95% feature parity with Python TianjiLLM. Phase 6 closes **all** remaining gaps to achieve full functional parity. This includes middleware completion (parallel request limiter, dynamic rate limiter), audit logging with soft-delete, missing management endpoints (key/team/spend), enterprise hooks, A2A protocol, Anthropic Skills API, Claude Code Marketplace, media endpoints (OCR/Video/Container), RAG pipeline, and provider coverage expansion.

**Exclusions** (not migrated):
- UI Dashboard / Admin UI (pure frontend)
- Client SDK functions (Python-specific)
- Qdrant Semantic Cache (Go uses embedded cosine similarity)
- OpenAPI Spec auto-generation (Go doesn't need it)
- DynamoDB as primary DB (Go uses PostgreSQL)
- Python asyncio debug endpoints (Go doesn't need them)

---

## User Scenarios & Testing

### User Story 1 — Parallel Request Limiting + Dynamic Rate Limiting + Middleware Completion (Priority: P0)

The Go proxy has basic RPM/TPM sliding-window rate limiting (`internal/proxy/middleware/ratelimit.go`) but lacks the critical parallel request limiter. Python TianjiLLM's `parallel_request_limiter_v3.py` (1,646 lines) enforces `max_parallel_requests` per key/team/user via Redis INCR/DECR with Lua scripts. The Go `BudgetTable` stores `max_parallel_requests` but no middleware enforces it. Additionally, the dynamic rate limiter (`dynamic_rate_limiter_v3.py`, 809 lines) provides saturation-aware priority-based throttling that is entirely missing.

**Why this priority**: Without parallel request limiting, a single key can monopolize all upstream capacity. This is the #1 enterprise complaint — it's a production safety mechanism.

**Independent Test**: Configure `max_parallel_requests=2` on a key, send 3 concurrent requests, verify the 3rd is rejected with 429.

**Acceptance Scenarios**:

1. **Given** a key with `max_parallel_requests=2`, **When** 3 concurrent requests arrive, **Then** the 3rd returns 429 with `"Rate limit exceeded: max parallel requests"` while the first 2 proceed.
2. **Given** a key with RPM=10 and a Redis-backed rate limiter, **When** the 11th request in a minute arrives, **Then** it's rejected with 429 and correct `Retry-After` header.
3. **Given** dynamic rate limiting enabled with priority weights, **When** model saturation exceeds threshold, **Then** lower-priority keys are throttled while higher-priority keys proceed.
4. **Given** `ModelBudgetLimiter` configured with per-model budgets, **When** a model's spend exceeds its budget, **Then** requests to that model return 429.
5. **Given** a key with `allowed_cache_controls=["no-cache"]`, **When** a request includes `cache: {"type": "s-maxage"}`, **Then** the request is rejected with 403.
6. **Given** Responses API with `responses_id_security` enabled, **When** user A creates a response, **Then** user B cannot access it via response ID (encrypted IDs).
7. **Given** a batch job submission, **When** the batch file contains 1000 requests, **Then** the batch rate limiter pre-reserves RPM/TPM capacity and rejects if insufficient.

| Component | Go Current State | Python Reference | Est. Lines |
|-----------|-----------------|------------------|------------|
| Parallel Request Limiter | `max_parallel_requests` field exists in BudgetTable but **no middleware** | `proxy/hooks/parallel_request_limiter_v3.py` (1,646 lines) — Redis INCR/DECR with Lua | ~250 |
| Dynamic Rate Limiter | Basic RPM/TPM sliding window, no saturation-aware throttling | `proxy/hooks/dynamic_rate_limiter_v3.py` (809 lines) | ~180 |
| Model Max Budget Middleware | `ModelBudgetLimiter` struct exists but not mounted in HTTP chain | `proxy/hooks/model_max_budget_limiter.py` (211 lines) | ~60 |
| Cache-Control Middleware | `allowed_cache_controls` field exists but no middleware | `proxy/hooks/cache_control_check.py` (58 lines) | ~60 |
| Response ID Security | None | `proxy/hooks/responses_id_security.py` (278 lines) | ~140 |
| Batch Rate Limiter | None | `proxy/hooks/batch_rate_limiter.py` (453 lines) | ~200 |

**Key Files**: `internal/proxy/middleware/parallel.go`, `internal/proxy/middleware/dynamic_ratelimit.go`, `internal/proxy/middleware/cache_control.go`, `internal/proxy/middleware/response_security.go`, `internal/proxy/middleware/batch_ratelimit.go`

---

### User Story 2 — Audit Logging + Soft-Delete (Priority: P0)

The Go proxy has zero audit trail. Python TianjiLLM records every management operation (key/team/user CRUD) in `TianjiLLM_AuditLog` table and preserves deleted entities in `TianjiLLM_DeletedVerificationToken` and `TianjiLLM_DeletedTeamTable` for compliance. Enterprise customers require this for SOC2/HIPAA.

**Why this priority**: Audit logging is a hard compliance requirement for enterprise deployment. Without it, Go proxy cannot be used in regulated environments.

**Independent Test**: Create a key, update it, delete it — verify 3 audit log entries exist with correct before/after values.

**Acceptance Scenarios**:

1. **Given** audit logging enabled (`store_audit_logs=true`), **When** a key is created via `POST /key/generate`, **Then** an audit log entry is written with `action="created"`, `table_name="verification_token"`, and `updated_values` containing the new key data.
2. **Given** a key exists, **When** it's updated via `POST /key/update`, **Then** an audit log captures `before_value` (old state) and `updated_values` (new state).
3. **Given** a key exists, **When** it's deleted via `POST /key/delete`, **Then** the key row is copied to `DeletedVerificationToken` with `deleted_at`, `deleted_by`, and an audit log with `action="deleted"`.
4. **Given** a team exists, **When** it's deleted, **Then** the team row is copied to `DeletedTeamTable` with deletion metadata.
5. **Given** audit logs exist, **When** `GET /audit` is called with filters (date range, action, table_name), **Then** paginated results are returned with correct filtering.

| Component | Description |
|-----------|-------------|
| New table `AuditLog` | Records all management CRUD operations |
| New table `DeletedVerificationToken` | Key soft-delete archive (mirrors VerificationToken + deletion metadata) |
| New table `DeletedTeamTable` | Team soft-delete archive (mirrors TeamTable + deletion metadata) |
| SQL migration | `007_audit.sql` |
| Audit helper | Insert audit records in key/team/user handlers |
| Endpoint `GET /audit` | Paginated + filtered audit log query |

**Key Files**: `internal/db/schema/007_audit.sql`, `internal/db/queries/audit.sql`, `internal/proxy/handler/audit.go`
**Est. Lines**: ~400 + SQL

---

### User Story 3 — Missing Management Endpoints (Priority: P0)

The Go proxy covers core CRUD for keys/teams/users but is missing several management endpoints that Python TianjiLLM provides. These are actively used by dashboards, automation scripts, and enterprise integrations.

**Why this priority**: Missing management endpoints break existing automation workflows and dashboard integrations.

**Independent Test**: Call each endpoint, verify correct response format matching Python behavior.

**Acceptance Scenarios**:

1. **Given** a team configured, **When** `POST /key/service-account/generate` is called with `team_id`, **Then** a service account key (no user_id) is created belonging only to the team.
2. **Given** a key with spend > 0, **When** `POST /key/{key}/reset_spend` is called, **Then** the key's spend resets to 0.
3. **Given** keys with aliases, **When** `GET /key/aliases` is called, **Then** all unique non-null aliases are returned sorted alphabetically.
4. **Given** a team member, **When** `POST /team/member_update` is called with new role/budget, **Then** the member's role and budget are updated.
5. **Given** teams exist, **When** `GET /team/available` is called by a user, **Then** teams the user hasn't joined are returned.
6. **Given** spend data exists, **When** `GET /global/activity` is called, **Then** daily request counts and token totals are returned.
7. **Given** spend data exists, **When** `GET /global/spend/report` is called with group_by=team, **Then** daily spend per team is returned.
8. **Given** admin access, **When** `POST /global/spend/reset` is called, **Then** all keys and teams have spend reset to 0.

**Key Management gaps**:
- `POST /key/service-account/generate` — service account key (no user, team-only)
- `POST /key/{key}/reset_spend` — reset key spend to 0
- `GET /key/aliases` — list all unique key aliases
- `POST /v2/key/info` — batch key info query

**Team Management gaps**:
- `POST /team/member_update` — update member role/budget
- `GET /team/available` — list teams user hasn't joined
- `GET /team/permissions_list` — get team member permissions
- `POST /team/permissions_update` — update team member permissions

**Global Spend gaps**:
- `GET /global/activity` — daily request counts + token totals
- `GET /global/activity/model` — activity grouped by model
- `GET /global/spend/report` — daily spend by team/customer/key
- `POST /global/spend/reset` — reset all spend to 0
- `GET /provider/budgets` — provider budget limits + current spend
- `GET /global/activity/cache_hits` — cache hit/miss statistics

**Key Files**: `internal/proxy/handler/key_ext.go` (extend), `internal/proxy/handler/team_ext.go` (extend), `internal/proxy/handler/spend_global.go` (new)
**Est. Lines**: ~1,100

---

### User Story 4 — Enterprise Hooks + Management Events (Priority: P1)

The Go proxy has a Hook framework (`internal/proxy/hook/hook.go` — PreCall/PostCall interface + Registry) but **zero implementations**. Python TianjiLLM has enterprise hooks for banned keywords, blocked users, and management event notifications.

**Why this priority**: Enterprise customers need content filtering (banned keywords) and lifecycle event notifications (key created -> webhook -> Slack).

**Independent Test**: Configure `banned_keywords: ["password"]`, send a message containing "password", verify 403 rejection.

**Acceptance Scenarios**:

1. **Given** `banned_keywords: ["credit card", "ssn"]` configured, **When** a chat request contains "what's your credit card", **Then** the request is rejected with 403 before reaching the provider.
2. **Given** `blocked_user_list: ["user-123"]` configured, **When** user-123 sends a request, **Then** it's rejected with 403.
3. **Given** key management event hooks enabled, **When** a key is generated, **Then** a webhook event is fired with key metadata.
4. **Given** user management event hooks enabled, **When** a user is created, **Then** a user invitation email is triggered (if email configured).

| Hook | Python Reference | Est. Lines |
|------|------------------|------------|
| Banned Keywords | `enterprise_hooks/banned_keywords.py` | ~80 |
| Blocked User List | `enterprise_hooks/blocked_user_list.py` | ~60 |
| Key Management Events | `proxy/hooks/key_management_event_hooks.py` (609 lines) | ~200 |
| User Management Events | `proxy/hooks/user_management_event_hooks.py` (209 lines) | ~100 |

**Key Files**: `internal/proxy/hook/banned_keywords.go`, `internal/proxy/hook/blocked_users.go`, `internal/proxy/hook/key_events.go`, `internal/proxy/hook/user_events.go`
**Est. Lines**: ~440

---

### User Story 5 — A2A Protocol (Agent-to-Agent) (Priority: P1)

Python TianjiLLM implements the full A2A (Agent-to-Agent) protocol: agent registration, discovery via `.well-known/agent-card.json`, JSON-RPC 2.0 message handling, permission-based access control, and daily agent spend tracking. Total Python implementation: ~2,256 lines across 6 files.

**Why this priority**: A2A is the emerging standard for multi-agent orchestration. Companies deploying agent fleets need centralized routing and cost tracking through their proxy.

**Independent Test**: Register an agent, call `GET /a2a/{id}/.well-known/agent-card.json`, send a JSON-RPC `message/send` — verify the agent card is returned and the message is routed to the correct LLM.

**Acceptance Scenarios**:

1. **Given** an agent registered via `POST /v1/agents`, **When** `GET /a2a/{id}/.well-known/agent-card.json` is called, **Then** the A2A agent card is returned with capabilities and endpoint URL.
2. **Given** a registered agent, **When** `POST /a2a/{id}` is called with JSON-RPC `message/send`, **Then** the message is routed to the agent's configured LLM and the response is returned as JSON-RPC result.
3. **Given** agent access groups configured, **When** a key without permission tries to call an agent, **Then** 403 is returned.
4. **Given** an agent processes messages, **When** spend tracking is enabled, **Then** daily agent spend is recorded in `DailyAgentSpend` table.
5. **Given** multiple agents, **When** `GET /v1/agents` is called, **Then** only agents the user has permission to access are returned.

| Component | Description | Key Implementation |
|-----------|-------------|-------------------|
| New table `AgentsTable` | agent_id, agent_name, tianji_params(JSON), agent_card_params(JSON), agent_access_groups[], created_by | SQL migration |
| New table `DailyAgentSpend` | agent_id, date, model, api_key, tokens, spend aggregation | SQL migration |
| Agent CRUD endpoints | GET/POST/PUT/PATCH/DELETE `/v1/agents/*` | ~400 lines handler |
| A2A protocol endpoints | `GET /a2a/{id}/.well-known/agent-card.json`, `POST /a2a/{id}` (JSON-RPC 2.0) | ~300 lines |
| Agent Registry | In-memory + DB dual registry, DB-to-memory sync on startup | ~200 lines |
| Permission Handler | Key/team level permission intersection logic | ~250 lines |
| TianjiLLM Completion Bridge | A2A call to chat completion routing | ~200 lines |

**Key Files**: `internal/a2a/registry.go`, `internal/a2a/protocol.go`, `internal/a2a/permission.go`, `internal/proxy/handler/agent.go`, `internal/proxy/handler/a2a.go`
**Est. Lines**: ~2,000

---

### User Story 6 — Anthropic Skills API (Priority: P1)

Python TianjiLLM proxies Anthropic's Skills Beta API: skill CRUD with multipart zip upload, multi-account routing, and DB persistence. Total: ~597 lines across 2 files. Phase 6 implements API pass-through + CRUD only (no local skill execution sandbox).

**Why this priority**: Teams using Claude with Skills need centralized skill management and multi-account routing.

**Independent Test**: Create a skill via `POST /v1/skills?beta=true` with a zip file, verify it's stored in DB and can be listed/retrieved.

**Acceptance Scenarios**:

1. **Given** Anthropic provider configured, **When** `POST /v1/skills?beta=true` is called with multipart form (zip file), **Then** the skill is stored in DB and forwarded to Anthropic.
2. **Given** skills exist in DB, **When** `GET /v1/skills?beta=true` is called, **Then** paginated skill list is returned.
3. **Given** a skill exists, **When** `GET /v1/skills/{id}?beta=true` is called, **Then** skill details are returned.
4. **Given** a skill exists, **When** `DELETE /v1/skills/{id}?beta=true` is called, **Then** the skill is removed from DB.
5. **Given** multiple Anthropic accounts, **When** `x-tianji-model: claude-account-1` header is set, **Then** the skill request is routed to the correct account.

| Component | Description |
|-----------|-------------|
| New table `SkillsTable` | skill_id, display_title, description, instructions, source, latest_version, file_content(bytes), file_name, file_type, metadata |
| Skills CRUD | POST/GET/DELETE `/v1/skills/*` (multipart zip upload support) |
| Anthropic API pass-through | Forward skill operations to Anthropic Skills Beta API |
| Model routing | Route to specific Anthropic account via header/query/body |

**Key Files**: `internal/db/schema/008_skills_agents.sql` (shared with US5), `internal/proxy/handler/skills.go`
**Est. Lines**: ~400

---

### User Story 7 — Claude Code Marketplace (Priority: P1)

Python TianjiLLM provides a Claude Code plugin marketplace: plugin registration, discovery endpoint for Claude Code clients, enable/disable management. Total: 546 lines. The `TianjiLLM_ClaudeCodePluginTable` is referenced in code but not defined in Prisma schema (schema inferred from code).

**Why this priority**: Teams using Claude Code need centralized plugin management across their organization.

**Independent Test**: Register a plugin, call `GET /claude-code/marketplace.json` (public endpoint), verify the plugin appears in the catalog.

**Acceptance Scenarios**:

1. **Given** a plugin registered via `POST /claude-code/plugins`, **When** `GET /claude-code/marketplace.json` is called (no auth required), **Then** the plugin appears in the marketplace catalog.
2. **Given** a registered plugin, **When** `POST /claude-code/plugins/{name}/enable` is called, **Then** the plugin is enabled.
3. **Given** an enabled plugin, **When** `POST /claude-code/plugins/{name}/disable` is called, **Then** the plugin is disabled and no longer appears in marketplace.
4. **Given** a plugin exists, **When** `DELETE /claude-code/plugins/{name}` is called, **Then** the plugin is removed.

| Component | Description |
|-----------|-------------|
| New table `ClaudeCodePluginTable` | name(unique), version, description, manifest_json, files_json, enabled, source, source_url, created_by |
| Discovery endpoint | `GET /claude-code/marketplace.json` (public, no auth) |
| Plugin CRUD | POST/GET/DELETE `/claude-code/plugins/*` |
| Enable/Disable | `POST /claude-code/plugins/{name}/enable` and `disable` |

**Key Files**: `internal/proxy/handler/marketplace.go`
**Est. Lines**: ~300

---

### User Story 8 — OCR / Video / Container Endpoints (Priority: P1)

Python TianjiLLM has endpoints for OCR (Mistral), Video generation (RunwayML/OpenAI), and Containers (OpenAI). These are pass-through proxy endpoints following the standard request processing pipeline.

**Why this priority**: Completeness — clients using these APIs expect them to work through the proxy.

**Independent Test**: For each endpoint type, send a request through the proxy, verify it's forwarded to the upstream provider and the response is returned.

**Acceptance Scenarios**:

1. **Given** Mistral OCR configured, **When** `POST /v1/ocr` is called with a document, **Then** the request is proxied to Mistral's OCR API.
2. **Given** a video provider configured, **When** `POST /v1/videos` is called, **Then** a video generation job is created.
3. **Given** a video job completed, **When** `GET /v1/videos/{id}/content` is called, **Then** the binary video content is returned.
4. **Given** OpenAI Containers configured, **When** `POST /v1/containers` is called, **Then** a container is created.
5. **Given** a container exists, **When** `GET /v1/containers/{id}` is called, **Then** container details are returned.

| Endpoint | Python Lines | Complexity | Description |
|----------|-------------|-----------|-------------|
| `POST /v1/ocr` | 97 | Very Low | Pure pass-through to Mistral OCR API |
| `POST /v1/videos`, `GET /v1/videos/{id}`, `GET /v1/videos/{id}/content`, `POST /v1/videos/{id}/remix` | 500 | Medium | Video generation + status + content download |
| `POST/GET/DELETE /v1/containers/*` | 414 | Medium | OpenAI Containers API |

**Key Files**: `internal/proxy/handler/ocr.go`, `internal/proxy/handler/video.go`, `internal/proxy/handler/container.go`
**Est. Lines**: ~700

---

### User Story 9 — RAG Endpoints (Priority: P1)

Python TianjiLLM provides RAG (Retrieval-Augmented Generation) endpoints: document ingestion (upload, chunk, embed, vector store) and query (search, optional rerank, LLM completion). Total: 553 lines.

**Why this priority**: RAG is the most common enterprise AI pattern. Teams need centralized RAG pipeline management.

**Independent Test**: Ingest a document via `POST /v1/rag/ingest`, then query via `POST /v1/rag/query`, verify the document content is retrieved and used in the LLM response.

**Acceptance Scenarios**:

1. **Given** a vector store configured, **When** `POST /v1/rag/ingest` is called with a document file (multipart), **Then** the document is chunked, embedded, and stored in the vector store.
2. **Given** documents ingested, **When** `POST /v1/rag/query` is called with a question, **Then** relevant chunks are retrieved, optionally reranked, and injected as context into an LLM completion.
3. **Given** RAG ingest with URL source, **When** a URL is provided instead of a file, **Then** the content is fetched, chunked, and stored.
4. **Given** a vector store, **When** the ingest creates a new store, **Then** the store is saved to `ManagedVectorStoreTable` for persistence.

| Component | Description |
|-----------|-------------|
| Ingest pipeline | multipart file upload, chunking, embedding call, vector store write |
| Query pipeline | vector search, rerank (optional), context injection, LLM completion |
| Table dependency | Reuses `ManagedVectorStoreTable` (Phase 3) |

**Key Files**: `internal/rag/ingest.go`, `internal/rag/query.go`, `internal/proxy/handler/rag.go`
**Est. Lines**: ~600

---

### User Story 10 — Fallback Management + Health Extensions + Team Callbacks + Minor Endpoints (Priority: P2)

Various smaller endpoints needed for full parity: fallback CRUD, extended health checks, team-specific callback configuration, prompt testing, route listing, request preview, and Anthropic batches pass-through.

**Why this priority**: These are individually small but collectively important for operational tooling.

**Independent Test**: For each endpoint, verify correct response format.

**Acceptance Scenarios**:

1. **Given** admin access, **When** `POST /fallback` is called with model fallback config, **Then** the fallback is persisted and applied.
2. **Given** team callbacks configured, **When** a team's request succeeds, **Then** the team-specific callback (e.g., Langfuse) is invoked.
3. **Given** a dotprompt template, **When** `POST /prompts/test` is called with variables, **Then** the template is rendered and an LLM call is made with the result.
4. **Given** a running proxy, **When** `GET /routes` is called, **Then** all registered routes are returned.
5. **Given** a request body, **When** `POST /utils/transform_request` is called, **Then** the transformed provider request is returned (without executing).

| Feature | Endpoints | Est. Lines |
|---------|----------|------------|
| Fallback CRUD | `POST/GET/DELETE /fallback/*` | ~150 |
| Health extensions | `/model/metrics`, deployment health | ~200 |
| Team Callbacks | Per-team callback config in team metadata | ~200 |
| Prompt test | `POST /prompts/test` | ~100 |
| Route listing | `GET /routes` | ~30 |
| Request preview | `POST /utils/transform_request` | ~60 |
| Anthropic batches | `/anthropic/v1/messages/batches` pass-through | ~50 |

**Key Files**: `internal/proxy/handler/fallback.go`, `internal/proxy/handler/prompt_ext.go`, various extensions
**Est. Lines**: ~790

---

### User Story 11 — Provider Coverage Expansion (Priority: P2)

Expand `providers.json` and add code-based providers for full coverage.

**Why this priority**: Each missing provider is a migration blocker for teams using that specific provider.

**Acceptance Scenarios**:

1. **Given** `providers.json` updated with additional OpenAI-compatible providers, **When** a request targets `provider/model`, **Then** it routes correctly.
2. **Given** a new code-based provider (e.g., datarobot), **When** registered via `init()`, **Then** requests route with correct auth and base URL.

| Type | Providers | Approach |
|------|----------|----------|
| JSON config only (~10) | ollama, vllm, lm_studio, llamafile, xinference, triton, oobabooga, lemonade, docker_model_runner, heroku | Add to `providers.json` |
| New code (~4) | datarobot, novita, hyperbolic, featherless_ai | openaicompat subclass, override auth/baseURL |
| Niche (~16) | aiml, empower, gradient_ai, petals, bytez, zai, parallel_ai, galadriel, morph, cometapi, compactifai, maritalk, nlp_cloud, predibase, clarifai | openaicompat JSON or on-demand |
| Multimedia | runwayml (video), topaz (image) | Simple HTTP forwarding |
| Search | linkup, firecrawl | Add to `internal/search/` registry |

**Key Files**: `providers.json`, `internal/provider/` (new subdirs), `internal/search/` (new providers)
**Est. Lines**: ~400 code + JSON config

---

### User Story 12 — Database Extensions (Priority: P2)

Additional tables needed for operational features.

**Why this priority**: Supporting tables for health history, error logging, and granular spend tracking.

**Acceptance Scenarios**:

1. **Given** migration applied, **When** health checks run, **Then** results are persisted to `HealthCheckTable`.
2. **Given** errors occur, **When** logged, **Then** error details are stored in `ErrorLogs`.

| New Table | Purpose | Priority |
|-----------|---------|----------|
| `HealthCheckTable` | Health check history | P1 |
| `ErrorLogs` | Error log persistence | P2 |
| `DailyOrganizationSpend` | Org-level daily spend aggregation | P2 |
| `DailyEndUserSpend` | End-user daily spend aggregation | P2 |
| `ManagedFileTable` | Cross-provider file mapping | P2 |
| `ManagedObjectTable` | Managed batch/fine-tuning jobs | P2 |
| `CronJob` | Scheduled task tracking | P2 |

**Key Files**: `internal/db/schema/009_extensions.sql`, `internal/db/queries/` (new query files)
**Est. Lines**: SQL migration + ~300 Go code

---

### Edge Cases

- **Redis unavailable during parallel request limiting**: Fall back to in-memory counting with degraded accuracy (fail-open behavior)
- **Audit log table full**: Implement TTL-based cleanup or archive to cold storage
- **A2A agent references deleted LLM model**: Return 404 with clear error indicating the backing model is unavailable
- **Skill zip upload exceeds size limit**: Return 413 with configurable max size
- **RAG ingest with unsupported file format**: Return 400 with list of supported formats
- **Concurrent key deletion + audit log write**: Use database transaction to ensure atomicity
- **Batch rate limiter with file not found**: Return 400 with clear error, don't reserve capacity

---

## Requirements

### Functional Requirements

- **FR-001**: System MUST enforce `max_parallel_requests` limit per key/team/user via Redis atomic operations
- **FR-002**: System MUST record audit logs for all key/team/user CRUD operations when `store_audit_logs=true`
- **FR-003**: System MUST preserve deleted keys in `DeletedVerificationToken` and deleted teams in `DeletedTeamTable`
- **FR-004**: System MUST implement all missing management endpoints (key service-account, reset spend, team member update, global activity/spend)
- **FR-005**: System MUST implement A2A protocol with JSON-RPC 2.0 message handling and agent card discovery
- **FR-006**: System MUST proxy Anthropic Skills API with DB persistence and multipart zip upload
- **FR-007**: System MUST provide Claude Code plugin marketplace with public discovery endpoint
- **FR-008**: System MUST proxy OCR, Video, and Container endpoints to their respective upstream providers
- **FR-009**: System MUST implement RAG ingest (upload, chunk, embed, store) and query (search, rerank, complete) pipelines
- **FR-010**: Enterprise hooks MUST support banned keywords and blocked user list filtering
- **FR-011**: All new endpoints MUST follow existing patterns (auth middleware, error model, JSON response format)
- **FR-012**: All new database tables MUST use sqlc-generated queries (no hand-written SQL in Go code)

### Key Entities

- **AuditLog**: Immutable record of management operations (who changed what, when, before/after values)
- **DeletedVerificationToken / DeletedTeamTable**: Soft-delete archives preserving full entity state at deletion time
- **AgentsTable**: A2A agent registration with LLM params, agent card metadata, and access groups
- **DailyAgentSpend**: Per-agent daily spend aggregation by model and key
- **SkillsTable**: Anthropic Skills with binary file content (zip), metadata, and version tracking
- **ClaudeCodePluginTable**: Marketplace plugins with manifest, source info, and enable/disable state

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: All 12 User Stories have passing contract tests in `test/contract/`
- **SC-002**: All 12 User Stories have passing integration tests in `test/integration/phase6_*.go`
- **SC-003**: `make check` (lint + test + build) passes with zero failures
- **SC-004**: All new endpoints return responses matching Python TianjiLLM's format (validated by contract tests)
- **SC-005**: Parallel request limiter correctly enforces limits under concurrent load (verified by integration test with goroutines)
- **SC-006**: Audit log entries are created for every management operation (verified by counting entries after CRUD sequence)
- **SC-007**: A2A agent card is discoverable and JSON-RPC message routing works end-to-end
- **SC-008**: Zero regressions in existing Phase 1-5 tests

---

## Implementation Summary

| US | Priority | Description | Est. Lines |
|----|----------|-------------|------------|
| US1 | P0 | Parallel Request Limiting + Dynamic Rate Limiting + Middleware | ~890 |
| US2 | P0 | Audit Logging + Soft-Delete | ~400 |
| US3 | P0 | Missing Management Endpoints (key/team/spend) | ~1,100 |
| US4 | P1 | Enterprise Hooks + Management Events | ~440 |
| US5 | P1 | A2A Protocol | ~2,000 |
| US6 | P1 | Anthropic Skills API | ~400 |
| US7 | P1 | Claude Code Marketplace | ~300 |
| US8 | P1 | OCR / Video / Container | ~700 |
| US9 | P1 | RAG Endpoints | ~600 |
| US10 | P2 | Fallback/Health/TeamCB/Minor Endpoints | ~790 |
| US11 | P2 | Provider Coverage Expansion | ~400 |
| US12 | P2 | Database Extensions | ~300 |
| **Total** | | | **~8,320** |

### Implementation Waves

- **Wave 1 (P0)**: US1, US2, US3 (rate limiting, audit, management endpoints)
- **Wave 2 (P1)**: US4, US5, US6, US7, US8, US9 (hooks, A2A, Skills, Marketplace, media, RAG)
- **Wave 3 (P2)**: US10, US11, US12 (minor endpoints, providers, DB)
