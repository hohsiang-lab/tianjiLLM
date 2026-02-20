# Feature Specification: Full Migration Gap Analysis (Phase 4)

**Feature Branch**: `004-migration-gap-phase4`
**Created**: 2026-02-17
**Status**: Draft
**Input**: Full migration gap analysis between Python TianjiLLM and Go TianjiLLM rewrite

## Overview

Go rewrite of TianjiLLM completed 3 phases (260+ tasks marked done), covering core proxy, provider, router, auth, budget, caching, callbacks, guardrails, and enterprise features. However, Python TianjiLLM continues to evolve rapidly, and several Go implementations have "code exists but not wired" or "field exists but no logic" issues. This spec catalogs every gap between the two codebases to enable prioritized batch implementation.

---

## Category A: Blocking Gaps (Prevent Python Users from Migrating)

These are features that Python TianjiLLM users actively rely on. Missing any of them is a hard blocker for migration.

### A1 - Pass-through Endpoints Not Wired (Priority: P0)

**Python**: `proxy/pass_through_endpoints/` — supports `/vertex/*`, `/anthropic/*`, `/bedrock/*`, `/azure/*`, `/gemini/*` + user-defined custom pass-through routes.

**Go Status**: `internal/proxy/passthrough/` package exists with handler code, but `server.go:154` returns `501 NotImplemented`. The package is never mounted to any route.

**Severity**: Critical

**Acceptance Scenarios**:

1. **Given** a proxy with Vertex AI credentials configured, **When** client sends `POST /vertex-ai/v1/projects/*/locations/*/publishers/*/models/*:generateContent`, **Then** the request is forwarded to Vertex AI and the response is returned verbatim.
2. **Given** a proxy with custom pass-through route defined in YAML config, **When** client sends a request matching that route, **Then** the request is forwarded to the configured upstream URL with configured headers.

---

### A2 - Responses API (Priority: P0)

**Python**: `proxy/response_api_endpoints/` + `proxy/response_polling/` — full implementation including background streaming + polling for async responses.

**Go Status**: `handler/responses.go:28` returns `501 NotImplemented`.

**Severity**: Critical

**Acceptance Scenarios**:

1. **Given** a configured proxy, **When** client sends `POST /v1/responses` with a prompt, **Then** the proxy returns a response object with a response ID.
2. **Given** a background response in progress, **When** client polls `GET /v1/responses/{id}`, **Then** the proxy returns current status and partial/complete results.

---

### A3 - SSO/OIDC Login Flow (Priority: P1)

**Python**: `proxy/custom_sso.py` — complete IDP redirect + callback + role mapping flow.

**Go Status**: `handler/sso.go:20,45` — both `/sso/login` and `/sso/callback` endpoints return `501 NotImplemented`.

**Severity**: High

**Acceptance Scenarios**:

1. **Given** SSO configured with an OIDC provider, **When** user navigates to `/sso/login`, **Then** they are redirected to the IDP login page.
2. **Given** a successful IDP authentication, **When** IDP redirects to `/sso/callback` with auth code, **Then** the proxy exchanges the code for tokens, maps roles, and returns a session JWT.

---

### A4 - General Fallback Chain (Priority: P1)

**Python**: `router.py` — `default_fallbacks` + `fallbacks=[{"model-A": ["model-B"]}]` — arbitrary error-based fallback to alternative models.

**Go Status**: Router only implements `ContextWindowFallbacks`. No general error-based fallback mechanism.

**Severity**: High

**Acceptance Scenarios**:

1. **Given** config with `fallbacks: [{"gpt-4": ["claude-3", "gemini-pro"]}]`, **When** `gpt-4` returns a 500 error, **Then** the router retries with `claude-3`, then `gemini-pro`.
2. **Given** config with `default_fallbacks: ["fallback-model"]`, **When** any model fails with a retryable error and no model-specific fallback is defined, **Then** the router tries `fallback-model`.

---

### A5 - Model Group Alias Resolution (Priority: P1)

**Python**: `router.py` — `model_group_alias` config field + alias lookup during routing.

**Go Status**: Config struct has `ModelGroupAlias` field, but `router.go`'s `Route()` method never reads or uses it.

**Severity**: High

**Acceptance Scenarios**:

1. **Given** config with `model_group_alias: {"gpt-4": "gpt-4-turbo-preview"}`, **When** client requests model `gpt-4`, **Then** the router resolves it to the `gpt-4-turbo-preview` model group.

---

### A6 - Realtime API (WebSocket) (Priority: P1)

**Python**: `common_utils/realtime_utils.py` — OpenAI Realtime API + Vertex Live API WebSocket proxy.

