# Feature Specification: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

**Feature Branch**: `HO-1180-openai-401-retry-refresh-disable`
**Created**: 2026-05-07
**Status**: Draft
**Input**: Linear HO-1180 - `[BE] OpenAI 401 retry, refresh failure, and disable behavior`

## Summary

Handle OpenAI subscription authentication failures deterministically during direct official OpenAI routing. When an upstream OpenAI request returns 401 for a subscription credential, TianjiLLM must force-refresh that same credential once, retry the same request once with the refreshed access token, then disable or mark the credential unusable and fail over to the next configured credential if authentication still fails. The request must fail only after every configured credential has failed, and all stored metadata and returned errors must remain redacted.

## Scope

- Direct official OpenAI HTTP endpoints that already use `openAISubscriptionProviderAttempts` / `openAISubscriptionFailoverTransport`.
- 401 handling for OpenAI subscription credentials only.
- Force-refresh same credential once after a 401, even when the local expiry buffer said the token was fresh.
- Retry the same request body once with the refreshed bearer token.
- On forced refresh failure, mark safe failure metadata and try the next configured credential.
- On retry still returning 401, mark the credential disabled/unusable with safe metadata and try the next configured credential.
- Return a clear reauthorization error only when all configured credentials fail authentication or refresh.
- Preserve HO-1179 rate-limit/5xx failover behavior and no-api-key-fallback invariant.
- Keep `last_error` and `disabled_reason` redacted through the HO-1183 redaction boundary.

## Out of Scope

- UI for reconnecting OpenAI accounts.
- Starting a new OAuth connect flow automatically.
- Background scheduled refresh.
- Changing OpenAI subscription persistence schema.
- Applying this behavior to API-key deployments, custom `api_base`, OpenAI-compatible providers, Assistants-only endpoints outside the already subscription-aware official OpenAI proxy, or Codex app-server transport.
- Retrying after a streaming response body has already been relayed.
- Real OpenAI network calls in tests.

## User Stories and Tests

### User Story 1 - Refresh and Retry Same Credential Once on 401 (Priority: P1)

As a TianjiLLM operator, I want a subscription credential that receives a 401 to refresh immediately and retry once, so a locally fresh but server-revoked or rotated access token does not fail the request without using its refresh token.

**Independent Test**: Endpoint tests seed `cred_a` with `access-a`, return 401 for `access-a`, configure the OAuth mock to refresh `cred_a` to `access-a-refreshed`, and assert the same endpoint request is retried once with `Authorization: Bearer access-a-refreshed`.

**Acceptance Scenarios**:

1. Given `cred_a` is selected and the first upstream response is 401, when forced refresh succeeds, then Tianji retries the same upstream request once with the refreshed bearer token.
2. Given the same request body is retried, when the retry is sent, then method, path, body, content type, and OpenAI-specific headers are preserved except for the bearer token.
3. Given retry succeeds, when metadata is persisted, then the credential remains active with refreshed token metadata and no raw token material is logged or returned.
4. Given no subscription credential IDs are configured, when an API-key request returns 401, then Tianji does not attempt subscription refresh and preserves existing API-key behavior.

### User Story 2 - Disable and Fail Over After Retry Still Returns 401 (Priority: P1)

As a TianjiLLM client, I want the request to try another configured subscription credential after one credential stays unauthorized, so one revoked account does not fail a model while another configured account can serve it.

**Independent Test**: Endpoint tests seed `cred_a` and `cred_b`; upstream returns 401 for `access-a`, OAuth refresh returns `access-a-refreshed`, upstream returns 401 again for `access-a-refreshed`, then upstream succeeds for `access-b`. The test asserts `cred_a` receives safe disabled metadata and `cred_b` serves the response.

**Acceptance Scenarios**:

1. Given `cred_a` returns 401 before refresh and again after forced refresh, when `cred_b` is configured and usable, then Tianji disables or marks `cred_a` unusable and sends the request with `cred_b`.
2. Given `cred_a` is disabled after retry failure, when a later route builds candidates, then `cred_a` is excluded by the existing disabled-status filtering.
3. Given the upstream 401 body contains token-shaped text, when `last_error` and `disabled_reason` are stored, then raw access tokens, refresh tokens, bearer tokens, JWTs, and full upstream sensitive payloads are absent.
4. Given all configured credentials return 401 after their one forced refresh, when no usable credential remains, then Tianji returns a clear reauthorization-required error.

### User Story 3 - Fail Over on Refresh Failure (Priority: P1)

As a request path using multiple OpenAI subscription credentials, I want refresh failure for one credential to try the next credential, so an expired or revoked refresh token does not block other configured accounts.

**Independent Test**: Endpoint tests seed `cred_a` with a refresh token whose forced refresh endpoint returns `invalid_grant`; `cred_b` is usable. The test asserts `cred_b` is attempted and `cred_a` stores redacted `refresh_failed` metadata without rewriting the encrypted bundle.

**Acceptance Scenarios**:

1. Given a 401 triggers forced refresh and the refresh endpoint returns `invalid_grant`, when another credential is configured, then Tianji records safe refresh failure metadata and tries the next credential.
2. Given forced refresh returns malformed token JSON, when another credential is configured, then Tianji records safe refresh failure metadata and tries the next credential.
3. Given metadata persistence itself fails after refresh failure, when no safe credential state can be stored, then the request returns a safe upstream/auth error and does not leak token material.
4. Given forced refresh succeeds for a credential, when another concurrent request also sees 401 for the same credential, then duplicate refreshes are suppressed or reuse the latest stored refreshed token without a refresh stampede.

