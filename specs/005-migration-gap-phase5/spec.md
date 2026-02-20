# Feature Specification: Phase 5 Migration Gap Closure

**Feature Branch**: `005-migration-gap-phase5`
**Created**: 2026-02-18
**Status**: Draft
**Input**: Phase 5 migration gap closure — high-value remaining features after Phases 1-4 (390/424 tasks completed). Focuses on MCP Server, search providers, image variations, discovery endpoints, high-value providers, and Phase 4 on-demand plugins. Explicitly excludes low-value experimental features (OCR, RAG, Video, Container, A2A).

## Overview

Phases 1-4 brought the Go rewrite to ~92% feature parity with Python TianjiLLM. Phase 5 closes the remaining high-value gaps that block real-world adoption scenarios: AI agent tooling (MCP), web search integrations, missing OpenAI-compatible endpoints, provider coverage for audio/image/specialized platforms, and the 34 on-demand plugins deferred from Phase 4. Low-value experimental endpoints (OCR, RAG, Video, Container, A2A) are explicitly out of scope.

---

## User Scenarios & Testing

### User Story 1 — MCP Server (Model Context Protocol) (Priority: P0)

Claude Code, Cursor, and other MCP-capable clients need to discover and invoke tools exposed by LLM providers through the tianjiLLM proxy. Python TianjiLLM ships a full MCP server (`proxy/_experimental/mcp_server/` — 18 files) with tool registry, semantic filtering, and SSE transport. The Go proxy has zero MCP support, which means MCP clients cannot use tianjiLLM as a tool gateway.

**Why this priority**: MCP adoption is accelerating rapidly. Without MCP server support, tianjiLLM is invisible to the fastest-growing category of AI clients. This is a hard blocker for teams using Claude Code or Cursor with centralized proxy infrastructure.

**Independent Test**: Start tianjiLLM with MCP enabled, connect an MCP client (e.g. `mcp-inspector`), verify tool listing and invocation round-trip.

**Acceptance Scenarios**:

1. **Given** MCP server enabled in config with registered tools, **When** an MCP client sends `tools/list`, **Then** the proxy returns the full tool catalog with descriptions and schemas.
2. **Given** a registered tool backed by an LLM provider, **When** an MCP client sends `tools/call` with valid arguments, **Then** the proxy invokes the provider and returns the tool result.
3. **Given** MCP server enabled with SSE transport, **When** an MCP client connects via SSE, **Then** the proxy establishes a persistent event stream and delivers tool results as SSE events.
4. **Given** MCP tools filtered by semantic tags in config, **When** an MCP client lists tools, **Then** only tools matching the configured filter are returned.
5. **Given** an MCP tool call that fails at the provider, **When** the error propagates back, **Then** the MCP client receives a properly formatted MCP error response with error code and message.

---

### User Story 2 — Search Provider Integration (Priority: P0)

Tool-calling agents (e.g. Claude with web search, GPT-4 with function calling) route web search requests through the proxy. Python TianjiLLM supports 6 search providers: `brave`, `tavily`, `searxng`, `exa_ai`, `google_pse`, `dataforseo`. The Go proxy has none, forcing agents to bypass the proxy for search or lose search capability entirely.

**Why this priority**: Search is the #1 tool used by AI agents. Without search provider support, the proxy breaks the most common agent workflow.

**Independent Test**: Configure a search provider (e.g. Brave), send a search request through the proxy, verify results are returned in the expected format.

**Acceptance Scenarios**:

1. **Given** Brave search provider configured with an API key, **When** a tool-call request invokes web search, **Then** the proxy sends the query to Brave Search API and returns formatted results.
2. **Given** Tavily search provider configured, **When** a search request arrives, **Then** the proxy translates the request to Tavily's API format and returns results.
3. **Given** no search provider configured, **When** a search tool-call arrives, **Then** the proxy returns a clear error indicating no search provider is available.
4. **Given** a search provider returns an error (rate limit, invalid key), **When** the error propagates, **Then** the proxy returns the appropriate error type following the standard error model.
5. **Given** multiple search providers configured, **When** a search request specifies a provider name, **Then** the proxy routes to the specified provider.

---