**Go Status**: No WebSocket support at all. The Go proxy is HTTP-only.

**Severity**: High

**Acceptance Scenarios**:

1. **Given** a proxy with OpenAI API key configured, **When** client opens a WebSocket connection to `/v1/realtime`, **Then** the proxy establishes an upstream WebSocket to OpenAI and relays messages bidirectionally.
2. **Given** a proxy with Vertex AI credentials, **When** client opens a WebSocket to the Vertex Live API path, **Then** the proxy relays the connection to Vertex AI's live endpoint.

---

## Category B: Router Advanced Strategy Gaps

These are routing capabilities that power users depend on for cost optimization, compliance, and traffic management.

### B1 - Region-based Routing (Priority: P2)

**Python**: `router.py` — `allowed_model_region` parameter + `is_region_allowed()` filtering.

**Go Status**: Config has `Region` field on deployments, but router strategy layer never uses it for selection filtering.

**Acceptance Scenarios**:

1. **Given** deployments in `us-east1` and `eu-west1`, **When** request specifies `allowed_model_region: "eu"`, **Then** only `eu-west1` deployments are considered.

---

### B2 - TPM/RPM-based Routing (Priority: P2)

**Python**: `router_strategy/lowest_tpm_rpm_v2.py` — route to deployment with lowest token/request-per-minute usage.

**Go Status**: `usage.go` tracker exists but is not used as a selection strategy.

**Acceptance Scenarios**:

1. **Given** two deployments with different current TPM usage, **When** `routing_strategy: lowest-tpm-rpm` is configured, **Then** the deployment with lower TPM usage is preferred.

---

### B3 - Priority Queue / Scheduler (Priority: P2)

**Python**: `router.py` — `Scheduler` class + `default_priority` + `scheduler_acompletion()` — request priority queue with weighted scheduling.

**Go Status**: Go has a `scheduler` package but it's a background job scheduler (cron-style), not a request priority queue. Completely different concept.

**Acceptance Scenarios**:

1. **Given** requests with `priority: 0` (high) and `priority: 3` (low), **When** both arrive simultaneously under capacity constraints, **Then** priority-0 requests are served first.

---

### B4 - Content-based Routing (AutoRouter) (Priority: P3)

**Python**: `router_strategy/auto_router/` — semantic router that analyzes prompt content to route to appropriate model group.

**Go Status**: Not implemented.

**Acceptance Scenarios**:

1. **Given** auto-router configured with model groups for different task types, **When** a coding prompt arrives, **Then** it's routed to the code-optimized model group.

---

### B5 - Model-group-specific Retry Policy (Priority: P2)

**Python**: `router.py` — `model_group_retry_policy` — per-model-group retry/timeout configuration.

**Go Status**: Only global `NumRetries` is supported.

**Acceptance Scenarios**:

1. **Given** config with `model_group_retry_policy: {"gpt-4": {"num_retries": 5}, "claude": {"num_retries": 2}}`, **When** `gpt-4` fails, **Then** it retries up to 5 times; when `claude` fails, it retries up to 2 times.

---

### B6 - Tag Routing match_any Mode (Priority: P2)

**Python**: `router_strategy/tag_based_routing.py` — supports both `match_any=True` and `match_any=False` modes.

**Go Status**: `strategy/tag.go` only implements `hasAllTags` (match_all). Missing `match_any` mode.

**Acceptance Scenarios**:

1. **Given** deployment tagged `["fast", "cheap"]` and request with tags `["fast"]` and `match_any=true`, **Then** the deployment is selected because it matches at least one tag.
2. **Given** the same setup with `match_any=false`, **Then** the deployment is NOT selected because it doesn't match ALL request tags.

---

## Category C: Guardrail Integration Gaps

### Existing Guardrails in Go (10)

openai_moderation, presidio, prompt_injection, lakera_guard, bedrock_guardrail, azure_prompt_shield, azure_text_moderation, content_filter, tool_permission, generic

### Missing Guardrails (20+) (Priority: P3)

The following guardrail integrations exist in Python but not in Go:

