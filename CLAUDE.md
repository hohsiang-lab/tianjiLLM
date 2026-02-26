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
make tools          # install templ + templui CLIs

# Single test
go test ./internal/provider/anthropic/... -run TestIsOAuthToken -v

# Single package
go test ./internal/router/... -v

# E2E requires PostgreSQL at postgres://tianji:tianji@localhost:5433/tianji_e2e
```

Pre-commit hooks (lefthook): `gofmt` + `golangci-lint` run in parallel on every commit.

## Architecture

### Request Flow

```
Client → chi router → auth middleware → handler.resolveProvider()
  → provider.TransformRequest() → HTTP to upstream
  → provider.TransformResponse() → OpenAI-format JSON back to client
```

Streaming: same flow but `TransformStreamChunk()` processes SSE events line-by-line.

### Provider System

Every provider implements `provider.Provider` (7 methods: TransformRequest, TransformResponse, TransformStreamChunk, GetSupportedParams, MapParams, GetRequestURL, SetupHeaders). Providers self-register via `init()` → `provider.Register("name", instance)` — adding a provider requires zero changes to existing code.

Model names use `"provider/model"` format (e.g. `"anthropic/claude-sonnet-4-5-20250929"`). `provider.ParseModelName()` splits this; bare names default to `"openai"`.

OpenAI-compatible providers (ollama, vllm, lm_studio, etc.) reuse the `openaicompat` provider — differentiated only by `api_base` in config. The `baseURLFactory` pattern creates fresh instances per base URL. Additional compatible providers can be registered via `configs/providers.json`.

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

YAML (`proxy_config.yaml`) with env var interpolation (`"$OPENAI_API_KEY"` or `"${OPENAI_API_KEY}"`). Auto-loads `.env` from the config file's directory via godotenv (never overwrites existing env vars). Two-phase secret resolution: env vars first, then optional `SecretResolver` for `os.environ/` paths (AWS Secrets Manager, Vault, Azure Key Vault, GCP Secret Manager).

### Database

PostgreSQL via pgx/v5. Queries generated by sqlc — schema in `internal/db/schema/` (10 progressive migrations), queries in `internal/db/queries/`. Run `make generate` after editing `.sql` files. Config: `sqlc.yaml` (emits JSON tags + null pointers).

### Key Directories

- `cmd/tianji/` — entry point, wires config → DB → cache → server
- `internal/provider/` — Provider interface + registry + all implementations
- `internal/proxy/handler/` — HTTP handlers (64 files, one per operation — chat, embedding, key mgmt, etc.)
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
- `internal/rag/` — RAG ingest + query pipelines
- `internal/search/` — search provider interface (Tavily, Firecrawl, Linkup)
- `test/contract/` — handler tests with mock upstream servers
- `test/integration/` — full server flow tests
- `test/e2e/` — Playwright browser tests (build tag: `e2e`)
- `test/fixtures/` — real provider request/response JSON examples

### Error Model

Sentinel errors in `internal/model/errors.go` (ErrAuthentication, ErrRateLimit, ErrBudgetExceeded, etc.) mapped from HTTP status codes. `TianjiError` wraps these with provider, model, status_code, type, message.

## Conventions

- All request/response types live in `internal/model/` — providers import from there, never define their own API types
- Test pattern: `httptest.NewServer()` mocks upstream, `httptest.NewRecorder()` captures responses; `testify` for assertions
- Auth: SHA256 hash comparison for master key, DB lookup for virtual keys
- E2E tests use build tag `e2e` and expect a containerized PostgreSQL instance

## Tech Stack

Go 1.24.4, chi/v5 (router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), templ + templUI + Tailwind CSS v4 (UI), Playwright (E2E), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (auth), godotenv (.env loading), coder/websocket (WebSocket proxy), tiktoken-go (token counting), MCP go-sdk v1.3.0, sqlc (query codegen).

## Active Technologies
- Go 1.24.4 + chi/v5（路由）、templ（模板）、HTMX 2.x（交互）、templUI v1.5.0（UI 組件）、Tailwind CSS v4 (009-request-logs)
- PostgreSQL（SpendLogs + ErrorLogs 表，via pgx/v5） (009-request-logs)
- Go 1.24.4 + chi/v5 (router), templ (templates), HTMX 2.x (interactions), templUI v1.5.0 (components), Tailwind CSS v4 (styling), Chart.js 4.x (charts) (010-spend-usage)
- PostgreSQL via pgx/v5, queries via sqlc (010-spend-usage)
- Go 1.26 (as per `go.mod`) + `github.com/jackc/pgx/v5` (already in go.mod) + stdlib `embed`, `io/fs` (002-auto-db-migration)
- PostgreSQL — new `schema_migrations` table (created at runtime) (002-auto-db-migration)
- Go 1.24.4 + empl (type-safe HTML templates), HTMX 2.x (server-driven interactions), templUI v1.5.0 (shadcn-style components), Tailwind CSS v4 (001-models-multiselect)
- PostgreSQL via pgx/v5 + sqlc (no schema changes needed; `models` column already `[]string`) (001-models-multiselect)
- Go 1.26 (module `github.com/praxisllmlab/tianjiLLM`) + `a-h/templ` (server-side HTML components), HTMX (partial page updates), templUI component library (existing — `internal/ui/components/`) (001-models-multiselect)
- PostgreSQL via sqlc — `VerificationToken.models text[]` column already exists; **no schema change** (001-models-multiselect)
- Go 1.24.4 + chi/v5 (router), templ (templates), HTMX 2.x (partials), templUI v1.5.0 (components), Tailwind CSS v4 (001-team-org-admin-ui)
- PostgreSQL via pgx/v5 + sqlc codegen. Tables: `TeamTable`, `OrganizationTable`, `OrganizationMembership`. No new migrations. (001-team-org-admin-ui)

## Recent Changes
- 009-request-logs: Added Go 1.24.4 + chi/v5（路由）、templ（模板）、HTMX 2.x（交互）、templUI v1.5.0（UI 組件）、Tailwind CSS v4
