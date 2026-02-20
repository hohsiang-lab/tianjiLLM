# Feature Specification: TianjiLLM Python-to-Go Migration Gap Analysis & Roadmap

**Feature Branch**: `001-migration-gap-analysis`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "Analyze all features in Python TianjiLLM proxy that are missing from tianjiLLM and create a prioritized migration roadmap"

## Clarifications

### Session 2026-02-16

- Q: Provider expansion strategy — should new providers use JSON config (openaicompat) or dedicated code modules? → A: Follow TianjiLLM pattern — every provider gets its own code module that inherits from an OpenAI base class, enabling per-provider parameter mapping, header customization, and error handling overrides.
- Q: Pass-through endpoint architecture — generic URL forwarding or per-provider handlers? → A: Follow TianjiLLM pattern — generic pass-through router with per-provider auth/logging handlers (Anthropic, Vertex AI, OpenAI, Cohere, Gemini, etc.), plus guardrails support and streaming.
- Q: Model pricing data source for spend tracking? → A: Follow TianjiLLM pattern — embed a maintained model pricing JSON (~37K lines with input/output token prices + context window sizes), allow user override via custom_pricing config.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Core API Parity for Drop-in Replacement (Priority: P1)

As a platform engineer currently using Python TianjiLLM proxy, I want tianjiLLM to support the same core LLM endpoints so that I can switch from Python to Go without breaking my existing client integrations.

**Why this priority**: Without core API parity, no user can adopt tianjiLLM as a replacement. This is the foundation everything else builds on.

**Independent Test**: Can be fully tested by sending the same client requests to both Python and Go proxies and comparing responses. Delivers value by enabling migration for users who only use basic LLM endpoints.

**Acceptance Scenarios**:

1. **Given** a client sending a chat completion request to tianjiLLM, **When** the request uses any model format supported by Python TianjiLLM (e.g. `cohere/command-r`, `mistral/mistral-large`), **Then** tianjiLLM routes it correctly and returns an OpenAI-compatible response
2. **Given** a client using the Files API, Batches API, or Fine-tuning API, **When** requests are sent to tianjiLLM, **Then** the proxy correctly forwards them to the upstream provider and returns compatible responses
3. **Given** a client using pass-through mode for provider-specific endpoints (e.g. Anthropic Messages API), **When** the request is sent to tianjiLLM, **Then** the proxy forwards it without transformation and returns the raw provider response

---

### User Story 2 - Enterprise Management & Access Control (Priority: P2)

As a team lead or admin, I want tianjiLLM to support full key/team/user/organization management with proper RBAC so that I can manage access, budgets, and permissions for my organization.

**Why this priority**: Enterprise customers require access control, budget management, and multi-tenant support before they can adopt any LLM proxy in production.

**Independent Test**: Can be fully tested by creating organizations, teams, users, and keys with different permission levels, then verifying access control enforcement.

**Acceptance Scenarios**:

1. **Given** an admin user, **When** they create an organization with teams and users, **Then** the hierarchy is enforced and each entity has correct budget and model access limits
2. **Given** a virtual key with model restrictions, **When** a request is made for a disallowed model, **Then** the proxy returns a 403 with a clear error message
3. **Given** SSO is configured, **When** a user logs in via their identity provider, **Then** they are assigned the correct team and role based on IDP claims

---

### User Story 3 - Observability & Cost Tracking (Priority: P2)

As a platform engineer, I want tianjiLLM to provide logging callbacks and spend analytics so that I can monitor usage, track costs, and integrate with my existing observability stack.

**Why this priority**: Production deployments require visibility into usage, costs, and errors. Without observability, operators are flying blind.

**Independent Test**: Can be fully tested by sending requests through the proxy and verifying that spend logs are recorded and callbacks fire to configured destinations.

**Acceptance Scenarios**:

1. **Given** logging callbacks are configured (e.g. to a webhook endpoint), **When** a request completes, **Then** the proxy sends a structured log payload with model, tokens, cost, latency, and metadata
2. **Given** spend tracking is enabled, **When** an admin queries spend by key/team/user/model, **Then** accurate aggregated spend data is returned
3. **Given** Prometheus metrics are enabled, **When** the proxy is scraped, **Then** standard LLM metrics (request count, latency histogram, token usage, error rate) are exposed

---

### User Story 4 - Guardrails & Content Safety (Priority: P3)

As a compliance officer, I want tianjiLLM to support content moderation and PII detection guardrails so that I can enforce safety policies on all LLM traffic.

**Why this priority**: Regulated industries require content filtering before they can use LLM proxies. This is a blocker for healthcare, finance, and government customers.

