# Contract: OpenAI Subscription Credential CRUD

## Auth Boundary

All routes use existing `/credentials` management surface:

```text
AuthMiddleware required
Minimum role: proxy_admin
```

## Create

```text
POST /credentials/new
```

Request:

```json
{
  "credential_name": "openai-main",
  "credential_type": "openai_subscription",
  "credential_value": "{\"access_token\":\"...\",\"refresh_token\":\"...\",\"expires_at\":\"2026-05-08T00:00:00Z\",\"account_id\":\"acct_123\"}",
  "credential_info": {
    "email": "owner@example.com",
    "status": "active"
  },
  "organization_id": "org_123"
}
```

Required behavior:

- Validate `credential_name`。
- Validate `credential_value` as OpenAI subscription token bundle when `credential_type=openai_subscription`。
- Encrypt canonical token bundle before DB write。
- Sanitize `credential_info`。
- Return redacted credential response only。

## List

```text
GET /credentials/list
GET /credentials/list?organization_id=org_123
```

Response:

```json
{
  "credentials": [
    {
      "credential_id": "cred_123",
      "credential_name": "openai-main",
      "credential_type": "openai_subscription",
      "credential_info": {
        "email": "owner@example.com",
        "status": "active"
      },
      "organization_id": "org_123",
      "created_at": "timestamp",
      "updated_at": "timestamp"
    }
  ]
}
```

Required behavior:

- Use existing list/list-by-org DB paths。
- Redact every row before response。
- Never include `credential_value`。

## Info

```text
GET /credentials/info/{credential_id}
```

Required behavior:

- Return 404/sanitized not-found error when missing。
- Return redacted credential response when found。
- Never include `credential_value` or raw token fields。

## Update

```text
POST /credentials/update
```

Request:

```json
{
  "credential_id": "cred_123",
  "credential_value": "{\"access_token\":\"...\",\"refresh_token\":\"...\",\"expires_at\":\"2026-05-08T00:00:00Z\",\"account_id\":\"acct_123\"}",
  "credential_info": {
    "status": "active",
    "last_refresh_at": "2026-05-08T00:00:00Z"
  }
}
```

Required behavior:

- Load existing credential row。
- For subscription semantics, require existing row type `openai_subscription`。
- Reject request attempts to change `credential_type`。
- Validate/encrypt replacement `credential_value` when supplied。
- Sanitize/persist `credential_info` when supplied。
- Reject secret-bearing metadata。
- Return sanitized status or redacted credential response; no secret material。

## Delete

```text
DELETE /credentials/delete/{credential_id}
```

Required behavior:

- Delete local DB row if present。
- Treat already-missing row as successful local no-op unless DB delete fails。
- Do not call OpenAI remote revoke/logout/delete。
- Audit only safe local lifecycle facts when existing subscription context is available。

## Error Contract

Errors must use existing `model.ErrorResponse` shape and sanitized messages。Errors must not include raw request token bundles, encrypted credential values, JWTs, bearer strings, or OpenAI account payloads。
