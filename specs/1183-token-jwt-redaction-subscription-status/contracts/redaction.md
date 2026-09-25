# Contract: Token/JWT Redaction

## Shared Helper

```go
func String(input string) string
func JSONValue(input any) any
func RawJSON(raw []byte) []byte
```

Behavior:

- Deterministic and side-effect free.
- Does not mutate caller-owned maps/slices unless implementation documents and tests that behavior.
- Replaces secret material with `[REDACTED]`.
- Handles invalid JSON in `RawJSON` by treating input as plain text.

## Credential Metadata Contract

Input metadata may include safe fields and free-text error fields.

Output metadata must:

- Preserve safe fields.
- Redact secret field values recursively.
- Redact bearer tokens and JWT-shaped values inside `last_error` and `disabled_reason`.
- Reject or sanitize raw secret field names consistently with existing credential behavior.

## Error/Audit Contract

- `ErrorLogs.error_message` must not include raw access token, refresh token, API key, bearer token, JWT, or raw account payload.
- `AuditLog.before_value` and `AuditLog.updated_values` must not include raw secret values even when handlers pass request structs directly.
- HTTP error responses must expose safe generic messages for auth/credential failures.
