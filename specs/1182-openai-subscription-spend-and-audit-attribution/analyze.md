# Analyze: HO-1182 SpecKit Consistency

**Date**: 2026-05-07
**Feature**: OpenAI subscription spend and audit attribution
**Artifacts**: spec.md, plan.md, research.md, data-model.md, quickstart.md, contracts/subscription-attribution.md, checklists/requirements.md, tasks.md

## Result

- Fatal issues: 0
- Critical issues: 0
- Owner input required: 0

## Cross-artifact Checks

### Scope Alignment

PASS. All artifacts scope HO-1182 to backend spend/callback/error attribution and audit coverage for OpenAI subscription credential lifecycle actions. UI, billing estimation, OAuth lifecycle, persistence, refresh manager, endpoint injection, routing/failover, quota parser/store, and shared redaction implementation remain out of scope.

### Linear Requirement Coverage

PASS.

- Attribute usage by `credential_id`/provider/org where available: covered by US1, FR-001 to FR-003, data model, contract, and tasks T001-T027.
- Audit connect/refresh/delete/test/disable actions with status: covered by US3, FR-007 to FR-009, contract, and tasks T028-T036.
- Ensure audit payloads contain no token/JWT/raw account payload: covered by US2/US3, FR-004, FR-009 to FR-010, data model redaction rules, and tasks T035/T037.
- Preserve existing spend/audit behavior for `api_key` traffic: covered by US4, FR-005 to FR-006, and tasks T008/T019/T026.

### Repo Reality Alignment

PASS. The plan references current TianjiLLM files that own this behavior:

- `internal/callback/callback.go`
- `internal/spend/tracker.go`
- `internal/db/queries/spend_logs.sql`
- `internal/proxy/handler/handler.go`
- `internal/proxy/handler/openai_subscription_resolution.go`
- `internal/proxy/handler/openai_subscription_routing.go`
- `internal/proxy/handler/openai_subscription_provider_retry.go`
- `internal/proxy/handler/audit_helper.go`
- `internal/proxy/handler/openai_oauth.go`

### External Evidence Alignment

PASS. Official OpenAI docs support bearer-auth handling, organization/project attribution, secret-management discipline, and the distinction between rate limits and usage/spend limits. The artifacts do not claim OpenAI exposes subscription monthly spend from response headers.

### Testing Alignment

PASS. `tasks.md` starts with failing tests before production tasks and covers successful subscription attribution, failover attribution, forced-refresh attribution, failure reason codes, lifecycle audits, redaction, and API-key regression.

### Security Alignment

PASS. All artifacts consistently forbid access tokens, refresh tokens, ID tokens, bearer strings, JWTs, encrypted credential values, raw account payloads, and fallback API keys from spend/audit/error/callback sinks.

## Open Questions

None requiring owner input.

Implementation can choose exact field names and whether attribution is carried through request context, response wrapper, or direct callback fields, as long as tests prove actual selected credential attribution and no API-key regression.

## Gate Decision

Todo -> Waiting is allowed after:

1. SpecKit artifacts are committed and pushed.
2. Draft PR exists.
3. `git diff --check origin/main...HEAD` passes.
4. Linear is updated to Waiting with PR/issue links.

## Implementation Gate Update

**Date**: 2026-05-08

- Fatal issues: 0
- Critical issues: 0
- Owner input required: 0

Implemented safe OpenAI subscription attribution through `callback.LogData`, spend metadata, selected provider-attempt propagation, and lifecycle audit events for connect/refresh/delete/disable. `T034` is skipped because no explicit OpenAI subscription test endpoint/helper exists in the current implementation surface.

Verification:

- `go test ./internal/callback ./internal/spend ./internal/proxy/handler -run 'TestOpenAISubscription.*Attribution|TestMergeOpenAISubscription|TestLogSuccessCopiesOpenAISubscription|TestOpenAISubscription.*Audit|TestOpenAISubscriptionDeleteAudit' -count=1`
- `go test ./internal/proxy/handler/... ./internal/spend/... ./internal/callback/... -count=1`
- `go tool golangci-lint run`
- `git diff --check`

`go test ./... -count=1` was also attempted; non-integration packages completed, then `test/integration` failed because local Postgres at `localhost:5433` was not running.
