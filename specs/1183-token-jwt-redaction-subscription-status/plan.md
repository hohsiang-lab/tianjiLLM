# Implementation Plan: Token/JWT Redaction and Subscription Credential Status Metadata

**Branch**: HO-1183-token-jwt-redaction-subscription-status
**Date**: 2026-05-06
**Spec**: [spec.md](spec.md)

## Summary

Add a central redaction utility for TianjiLLM backend secret material and route every credential/auth/error/audit sink through it. Keep HO-1176 encrypted credential persistence intact, but sanitize `last_error`, `disabled_reason`, upstream error text, JWT auth failures, ErrorLogs, AuditLog JSON, and management event payloads before persistence or response.

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Primary Dependencies**: standard library `encoding/json`, `regexp`, `strings`; existing `github.com/golang-jwt/jwt/v5`; existing zerolog/ErrorLogs/AuditLog helpers
**Storage**: Existing `CredentialTable.credential_info JSONB`, `ErrorLogs.error_message`, `AuditLog.before_value`, `AuditLog.updated_values`
**Testing**: Go unit tests only; offline fixtures
**Constraints**: No production implementation in Todo; no schema migration unless implementation proves unavoidable; no real OpenAI network calls
**Scale/Scope**: Backend-only redaction boundary across known credential/auth/log sinks

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| Repo reality first | PASS | Existing HO-1176 helpers already redact list/info; gaps are shared sanitization and other sinks. |
| Research before build | PASS | Repo search, OpenAI key safety docs, OWASP logging/secrets guidance, Context7 JWT docs, and grep-app attempt recorded in research.md. |
| Failing-tests-first | PASS | tasks.md starts with helper and sink tests before implementation tasks. |
| sqlc-first DB access | PASS | No new DB access planned; if schema/query changes emerge, use sqlc. |
| No real secrets | PASS | Tests must use synthetic token/JWT fixtures only. |

## Repo Findings

- `internal/proxy/handler/credentials.go` already defines `CredentialTypeOpenAISubscription`, token bundle metadata structs, `sanitizeCredentialInfoJSON`, and redacted list/info mapping from HO-1176.
- Current credential metadata sanitizer only blocks a short field list and does not sanitize free-text `last_error` / `disabled_reason`.
- `internal/proxy/middleware/auth.go` logs JWT validation failures through `zerolog.Ctx(...).Warn().Err(err)` and writes safe fixed auth error messages to `ErrorLogs`; plan still needs tests to lock that no raw JWT leaks through wrapped errors.
- `internal/proxy/handler/chat.go` stores `err.Error()` directly into `ErrorLogs.error_message`.
- `internal/proxy/handler/audit_helper.go` marshals arbitrary `beforeValue` / `updatedValues` directly before `InsertAuditLog`.
- `internal/proxy/handler/audit.go` returns audit rows directly, so persisted audit redaction must happen before insert.

## Architecture

### Redaction Utility

Create a small shared helper under `internal/security/redact` or `internal/auth/redact` after implementation review of package ownership.

Required API shape:

```go
func String(input string) string
func JSONValue(input any) any
func RawJSON(raw []byte) []byte
```

Rules:

- Replace secrets with a stable marker such as `[REDACTED]`.
- Match secret field names case-insensitively.
- Redact JWT-shaped values without validating or decoding.
- Redact bearer-token substrings in free text.
- Recurse through `map[string]any` and `[]any`.
- Keep non-secret metadata fields unchanged.

### Sink Integration

- `credentials.go`: use shared redaction in `sanitizeCredentialInfoJSON`, `redactCredential`, `marshalSafeCredentialInfo`, and failure metadata helpers.
- `chat.go`: sanitize `params.ErrorMessage` before `InsertErrorLog` and callback failure data if the callback receives raw error strings.
- `audit_helper.go`: sanitize `beforeValue` and `updatedValues` before marshaling.
- `auth.go` / `db_validator.go`: add regression tests around JWT/API-key failure logging; only change code if tests expose leakage.
- management event dispatch: sanitize payloads where credential request structs or DB rows are dispatched.

## Data Model

No schema migration planned.

Safe `credential_info` example:

```json
{
  "email": "owner@example.com",
  "scopes": ["openid", "profile"],
  "status": "refresh_failed",
  "last_refresh_at": "2026-05-06T14:50:00Z",
  "last_error": "OpenAI token refresh failed: [REDACTED]",
  "disabled_reason": "refresh failed: [REDACTED]"
}
```

Forbidden fields must not survive in persisted metadata, HTTP responses, ErrorLogs, AuditLogs, or event payloads.

## Failing Tests

| Test | File | Assertion |
|------|------|-----------|
| `TestRedactString_RemovesBearerTokenAndJWT` | new redaction package test | Raw bearer token and JWT fixture are absent; marker present. |
| `TestRedactJSONValue_RemovesNestedCredentialFields` | new redaction package test | Nested access/refresh/id token/account payload fields are redacted recursively. |
| `TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets` | `internal/proxy/handler/credential_test.go` | `last_error` / `disabled_reason` persist without raw token/JWT. |
| `TestCredentialInfo_RedactsNestedAccountPayload` | `internal/proxy/handler/credential_test.go` | list/info responses do not expose nested token-bearing account payload. |
| `TestRecordErrorLog_RedactsUpstreamTokenText` | handler test | `InsertErrorLogParams.ErrorMessage` contains marker and no raw upstream token/JWT. |
| `TestCreateAuditLog_RedactsCredentialPayloads` | handler test | `InsertAuditLogParams.BeforeValue` / `UpdatedValues` JSON omits raw secrets. |
| `TestJWTValidationFailure_DoesNotLogRawToken` | `internal/proxy/middleware/auth_test.go` | zerolog output and spy ErrorLogger do not contain the JWT fixture. |

## Plan Review

- **Repo evidence**: HO-1176 merged encrypted `openai_subscription` credential helpers in `internal/proxy/handler/credentials.go`; this plan extends existing helpers instead of replacing the storage model.
- **Repo evidence**: `recordErrorLog` currently persists `err.Error()` directly, and `createAuditLog` currently marshals arbitrary payloads directly; these are the highest-risk sinks.
- **OpenAI docs**: OpenAI Help Center key-safety guidance says API keys should not be exposed in client-side code, committed to repos, or shared; production apps should route through backend and protect keys with environment variables or key-management services.
- **OWASP guidance**: OWASP Secrets Management says secrets must never be logged unless masked/encrypted, and OWASP logging guidance identifies access tokens/JWT failures as sensitive logging risks.
- **Context7 / golang-jwt**: JWT docs recommend classifying parser errors with `errors.Is`; implementation can keep operator-visible categories without including raw token material.
- **GitHub grep-app**: `/Users/n0rmanc/.cargo/bin/grep-app-cli` ran but returned no useful Go redaction examples for `Redact` / `access_token`; no public pattern was adopted.

## Risks

| Risk | Mitigation |
|------|------------|
| Over-redaction removes useful status metadata | Keep allowlisted metadata fields and test `status`, `last_refresh_at`, `email`, and `scopes`. |
| Under-redaction misses free-text upstream errors | Add string redaction tests for bearer/JWT/error-description fixtures. |
| Audit rows already expose persisted historical secrets | This issue can prevent future writes only; historical cleanup is out of scope unless owner expands scope. |
| Regex false positives | Use targeted JWT/bearer patterns and fixture tests; prefer field-name redaction for structured JSON. |
