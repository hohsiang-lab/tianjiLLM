# Feature Specification: OpenAI Subscription Token Refresh Manager

**Feature Branch**: `HO-1177-openai-subscription-token-refresh-manager`
**Created**: 2026-05-07
**Input**: Linear HO-1177 - `[BE] OpenAI subscription token refresh manager`

> Informed by memory: no direct memory/wiki hits for HO-1177; repo-adjacent artifacts HO-1167, HO-1176, HO-1183, and HO-1190 provide the credential-resolution, persistence, redaction, and mock-harness context.

## Summary

Add a backend refresh manager for stored `openai_subscription` credentials. Runtime credential resolution must return a usable access token when the stored token is still fresh, refresh before expiry using a fixed buffer when needed, suppress concurrent refresh stampedes per credential, persist rotated refresh tokens, and surface safe typed errors for unusable credentials without leaking token material or falling back to API keys.

## Scope

- Resolve usable OpenAI subscription access tokens for existing direct OpenAI HTTP and Codex app-server resolution boundaries.
- Refresh with `grant_type=refresh_token` before the stored access token enters the expiry buffer.
- Deduplicate concurrent refresh attempts for the same `credential_id`.
- Persist successful refresh results through the HO-1176 credential update helper, including rotated refresh token handling.
- Persist safe failure metadata through the HO-1176/HO-1183 metadata/redaction boundary.
- Return typed safe errors for missing, wrong type, disabled, malformed, expired/no-refresh-token, and refresh-failed credentials.

## Out of Scope

- UI for credential refresh status.
- New credential storage tables or schema migrations unless implementation proves an existing sqlc contract is insufficient.
- Starting a new OpenAI OAuth connect flow when a credential cannot refresh.
- Codex app-server protocol changes beyond passing already-refreshed `chatgptAuthTokens`.
- Real OpenAI network calls in tests.
- Background scheduled refresh; this issue refreshes on demand during credential resolution.

## User Stories & Tests

### User Story 1 - Reuse Fresh Tokens and Refresh Before Expiry (Priority: P1)

As a request path using an OpenAI subscription credential, I need credential resolution to reuse fresh access tokens and refresh tokens before expiry so requests do not fail with stale credentials.

**Independent Test**: Handler/provider unit tests seed an encrypted `openai_subscription` credential with deterministic expiry times. Fresh credentials return the existing access token without hitting the mock token endpoint; credentials inside the expiry buffer call the refresh endpoint and return the refreshed access token.

1. **Given** an active credential expires after the configured refresh buffer, **When** resolution runs, **Then** it returns the stored access token and does not call OpenAI.
2. **Given** an active credential expires inside the refresh buffer, **When** resolution runs, **Then** it posts a refresh-token grant to the configured OpenAI token URL and returns the refreshed access token.
3. **Given** an active credential is already expired but still has a refresh token, **When** resolution runs, **Then** it attempts refresh before returning an expired-credential error.

### User Story 2 - Deduplicate Concurrent Refresh Per Credential (Priority: P1)

As the proxy under concurrent load, I need only one upstream refresh call per credential so refresh token rotation does not stampede and invalidate otherwise usable credentials.

**Independent Test**: A test launches concurrent resolution calls against the same near-expiry credential and a blocking mock refresh endpoint. The mock records exactly one refresh request, and all callers receive the same refreshed token or same safe refresh error.

1. **Given** multiple goroutines resolve the same near-expiry credential concurrently, **When** refresh starts, **Then** only one upstream refresh request is in flight for that credential.
2. **Given** concurrent callers wait on the same refresh, **When** the upstream refresh succeeds, **Then** all callers receive the refreshed credential result.
3. **Given** concurrent callers wait on the same refresh, **When** the upstream refresh fails, **Then** all callers receive the same typed safe error and no token material is exposed.

### User Story 3 - Persist Rotated Tokens and Safe Metadata (Priority: P1)