| # | Guardrail | Python Location |
|---|-----------|-----------------|
| C1 | AIM | `guardrails/aim_guard.py` |
| C2 | Aporia | `guardrails/aporia_guard.py` |
| C3 | Custom Code | `guardrails/custom_code_guard.py` |
| C4 | DynamoAI | `guardrails/dynamoai_guard.py` |
| C5 | EnkryptAI | `guardrails/enkryptai_guard.py` |
| C6 | GraySwan | `guardrails/grayswan_guard.py` |
| C7 | Guardrails AI | `guardrails/guardrails_ai_guard.py` |
| C8 | HiddenLayer | `guardrails/hiddenlayer_guard.py` |
| C9 | IBM Guardrails | `guardrails/ibm_guardrails.py` |
| C10 | Javelin | `guardrails/javelin_guard.py` |
| C11 | Lakera AI v2 | `guardrails/lakera_ai_v2_guard.py` |
| C12 | Lasso | `guardrails/lasso_guard.py` |
| C13 | Model Armor | `guardrails/model_armor_guard.py` |
| C14 | Noma | `guardrails/noma_guard.py` |
| C15 | Onyx | `guardrails/onyx_guard.py` |
| C16 | Pangea | `guardrails/pangea_guard.py` |
| C17 | PANW Prisma AIRS | `guardrails/panw_prisma_airs_guard.py` |
| C18 | Pillar | `guardrails/pillar_guard.py` |
| C19 | Prompt Security | `guardrails/prompt_security_guard.py` |
| C20 | Qualifire | `guardrails/qualifire_guard.py` |
| C21 | Unified Guardrail | `guardrails/unified_guardrail.py` |
| C22 | Zscaler AI Guard | `guardrails/zscaler_ai_guard.py` |

**Acceptance Scenario** (applies to all):

1. **Given** a guardrail configured in `guardrails` YAML section, **When** a request arrives, **Then** the guardrail's pre-call check runs and blocks/allows the request based on the external service response.

---

## Category D: Proxy Feature Gaps

### D1 - UI Dashboard Backend (Priority: P2)

**Python**: `proxy/ui_crud_endpoints/`, `common_utils/admin_ui_utils.py` — backend for the admin dashboard UI.

**Go Status**: Completely missing.

**Acceptance Scenarios**:

1. **Given** a running proxy, **When** admin navigates to the dashboard URL, **Then** the backend serves the dashboard UI and provides CRUD API endpoints for keys, teams, users, and models.

---

### D2 - Prompt Management (Priority: P2)

**Python**: `proxy/prompts/` — prompt template registry + CRUD endpoints.

**Go Status**: DB has `PromptTemplateTable` but no handler endpoints exist.

**Acceptance Scenarios**:

1. **Given** an authenticated admin, **When** they `POST /prompt/new` with a template, **Then** the prompt is stored and retrievable by name.
2. **Given** a stored prompt template, **When** a chat request references it, **Then** the template is expanded with provided variables before being sent to the LLM.

---

### D3 - Agent (A2A) Endpoints (Priority: P3)

**Python**: `proxy/agent_endpoints/` — Agent-to-Agent routing + agent registry.

**Go Status**: Completely missing.

---

### D4 - MCP Tools Integration (Priority: P3)

**Python**: `proxy/mcp_tools.py` + `mcp_registry.json` — MCP tool server integration.

**Go Status**: Completely missing.

---

### D5 - OCR Endpoints (Priority: P3)

**Python**: `proxy/ocr_endpoints/` — OCR processing endpoints.

**Go Status**: Completely missing.

---

### D6 - Video Endpoints (Priority: P3)

**Python**: `proxy/video_endpoints/` — video processing endpoints.

**Go Status**: Completely missing.

---

### D7 - RAG Endpoints (Priority: P3)

**Python**: `proxy/rag_endpoints/` — RAG (Retrieval-Augmented Generation) endpoints.

**Go Status**: Completely missing.

---

### D8 - Search Endpoints (Priority: P3)

**Python**: `proxy/search_endpoints/` — search functionality endpoints.

**Go Status**: Completely missing.

---

### D9 - Container Endpoints (Priority: P3)

**Python**: `proxy/container_endpoints/` — container management endpoints.

**Go Status**: Completely missing.

---

### D10 - Config Pass-through Endpoints (Priority: P2)

**Python**: `proxy/config_management_endpoints/pass_through_endpoints.py` — user-defined custom routes in YAML config that forward to arbitrary upstreams.

**Go Status**: Completely missing.

**Acceptance Scenarios**:

1. **Given** YAML config defines a pass-through route `path: /custom/api, target: https://internal.service/v1`, **When** client sends `POST /custom/api/resource`, **Then** the proxy forwards to `https://internal.service/v1/resource`.

---

### D11 - Proxy Client SDK (Priority: P3)

**Python**: `proxy/client/` — Python SDK for proxy management API.

**Go Status**: N/A (design difference — Go is server-only). Consider whether a Go client SDK is needed.

---

### D12 - Custom Hooks/Plugins (Priority: P2)

