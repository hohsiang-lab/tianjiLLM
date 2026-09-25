# Research: Discord OpenAI Credential Usage Report

**Date**: 2026-08-14
**Feature**: [spec.md](./spec.md)

## Decision 1 — Reuse the existing safe Codex usage snapshot

**Decision**: Build report rows from the existing
`OpenAISubscriptionCodexUsageSnapshot(ctx, credentialID, false)` path.

**Rationale**:

- It already normalizes primary 5-hour, weekly, additional-bucket, plan,
  status, reset, cache, and backoff information.
- It already reuses the OpenAI credential refresh path after an initial
  unauthorized usage request and preserves safe error states.
- Passing `manual=false` respects the existing cache/backoff behavior and
  avoids forcing an upstream refresh for every report.
- The existing UI uses `ListCredentials` plus that snapshot method to produce
  a credential-wide Codex view, so the inventory semantics are proven in the
  repository.

**Alternatives considered**:

- Query usage directly from a new reporting client — rejected because it would
  duplicate credential resolution, token handling, cache/backoff, and
  redaction risk.
- Use `SpendLogs` for token/cost totals — rejected because those are Tianji
  traffic attribution/API-equivalent values, not the requested subscription
  usage snapshot.
- Fetch usage during proxied model requests — rejected because HO-1340 and
  existing code keep the unofficial usage refresh off the proxy hot path.

**Evidence**:

- Repository: `internal/proxy/handler/openai_subscription_codex_usage.go`
  (`OpenAISubscriptionCodexUsageSnapshot` and cache/backoff path).
- Repository: `internal/ui/handler_codex_usage.go`
  (`ListCredentials` plus per-credential snapshot pattern).
- Specification: `specs/1340-add-codex-usage-snapshot-for-openai-subscription/spec.md`.

## Decision 2 — Reuse the existing scheduler and lock, not a new timing system

**Decision**: Register one fixed-interval scheduler job with
`30*time.Minute`; do not add startup delivery or `HH:00`/`HH:30` alignment.
Wrap it with the existing Redis distributed lock when the deployment has
Redis-backed coordination.

**Rationale**:

- `scheduler.Add` is the smallest existing mechanism for recurring jobs.
- The feature specification explicitly accepts process-start-relative
cadence.
- `scheduler.NewWithLock` already prevents a second runner from executing the
same named job across pods.
- Reusing that wrapper avoids a new leader-election, database-lock, or outbox
system.

**Alternatives considered**:

- New aligned scheduler API — rejected; exact wall-clock alignment is not a
first-version requirement.
- Database advisory lock or new report table — rejected; the existing
Redis lock solves the required coordinated-runner case without schema work.
- Run unconditionally on every replica — rejected; it creates report storms.

**Evidence**:

- Repository: `internal/scheduler/scheduler.go`.
- Repository: `internal/scheduler/lock.go`.
- Repository: `cmd/tianji/main.go` (existing proactive OpenAI refresh job
registration and optional Redis lock).
- Specification: `specs/HO-2091-proactively-refresh-idle-openai-subscription/spec.md`.

## Decision 3 — Keep Discord delivery small and safe

**Decision**: Send plain-text report chunks with the existing webhook URL using
`POST` with `wait=true`, `allowed_mentions: {"parse":[]}`, and a final content
size at or below 2,000 characters. Permit only one bounded retry after a
confirmed Discord rate-limit response.

**Rationale**:

- Discord's official webhook documentation describes message content up to
  2,000 characters, `allowed_mentions`, and `wait` confirmation behavior.
- Discord's official rate-limit documentation exposes a `retry_after` value
  for `429` responses, allowing a bounded retry without creating an arbitrary
  retry framework. Transport and `5xx` failures are left for the next
  half-hourly report to avoid duplicate sends after an ambiguous outcome.
- Blocking mentions is a required trust-boundary control because credential
  display data can contain user-controlled text. The report minimizes this
  surface further by sending credential IDs rather than email, credential
  name, or organization data.
- Plain text is sufficient for an operator report and avoids an embed-builder,
  pagination protocol, or a new dependency.

**Alternatives considered**:

- Reuse `DiscordRateLimitAlerter` directly — rejected; it is an Anthropic
threshold/cooldown implementation and logs raw webhook responses, which does
not meet this report's status, chunking, and redaction contract.
- Build a generic notification service — rejected; only one new sender is
needed.
- Add a durable outbox/exactly-once delivery protocol — rejected; report
  observability does not justify schema and delivery-state complexity. The
  feature documents best-effort delivery instead.

**Evidence**:

- Discord API Docs, resolved through Context7 on 2026-08-14:
  library `/discord/discord-api-docs`; “Execute Webhook” and “Rate Limits”
  documentation.
- Repository: `internal/callback/discord_ratelimit.go`.

## Decision 4 — Do not add a configuration or database surface

**Decision**: Reuse `DiscordWebhookURL` already loaded into `ProxyConfig` and
passed through `cmd/tianji/main.go`; use existing `CredentialTable` listing
only.

**Rationale**:

- A second webhook field, separate channel selector, or UI control is
speculative for one operator report.
- `ListCredentials` already supplies the credential inventory used by the
existing Codex usage UI.
- No report history is required for the requested half-hour status message.
- The report has deployment-operator scope: the configured channel must already
  be authorized for instance-wide operational status, and the report omits
  email, credential name, and organization data.

**Alternatives considered**:

- New report-specific webhook configuration — rejected; no separate
destination was requested.
- Database report/outbox tables — rejected; no durable audit/history feature
was requested.
- New SQL query limited to subscription credentials — rejected; filtering the
existing small credential inventory avoids an unnecessary schema/query/codegen
change.

**Evidence**:

- Repository: `internal/config/config.go`.
- Repository: `cmd/tianji/main.go`.
- Repository: `internal/db/queries/credential.sql`.

## Decision 5 — Make deployment coordination an explicit rollout check

**Decision**: Treat an existing Redis distributed lock as required for a
multi-replica rollout; a deployment without distributed coordination is
single-replica-only for this report.

**Rationale**:

- The current scheduler lock is optional at process composition time, so
static source alone cannot prove the running topology.
- Claiming exactly-once external webhook delivery would be false after an
ambiguous network outcome.
- A rollout check is smaller and safer than introducing new coordination
configuration or durable state.

**Alternatives considered**:

- Infer pod count in application code — rejected; deployment topology belongs
to the runtime owner and should be validated at rollout.
- Fail all service startup when Redis is absent — rejected; this feature must
not turn an optional observability report into a proxy availability dependency.

**Evidence**:

- Repository: `internal/scheduler/lock.go`.
- Repository: `cmd/tianji/main.go`.
- Constitution Principle VI, current mutable runtime decisions.
