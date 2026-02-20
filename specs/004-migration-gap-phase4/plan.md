# Implementation Plan: Phase 4 — Full Migration Gap Closure

**Branch**: `004-migration-gap-phase4` | **Date**: 2026-02-17 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-migration-gap-phase4/spec.md`

## Summary

Phase 4 closes 70+ gaps across 6 categories between Python TianjiLLM and the Go rewrite. Research reveals that many "missing" features actually have partial implementations — config structs parsed but unused, handler stubs returning 501, interfaces defined but not wired. The plan organizes work into 4 implementation waves:

- **Wave 1 (Quick Wins)**: Wire existing code — A1, A2, A3, A5, B6 — ~125 lines total
- **Wave 2 (Medium)**: Add missing logic to existing infrastructure — A4, F2, F5, F6, F8, B1, B2, B5, D10, D12, D14
- **Wave 3 (Large)**: Build from scratch — A6 (WebSocket), F1 (token counting), B3 (priority queue)
- **Wave 4 (On-demand)**: Guardrails (C1-C22), callbacks (E1-E12), proxy features (D1-D9, D13-D16)

New dependencies: `github.com/coder/websocket` (Realtime API), `github.com/pkoukk/tiktoken-go` (token counting).

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**: chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth) — existing; NEW: `github.com/coder/websocket` (WebSocket proxy), `github.com/pkoukk/tiktoken-go` (token counting)
**Storage**: PostgreSQL (primary, existing), Redis/Redis Cluster (cache + rate limiting, existing)
**Testing**: `go test` + `testify` for assertions, `httptest` for HTTP mocks, contract tests with JSON fixtures
**Target Platform**: Linux server (amd64/arm64), macOS (development)
**Project Type**: Single project (existing Go repository)
**Performance Goals**: Pass-through <2ms added latency; WebSocket relay <1ms per message; cache hit <5ms; token counting <1ms per request
**Constraints**: Zero regression on existing 260+ task test suite; config format 100% compatible with Python TianjiLLM
**Scale/Scope**: 70+ gaps, 4 waves, ~20 new/modified files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Python-First Reference | ✅ PASS | All gaps derived from Python TianjiLLM source analysis; Python locations documented for every gap item |
| II. Feature Parity | ✅ PASS | Spec explicitly tracks Python behavior → Go parity; config format compatibility is FR-008 |
| III. Research Before Build | ✅ PASS | 4 research agents dispatched; Context7 queried for coder/websocket, tiktoken-go; GitHub code search for WebSocket proxy patterns; Python codebase analyzed for all categories |
| IV. Test-Driven Migration | ✅ PASS | FR-007 requires contract tests for every gap; existing test patterns (httptest + testify) reused |
| V. Go Best Practices | ✅ PASS | WebSocket uses context-native library; token counting uses established community library; plugin systems follow existing init() + registry pattern; no unnecessary abstractions |
| VI. No Stale Knowledge | ✅ PASS | coder/websocket verified via Context7 + GitHub (cloudflared, boundary, coder/coder); tiktoken-go verified (884 stars, 2700+ dependents, o200k_base support confirmed); Responses API format verified from OpenAI docs |
| VII. sqlc-First DB Access | ✅ PASS | No new DB queries required for Wave 1-2; any future DB needs (prompt management, policy) already have sqlc queries from Phase 3 |

### Post-Design Re-evaluation (Phase 1 Complete)

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Python-First Reference | ✅ PASS | data-model.md entities map to Python config/router structures; contracts mirror Python endpoint paths |
| II. Feature Parity | ✅ PASS | Pass-through, SSO, fallback, alias all match Python behavior; Responses API Phase 1 is pass-through (identical behavior) |
| III. Research Before Build | ✅ PASS | research.md complete with 8 decisions, all backed by Context7/GitHub verification |
| IV. Test-Driven Migration | ✅ PASS | Contract test patterns defined for each wave; existing test infrastructure reused |
| V. Go Best Practices | ✅ PASS | No new abstractions; wave 1 is pure wiring; wave 2 extends existing patterns; wave 3 uses proven libraries |
| VI. No Stale Knowledge | ✅ PASS | All library choices backed by external verification; no unverified claims |
| VII. sqlc-First DB Access | ✅ PASS | No hand-written SQL; all DB access through existing sqlc-generated queries |

## Project Structure

### Documentation (this feature)

```text
specs/004-migration-gap-phase4/
├── plan.md              # This file
├── research.md          # Phase 0 output — 8 decisions documented
├── data-model.md        # Phase 1 output — config/router/cache data models
├── quickstart.md        # Phase 1 output — implementation quickstart guide
├── contracts/           # Phase 1 output
│   ├── passthrough-api.yaml    # A1 pass-through endpoint contract
│   ├── responses-api.yaml      # A2 responses API contract
│   ├── realtime-api.yaml       # A6 WebSocket realtime contract
│   └── router-strategies.yaml  # B1-B6 router strategy contracts
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/tianji/                                  # Modify: add SSO wire, WebSocket setup
internal/
├── proxy/
│   ├── server.go                             # Modify: mount pass-through routes, WebSocket endpoint
│   ├── handler/
│   │   ├── passthrough.go                    # NEW: dynamic pass-through handler
│   │   ├── responses.go                      # Modify: replace 501 with proxy call
│   │   ├── sso.go                            # Modify: wire SSOHandler (already implemented)
│   │   ├── chat.go                           # Modify: add cache Get/Set, fallback retry
│   │   └── realtime.go                       # NEW: WebSocket relay handler
│   └── middleware/
│       └── ratelimit.go                      # Modify: add CheckTPM
├── router/
│   ├── router.go                             # Modify: alias resolution, general fallback, tag filtering
│   ├── strategy/
│   │   ├── tag.go                            # Modify: add hasAnyTag + match_any mode
│   │   ├── region.go                         # NEW: region-based filtering
│   │   ├── tpm_rpm.go                        # NEW: lowest-TPM/RPM selection (reuse usage.go tracker)
│   │   └── priority.go                       # NEW: priority queue strategy
│   └── fallback.go                           # NEW: general fallback + default_fallbacks
├── auth/
│   └── sso.go                                # Already implemented — no changes
├── cache/                                    # No changes — infrastructure ready
├── callback/
│   ├── slack.go                              # Modify: add alert_types, multi-channel, daily report
│   └── [new callbacks as needed]             # Wave 4
├── guardrail/
│   └── [new guardrails as needed]            # Wave 4
├── token/                                    # NEW: pre-request token counting
│   ├── counter.go                            #   tiktoken-go wrapper
│   └── counter_test.go
└── config/
    └── config.go                             # Modify: add SSO fields, fix ModelGroupAlias type