As the credential store, I need refresh results and failures persisted atomically with safe metadata so later requests use the newest refresh token and operators can inspect status without seeing secrets.

**Independent Test**: Unit tests decrypt the stored bundle after refresh. A response with `refresh_token` replaces the old refresh token; a response without `refresh_token` preserves it. Failure metadata contains redacted error text and does not change the encrypted bundle.

1. **Given** a refresh response includes a new refresh token, **When** refresh succeeds, **Then** the encrypted stored bundle contains the new refresh token and not the old one.
2. **Given** a refresh response omits `refresh_token`, **When** refresh succeeds, **Then** the encrypted stored bundle preserves the old refresh token while updating access token and expiry.
3. **Given** refresh fails with upstream text containing token-shaped data, **When** failure metadata is stored, **Then** `last_error` and `disabled_reason` are redacted and the encrypted bundle is not rewritten.

### User Story 4 - Typed Safe Failure Modes (Priority: P2)

As callers of credential resolution, I need stable failure codes so direct HTTP and Codex app-server paths can distinguish missing, disabled, expired, and refresh-failed credentials without string parsing or leaking secrets.

**Independent Test**: Unit tests create each unusable credential condition and assert `errors.As` can read the typed code while `err.Error()` contains no access token, refresh token, bearer token, JWT, code verifier, or raw upstream body.

1. **Given** the configured credential ID is not found or has the wrong type, **When** resolution runs, **Then** the error code is `credential_missing` or `credential_wrong_type`.
2. **Given** metadata status is `disabled`, **When** resolution runs, **Then** the error code is `credential_disabled` and no refresh attempt occurs.
3. **Given** token bundle validation fails or no refresh token is available for an expiring credential, **When** resolution runs, **Then** the error code is `credential_malformed` or `credential_expired`.
4. **Given** the upstream refresh request fails, returns non-2xx, or returns invalid JSON, **When** resolution runs, **Then** the error code is `refresh_failed`.

## Functional Requirements

- **FR-001**: The manager MUST use `CredentialTable` rows with `credential_type = "openai_subscription"` and MUST reject any other credential type.
- **FR-002**: The manager MUST reuse an active access token when `expires_at` is after `now + refreshBuffer`.
- **FR-003**: The manager MUST refresh when `expires_at <= now + refreshBuffer`; default refresh buffer is `5m` unless implementation exposes an internal constant for tests.
- **FR-004**: Refresh requests MUST post `grant_type=refresh_token`, `refresh_token`, and OpenAI OAuth `client_id` to the resolved OpenAI token URL as `application/x-www-form-urlencoded`.
- **FR-005**: Refresh requests MUST NOT send `client_secret` or Basic authorization because the current OpenAI OAuth flow is public-client PKCE.
- **FR-006**: Successful refresh MUST compute a new `expires_at` from `expires_in` relative to a testable clock.
- **FR-007**: Successful refresh MUST update the encrypted token bundle through existing OpenAI subscription credential persistence helpers.
- **FR-008**: If refresh returns a non-empty `refresh_token`, the manager MUST persist it; if refresh omits `refresh_token`, the manager MUST preserve the previous refresh token.
- **FR-009**: Successful refresh MUST update safe metadata with `status=active`, `last_refresh_at`, and cleared or absent refresh failure fields.
- **FR-010**: Refresh failure MUST update safe metadata with `status=refresh_failed`, redacted `last_error`, and a safe `disabled_reason` or failure reason string without rewriting the encrypted bundle.
- **FR-011**: The manager MUST suppress duplicate in-flight refreshes per `credential_id` so concurrent callers share one upstream refresh result.
- **FR-012**: The manager MUST return a typed error carrying a stable code for missing, wrong type, disabled, malformed, expired, and refresh-failed credentials.
- **FR-013**: Error strings and persisted metadata MUST pass through HO-1183 redaction and MUST NOT expose access tokens, refresh tokens, ID tokens, bearer tokens, JWTs, code verifiers, or raw upstream error bodies.
- **FR-014**: Existing API-key behavior MUST remain unchanged when no `openai_subscription_credential_ids` are configured.
- **FR-015**: Direct OpenAI HTTP resolution MUST receive refreshed bearer material; Codex app-server resolution MUST receive refreshed `chatgptAuthTokens` material, not stale tokens.
- **FR-016**: Tests MUST use `httptest`/`openaitest` mocks and MUST NOT call `auth.openai.com` or `api.openai.com`.

