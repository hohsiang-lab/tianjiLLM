# Implementation Plan: OpenAI Subscription Spend and Audit Attribution

**Branch**: `HO-1182-openai-subscription-spend-and-audit-attribution`
**Date**: 2026-05-07
**Spec**: [spec.md](spec.md)
**Linear**: HO-1182

## Summary

Current `origin/main` already has OpenAI subscription credential resolution, routing/failover, quota state, 401 refresh/disable behavior, and shared redaction. HO-1182 should connect that runtime selection to existing spend, error, callback, and audit attribution. The implementation should add a narrow safe attribution object, carry it from the selected subscription attempt into request context/callback data, and audit subscription credential lifecycle actions through the existing redacted audit helper.

## Technical Context

**Language/Version**: Go
**Relevant packages**: `internal/proxy/handler`, `internal/callback`, `internal/spend`, `internal/db`, `internal/security/redact`
**Existing spend path**: `callback.LogData` -> `spend.Tracker.LogSuccess` -> `SpendRecord` -> `CreateSpendLog`
**Existing audit path**: `Handlers.createAuditLog` -> `redact.JSONValue` -> `InsertAuditLog`
**Existing subscription attempt loop**: `doOpenAISubscriptionProviderRequest` and `openAISubscriptionProviderAttempts`
**Testing**: Go tests with `httptest`, handler harnesses, sqlc mock DBs, and no real OpenAI network calls
**Constraints**: Todo state is docs-only; production code starts only after Linear moves to In Progress

## Current Repo Findings

- `callback.LogData` already contains `Provider`, `OrganizationID`, and `UpstreamTokenKey`, but no OpenAI subscription `CredentialID` or lifecycle action/status fields.
- `spend.SpendRecord` maps `LogData` into `SpendLogs` and already writes `provider`, `organization_id`, and `upstream_token_key`.
- `SpendLogs` currently has no dedicated `credential_id` column; the lowest-risk path is to store subscription attribution in `metadata` unless implementation proves a schema migration is needed.
- `buildBaseLogData` extracts virtual key hash, user, team, organization, requester IP, and upstream token key from request context.
- `doOpenAISubscriptionProviderRequest` knows each attempt's `credentialID`, but that ID is not currently propagated to logging/callback code after a response is returned.
- `createAuditLog` and `dispatchEvent` already sanitize payloads through `redact.JSONValue`.
- `OpenAIOAuthCallback` saves subscription credentials but does not visibly audit connect success/failure in the inspected file.
- Existing error logging uses `redact.String(err.Error())` before `InsertErrorLog`.

## External Evidence

- OpenAI API reference states API requests use HTTP bearer authentication and optional `OpenAI-Organization` / `OpenAI-Project` headers; usage counts against the specified organization/project when supplied: https://platform.openai.com/docs/api-reference/authentication
- OpenAI production guidance emphasizes securing API keys through environment variables or secret management and not exposing them in code or public repositories: https://platform.openai.com/docs/guides/production-best-practices
- OpenAI rate-limit guidance distinguishes request/token rate limits from usage/spend limits and exposes request/token rate-limit headers; HO-1182 should not estimate subscription monthly spend locally: https://platform.openai.com/docs/guides/rate-limits

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| Repo reality first | PASS | Plan is based on current handler/callback/spend/audit code. |
| Research before build | PASS | Official OpenAI docs checked for auth, organization attribution, key safety, and rate/spend distinction. |
| Failing tests first | PASS | `tasks.md` starts with failing attribution/audit/regression tests. |
| No production code in Todo | PASS | This branch contains SpecKit artifacts only. |
| Redaction discipline | PASS | All new fields are safe identifiers/status codes; secret material remains forbidden. |

## Project Structure

### Documentation

```text
specs/1182-openai-subscription-spend-and-audit-attribution/
+-- spec.md
+-- plan.md
+-- research.md
+-- data-model.md
+-- quickstart.md
+-- analyze.md
+-- contracts/
|   +-- subscription-attribution.md
+-- checklists/
|   +-- requirements.md
+-- tasks.md
```

### Source Code, Future Implementation Scope

