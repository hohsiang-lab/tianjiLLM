# Research: Proactive OpenAI Subscription Credential Refresh

**Date**: 2026-06-18
**Feature**: HO-2091

## Repo Evidence

### Current refresh behavior is request/lifecycle-triggered

**Decision**: Add a scheduler-owned proactive path rather than changing only request-time resolution.

**Rationale**: `resolveUsableOpenAISubscriptionBundle` loads one configured credential, checks `openAISubscriptionTokenIsFresh`, and refreshes only when a credential is being resolved. `OpenAISubscriptionCredentialRefresh`, credential test, Codex usage refresh, and upstream 401 forced refresh also require an explicit touch. There is no background scan of idle credentials.

**Alternatives considered**:

- Only rely on route-time refresh: rejected because idle credentials can expire before the next route touch.
- Refresh all credentials on every request: rejected because it couples unrelated traffic to credential maintenance.

### Existing 5-minute freshness buffer must remain request-time behavior

**Decision**: Keep `openAISubscriptionRefreshBuffer = 5 * time.Minute` for request-time freshness and introduce a separate 30-minute proactive buffer.

**Rationale**: `openAISubscriptionTokenIsFresh` currently returns true only when `expires_at > now + 5m`. The issue explicitly requires preserving this while using `30m` for proactive refresh candidates.

### Existing scheduler is the correct job host

**Decision**: Implement a new scheduler job and register it from `cmd/tianji/main.go` when DB-backed credential storage exists.

**Rationale**: `internal/scheduler` already defines `Job`, fixed-interval `Scheduler`, `Add`, `AddWithStartupRun`, panic recovery, and graceful `Stop`. `cmd/tianji/main.go` registers DB-backed jobs there. This avoids a second background worker framework.

### Existing refresh locking can cover request-time concurrency

**Decision**: Reuse `Handlers.openAIRefreshGroup` and `refreshOpenAISubscriptionCredential` for proactive refresh.

**Rationale**: `openai_subscription_refresh.go` already uses `singleflight.Group.Do` keyed by credential ID for normal refresh, and force/usage refresh use namespaced keys. Proactive refresh should use the same normal key when refreshing due to expiry so a route-time resolver and proactive job share work.

### Cross-pod coordination needs existing distributed lock or DB claiming

**Decision**: Prefer wrapping the proactive scheduler job with the existing Redis `scheduler.NewWithLock` when Redis is available, and also use per-credential reload/freshness checks before refresh. If Redis is unavailable, implementation must explicitly document and test the single-pod fallback behavior.

**Rationale**: `internal/scheduler/lock.go` already wraps jobs with `redsync` using `WithExpiry` and `WithTries(1)`. The issue requires no stampede across concurrent pods; a global job lock is the lowest-risk fit for existing infrastructure, while reloading a credential inside `singleflight` prevents stale local decisions.

### sqlc query layer needs a candidate query

**Decision**: Add a sqlc named query that returns active `openai_subscription` credentials whose safe metadata is selectable and then filter/decrypt expiry in Go, unless implementation proves an indexable metadata shape is needed.

**Rationale**: `CredentialTable` stores encrypted `credential_value`, so SQL cannot safely evaluate `expires_at`. `credential_info` is JSONB and can filter `credential_type`, status, and deleted/non-selectable metadata first. sqlc-first constitution forbids ad hoc SQL in handler/scheduler code.

### Routing already excludes disabled, but not all failed states

**Decision**: Extend credential loading/routing eligibility so `refresh_failed`, reconnect-required, and deleted/missing references are not repeatedly considered selectable.

**Rationale**: `loadOpenAISubscriptionCredential` currently rejects `status=disabled` only. Existing routing captures failures for missing/malformed/disabled but will still attempt credentials with `status=refresh_failed` unless new eligibility rules exclude them.

## External Evidence

### Go ticker and scheduler lifecycle

**Decision**: Continue using `time.NewTicker` through the existing scheduler rather than a custom sleep loop.

**Rationale**: Official Go `time` package documentation describes ticker-based periodic delivery and stop behavior. The repo already wraps this pattern in `internal/scheduler`.

**Source**: <https://pkg.go.dev/time>

### `singleflight.Group.Do`

**Decision**: Use the existing `singleflight.Group.Do` path for same-process duplicate suppression.

**Rationale**: Context7/pkg.go.dev documents that `Group.Do` ensures only one execution is in-flight for a key; duplicate callers wait and receive the same result.

**Source**: Context7 `/websites/pkg_go_dev_golang_org_x_sync`, upstream source <https://pkg.go.dev/golang.org/x/sync/singleflight>

### `redsync` distributed mutex

**Decision**: Use the existing scheduler distributed lock wrapper when Redis exists.

**Rationale**: Context7 documents `redsync.NewMutex` with `WithExpiry`, `WithTries`, `LockContext`, and `UnlockContext`; the repo already has `NewWithLock` using these primitives with one try.

**Source**: Context7 `/go-redsync/redsync`

### sqlc named queries

**Decision**: Add DB access through `internal/db/queries/credential.sql` and regenerate via `sqlc`, not through raw `pgx.Query` in the job.

**Rationale**: Context7/sqlc docs show `-- name:` comments generate Go methods and params structs. This matches the Tianji constitution and existing repo pattern.

**Source**: Context7 `/websites/sqlc_dev_en`, sqlc docs <https://docs.sqlc.dev/>

### GitHub public code search

**Decision**: Do not import a public implementation pattern.

**Rationale**: grep.app API returned a Vercel security checkpoint HTML page for both `singleflight.Group RefreshToken Go` and `FOR UPDATE SKIP LOCKED sqlc`; no usable public-code evidence was obtained. Repo-local patterns are stronger here.

## Decisive Scope Answers

- **Repo ownership**: `hohsiang-lab/tianjiLLM`.
- **Layer boundary**: backend scheduler + handler refresh/routing + existing credential/model UI status text; no production code in Todo.
- **Data/API shape**: existing `CredentialTable.credential_value` encrypted bundle and `credential_info` safe JSONB; add safe metadata fields only.
- **Dependencies**: existing scheduler, sqlc, `singleflight`, optional Redis `redsync`, OpenAI OAuth refresh mock harness.
- **Verification method**: Go unit/handler/scheduler tests with mock token servers and redaction assertions.
- **Implementation detail clarity**: 已釐清 — repo path, scheduler host, refresh helper, locking strategy, metadata shape, routing exclusion, and test surfaces are evidence-backed; no owner decision is needed before tasks.