### User Story 3 — Image Variations Endpoint (Priority: P0)

OpenAI's API includes `POST /v1/images/variations` for creating variations of existing images. The Go proxy supports `images/generations` and `images/edits` (pass-through) but is missing `images/variations`. This is a standard OpenAI endpoint that clients expect to exist.

**Why this priority**: Clients using the OpenAI SDK for image workflows hit 404 on variations. This is a low-effort, high-value gap — it follows the same pass-through pattern as `images/edits`.

**Independent Test**: Send a multipart form request to `/v1/images/variations` with an image file, verify the request is proxied to the upstream provider and the response is returned.

**Acceptance Scenarios**:

1. **Given** an OpenAI provider configured, **When** client sends `POST /v1/images/variations` with an image, **Then** the proxy forwards to OpenAI's variations endpoint and returns the response.
2. **Given** a non-OpenAI provider that doesn't support variations, **When** client sends the request, **Then** the proxy returns a clear "not supported" error.

---

### User Story 4 — Prompt Template Resolution in Chat Flow (Priority: P1)

The prompt CRUD endpoints (`/prompts/*`) are already wired in `server.go` with full handler implementations. However, the chat completion flow does not integrate prompt template resolution — when a request references a stored prompt by name, the proxy does not expand it. Python TianjiLLM resolves prompt templates with variable substitution before sending to the LLM.

**Why this priority**: The endpoints exist but the integration loop is broken. Users can create prompts but cannot use them in chat completions, which defeats the purpose.

**Independent Test**: Store a prompt template, send a chat completion that references it by name, verify the template is expanded with variables before reaching the provider.

**Acceptance Scenarios**:

1. **Given** a stored prompt template with `{{variable}}` placeholders, **When** a chat completion request includes `prompt_name` and `prompt_variables`, **Then** the proxy resolves the template, substitutes variables, and sends the expanded prompt to the provider.
2. **Given** a prompt reference with a specific version, **When** the chat completion arrives, **Then** the proxy resolves the exact version requested.
3. **Given** a prompt reference for a non-existent name, **When** the request arrives, **Then** the proxy returns a clear error indicating the prompt was not found.

---

### User Story 5 — High-Value Provider Coverage (~20 providers) (Priority: P1)

Python TianjiLLM supports 100+ provider integrations. The Go rewrite covers 36 providers. The following ~20 providers are high-value due to customer demand or growing adoption, but are missing from the Go codebase:

**Audio/Speech**: `elevenlabs`, `deepgram`, `aws_polly`
**Image/Creative**: `stability`, `fal_ai`, `recraft`
**Platform (explicit registration)**: `baseten`, `hosted_vllm`
**AI/Embedding**: `codestral`, `friendliai`, `jina_ai`, `voyage`, `infinity`
**Cloud/Infra**: `nebius`, `ovhcloud`, `lambda_ai`, `nscale`, `gigachat`

Note: `ollama`, `vllm`, and `lm_studio` are already covered via the `openaicompat` provider with `api_base` differentiation. The explicit registrations listed above provide named routing (`provider/model` format) without requiring `api_base` in config for every deployment.

**Why this priority**: Each missing provider is a migration blocker for teams using that specific provider. The self-register 2-file pattern makes each provider addition low-risk and independent.

**Independent Test**: For each provider, configure it in YAML, send a chat/embedding/audio request, verify the request reaches the correct upstream API with proper headers and format.

**Acceptance Scenarios**:

1. **Given** a provider configured with its API key in YAML, **When** a request uses `provider/model` format, **Then** the proxy routes to the correct upstream URL with provider-specific headers.
2. **Given** a provider that uses OpenAI-compatible API format, **When** registered via the compatibility base, **Then** the proxy uses the standard request/response transformation.
3. **Given** a provider with a unique request format (e.g. ElevenLabs for TTS), **When** a request arrives, **Then** the proxy transforms the request to the provider's native format and transforms the response back.
4. **Given** an unsupported operation for a provider (e.g. embeddings on a TTS-only provider), **When** the request arrives, **Then** the proxy returns a clear "operation not supported" error.

---

### User Story 6 — Discovery Endpoints (Priority: P1)

