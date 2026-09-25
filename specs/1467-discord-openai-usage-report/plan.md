# Implementation Plan: Discord OpenAI Credential Usage Report

**Branch**: `1467-discord-openai-usage-report` | **Date**: 2026-08-14 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from
`/specs/1467-discord-openai-usage-report/spec.md`

## Summary

Add a best-effort, every-30-minute operational report that sends the existing
safe Codex usage snapshot for every `openai_subscription` credential to the
already configured Discord destination. Reuse the current snapshot/cache and
the scheduler's existing distributed-lock wrapper; add only a small report
job, report renderer, and Discord webhook sender. Do not add a database
schema, an outbox, a second webhook setting, a generic notification framework,
or wall-clock scheduler support.

## Technical Context

**Language/Version**: Go 1.26.0 (`go.mod`)

**Primary Dependencies**: Existing standard-library `net/http`, `encoding/json`,
and `time`; existing Tianji scheduler, OpenAI subscription snapshot handler,
redaction package, and Redis-backed scheduler lock. No new dependency.

**Storage**: Existing `CredentialTable` and existing Codex usage-cache
metadata only; no migration, new query, durable outbox, or new persisted
report record.

**Testing**: Go `httptest`, existing handler mock store/fake usage fetcher,
`miniredis`, focused `go test`, then applicable `make` quality gates.

**Target Platform**: Tianji server process on its existing Linux/container
deployment target.

**Project Type**: Go proxy web service with background scheduler jobs.

**Performance Goals**: A report with up to 50 cached or mock-backed credential
rows is assembled and accepted by a responsive Discord webhook within 60
seconds of the scheduled job starting.

**Constraints**: Run every 30 minutes after scheduler start; cap each report
run at 60 seconds; do not fetch in the proxy request hot path; preserve
snapshot cache/backoff and credential refresh behavior; prevent accidental
mentions; redact all secrets; cap webhook retries; ordinary proxy traffic
remains independent.

**Scale/Scope**: One existing Discord destination, all OpenAI subscription
credentials visible to the current server, ordered multi-message reports when
the content exceeds Discord's per-message limit, and a single logical runner
per reporting period in a coordinated multi-replica deployment.

## Constitution Check

### Pre-Research Gate

| Principle | Applicability | Plan Response |
|-----------|---------------|---------------|
| I. Python-First Reference | Not a Python parity surface | No Python implementation is changed; current Go behavior and existing specs are the source of truth. |
| II. Feature Parity | Existing configuration and proxy behavior must remain stable | Reuse `discord_webhook_url`; do not alter OpenAI-compatible routes, routing, OAuth, or credential lifecycle. |
| III. Evidence-Based Research | External Discord webhook contract and current repository integration are material | Research records current Discord primary-source documentation via Context7 plus repository source evidence. |
| IV. Failing-Tests-First Development | User-visible delivery/status behavior changes | The first task in every user-story phase adds named failing tests before implementation. |
| V. Idiomatic Go & Simplicity | Directly applicable | Use existing scheduler, lock, snapshot/cache, redaction, and standard library. No interfaces, database outbox, or package additions without a second need. |
| VI. Verified, Current Decisions | Deployment coordination and webhook configuration are mutable | Plan separates static repository evidence from the required pre-rollout live checks for replica count, Redis, and resolved webhook configuration. |
| VII. SQL-First Database Access | No new database behavior | Reuse `ListCredentials`; no SQL or generated code changes. |
| VIII. Standards-First Compatibility | No public Tianji API is added | The only external contract is an outbound Discord webhook request documented in `contracts/`. |
| IX. Secure-by-Default Operations | Credential and webhook data are trust-boundary material | Send only safe snapshot fields, block mentions, redact errors, never log the webhook URL, and keep delivery failures off the proxy path. |

**Gate result**: PASS. No exception or complexity justification is required.

### Post-Design Gate

The Phase 1 design preserves the pre-research decisions: no schema or public
API, no new dependency, test-first tasks, and a redacted outbound-only
contract. **PASS.**

## Failing Tests

Implementation starts by adding the following tests and confirming they fail
before the corresponding behavior is added:

1. `TestOpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets`
   in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`
   — verifies one safe row per listed subscription credential with primary,
   weekly, plan, reset, and additional-bucket data.
2. `TestOpenAISubscriptionDiscordUsageReport_UsesSafeStatusWithoutUpstreamForNonSelectableCredential`
   in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`
   — verifies disabled/reconnect-required rows do not trigger an upstream
   usage request and do not expose credential material.
3. `TestDiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure`
   in `internal/callback/discord_usage_report_test.go`
   — verifies `wait=true`, `allowed_mentions.parse=[]`, ordered messages
   under the Discord content limit, and sanitized failures/log output.
4. `TestDiscordUsageReportSender_RetriesOnlyWithinBoundedPolicy`
   in `internal/callback/discord_usage_report_test.go`
   — verifies one bounded retry for a confirmed rate limit and no retry loop
   for ambiguous transport failures.
5. `TestOpenAISubscriptionDiscordUsageReportJob_RunAndDistributedLock`
   in `internal/scheduler/jobs_test.go`
   — verifies the job invokes the report closure and a second locked runner
   skips without sending.

## Design

### Reused boundaries

- `internal/proxy/handler/openai_subscription_codex_usage.go` already provides
  `OpenAISubscriptionCodexUsageSnapshot`, per-credential cache/backoff,
  safe normalized snapshots, and singleflight refresh behavior.
- `internal/ui/handler_codex_usage.go` already proves the correct inventory
  pattern: `ListCredentials`, filter `openai_subscription`, then retrieve a
  safe snapshot for each credential.
- `internal/scheduler/scheduler.go` already runs a job at a fixed interval;
  it is intentionally process-start-relative. The feature uses `Add`, not
  `AddWithStartupRun` or a new aligned scheduler.
- `internal/scheduler/lock.go` already wraps a job with a Redis distributed
  lock and skips a second runner. The report uses that wrapper in the same
  way as proactive OpenAI credential refresh.
- `internal/config/config.go` and `cmd/tianji/main.go` already carry
  `DiscordWebhookURL` into the running process. This feature does not add a
  second configuration field or change secret resolution.

### Report flow

1. `cmd/tianji/main.go` registers an
   `openai_subscription_discord_usage_report` scheduler job only when both
   database-backed credentials and the existing Discord webhook setting are
   available.
2. The job runs every `30*time.Minute` without an immediate startup report.
   When Redis is configured, it is wrapped with the existing
   `scheduler.NewWithLock`; multi-replica rollout requires that existing
   coordination path. A no-Redis rollout is validated as single-replica
   before enabling this feature.
3. A small handler-owned report method lists credentials through the existing
   `ListCredentials` store method, filters `openai_subscription`, and orders
   rows deterministically by credential ID.
4. For every row, it uses the existing
   `OpenAISubscriptionCodexUsageSnapshot(ctx, id, false)` path. It never
   performs a separate token decrypt, refresh, or direct upstream request.
   Existing non-selectable and cache/backoff outcomes are mapped to safe
   report statuses.
5. The method renders only credential ID, plan, primary/weekly/additional
   bucket usage, reset values, snapshot state, and a redacted status reason.
   It omits email, credential name, and organization data and labels the
   result as an operational snapshot, not billing data. An empty credential
   inventory renders a clearly labeled zero-credential report instead of
   silently skipping configured delivery.
6. It splits the rendered report on row boundaries into ordered message parts,
   keeping each final Discord `content` value within 2,000 characters after
   adding the report-period and part prefix.
7. A small callback-package sender posts each message to the configured
   webhook using `wait=true` and `allowed_mentions: {"parse":[]}`. It does
   not reuse the Anthropic alert cooldown or raw-response logging behavior.

### Delivery and failure policy

- Each scheduled run has a 60-second report context so a slow snapshot or
  webhook cannot stall scheduler shutdown or normal proxy traffic.
- The sender allows a second attempt only after a confirmed `429`. It uses the
  documented `retry_after` value only when it fits the remaining job budget;
  transport failures, `5xx`, and all other failures end that part's attempt
  and are reported through sanitized status only.
- Delivery is best-effort. A timeout after Discord accepted a request is not
  retried, so the feature does not add a database outbox or make a false
  exactly-once claim.
