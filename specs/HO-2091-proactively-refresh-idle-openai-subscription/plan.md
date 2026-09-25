# Implementation Plan: Proactive OpenAI Subscription Credential Refresh

**Branch**: `HO-2091-proactively-refresh-idle-openai-subscription` | **Date**: 2026-06-18 | **Spec**: [spec.md](spec.md)
**Input**: Linear HO-2091

## Summary

Add a DB-backed scheduler job that scans active `openai_subscription` credentials, decrypts only eligible rows, refreshes credentials whose access token expires within a 30-minute proactive buffer, and persists either refreshed token material or safe reconnect-required metadata. Keep the existing 5-minute request-time freshness rule and reuse the current refresh helper / `singleflight` semantics so proactive and request-time refresh do not stampede.

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Primary Dependencies**: existing `internal/scheduler`, `time`, `context`, `golang.org/x/sync/singleflight`, optional existing `go-redsync/redsync`, existing sqlc/pgx credential queries
**Storage**: PostgreSQL `CredentialTable` via sqlc, encrypted `credential_value`, safe JSONB `credential_info`
**Testing**: `go test`, existing handler mock stores, `httptest`, `internal/testutil/openaitest` guarded OAuth server
**Target Platform**: Linux server / containerized Tianji deployment with possible multiple pods
**Performance Goals**: 5-minute job tick; zero upstream calls for credentials outside 30-minute buffer; one upstream refresh per credential under concurrent refresh attempts
**Constraints**: Todo phase is docs-only; implementation must start with failing tests; no real OpenAI calls; no secrets in logs/metadata/UI/evidence
**Scale/Scope**: Active OpenAI subscription credentials stored in DB and referenced by model routes

## Constitution Check

| Principle              | Status | Notes                                                                                                                         |
| ---------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------- |
| Python-first reference | PASS   | This is Tianji-Go OpenAI subscription lifecycle behavior; no Python parity source owns this stored-credential scheduler path. |
| Feature parity         | PASS   | Does not alter external OpenAI-compatible request/response contracts.                                                         |
| Research before build  | PASS   | research.md records repo evidence plus Context7/sqlc/redsync/singleflight and Go docs evidence.                               |
| Failing-tests-first    | PASS   | Failing tests are specified below and tasks start each story with tests.                                                      |
| Go idioms              | PASS   | Uses existing scheduler/job interfaces, context cancellation, and handler helpers.                                            |
| No stale knowledge     | PASS   | Technical API claims are backed by repo reads and external docs.                                                              |
| sqlc-first DB access   | PASS   | New DB access must be a named sqlc query in `internal/db/queries/credential.sql`.                                             |

## Project Structure

### Documentation

```text
specs/HO-2091-proactively-refresh-idle-openai-subscription/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── analyze.md
├── scope-confirmation.md
├── contracts/
│   └── proactive-refresh.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
cmd/tianji/main.go
internal/db/queries/credential.sql
internal/db/interface.go
internal/db/credential.sql.go
internal/scheduler/jobs.go
internal/scheduler/jobs_test.go
internal/proxy/handler/openai_subscription_refresh.go
internal/proxy/handler/openai_subscription_refresh_test.go
internal/proxy/handler/openai_subscription_routing.go
internal/proxy/handler/openai_subscription_routing_test.go
internal/proxy/handler/credentials.go
internal/proxy/handler/openai_subscription_lifecycle.go
internal/ui/handler_credentials.go
internal/ui/handler_models.go
internal/ui/pages/*.templ
internal/ui/pages/*_templ.go
```

**Structure Decision**: Place candidate scanning and scheduling in `internal/scheduler`, but keep token decrypt/refresh/classification behavior in `internal/proxy/handler` or a small interface owned by handler so the proactive job reuses existing refresh and redaction semantics instead of duplicating credential logic.

## Design Decisions

### Separate proactive buffer from request-time freshness

Use a new 30-minute proactive candidate buffer. Do not change `openAISubscriptionRefreshBuffer = 5 * time.Minute`; request-time resolution keeps the current fresh-token rule.

### Register as DB-backed scheduler job

