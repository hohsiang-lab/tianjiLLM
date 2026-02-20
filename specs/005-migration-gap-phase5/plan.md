# Implementation Plan: Phase 5 Migration Gap Closure

**Branch**: `005-migration-gap-phase5` | **Date**: 2026-02-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-migration-gap-phase5/spec.md`

## Summary

Phase 5 closes the remaining high-value gaps in the Python TianjiLLM → Go rewrite (112 tasks). The work spans 8 independent user stories: MCP Server (official Go SDK, `mcp.NewServer()` + SSE/HTTP transport mounted on chi), 6 search providers (direct HTTP clients with shared `SearchProvider` interface), image variations (5-line pass-through handler), prompt template resolution in chat flow, ~20 new providers (self-register pattern), discovery endpoints (model group capability aggregation), 34 on-demand plugins (guardrails + callbacks), and AutoRouter (embedding-based cosine similarity routing).

Key technical decision: Use `github.com/modelcontextprotocol/go-sdk` v1.3.0 as the only new external dependency. All other features use existing patterns and `net/http` + `encoding/json`.

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**: chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), `github.com/modelcontextprotocol/go-sdk` v1.3.0 (MCP — NEW)
**Storage**: PostgreSQL (MCP server configs, prompt templates — existing), Redis (cache — existing)
**Testing**: `go test` + `testify`, `httptest.NewServer()` for mocks, `httptest.NewRecorder()` for captures
**Target Platform**: Linux server (same as existing)
**Project Type**: Single Go project (existing `internal/` layout)
**Performance Goals**: MCP tool call overhead < 50ms; search provider latency dominated by upstream; discovery endpoint < 1s for 100 model groups
**Constraints**: Zero regression on existing 390+ completed tasks; zero changes to existing code for new providers/plugins
**Scale/Scope**: 112 tasks across 8 user stories

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | PASS | All 8 features researched against Python codebase (R2-R5 in research.md) |
| II. Feature Parity | PASS | API contracts match Python (search response format, MCP protocol, discovery schema) |
| III. Research Before Build | PASS | Context7 for MCP Go SDK, GitHub search for Go search clients, Python codebase analysis for all features. Documented in research.md R1-R8 |
| IV. Test-Driven Migration | PASS | Each feature includes contract tests. Fixtures from Python test data where available |
| V. Go Best Practices | PASS | MCP SDK is `http.Handler` (chi-compatible). Providers use `init()` registration. Interfaces for polymorphism. Channels for streaming |
| VI. No Stale Knowledge | PASS | MCP Go SDK v1.3.0 verified via Context7. Search API patterns verified via GitHub code search. All decisions backed by external sources |
| VII. sqlc-First DB Access | PASS | MCP server CRUD uses existing sqlc patterns. Prompt templates already in sqlc. No new hand-written SQL |

### Post-Design Re-check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | PASS | Data model matches Python types. Contracts match Python response schemas |
| II. Feature Parity | PASS | MCP config format matches. Search tool YAML matches. Discovery response matches |
| V. Go Best Practices | PASS | `SearchProvider` interface for polymorphism. `mcp.AddTool` generic for type safety. Factory pattern for plugins |
| VII. sqlc-First DB Access | PASS | MCP server management endpoints will use sqlc for DB queries |

## Project Structure

### Documentation (this feature)

```text
specs/005-migration-gap-phase5/
├── plan.md              # This file
├── research.md          # Phase 0: 8 research decisions
├── data-model.md        # Phase 1: Entity definitions
├── quickstart.md        # Phase 1: Usage examples
├── contracts/           # Phase 1: API contracts
│   ├── mcp-server.yaml
│   ├── search-providers.yaml
│   ├── discovery.yaml
│   ├── image-variations.yaml
│   └── auto-router.yaml
├── checklists/
│   └── requirements.md  # Quality gate checklist
└── tasks.md             # Phase 2 (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── mcp/                           # NEW — MCP server package
│   ├── server.go                  # MCP server setup, tool listing, tool calling
│   ├── manager.go                 # MCPServerManager — upstream server registry
│   ├── transport.go               # SSE + Streamable HTTP handler wiring
│   ├── rest.go                    # REST endpoints (/mcp-rest/tools/list, /mcp-rest/tools/call)
│   ├── config.go                  # MCPServerConfig types
│   └── server_test.go
├── search/                        # NEW — Search provider package
│   ├── provider.go                # SearchProvider interface + SearchResult/SearchResponse types
│   ├── registry.go                # Provider registry (map[string]SearchProvider)
│   ├── brave.go                   # Brave Search implementation
│   ├── tavily.go                  # Tavily implementation
│   ├── searxng.go                 # SearXNG implementation
│   ├── exa.go                     # Exa AI implementation
│   ├── google_pse.go              # Google PSE implementation
│   ├── dataforseo.go              # DataForSEO implementation
│   └── *_test.go                  # Per-provider tests
├── provider/                      # EXISTING — add ~20 new provider packages
│   ├── elevenlabs/                # NEW
│   │   └── elevenlabs.go
│   ├── deepgram/                  # NEW
│   │   └── deepgram.go
│   ├── stability/                 # NEW
│   │   └── stability.go
│   ├── baseten/                   # NEW (OpenAI-compat, ~15 lines)
│   │   └── baseten.go
│   ├── jina/                      # NEW (OpenAI-compat)
│   │   └── jina.go
│   ├── voyage/                    # NEW (OpenAI-compat)
│   │   └── voyage.go
│   └── ... (15+ more)
├── proxy/
│   ├── handler/
│   │   ├── native_format.go       # MODIFY — add ImageVariation handler
│   │   ├── search.go              # NEW — /v1/search/{tool_name} handler
│   │   ├── discovery.go           # NEW — /model_group/info, /public/providers, /public/tianji_model_cost_map
│   │   ├── mcp_mgmt.go            # NEW — /v1/mcp/server CRUD handlers
│   │   └── chat.go                # MODIFY — add prompt template resolution
│   └── server.go                  # MODIFY — add routes for new endpoints
├── router/
│   └── strategy/
│       └── auto/                  # NEW — AutoRouter package
│           ├── auto.go            # Embedding-based semantic router
│           ├── encoder.go         # TianjiLLM embedding encoder
│           ├── cosine.go          # Cosine similarity computation
│           └── auto_test.go
├── guardrail/                     # EXISTING — add 22 new guardrail files + factory cases
│   ├── aim.go                     # NEW (T067)
│   ├── aporia.go                  # NEW (T068)
│   ├── ... (20 more)
│   └── factory.go                 # MODIFY — add 22 case statements
├── callback/                      # EXISTING — add 12 new callback files + factory cases
│   ├── arize_full.go              # NEW (T089)
│   ├── agentops.go                # NEW (T090)
│   ├── ... (10 more)
│   └── factory.go                 # MODIFY — add 12 case statements
├── config/
│   └── config.go                  # MODIFY — add MCPServerConfig, SearchToolConfig, AutoRouterConfig
└── model/
    └── request.go                 # MODIFY — add PromptName, PromptVariables, PromptVersion fields

cmd/tianji/
└── main.go                        # MODIFY — add _ imports for new provider packages

test/
├── contract/
│   ├── mcp_test.go                # NEW
│   ├── search_test.go             # NEW
│   └── discovery_test.go          # NEW
└── fixtures/
    ├── mcp/                       # NEW — MCP request/response fixtures
    └── search/                    # NEW — Search provider fixtures
```

**Structure Decision**: Extends existing `internal/` layout. Two new top-level packages (`internal/mcp/`, `internal/search/`). AutoRouter lives under existing `internal/router/strategy/`. All providers, guardrails, and callbacks follow established package conventions.

## Complexity Tracking

No constitution violations. All features follow established patterns:
- MCP Server uses official SDK (`http.Handler`) — no custom protocol implementation
- Search providers follow the same `interface + registry + per-provider file` pattern as LLM providers
- All ~20 new providers use proven `init()` + `provider.Register()` pattern
- All 34 plugins use proven `factory.go` switch-case pattern
- AutoRouter is a simple `pre-routing hook` — cosine similarity in 10 lines of Go, no ML libraries
