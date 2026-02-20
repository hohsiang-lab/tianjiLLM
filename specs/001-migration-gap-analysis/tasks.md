# Tasks: TianjiLLM Python-to-Go Migration

**Input**: Design documents from `/specs/001-migration-gap-analysis/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Constitution principle IV (Test-Driven Migration) requires tests for each migrated feature. Test tasks included per user story. Each provider MUST have fixture-based tests with >=90% translation layer coverage.

**Organization**: Tasks grouped by user story (US1–US5) mapping to spec.md priorities (P1–P3).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Entry point: `cmd/tianji/`
- Internal packages: `internal/`
- Tests: `test/contract/`, `test/integration/`, `test/fixtures/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Config compatibility foundation and shared types needed by all user stories

- [X] T001 Extend config struct to parse all Python TianjiLLM proxy_config.yaml fields with overflow map for unknown fields in `internal/config/config.go` (FR-029)
- [X] T002 Add `failure_callback` field (YAML tag: `failure_callback`) to `TianjiLLMSettings` alongside existing `Callbacks` (YAML tag: `success_callback`), and parse `guardrails` + `alerting` + `alerting_threshold` YAML sections in `internal/config/config.go`
- [X] T003 [P] Add startup warning system that logs all unrecognized/unsupported config fields in `internal/config/validate.go` (FR-027)
- [X] T004 [P] Add 501 Not Implemented handler for all Python TianjiLLM endpoints not yet implemented in Go in `internal/proxy/handler/notimplemented.go` (FR-028)
- [X] T005 Register 501 stub routes in `internal/proxy/server.go` for all endpoints from api-endpoints.md not yet implemented

**Checkpoint**: Config loads any Python proxy_config.yaml without errors; unimplemented endpoints return 501 with descriptive messages

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared model types and DB infrastructure that multiple user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Add request model types for Files, Batches, Fine-tuning, Rerank APIs in `internal/model/files.go`, `internal/model/batches.go`, `internal/model/finetuning.go`, `internal/model/rerank.go`
- [X] T007 [P] Add Organization, Credential, AccessGroup entity types to `internal/model/management.go`
- [X] T008 [P] Add SpendLog new fields (organization_id, provider, request_tags, cache_hit) to existing model in `internal/db/models.go`
- [X] T009 [P] Add VirtualKey new fields (max_parallel_requests, guardrails) to existing model in `internal/db/models.go`

**Checkpoint**: Foundation ready — all shared types exist, user story implementation can begin

---

## Phase 3: User Story 1 — Core API Parity for Drop-in Replacement (Priority: P1) 🎯 MVP

**Goal**: Support 20+ providers + Files/Batches/Fine-tuning/Rerank/Pass-through APIs so existing Python TianjiLLM users can switch to Go

**Independent Test**: Send identical requests to Python proxy (port 4000) and Go proxy (port 8000), compare response format per quickstart.md

### New Providers (14 providers, all parallelizable — each is a separate directory)

- [X] T010 [P] [US1] Implement Cohere provider with supported params and param mapping in `internal/provider/cohere/cohere.go`
- [X] T011 [P] [US1] Implement Mistral provider in `internal/provider/mistral/mistral.go`
- [X] T012 [P] [US1] Implement Together AI provider in `internal/provider/together/together.go`
- [X] T013 [P] [US1] Implement Fireworks AI provider in `internal/provider/fireworks/fireworks.go`
- [X] T014 [P] [US1] Implement Groq provider in `internal/provider/groq/groq.go`
- [X] T015 [P] [US1] Implement DeepSeek provider in `internal/provider/deepseek/deepseek.go`
- [X] T016 [P] [US1] Implement Replicate provider in `internal/provider/replicate/replicate.go`
- [X] T017 [P] [US1] Implement Hugging Face provider in `internal/provider/huggingface/huggingface.go`
- [X] T018 [P] [US1] Implement Databricks provider in `internal/provider/databricks/databricks.go`
- [X] T019 [P] [US1] Implement Cloudflare Workers AI provider in `internal/provider/cloudflare/cloudflare.go`
- [X] T020 [P] [US1] Implement Cerebras provider in `internal/provider/cerebras/cerebras.go`
- [X] T021 [P] [US1] Implement Perplexity provider in `internal/provider/perplexity/perplexity.go`
- [X] T022 [P] [US1] Implement xAI (Grok) provider in `internal/provider/xai/xai.go`
- [X] T023 [P] [US1] Implement SambaNova provider in `internal/provider/sambanova/sambanova.go`