Python TianjiLLM provides `/discovery/*` endpoints that let the admin UI and API consumers query available models, providers, and their capabilities. The Go proxy has no discovery mechanism — clients must know the exact model names and capabilities in advance.

**Why this priority**: Without discovery, the admin dashboard cannot populate model dropdowns, and API consumers cannot programmatically discover what the proxy offers. This blocks UI integration.

**Independent Test**: Start tianjiLLM with multiple providers configured, query the discovery endpoints, verify the response lists all available models with their capabilities.

**Acceptance Scenarios**:

1. **Given** multiple providers configured, **When** client queries `GET /model_group/info`, **Then** the proxy returns a list of all model groups with aggregated capabilities (vision, function calling, streaming, web search), context window sizes, and cost per token.
2. **Given** no authentication, **When** client queries `GET /public/providers`, **Then** the proxy returns a sorted list of all supported provider names.
3. **Given** no authentication, **When** client queries `GET /public/tianji_model_cost_map`, **Then** the proxy returns the full model pricing and context window data.
4. **Given** a specific model group name, **When** client queries `GET /model_group/info?model_group={name}`, **Then** only that model group's aggregated capabilities are returned.

---

### User Story 7 — Phase 4 Remaining On-demand Plugins (Priority: P2)

Phase 4 defined 34 on-demand plugins (22 guardrails + 12 callbacks) in tasks T082-T115 that were deferred to "implement as needed." These plugins follow the established 2-file pattern (implementation + factory registration) and are independent of each other. Phase 5 provides the complete implementations to achieve full plugin parity.

**Guardrails (22)**: AIM, Aporia, Custom Code, DynamoAI, EnkryptAI, GraySwan, Guardrails AI, HiddenLayer, IBM, Javelin, Lakera AI v2, Lasso, Model Armor, Noma, Onyx, Pangea, PANW Prisma AIRS, Pillar, Prompt Security, Qualifire, Unified Guardrail, Zscaler AI Guard.

**Callbacks (12)**: Arize (full), AgentOps, Focus, HumanLoop, LangTrace, Levo, Weave, Bitbucket, GitLab, DotPrompt, WebSearch Interception, Custom Batch Logger.

**Why this priority**: Each plugin is requested by specific customers/use cases. The plugin system is designed for this — adding one has zero blast radius on existing functionality.

**Independent Test**: For each plugin, configure it in YAML, trigger a request, verify the plugin's pre/post-call hook is invoked and communicates with the external service.

**Acceptance Scenarios**:

1. **Given** a guardrail configured in the `guardrails` YAML section, **When** a request arrives, **Then** the guardrail's pre-call check runs and blocks/allows the request based on the external service response.
2. **Given** a callback configured in `tianji_settings.callbacks`, **When** an LLM call completes, **Then** the callback receives the call metadata and sends it to the external service.
3. **Given** a guardrail service is unreachable, **When** the pre-call check runs, **Then** the behavior follows the configured fail mode (fail-open or fail-closed).
4. **Given** multiple guardrails configured, **When** a request arrives, **Then** all guardrails execute in the configured order and the first rejection stops the chain.

---

### User Story 8 — AutoRouter (Semantic Routing) (Priority: P2)

Python TianjiLLM includes an ML-based auto-router that analyzes prompt content to select the best model group. This is a differentiating feature for cost optimization — simple prompts go to cheaper models, complex prompts go to capable models.

**Why this priority**: This is a power-user feature with high complexity. Implementing it closes the last major algorithmic gap, but it's lower priority than the infrastructure gaps above.

**Independent Test**: Configure auto-router with model groups tagged by capability, send prompts of varying complexity, verify routing decisions match expected capability tiers.

**Acceptance Scenarios**:

1. **Given** auto-router configured with model groups for different capability tiers, **When** a simple factual prompt arrives, **Then** it's routed to the cost-effective model group.
2. **Given** a complex reasoning prompt, **When** auto-router evaluates it, **Then** it's routed to the high-capability model group.
3. **Given** auto-router disabled (default), **When** requests arrive, **Then** standard routing strategy applies with zero overhead.
4. **Given** auto-router's classification model is unavailable, **When** a request arrives, **Then** it falls back to the default routing strategy with a logged warning.

