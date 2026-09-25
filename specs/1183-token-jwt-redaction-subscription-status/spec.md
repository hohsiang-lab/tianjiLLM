# Feature Specification: Token/JWT Redaction and Subscription Credential Status Metadata

**Feature Branch**: HO-1183-token-jwt-redaction-subscription-status
**Created**: 2026-05-06
**Input**: Linear HO-1183 — `[BE] Token/JWT redaction and subscription credential status metadata`

## Summary

Centralize backend redaction for OpenAI subscription token bundles, JWT/API-key auth material, credential metadata, account payloads, upstream error text, error logs, audit logs, and callback failure payloads while preserving safe OpenAI subscription credential status metadata.

## Scope

- Add a shared redaction/sanitization boundary for secret-bearing strings and JSON-like payloads.
- Cover OpenAI subscription token bundle fields: `access_token`, `refresh_token`, `id_token`, `credential_value`, and account payload variants that may carry those fields.
- Cover JWT-shaped bearer tokens and API-key shaped values before they enter logs, ErrorLogs, audit logs, callback payloads, or user-facing error text.
- Preserve safe metadata in `credential_info`: `status`, `last_refresh_at`, `last_error`, `disabled_reason`, plus existing non-secret profile fields such as `email` and `scopes`.
- Ensure upstream/provider/OAuth error text is stored and returned only after redaction.

## Out of Scope

- Changing OAuth token acquisition or refresh semantics from HO-1176.
- Adding a new credential table or schema migration unless implementation proves existing JSONB metadata is insufficient.
- UI work.
- Real OpenAI network calls or real subscription credentials in tests.
- Implementing a full DLP engine; this issue owns deterministic secret patterns and known credential payloads.

## User Stories

### US1 — Secret-safe credential metadata

As a proxy admin, I can inspect OpenAI subscription credential status without seeing raw tokens or account payload secrets.

**Acceptance Scenarios**

1. Given an OpenAI subscription credential stores a token bundle, when list/info responses are produced, then no response contains `credential_value`, `access_token`, `refresh_token`, `id_token`, or JWT-looking values.
2. Given refresh succeeds, when metadata is updated, then `status=active` and `last_refresh_at` are persisted without raw token fields.
3. Given refresh fails with an upstream error containing a token/JWT/account payload, when metadata is updated, then `last_error` and `disabled_reason` contain redacted text only.

### US2 — Secret-safe auth and error logs

As an operator, I can use logs/ErrorLogs for troubleshooting without leaking bearer tokens, JWTs, or upstream credential payloads.

**Acceptance Scenarios**

1. Given JWT validation fails, when zerolog and `ErrorLogs` entries are written, then the token string and JWT segments are absent.
2. Given a provider/upstream error includes a token, JWT, Authorization header, or account JSON, when `recordErrorLog` runs, then `error_message` and callback failure data are redacted.
3. Given DB/auth failures include API-key material, when auth middleware logs them, then only existing token hash or token hash prefix is allowed.

### US3 — Secret-safe audit/event payloads

As an operator reviewing audit data, I can trust audit/event rows to omit secret material even when handlers pass request structs or metadata maps.

**Acceptance Scenarios**

1. Given `createAuditLog` receives `beforeValue` or `updatedValues` containing credential fields, when JSON is stored, then secret fields are replaced by a fixed redaction marker.
2. Given management events receive credential payloads, when dispatched, then payloads use the same redaction helper before leaving the handler boundary.
3. Given nested arrays/maps carry token fields, when sanitized, then nested secret material is redacted recursively.

## Functional Requirements

- **FR-001**: Provide one shared redaction helper package or module for secret strings and arbitrary JSON-like payloads.
- **FR-002**: Redact known secret field names case-insensitively: `access_token`, `refresh_token`, `id_token`, `credential_value`, `authorization`, `api_key`, `x-api-key`, `api-key`, `token`, `jwt`.
- **FR-003**: Redact JWT-shaped values with three base64url-like dot-separated segments without parsing claims.
- **FR-004**: Redact bearer-token text in strings, including `Authorization: Bearer ...` and JSON/string fragments containing bearer tokens.
- **FR-005**: Keep safe OpenAI subscription metadata fields available: `email`, `scopes`, `status`, `last_refresh_at`, `last_error`, `disabled_reason`.
- **FR-006**: Sanitize `last_error` and `disabled_reason` before persistence.
- **FR-007**: Replace direct uses of raw `err.Error()` in persistent/user-facing credential/auth/upstream paths with redacted/safe messages where secret-bearing inputs may flow.
- **FR-008**: Apply redaction before writing `ErrorLogs.error_message` and `AuditLog.before_value` / `updated_values`.
- **FR-009**: Preserve existing redacted credential list/info behavior from HO-1176 and extend it to JWT/account-payload leakage cases.
- **FR-010**: Tests must prove raw access token, refresh token, JWT, account payload, and upstream error fixtures do not appear in HTTP responses, log/event payloads, ErrorLogs, or AuditLog JSON.
- **FR-011**: Implementation must not use real OpenAI accounts, live token exchange, or real secrets.

## Edge Cases

- Token appears as a nested JSON field inside an arbitrary account payload.
- Token appears inside upstream error text rather than structured JSON.
- JWT validation error wraps library errors but must not include the original token.
- Secret field names appear with different casing or separators.
- Redaction must not erase safe metadata such as `status` or `last_refresh_at`.

## Success Criteria

- Unit tests cover redaction helpers, credential metadata updates, JWT auth failure logs, upstream error text, and audit/event payload sanitization.
- `go test` targeted backend packages pass after implementation.
- No production code is written during Todo planning.
