# Quickstart: Safe OpenAI Subscription Credential CRUD

## Create

```bash
curl -X POST "$TIANJI_URL/credentials/new" \
  -H "Authorization: Bearer $PROXY_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "credential_name": "openai-main",
    "credential_type": "openai_subscription",
    "credential_value": "{\"access_token\":\"test-access\",\"refresh_token\":\"test-refresh\",\"expires_at\":\"2026-05-08T00:00:00Z\",\"account_id\":\"acct_123\"}",
    "credential_info": {
      "email": "owner@example.com",
      "status": "active"
    }
  }'
```

Expected:

- Response includes `credential_id`, `credential_name`, `credential_type`, `credential_info`, and timestamps。
- Response does not include `credential_value`, `test-access`, or `test-refresh`。

## List

```bash
curl "$TIANJI_URL/credentials/list" \
  -H "Authorization: Bearer $PROXY_ADMIN_TOKEN"
```

Expected:

- `credentials[]` contains safe metadata only。
- No encrypted or raw secret material appears。

## Info

```bash
curl "$TIANJI_URL/credentials/info/$CREDENTIAL_ID" \
  -H "Authorization: Bearer $PROXY_ADMIN_TOKEN"
```

Expected:

- Safe detail response。
- No `credential_value`。

## Update

```bash
curl -X POST "$TIANJI_URL/credentials/update" \
  -H "Authorization: Bearer $PROXY_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "credential_id": "'"$CREDENTIAL_ID"'",
    "credential_value": "{\"access_token\":\"rotated-access\",\"refresh_token\":\"rotated-refresh\",\"expires_at\":\"2026-05-09T00:00:00Z\",\"account_id\":\"acct_123\"}",
    "credential_info": {
      "status": "active",
      "last_refresh_at": "2026-05-08T00:00:00Z"
    }
  }'
```

Expected:

- Update succeeds for existing `openai_subscription`。
- Update rejects type changes and secret metadata。

## Delete

```bash
curl -X DELETE "$TIANJI_URL/credentials/delete/$CREDENTIAL_ID" \
  -H "Authorization: Bearer $PROXY_ADMIN_TOKEN"
```

Expected:

- Local delete succeeds。
- Repeating delete also succeeds as an idempotent local no-op。
- No remote OpenAI revoke is attempted。

## Verification

```bash
go test ./internal/proxy/handler/... -run 'TestCredential(New|List|Info|Update|Delete)_OpenAISubscription|TestOpenAISubscriptionCredential' -count=1 -v
go test ./internal/auth/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```