---

### Edge Cases

- What happens when an MCP client sends `tools/call` for a tool that was removed since the last `tools/list`?
- How does the search provider handle queries in non-English languages?
- What happens when image variations are requested with an image format the provider doesn't support?
- How does prompt template resolution interact with streaming responses?
- What happens when a guardrail and a callback both target the same external service with different credentials?
- How does auto-router handle multimodal prompts (text + image)?
- What happens when discovery endpoints are queried while provider configuration is being hot-reloaded?
- How does the MCP server handle concurrent tool calls from the same client session?

---

## Requirements

### Functional Requirements

- **FR-001**: The MCP server MUST implement the MCP protocol specification (tools/list, tools/call) with both SSE and Streamable HTTP transports.
- **FR-002**: Search providers MUST self-register using the existing provider registration pattern and return results in a standardized format.
- **FR-003**: The `POST /v1/images/variations` endpoint MUST be proxied to upstream providers following the same pass-through pattern as `images/edits`.
- **FR-004**: Prompt template resolution MUST be integrated into the chat completion flow, supporting `{{variable}}` substitution and version pinning.
- **FR-005**: Each new provider MUST follow the self-register 2-file pattern (implementation + `init()` registration) with zero changes to existing code.
- **FR-006**: Discovery endpoints MUST return accurate, real-time information about configured models and their capabilities.
- **FR-007**: Each guardrail and callback plugin MUST follow the established 2-file pattern (implementation + factory registration).
- **FR-008**: AutoRouter MUST be opt-in (disabled by default) and MUST fall back to standard routing when the classification model is unavailable.
- **FR-009**: All new features MUST NOT break any existing functionality (zero regression).
- **FR-010**: All new features MUST include contract tests verifying behavior.

### Key Entities

- **MCP Tool**: A callable function exposed to MCP clients. Has name, description, input schema, and a backing provider/model.
- **Search Provider**: An external search API (Brave, Tavily, etc.) registered as a provider for tool-call routing.
- **Prompt Template**: A stored template with name, version, variable placeholders, and optional model binding. Resolved at request time.
- **Discovery Model**: A read-only view of a configured model's name, provider, capabilities, and constraints. Assembled from config + provider metadata.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: An MCP client can list and invoke tools through tianjiLLM with proxy layer overhead < 50ms (excluding upstream MCP server latency).
- **SC-002**: All 6 search providers pass contract tests with real API responses (mocked in CI, live in integration).
- **SC-003**: `POST /v1/images/variations` returns the same response structure as the upstream provider's API for configured providers.
- **SC-004**: A chat completion referencing a stored prompt template produces identical output to one with the template manually expanded.
- **SC-005**: All ~20 new providers pass the standard provider contract test suite (request transformation → HTTP → response transformation round-trip).
- **SC-006**: Discovery endpoints return accurate model lists within 1 second for configurations with up to 100 model groups.
- **SC-007**: Adding a new guardrail or callback plugin requires exactly 2 files with zero changes to existing code.
- **SC-008**: Zero regressions detected by existing test suite after each feature is implemented.
- **SC-009**: Go proxy can run the same `proxy_config.yaml` as Python TianjiLLM for all resolved features without modification.

---

## Out of Scope

The following Python TianjiLLM features are explicitly excluded from Phase 5:

- **OCR Endpoints** (`proxy/ocr_endpoints/`) — experimental, low adoption
- **RAG Endpoints** (`proxy/rag_endpoints/`) — experimental, better served by dedicated RAG frameworks
- **Video Endpoints** (`proxy/video_endpoints/`) — experimental, low adoption
- **Container Endpoints** (`proxy/container_endpoints/`) — experimental, niche use case
- **A2A (Agent-to-Agent) Endpoints** (`proxy/agent_endpoints/`) — protocol still evolving
- **Proxy Client SDK** (`proxy/client/`) — design difference, Go is server-only
- **Qdrant Semantic Cache** — niche, can be added later as a cache backend plugin
- **OpenAPI Spec Generation** — nice-to-have, not a migration blocker
- **UI Dashboard Backend** — large scope, deserves its own dedicated phase
