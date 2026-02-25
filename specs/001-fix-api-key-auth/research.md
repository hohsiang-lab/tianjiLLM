# Research: Fix API Key Authentication

**Feature**: `001-fix-api-key-auth` | **Phase**: 0 | **Date**: 2026-02-25

## Summary

This is a pure wiring bug — no new libraries, no new algorithms, no external
research needed. The research phase focuses on verifying existing codebase
conventions that the fix must follow.

---

## Decision 1: Where to place the `DBValidator` adapter

**Decision**: `internal/proxy/middleware/db_validator.go`

**Rationale**:
- `internal/proxy/middleware/budget.go` already imports `internal/db` and defines
  a `BudgetChecker` interface that wraps `GetVerificationToken`. This establishes the
  pattern of middleware-layer interfaces over DB queries.
- No new package is needed; no import cycles created.
- The `TokenValidator` interface lives in `middleware/auth.go`, so the adapter
  satisfying that interface naturally belongs in the same package.

**Alternatives considered**:
- `internal/db/auth_adapter.go`: Rejected — the constitution permits only
  `extensions.go` for runtime-type-assertion methods; business logic adapters
  belong above the DB layer.
- `cmd/tianji/main.go` (local type): Rejected — untestable as a local struct.
- New `internal/proxy/authdb/` package: Rejected — over-engineering a 4-method struct.

**Source**: `/private/tmp/tianjiLLM/internal/proxy/middleware/budget.go:12-15`

---

## Decision 2: Error differentiation (key-not-found vs. DB unavailable)

**Decision**: Two package-level sentinel errors in `middleware` package:
```go
var (
    ErrKeyNotFound   = errors.New("virtual key not found")
    ErrDBUnavailable = errors.New("database unavailable")
)
```
- `DBValidator.ValidateToken` maps `pgx.ErrNoRows` → `ErrKeyNotFound` (→ 401 in middleware)
- Any other DB error → `fmt.Errorf("%w: %v", ErrDBUnavailable, err)` (→ 503 in middleware)

**Rationale**:
- `pgx.ErrNoRows` (from `pgx.Row.Scan()` on a missing row) is the canonical
  "not found" signal in pgx/v5. Verified: `GetVerificationToken` calls
  `q.db.QueryRow(...).Scan(...)`, which returns `pgx.ErrNoRows` when no row matches.
- No existing sentinel in the codebase currently handles this distinction
  (confirmed: zero `errors.Is(err, pgx.ErrNoRows)` calls found in the repo).
- Middleware must not import pgx directly (concern of the adapter layer only);
  sentinels keep the auth logic decoupled from the DB driver.
- `model.ErrServiceUnavailable` already exists but importing `internal/model`
  from `middleware` just for a sentinel is extra coupling. Defining them in
  `middleware` is self-contained.

**pgx version verified**: `github.com/jackc/pgx/v5 v5.8.0` (go.mod)
**pgx ErrNoRows path**: `github.com/jackc/pgx/v5` — `pgx.ErrNoRows`

**Source**: `/private/tmp/tianjiLLM/internal/db/db.go:8-12`,
`/private/tmp/tianjiLLM/internal/db/verification_token.sql.go` (GetVerificationToken
calls `row.Scan` which propagates `pgx.ErrNoRows` on no match)

---

## Decision 3: Guardrail loading

**Decision**: `DBValidator` implements `GuardrailProvider` by returning
`VerificationToken.Policies []string` as the guardrail/policy names.

**Rationale**:
- `VerificationToken.Policies []string` (`/internal/db/models.go:493`) holds
  the policy/guardrail names associated with a key.
- The existing `GuardrailProvider` interface (`middleware/auth.go:34-37`) expects
  `[]string` of guardrail names — direct match.
- No additional SQL query is needed (reuse the same `GetVerificationToken` result
  from a separate call, matching the existing two-phase interface design).

**Source**: `/private/tmp/tianjiLLM/internal/db/models.go:493`,
`/private/tmp/tianjiLLM/internal/proxy/middleware/auth.go:34-37`

---

## Decision 4: Structured logging approach

**Decision**: Use Go's `log` package (already used in the codebase and in
`middleware/auth.go:80`). Log at INFO for success, WARN for auth failure,
ERROR for DB failure.

**Rationale**: Existing codebase uses `log.Printf` throughout (including in
`middleware/auth.go:80`: `log.Printf("JWT validation failed: %v", err)`).
No structured logging framework (zerolog, zap, slog) is in use. Consistency
with existing code means `log.Printf` with a consistent prefix format.

**Source**: `/private/tmp/tianjiLLM/internal/proxy/middleware/auth.go:80`

---

## Decision 5: Wiring in main.go

**Decision**:
```go
var validator middleware.TokenValidator
if queries != nil {
    validator = &middleware.DBValidator{DB: queries}
}
srv := proxy.NewServer(proxy.ServerConfig{
    ...
    DBQueries: validator,
    ...
})
```

**Rationale**: `ServerConfig.DBQueries` is typed as `middleware.TokenValidator`
(a nil interface). When `queries == nil` (no DB configured), `validator` stays
nil, preserving existing nil-safe behaviour in `NewServer` (which checks
`cfg.Validator != nil` in the middleware).

**Source**: `/private/tmp/tianjiLLM/internal/proxy/server.go:41`,
`/private/tmp/tianjiLLM/cmd/tianji/main.go:132-152`

---

## Verified: No NEEDS CLARIFICATION items

All technical decisions are resolved. No unknowns remain.