### Provider Tests (Constitution IV — fixture-based, parallelizable)

- [X] T024 [P] [US1] Add request/response fixtures from Python test data and write TransformRequest/TransformResponse tests for all 14 new providers in `test/fixtures/{provider}/` and `internal/provider/{provider}/*_test.go`

### New API Endpoints

- [X] T025 [P] [US1] Implement Files API handlers (upload, list, get, download, delete) in `internal/proxy/handler/files.go`
- [X] T026 [P] [US1] Implement Batches API handlers (create, get, cancel, list) in `internal/proxy/handler/batches.go`
- [X] T027 [P] [US1] Implement Fine-tuning API handlers (create, get, cancel, events, checkpoints) in `internal/proxy/handler/finetuning.go`
- [X] T028 [P] [US1] Implement Rerank API handler in `internal/proxy/handler/rerank.go`

### API Endpoint Contract Tests (Constitution IV)

- [X] T029 [P] [US1] Write contract tests for Files, Batches, Fine-tuning, Rerank handlers using httptest mock upstream in `test/contract/forward_api_test.go`

### Pass-through System

- [X] T030 [US1] Implement generic pass-through reverse proxy router with SSE streaming support in `internal/proxy/passthrough/router.go`
- [X] T031 [P] [US1] Implement base pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T032 [P] [US1] Implement OpenAI pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T033 [P] [US1] Implement Anthropic pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T034 [P] [US1] Implement Vertex AI pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T035 [P] [US1] Implement Cohere pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T036 [P] [US1] Implement Gemini pass-through logging handler in `internal/proxy/passthrough/handlers.go`
- [X] T037 [US1] Add pass-through guardrail hook points (pre_call on request body, post_call on response) following Python's PassthroughGuardrailHandler pattern in `internal/proxy/passthrough/router.go`
- [X] T038 [US1] Add unknown parameter pass-through with warning logging — forward unrecognized params to upstream provider and log warning in `internal/proxy/handler/chat.go` and `internal/model/request.go`

### Route Registration

- [X] T039 [US1] Register all new US1 routes (Files, Batches, Fine-tuning, Rerank, Pass-through) in `internal/proxy/server.go`

**Checkpoint**: 20+ providers operational. Files/Batches/Fine-tuning/Rerank/Pass-through all functional. Existing Python TianjiLLM clients can hit Go proxy with no changes.

---

## Phase 4: User Story 2 — Enterprise Management & Access Control (Priority: P2)

**Goal**: Full org/team/user/key CRUD + RBAC + SSO so enterprise customers can manage access and budgets

**Independent Test**: Create org → team → user → key hierarchy, verify RBAC enforcement on model access

### Database Schema

- [X] T040 [US2] Create organization migration adding organizations table, org FK to teams/users/keys/spend_logs in `internal/db/schema/003_organization.sql`
- [X] T041 [US2] Create credential migration adding credentials table with NaCl SecretBox encrypted storage in `internal/db/schema/004_credentials.sql`
- [X] T042 [US2] Add queries for Organization CRUD, Credential CRUD, AccessGroup CRUD, key update, team update/members in `internal/db/management.go`

### Encryption Infrastructure

- [X] T043 [US2] Implement NaCl SecretBox encrypt/decrypt helpers (SHA256(master_key) → 32-byte key, base64url output) matching Python's `encrypt_value`/`decrypt_value` in `internal/auth/encrypt.go`

### Auth Infrastructure

- [X] T044 [US2] Implement JWT validation with JWKS endpoint caching using golang-jwt/jwt/v5 + keyfunc/v3 in `internal/auth/jwt.go`
- [X] T045 [US2] Implement RBAC engine with 4 roles (proxy_admin, team, internal_user, end_user) and 3-layer checks (Role → Routes → Models) in `internal/auth/rbac.go`
- [X] T046 [US2] Implement SSO/OIDC login flow with IDP role mapping in `internal/auth/sso.go`
- [X] T047 [US2] Extend auth middleware to support JWT + RBAC alongside existing Bearer token auth in `internal/proxy/middleware/auth.go`

