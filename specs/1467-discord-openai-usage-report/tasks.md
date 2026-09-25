---

description: "Task list for the Discord OpenAI credential usage report"
---

# Tasks: Discord OpenAI Credential Usage Report

**Input**: Design documents from `/specs/1467-discord-openai-usage-report/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`,
`contracts/discord-webhook-report.md`, and `quickstart.md`

**Tests**: Required by the feature specification and the constitution's
failing-tests-first principle. Every user-story phase starts by adding named
tests and observing their expected failure before the corresponding source
change.

**Organization**: Tasks are grouped by user story. The report assembly and
sender are deliberately narrow package-local additions; no database, UI,
configuration, proxy-routing, OAuth, or generic notification work is added.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because its files and prerequisites do not
  overlap.
- **[Story]**: User-story traceability label; setup, foundational, and
  cross-cutting tasks omit it.
- Every task names an exact repository-relative file or command scope and
  cites its applicable requirements.

## Phase 1: Setup (Repository Baseline)

**Purpose**: Reconfirm the existing reuse boundary before production files are
changed.

- [X] T001 Record the implementation baseline with `rtk git status --short --branch`, `rtk git rev-parse HEAD`, and focused reads of `internal/proxy/handler/openai_subscription_codex_usage.go`, `internal/ui/handler_codex_usage.go`, `internal/scheduler/scheduler.go`, `internal/scheduler/lock.go`, `internal/scheduler/jobs.go`, `internal/callback/discord_ratelimit.go`, and `cmd/tianji/main.go`; preserve unrelated worktree artifacts (FR-010, FR-012).
- [X] T002 [P] Re-run `rtk ccc search --refresh "OpenAI subscription Codex usage snapshot Discord webhook scheduler distributed lock"` and inspect the existing graph query for `OpenAISubscriptionCodexUsageSnapshot` and `DistributedLock`; confirm the implementation reuses those boundaries and adds no database/configuration layer (FR-001, FR-002, FR-007, FR-011).

---

## Phase 2: Foundational (Test and Contract Gate)

**Purpose**: Establish the existing test seams and immutable outbound contract
before any user-story implementation.

- [X] T003 Confirm the fake usage-fetcher, credential-store, `httptest`, and `miniredis` seams in `internal/proxy/handler/openai_subscription_codex_usage_test.go`, `internal/callback/discord_ratelimit_test.go`, and `internal/scheduler/jobs_test.go`; record the RED-test commands from `specs/1467-discord-openai-usage-report/quickstart.md` without using a real OpenAI credential or Discord webhook (FR-012).
- [X] T004 Confirm `specs/1467-discord-openai-usage-report/contracts/discord-webhook-report.md` remains the implementation boundary: plain text only, final `content` at most 2,000 characters, `wait=true`, `allowed_mentions.parse=[]`, exactly one retry only for confirmed `429`, and no raw response/error or secret-bearing diagnostic data (FR-007, FR-008, FR-009, FR-010, FR-011).

**Checkpoint**: Existing reuse points, test seams, and the redacted Discord
contract are fixed; no runtime behavior has changed.

---

## Phase 3: User Story 1 - Receive a recurring credential-usage report (Priority: P1) 🎯 First executable slice

**Goal**: Assemble an ordered, safe report for every OpenAI subscription
credential and deliver its chunks through the existing Discord destination.

**Independent Test**: Seed several `openai_subscription` credentials, including
additional usage buckets and enough rows to require chunking; inject a local
`httptest` webhook and verify every credential ID appears exactly once in
ordered plain-text report content, an empty inventory produces a zero-credential
report, the request contract is correct, and all parts are accepted within the
60-second report context.

### Tests for User Story 1

