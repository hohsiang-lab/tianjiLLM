# Contract: OpenAI Credential Lifecycle APIs

## Auth Boundary

All endpoints are management endpoints and must require the same auth/RBAC boundary as existing `/credentials` APIs:

- Authenticated request only。
- Proxy admin role / existing credential-management permission。
- No public or model-proxy API key access。

## POST `/credentials/openai-subscription/{credential_id}/test`

Tests a local OpenAI subscription credential against configured/mockable OpenAI-compatible `/v1/models`。

### Success Response

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "ok",
  "models_count": 2
}
```

### Failure Response

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "error",
  "reason_code": "upstream_401"
}
```

## POST `/credentials/openai-subscription/{credential_id}/refresh`

Force-refreshes a local OpenAI subscription credential through configured OAuth token endpoint。

### Success Response

```json
{
  "credential_id": "cred_123",
  "action": "refresh",
  "status": "ok",
  "last_refresh_at": "2026-05-08T00:00:00Z"
}
```

### Failure Response

```json
{
  "credential_id": "cred_123",
  "action": "refresh",
  "status": "error",
  "reason_code": "refresh_failed"
}
```

## POST `/credentials/openai-subscription/{credential_id}/disable`

Locally disables a credential by safe metadata update。

### Success / Idempotent No-op Response

```json
{
  "credential_id": "cred_123",
  "action": "disable",
  "status": "ok"
}
```

## Shared Error Cases

- Missing credential: `status=error`、`reason_code=credential_missing`。
- Wrong type: `status=error`、`reason_code=credential_wrong_type`。
- Malformed credential: `status=error`、`reason_code=credential_malformed`。
- DB failure: `status=error`、stable credential/update failure reason code。

## Forbidden Response Content

Responses must not contain:

- `credential_value`
- `access_token`
- `refresh_token`
- `id_token`
- `Authorization`
- bearer token strings
- JWT-looking strings
- encrypted credential blobs
- fallback API keys
- raw upstream OpenAI token/model response bodies
