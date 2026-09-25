# Contract: OpenAI Subscription Attribution

## Spend/Callback Attribution

### Success Payload

For a successful official OpenAI subscription request, callback/spend metadata must expose:

```json
{
  "openai_subscription": {
    "credential_id": "cred_a",
    "provider": "openai",
    "organization_id": "org_123",
    "action": "request",
    "status": "success"
  }
}
```

Required fields:

- `credential_id`
- `provider`
- `action`
- `status`

Optional fields:

- `organization_id`
- `account_id`
- `request_id`
- `reason_code`
- safe redacted metadata

### Failure Payload

```json
{
  "openai_subscription": {
    "credential_id": "cred_a",
    "provider": "openai",
    "organization_id": "org_123",
    "action": "request",
    "status": "failure",
    "reason_code": "refresh_failed"
  }
}
```

Failure payloads may include only stable reason codes and redacted message text.

## Audit Attribution

Lifecycle audit events must use `CredentialTable` as the object table unless implementation already has a narrower table/entity name.

### Connect

```json
{
  "credential_id": "cred_a",
  "provider": "openai",
  "organization_id": "org_123",
  "action": "connect",
  "status": "success"
}
```

### Refresh

```json
{
  "credential_id": "cred_a",
  "provider": "openai",
  "organization_id": "org_123",
  "action": "refresh",
  "status": "failure",
  "reason_code": "refresh_failed",
  "metadata": {
    "last_error": "[REDACTED]"
  }
}
```

### Delete/Test/Disable

```json
{
  "credential_id": "cred_a",
  "provider": "openai",
  "organization_id": "org_123",
  "action": "disable",
  "status": "success",
  "reason_code": "auth_failed_after_refresh"
}
```

## Forbidden Data

The contract forbids these keys and values at every depth:

- `access_token`
- `refresh_token`
- `id_token`
- `credential_value`
- `authorization`
- `api_key`
- `token`
- `jwt`
- bearer-token text
- JWT-shaped values
- raw OAuth/account JSON

Tests should assert raw fixture secrets are absent, not only that `[REDACTED]` appears.

## API-key Regression Contract

For non-subscription `api_key` traffic:

```json
{
  "openai_subscription": null
}
```

Equivalent omission is acceptable. The implementation must not add a fake `credential_id`, and must preserve existing virtual key spend behavior.
