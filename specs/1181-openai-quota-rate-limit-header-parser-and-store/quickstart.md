# Quickstart: HO-1181 Verification

## Targeted Parser and Store Tests

```bash
go test ./internal/proxy/handler/... -run 'TestParseOpenAIRateLimit|TestOpenAIQuota' -count=1 -v
go test ./internal/callback/... -run 'TestOpenAIQuota' -count=1 -v
```

## Routing Integration Tests

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting' -count=1 -v
```

## Affected Package Sweep

```bash
go test ./internal/proxy/handler/... ./internal/callback/... ./internal/testutil/openaitest/... -count=1
```

## Diff Sanity

```bash
git diff --check origin/main...HEAD
git diff --name-only origin/main...HEAD
```

## Manual Test Fixture Shape

Use `httptest` or `internal/testutil/openaitest` to return:

```text
x-ratelimit-limit-requests: 100
x-ratelimit-remaining-requests: 0
x-ratelimit-reset-requests: 1s
x-ratelimit-limit-tokens: 200000
x-ratelimit-remaining-tokens: 0
x-ratelimit-reset-tokens: 6m0s
```

No test may call real `api.openai.com` or `auth.openai.com`.
