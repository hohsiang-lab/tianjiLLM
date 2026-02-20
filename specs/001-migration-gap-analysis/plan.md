# Implementation Plan: TianjiLLM Python-to-Go Migration

**Branch**: `001-migration-gap-analysis` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-migration-gap-analysis/spec.md`

## Summary

Migrate Python TianjiLLM proxy features to tianjiLLM in 5 phases: (1) expand providers from 6 to 20+ and add Files/Batches/Fine-tuning/Rerank/Pass-through APIs, (2) complete enterprise management with Organization CRUD, full RBAC, SSO/JWT, credentials, (3) add callback/hook system with 5 logging integrations + spend analytics + embedded model pricing, (4) implement guardrail framework with PII/moderation/prompt-injection, (5) add cost-based/usage-based/tag-based routing strategies + policy engine.

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**: chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth)
**Storage**: PostgreSQL (primary), Redis (cache + rate limiting)
**Testing**: `go test` + `testify` for assertions, `httptest` for mock servers
**Target Platform**: Linux server (Docker), macOS for development
**Project Type**: Single Go binary (existing `cmd/tianji/` entry point)
**Performance Goals**: 2x throughput vs Python TianjiLLM, <50ms p95 proxy overhead
**Constraints**: 100% Python proxy_config.yaml compatibility, zero breaking changes to existing Go API
**Scale/Scope**: 20+ providers, ~80 new endpoints, 5 callback integrations, 3 guardrails, 4 routing strategies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | PASS | All design decisions reference Python source code |
| II. Feature Parity | PASS | API contracts, config format, model name format all match Python |
| III. Research Before Build | PASS | Context7 used for pgx, go-redis, prometheus, otel docs. GitHub search used for patterns. |
| IV. Test-Driven Migration | PASS | Plan requires fixtures from Python test data for each provider |
| V. Go Best Practices | PASS | Interface dispatch, context propagation, error wrapping, composition over inheritance |
| VI. No Stale Knowledge | PASS | All library APIs verified via Context7. No unverified claims. |

**Post Phase 1 Re-check**: PASS — data model, interfaces, and contracts all follow Go idioms (interfaces, not class hierarchies). No unnecessary abstractions.

## Project Structure

### Documentation (this feature)

```text
specs/001-migration-gap-analysis/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research findings
├── data-model.md        # Phase 1 entity model
├── quickstart.md        # Phase 1 dev guide
├── contracts/
│   ├── api-endpoints.md # All new HTTP endpoints
│   └── interfaces.md    # Go interface definitions
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/tianji/                          # Entry point (exists)

