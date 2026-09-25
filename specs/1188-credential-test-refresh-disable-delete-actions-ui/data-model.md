# Data Model: Credential actions UI

No DB schema changes.

## Existing Inputs

### `CredentialTable`

Existing DB row from `internal/db/credential.sql.go`.

- `credential_id`: action target identifier.
- `credential_name`: safe display label.
- `credential_type`: must be `openai_subscription`.
- `credential_value`: encrypted secret; never read or rendered by UI.
- `credential_info`: JSON safe metadata after redaction/normalization.
- `organization_id`: safe display/context.
- `created_at`, `updated_at`: safe timestamps.

### `OpenAISubscriptionCredentialInfo`

Existing metadata contract.

- `email`
- `scopes`
- `status`
- `last_refresh_at`
- `last_error`
- `disabled_reason`

UI must trim/redact display strings and tolerate missing/malformed JSON.

### `OpenAIQuotaState`

Existing HO-1181/HO-1186 optional quota state.

- Used for list/detail quota/status display.
- Not mutated by this issue.
- Must remain compatible after lifecycle action UI refreshes.

## New View Models

### `CredentialAction`

Planned UI-only view model.

- `CredentialID`
- `Name`
- `Action`: `test` | `refresh` | `disable` | `delete`
- `Label`
- `Variant`: default/outline/destructive
- `ConfirmMessage`
- `Endpoint`
- `Target`
- `Disabled`
- `DisabledReason`

### `CredentialActionResult`

Planned UI-only view model derived from safe backend result.

- `CredentialID`
- `Action`
- `Status`: `ok` | `error`
- `ToastTitle`
- `ToastVariant`
- `SafeMessage`
- `ReasonCode`
- `ModelsCount`
- `LastRefreshAt`
- `Deleted`

## Invariants

- View models must not contain raw `credential_value`.
- View models must not contain raw `access_token`、`refresh_token`、`id_token`、JWT、bearer string、authorization code、raw OpenAI account payload.
- HTMX forms may contain credential ID and action context only.
- Error display may contain stable reason code and sanitized copy only.
- Delete removes local DB credential; it does not claim remote OpenAI revocation.
