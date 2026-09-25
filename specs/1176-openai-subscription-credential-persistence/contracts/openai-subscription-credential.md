# Contract: OpenAI Subscription Credential Persistence

## Credential Type

```text
openai_subscription
```

## Secret Payload Contract

Persisted encrypted in `CredentialTable.credential_value`.

```json
{
  "access_token": "required string",
  "refresh_token": "required string",
  "expires_at": "required RFC3339 timestamp",
  "account_id": "required string"
}
```

## Metadata Contract

Persisted plaintext JSONB in `CredentialTable.credential_info`; must not contain secrets.

```json
{
  "email": "optional string",
  "scopes": ["optional", "string", "array"],
  "status": "active | refresh_failed | disabled",
  "last_refresh_at": "optional RFC3339 timestamp",
  "last_error": "optional string",
  "disabled_reason": "optional string"
}
```

## Create/Save Helper Contract

Future implementation API shape may vary, but must satisfy:

```go
SaveOpenAISubscriptionCredential(ctx, input) (db.CredentialTable, error)
```

Required behavior:

- Validate required secret payload fields.
- Sanitize metadata and reject secret field names.
- Marshal secret payload JSON.
- Encrypt payload with configured master key.
- Call sqlc `CreateCredential` with:
  - `CredentialType: "openai_subscription"`
  - encrypted `CredentialValue`
  - safe `CredentialInfo`

## Refresh Update Helper Contract

Future implementation API shape may vary, but must satisfy:

```go
UpdateOpenAISubscriptionCredentialFromRefresh(ctx, credentialID, refreshResult) error
```

Required behavior:

- Load/decrypt existing bundle when needed to preserve refresh token.
- Update access token and expiry from refresh result.
- If refresh result includes a non-empty refresh token, persist it.
- If refresh result omits refresh token, preserve existing refresh token.
- Update safe refresh metadata.
- Persist encrypted bundle and metadata using a sqlc query.

## Redacted API Response Contract

`GET /credentials/list` and `GET /credentials/info/{credential_id}` must return redacted credential objects.

Allowed fields:

- `credential_id`
- `credential_name`
- `credential_type`
- `credential_info`
- `organization_id`
- `created_at`
- `updated_at`

Forbidden fields:

- `credential_value`
- `access_token`
- `refresh_token`
- `id_token`
- raw token bundle JSON
