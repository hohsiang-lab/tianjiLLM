# Quickstart: HO-1179 Verification

## Todo Gate

This PR is docs-only. Verify planning scope with:

```bash
git diff --name-only origin/main...HEAD
git diff --check origin/main...HEAD
```

Expected Todo-gate diff:

```text
specs/1179-anthropic-like-sticky-failover-openai-subscriptions/
```

## Future Implementation Gate

After Linear HO-1179 moves to In Progress, use these commands while implementing:

```bash
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscription|TestOpenAISubscriptionRouting|TestParseOpenAIRateLimit|TestOpenAISubscriptionEndpoints' -count=1 -v
go test ./internal/config/... -run 'TestValidateOpenAISubscription' -count=1 -v
go test ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Manual Smoke Cases

### Multiple credentials route without API-key fallback

1. Configure an official OpenAI model with `openai_subscription_credential_ids: [cred_a, cred_b]`.
2. Include an `api_key` value in the same config only as a fallback leak sentinel.
3. Disable or break `cred_a`.
4. Send a non-streaming official OpenAI endpoint request.
5. Expected: request uses `cred_b`; sentinel `api_key` is never sent upstream.

### Sticky is org-scoped

1. Set `native_upstream_strategy: sticky`.
2. Send repeated requests from org `org_a`.
3. Send repeated requests from org `org_b`.
4. Expected: each org has stable sticky reuse, and orgs do not share sticky state.

### Rate-limit gate

1. Use an `httptest` upstream that returns `x-ratelimit-remaining-requests: 0` and `x-ratelimit-reset-requests: 60s` for `cred_a`.
2. Configure `cred_b` as usable.
3. Expected: `cred_a` is gated and the request retries with `cred_b`.

### Streaming boundary

1. Use a streaming upstream that emits bytes and then fails.
2. Expected: Tianji does not retry with another credential after partial body relay.