**Python**: `proxy/hooks/`, `proxy/custom_hooks/` — extensible hook system including `dynamic_rate_limiter`, `batch_rate_limiter`, `cache_control_check`, `responses_id_security`.

**Go Status**: Has guardrail hook interface but no custom hook plugin mechanism for arbitrary user-defined pre/post-call logic.

**Acceptance Scenarios**:

1. **Given** a custom hook registered in config, **When** a request arrives, **Then** the hook's pre-call function is invoked and can modify or reject the request.

---

### D13 - Post-call Rules (Priority: P3)

**Python**: `proxy/post_call_rules.py`, `_experimental/post_call_rules.py` — rules applied after LLM response is received.

**Go Status**: Completely missing.

---

### D14 - Key Rotation Manager (Priority: P2)

**Python**: `common_utils/key_rotation_manager.py` — automatic rotation of provider API keys.

**Go Status**: Scheduler has `CredentialRefreshJob` but it's not the same as key rotation (refreshing OAuth tokens vs. rotating API keys).

**Acceptance Scenarios**:

1. **Given** key rotation configured for a provider, **When** the rotation interval elapses, **Then** the proxy generates/fetches a new API key and seamlessly switches to it without downtime.

---

### D15 - Discovery Endpoints (Priority: P3)

**Python**: `proxy/discovery_endpoints/ui_discovery_endpoints.py` — endpoints for UI to discover available models, providers, and capabilities.

**Go Status**: Completely missing.

---

### D16 - OpenAPI Spec Generation (Priority: P3)

**Python**: `common_utils/custom_openapi_spec.py`, `swagger_utils.py` — auto-generated OpenAPI documentation.

**Go Status**: Completely missing.

---

## Category E: Callback / Observability Gaps

### Existing Callbacks in Go (33+)

prometheus, otel, langfuse, datadog, datadog_llm, webhook, slack, lunary, traceloop, posthog, opik, gcspubsub, openmeter, greenscale, promptlayer, argilla, lago, azuresentinel, supabase, cloudzero, logfire, athina, deepeval, galileo, literalai, s3, gcs, azureblob, dynamodb, sqs, wandb, mlflow, helicone, langsmith, braintrust, email

### Missing Callbacks (Priority: P3)

