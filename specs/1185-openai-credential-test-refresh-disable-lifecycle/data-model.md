# Data Model: OpenAI Credential Test/Refresh/Disable Lifecycle APIs

## Existing Entities

### CredentialTable

Existing DB row used for all credential types。

- `credential_id`: stable credential identifier。
- `credential_type`: must be `openai_subscription` for lifecycle APIs。
- `credential_value`: encrypted OpenAI subscription token bundle; never returned。
- `credential_info`: safe JSON metadata used for lifecycle status。
- `organization_id`: optional local organization attribution。
- `created_at` / `updated_at`: existing timestamps。

### OpenAISubscriptionTokenBundle

Secret payload encrypted in `credential_value`。

- `access_token`: secret bearer token。
- `refresh_token`: secret refresh token; required when persistence validates full bundle。
- `expires_at`: local expiry timestamp。
- `account_id`: OpenAI account identifier used as safe-ish attribution, not a bearer secret。

Lifecycle responses must never expose this entity。

### OpenAISubscriptionCredentialInfo

Safe metadata persisted in `credential_info`。

- `email`: safe display metadata when available。
- `scopes`: safe OAuth scopes。
- `status`: `active`、`refresh_failed`、`disabled`。
- `last_refresh_at`: timestamp of last successful refresh。
- `last_error`: redacted stable error text only。
- `disabled_reason`: stable safe code such as `operator_disabled`、`auth_failed_after_refresh`、`refresh_failed`。

## New API Response Shapes

### OpenAISubscriptionLifecycleResponse

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "ok",
  "reason_code": "",
  "metadata": {
    "models_count": 3,
    "last_refresh_at": "2026-05-08T00:00:00Z"
  }
}
```

Fields:

- `credential_id`: target credential ID。
- `action`: `test`、`refresh`、`disable`。
- `status`: stable result such as `ok`、`disabled`、`refresh_failed`、`upstream_failed`、`credential_missing`、`credential_wrong_type`。
- `reason_code`: empty on success; stable safe error code on failure。
- `metadata`: optional redacted map. Never include raw OpenAI response or token bundle。

### Credential Test Metadata

Allowed safe fields:

- `models_count`
- `sample_model_id` only if needed and obtained from model list `id`
- `request_id` from upstream response headers when present
- `refreshed`: boolean
- `last_refresh_at`

Forbidden fields:

- `access_token`
- `refresh_token`
- `id_token`
- `credential_value`
- `Authorization`
- raw OpenAI response body
- bearer/JWT/token-shaped text

## State Transitions

### Refresh Success

`credential_info.status` becomes `active`; `last_refresh_at` updates; `last_error` and `disabled_reason` are cleared unless existing helper behavior chooses empty omission。

### Refresh Failure

`credential_info.status` becomes `refresh_failed`; `last_error` stores redacted error; `disabled_reason` becomes `refresh_failed`。

### Disable Active Credential

`credential_info.status` becomes `disabled`; `disabled_reason` becomes `operator_disabled`; existing safe `email`、`scopes`、`last_refresh_at` should be preserved。

### Disable Already Disabled Credential

No secret read or remote call is required beyond safe row load/update as implementation permits. Response remains successful idempotent status。

## Validation Rules

- Target credential must exist。
- Target credential must be `credential_type=openai_subscription`。
- Decrypted token bundle must pass existing validation before test/refresh when a bearer is needed。
- All response/audit/metadata maps must pass shared redaction before emission。
