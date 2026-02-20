# Implementation Plan: Phase 3 — Enterprise Features & Full Parity

**Branch**: `003-migration-gap-phase3` | **Date**: 2026-02-17 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-migration-gap-phase3/spec.md`

## Summary

Phase 3 closes the remaining enterprise gaps in tianjiLLM: policy engine for conditional guardrail assignment, SCIM 2.0 identity provisioning, Assistants API pass-through, background scheduler, 19 new callbacks, management endpoint extensions, 12 native providers, guardrail/prompt CRUD, and cache/secret/auth enhancements. ~130 tasks across 12 work streams (A-L). Technical approach: self-implement policy engine + scheduler using stdlib patterns; SCIM via `elimity-com/scim` library; distributed lock via `go-redsync/redsync/v4`; OCI auth via `oracle/oci-go-sdk/v65`; use official SDKs where mature (PostHog, GCS Pub/Sub, Lago, Azure Sentinel), HTTP API for the rest.

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**: chi (HTTP router), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), prometheus/client_golang (metrics), opentelemetry-go (tracing), golang-jwt/jwt/v5 (JWT auth), elimity-com/scim (SCIM 2.0 server), scim2/filter-parser/v2 (SCIM filter parsing), go-redsync/redsync/v4 (distributed lock), oracle/oci-go-sdk/v65 (OCI request signing), posthog/posthog-go (PostHog), cloud.google.com/go/pubsub (GCS Pub/Sub), getlago/lago-go-client (Lago), Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs (Azure Sentinel), cyberark/conjur-api-go (CyberArk Conjur), aws-sdk-go-v2 (S3 cache + cold storage), cloud.google.com/go/storage (GCS cache + cold storage), Azure/azure-sdk-for-go/sdk/storage/azblob (Azure Blob cache)
**Storage**: PostgreSQL (primary, existing), Redis/Redis Cluster (cache, existing), S3/GCS/Azure Blob (cold storage archival + cache backends, new)
**Testing**: `go test` + `testify` for assertions, `httptest` for HTTP mocks, contract tests with real JSON fixtures
**Target Platform**: Linux server (amd64/arm64), macOS (development)
**Project Type**: Single project (existing Go repository)
**Performance Goals**: Policy resolution <10ms for 3-level chains; SCIM bulk provisioning 100 users in <30s; management CRUD <200ms; cold storage archival 1M entries/hour
**Constraints**: <5ms added latency for Assistants pass-through; scheduler jitter <10%; zero downtime for hot-reload; fail-closed on secret resolution failure
**Scale/Scope**: 36+ providers, 34 callbacks, 25+ management endpoints, ~130 new tasks

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Python-First Reference | ✅ PASS | All 12 work streams derived from Python TianjiLLM source analysis (policy engine from `tianji/proxy/policy/`, SCIM from `tianji/proxy/scim/`, scheduler from `tianji/proxy/proxy_server.py` APScheduler jobs, callbacks from `tianji/integrations/`, providers from `tianji/llms/`) |
| II. Feature Parity | ✅ PASS | API contracts match Python TianjiLLM (SCIM RFC 7644 endpoints, policy CRUD, assistants pass-through, management endpoint paths); YAML config format compatible |
| III. Research Before Build | ✅ PASS | 6 parallel research agents dispatched; Context7 queried for SCIM/callback/provider SDKs; Python source code analyzed for all 12 work streams; decisions documented in research.md |
| IV. Test-Driven Migration | ✅ PASS | Contract tests planned with real JSON fixtures; Python test cases used as reference; >=90% coverage target for translation layers |
| V. Go Best Practices | ✅ PASS | Policy engine uses stdlib patterns (sync.RWMutex, map rebuild); scheduler uses time.Ticker + context cancellation; SCIM uses elimity-com/scim (proven library, used by Zitadel/Casdoor); distributed lock uses redsync (Redlock, used by Gitea/GitLab); OCI uses official SDK; providers follow existing self-registration init() pattern; dependency injection via interfaces |
| VI. No Stale Knowledge | ✅ PASS | All SDK recommendations verified via Context7 + GitHub code search (PostHog Go SDK confirmed, Lago Go client confirmed, elimity-com/scim confirmed in Casdoor/Zitadel/Getprobo, redsync confirmed in Gitea/GitLab/SeaweedFS, oracle/oci-go-sdk/v65 confirmed, Opik has no Go SDK — use HTTP API); provider API formats verified from Python source |

### Post-Design Re-evaluation (Phase 1 Complete)

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Python-First Reference | ✅ PASS | data-model.md entities map directly to Python DB models; 6 OpenAPI contracts mirror Python endpoint paths exactly |
| II. Feature Parity | ✅ PASS | Contracts verified: SCIM uses RFC 7644 schemas matching Python; policy/guardrail/prompt CRUD paths match; assistants pass-through covers all Python endpoints; management API paths identical |
| III. Research Before Build | ✅ PASS | research.md complete with 8 decisions, all NEEDS CLARIFICATION resolved before design phase |
| IV. Test-Driven Migration | ✅ PASS | Test fixtures directories planned (scim/, policy/, assistants/, providers/); contract test patterns defined |
| V. Go Best Practices | ✅ PASS | Project structure follows existing `internal/` layout; SCIM uses elimity-com/scim library (not reinventing RFC 7644); distributed lock uses redsync (not custom SETNX); OCI uses official SDK for request signing; policy engine self-implemented (narrow requirements, no suitable library); self-registration pattern preserved |
| VI. No Stale Knowledge | ✅ PASS | All SDK choices in research.md backed by Context7 queries + GitHub code search; elimity-com/scim verified in Casdoor/Zitadel/Getprobo; redsync verified in Gitea/GitLab/SeaweedFS; oracle/oci-go-sdk/v65 verified; conjur-api-go verified; no unverified claims in design artifacts |

## Project Structure

### Documentation (this feature)

```text
specs/003-migration-gap-phase3/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── policy-api.yaml
│   ├── scim-api.yaml
│   ├── assistants-api.yaml
│   ├── management-api.yaml
│   ├── guardrail-prompt-api.yaml
│   └── spend-api.yaml
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/tianji/                          # Entry point (modify: add scheduler, policy engine init)
internal/
├── policy/                           # NEW: Policy engine (Work Stream A)
│   ├── engine.go                     #   PolicyEngine struct, Evaluate(), Update()
│   ├── resolver.go                   #   Inheritance chain resolution, cycle detection
│   ├── pipeline.go                   #   Pipeline step execution with on_pass/on_fail/modify_response + pass_data
│   ├── matcher.go                    #   Multi-dimensional attachment matching (teams, keys, models, tags) — prefix wildcard matching (NOT regexp)
│   └── policy_test.go
├── scim/                             # NEW: SCIM 2.0 protocol (Work Stream B) — uses elimity-com/scim library
│   ├── handler.go                    #   ResourceHandler implementations for User + Group (reuses existing User/Team tables)
│   ├── mapper.go                     #   SCIM ↔ internal User/Team field mapping (userName→user_id, active→metadata["scim_active"], etc.)
│   └── scim_test.go                  #   Filter parsing provided by scim2/filter-parser/v2 (not self-implemented)
├── scheduler/                        # NEW: Background job scheduler (Work Stream D)
│   ├── scheduler.go                  #   Scheduler struct, Add(), Start(), Stop(), distributed lock
│   ├── jobs.go                       #   10 core jobs: budget reset, spend update, spend monitor, cleanup, hot-reload, health check, batch cost, responses cost, key rotation, credential refresh
│   ├── lock.go                       #   Distributed lock using go-redsync/redsync/v4 (Redlock algorithm, PodLockManager equivalent)
│   └── scheduler_test.go
│   # Note: Python has 15 total jobs. 5 are callback-specific (Slack weekly/monthly reports,
│   # prometheus_fallback_stats, cloudzero_export, focus_export). These register themselves
│   # via Scheduler.Add() in their respective callback init(), not hardcoded in scheduler.
├── provider/                         # EXTEND: 12 new providers (Work Stream J)
│   ├── githubcopilot/                #   OAuth device flow auth
│   ├── snowflake/                    #   Snowflake auth + tool format transform
│   ├── oci/                          #   OCI auth via oracle/oci-go-sdk/v65 (RequestSigner handles RSA-SHA256)
│   ├── sap/                          #   SAP AI Core OAuth + nested config
│   ├── dashscope/                    #   Simple OpenAI-compat + content transform
│   ├── volcengine/                   #   Simple OpenAI-compat + thinking param
│   ├── minimax/                      #   Simple OpenAI-compat + reasoning_split
│   ├── moonshot/                     #   Simple OpenAI-compat + temp constraint
│   ├── nvidia/                       #   Simple OpenAI-compat + per-model params
│   ├── openrouter/                   #   Simple OpenAI-compat + cache_control
│   ├── deepinfra/                    #   Simple OpenAI-compat + tool msg transform
│   └── azureai/                      #   Azure AD auth + content transform
├── callback/                         # EXTEND: 19 new callbacks (Work Streams E, G)
│   ├── lunary/                       #   HTTP API
│   ├── traceloop/                    #   OpenTelemetry SDK
│   ├── posthog/                      #   posthog-go SDK
│   ├── opik/                         #   HTTP API (REST)
│   ├── datadog_llm/                  #   HTTP API (datadog-go/v5 is DogStatsD only; dd-trace-go/v2 llmobs is experimental)
│   ├── gcspubsub/                    #   cloud.google.com/go/pubsub
│   ├── openmeter/                    #   HTTP API (CloudEvents)
│   ├── greenscale/                   #   HTTP API
│   ├── promptlayer/                  #   HTTP API
│   ├── argilla/                      #   HTTP API
│   ├── lago/                         #   lago-go-client SDK
│   ├── azuresentinel/                #   Azure Monitor Log Ingestion SDK (azure-sdk-for-go/sdk/monitor/ingestion/azlogs)
│   ├── supabase/                     #   pgx (PostgreSQL direct)
│   ├── cloudzero/                    #   HTTP API
│   ├── logfire/                      #   HTTP API
│   ├── athina/                       #   HTTP API
│   ├── deepeval/                     #   HTTP API
│   ├── galileo/                      #   HTTP API
│   └── literalai/                    #   HTTP API
├── proxy/handler/                    # EXTEND: Management + API endpoints (Work Streams F, H, K)
│   ├── policy.go                     #   Policy CRUD handlers
│   ├── scim.go                       #   SCIM route registration
│   ├── assistants.go                 #   Assistants/threads/runs pass-through
│   ├── model_mgmt.go                 #   Model CRUD
│   ├── tag_mgmt.go                   #   Tag CRUD
│   ├── customer_mgmt.go              #   End user CRUD
│   ├── config_mgmt.go                #   Config management API
│   ├── guardrail_mgmt.go             #   Guardrail CRUD
│   ├── prompt_mgmt.go                #   Prompt CRUD + test endpoint
│   ├── vectorstore.go                #   Vector store file + search pass-through
│   ├── responses_ext.go              #   GET/cancel/input_items for Responses API
│   ├── passthrough.go                #   Provider native pass-through namespaces
│   └── spend_global.go               #   Global spend endpoints
├── cache/                            # EXTEND: S3/GCS/Azure Blob backends (Work Stream L)
│   ├── s3.go
│   ├── gcs.go
│   └── azureblob.go
├── secretmanager/                    # EXTEND: CyberArk Conjur (Work Stream L)
│   └── conjur.go
├── spend/                            # EXTEND: Cold storage archival (Work Stream I)
│   ├── archiver.go                   #   Batch export to S3/GCS
│   └── views.go                      #   Pre-computed spend aggregation queries
├── db/                               # EXTEND: New migrations + sqlc queries
│   └── queries/
│       ├── policy.sql
│       ├── guardrail_mgmt.sql
│       ├── prompt_mgmt.sql
│       ├── model_mgmt.sql
│       ├── tag_mgmt.sql
│       ├── customer_mgmt.sql
│       └── spend_views.sql
└── model/                            # EXTEND: New types for policy, SCIM, etc.

test/
├── contract/                         # Handler tests with mock upstream
├── integration/                      # Full server flow tests
└── fixtures/                         # Real provider request/response JSON
    ├── scim/
    ├── policy/
    ├── assistants/
    └── providers/                    # Per-provider fixtures
```

**Structure Decision**: Follows existing single-project layout (`cmd/` + `internal/` + `test/`). New subsystems (policy, scim, scheduler) get their own packages under `internal/`. New providers and callbacks follow the existing self-registration pattern. No new top-level directories needed.

## Complexity Tracking

> No constitution violations. All designs use existing patterns (self-registration, sync.RWMutex + map rebuild, stdlib ticker). Three modules use proven third-party libraries instead of self-implementing: SCIM (elimity-com/scim), distributed lock (redsync), OCI auth (oracle/oci-go-sdk). This reduces custom code by ~500-600 lines and improves RFC compliance + reliability.
