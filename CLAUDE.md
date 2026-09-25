# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

TianjiLLM — an OpenAI-compatible LLM proxy in Go that translates requests to 29+ LLM providers (OpenAI, Anthropic, Azure, Gemini, Bedrock, and any OpenAI-compatible endpoint). Includes a web UI built with templ + HTMX + Tailwind CSS v4.

## Commands

```bash
make build          # templ generate + tailwind build + go build → bin/tianji
make test           # go test -race -cover ./...
make lint           # golangci-lint run
make check          # lint + test + build
make run            # go run ./cmd/tianji --config proxy_config.yaml
make generate       # sqlc generate (DB query codegen)
make ui             # templ generate + tailwind build (no Go compile)
make dev            # wgo hot-reload: watches .go/.templ/.css, rebuilds everything
make ui-dev         # templ watch + tailwind watch in parallel
make e2e            # Playwright E2E tests against containerized PostgreSQL
make e2e-headed     # same but with visible browser
make tools          # install templ + templui CLIs + download tailwindcss binary

# Single test
go test ./internal/provider/anthropic/... -run TestIsOAuthToken -v

# Single package
go test ./internal/router/... -v

# E2E requires PostgreSQL at postgres://tianji:tianji@localhost:5433/tianji_e2e
```

Pre-commit hooks (lefthook): `gofmt` + `golangci-lint` run in parallel on every commit.

CI pipeline (`.github/workflows/ci.yml`): lint → test (50% coverage gate) → e2e (Playwright) → build → docker (push-only). E2E job runs after test passes, uses its own PostgreSQL service container (`tianji_e2e` DB).

## Architecture

### Request Flow

```
Client → chi router → auth middleware → handler.resolveProvider()
  → provider.TransformRequest() → HTTP to upstream
  → provider.TransformResponse() → OpenAI-format JSON back to client
```

Streaming: same flow but `TransformStreamChunk()` processes SSE events line-by-line.

### Native Format Passthrough

For Anthropic (`/v1/messages`) and Gemini (`/v1beta/models/*`), a separate passthrough path forwards requests in the provider's native format without OpenAI translation:

```
Client → chi router → auth middleware → nativeProxy()
  → httputil.ReverseProxy Director (replace auth, merge beta headers, preserve query params)
  → upstream provider API → response forwarded as-is to client
```

OAuth tokens (`sk-ant-oat*`): sets `Authorization: Bearer`, merges `anthropic-beta` via `MergeBetaHeaders()` (dedup), sets `anthropic-dangerous-direct-browser-access: true`. All client headers forwarded except `Authorization`/`Host`/`Content-Length`.

### Provider System

Every provider implements `provider.Provider` (7 methods: TransformRequest, TransformResponse, TransformStreamChunk, GetSupportedParams, MapParams, GetRequestURL, SetupHeaders). Providers self-register via `init()` → `provider.Register("name", instance)` — adding a provider requires zero changes to existing code.

Model names use `"provider/model"` format (e.g. `"anthropic/claude-sonnet-4-5-20250929"`). `provider.ParseModelName()` splits this; bare names default to `"openai"`.

OpenAI-compatible providers (vllm, lm_studio, etc.) reuse the `openaicompat` provider — differentiated only by `api_base` in config. Ollama has a first-class adapter for embedding-specific request handling. The `baseURLFactory` pattern creates fresh instances per base URL. Additional compatible providers can be registered via `configs/providers.json`.

### Model Resolution

`resolveProvider()` in `chat.go` is the main entry point:

```
Router != nil → Router.Route(modelName)
                  ├─ exact deployment lookup
                  ├─ wildcard fallback (wildcardMatch)
                  └─ failure → GeneralFallback chain
Router == nil → resolveProviderFromConfig(modelName)
                  └─ findModelConfig(modelName) → (config, resolvedModel)
```

`findModelConfig` returns `(*config.ModelConfig, string)` — the second value is the fully-resolved `tianji_params.model` with wildcards replaced. Exact match always wins over wildcards.

### Wildcard Model Names

