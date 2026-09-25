# Quickstart: HO-1185 Lifecycle API Verification

## Todo Planning Verification

```bash
git diff --name-only origin/main...HEAD
git diff --check origin/main...HEAD
```

Expected Todo diff: only files under `specs/1185-openai-credential-test-refresh-disable-lifecycle/`。

## Implementation Verification After In Progress

Targeted lifecycle tests:

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionLifecycle|TestOpenAICredentialLifecycle' -count=1 -v
```

Existing credential CRUD tests:

```bash
go test ./internal/proxy/handler/... -run 'TestCredential|TestOpenAISubscriptionCredential' -count=1 -v
```

OpenAI subscription refresh/routing regressions:

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting|TestResolveOpenAISubscription|TestForceRefreshOpenAISubscriptionCredential' -count=1 -v
```

Affected package gate:

```bash
go test ./internal/proxy/handler/... ./internal/testutil/openaitest/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

## Manual Contract Smoke With Mock Server

Use only local/mock OpenAI upstream in tests. Do not call `api.openai.com` or `auth.openai.com` from automated verification。

Expected lifecycle response shape:

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "ok",
  "reason_code": "",
  "metadata": {
    "models_count": 1
  }
}
```

Response body must not include token fields, bearer strings, JWT-looking strings, encrypted blobs, or raw upstream response。
