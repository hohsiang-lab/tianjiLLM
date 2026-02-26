# Implementation Plan: Fix API Key Authentication

**Branch**: `001-fix-api-key-auth` | **Date**: 2026-02-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-fix-api-key-auth/spec.md`

## Summary

`cmd/tianji/main.go:423` constructs `proxy.ServerConfig` without the `DBQueries` field,
so `middleware.AuthConfig.Validator` is always `nil`. The virtual key branch in
`NewAuthMiddleware` is never reached, causing every virtual-key request to return
401 Unauthorized.

The fix has three parts:
1. **Adapter** (`internal/proxy/middleware/db_validator.go`): a thin `DBValidator`
   struct that wraps `*db.Queries` and satisfies the existing `TokenValidator` and
   `GuardrailProvider` interfaces. Distinguishes "key not found" (→ 401) from
   "DB unavailable" (→ 503) via a sentinel error.
2. **Auth middleware update** (`internal/proxy/middleware/auth.go`): handle the
   sentinel error to return 503, and add structured log entries (INFO on success,
   WARN/ERROR on failure) per FR-010.
3. **Wiring fix** (`cmd/tianji/main.go`): pass `DBQueries: &middleware.DBValidator{DB: queries}`
   when `queries != nil`.

No schema changes, no new SQL queries, no new API endpoints.

## Technical Context

**Language/Version**: Go 1.24.4
**Primary Dependencies**: chi/v5 (router), pgx/v5 (PostgreSQL driver), sqlc (query codegen)
**Storage**: PostgreSQL — existing `VerificationToken` table via sqlc-generated `*db.Queries`
**Testing**: `go test` + testify; Playwright E2E
**Target Platform**: Linux server (containerised)
**Project Type**: Single project (Go monorepo)
**Performance Goals**: Virtual key auth adds ≤ 50ms overhead (one DB SELECT per request)
**Constraints**: No schema changes; no caching (out of scope per clarification)
**Scale/Scope**: All authenticated proxy requests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ✅ PASS | Bug fix is Go-only wiring; no Python LiteLLM feature to reference. The virtual-key auth concept mirrors Python's `prisma_db.get_key` lookup pattern but no direct code migration needed. |
| II. Feature Parity | ✅ PASS | Existing API contracts unchanged; fix restores behaviour that should already work. |
| III. Research Before Build | ✅ PASS | No new libraries introduced; all dependencies (pgx/v5, sqlc) already in use. Research phase confirms this via targeted verification (see research.md). |
| IV. Test-Driven Migration | ✅ PASS | FR-011 mandates unit tests for wiring + integration test for full auth path. |
| V. Go Best Practices | ✅ PASS | Adapter uses interface-based DI; no globals; context passed through; errors wrapped with `%w`. |
| VI. No Stale Knowledge | ✅ PASS | `pgx.ErrNoRows` sentinel verified in research.md; no unverified claims. |
| VII. sqlc-First DB Access | ✅ PASS | Adapter calls existing sqlc-generated `GetVerificationToken`; zero hand-written SQL. |

**Gate result: PASS — no violations, no Complexity Tracking needed.**

## Project Structure

### Documentation (this feature)

```text
specs/001-fix-api-key-auth/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (affected files)

```text
internal/proxy/middleware/
├── auth.go              # MODIFY: 503 handling, structured logging
└── db_validator.go      # NEW: DBValidator adapter (TokenValidator + GuardrailProvider)

cmd/tianji/
└── main.go              # MODIFY: pass DBQueries to ServerConfig

internal/proxy/middleware/
└── auth_test.go         # NEW: unit tests for auth middleware wiring

test/integration/
└── virtual_key_auth_test.go  # NEW: integration test for full virtual-key auth path
```

**Structure Decision**: Single project layout. No new packages. The adapter lives in
`internal/proxy/middleware/` because (a) `middleware` already imports `internal/db`
(via `budget.go`), (b) it's the natural home for auth-adjacent adapters, and
(c) avoids adding a new package for a four-method struct.
