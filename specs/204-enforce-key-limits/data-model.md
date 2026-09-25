# Data Model: Enforce Per-Key Rate Limits & Budget Controls

## Entities

### TokenInfo (Expanded)

現有 `TokenInfo` struct 在 `db_validator.go` 中，需擴展：

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| UserID | *string | VerificationToken.UserID | 已有 |
| TeamID | *string | VerificationToken.TeamID | 已有 |
| OrgID | *string | VerificationToken.OrganizationID | 已有 |
| Blocked | bool | VerificationToken.Blocked | 已有 |
| Guardrails | []string | VerificationToken.Policies | 已有 |
| **RpmLimit** | ***int64** | **resolved(key → team → org)** | **新增** |
| **TpmLimit** | ***int64** | **resolved(key → team → org)** | **新增** |
| **MaxBudget** | ***float64** | **resolved(key → team → org)** | **新增** |
| **Spend** | **float64** | **VerificationToken.Spend** | **新增** |
| **MaxParallelRequests** | ***int32** | **BudgetTable.MaxParallelRequests** | **新增** |
| **Models** | **[]string** | **VerificationToken.Models** | **新增** |
| **Expires** | **pgtype.Timestamptz** | **VerificationToken.Expires** | **新增** |

### Context Keys (New)

新增到 `helpers.go`：

| Key | Type | Used By |
|-----|------|---------|
| `rpmLimitKey` | int64 | 已有，dynamic_ratelimit.go |
| `tpmLimitKey` | int64 | 已有，dynamic_ratelimit.go |
| `maxParallelLimitKey` | int64 | 已有，parallel.go |
| `maxBudgetKey` | *float64 | **新增**，budget.go |
| `spendKey` | float64 | **新增**，budget.go |
| `allowedModelsKey` | []string | 已定義 (`ContextKeyAllowedModels`)，未使用 |
| `expiresKey` | pgtype.Timestamptz | **新增**，auth.go |

### Limit Resolution Chain

```
resolveLimit(key, team, org) → *int64:
  if key.limit != nil → return key.limit
  if team != nil && team.limit != nil → return team.limit
  if org != nil && org.limit != nil → return org.limit
  return nil  // no limit
```

適用於：RpmLimit, TpmLimit, MaxBudget。

### Redis Keys (Existing, No Change)

| Redis Key Pattern | TTL | Used By |
|---|---|---|
| `tianji:dynamic_rpm:{keyHash}` | 60s | DynamicRateLimiter RPM |
| `tianji:dynamic_tpm:{keyHash}` | 60s | DynamicRateLimiter TPM |
| `tianji:parallel:{keyHash}` | per-request | Parallel middleware |
| `tianji:spend:{keyHash}` | none | **新增** — spend cache for budget hot-path |

### DB Queries Used (Existing, No New Queries)

| Query | File | When Called |
|---|---|---|
| `GetVerificationToken` | verification_token.sql.go | Auth — every request |
| `GetTeam` | team.sql.go | Auth — only if key limit is null and teamID exists |
| `GetOrganization` | organization.sql.go | Auth — only if team limit also null and orgID exists |
| `GetBudget` | budget.sql.go | Auth — only if budgetID exists (for max_parallel_requests) |
| `UpdateVerificationTokenSpend` | verification_token.sql.go | Post-call — spend tracker (existing) |

## State Transitions

### Rate Limit Counter Lifecycle

```
Request arrives → Pre-check: read Redis counter
  → counter < limit → INCR counter, allow request
  → counter >= limit → reject 429

60s TTL expires → counter auto-deleted → new window starts
```

### Budget Check Lifecycle

```
Request arrives → Read spend from context (injected by auth from DB)
  → Also check Redis spend cache (if available, more recent)
  → max(db_spend, cache_spend) >= max_budget → reject 429
  → else → allow

Post-call → spend tracker updates:
  1. DB: UpdateVerificationTokenSpend (existing)
  2. Redis: INCRBYFLOAT tianji:spend:{keyHash} cost (new)
```

### Parallel Request Lifecycle

```
Request arrives → Redis INCR tianji:parallel:{keyHash}
  → count > limit → DECR, reject 429
  → count <= limit → allow, proceed

Response complete → Redis DECR tianji:parallel:{keyHash}
```
