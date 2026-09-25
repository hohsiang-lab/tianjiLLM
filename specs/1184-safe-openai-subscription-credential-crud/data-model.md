# Data Model: Safe OpenAI Subscription Credential CRUD

## Existing Table: `CredentialTable`

No schema change planned。

| Column | Use in HO-1184 |
| --- | --- |
| `credential_id` | Stable local credential identifier. |
| `credential_name` | Operator-facing label. |
| `credential_type` | Must be `openai_subscription` for subscription CRUD semantics. |
| `credential_value` | Encrypted canonical subscription token bundle; write-only through API responses. |
| `credential_info` | Sanitized safe metadata JSONB. |
| `organization_id` | Optional org ownership/filtering field. |
| `created_at` / `updated_at` | Returned as safe metadata. |

## Secret Payload

Stored encrypted in `credential_value`。

```json
{
  "access_token": "required string",
  "refresh_token": "required string",
  "expires_at": "required RFC3339 timestamp",
  "account_id": "required string"
}
```

Forbidden in responses and metadata:

- `access_token`
- `refresh_token`
- `id_token`
- `credential_value`
- bearer-token strings
- JWT-shaped strings
- raw account payloads containing secret fields

## Safe Metadata

Stored plaintext as sanitized JSONB in `credential_info`。

```json
{
  "email": "optional string",
  "scopes": ["optional string array"],
  "status": "active | disabled | refresh_failed | optional string",
  "last_refresh_at": "optional RFC3339 timestamp",
  "last_error": "optional redacted string",
  "disabled_reason": "optional redacted string"
}
```

Metadata may be extended with other safe labels, but nested secret fields must be rejected or redacted through existing shared redaction boundary。

## Redacted Credential Response

Allowed response fields:

- `credential_id`
- `credential_name`
- `credential_type`
- `credential_info`
- `organization_id`
- `created_at`
- `updated_at`

Forbidden response fields:

- `credential_value`
- raw token bundle JSON
- encrypted credential blob
- `access_token`
- `refresh_token`
- `id_token`
- bearer strings
- JWTs

## State Transitions

```text
Create
  input token bundle + safe metadata
  -> validate
  -> encrypt credential_value
  -> insert CredentialTable row
  -> redacted response

Update
  credential_id
  -> load existing row
  -> require existing credential_type=openai_subscription for subscription update
  -> validate optional replacement token bundle
  -> sanitize optional metadata
  -> update value and/or metadata
  -> redacted/safe status response

Delete
  credential_id
  -> local DeleteCredential
  -> return success for existing or already-missing row
  -> no remote OpenAI revoke
```
