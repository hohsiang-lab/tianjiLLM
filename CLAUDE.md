# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

TianjiLLM — an OpenAI-compatible LLM proxy in Go that translates requests to 6+ LLM providers (OpenAI, Anthropic, Azure, Gemini, Bedrock, and any OpenAI-compatible endpoint). Originally a Go rewrite of [Python LiteLLM](https://github.com/BerriAI/litellm), now an independent project.

## Commands

```bash
make build          # → bin/tianji
make test           # go test -race -cover ./...
make lint           # golangci-lint run
make check          # lint + test + build
make run            # go run ./cmd/tianji --config proxy_config.yaml
make generate       # sqlc generate (DB query codegen)

# Single test
go test ./internal/provider/anthropic/... -run TestIsOAuthToken -v

# Single package
go test ./internal/router/... -v
```

## Architecture

### Request Flow

```
Client → chi router → auth middleware → handler.resolveProvider()
  → provider.TransformRequest() → HTTP to upstream
  → provider.TransformResponse() → OpenAI-format JSON back to client
```

Streaming: same flow but `TransformStreamChunk()` processes SSE events line-by-line.

### Provider System

Every provider implements `provider.Provider` (7 methods: TransformRequest, TransformResponse, TransformStreamChunk, GetSupportedParams, MapParams, GetRequestURL, SetupHeaders). Providers register themselves via `init()` → `provider.Register("name", instance)`.

Model names use `"provider/model"` format (e.g. `"anthropic/claude-sonnet-4-5-20250929"`). `provider.ParseModelName()` splits this; bare names default to `"openai"`.

OpenAI-compatible providers (ollama, vllm, lm_studio) all use the same `openaicompat` provider — differentiated only by `api_base` in config. The `baseURLFactory` pattern creates fresh instances per base URL.

### Router

Handles multi-deployment load balancing with retry + fallback. Pluggable strategy (shuffle/latency/cost). Deployment health tracked via failure count + cooldown + EMA latency (α=0.3). Optional — handler falls back to direct config resolution when Router is nil.

### Key Directories

- `cmd/tianji/` — entry point, wires config → DB → cache → server
- `internal/provider/` — Provider interface + registry + all implementations
- `internal/proxy/handler/` — HTTP handlers (chat, embedding, completion, key/team/user mgmt)
- `internal/proxy/middleware/` — auth, budget, rate limit, parallel request limiter, cache control, response ID security
- `internal/proxy/hook/` — hook interface + factory + enterprise hooks (banned keywords, blocked user, management events)
- `internal/a2a/` — Agent-to-Agent protocol (registry, permissions, completion bridge, JSON-RPC)
- `internal/rag/` — RAG ingest (chunk → embed → store) and query (search → context → complete) pipelines
- `internal/search/` — search provider interface + implementations (Tavily, Firecrawl, Linkup)
- `internal/router/` — load balancer + deployment health + selection strategies
- `internal/model/` — shared types: request, response, errors, embeddings
- `internal/config/` — YAML loader with `$ENV_VAR` resolution
- `internal/cache/` — Cache interface with memory, Redis, and dual (hybrid) impls
- `internal/db/` — pgx pool + sqlc-generated queries
- `test/contract/` — handler tests with mock upstream servers
- `test/integration/` — full server flow tests
- `test/fixtures/` — real provider request/response JSON examples

### Config

YAML with env var interpolation (`"$OPENAI_API_KEY"` or `"${OPENAI_API_KEY}"`). Resolved at load time by `config.Load()`. Optional `providers.json` in same directory registers additional OpenAI-compatible providers at startup.

### Error Model

Sentinel errors in `internal/model/errors.go` (ErrAuthentication, ErrRateLimit, ErrBudgetExceeded, etc.) mapped from HTTP status codes. `TianjiError` wraps these with provider, model, status_code, type, message.

## Conventions

- Providers self-register in `init()` — adding a provider requires zero changes to existing code
- All request/response types live in `internal/model/` — providers import from there, never define their own API types
- Test pattern: `httptest.NewServer()` mocks upstream, `httptest.NewRecorder()` captures responses; `testify` for assertions
- Auth: SHA256 hash comparison for master key, DB lookup for virtual keys

## Active Technologies
- Go 1.22+ (latest stable) + chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth) (001-migration-gap-analysis)
- PostgreSQL (primary), Redis (cache + rate limiting) (001-migration-gap-analysis)
- PostgreSQL (primary, existing), Redis/Redis Cluster (cache, existing go-redis), S3/GCS/Azure Blob (log storage, new) (002-migration-gap-phase2)
- Go 1.22+ (latest stable) + chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth), elimity-com/scim (SCIM 2.0 server), scim2/filter-parser/v2 (SCIM filter parsing), go-redsync/redsync/v4 (distributed lock), oracle/oci-go-sdk/v65 (OCI request signing), posthog/posthog-go (PostHog), cloud.google.com/go/pubsub (GCS Pub/Sub), getlago/lago-go-client (Lago), Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs (Azure Sentinel), cyberark/conjur-api-go (CyberArk Conjur), aws-sdk-go-v2 (S3 cache + cold storage), cloud.google.com/go/storage (GCS cache + cold storage), Azure/azure-sdk-for-go/sdk/storage/azblob (Azure Blob cache) (003-migration-gap-phase3)
- PostgreSQL (primary, existing), Redis/Redis Cluster (cache, existing), S3/GCS/Azure Blob (cold storage archival + cache backends, new) (003-migration-gap-phase3)
- Go 1.22+ (latest stable) + chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth) — existing; NEW: `github.com/coder/websocket` (WebSocket proxy), `github.com/pkoukk/tiktoken-go` (token counting) (004-migration-gap-phase4)
- PostgreSQL (primary, existing), Redis/Redis Cluster (cache + rate limiting, existing) (004-migration-gap-phase4)
- Go 1.22+ (latest stable) + chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), `github.com/modelcontextprotocol/go-sdk` v1.3.0 (MCP — NEW) (005-migration-gap-phase5)
- PostgreSQL (MCP server configs, prompt templates — existing), Redis (cache — existing) (005-migration-gap-phase5)
- Go 1.22+ (latest stable) + all existing deps; NEW tables: audit_log, deleted_verification_token, deleted_team_table, agents_table, daily_agent_spend, skills_table, claude_code_plugin_table, health_check_table, error_logs, daily_organization_spend, daily_end_user_spend (006-migration-gap-phase6)
- PostgreSQL (audit logs, A2A agents, skills, marketplace plugins, health checks, error logs — new tables), Redis (parallel request limiting, dynamic rate limiting — enhanced usage) (006-migration-gap-phase6)

## Recent Changes
- 001-migration-gap-analysis: Added Go 1.22+ (latest stable) + chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth)
- 006-migration-gap-phase6: Added audit logging, soft-delete tables, A2A protocol (agents + JSON-RPC), Skills API, Claude Code Marketplace, OCR/Video/Container/RAG endpoints, parallel request limiter, dynamic rate limiter, enterprise hooks, missing management endpoints
- 007-migration-gap-phase7: Bare path aliases (69+ routes), Azure engine/deployment paths, 5 path/method compat fixes, /utils/supported_openai_params + /utils/token_counter, Budget CRUD completion, Organization member management (new OrganizationMembership table), dynamic_rate_limiter v3 (model-level TPM/RPM + headers), model_max_budget_limiter middleware, generic_api callback framework, GitHub Models provider
