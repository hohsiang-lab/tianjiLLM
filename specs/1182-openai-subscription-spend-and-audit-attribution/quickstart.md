# Quickstart: OpenAI Subscription Spend and Audit Attribution

## Goal

Verify that OpenAI subscription traffic and credential lifecycle operations are attributable without leaking secret material.

## Local Verification Flow

1. Run targeted attribution tests:

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscription.*Attribution|TestOpenAISubscription.*Audit' -count=1 -v
```

2. Run redaction regression tests:

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscription.*Redaction|TestRedactionSinks' -count=1 -v
```

3. Run spend/callback tests:

```bash
go test ./internal/spend/... ./internal/callback/... -run 'Test.*Subscription.*Attribution|TestRecord.*Metadata|TestLogData' -count=1 -v
```

4. Run affected packages:

```bash
go test ./internal/proxy/handler/... ./internal/spend/... ./internal/callback/... -count=1
```

5. Check docs/code diff hygiene:

```bash
git diff --check origin/main...HEAD
```

## Manual Inspection Checklist

- A successful subscription request logs selected `credential_id`, not just configured first ID.
- A failover request from `cred_a` to `cred_b` attributes success to `cred_b`.
- `SpendLogs.api_key` remains the client virtual key hash and never contains subscription bearer material.
- Lifecycle audit rows exist for connect, refresh, delete, test, and disable.
- Audit/error/callback payloads contain stable reason codes and no raw upstream token/account payload.
- API-key traffic has no `openai_subscription.credential_id`.

## No-real-OpenAI Rule

All tests must use local mocks or `httptest`. Do not call `api.openai.com`, `auth.openai.com`, or use a real OpenAI subscription credential.
