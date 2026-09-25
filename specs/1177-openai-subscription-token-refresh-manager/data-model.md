# Data Model: OpenAI Subscription Token Refresh Manager

## Existing Storage

### `CredentialTable`

Reused as-is.

| Column | Use |
|--------|-----|
| `credential_id` | Refresh lock key and resolver lookup key. |
| `credential_type` | Must equal `openai_subscription`. |
| `credential_value` | Encrypted JSON `OpenAISubscriptionTokenBundle`. |
| `credential_info` | Safe JSON metadata `OpenAISubscriptionCredentialInfo`. |
| `updated_at` | Updated through existing sqlc helpers after success/failure metadata writes. |

No new migration is planned.

## Secret Bundle

Encrypted JSON stored in `credential_value`.

```json
{
  "access_token": "required string",
  "refresh_token": "required string for refresh-capable credentials",
  "expires_at": "required RFC3339 timestamp",
  "account_id": "required string"
}
```

Rules:

- `access_token`, `expires_at`, and `account_id` are required before returning a resolved credential.
- `refresh_token` is required when the token is inside the refresh buffer or already expired.
- A successful refresh response with no `refresh_token` preserves the previous stored refresh token.
- A successful refresh response with a new `refresh_token` overwrites the previous stored refresh token.

## Safe Metadata

Plain JSONB stored in `credential_info`.

```json
{
  "email": "optional string",
  "scopes": ["optional", "strings"],
  "status": "active | refresh_failed | disabled",
  "last_refresh_at": "optional RFC3339 timestamp",
  "last_error": "optional redacted string",
  "disabled_reason": "optional redacted string"
}
```

Rules:

- `status=disabled` prevents refresh and returns a typed disabled error.
- `status=refresh_failed` records the last failure but does not prevent a later retry by itself.
- `last_error` and `disabled_reason` must be redacted before persistence.
- Secret-like fields are forbidden by existing credential metadata sanitization.

## Runtime Types

### `OpenAISubscriptionCredentialError`

Planned typed error shape:

```go
type OpenAISubscriptionCredentialError struct {
    Code         OpenAISubscriptionCredentialErrorCode
    CredentialID string
    Cause        error
}
```

Stable codes:

```text
credential_missing
credential_wrong_type
credential_disabled
credential_malformed
credential_expired
refresh_failed
```

Rules:

- `Error()` must redact `Cause`.
- Callers use `errors.As` or equivalent helper for code checks.
- Error text must not contain raw token material.

### `RefreshManager`

Planned handler-local dependencies:

```go
type openAISubscriptionRefreshManager struct {
    now           func() time.Time
    refreshBuffer time.Duration
    group         singleflight.Group
}
```

Implementation may embed these fields directly on `Handlers` if simpler.

Rules:

- Key singleflight by `credential_id`.
- Reload credential inside the singleflight function.
- Return a complete `OpenAISubscriptionTokenBundle` after success.

### `Refresh Result`

Uses existing provider `TokenBundle` plus conversion to stored bundle.

Required fields:

- `access_token`
- `expires_in`

Optional fields:

- `refresh_token`
- `id_token`
- `token_type`
- `scope`
- raw account/profile metadata

Rules:

- Missing/invalid `expires_in` is a refresh failure.
- Missing `refresh_token` is not a failure on successful refresh.
- `id_token` is not persisted in the secret bundle.

## State Transitions

```text
active + fresh token
  -> active, no DB update

active + near-expiry token + refresh success
  -> active, credential_value updated, last_refresh_at updated

active/refresh_failed + near-expiry token + refresh failure
  -> refresh_failed, credential_info updated, credential_value unchanged

disabled
  -> disabled typed error, no refresh

malformed/missing/no refresh token when required
  -> typed error, no upstream call
```
