# Data Model: HO-1183 Token/JWT Redaction

## Existing Tables

### CredentialTable

No schema change planned.

- `credential_value`: encrypted token bundle, never returned.
- `credential_info`: plaintext JSONB metadata, sanitized before write and before response.

Safe fields:

- `email`
- `scopes`
- `status`
- `last_refresh_at`
- `last_error`
- `disabled_reason`

Secret fields:

- `access_token`
- `refresh_token`
- `id_token`
- `credential_value`
- `authorization`
- `api_key`
- `x-api-key`
- `api-key`
- `token`
- `jwt`

### ErrorLogs

No schema change planned.

- `error_message`: must be redacted before insert.
- `api_key_hash`: may store hash only, never raw key.
- `traceback`: not in primary scope, but if implementation writes secret-bearing text here, same sanitizer must apply.

### AuditLog

No schema change planned.

- `before_value`: sanitized JSON bytes before insert.
- `updated_values`: sanitized JSON bytes before insert.

## Redaction Marker

Use one stable marker across string and JSON redaction:

```text
[REDACTED]
```

Tests assert raw fixture tokens are absent, not just marker presence.