- [X] T005 [US1] Add the initially failing `TestOpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets` in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`; cover deterministic credential-ID ordering, one row per seeded `openai_subscription` credential, plan/primary/weekly/reset/additional-bucket rendering, the operational-snapshot label, a zero-credential report, and row-boundary splitting for a 50-row fixture (FR-001, FR-002, FR-004, FR-005, FR-008, FR-012; SC-001, SC-002).
- [X] T006 [P] [US1] Add the initially failing `TestDiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure` in `internal/callback/discord_usage_report_test.go`; use `httptest` and captured logger output to assert `wait=true`, `allowed_mentions.parse=[]`, final content no longer than 2,000 characters, ordered `part n/m` prefixes, responsive local delivery within a 60-second report context, and redaction of token-shaped fixture values from safe outcomes and logs (FR-007, FR-008, FR-009, FR-012; SC-002, SC-003).
- [X] T007 [US1] Run `rtk go test ./internal/proxy/handler -run '^TestOpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets$' -count=1` and confirm the new handler test fails before modifying `internal/proxy/handler/openai_subscription_discord_usage_report.go` (FR-001, FR-004, FR-005, FR-008; SC-001, SC-002).
- [X] T008 [US1] Run `rtk go test ./internal/callback -run '^TestDiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure$' -count=1` and confirm the new sender test fails before modifying `internal/callback/discord_usage_report.go` (FR-007, FR-008, FR-009; SC-002, SC-003).

### Implementation for User Story 1

- [X] T009 [US1] Add the handler-owned report result, safe row collection, deterministic credential-ID ordering, operational-snapshot text rendering, zero-credential report header, and row-boundary chunking in `internal/proxy/handler/openai_subscription_discord_usage_report.go`; list with the existing credential store, filter `openai_subscription`, and call only `OpenAISubscriptionCodexUsageSnapshot(ctx, credentialID, false)` for usage data (FR-001, FR-002, FR-004, FR-005, FR-008).
- [X] T010 [US1] Add the small Discord webhook sender in `internal/callback/discord_usage_report.go`; accept already-rendered report parts, use the existing configured URL, send plain-text JSON with `wait=true` and `allowed_mentions.parse=[]`, enforce the final 2,000-character bound, and return only sanitized part/outcome data (FR-007, FR-008, FR-009).
- [X] T011 [US1] Run the focused US1 checks in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go` and `internal/callback/discord_usage_report_test.go` with `rtk go test ./internal/proxy/handler ./internal/callback -run 'Test(OpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets|DiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure)' -count=1`; confirm the previously RED tests pass without real network credentials (FR-001, FR-004, FR-005, FR-007, FR-008, FR-009, FR-012; SC-001, SC-002, SC-003).

**Checkpoint**: A test-invoked report can safely enumerate and deliver the
requested current-usage view, including ordered multi-part content. Scheduler
registration is intentionally deferred to User Story 3.

---

## Phase 4: User Story 2 - See safe unavailable and stale credential states (Priority: P1)

**Goal**: Ensure that every inventory entry remains visible as a safe status
row without refreshing, reviving, or exposing sensitive credential data.

**Independent Test**: Seed active, disabled, reconnect-required, stale,
backoff, unavailable, malformed, and deletion-race fixture states; run report
assembly and verify each retained credential has a safe row, non-selectable
states do not invoke the usage fetcher, and all known secret-bearing fixture
values are absent from content and outcomes.

### Tests for User Story 2

