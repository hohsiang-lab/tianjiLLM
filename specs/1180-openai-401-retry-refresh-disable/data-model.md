# Data Model: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

## Existing Tables

No new table or schema migration is planned.

### CredentialTable

Existing row used by OpenAI subscription credentials.

- `credential_id`: stable configured ID used in `openai_subscription_credential_ids`
- `credential_type`: must be `openai_subscription`
- `credential_value`: encrypted OpenAI subscription token bundle
- `credential_info`: safe JSON metadata

## Existing Secret Payload

### OpenAISubscriptionTokenBundle

Encrypted in `credential_value`.

- `access_token`: secret bearer token for official OpenAI requests
- `refresh_token`: secret refresh token used by forced refresh
- `expires_at`: local expiry timestamp
- `account_id`: account identity used by subscription/Codex paths

HO-1180 must not add this payload to logs, metadata, HTTP responses, or map keys.

## Existing Safe Metadata

### OpenAISubscriptionCredentialInfo

Stored in `credential_info`.

- `email`
- `scopes`
- `status`
- `last_refresh_at`
- `last_error`
- `disabled_reason`

HO-1180 adds stable auth-failure values to existing fields.

## Proposed Metadata States

### Active After Forced Refresh

Used when forced refresh succeeds and retry succeeds.

```json
{
  "status": "active",
  "last_refresh_at": "2026-05-07T13:00:00Z"
}
```

Existing email/scopes should be preserved unless refresh response safely updates scopes.

### Refresh Failed

Used when forced refresh cannot produce a usable token.

```json
{
  "status": "refresh_failed",
  "last_error": "invalid_grant",
  "disabled_reason": "refresh_failed"
}
```

The encrypted token bundle must not be rewritten on failure.

### Auth Failed After Refresh

Used when upstream returns 401 again after forced refresh and one retry.

```json
{
  "status": "disabled",
  "last_error": "OpenAI authentication failed after forced refresh",
  "disabled_reason": "auth_failed_after_refresh"
}
```

`status=disabled` intentionally reuses existing candidate filtering in `loadOpenAISubscriptionCredential`.

## Runtime Types

### openAISubscriptionProviderAttempt

Existing attempt type.

- `credentialID`: non-empty for subscription candidates
- `apiKey`: current bearer material

HO-1180 needs to carry auth-failure reason for aggregate errors. This can be a new local struct/map keyed by `credentialID`; it must not store raw bearer tokens.

### Forced Refresh Result

Recommended shape for an internal helper return.

- `CredentialID`
- `BearerToken`
- `AccountID`
- `CodexLogin` optional, if helper is shared later
- `FailureCode` optional for refresh/auth failure

## Error Codes

Recommended stable internal codes:

- `refresh_failed`: existing HO-1177 code for refresh-token grant failure or invalid refresh response
- `auth_failed_after_refresh`: new code for 401 after forced refresh retry
- `credential_disabled`: existing disabled credential code
- `credential_missing`: existing missing credential code
- `credential_malformed`: existing malformed credential code
- `all_credentials_auth_failed`: aggregate all-failed request code

## Redaction Requirements

The following must never appear in `credential_info`, HTTP errors, callback payloads, or logs:

- access token
- refresh token
- ID token
- `Authorization: Bearer ...`
- JWT-looking strings
- fallback API key
- encrypted `credential_value`
- full upstream body if it contains token material

Allowed in aggregate errors:

- credential ID
- stable reason code
- safe fixed message
