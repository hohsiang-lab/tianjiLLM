# Data Model: OpenAI Subscription Credential Persistence

## Existing Table: CredentialTable

No schema change is planned.

| Column | Existing Type | HO-1176 Use |
|--------|---------------|-------------|
| `credential_id` | `TEXT PRIMARY KEY` | Existing credential identifier |
| `credential_name` | `TEXT NOT NULL` | Human-readable credential name |
| `credential_type` | `TEXT NOT NULL DEFAULT 'api_key'` | Set to `openai_subscription` for this feature |
| `credential_value` | `TEXT NOT NULL` | NaCl SecretBox encrypted JSON token bundle |
| `credential_info` | `JSONB NOT NULL DEFAULT '{}'` | Non-secret metadata JSON |
| `organization_id` | nullable `TEXT` | Existing organization scoping |
| `created_at` / `updated_at` | `TIMESTAMPTZ` | Existing timestamps |

## Secret Entity: OpenAISubscriptionTokenBundle

Stored only as encrypted JSON in `CredentialTable.credential_value`.

```json
{
  "access_token": "string",
  "refresh_token": "string",
  "expires_at": "2026-05-06T13:00:00Z",
  "account_id": "string"
}
```

### Validation

- `access_token` required, non-empty.
- `refresh_token` required, non-empty for initial create.
- `expires_at` required and parseable as RFC3339.
- `account_id` required, non-empty.
- Unknown fields should be rejected or ignored consistently; implementation should prefer rejecting unknown secret fields in tests if using strict decoding.

## Metadata Entity: OpenAISubscriptionCredentialInfo

Stored as JSON object in `CredentialTable.credential_info`.

```json
{
  "email": "user@example.com",
  "scopes": ["openid", "profile", "email", "offline_access", "api.connectors.read", "api.connectors.invoke"],
  "status": "active",
  "last_refresh_at": "2026-05-06T13:00:00Z",
  "last_error": "",
  "disabled_reason": ""
}
```

### Validation

- Metadata must be a JSON object.
- `scopes` should be an array of strings when present.
- `status` should be one of `active`, `refresh_failed`, `disabled`, or future explicitly documented values.
- Metadata must not include `access_token`, `refresh_token`, `id_token`, `credential_value`, or `token_bundle`.

## Redacted Response Entity: CredentialResponse

Used by `/credentials/list` and `/credentials/info/{credential_id}`.

```json
{
  "credential_id": "cred_123",
  "credential_name": "OpenAI Pro",
  "credential_type": "openai_subscription",
  "credential_info": {
    "email": "user@example.com",
    "scopes": ["openid", "profile", "email"],
    "status": "active",
    "last_refresh_at": "2026-05-06T13:00:00Z"
  },
  "organization_id": "org_123",
  "created_at": "2026-05-06T12:00:00Z",
  "updated_at": "2026-05-06T13:00:00Z"
}
```

### Redaction Rule

Response must not include:

- `credential_value`
- `access_token`
- `refresh_token`
- `id_token`
- raw encrypted bundle copies

## New sqlc Query

File: `internal/db/queries/credential.sql`

```sql
-- name: UpdateCredentialValueAndInfo :exec
UPDATE "CredentialTable"
SET credential_value = $2,
    credential_info = $3,
    updated_at = NOW()
WHERE credential_id = $1;
```

Generated params are expected to include:

```go
type UpdateCredentialValueAndInfoParams struct {
    CredentialID    string `json:"credential_id"`
    CredentialValue string `json:"credential_value"`
    CredentialInfo  []byte `json:"credential_info"`
}
```

## Relationships

```text
OpenAI OAuth token response (HO-1174/refresh issue)
  -> OpenAISubscriptionTokenBundle
  -> auth.Encrypt(...)
  -> CredentialTable.credential_value

Decoded ID token / callback account info / refresh status
  -> OpenAISubscriptionCredentialInfo
  -> CredentialTable.credential_info

Credential list/info API
  -> CredentialTable row
  -> CredentialResponse redacted mapper
```
