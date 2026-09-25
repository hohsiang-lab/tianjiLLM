# Research: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

## Repo Evidence

### Existing retry path excludes 401

`internal/proxy/handler/openai_subscription_provider_retry.go` builds attempts from `apiKey` or subscription candidates. It records OpenAI rate-limit headers and retries only when `openAISubscriptionRetryableStatus(resp.StatusCode)` is true. That helper currently returns true for 429 and 5xx only, so upstream 401 is returned to the caller without refresh or failover.

### Existing proxy transport also excludes 401

`internal/proxy/handler/assistants.go` contains `openAISubscriptionFailoverTransport`. It clones the request, injects candidate bearer tokens, records rate-limit headers, and retries only through `openAISubscriptionRetryableStatus`. This means `/v1/responses` style proxy paths need the same 401-specific update as direct handlers.

### Existing refresh manager is freshness-based

`internal/proxy/handler/openai_subscription_refresh.go` uses `openAISubscriptionTokenIsFresh(bundle)` with a 5-minute buffer. It refreshes before expiry but does not force refresh a locally fresh token after upstream rejects it. HO-1180 needs a forced-refresh helper rather than changing the normal expiry path.

### Existing metadata can represent unavailable credentials

`loadOpenAISubscriptionCredential` rejects `credential_info.status == "disabled"`, so post-refresh 401 can mark the credential disabled and rely on current candidate filtering. Existing failure persistence uses `UpdateOpenAISubscriptionCredentialFailure`.

### Existing redaction is available

`internal/security/redact` and HO-1183 tests already cover token/JWT redaction in metadata, error logs, audit payloads, and callback/error sinks. HO-1180 should call this boundary for upstream 401 body text and refresh failures.

### Existing endpoint tests provide harnesses

`internal/proxy/handler/openai_subscription_endpoints_test.go` already has:

- OpenAI host rewrite via `withOpenAIHostRewrite`
- `openAIEndpointRecorder`
- multi-credential endpoint harnesses
- existing 429/5xx failover tests
- an existing `TestOpenAISubscriptionRouting_NoFailoverOnNonRateLimit4xx` that must be narrowed because 401 becomes a special subscription-auth path.

## Official/OpenAI Evidence

OpenAI official error-code docs list 401 cases including invalid authentication, incorrect API key, organization membership, and IP allowlist. The same page describes `AuthenticationError` as an invalid, expired, or revoked API key or token. This supports handling 401 as an authentication/credential lifecycle problem, not as a generic transient retry.

Context7 `/openai/openai-go` says the official Go client retries connection errors, 408, 409, 429, and 5xx by default. It does not list 401 as a generic retry. This supports keeping 401 out of `openAISubscriptionRetryableStatus`.

OpenAI official docs advise retrying 500/503 and pacing/backoff for 429. HO-1179 already owns that behavior; HO-1180 should only add subscription-aware refresh/retry for 401.

## GitHub / grep-app

grep-app searches were attempted:

- `StatusUnauthorized` in Go
- `refresh_token` in Go OAuth paths

Both failed with transport `Unexpected content type: text/html`. No public GitHub implementation was adopted. Repo reality remains the source of truth for this plan.

## Decisions

### Decision 1: Add 401-specific forced-refresh branch, not generic retry

**Decision**: Do not include 401 in `openAISubscriptionRetryableStatus`. Add a separate branch that only applies when the attempt has a subscription `credentialID`.

**Rationale**: Generic retry would affect API-key paths and replay stale bearer material. Only subscription credentials have refresh-token state that can remediate a 401.

### Decision 2: Force refresh bypasses local freshness

**Decision**: Add a forced refresh helper that reloads the credential and refreshes even if `expires_at` is still outside the normal buffer.

**Rationale**: Upstream 401 can mean the server revoked or invalidated a token before local expiry. Local freshness is insufficient evidence after upstream rejects the bearer.

### Decision 3: Disable after refreshed retry also returns 401

**Decision**: If the same credential returns 401 after forced refresh, persist safe unavailable metadata with stable reason `auth_failed_after_refresh` and exclude it from later routing.

**Rationale**: The refresh token produced a new access token, but OpenAI still rejected the account/credential. Further attempts in the same request would loop and later requests should not keep selecting it.

### Decision 4: Refresh failure fails over but records `refresh_failed`

**Decision**: Preserve HO-1177 refresh-failure semantics. Forced refresh failure records redacted `refresh_failed` metadata and then tries the next configured credential.

**Rationale**: Invalid grant or malformed refresh response belongs to credential metadata. Other configured credentials can still serve the request.

### Decision 5: Direct handler and proxy transport share semantics

**Decision**: Implement 401 forced refresh in both `doOpenAISubscriptionProviderRequest` and `openAISubscriptionFailoverTransport`.

**Rationale**: HO-1178/1179 spread official OpenAI subscription auth across both direct provider handlers and proxy-style responses endpoints. A partial implementation would create endpoint-specific auth behavior.

## Alternatives Considered

### Treat 401 as retryable in `openAISubscriptionRetryableStatus`

Rejected. It does not refresh the credential and would affect non-subscription API-key requests.

### Only rely on scheduled/expiry refresh

Rejected. The issue explicitly requires refresh on 401, and OpenAI can reject tokens before local expiry.

### Always disable on first 401

Rejected. The issue explicitly requires refresh and retry same credential once before disabling/failover.

### Return failure immediately after refresh failure

Rejected. The issue requires trying the next credential and failing only when all configured credentials fail.

### Start OAuth reconnect automatically

Rejected. There is no UI/operator consent in this backend request path, and the issue asks for clear reauth error after all credentials fail.