| # | Callback | Python Location | Notes |
|---|----------|-----------------|-------|
| E1 | Arize (full) | `integrations/arize/` (directory-level, more complete than Go's single file) | Go has basic Arize but Python's is richer |
| E2 | AgentOps | `integrations/agentops/` | Agent observability platform |
| E3 | Focus | `integrations/focus/` | Focus AI integration |
| E4 | HumanLoop | `integrations/humanloop.py` | Human-in-the-loop platform |
| E5 | LangTrace | `integrations/langtrace.py` | LangTrace observability |
| E6 | Levo | `integrations/levo/` | API security testing |
| E7 | Weave (W&B Weave) | `integrations/weave/` | Weights & Biases Weave |
| E8 | Bitbucket | `integrations/bitbucket/` | Bitbucket integration |
| E9 | GitLab | `integrations/gitlab/` | GitLab integration |
| E10 | DotPrompt | `integrations/dotprompt/` | Prompt management |
| E11 | WebSearch Interception | `integrations/websearch_interception/` | Web search result interception |
| E12 | Custom Batch Logger | `integrations/custom_batch_logger.py` | Custom batch logging |

**Acceptance Scenario** (applies to all):

1. **Given** a callback configured in `tianji_settings.callbacks`, **When** an LLM call completes, **Then** the callback receives the call metadata and sends it to the external service.

---

## Category F: Core Capability Gaps

### F1 - Token Counting (Pre-request) (Priority: P2)

**Python**: `utils.py` — tiktoken-based precise token counting before request.

**Go Status**: No pre-request token counting. Relies entirely on provider-returned usage.

**Acceptance Scenarios**:

1. **Given** a chat request with messages, **When** the proxy processes it, **Then** it calculates input token count before sending to the provider (for budget checks, rate limiting, routing decisions).

---

### F2 - LLM Response Caching Handler (Priority: P1)

**Python**: `caching_handler.py` / `llm_caching_handler.py` — request-level cache hit/miss handling integrated into the chat completion flow.

**Go Status**: Cache package exists with memory/Redis/dual backends, but the chat handler does not integrate cache lookup/store logic.

**Acceptance Scenarios**:

1. **Given** caching enabled and a previous identical request cached, **When** the same request arrives, **Then** the proxy returns the cached response without calling the upstream LLM.
2. **Given** a cache miss, **When** the LLM responds, **Then** the response is stored in cache with the configured TTL.

---

### F3 - Qdrant Semantic Cache (Priority: P3)

**Python**: `integrations/vector_store_integrations/` — vector similarity-based caching using Qdrant.

**Go Status**: Not implemented.

---

### F4 - Custom Secret Manager Plugin (Priority: P3)

**Python**: `integrations/custom_secret_manager.py` — pluggable secret manager for fetching API keys from external vaults.

**Go Status**: Not implemented.

---

### F5 - Slack Alerting Advanced Features (Priority: P2)

**Python**: `integrations/SlackAlerting/` — hanging request detection, daily reports, outage detection, per-type webhook routing.

**Go Status**: `callback/slack.go` is only 111 lines with slow-request + budget threshold alerts.

**Acceptance Scenarios**:

1. **Given** Slack alerting configured, **When** a request hangs for longer than the threshold, **Then** a Slack alert is sent with request details.
2. **Given** daily report enabled, **When** the scheduled time arrives, **Then** a summary of the day's usage, errors, and costs is posted to Slack.
3. **Given** outage detection enabled, **When** a provider returns errors above threshold, **Then** a Slack alert is sent identifying the outage.

---

### F6 - Dynamic Rate Limiter (Priority: P2)

**Python**: `proxy/hooks/dynamic_rate_limiter_v3.py` — TPM-based dynamic rate limiting that adjusts limits based on current utilization.

**Go Status**: Not implemented.

**Acceptance Scenarios**:

1. **Given** dynamic rate limiting enabled, **When** a model group's TPM usage approaches the configured limit, **Then** new requests are throttled proportionally.

---

### F7 - Batch Rate Limiter (Priority: P3)

**Python**: `proxy/hooks/batch_rate_limiter.py` — rate limiting for batch API endpoints.

**Go Status**: Not implemented.

---

### F8 - Model Max Budget Limiter (Priority: P2)

**Python**: `proxy/hooks/model_max_budget_limiter.py` — per-model budget cap enforcement.

**Go Status**: Not implemented.

**Acceptance Scenarios**:

1. **Given** model `gpt-4` has a max budget of $100/month configured, **When** the accumulated spend reaches $100, **Then** all subsequent requests to `gpt-4` are rejected with a budget exceeded error.

---

## Requirements

### Functional Requirements

- **FR-001**: All Category A gaps (A1-A6) MUST be resolved before declaring Go proxy as migration-ready.
- **FR-002**: Category B gaps (B1-B6) MUST have at least B1, B2, B5, B6 resolved for feature parity with common Python TianjiLLM deployments.
- **FR-003**: Category C guardrails SHOULD be implemented on-demand based on user adoption data. The guardrail plugin system MUST make adding new guardrails a single-file addition.
- **FR-004**: Category D features D1, D2, D10, D12, D14 MUST be resolved for enterprise deployments.
- **FR-005**: Category E callbacks SHOULD be implemented on-demand. The callback system MUST make adding new callbacks a single-file addition.
- **FR-006**: Category F gaps F1, F2, F5, F6, F8 MUST be resolved as they affect correctness and observability.
- **FR-007**: Each gap resolution MUST include contract tests verifying behavior matches Python TianjiLLM.
- **FR-008**: Each gap resolution MUST NOT break any existing functionality (zero regression).

### Key Entities

- **Gap Item**: A specific feature/capability that exists in Python TianjiLLM but is missing or incomplete in Go. Has category, priority, severity, Python location, Go status.
- **Category**: One of A (Blocking), B (Router), C (Guardrail), D (Proxy), E (Callback), F (Core). Determines implementation priority band.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All Category A (P0/P1) gaps pass contract tests that mirror Python TianjiLLM behavior.
- **SC-002**: Router handles all Category B strategies with config-only activation (no code changes needed to switch strategy).
- **SC-003**: Adding a new guardrail or callback requires touching at most 2 files (implementation + registration).
- **SC-004**: Zero regressions detected by existing test suite after each gap is resolved.
- **SC-005**: Go proxy can run the same `proxy_config.yaml` as Python TianjiLLM for all resolved gaps without modification.

## Edge Cases

- What happens when a pass-through endpoint conflicts with a built-in route?
- How does the fallback chain interact with budget limits (should fallback be tried even if budget is exceeded)?
- What happens when WebSocket connection drops mid-stream — does the proxy clean up upstream connections?
- How does model group alias interact with fallback chains (alias resolution before or after fallback lookup)?
- What happens when a guardrail service is unreachable — fail-open or fail-closed?
- How does the cache handler interact with streaming responses?