internal/
├── provider/                         # Provider implementations (exists)
│   ├── provider.go                   # Interface + registry (exists)
│   ├── openai/                       # Base provider (exists)
│   ├── anthropic/                    # (exists)
│   ├── azure/                        # (exists)
│   ├── gemini/                       # (exists)
│   ├── bedrock/                      # (exists)
│   ├── openaicompat/                 # Generic compat (exists)
│   ├── cohere/                       # NEW — Phase 1
│   ├── mistral/                      # NEW — Phase 1
│   ├── together/                     # NEW — Phase 1
│   ├── fireworks/                    # NEW — Phase 1
│   ├── groq/                         # NEW — Phase 1
│   ├── deepseek/                     # NEW — Phase 1
│   ├── replicate/                    # NEW — Phase 1
│   ├── huggingface/                  # NEW — Phase 1
│   ├── databricks/                   # NEW — Phase 1
│   ├── cloudflare/                   # NEW — Phase 1
│   ├── cerebras/                     # NEW — Phase 1
│   ├── perplexity/                   # NEW — Phase 1
│   ├── xai/                          # NEW — Phase 1
│   └── sambanova/                    # NEW — Phase 1
│
├── proxy/
│   ├── handler/                      # HTTP handlers (exists)
│   │   ├── files.go                  # NEW — Phase 1 (Files API)
│   │   ├── batches.go                # NEW — Phase 1 (Batches API)
│   │   ├── finetuning.go             # NEW — Phase 1 (Fine-tuning API)
│   │   ├── rerank.go                 # NEW — Phase 1 (Rerank API)
│   │   ├── organization.go           # NEW — Phase 2
│   │   ├── credentials.go            # NEW — Phase 2
│   │   └── accessgroup.go            # NEW — Phase 2
│   │
│   ├── middleware/                    # Middleware (exists)
│   │   ├── auth.go                   # EXISTS — extend for JWT/RBAC
│   │   ├── budget.go                 # EXISTS
│   │   └── ratelimit.go              # EXISTS
│   │
│   └── passthrough/                  # NEW — Phase 1
│       ├── router.go                 # Generic reverse proxy router
│       └── handlers/                 # Per-provider logging handlers
│           ├── base.go
│           ├── openai.go
│           ├── anthropic.go
│           ├── vertex.go
│           ├── cohere.go
│           └── gemini.go
│
├── callback/                         # NEW — Phase 3
│   ├── callback.go                   # Interface + registry
│   ├── webhook/                      # Generic HTTP webhook
│   ├── prometheus/                   # Prometheus metrics
│   ├── otel/                         # OpenTelemetry traces
│   ├── langfuse/                     # Langfuse integration
│   ├── datadog/                      # Datadog integration
│   └── alerting/                     # Budget alerting (FR-019)
│
├── guardrail/                        # NEW — Phase 4
│   ├── guardrail.go                  # Interface + runner
│   ├── presidio/                     # PII detection
│   ├── moderation/                   # Content moderation (OpenAI)
│   └── promptinjection/              # Prompt injection detection
│
├── auth/                             # NEW — Phase 2
│   ├── encrypt.go                    # NaCl SecretBox encrypt/decrypt (matches Python)
│   ├── jwt.go                        # JWT validation + JWKS
│   ├── rbac.go                       # Role-based access control
│   └── sso.go                        # SSO/OIDC flow
│
├── router/                           # Load balancer (exists)
│   ├── router.go                     # EXISTS
│   ├── deployment.go                 # EXISTS
│   └── strategy/                     # Strategy sub-package (exists)
│       ├── shuffle.go                # EXISTS
│       ├── latency.go                # EXISTS — complete EMA implementation
│       ├── cost.go                   # EXISTS — extend for embedded pricing (Phase 5)
│       ├── usage.go                  # NEW — Phase 5 (TPM/RPM tracking)
│       └── tag.go                    # NEW — Phase 5 (tag-based filtering)
│
├── pricing/                          # NEW — Phase 3
│   ├── pricing.go                    # Loader + lookup
│   └── model_prices.json             # Embedded from Python
│
├── model/                            # Shared types (exists)
├── config/                           # YAML loader (exists)
├── cache/                            # Cache (exists)
└── db/                               # Database (exists)
    ├── migrations/
    │   ├── 001_initial.sql           # EXISTS
    │   ├── 002_management.sql        # EXISTS
    │   └── 003_organization.sql      # NEW — Phase 2
    └── management.go                 # EXISTS — extend

test/
├── contract/                         # Handler tests (exists)
├── integration/                      # Full flow tests (exists)
└── fixtures/                         # Provider request/response JSON (exists)
```

**Structure Decision**: Extends existing Go project layout. New packages (`callback/`, `guardrail/`, `auth/`, `pricing/`, `proxy/passthrough/`) follow the same `internal/` convention. Each new provider gets its own directory under `internal/provider/`. No new top-level directories created.

## Review Corrections (2026-02-16)

Corrections identified during code-verified plan review:

1. **Strategy files already exist**: `strategy/shuffle.go`, `strategy/latency.go`, `strategy/cost.go` all exist in `internal/router/strategy/` sub-package. Latency is complete. Cost is partial (needs embedded pricing). Only `usage.go` and `tag.go` are truly NEW for Phase 5.
2. **Strategy directory structure**: Files are in `internal/router/strategy/` sub-package, not directly under `internal/router/`.
3. **Config callback field**: `TianjiLLMSettings.Callbacks` maps to YAML `success_callback` (singular). Python also has `failure_callback` — Phase 3 needs to add this field to `TianjiLLMSettings`.

## Complexity Tracking

No constitution violations requiring justification. All designs follow established patterns:
- Provider: interface dispatch + registry (existing pattern)
- Callback: single interface, no framework
- Guardrail: extends callback interface (matches Python's design)
- Auth: standard JWT + RBAC, no custom IDP
- Routing: pluggable strategy interface (existing pattern, 3 of 5 strategies already implemented)