Register `OpenAISubscriptionProactiveRefreshJob` from `cmd/tianji/main.go` only when `queries != nil`. Cadence is 5 minutes. Do not run on startup unless implementation explicitly proves startup catch-up is needed and tests cover it; the Linear note says run every 5 minutes.

### Locking

Use two layers:

1. Existing `singleflight.Group` path for same-process and request-time/proactive duplicate suppression.
2. Existing Redis scheduler `NewWithLock` when Redis is available to avoid cross-pod job stampede.

If Redis is not configured, the job still works in a single pod and each credential reloads state before refresh; multi-pod no-stampede requires the Redis lock or a DB claim query. Implementation must not silently claim cross-pod protection without one of those mechanisms.

### Candidate selection

Add sqlc query to list active `openai_subscription` rows excluding obvious non-selectable metadata (`disabled`, `refresh_failed`, reconnect-required/deleted). Decrypt in Go to inspect `expires_at`; skip if outside `now + 30m`.

### Invalidated session classification

Classify upstream text/code containing `refresh_token_invalidated` as terminal reconnect-required. Persist the safe operator message `OpenAI session ended; reconnect required`, first/last failure timestamps, and stable reason. Mark it non-selectable for routing.

### Routing/stale route references

Update candidate loading to reject `refresh_failed` and reconnect-required states, not only `disabled`. Surface stale IDs in model UI/log diagnostics or auto-prune them through a dedicated, tested path. Do not mutate model routes silently without clear operator-visible evidence.

## Failing Tests

### User Story 1 Tests

| Test Function                                                                          | File                                                                           | Assertion                                                                                | Covers  |
| -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ------- |
| `TestOpenAISubscriptionProactiveRefreshJob_RefreshesIdleCredentialInsideBuffer`        | `internal/proxy/handler/openai_subscription_refresh_test.go` or scheduler test | Credential expiring in 20m triggers one mock refresh and stores new access token/expiry. | US1 AS1 |
| `TestOpenAISubscriptionProactiveRefreshJob_SkipsCredentialOutsideBuffer`               | same                                                                           | Credential expiring in 45m causes zero token endpoint requests and zero writes.          | US1 AS2 |
| `TestOpenAISubscriptionProactiveRefreshJob_RegisteredEveryFiveMinutesWhenDBConfigured` | `cmd/tianji`-adjacent test or scheduler wiring test                            | DB-backed startup registers the job with 5-minute cadence.                               | US1 AS3 |

### User Story 2 Tests

| Test Function                                                                       | File                                                         | Assertion                                                                                         | Covers      |
| ----------------------------------------------------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------- | ----------- |
| `TestOpenAISubscriptionProactiveRefreshJob_SharesSingleflightWithRequestResolution` | `internal/proxy/handler/openai_subscription_refresh_test.go` | Blocking proactive refresh and route resolution for same credential produce one upstream refresh. | US2 AS2     |
| `TestOpenAISubscriptionProactiveRefreshJob_DistributedLockSkipsSecondRunner`        | `internal/scheduler/jobs_test.go` or lock wrapper test       | Second concurrent job instance reports locked/skipped and does not refresh.                       | US2 AS1/AS3 |

### User Story 3 Tests

| Test Function                                                                             | File                                                         | Assertion                                                                                                              | Covers      |
| ----------------------------------------------------------------------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- | ----------- |
| `TestOpenAISubscriptionProactiveRefreshJob_RefreshTokenInvalidatedMarksReconnectRequired` | `internal/proxy/handler/openai_subscription_refresh_test.go` | `401 refresh_token_invalidated` persists first failure timestamp, reason, operator message, and non-selectable status. | US3 AS1/AS2 |
| `TestOpenAISubscriptionProactiveRefreshJob_TransientFailureDoesNotClaimReconnectRequired` | same                                                         | 5xx/network failure stores redacted transient failure without reconnect-required reason.                               | US3 AS3     |
| `TestOpenAISubscriptionProactiveRefreshJob_SuccessClearsTransientFailureMetadata`         | same                                                         | Success after transient failure clears `last_error`/failure metadata and restores active status.                       | FR-008      |

### User Story 4 Tests

