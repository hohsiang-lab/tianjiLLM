# Contract: Codex usage snapshot

## Provider request

```http
GET /backend-api/wham/usage HTTP/1.1
Host: chatgpt.com
Authorization: Bearer <access_token>
ChatGPT-Account-Id: <account_id>
Referer: https://chatgpt.com/codex/settings/usage
Accept: application/json
```

`ChatGPT-Account-Id` is omitted when the resolved credential has no account id.

The actual base URL must follow the configured ChatGPT Codex backend base URL defaulting behavior. The provider helper must support mock base URLs in tests.

## Normalized UI/API response

Example shape for internal UI JSON or service output:

```json
{
  "credential_id": "cred_123",
  "email": "user@example.com",
  "plan_type": "pro",
  "status": "fresh",
  "primary_window": {
    "name": "primary_5h",
    "used_percent": 0.06,
    "reset_at": "2026-05-13T14:30:00Z",
    "status": "available"
  },
  "weekly_window": {
    "name": "weekly",
    "used_percent": 0.14,
    "reset_at": "2026-05-18T00:00:00Z",
    "status": "available"
  },
  "additional_buckets": [
    {
      "name": "GPT-5.3-Codex-Spark",
      "used_percent": 0,
      "reset_at": "2026-05-18T00:00:00Z",
      "status": "available"
    }
  ],
  "credits_status": "available",
  "fetched_at": "2026-05-13T02:00:00Z",
  "expires_at": "2026-05-13T02:01:00Z",
  "backoff_until": "",
  "last_error_reason": ""
}
```

## Error response/state

Safe per-credential state:

```json
{
  "credential_id": "cred_123",
  "status": "backoff",
  "last_error_reason": "upstream_server_error",
  "backoff_until": "2026-05-13T02:05:00Z",
  "snapshot": {
    "status": "stale",
    "fetched_at": "2026-05-13T01:59:00Z"
  }
}
```

Rules:

- Never include upstream raw body.
- Never include `Authorization`, bearer tokens, refresh tokens, cookies, encrypted credential values, or raw headers.
- Use stable reason codes such as `auth_error`, `rate_limited`, `upstream_server_error`, `parse_error`, `credential_disabled`, `credential_missing`, `credential_malformed`, `database_unavailable`.

## Cache contract

- First read with empty cache may call upstream.
- Read inside TTL returns cache and does not call upstream.
- Manual refresh inside TTL returns cache unless implementation explicitly marks the request as allowed to refresh and no backoff is active.
- 401 performs refresh-once retry.
- 429/5xx activates backoff and preserves last successful normalized snapshot.
- Proxy model requests do not call this contract.
