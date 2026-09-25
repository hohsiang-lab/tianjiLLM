# Research: Upstream Token in SpendLogs + Key Alias

## R1: How does upstream token selection reach the spend callback?

**Decision**: Inject `upstream_token_key` into `context.Context` at the selection point, then extract it in `buildBaseLogData()` — the same pattern used for APIKey, UserID, TeamID, OrgID, and RequesterIP (handler.go:217-235).

**Rationale**: `buildBaseLogData()` already extracts 5 fields from context. Adding a 6th keeps the pattern uniform. The alternative (passing apiKey through function signatures) would require changing `buildNativeLogData()` signature, adding `apiKey` to `sseSpendReader` struct, and updating 2 call sites — shotgun surgery for one field. Context injection is 3 lines total: 1 constant + 1 WithValue + 1 extraction.

**Data flow**:
```
nativeProxy():
  upstream := selectUpstreamWithThrottle()
  apiKey := upstream.APIKey
  ctx = context.WithValue(ctx, ContextKeyUpstreamToken, sha256(apiKey)[:12])  ← 1 line
  ...
  ModifyResponse closure captures ctx (same variable)
    → buildNativeLogData(ctx, ...) → buildBaseLogData(ctx, startTime)
      → data.UpstreamTokenKey = ctx.Value(ContextKeyUpstreamToken)            ← extracted here
        → Tracker.LogSuccess() → SpendRecord{..., UpstreamTokenKey}
          → Record() → CreateSpendLogParams{..., UpstreamTokenKey}
            → INSERT INTO SpendLogs (..., upstream_token_key)

Standard provider path:
  ctx has no ContextKeyUpstreamToken → data.UpstreamTokenKey = "" (zero value)
```

**Why context works here**: `ctx` is a local variable (native_format.go:73). `context.WithValue()` returns a new context, and reassigning `ctx = context.WithValue(ctx, ...)` before the `ReverseProxy` closure is created means the closure captures the updated variable. This is safe because Go closures capture variables by reference.

**Alternatives considered**:
- Function signature threading (pass apiKey through buildNativeLogData → LogData → SpendRecord): Rejected — requires modifying 5 structs and 1 function signature + sseSpendReader struct for one field. Violates "消除特殊情況" — this would be the only field threaded through signatures while all others use context.
- Store raw apiKey instead of hash: Rejected — security risk; hash is already the canonical format used by rate limit store.

## R2: How to resolve key alias in the request log query?

**Decision**: LEFT JOIN `VerificationToken` in `ListRequestLogs` SQL query, same pattern as `GetTopKeysBySpend`.

**Rationale**: `GetTopKeysBySpend` (spend_views.sql) already does `LEFT JOIN "VerificationToken" vt ON sl.api_key = vt.token` with `COALESCE(vt.key_alias, '')`. Reuse the same pattern. LEFT JOIN ensures rows without matching keys still appear (deleted keys, master key).

**UNION concern**: `ListRequestLogs` is a UNION of SpendLogs and ErrorLogs. Both halves need the JOIN:
- SpendLogs half: `LEFT JOIN "VerificationToken" vt ON sl.api_key = vt.token`
- ErrorLogs half: `LEFT JOIN "VerificationToken" vt2 ON el.api_key_hash = vt2.token`

**Performance**: VerificationToken is small (4 rows in current deployment). JOIN cost is negligible. The `api_key` column is already indexed (`idx_spend_logs_api_key`), and `VerificationToken.token` is the PRIMARY KEY.

**Alternatives considered**:
- Application-level resolution (query VerificationToken separately, map in Go): Rejected — adds N+1 query risk and more code. SQL JOIN is simpler and atomic.
- Denormalize alias into SpendLogs: Rejected — alias can change; JOIN always shows current alias.

## R3: DB migration strategy for upstream_token_key

**Decision**: Migration 019 adds columns to both SpendLogs and ErrorLogs in a single migration.

**Rationale**:
- `NOT NULL DEFAULT ''` means existing rows get empty string (not NULL), consistent with FR-002. PG11+ handles constant DEFAULT as metadata-only (no table rewrite).
- SpendLogs index on `upstream_token_key` enables efficient filtering (FR-004).
- ErrorLogs gains `upstream_token_key` (FR-011), `end_user`, `organization_id` (FR-009) for parity with SpendLogs — enables consistent cross-table investigation of both success and failure requests.
- ErrorLogs index on `upstream_token_key` enables filter to capture failed requests (e.g. 429s) for the same token.
- Next available migration number is 019 (after 018_oauth_token_disabled).

**Migration SQL**:
```sql
-- 019_upstream_token.up.sql
ALTER TABLE "SpendLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_spend_logs_upstream_token ON "SpendLogs" (upstream_token_key);

ALTER TABLE "ErrorLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN end_user TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN organization_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_error_logs_upstream_token ON "ErrorLogs" (upstream_token_key);

-- 019_upstream_token.down.sql
DROP INDEX IF EXISTS idx_error_logs_upstream_token;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS organization_id;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS end_user;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS upstream_token_key;
DROP INDEX IF EXISTS idx_spend_logs_upstream_token;
ALTER TABLE "SpendLogs" DROP COLUMN IF EXISTS upstream_token_key;
```

## R4: Token identifier format consistency

**Decision**: Use `sha256(apiKey)[:12]` — same as `InMemoryRateLimitStore`.

**Rationale**: The rate limit store uses `sha256(apiKey)[:12]` as the token key (callback/ratelimit_store.go). The rate limit dashboard displays this format (e.g. `b545c01499c2`). Using the same format enables direct cross-referencing between the two UIs.

**Implementation**: In `native_format.go`, compute `crypto/sha256.Sum256([]byte(apiKey))` and hex-encode first 6 bytes (12 hex chars). This matches the existing `tokenKeyFromAPIKey()` helper pattern.

## R5: Filter UI for upstream token

**Decision**: Add `filter_upstream_token` parameter to `ListRequestLogs` and `CountRequestLogs` queries, expose as URL query param in handler_logs.go, add dropdown in logs.templ filter bar.

**Rationale**: Follows the exact same pattern as existing filters (filter_api_key, filter_team_id, filter_model, etc.). The filter bar in logs.templ already has input fields for these; adding one more is consistent.

**Dropdown population**: Query distinct `upstream_token_key` values from SpendLogs for the time range, or hardcode from config's OAuth tokens. The simpler approach is a text input (same as Key Hash filter) since the operator copies the token ID from the rate limit dashboard.