- [X] T012 [US2] Add the initially failing `TestOpenAISubscriptionDiscordUsageReport_UsesSafeStatusWithoutUpstreamForNonSelectableCredential` in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`; cover disabled, reconnect-required, stale, backoff, unavailable, malformed-metadata, and post-enumeration-deletion cases, assert no usage-fetcher invocation for non-selectable credentials, and assert redaction of token, email, credential-name, organization, raw-error, and webhook fixture values (FR-003, FR-006, FR-009, FR-012; SC-003).
- [X] T013 [US2] Run `rtk go test ./internal/proxy/handler -run '^TestOpenAISubscriptionDiscordUsageReport_UsesSafeStatusWithoutUpstreamForNonSelectableCredential$' -count=1` and confirm it fails before extending `internal/proxy/handler/openai_subscription_discord_usage_report.go` (FR-003, FR-006, FR-009; SC-003).

### Implementation for User Story 2

- [X] T014 [US2] Extend safe status mapping and redacted rendering in `internal/proxy/handler/openai_subscription_discord_usage_report.go` so disabled, reconnect-required, stale, backoff, unavailable, malformed, and deletion-race results produce a status row without credential lifecycle mutation, direct token access, an upstream-client call outside the reused snapshot path, or raw diagnostic output (FR-003, FR-006, FR-009, FR-010).
- [X] T015 [US2] Run both handler report tests in `internal/proxy/handler/openai_subscription_discord_usage_report_test.go` with `rtk go test ./internal/proxy/handler -run '^TestOpenAISubscriptionDiscordUsageReport_' -count=1`; confirm the previously RED status test passes and normal proxy/routing source files remain untouched (FR-001, FR-003, FR-004, FR-006, FR-009, FR-010, FR-012; SC-001, SC-003, SC-004).

**Checkpoint**: The report preserves a complete, safe operational inventory
even when usage cannot be fetched; it has not changed credential behavior or
proxy traffic.

---

## Phase 5: User Story 3 - Keep periodic reporting operationally isolated (Priority: P2)

**Goal**: Schedule the report every 30 minutes without a startup send, isolate
delivery failures from proxy traffic, and coordinate the job across replicas
using the existing lock when Redis is present.

**Independent Test**: Use a missing-config fixture, failing local webhook, and
two concurrent `miniredis`-backed job runners; verify a failed or skipped
report changes no proxy/credential result, a confirmed `429` receives at most
one in-budget retry, other failures are not retried, and only the lock holder
starts a report for the period.

### Tests for User Story 3

- [X] T016 [US3] Add the initially failing `TestDiscordUsageReportSender_RetriesOnlyWithinBoundedPolicy` in `internal/callback/discord_usage_report_test.go`; verify missing configuration skips safely, only a confirmed `429` with an in-budget `retry_after` receives one retry, transport/5xx/other non-2xx outcomes receive no retry, and all failures are sanitized (FR-007, FR-009, FR-010, FR-011, FR-012; SC-003, SC-004).
- [X] T017 [P] [US3] Add the initially failing `TestOpenAISubscriptionDiscordUsageReportJob_RunAndDistributedLock` in `internal/scheduler/jobs_test.go` and the reused-scheduler regression `TestScheduler_AddDoesNotRunAtStartup` in `internal/scheduler/scheduler_test.go`; use `miniredis`, a report closure, and captured logger output to verify the stable job name, the 30-minute interval constant, a closure deadline no later than the 60-second report budget, safe failure return/logging, only one of two locked runners starts delivery for a reporting period, and the existing `Add` path has no startup run (FR-002, FR-009, FR-010, FR-011, FR-012; SC-002, SC-003, SC-004, SC-005).
- [X] T018 [US3] Run the new RED checks with `rtk go test ./internal/callback ./internal/scheduler -run 'Test(DiscordUsageReportSender_RetriesOnlyWithinBoundedPolicy|OpenAISubscriptionDiscordUsageReportJob_RunAndDistributedLock|Scheduler_AddDoesNotRunAtStartup)' -count=1` and confirm both packages fail before changing `internal/callback/discord_usage_report.go`, `internal/scheduler/jobs.go`, `internal/scheduler/scheduler_test.go`, or `cmd/tianji/main.go` (FR-002, FR-007, FR-010, FR-011; SC-004, SC-005).

### Implementation for User Story 3

- [X] T019 [US3] Extend the sender in `internal/callback/discord_usage_report.go` to honor the caller's report context and allow at most one retry only for a confirmed `429` whose `retry_after` fits the remaining deadline; keep transport, 5xx, and other non-2xx outcomes single-attempt and safe (FR-009, FR-010, FR-011; SC-002).
- [X] T020 [US3] Add the thin `OpenAISubscriptionDiscordUsageReportJob`, stable job name, `30*time.Minute` interval constant, and one 60-second child context around its report closure in `internal/scheduler/jobs.go`; log only safe counts/reason codes and leave later periods eligible after failure (FR-002, FR-010, FR-011; SC-002).
- [X] T021 [US3] Register the report job in `cmd/tianji/main.go` only when the existing database-backed credential access and `DiscordWebhookURL` are available; use `sched.Add` without startup delivery and wrap it with the existing Redis `scheduler.NewWithLock` path when Redis is configured, without adding configuration, database, routing, OAuth, or UI changes (FR-002, FR-007, FR-010, FR-011).
- [X] T022 [US3] Run the focused callback and scheduler tests in `internal/callback/discord_usage_report_test.go` and `internal/scheduler/jobs_test.go` with `rtk go test ./internal/callback ./internal/scheduler -run 'Test(DiscordUsageReportSender|OpenAISubscriptionDiscordUsageReportJob)' -count=1`; confirm both previously RED tests pass, no real Discord/OpenAI call occurs, and a delivery failure does not enter the proxy request path (FR-002, FR-007, FR-009, FR-010, FR-011, FR-012; SC-003, SC-004, SC-005).

**Checkpoint**: The requested recurring job is wired through existing
composition and coordination only; delivery remains explicitly best-effort.

---

## Phase 6: Polish & Cross-Cutting Validation

**Purpose**: Prove the complete narrow slice and prepare a controlled rollout
without inventing durable delivery machinery.

- [X] T023 [P] Run the focused feature validation from `specs/1467-discord-openai-usage-report/quickstart.md` with `rtk go test ./internal/proxy/handler ./internal/callback ./internal/scheduler -run 'Test(OpenAISubscriptionDiscordUsageReport|DiscordUsageReportSender|OpenAISubscriptionDiscordUsageReportJob)' -count=1`; verify all FR-012 scenarios and SC-001 through SC-005 without a real credential or webhook.
- [X] T024 [P] Run affected-package regression checks for `internal/proxy/handler`, `internal/callback`, and `internal/scheduler`, then the applicable `rtk make lint`, `rtk make test`, and `rtk make build` gates; record exact unavailable/failing commands rather than claiming a pass (FR-010, FR-012).
- [X] T025 Re-run `rtk ccc index`, `rtk rg -n 'DiscordWebhookURL|openai_subscription_discord_usage_report|OpenAISubscriptionCodexUsageSnapshot' internal/ cmd/tianji/main.go`, `rtk git diff --check`, and `rtk git diff --stat`; confirm the changed runtime paths are limited to `internal/proxy/handler/openai_subscription_discord_usage_report.go`, `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`, `internal/callback/discord_usage_report.go`, `internal/callback/discord_usage_report_test.go`, `internal/scheduler/jobs.go`, `internal/scheduler/jobs_test.go`, `internal/scheduler/scheduler_test.go`, and `cmd/tianji/main.go` (FR-001 through FR-012).
- [ ] T026 Before enabling the job outside automated tests, execute the non-secret rollout checks in `specs/1467-discord-openai-usage-report/quickstart.md`: verify actual replica count, database-backed credentials, Redis availability for a multi-replica deployment, and a non-empty resolved existing Discord webhook setting without printing its value; use an approved non-production destination for the first smoke report (FR-002, FR-007, FR-009, FR-011).

## Requirement Coverage

| Requirement | Covered by |
|-------------|------------|
| FR-001 | T005, T009, T011, T015 |
| FR-002 | T005, T009, T017, T020, T021, T022, T026 |
| FR-003 | T012, T014, T015 |
| FR-004 | T005, T009, T011 |
| FR-005 | T005, T009, T011 |
| FR-006 | T012, T014, T015 |
| FR-007 | T004, T006, T008, T010, T016, T021, T022, T026 |
| FR-008 | T004, T005, T006, T009, T010, T011 |
| FR-009 | T004, T006, T010, T012, T014, T016, T019, T022, T026 |
| FR-010 | T001, T004, T014, T015, T016, T017, T019, T020, T021, T022, T024 |
| FR-011 | T004, T016, T017, T019, T020, T021, T022, T026 |
| FR-012 | T001, T003, T005, T006, T011, T012, T015, T016, T017, T022, T023, T024 |
| SC-001 | T005, T011, T015, T023 |
| SC-002 | T005, T006, T007, T008, T011, T019, T023 |
| SC-003 | T006, T011, T012, T015, T016, T022, T023 |
| SC-004 | T015, T016, T017, T018, T022, T023 |
| SC-005 | T017, T018, T022, T023, T026 |

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No code dependency; reconfirms the bounded reuse plan.
- **Foundational (Phase 2)**: Depends on Setup; fixes the test and outbound
  contract boundary before runtime files change.
- **User Story 1 (Phase 3)**: Depends on Foundational; T005/T006 and their RED
  checks precede T009/T010.
- **User Story 2 (Phase 4)**: Depends on US1 because it extends the same
  handler-owned report file; T012/T013 precede T014.
- **User Story 3 (Phase 5)**: Depends on US1's sender and US2's safe report
  outcome; T016/T017/T018 precede T019 through T021.
- **Polish (Phase 6)**: Depends on completed user-story work and validates the
  declared narrow change boundary.

### User Story Dependencies

- **User Story 1 (P1)**: First executable slice; test-invoked safe report
  assembly and local webhook delivery.
- **User Story 2 (P1)**: Follows US1 to avoid concurrent edits to
  `internal/proxy/handler/openai_subscription_discord_usage_report.go`; it
  adds the unsafe/unavailable state coverage required for a truthful report.
- **User Story 3 (P2)**: Follows US1/US2 because it composes their report
  result; it is required before the requested periodic production behavior is
  enabled.

### Parallel Opportunities

- T002 can run while T001's source reads are underway.
- T005 and T006 write tests in distinct packages; their RED commands T007 and
  T008 can run after their respective test additions.
- T016 and T017 write tests in distinct packages before T018.
- T023 and T024 can run in parallel after T022; T025 remains the final source
  scope check.

## Parallel Example: User Story 1

```bash
# These create tests in distinct packages; run their RED checks separately
# before their corresponding source file is implemented.
rtk go test ./internal/proxy/handler \
  -run '^TestOpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets$' \
  -count=1