**Independent Test**: Can be fully tested by configuring guardrails and sending requests with PII or harmful content, then verifying they are blocked or redacted.

**Acceptance Scenarios**:

1. **Given** a PII detection guardrail is configured, **When** a request contains personal information, **Then** the proxy redacts the PII before forwarding to the provider
2. **Given** a content moderation guardrail is configured, **When** a response contains harmful content, **Then** the proxy blocks the response and returns a policy violation error
3. **Given** guardrails are configured per-key or per-team, **When** different keys send requests, **Then** only the guardrails assigned to that key/team are applied

---

### User Story 5 - Advanced Routing & Reliability (Priority: P3)

As a platform engineer, I want tianjiLLM to support advanced routing strategies (cost-based, latency-based, tag-based) and comprehensive fallback chains so that I can optimize for cost and reliability.

**Why this priority**: Advanced routing directly impacts cost and reliability. Users with multi-provider setups need granular control over routing decisions.

**Independent Test**: Can be fully tested by configuring multiple deployments with different strategies and verifying that routing decisions match expected behavior.

**Acceptance Scenarios**:

1. **Given** cost-based routing is configured, **When** multiple deployments are available, **Then** the proxy selects the cheapest healthy deployment
2. **Given** tag-based routing is configured, **When** a request includes a routing tag, **Then** only deployments matching that tag are considered
3. **Given** context window fallbacks are configured, **When** a request exceeds a model's context limit, **Then** the proxy automatically retries with a model that has a larger context window

---

### Edge Cases

- What happens when a provider is added in Python TianjiLLM but has no equivalent in tianjiLLM? The proxy should return a clear "unsupported provider" error with the provider name
- What happens when a Python config file uses features not yet migrated to Go? The config loader should warn about unsupported fields at startup rather than silently ignoring them
- What happens when a client sends a request to an endpoint that exists in Python but not in Go? The proxy should return 501 Not Implemented with a message indicating the feature is not yet available
- How does the system handle provider-specific parameters that only exist in Python's transformation layer? The proxy should pass through unknown parameters and log a warning

## Requirements *(mandatory)*

### Functional Requirements

**Phase 1: Provider Expansion (Core API Parity)**

- **FR-001**: System MUST support at least 20 of the most popular LLM providers (including Cohere, Mistral, Together AI, Fireworks AI, Groq, Replicate, Deepseek, Hugging Face, Databricks, Cloudflare, Cerebras, Perplexity, XAI, SambaNova). Each provider MUST have its own code module inheriting from an OpenAI base, enabling per-provider parameter mapping, header customization, and error handling
- **FR-002**: System MUST implement the Files API (upload, list, get, download, delete)
- **FR-003**: System MUST implement the Batches API (create, get, cancel, list)
- **FR-004**: System MUST implement the Fine-tuning API (create, get, cancel, list events, list checkpoints)
- **FR-005**: System MUST implement pass-through endpoints with a generic router and per-provider auth/logging handlers (at minimum: Anthropic, Vertex AI, OpenAI, Cohere, Gemini). Pass-through MUST support guardrails and streaming
- **FR-006**: System MUST implement the Rerank API endpoint

**Phase 2: Enterprise Management**

- **FR-007**: System MUST implement full CRUD for organizations (create, read, update, delete, member management)
- **FR-008**: System MUST implement key update operations (currently only create/delete/block exist)
- **FR-009**: System MUST implement team update and member management operations
- **FR-010**: System MUST implement SSO authentication via standard identity protocols with role mapping
- **FR-011**: System MUST implement role-based access control with at least 4 roles (proxy_admin, team, internal_user, end_user)
- **FR-012**: System MUST implement working budget endpoints (currently return 501)
- **FR-013**: System MUST implement access group and model access group management
- **FR-014**: System MUST implement credential management with encrypted storage

**Phase 3: Observability & Cost**

- **FR-015**: System MUST implement a callback/hook system that fires on request completion with structured log payloads
- **FR-016**: System MUST support at least 5 logging integrations (starting with the most popular: webhook, Prometheus metrics, OpenTelemetry, Langfuse, Datadog)
- **FR-017**: System MUST implement spend analytics by team, tag, model, and end_user (customer) dimensions
- **FR-018**: System MUST embed a maintained model pricing table (input/output token prices + context window sizes for all supported models) and allow user override via custom_pricing config
- **FR-019**: System MUST implement alerting for budget thresholds (configurable notifications)

**Phase 4: Guardrails & Safety**

