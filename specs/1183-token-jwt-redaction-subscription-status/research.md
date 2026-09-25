# Research: HO-1183 Token/JWT Redaction

## Repo Reality

- HO-1176 already introduced `CredentialTypeOpenAISubscription`, encrypted token bundle helpers, `OpenAISubscriptionCredentialInfo`, and redacted credential list/info responses in `internal/proxy/handler/credentials.go`.
- Existing `secretCredentialInfoFields` covers `access_token`, `refresh_token`, `id_token`, and `credential_value`, but it is local to credential helpers and does not cover bearer/JWT free text or other sinks.
- `recordErrorLog` in `internal/proxy/handler/chat.go` persists `err.Error()` directly to `ErrorLogs.error_message`.
- `createAuditLog` in `internal/proxy/handler/audit_helper.go` marshals arbitrary payloads directly.
- JWT middleware currently returns fixed client messages and ErrorLogger messages, but zerolog uses wrapped JWT errors; regression tests should prove raw token strings never enter logs.

## External Evidence

- OpenAI API key safety guidance treats API keys as secret credentials: do not expose them in client-side environments, do not commit them, and use backend/key-management patterns for production.
- OWASP Secrets Management guidance says secrets must not be logged unless protected by masking/encryption.
- OWASP Logging guidance lists access tokens and JWT validation failures as sensitive log categories.
- Context7 `github.com/golang-jwt/jwt/v5` docs show parser errors can be classified with `errors.Is`, allowing operator-safe categories without including raw token material.

## GitHub Code Search

`/Users/n0rmanc/.cargo/bin/grep-app-cli --json 'Redact' --language Go` and `... 'access_token' --language Go` returned no useful examples, so this plan uses repo-local design plus official security guidance.

## Decisions

### Centralize redaction in a shared package

**Decision**: Add a small redaction package and remove ad hoc credential-only sanitization where possible.

**Rationale**: The issue owns logs/errors/audit as well as credential responses. A credential-local helper cannot protect `ErrorLogs`, `AuditLog`, and auth middleware.

### Preserve current schema

**Decision**: Store status metadata in existing `credential_info JSONB`; no migration is planned.

**Rationale**: HO-1176 already validated this storage model and added `UpdateCredentialInfo`.

### Redact before persistence

**Decision**: Sanitize values before writing ErrorLogs/AuditLog/credential metadata rather than relying on response-time filtering.

**Rationale**: Audit and error APIs return persisted rows directly in current code, so persisted raw secrets would remain leakable.

### Treat JWTs as opaque secrets

**Decision**: Match JWT-shaped values syntactically and redact without decoding claims.

**Rationale**: Decoding is unnecessary and can itself create leakage risk; the goal is absence of raw token material.