## Key Entities

- **OpenAI Subscription Token Bundle**: Encrypted JSON in `credential_value` containing access token, optional refresh token during refresh parsing, expiry, and account ID.
- **OpenAI Subscription Credential Info**: Plain JSONB metadata containing safe status, email/scopes, `last_refresh_at`, `last_error`, and `disabled_reason`.
- **Refresh Manager**: Handler/service boundary that loads, validates, refreshes, persists, and returns a usable subscription credential.
- **Refresh Result**: Parsed OpenAI token response with access token, optional refresh token, token type, `expires_in`, scopes, and raw metadata as needed.
- **Refresh Error Code**: Stable typed error code used by resolver callers and tests.

## Edge Cases

- Credential does not exist.
- Credential exists but type is not `openai_subscription`.
- Credential metadata is malformed JSON.
- Credential metadata marks status `disabled`.
- Encrypted credential cannot decrypt with configured master key.
- Token bundle is malformed or missing access token, account ID, expiry, or refresh token when refresh is required.
- Stored token is already expired but refresh token is still present.
- Refresh response succeeds but omits `refresh_token`.
- Refresh response succeeds but omits/invalidates `expires_in`.
- Refresh endpoint returns `invalid_grant`, rate limit, 5xx, invalid JSON, or network error.
- Two credentials refresh concurrently; locking must isolate by credential ID, not global.
- Same credential refreshes concurrently across multiple handler instances/processes; this issue only guarantees in-process suppression unless implementation adds DB/distributed locking.

## Success Criteria

- **SC-001**: Fresh credential resolution performs zero refresh HTTP requests and returns the stored token.
- **SC-002**: Near-expiry credential resolution performs exactly one refresh and returns the refreshed token.
- **SC-003**: Concurrent same-credential refresh performs exactly one upstream refresh request.
- **SC-004**: Rotated refresh token is persisted and old refresh token is absent from decrypted storage.
- **SC-005**: Omitted refresh token preserves existing stored refresh token.
- **SC-006**: Refresh failure persists redacted metadata and preserves encrypted token bundle.
- **SC-007**: All failure modes expose typed codes without raw secrets.

## Alternatives Considered

- **Use `golang.org/x/oauth2.TokenSource` directly**: Rejected for the first implementation because Tianji already has OpenAI-specific endpoint overrides, public-client request-shape tests, raw response metadata extraction, and credential persistence semantics. The plan still follows the documented early-expiry pattern.
- **Global mutex around all subscription refreshes**: Rejected because it serializes unrelated credentials and can block healthy traffic behind one slow account.
- **Distributed DB/advisory lock**: Deferred because the Linear scope says per-credential refresh lock and current handler tests can enforce in-process stampede suppression; distributed locking can be added if multi-process deployment evidence requires it.
- **Mark credential disabled on every refresh failure**: Rejected because transient 5xx/network failures should not permanently disable an account. Persist `refresh_failed` metadata and reserve `disabled` for existing/operator-disabled or a later explicit terminal-failure policy.

## Dependencies

- HO-1167: Config and resolver boundary for `openai_subscription_credential_ids`.
- HO-1176: Encrypted persistence and refresh-token rotation helpers.
- HO-1183: Redaction of credential metadata and error sinks.
- HO-1190: Reusable OpenAI OAuth mock harness and no-real-OpenAI guard.