| Test Function                                                                | File                                                                           | Assertion                                                                                              | Covers  |
| ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------ | ------- |
| `TestResolveOpenAISubscriptionCandidates_RefreshFailedExcluded`              | `internal/proxy/handler/openai_subscription_routing_test.go`                   | `status=refresh_failed` candidate is skipped with stable reason.                                       | US4 AS1 |
| `TestModelsUI_OpenAISubscriptionStaleReferencesVisible`                      | `internal/ui/handler_models_test.go`                                           | Missing/deleted/failed credential IDs appear as stale route references without secrets.                | US4 AS2 |
| `TestOpenAISubscriptionRouting_AllFailedCandidatesReturnsClearUnusableError` | `internal/proxy/handler/openai_subscription_routing_test.go`                   | All non-selectable candidates return sanitized unusable-credential error and no sticky freshness loop. | US4 AS3 |
| `TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets`             | `internal/proxy/handler/openai_subscription_lifecycle_test.go` or refresh test | access/refresh/JWT/bearer/encrypted-looking strings are absent from metadata/loggable payloads.        | FR-015  |

### Verification Command

```bash
go test ./internal/proxy/handler/... ./internal/scheduler/... ./internal/ui/... -run 'TestOpenAISubscriptionProactive|TestResolveOpenAISubscriptionCandidates_RefreshFailedExcluded|TestModelsUI_OpenAISubscriptionStaleReferencesVisible|TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets' -count=1 -v
```

## Implementation Phases

### Phase 1: Test Harness And Candidate Query

Add failing tests and sqlc query for candidate listing. Regenerate sqlc before implementation code uses the query.

### Phase 2: Proactive Refresh Job

Add job result struct and job runner that scans candidates, decrypts expiry, skips outside buffer, and calls the existing refresh path for eligible credentials.

### Phase 3: Locking Integration

Wire same-process `singleflight` through the existing handler refresh method and wrap scheduler job with Redis distributed lock when available.

### Phase 4: Failure Classification

Add reconnect-required classification for `refresh_token_invalidated`, first failure timestamp preservation, transient failure handling, and success cleanup.

### Phase 5: Routing And UI Visibility

Exclude `refresh_failed` / reconnect-required from routing. Surface stale route references in existing model/credential UI or explicit route diagnostics.

### Phase 6: Regression Verification

Run targeted tests, affected package tests, sqlc generation, templ generation if UI templates change, and diff checks.

## Risk Register

| Risk                                                  | Mitigation                                                                                                               |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Proactive job refreshes every credential every tick   | Candidate filter and expiry decrypt check enforce 30-minute buffer.                                                      |
| Multi-pod stampede remains                            | Use existing Redis distributed lock or implement/test DB claim semantics; do not rely only on in-process `singleflight`. |
| Request-time freshness behavior changes               | Keep existing 5-minute constant and add separate proactive constant/tests.                                               |
| Refresh failure disables recoverable transient errors | Only `refresh_token_invalidated` becomes reconnect-required; transient failures stay distinct.                           |
| Secret leakage in metadata/logs                       | Reuse redaction helpers and add explicit redaction tests.                                                                |
| UI silently prunes route references                   | Prefer visible stale reference reporting; auto-prune only if tested and operator-visible.                                |

## Plan Review Evidence

- Context7 `/websites/pkg_go_dev_golang_org_x_sync`: `singleflight.Group.Do` suppresses duplicate in-flight calls per key.
- Context7 `/go-redsync/redsync`: `NewMutex`, `WithExpiry`, `WithTries`, `LockContext`, and `UnlockContext` support the existing distributed lock wrapper.
- Context7 `/websites/sqlc_dev_en`: sqlc named queries generate Go methods and params structs from `-- name:` SQL comments.
- Official Go docs: `time` package supports ticker-based periodic work; repo already centralizes this in `internal/scheduler`.
- grep.app public code search was blocked by Vercel checkpoint HTML, so no external implementation pattern was adopted.

> Plan reviewed via context7 / grep-app / Google — revisions applied: use repo scheduler, sqlc query, existing `singleflight`, and existing Redis distributed lock wrapper; no unsupported external pattern adopted.