- A missing webhook, database error, malformed metadata, an empty credential
  inventory, a stale/backoff snapshot, or a failed delivery leaves proxy
  routing and credential state unchanged. A configured empty inventory still
  sends its zero-credential report; the next scheduled period remains
  eligible.
- Logs record job counts and safe reason codes only. They never include a
  webhook URL, webhook body, token, credential bundle, raw upstream error,
  email address, credential name, or organization identifier.

### Files and narrow change boundary

| Path | Change |
|------|--------|
| `internal/proxy/handler/openai_subscription_discord_usage_report.go` | Add the report row collection, safe rendering, row-boundary chunking, and report result; reuse the existing snapshot method and redaction. |
| `internal/proxy/handler/openai_subscription_discord_usage_report_test.go` | Add first-failing/then-passing coverage for rows, status states, redaction, ordering, and chunking. |
| `internal/callback/discord_usage_report.go` | Add the small outbound Discord usage-report sender with bounded retry and mention blocking. |
| `internal/callback/discord_usage_report_test.go` | Use `httptest` to verify request shape, retry limits, and sanitized failures/log output. |
| `internal/scheduler/jobs.go` | Add a thin report job and the 30-minute interval constant. |
| `internal/scheduler/jobs_test.go` | Add job-name/run and lock coverage. |
| `internal/scheduler/scheduler_test.go` | Add the fixed-interval/no-startup regression that guards the reused `Add` behavior. |
| `cmd/tianji/main.go` | Wire the job with the existing webhook setting, scheduler, and optional Redis lock. |

No files under `internal/db/schema/`, `internal/db/queries/`, generated
`internal/db/*.sql.go`, `internal/config/`, UI templates, proxy routing, or
OAuth lifecycle are changed.

## Implementation Phases

### Phase 0 — Research and design

Complete `research.md`, `data-model.md`, `contracts/discord-webhook-report.md`,
and `quickstart.md`; confirm repository evidence and current Discord webhook
contract before implementation.

### Phase 1 — Safe report assembly (US1 and US2)

Write the handler tests first. Implement the narrow report loop and renderer
using existing credential listing, safe snapshots, state mapping, deterministic
ordering, and chunking. No outbound delivery is needed to prove this phase.

### Phase 2 — Safe Discord delivery (US1 and US3)

Write sender tests first. Add webhook delivery with wait confirmation, blocked
mentions, 2,000-character-safe chunks, bounded retries, and sanitized errors.
Connect it to the handler report result.

### Phase 3 — Scheduled coordinated execution (US3)

Write scheduler tests first. Add the thin job, create one 60-second report
context around its closure, and register it from `main.go` at a 30-minute
cadence, wrapped by the existing Redis lock when available. Do not change the
scheduler framework or add a startup send. The job's context stays below the
existing 30-minute lock lifetime.

### Phase 4 — Focused validation and rollout readiness

Run the focused tests, full affected-package tests, formatter/linter/build
gates, and the quickstart validation. Before production enablement, inspect
the owning runtime for actual replica count, Redis availability, and a resolved
webhook setting without printing the value.

## Project Structure

### Documentation (this feature)

```text
specs/1467-discord-openai-usage-report/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── discord-webhook-report.md
├── checklists/
│   └── requirements.md
└── tasks.md                  # Generated by $speckit-tasks
```

### Source Code (repository root)

```text
cmd/tianji/
└── main.go

internal/
├── callback/
│   ├── discord_usage_report.go
│   └── discord_usage_report_test.go
├── proxy/handler/
│   ├── openai_subscription_codex_usage.go
│   ├── openai_subscription_discord_usage_report.go
│   └── openai_subscription_discord_usage_report_test.go
└── scheduler/
    ├── jobs.go
    └── jobs_test.go
    └── scheduler_test.go
```

**Structure Decision**: Extend the current package ownership: the handler owns
credential/snapshot interpretation, callback owns the narrow outbound webhook
trust boundary, scheduler owns timed execution, and `main.go` composes existing
dependencies. No new service or repository layer is justified.

## Complexity Tracking

No constitution violation or additional complexity is required. The only
cross-pod coordination reuses the existing scheduler lock; durable delivery
tracking, a second configuration surface, and scheduler alignment are
deliberately deferred.
