# Quickstart: Verify OpenAI 401 Refresh Retry and Failover

## Preconditions

- Linear HO-1180 is In Progress before implementation starts.
- OpenAI subscription sibling issues HO-1167, HO-1176, HO-1177, HO-1178, HO-1179, HO-1183, and HO-1190 are present on the branch base.
- Tests use local `httptest` OpenAI upstream and `openaitest.OAuthServer`.
- No real OpenAI network calls are allowed.

## Scenario 1: 401 Refresh Retry Succeeds

1. Configure one model with `openai_subscription_credential_ids: ["cred-a"]`.
2. Seed `cred-a` with encrypted bundle:
   - access token: `access-a`
   - refresh token: `refresh-a`
   - expires at: one hour in the future
3. Mock OpenAI upstream:
   - `Authorization: Bearer access-a` returns 401
   - `Authorization: Bearer access-a-refreshed` returns 200
4. Mock token endpoint:
   - `refresh_token=refresh-a` returns `access_token=access-a-refreshed`
5. Send `/v1/responses` or `/v1/chat/completions`.
6. Expected:
   - upstream receives two requests
   - second request uses refreshed bearer
   - final response is success
   - metadata is active and redacted

## Scenario 2: Refresh Fails Then Failover Succeeds

1. Configure model with `["cred-a", "cred-b"]`.
2. Seed both credentials.
3. Mock upstream:
   - `cred-a` access token returns 401
   - `cred-b` access token returns 200
4. Mock token endpoint:
   - `cred-a` refresh returns `invalid_grant`
5. Send request.
6. Expected:
   - request succeeds with `cred-b`
   - `cred-a` stores `status=refresh_failed`
   - `cred-a.last_error` is redacted
   - no fallback API key is used

## Scenario 3: Refresh Retry Still 401 Then Failover Succeeds

1. Configure model with `["cred-a", "cred-b"]`.
2. Mock upstream:
   - `access-a` returns 401
   - `access-a-refreshed` returns 401
   - `access-b` returns 200
3. Mock token endpoint:
   - `refresh-a` returns `access-a-refreshed`
4. Send request.
5. Expected:
   - `cred-a` is attempted twice
   - `cred-a` stores `status=disabled` and `disabled_reason=auth_failed_after_refresh`
   - `cred-b` serves final response

## Scenario 4: All Credentials Need Reauth

1. Configure model with `["cred-a", "cred-b"]` and an `api_key`.
2. Make both credentials fail refresh or post-refresh 401.
3. Send request.
4. Expected:
   - final response is a clear reauthorization-required authentication error
   - response does not include raw upstream body or token material
   - configured API key is not used

## Targeted Commands

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting_401|TestOpenAISubscriptionProxyTransport_401|TestForceRefreshOpenAISubscriptionCredential|TestOpenAISubscriptionCredential_AuthFailureMetadata' -count=1 -v
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting|TestOpenAISubscriptionEndpoints|TestResolveOpenAISubscription' -count=1 -v
go test ./internal/proxy/handler/... ./internal/provider/openai/... ./internal/testutil/openaitest/... -count=1
git diff --check origin/main...HEAD
```

## Manual Review Checklist

- Confirm 401 is not added to generic retry status.
- Confirm API-key deployments do not call refresh.
- Confirm `Authorization`, access tokens, refresh tokens, fallback API keys, and JWT-looking text do not appear in response or metadata assertions.
- Confirm direct handler and proxy transport endpoint families both have 401 coverage.