`internal/wildcard/` provides LiteLLM-compatible pattern matching. Config `model_name: "claude-*"` with `model: "anthropic/claude-*"` matches request `claude-sonnet-4-5` → routes to `anthropic/claude-sonnet-4-5`.

- `wildcard.Match(pattern, name)` → captured segments (one per `*`) or nil
- `wildcard.ResolveModel(template, captured)` → replaces `*` sequentially
- `wildcard.Specificity(pattern)` → `(length, wildcardCount)` for sorting

Both `findModelConfig` and `Router.wildcardMatch` use the same flow: collect matching patterns, sort by specificity (longest wins, fewer `*` breaks ties), resolve model template with captured segments. Router clones deployments with the resolved `ModelName`.

### Router

Multi-deployment load balancing with retry + fallback. Pluggable strategy (shuffle/latency/cost). Deployment health tracked via failure count + cooldown + EMA latency (α=0.3). Optional — handler falls back to direct config resolution when Router is nil.

Fallback chain (`fallback.go`): `GeneralFallback` tries model-specific fallbacks first (`settings.Fallbacks[model]`), then `DefaultFallbacks`. `ContextWindowFallback` and `ContentPolicyFallback` handle specific error types.

### UI System

Server-rendered UI using templ (type-safe Go HTML templates) + HTMX 2.x (server-driven interactions) + templUI v1.5.0 (shadcn-style components) + Tailwind CSS v4.

- `internal/ui/components/` — reusable templ components (button, card, table, dropdown, tabs, etc.)
- `internal/ui/pages/` — page templates (dashboard, keys, models, spend, login)
- `internal/ui/handler.go` — UI route handlers and session management
- `internal/ui/input.css` — Tailwind config with `@theme inline` design tokens
- `internal/ui/assets/` — compiled CSS + JS (templui runtime)

`make ui` regenerates templ Go files and compiles Tailwind. `make dev` watches all file types and hot-reloads.

### Config

YAML (`proxy_config.yaml`) with env var interpolation (`"$OPENAI_API_KEY"` or `"${OPENAI_API_KEY}"`). Auto-loads `.env` from the config file's directory via godotenv (never overwrites existing env vars). Two-phase secret resolution: env vars first, then an optional caller-provided `SecretResolver` for `os.environ/` paths.

### Database

PostgreSQL via pgx/v5. Queries generated by sqlc — schema in `internal/db/schema/` (18 progressive migrations), queries in `internal/db/queries/`. Run `make generate` after editing `.sql` files. Config: `sqlc.yaml` (emits JSON tags + null pointers).

### Key Directories

- `cmd/tianji/` — entry point, wires config → DB → cache → server
- `internal/provider/` — Provider interface + registry + all implementations
- `internal/proxy/handler/` — HTTP handlers (~100 files, one per operation — chat, embedding, key mgmt, native format passthrough, etc.)
- `internal/proxy/middleware/` — auth, budget, rate limit, parallel request limiter, cache control
- `internal/proxy/hook/` — hook interface + factory + enterprise hooks (banned keywords, blocked user)
- `internal/model/` — shared types: request, response, errors, embeddings
- `internal/config/` — YAML loader with env var + secret resolution
- `internal/router/` — load balancer + deployment health + selection strategies + fallback chain
- `internal/wildcard/` — wildcard pattern matching for model names (`*` → regex capture groups)
- `internal/cache/` — Cache interface with memory, Redis, and dual (hybrid) impls
- `internal/db/` — pgx pool + sqlc-generated queries
- `internal/ui/` — templ components, pages, assets, UI handlers
- `internal/a2a/` — Agent-to-Agent protocol (JSON-RPC)
- `internal/auth/` — JWT validation, RBAC, token helpers
- `internal/callback/` — spend logging, rate limit header parsing, Discord alerts
- `internal/guardrail/` — content safety filters (pre-call / post-call hooks)
- `internal/policy/` — policy engine for dynamic guardrail/model routing rules
- `internal/pricing/` — model pricing lookup and cost calculation
- `internal/spend/` — spend tracking and analytics
- `internal/token/` — token counting helpers
- `internal/scheduler/` — scheduled background tasks
- `internal/mcp/` — Model Context Protocol server integration
- `internal/proxy/passthrough/` — provider-native reverse proxy (Anthropic, Gemini, Vertex AI, Bedrock)
- `test/contract/` — handler tests with mock upstream servers
- `test/integration/` — full server flow tests
- `test/e2e/` — Playwright browser tests (build tag: `e2e`)
- `test/fixtures/` — real provider request/response JSON examples