```text
internal/
+-- callback/
|   +-- callback.go                         # MODIFY: add safe subscription attribution fields
+-- spend/
|   +-- tracker.go                          # MODIFY: persist attribution metadata without changing api_key path
+-- proxy/handler/
|   +-- handler.go                          # MODIFY: build/extract attribution context
|   +-- openai_subscription_provider_retry.go # MODIFY: capture selected/failed credential attribution
|   +-- openai_oauth.go                     # MODIFY: audit connect success/failure
|   +-- openai_subscription_refresh*.go      # MODIFY: audit refresh/test/disable where implemented
|   +-- audit_helper.go                     # REUSE: redacted audit insertion
|   +-- *_test.go                           # ADD/MODIFY: attribution, audit, redaction, regression tests
```

## Proposed Data Flow

```text
OpenAI subscription route resolution
  -> ordered candidates with credential_id
  -> provider attempt loop sends request with bearer token
  -> selected attempt records safe attribution in request context or response wrapper
       credential_id
       provider=openai
       organization_id from context/credential metadata
       action/status/reason_code where relevant
  -> buildBaseLogData / callback.LogData
  -> spend.Tracker
       SpendLogs.metadata.subscription.credential_id
       provider / organization_id existing columns
  -> lifecycle handler/service
       createAuditLog(..., redact.JSONValue(payload))
```

## Design Decisions

### Keep `SpendLogs.api_key` for virtual key attribution

`SpendLogs.api_key` currently stores the client virtual key hash and drives `UpdateVerificationTokenSpend`. Subscription bearer tokens must not enter that field. Store subscription credential attribution in metadata unless a later implementation review proves a dedicated column is needed.

### Propagate selected credential, not configured credential list

Failover means the first configured credential may not serve the request. Attribution must be set after the actual attempt outcome is known, not during config parsing.

### Audit lifecycle actions at operation boundaries

Audit should be emitted where connect/refresh/delete/test/disable succeeds or fails, not from low-level redaction helpers. The payload should use action/status/reason code and safe metadata only.

### Failure attribution should use stable reason codes

Free-form upstream error text is useful only after redaction and should not be the primary audit key. Tests should assert stable codes such as `refresh_failed`, `auth_failed_after_refresh`, `rate_limited`, `credential_disabled`, and `credential_missing`.

### API-key behavior is the regression anchor

Existing API-key traffic must continue to update virtual key spend and omit subscription-only metadata. Subscription attribution should be additive only when a subscription credential is selected.

## Verification Commands

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscription.*Attribution|TestOpenAISubscription.*Audit|TestOpenAISubscription.*Redaction|TestOpenAISubscription.*APIKey' -count=1 -v
go test ./internal/spend/... ./internal/callback/... -run 'Test.*Subscription.*Attribution|TestRecord.*Metadata|TestLogData' -count=1 -v
go test ./internal/proxy/handler/... ./internal/spend/... ./internal/callback/... -count=1
git diff --check origin/main...HEAD
```

## Implementation Phases

### Phase 1: Tests First

Write failing tests for successful subscription spend attribution, failover attribution, failure attribution, lifecycle audit actions, redaction, and API-key regression.

### Phase 2: Safe Attribution Type

Add a small safe attribution model to callback/context handling. It must contain only safe IDs/status/reason fields and must be cheap to copy before asynchronous callback dispatch.

### Phase 3: Selected Attempt Propagation

Update the subscription provider attempt loop and proxy transport paths so the actual serving or failed credential ID is propagated to logging/audit surfaces.

### Phase 4: Spend/Callback Metadata

Persist subscription attribution in `SpendLogs.metadata` and callback data without changing `SpendLogs.api_key` semantics or virtual key budget updates.

### Phase 5: Lifecycle Audit

Add redacted audit events to connect, refresh success/failure, delete, test, and disable operations. Reuse existing `createAuditLog` and `redact.JSONValue`.

### Phase 6: Regression Verification

Run targeted attribution/audit/redaction tests, affected package tests, and `git diff --check`.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Selected credential lost before async callback | Copy attribution into `callback.LogData` before goroutine dispatch. |
| Bearer token accidentally stored as API key | Keep `SpendLogs.api_key` tied to client token hash only; add negative token assertions. |
| Failover logs wrong credential | Add test where `cred_a` fails and `cred_b` succeeds. |
| Audit emits raw request structs | Use narrow audit payload structs/maps and existing redaction helper. |
| Schema churn for a narrow feature | Start with `metadata`; require explicit implementation evidence before migration. |
| API-key regression | Add API-key success/budget regression tests before production edits. |

> Plan reviewed against repo reality and official OpenAI docs on 2026-05-07.
