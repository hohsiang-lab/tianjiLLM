# Research: TianjiLLM Python-to-Go Migration

**Date**: 2026-02-16
**Branch**: `001-migration-gap-analysis`

## R1: Provider Architecture Pattern

**Decision**: Each provider gets a dedicated Go struct inheriting from an OpenAI base via interface embedding. Provider dispatching via registry pattern (existing `provider.Register()` + `init()`).

**Rationale**: Python TianjiLLM uses `OpenAIGPTConfig` as base class. Most providers (Together AI, Fireworks, Groq, etc.) inherit from it and override 1-2 methods: `get_supported_openai_params()` and `map_openai_params()`. Go equivalent: interface satisfaction + struct embedding of a base `OpenAIProvider`.

**Alternatives considered**:
- JSON-config-only (`openaicompat`) — rejected per clarification; Python TianjiLLM gives every provider its own module
- Code generation from Python source — too fragile, Python code changes frequently

**Source**: Python `tianji/llms/together_ai/chat.py` inherits `OpenAIGPTConfig`, overrides `get_supported_openai_params()` only.

## R2: Callback/Hook System Design

**Decision**: Single `Callback` interface with lifecycle methods (PreCall, PostCall, OnSuccess, OnFailure, StreamEvent). Guardrails are specialized callbacks with `ShouldRun()` + `Apply()` methods — not a separate system.

**Rationale**: Python's `CustomGuardrail` extends `CustomLogger`. This is elegant — guardrails reuse the existing callback infrastructure. Go equivalent: `Guardrail` interface embeds `Callback` interface, adds `ShouldRun(data, eventType) bool` and `Apply(inputs, requestData, inputType)`.

**Key lifecycle methods** (from Python `CustomLogger`):
1. `async_pre_call_hook` — before sending to provider
2. `async_post_call_success_hook` — after successful response
3. `async_post_call_failure_hook` — after error
4. `async_moderation_hook` — runs in parallel during call
5. `async_log_stream_event` — for each SSE chunk

**Alternatives considered**:
- Separate guardrail pipeline — rejected; Python proves callback reuse works
- Event bus pattern — over-engineered for this use case

**Source**: Python `tianji/integrations/custom_guardrail.py`, `tianji/integrations/custom_logger.py`

## R3: Pass-through Endpoint Architecture

**Decision**: Generic reverse proxy router with `{provider}/{path...}` wildcard route. Per-provider logging handlers for cost tracking. Credentials resolved from config → env var fallback.

**Rationale**: Python uses FastAPI's `{endpoint:path}` to capture arbitrary paths, then constructs target URL from provider's `api_base` + path. Each major provider (OpenAI, Anthropic, Vertex, Cohere, Gemini) has a `PassthroughLoggingHandler` that extracts token usage for spend tracking.

**Go equivalent**: chi's `/{provider}/{path:*}` wildcard. `httputil.ReverseProxy` with per-provider `Director` functions.

**Alternatives considered**:
- Pre-registered routes per provider — too rigid, can't support arbitrary provider APIs
- No logging handlers — would lose cost tracking for pass-through traffic

**Source**: Python `tianji/proxy/pass_through_endpoints/passthrough_endpoint_router.py`

## R4: RBAC and JWT Authentication

**Decision**: JWT validation with JWKS support, 4+ roles (proxy_admin, team, internal_user, end_user), role-based route + model access control. Role mapping from external IDP claims.

**Rationale**: Python's auth is 3-layer: Role → Routes → Models. JWT claims are extracted via configurable field paths (supporting nested fields like `custom.team.id`). Roles can be mapped from external IDP roles (e.g., `sso_admin` → `proxy_admin`).

**Go libraries needed**:
- `golang-jwt/jwt/v5` for JWT parsing — verified via Context7
- JWKS: `MicahParks/keyfunc/v3` for JWKS endpoint caching
- Standard `crypto` for key operations

**Alternatives considered**:
- Casbin for RBAC — over-engineered; Python's simple role → routes/models mapping is sufficient
- OAuth2 proxy sidecar — out of scope; proxy handles auth directly

**Source**: Python `tianji/proxy/auth/auth_checks.py`, `tianji/proxy/auth/handle_jwt.py`

## R5: Observability Stack

**Decision**: Plugin-based callback system. Priority integrations: (1) Webhook/Generic HTTP, (2) Prometheus, (3) OpenTelemetry, (4) Langfuse, (5) Datadog.

**Libraries verified via Context7**:
- `prometheus/client_golang` — v1.x, custom registry pattern, HistogramVec for latency, CounterVec for request counts
- `open-telemetry/opentelemetry-go` — SDK for traces, W3C propagation, span creation/annotation
- `redis/go-redis/v9` — pipeline + Lua scripts for rate limiting (already in use)
- `jackc/pgx/v5` — connection pool with health checks (already in use)

