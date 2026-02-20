# Phase 7 Migration Gap Analysis — Implementation Record

## Phase 7a: Client Compatibility (P0) — COMPLETED

### 1. Bare Path Aliases
**Status**: Done
**Files**: `internal/proxy/server.go`

Extracted LLM route registration into `registerLLMRoutes()` closure, called under both `/v1` (canonical) and bare path (no prefix) via `r.Group()`. All 69+ LLM endpoints now accessible at both paths.

### 2. Azure Engine/Deployment Paths
**Status**: Done
**Files**: `internal/proxy/server.go`

Added `/engines/{model}/chat/completions|completions|embeddings` and `/openai/deployments/{model}/chat/completions|completions|embeddings` route groups.

### 3. Path/Method Inconsistencies (5 fixes)
**Status**: Done
**Files**: `internal/proxy/server.go`

| Fix | Route Added |
|-----|------------|
| POST /budget/info | Alias for GET handler |
| POST /team/member_add | Alias for /team/member/add |
| POST /team/member_delete | Alias for /team/member/delete |
| GET /user/daily/activity | Alias for /user/daily_activity |
| GET /team/{team_id}/callback | Alias for /team/callback |

### 4. Utility Endpoints
**Status**: Done
**Files**: `internal/proxy/handler/utils.go`, `internal/proxy/server.go`

| Endpoint | Implementation |
|----------|---------------|
| GET /utils/supported_openai_params | Uses `provider.ParseModelName()` + `provider.Get()` + `GetSupportedParams()` |
| POST /utils/token_counter | Uses `internal/token.Counter` for tiktoken-based counting |

---

## Phase 7b: Management Completeness (P1) — COMPLETED

### 5. Budget CRUD Completion
**Status**: Done
**Files**: `internal/proxy/handler/budget.go`, `internal/proxy/server.go`

| Endpoint | DB Query Used |
|----------|--------------|
| POST /budget/update | `db.UpdateBudget` (existing) |
| GET /budget/list | `db.ListBudgets` (existing) |
| POST /budget/delete | `db.DeleteBudget` (existing) |
| GET /budget/settings | Returns budget duration options |

Also enhanced `BudgetInfo` to accept POST with JSON body (Python compat).

### 6. Organization Member Management
**Status**: Done
**Files**: `internal/proxy/handler/organization.go`, `internal/db/schema/010_org_membership.sql`, `internal/db/queries/org_membership.sql`

New `OrganizationMembership` table with composite PK (user_id, organization_id).

| Endpoint | Handler |
|----------|---------|
| POST /organization/member_add | `OrgMemberAdd` |
| PATCH /organization/member_update | `OrgMemberUpdate` |
| DELETE /organization/member_delete | `OrgMemberDelete` |

### 7. Dynamic Rate Limiter v3 Enhancement
**Status**: Done
**Files**: `internal/proxy/middleware/dynamic_ratelimit.go`, `internal/proxy/middleware/helpers.go`

Enhancements:
- **Fixed bug**: `Expire(ctx, key, 60*1e9)` → `Expire(ctx, key, 60*time.Second)`
- **Model-level saturation**: Per-model Redis keys (`tianji:dynamic_rate:saturation:{model}`)
- **TPM dimension**: Separate counters for tokens per minute (`tianji:dynamic_tpm:{key}:{model}`)
- **X-RateLimit headers**: `X-RateLimit-Limit-Requests`, `X-RateLimit-Remaining-Requests`, `X-RateLimit-Limit-Tokens`, `X-RateLimit-Remaining-Tokens`, `X-RateLimit-Reset-Requests`
- **New API**: `CheckFull()` returns `CheckResult` with remaining counts; `RecordTokens()` for TPM tracking; `RecordModelUtilization()` for per-model saturation

### 8. Model Max Budget Limiter Middleware
**Status**: Done
**Files**: `internal/proxy/middleware/model_budget.go`

Reads `model_max_budget` and `model_spend` JSONB fields from VerificationToken. Blocks requests when per-model spend exceeds per-model budget limit for the specific key.

### 9. Generic API Callback Framework
**Status**: Done
**Files**: `internal/callback/generic_api.go`, `internal/callback/factory.go`

`GenericAPICallback` POSTs structured JSON events to any HTTP endpoint. Registered as `"generic_api"` in callback factory. Users can configure arbitrary webhook URLs with optional headers.

### 10. GitHub Models Provider
**Status**: Done
**Files**: `internal/provider/github/github.go`, `cmd/tianji/main.go`

OpenAI-compatible wrapper pointing to `models.inference.ai.azure.com`. Uses Bearer token auth with GitHub PAT. Provider name: `"github"`.

---

## Verification

```bash
make check  # lint ✓ test ✓ build ✓
```

All 10 tasks completed. Total provider count: **51** (was 50).