### User Story 4 - All Credentials Failed Returns Clear Reauth Error (Priority: P1)

As a TianjiLLM operator, I want an explicit safe error when every configured OpenAI subscription credential requires reauthorization, so the failure is actionable without exposing secrets.

**Independent Test**: Endpoint tests configure two credentials whose forced refresh/retry path fails; assert the HTTP response is a clear auth/reauth error and contains sanitized credential reason codes, not token strings or raw upstream bodies.

**Acceptance Scenarios**:

1. Given every configured credential fails refresh or still returns 401 after refresh, when the request completes, then Tianji returns a 401 authentication-style response with a clear reauthorization message.
2. Given `api_key` is present beside subscription IDs, when all subscription credentials fail, then Tianji does not fall back to `api_key`.
3. Given mixed failures occur, such as `refresh_failed`, `auth_failed_after_refresh`, and `credential_disabled`, when the aggregate error is built, then only stable reason codes and credential IDs are included.
4. Given callback/error/audit logging runs for the failed request, then error sinks receive redacted text only.

## Functional Requirements

- **FR-001**: System MUST detect upstream 401 responses from direct official OpenAI HTTP attempts that use OpenAI subscription credentials.
- **FR-002**: System MUST force-refresh the same credential once after a 401, even if the stored token was locally considered fresh.
- **FR-003**: System MUST retry the same upstream request at most once with the refreshed bearer token for that credential.
- **FR-004**: System MUST preserve request method, URL, body, content type, and endpoint-specific headers during the same-credential retry.
- **FR-005**: System MUST NOT retry indefinitely; per credential, one 401-triggered forced refresh and one retry is the maximum for a single client request.
- **FR-006**: System MUST fail over to the next configured subscription credential when forced refresh fails.
- **FR-007**: System MUST fail over to the next configured subscription credential when the refreshed retry still returns 401.
- **FR-008**: System MUST mark a credential that still returns 401 after forced refresh as disabled or otherwise unavailable to later candidate resolution.
- **FR-009**: System MUST persist safe `last_error` and `disabled_reason` metadata for refresh failures and post-refresh 401 failures.
- **FR-010**: System MUST use HO-1183 redaction for upstream 401 bodies, refresh errors, metadata, user-facing errors, logs, callbacks, audit payloads, and aggregate errors.
- **FR-011**: System MUST return a clear reauthorization-required error only after all configured credentials fail authentication/refresh.
- **FR-012**: System MUST NOT fall back to configured `api_key` when subscription IDs are present and all subscription credentials fail.
- **FR-013**: System MUST preserve HO-1179 429/5xx failover and rate-limit-header behavior.
- **FR-014**: System MUST leave non-subscription API-key 401 behavior unchanged.
- **FR-015**: Tests MUST use `httptest` and `internal/testutil/openaitest`; no test may call `api.openai.com` or `auth.openai.com`.
- **FR-016**: Tests MUST assert raw access tokens, refresh tokens, bearer values, JWT-like strings, fallback API keys, and upstream sensitive payloads are absent from HTTP responses and persisted metadata.

## Key Entities

- **401 Auth Failure Attempt**: A direct official OpenAI upstream attempt whose response status is 401 before the response body has been committed to the client.
- **Forced Refresh**: A refresh-token grant executed because upstream rejected the current bearer token, bypassing the normal local freshness check.
- **Same-Credential Auth Retry**: One replay of the original upstream request with the forced-refreshed bearer token for the same credential.
- **Auth-Failed Credential Metadata**: Safe `credential_info` state marking the credential unavailable after post-refresh 401, including `status`, `last_error`, and `disabled_reason`.
- **Reauthorization Required Error**: Sanitized aggregate error returned when every configured subscription credential failed refresh or post-refresh authentication.

## Edge Cases

- Upstream 401 body is malformed JSON or empty.
- Upstream 401 body contains `Authorization: Bearer ...`, JWT-shaped text, access token, refresh token, ID token, or fallback API key.
- Forced refresh succeeds but omits a rotated refresh token; existing HO-1177 persistence must preserve the old refresh token.
- Forced refresh succeeds, but retry returns 401.
- Forced refresh fails with network error, 400 `invalid_grant`, 429, 5xx, malformed JSON, or missing `expires_in`.
- Two concurrent requests see 401 for the same credential.
- A credential is already disabled by metadata before candidate selection.
- A streaming response has already started; this feature must not replay a partially delivered stream.
- Request body replay is impossible because a handler did not buffer or configure `GetBody`; implementation must restrict retry to paths that can safely rebuild the request.

## Success Criteria

- **SC-001**: 401 followed by successful forced refresh retries the same credential once and succeeds.
- **SC-002**: 401 followed by forced refresh failure tries the next credential.
- **SC-003**: 401 followed by refresh success and another 401 disables/marks the credential unavailable and tries the next credential.
- **SC-004**: All credentials failing authentication/refresh returns a clear reauthorization-required error and no API-key fallback.
- **SC-005**: Persisted metadata and user-facing errors are redacted.
- **SC-006**: Existing API-key routing, 429 failover, 5xx failover, and rate-limit-header gates keep passing.

## Dependencies

- HO-1167: OpenAI subscription config and credential resolution.
- HO-1176: Encrypted credential persistence and safe metadata helpers.
- HO-1177: Refresh manager, refresh-token rotation, typed refresh errors.
- HO-1178: Subscription bearer injection across official OpenAI endpoints.
- HO-1179: Multi-credential routing, rate-limit gating, and failover attempt loops.
- HO-1183: Shared redaction for token/JWT/error sinks.
- HO-1190: OpenAI OAuth/upstream mock harness and no-real-OpenAI guard.