rtk go test ./internal/callback \
  -run '^TestDiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure$' \
  -count=1
```

## Parallel Example: User Story 3

```bash
# The sender and scheduler tests are package-isolated and can be run together
# after their tests are added, before implementation.
rtk go test ./internal/callback ./internal/scheduler \
  -run 'Test(DiscordUsageReportSender_RetriesOnlyWithinBoundedPolicy|OpenAISubscriptionDiscordUsageReportJob_RunAndDistributedLock)' \
  -count=1
```

## Implementation Strategy

### First executable slice

1. Complete the Phase 1 and Phase 2 evidence gates.
2. Add and observe the US1 RED tests.
3. Implement safe assembly and test-only local webhook delivery.
4. Stop and validate ordered, redacted report content.

This slice proves report correctness, but the requested recurring runtime
behavior is not enabled until User Story 3 wires the scheduler and lock.

### Incremental Delivery

1. Complete US1 → validate safe current snapshots and ordered delivery.
2. Complete US2 → validate truthful unavailable/stale status rows and
   redaction.
3. Complete US3 → validate 30-minute scheduling, bounded delivery failure, and
   multi-replica lock behavior.
4. Complete Phase 6 → run quality gates and controlled rollout checks.

## Notes

- No task adds a package dependency, database migration/query, durable outbox,
  second Discord setting, generic notification framework, scheduler alignment,
  UI setting, proxy route, or OAuth lifecycle behavior.
- Discord `429` is the only retryable result. An ambiguous external delivery
  outcome stays best-effort and is never described as exactly-once.
- Do not print a webhook URL or any secret-bearing fixture value in test
  failures, logs, report text, or rollout evidence.