**Prometheus metrics design** (following Python TianjiLLM's pattern):
- `tianji_requests_total{model, provider, status, key_alias}` — Counter
- `tianji_request_duration_seconds{model, provider}` — Histogram
- `tianji_tokens_total{model, provider, type}` — Counter (input/output)
- `tianji_spend_total{model, provider, team, key_alias}` — Counter
- `tianji_deployment_health{deployment, status}` — Gauge

**Source**: Context7 docs for prometheus/client_golang, open-telemetry/opentelemetry-go

## R6: Model Pricing Data

**Decision**: Embed Python TianjiLLM's `model_prices_and_context_window.json` directly (symlink or copy). ~37K lines, updated with Python releases.

**Rationale**: Python maintains this as the canonical pricing source. Re-implementing would cause drift. The JSON file is pure data — no code dependency.

**Update strategy**: Periodic sync from Python repo (can be automated via CI).

**Source**: Python `tianji/model_prices_and_context_window_backup.json` (36,942 lines)

## R7: Config Compatibility (FR-029)

**Decision**: Parse all Python TianjiLLM proxy_config.yaml fields. Unknown/unsupported fields produce startup warnings, not errors. Use `yaml.v3` with `KnownFields: false` to capture unknown fields.

**Rationale**: FR-029 requires 100% field recognition. Go's `yaml.v3` can decode into a struct with known fields + a `map[string]interface{}` for overflow. Compare overflow keys against a "known but unimplemented" list to generate targeted warnings.

**Source**: Existing `internal/config/config.go`

## R8: Routing Strategies

**Decision**: Implement 4 strategies matching Python: shuffle (existing, complete), latency-based (existing, complete EMA implementation), cost-based (existing partial — picks by config `InputCost`, needs extension for embedded pricing table), usage-based (NEW — TPM/RPM tracking). Add tag-based filtering as 5th strategy.

**Python patterns**:
- `simple_shuffle` — random weighted selection
- `LowestLatencyLoggingHandler` — EMA α=0.3, tracks per-deployment latency
- `LowestCostLoggingHandler` — uses model_prices to score deployments
- `LowestTPMLoggingHandler` — tracks current TPM/RPM per deployment, selects lowest utilization

**Go existing implementations** (in `internal/router/strategy/`):
- `shuffle.go` — complete, random selection
- `latency.go` — complete, `LowestLatency.Pick()` selects lowest EMA latency, prefers untested deployments
- `cost.go` — partial, `LowestCost.Pick()` selects lowest `ModelInfo.InputCost` from config; needs extension to query embedded pricing table when config cost is absent

**Go remaining work**: Usage strategy needs per-deployment TPM/RPM counters (sliding window). Tag strategy needs deployment tag filtering before delegating to another strategy.

**Source**: Python `tianji/router_strategy/`

## R9: Credential Encryption

**Decision**: Use NaCl SecretBox (XSalsa20-Poly1305) with SHA256(master_key) as 32-byte symmetric key. Per-value encryption with base64url-encoded output. Key derivation from `TIANJI_SALT_KEY` env var, falling back to master_key.

**Rationale**: Python TianjiLLM uses `nacl.secret.SecretBox` with `hashlib.sha256(signing_key).digest()` as the key. Each credential value is encrypted individually (not the whole JSON blob), enabling partial updates without full decryption. Output is base64url-encoded for safe DB storage.

**Go equivalent**: `golang.org/x/crypto/nacl/secretbox` — same XSalsa20-Poly1305 algorithm. SHA256 from `crypto/sha256`. Base64url from `encoding/base64`.

**Alternatives considered**:
- AES-256-GCM — viable but would break compatibility with existing Python-encrypted values
- age/SOPS — overkill for per-field encryption

**Source**: Python `tianji/proxy/common_utils/encrypt_decrypt_utils.py` — `encrypt_value()` uses `nacl.secret.SecretBox`

## R10: Budget Alerting

**Decision**: Implement budget alerting as a specialized callback (following Python's `SlackAlerting` pattern). Alert types: budget_alerts, llm_exceptions, llm_requests_hanging, daily_reports. Configurable webhook URL + threshold.

**Rationale**: Python's `SlackAlerting` extends `CustomBatchLogger` (itself a CustomLogger). It fires on budget threshold crossing (>80% spend), slow/hanging requests, and daily/weekly spend reports. Config via `general_settings.alerting` + `general_settings.alerting_threshold`.

**Go equivalent**: Implement as a Callback integration in `internal/callback/alerting/`. Wire into budget middleware to fire when spend crosses configurable thresholds. Initial webhook-only (Slack compatible); extend to email later.

**Source**: Python `tianji/integrations/SlackAlerting/slack_alerting.py`, `tianji/types/router.py:AlertingConfig`
