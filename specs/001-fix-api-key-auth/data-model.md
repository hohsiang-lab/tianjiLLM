# Data Model: Fix API Key Authentication

**Feature**: `001-fix-api-key-auth` | **Phase**: 1 | **Date**: 2026-02-25

## Overview

No new data models. This fix adds a thin adapter layer that bridges the existing
`VerificationToken` DB entity to the existing `TokenValidator` middleware interface.

---

## Existing Entity: VerificationToken

**Table**: `"VerificationToken"` (PostgreSQL)
**Go type**: `db.VerificationToken` (sqlc-generated, `internal/db/models.go:471`)

Relevant fields used by the adapter:

| Field | Type | Role in auth |
|-------|------|-------------|
| `Token` | `string` | SHA-256 hex hash of the raw API key (lookup key) |
| `UserID` | `*string` | Owner user identity → set in request context |
| `TeamID` | `*string` | Owning team identity → set in request context |
| `Blocked` | `*bool` | If true → 403 Forbidden |
| `Policies` | `[]string` | Guardrail/policy names to load → `ContextKeyGuardrails` |

All other fields (`Spend`, `MaxBudget`, `Expires`, `Models`, etc.) are used by
other middlewares (budget, rate-limit) and are unaffected by this fix.

---

## New Types (adapter layer)

### `middleware.DBValidator` (new struct)

**File**: `internal/proxy/middleware/db_validator.go`

```
DBValidator
├── DB   *db.Queries          sqlc queries object (injected)
├── ValidateToken(ctx, hash) → (userID, teamID *string, blocked bool, err error)
│     calls: DB.GetVerificationToken(ctx, hash)
│     pgx.ErrNoRows     → ErrKeyNotFound  (caller maps to 401)
│     other DB error    → ErrDBUnavailable (caller maps to 503)
│     blocked == true   → return blocked=true (caller maps to 403)
│     success           → return userID, teamID (caller sets context)
└── GetGuardrails(ctx, hash) → ([]string, error)
      calls: DB.GetVerificationToken(ctx, hash)
      returns VerificationToken.Policies
```

**Implements**:
- `middleware.TokenValidator` (satisfies `AuthConfig.Validator`)
- `middleware.GuardrailProvider` (optional interface, loaded by auth middleware)

### Sentinel Errors (new package-level vars)

**File**: `internal/proxy/middleware/db_validator.go`

```go
var (
    // ErrKeyNotFound signals the token hash has no matching row → HTTP 401
    ErrKeyNotFound = errors.New("virtual key not found")

    // ErrDBUnavailable signals a DB connectivity/query failure → HTTP 503
    ErrDBUnavailable = errors.New("database unavailable")
)
```

---

## Auth Context Keys (existing, unchanged)

The following context keys are populated after successful virtual key auth.
They already exist in `middleware/auth.go`; no new keys are introduced.

| Key | Type | Set by |
|-----|------|--------|
| `ContextKeyIsMasterKey` | `bool` | `false` for virtual keys |
| `ContextKeyTokenHash` | `string` | SHA-256 hex of raw token |
| `ContextKeyUserID` | `string` | `VerificationToken.UserID` (if non-nil) |
| `ContextKeyTeamID` | `string` | `VerificationToken.TeamID` (if non-nil) |
| `ContextKeyGuardrails` | `[]string` | `VerificationToken.Policies` (if non-empty) |

---

## Data Flow

```
Client request (Authorization: Bearer sk-xxxxx)
        │
        ▼
auth middleware: extractToken(r) → "sk-xxxxx"
        │
        ▼
hashToken("sk-xxxxx") → SHA-256 hex "abc123..."
        │
        ├── masterKey match? → set is_master_key=true, continue
        │
        ├── JWT? → JWT path (unchanged)
        │
        └── DBValidator.ValidateToken(ctx, "abc123...")
                │
                ▼
            db.GetVerificationToken(ctx, "abc123...")
                │
                ├── pgx.ErrNoRows → ErrKeyNotFound
                │       → authError(w, "invalid API key", 401)
                │
                ├── other error → ErrDBUnavailable
                │       → authError(w, "service unavailable", 503)
                │
                └── row found
                        ├── blocked==true → authError(w, "API key is blocked", 403)
                        └── success → set context: userID, teamID, tokenHash
                                          + guardrails (GetGuardrails call)
                                      → next.ServeHTTP(w, r)
```

---

## Schema & Queries

**No schema changes.** The existing `GetVerificationToken` sqlc query is sufficient:

```sql
-- name: GetVerificationToken :one
SELECT * FROM "VerificationToken"
WHERE token = $1;
```

Lookup is by `token` (the SHA-256 hash stored in the DB). The raw API key is
hashed in `auth.go:hashToken()` before being passed to `ValidateToken`.
Confirmed: existing `hashToken()` (SHA-256 hex) matches the storage format used
by key creation handlers in `key.go`.