### Management Handlers

- [X] T048 [P] [US2] Implement Organization CRUD handlers (new, info, update, delete, member/add, member/delete) in `internal/proxy/handler/organization.go`
- [X] T049 [P] [US2] Implement Credential management handlers (new, list, info, update, delete) using NaCl encryption from T043 in `internal/proxy/handler/credentials.go`
- [X] T050 [P] [US2] Implement Access Group handlers (new, info, update, delete) in `internal/proxy/handler/accessgroup.go`
- [X] T051 [P] [US2] Implement key update handler (POST /key/update) in `internal/proxy/handler/key.go`
- [X] T052 [P] [US2] Implement team update and member management handlers (POST /team/update, /team/member/add, /team/member/delete) in `internal/proxy/handler/team.go`
- [X] T053 [P] [US2] Implement real budget handlers replacing 501 stubs in `internal/proxy/handler/budget.go`
- [X] T054 [P] [US2] Implement SSO endpoints (GET /sso/login, GET /sso/callback) in `internal/proxy/handler/sso.go`

### US2 Tests (Constitution IV)

- [X] T055 [P] [US2] Write contract tests for Organization, Credential, AccessGroup, key update, team update handlers in `test/contract/organization_test.go`, `test/contract/credentials_test.go`, `test/contract/accessgroup_test.go`
- [X] T056 [P] [US2] Write RBAC integration tests verifying 4-role access control enforcement (admin can do everything, end_user restricted) in `test/integration/rbac_test.go`

### Route Registration