- **FR-020**: System MUST implement a guardrail hook system with pre-call and post-call phases
- **FR-021**: System MUST support at least 3 guardrail integrations (PII detection, content moderation, prompt injection detection)
- **FR-022**: System MUST support per-key and per-team guardrail assignment

**Phase 5: Advanced Routing**

- **FR-023**: System MUST implement at least 4 routing strategies (shuffle, latency-based, cost-based, usage-based)
- **FR-024**: System MUST implement tag-based routing
- **FR-025**: System MUST implement context window fallbacks
- **FR-026**: System MUST implement a policy engine for conditional routing rules

**Cross-cutting**

- **FR-027**: System MUST warn at startup about any unsupported config fields from Python TianjiLLM format
- **FR-028**: System MUST return 501 with descriptive messages for endpoints not yet implemented
- **FR-029**: System MUST accept 100% of Python TianjiLLM proxy_config.yaml fields — all fields must be recognized, and any field for a feature not yet implemented must produce a clear startup warning (not an error)

### Key Entities

- **Provider**: An LLM service backend (OpenAI, Anthropic, etc.) with its transformation logic, auth method, and supported parameters
- **Guardrail**: A content safety check that can be applied pre-call (on request) or post-call (on response), assignable to keys/teams
- **Callback**: An integration that receives structured log payloads on request completion, used for observability and cost tracking
- **Organization**: Top-level multi-tenant entity containing teams, with its own budget and model access controls
- **Policy**: A set of routing rules and guardrail assignments that can be applied conditionally based on request attributes
- **Credential**: An encrypted provider credential that can be referenced by name in model configurations

## Scope & Boundaries

### Current State (tianjiLLM)

| Dimension       | Count |
| --------------- | ----- |
| Providers       | 6     |
| API Endpoints   | ~30   |
| Logging         | 0     |
| Guardrails      | 0     |
| Cache backends  | 3     |
| Router strategy | 3     |
| Auth methods    | 2     |

### Target State (Python TianjiLLM parity)

| Dimension       | Count |
| --------------- | ----- |
| Providers       | 119   |
| API Endpoints   | 200+  |
| Logging         | 45+   |
| Guardrails      | 22+   |
| Cache backends  | 9     |
| Router strategy | 6     |
| Auth methods    | SSO + JWT + RBAC + SCIM |

### In Scope

- All functional requirements listed above (FR-001 through FR-029)
- Maintaining config compatibility with Python TianjiLLM's proxy_config.yaml format
- Parity for the most commonly used features (the "80/20 rule" — 20% of features cover 80% of users)

### Out of Scope

- UI/dashboard (Python TianjiLLM has UI endpoints — these are out of scope for the Go proxy)
- All 119 providers (target the top 20 by usage, then add more incrementally)
- All 45+ logging integrations (target the top 5, provide a plugin interface for the rest)
- Python TianjiLLM SDK/client library features (this spec covers proxy server only)
- Assistants API and Vector Stores API (lower priority, can be added later as pass-through)
- Video Generation API, A2A endpoints, Container endpoints (niche features)

## Assumptions

- The Go version does not need to replicate every Python feature — it should prioritize the features that matter most for production deployments
- Each provider gets a dedicated code module (following Python TianjiLLM's pattern), even if OpenAI-compatible; modules inherit from an OpenAI base to maximize code reuse while enabling per-provider overrides
- Pass-through endpoints use a generic router with dedicated per-provider logging/auth handlers, following Python TianjiLLM's architecture
- The callback/hook system design should be plugin-based so new integrations can be added without modifying core code
- Budget and spend tracking accuracy requirements match Python TianjiLLM's current behavior (eventual consistency is acceptable)
- SSO integration follows standard OIDC/OAuth2 protocols — no custom IDP implementations needed

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Proxy supports at least 20 LLM providers and can route requests to any of them using the same config format as Python TianjiLLM
- **SC-002**: All management endpoints (key, team, user, organization) support full CRUD operations and enforce access control
- **SC-003**: At least 5 observability integrations are functional and report accurate usage/cost data within 1% variance of actual provider billing
- **SC-004**: 80% of Python TianjiLLM proxy users can migrate to tianjiLLM without modifying their existing client code
- **SC-005**: Config files from Python TianjiLLM work in tianjiLLM with clear warnings for any unsupported fields
- **SC-006**: At least 3 guardrail types are functional and can block or redact content within the request lifecycle
- **SC-007**: All 4+ routing strategies produce measurably different deployment selection patterns under controlled load
- **SC-008**: Migration from Python to Go results in at least 3x reduction in memory usage and 2x improvement in request throughput for equivalent workloads