test/
├── contract/
│   ├── passthrough_test.go                   # NEW
│   ├── responses_test.go                     # NEW
│   ├── sso_test.go                           # NEW
│   ├── fallback_test.go                      # NEW
│   ├── alias_test.go                         # NEW
│   ├── realtime_test.go                      # NEW
│   └── cache_handler_test.go                 # NEW
└── integration/
    └── [existing tests — must pass after each wave]
```

**Structure Decision**: Follows existing `internal/` layout. No new top-level directories. New packages only for genuinely new capabilities (`internal/token/`). All other changes are modifications to existing packages.

## Complexity Tracking

> No constitution violations. All changes follow existing patterns.

| Decision | Why This Way | Simpler Alternative Considered |
|----------|-------------|-------------------------------|
| New `internal/token/` package | Token counting is a cross-cutting concern used by router, budget, rate limiter | Inline in handler — rejected because multiple consumers |
| `coder/websocket` dependency | WebSocket proxy requires context-aware bidirectional relay | stdlib only — rejected because `net/http` has no WebSocket support |
| `tiktoken-go` dependency | Pre-request token counting requires BPE tokenizer | Estimate from character count — rejected because 4x inaccuracy breaks budget enforcement |

## Implementation Waves

### Wave 1: Wire Existing Code (Quick Wins — ~125 lines)

| Task | Gap | Change | Est. Lines |
|------|-----|--------|-----------|
| Mount pass-through routes | A1 | `server.go` — wire existing `passthrough.Router` (already has `httputil.ReverseProxy` + guardrail pre/post-call + usage logging + provider auth) to route table; `passthrough/router.go` Handler() is ready, just needs mounting + config-to-endpoint wiring | ~30 |
| Wire CreateResponse | A2 | `responses.go` — replace 501 with `assistantsProxy()` call | ~5 |
| Wire SSO config | A3 | `config.go` — add SSO fields; `cmd/tianji/main.go` — construct `auth.SSOHandler` from config | ~50 |
| Wire model group alias | A5 | `router.RouterSettings` — add `ModelGroupAlias map[string]ModelGroupAliasItem` (struct with Model + Hidden fields, matching Python's `RouterModelGroupAliasItem`); `router.Route()` — insert alias lookup before deployment selection; `/v1/models` handler — filter hidden aliases | ~25 |
| Wire tag match_any | B6 | `router.Route()` — call `PickWithTags` when `EnableTagFiltering` is set | ~15 |

### Wave 2: Add Logic to Existing Infrastructure (~500 lines)

| Task | Gap | Reuses | New Logic |
|------|-----|--------|-----------|
| General fallback | A4 | `ContextWindowFallback()` pattern | `GeneralFallback()` + handler integration |
| Response caching | F2 | Cache interface + all backends | chat.go Get/Set with content hash key |
| Slack advanced alerts | F5 | Throttled sender | Alert type routing + `AlertToWebhookURL` config |
| Dynamic rate limiter | F6 | RPM Lua script | TPM tracking + proportional throttle |
| Model budget limiter | F8 | Budget middleware | Per-model spend tracking |
| Region routing | B1 | Strategy interface | Region field + filter strategy |
| TPM/RPM routing | B2 | usage.go tracker | Selection strategy using tracker data |
| Per-group retry policy | B5 | Retry loop in Route() | Per-model config lookup |
| Config pass-through | D10 | Pass-through from A1 | User-defined routes from YAML |
| Hook plugin system | D12 | Guardrail hook interface | Generic pre/post-call hook chain |
| Key rotation | D14 | Scheduler + CredentialRefreshJob | Rotation interval + seamless swap |

### Wave 3: Build from Scratch (~800 lines)

| Task | Gap | New Dependency | Complexity |
|------|-----|---------------|------------|
| WebSocket realtime proxy | A6 | `coder/websocket` | Bidirectional relay + auth + connection lifecycle |
| Token counting | F1 | `tiktoken-go` | Counter wrapper + model mapping + handler integration |
| Priority queue | B3 | None (stdlib) | Request queue with weighted scheduling |

### Wave 4: On-demand (Plugin Additions)

Each guardrail/callback follows the established 2-file pattern:
1. Implementation file (implements `Guardrail` or `CustomLogger` interface)
2. Registration in `factory.go` switch statement

Prioritize based on user adoption data. Category C (22 guardrails) and E (12 callbacks) are independent — can be added in any order without architectural changes.