- [X] T057 [US2] Register all US2 routes (organization/*, credentials/*, model_access_group/*, key/update, team/update, team/member/*, sso/*) in `internal/proxy/server.go`

**Checkpoint**: Full enterprise management operational. Org hierarchy enforced. RBAC blocks unauthorized access. SSO login works.

---

## Phase 5: User Story 3 — Observability & Cost Tracking (Priority: P2)

**Goal**: Callback/hook system with 5 integrations + spend analytics + embedded model pricing

**Independent Test**: Send requests through proxy, verify webhook receives structured log payload with correct token count and cost

### Pricing Infrastructure

- [X] T058 [US3] Copy Python's model_prices_and_context_window.json into `internal/pricing/model_prices.json` and implement loader with Go embed + lookup by model name in `internal/pricing/pricing.go`
- [X] T059 [US3] Add `custom_pricing` config support to override embedded prices in `internal/config/config.go`

### Callback Framework

- [X] T060 [US3] Implement Callback interface, CallbackData struct, TokenUsage struct, and CallbackRegistry with Fire methods in `internal/callback/callback.go`
- [X] T061 [US3] Wire CallbackRegistry into request handler lifecycle (pre-call, post-call-success, post-call-failure, stream-event) in `internal/proxy/handler/chat.go` and other handler files

### Callback Integrations (all parallelizable — separate directories)

- [X] T062 [P] [US3] Implement generic HTTP webhook callback in `internal/callback/webhook/webhook.go`
- [X] T063 [P] [US3] Implement Prometheus metrics callback (requests_total, duration_seconds, tokens_total, spend_total, deployment_health) in `internal/callback/prometheus/prometheus.go`
- [X] T064 [P] [US3] Implement OpenTelemetry trace callback with W3C propagation in `internal/callback/otel/otel.go`
- [X] T065 [P] [US3] Implement Langfuse logging callback in `internal/callback/langfuse/langfuse.go`
- [X] T066 [P] [US3] Implement Datadog logging callback in `internal/callback/datadog/datadog.go`

### Budget Alerting (FR-019)

- [X] T067 [US3] Implement budget alerting callback following Python's SlackAlerting pattern — alert on budget threshold crossing (>80% spend), slow/hanging requests, configurable webhook URL + threshold in `internal/callback/alerting/alerting.go`

### Spend Analytics Endpoints

- [X] T068 [P] [US3] Implement spend by team endpoint (GET /spend/teams) in `internal/proxy/handler/spend.go`
- [X] T069 [P] [US3] Implement spend by tag endpoint (GET /spend/tags) in `internal/proxy/handler/spend.go`
- [X] T070 [P] [US3] Implement spend by model endpoint (GET /spend/models) in `internal/proxy/handler/spend.go`
- [X] T071 [P] [US3] Implement spend by end_user/customer endpoint (GET /spend/end_users) — aggregate by `end_user` field following Python's `group_by == "customer"` pattern in `internal/proxy/handler/spend.go`

### Management & Health Endpoints

- [X] T072 [P] [US3] Implement callback list endpoint (GET /callback/list) in `internal/proxy/handler/callback.go`
- [X] T073 [P] [US3] Implement cache management endpoints (GET /cache/ping, POST /cache/delete, POST /cache/flushall) in `internal/proxy/handler/cache.go`
- [X] T074 [P] [US3] Implement service health endpoint (GET /health/services) checking DB, Redis, provider statuses in `internal/proxy/handler/health.go`

### US3 Tests (Constitution IV)

- [X] T075 [P] [US3] Write callback integration test verifying webhook fires with correct CallbackData (model, tokens, cost, metadata) in `test/integration/callback_test.go`
- [X] T076 [P] [US3] Write spend analytics contract tests for /spend/teams, /spend/tags, /spend/models, /spend/end_users in `test/contract/spend_test.go`

### Route Registration

- [X] T077 [US3] Register all US3 routes (spend/teams, spend/tags, spend/models, spend/end_users, callback/list, cache/*, health/services) in `internal/proxy/server.go`

**Checkpoint**: Callbacks fire on every request. Prometheus metrics scrapeable. Spend queries work by team/tag/model/customer. Budget alerting fires on threshold. Cost calculated from embedded pricing.

---

## Phase 6: User Story 4 — Guardrails & Content Safety (Priority: P3)

**Goal**: Guardrail framework with PII detection, content moderation, prompt injection detection — assignable per-key/per-team

**Independent Test**: Configure PII guardrail, send request with SSN/email, verify it's redacted before reaching provider

**Depends on**: US3 (callback framework — guardrails embed the Callback interface)

### Guardrail Framework

- [X] T078 [US4] Implement Guardrail interface (embeds Callback), GuardrailHook/InputType types, and guardrail runner in `internal/guardrail/guardrail.go`
- [X] T079 [US4] Wire guardrail runner into CallbackRegistry.RunGuardrails() and add pre-call/post-call hooks in request handler in `internal/callback/callback.go` and `internal/proxy/handler/chat.go`

### Guardrail Integrations (all parallelizable — separate directories)

- [X] T080 [P] [US4] Implement Presidio PII detection guardrail (detect and redact PII entities) in `internal/guardrail/presidio/presidio.go`
- [X] T081 [P] [US4] Implement OpenAI content moderation guardrail (block harmful content) in `internal/guardrail/moderation/moderation.go`
- [X] T082 [P] [US4] Implement prompt injection detection guardrail in `internal/guardrail/promptinjection/promptinjection.go`

### Per-Key/Per-Team Assignment

- [X] T083 [US4] Wire guardrail assignment from VirtualKey.guardrails and Team.guardrails fields into guardrail runner selection in `internal/proxy/middleware/auth.go` and `internal/guardrail/guardrail.go`

### US4 Tests (Constitution IV)

- [X] T084 [P] [US4] Write guardrail integration tests: PII redaction (SSN/email), moderation blocking, prompt injection detection, per-key assignment in `test/integration/guardrail_test.go`

**Checkpoint**: 3 guardrail types functional. Per-key/per-team assignment works. PII redacted, harmful content blocked, prompt injection detected.

---

## Phase 7: User Story 5 — Advanced Routing & Reliability (Priority: P3)

**Goal**: Cost-based, usage-based, tag-based routing + context window fallback + policy engine

**Independent Test**: Configure cost-based routing with 2 deployments at different prices, verify cheaper one is always selected

### Routing Strategies

- [X] T085 [US5] Extend cost strategy to query embedded pricing table when ModelInfo.InputCost is absent in `internal/router/strategy/cost.go`
- [X] T086 [P] [US5] Implement usage-based strategy with per-deployment TPM/RPM sliding window counters in `internal/router/strategy/usage.go`
- [X] T087 [P] [US5] Implement tag-based strategy that filters deployments by tag before delegating to inner strategy in `internal/router/strategy/tag.go`
- [X] T088 [US5] Implement context window fallback — auto-retry with larger context model when request exceeds limit in `internal/router/router.go`

### Policy Engine (F8 split — 3 tasks for clarity)

- [X] T089 [US5] Define Policy config schema (conditions, guardrail bindings, routing_strategy override) and parse from YAML in `internal/router/policy.go`
- [X] T090 [US5] Implement policy condition matching (wildcard patterns on team_alias, key_alias, model, tags) following Python's PolicyMatcher pattern in `internal/router/policy.go`
- [X] T091 [US5] Wire policy engine into request flow — resolve matching policies, apply guardrail bindings and strategy overrides in `internal/proxy/handler/chat.go`

### Router Management Endpoints

- [X] T092 [US5] Implement router settings endpoints (GET/PATCH /router/settings) in `internal/proxy/handler/router_settings.go`
- [X] T093 [US5] Register router settings routes in `internal/proxy/server.go`

### US5 Tests (Constitution IV)

- [X] T094 [P] [US5] Write routing strategy tests: cost picks cheapest, usage picks least utilized, tag filters correctly, context fallback retries in `test/integration/router_test.go`

**Checkpoint**: All 5 routing strategies operational (shuffle, latency, cost, usage, tag). Context window fallback works. Policy engine routes conditionally.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Clean up 501 stubs, validate end-to-end, performance

- [X] T095 Remove all 501 stub routes that have been replaced by real implementations in `internal/proxy/server.go` and `internal/proxy/handler/notimplemented.go`
- [X] T096 Validate config compatibility by loading Python TianjiLLM's example proxy_config.yaml files and checking for zero errors, only warnings
- [X] T097 Validate error response format matches Python TianjiLLM for all new endpoints — verify TianjiLLMError wrapping, status codes, error type/message structure per `internal/model/errors.go` (Constitution II.5)
- [X] T098 Run quickstart.md migration verification: compare responses between Python proxy (port 4000) and Go proxy (port 8000) for all supported endpoints
- [X] T099 [P] Run performance benchmark: compare Go vs Python proxy throughput (req/s) and memory usage under load to validate SC-008 (3x memory reduction, 2x throughput)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — no other story dependencies
- **US2 (Phase 4)**: Depends on Phase 2 — no dependency on US1
- **US3 (Phase 5)**: Depends on Phase 2 — no dependency on US1/US2
- **US4 (Phase 6)**: Depends on Phase 2 + **US3** (callback framework required)
- **US5 (Phase 7)**: Depends on Phase 2 + **US3** (pricing data for cost strategy)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

```
Phase 1 (Setup) ─────┐
                      ▼
Phase 2 (Foundation) ─┬──→ US1 (Phase 3) ──→ Phase 8
                      ├──→ US2 (Phase 4) ──→ Phase 8
                      └──→ US3 (Phase 5) ─┬→ US4 (Phase 6) ──→ Phase 8
                                          └→ US5 (Phase 7) ──→ Phase 8
```

- **US1, US2, US3**: Can run in parallel after Phase 2
- **US4**: Must wait for US3 (callback framework)
- **US5**: Must wait for US3 (pricing data)

### Within Each User Story

- Tests written alongside implementation (Constitution IV)
- Models/types before handlers
- Handlers before route registration
- Infrastructure (DB, auth) before handlers that use it
- Framework before integrations (callback.go before webhook.go)

### Parallel Opportunities

- **Phase 1**: T003, T004 in parallel
- **Phase 2**: T007, T008, T009 in parallel (after T006)
- **US1**: All 14 providers (T010–T023) in parallel; all 4 API handlers (T025–T028) in parallel; all 6 pass-through handlers (T031–T036) in parallel
- **US2**: All 7 management handlers (T048–T054) in parallel (after DB/auth)
- **US3**: All 5 callback integrations (T062–T066) in parallel; all 4 spend endpoints (T068–T071) in parallel
- **US4**: All 3 guardrail integrations (T080–T082) in parallel
- **US5**: Usage and tag strategies (T086, T087) in parallel

---

## Parallel Example: User Story 1 (Providers)

```bash
# Launch all 14 providers in parallel (each is a separate directory, zero shared state):
Task: T010 "Implement Cohere provider in internal/provider/cohere/cohere.go"
Task: T011 "Implement Mistral provider in internal/provider/mistral/mistral.go"
Task: T012 "Implement Together AI provider in internal/provider/together/together.go"
Task: T013 "Implement Fireworks AI provider in internal/provider/fireworks/fireworks.go"
Task: T014 "Implement Groq provider in internal/provider/groq/groq.go"
Task: T015 "Implement DeepSeek provider in internal/provider/deepseek/deepseek.go"
Task: T016 "Implement Replicate provider in internal/provider/replicate/replicate.go"
Task: T017 "Implement Hugging Face provider in internal/provider/huggingface/huggingface.go"
Task: T018 "Implement Databricks provider in internal/provider/databricks/databricks.go"
Task: T019 "Implement Cloudflare provider in internal/provider/cloudflare/cloudflare.go"
Task: T020 "Implement Cerebras provider in internal/provider/cerebras/cerebras.go"
Task: T021 "Implement Perplexity provider in internal/provider/perplexity/perplexity.go"
Task: T022 "Implement xAI provider in internal/provider/xai/xai.go"
Task: T023 "Implement SambaNova provider in internal/provider/sambanova/sambanova.go"
```

## Parallel Example: User Story 3 (Callbacks)

```bash
# Launch all 5 callback integrations in parallel (after T060 callback framework):
Task: T062 "Implement webhook callback in internal/callback/webhook/webhook.go"
Task: T063 "Implement Prometheus callback in internal/callback/prometheus/prometheus.go"
Task: T064 "Implement OpenTelemetry callback in internal/callback/otel/otel.go"
Task: T065 "Implement Langfuse callback in internal/callback/langfuse/langfuse.go"
Task: T066 "Implement Datadog callback in internal/callback/datadog/datadog.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (config compat + 501 stubs)
2. Complete Phase 2: Foundational (shared types)
3. Complete Phase 3: User Story 1 (20+ providers + new APIs + pass-through)
4. **STOP and VALIDATE**: Run migration verification per quickstart.md
5. Deploy — this alone covers 80% of Python TianjiLLM users

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 → Test independently → Deploy (MVP!)
3. Add US2 + US3 in parallel → Test independently → Deploy (Enterprise + Observability)
4. Add US4 → Test independently → Deploy (Guardrails)
5. Add US5 → Test independently → Deploy (Advanced Routing)
6. Polish → Final validation → Release

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (providers + APIs) — highest parallelism, can split providers across devs
   - Developer B: US2 (enterprise management)
   - Developer C: US3 (observability + pricing)
3. After US3 completes:
   - Developer C: US4 (guardrails)
   - Developer D: US5 (routing)
4. Stories integrate independently

---

## Summary

| Metric | Count |
|--------|-------|
| Total tasks | 99 |
| Phase 1 (Setup) | 5 |
| Phase 2 (Foundation) | 4 |
| US1 (Core API Parity) | 30 |
| US2 (Enterprise Mgmt) | 18 |
| US3 (Observability) | 20 |
| US4 (Guardrails) | 7 |
| US5 (Adv Routing) | 10 |
| Phase 8 (Polish) | 5 |
| Test tasks (Constitution IV) | 8 |
| Parallelizable tasks | 52 (53%) |
| MVP scope | Phase 1 + 2 + 3 (39 tasks) |

---

## Notes

- [P] tasks = different files, no dependencies, can run in parallel
- [Story] label maps task to specific user story for traceability
- Each provider follows the same pattern: struct embedding openai.BaseProvider, override GetSupportedParams/MapParams, init() with provider.Register()
- Reference `internal/provider/anthropic/` for non-OpenAI provider implementation pattern
- Reference Python source for each provider's supported params and param mapping
- RBAC role names follow Python: proxy_admin, team, internal_user, end_user (not admin/team manager/etc.)
- Credential encryption uses NaCl SecretBox (XSalsa20-Poly1305) matching Python — key = SHA256(TIANJI_SALT_KEY || master_key)
- Python "customer" dimension in spend = `end_user` field in SpendLog (aliased as "customer" in aggregation)
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