### OAuth Token Throttling (Native Proxy)

For Anthropic native proxy, multiple OAuth tokens are load-balanced with utilization-aware throttling:

```
nativeProxy() → resolveAllNativeUpstreams() → selectUpstreamWithThrottle()
  ├─ status gate: skip rate_limited/overage tokens
  ├─ 5h_gate: skip tokens with 5h utilization >= 80% (configurable)
  ├─ 7d_gate: skip tokens with 7d utilization >= 90% (hardcoded)
  └─ strategy: round-robin (default) or lowest-utilization (quadratic composite score)
```

Utilization data comes from Anthropic's response headers (`anthropic-ratelimit-unified-*`), parsed by `ParseAnthropicOAuthRateLimitHeaders()` and cached in `InMemoryRateLimitStore`. Store flushes dirty entries to DB every 30s (`RateLimitFlusher`), prunes expired entries every 1min (`PruneExpired`).

Key invariant: `Get()`/`GetAll()` call `NormalizeExpiredWindows()` to zero out stale utilization when Anthropic's reset time has passed. Without this, skipped tokens never receive requests → utilization never updates → dead-loop 429.

If all tokens are throttled, returns `allTokensThrottledError` → HTTP 429 with `Retry-After` header.

### Error Model

Sentinel errors in `internal/model/errors.go` (ErrAuthentication, ErrRateLimit, ErrBudgetExceeded, etc.) mapped from HTTP status codes. `TianjiError` wraps these with provider, model, status_code, type, message.

## Conventions

- All request/response types live in `internal/model/` — providers import from there, never define their own API types
- Test pattern: `httptest.NewServer()` mocks upstream, `httptest.NewRecorder()` captures responses; `testify` for assertions
- Auth: SHA256 hash comparison for master key, DB lookup for virtual keys. Virtual key prefix is `sk-ant-oat01-tianji-` (20 chars) followed by 60 random hex chars
- E2E tests use build tag `e2e` and expect a containerized PostgreSQL instance
- E2E test IDs: `generateTestKey()[20:32]` extracts 12 random hex chars after the fixed prefix. Never slice `[3:15]` — that's the fixed prefix, not random

## Tech Stack

Go 1.27 (current module/CI/Docker toolchain), chi/v5 (router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), templ + templUI + Tailwind CSS v4 (UI), Playwright (E2E), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (auth), godotenv (.env loading), coder/websocket (WebSocket proxy), tiktoken-go (token counting), MCP go-sdk v1.3.0, sqlc (query codegen).

## Active Technologies
- Go 1.27 + chi/v5 (router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), testify (testing) (204-enforce-key-limits)
- PostgreSQL (key/team/org data), Redis (rate counters, spend cache) (204-enforce-key-limits)
- Go 1.27 + chi/v5 (router), pgx/v5 (PostgreSQL), sqlc (query codegen), templ + HTMX (UI) (001-upstream-token-spendlogs)
- PostgreSQL (SpendLogs, VerificationToken), Redis (rate limit counters) (001-upstream-token-spendlogs)
- Go 1.27 + No new dependencies. Changes are internal to existing packages: `callback`, `proxy/handler`, `ui`. (491-composite-score-v2)
- No schema changes. `Unified7dSonnetUtilization` is already stored in `AnthropicOAuthRateLimitState` and persisted to DB via `RateLimitFlusher`. (491-composite-score-v2)

## Recent Changes
- 204-enforce-key-limits: Added Go 1.26 + chi/v5 (router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), testify (testing)
