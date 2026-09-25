# Feature Specification: OpenAI Subscription Spend and Audit Attribution

**Feature Branch**: `HO-1182-openai-subscription-spend-and-audit-attribution`
**Created**: 2026-05-07
**Input**: Linear HO-1182 - `[BE] OpenAI subscription spend and audit attribution`

## Summary

Attribute OpenAI subscription traffic to the selected credential, provider, and organization in the existing spend/error/callback flows, and add audit coverage for OpenAI subscription credential lifecycle actions. The feature must keep existing `api_key` traffic behavior unchanged and must route every audit/error/callback payload through the existing redaction boundary so no token, JWT, raw OAuth response, or encrypted credential blob is persisted or emitted.

## Scope

- Add subscription credential attribution to successful spend records for direct official OpenAI subscription traffic.
- Attribute failed subscription attempts in error/callback/audit surfaces where the current request context can identify the credential.
- Preserve existing `api_key`, `provider`, `organization_id`, `team_id`, `user`, and `upstream_token_key` behavior for non-subscription traffic.
- Add audit events for OpenAI subscription `connect`, `refresh`, `delete`, `test`, and `disable` lifecycle actions.
- Include safe audit fields: `credential_id`, `provider`, `organization_id`, `action`, `status`, `reason_code`, `request_id`, and safe metadata.
- Ensure all audit, callback, error, and spend metadata payloads are redacted before persistence or outbound callbacks.

## Out of Scope

- Production implementation while Linear is Todo.
- New UI screens or dashboard filters.
- Billing/quota estimation or monthly subscription allowance calculation.
- OpenAI OAuth connect/callback state lifecycle, credential persistence, refresh manager, endpoint injection, routing/failover, quota parser/store, or shared redaction implementation owned by sibling HO-1175/1176/1177/1178/1179/1180/1181/1183.
- Schema changes unless implementation proves existing `SpendLogs.metadata`, `AuditLog.updated_values`, `ErrorLogs`, and callback payloads cannot carry the required safe attribution.
- Real OpenAI network calls or real subscription credentials in tests.

## User Stories and Tests

### User Story 1 - Spend Records Identify Subscription Credential (Priority: P1)

As an operator, I want OpenAI subscription requests to be attributed by credential, provider, and organization in spend records so usage can be traced back to the selected subscription credential without storing bearer tokens.

**Independent Test**: A handler/callback test sends a successful official OpenAI subscription request through a configured credential and asserts the captured `callback.LogData` and resulting `SpendLogs` params include `credential_id`, `provider=openai`, and `organization_id`, while no access token, refresh token, bearer value, JWT, or fallback API key appears.

**Acceptance Scenarios**:

1. Given `openai_subscription_credential_ids: ["cred_a"]` and a successful official OpenAI request, when spend logging runs, then attribution includes `credential_id=cred_a`, `provider=openai`, and the request organization when present.
2. Given multiple subscription credentials and failover selects `cred_b`, when the successful attempt is logged, then attribution uses `cred_b`, not the first configured ID.
3. Given no subscription IDs are configured, when an `api_key` request succeeds, then existing `api_key` spend attribution remains unchanged.
4. Given subscription bearer material is used upstream, when spend/callback payloads are inspected, then raw access tokens, refresh tokens, bearer values, JWTs, encrypted credential values, and raw account payloads are absent.

### User Story 2 - Failed Subscription Attempts Keep Safe Attribution (Priority: P1)

As an operator debugging failed subscription traffic, I want failures to carry stable credential/action/status reason codes so I can identify the affected credential without seeing secrets.

**Independent Test**: Endpoint tests force refresh failure, post-refresh 401, 429 gating, and all-credentials-failed cases, then assert failure callback/error/audit data includes safe credential IDs and reason codes only.

**Acceptance Scenarios**:

1. Given `cred_a` fails refresh and `cred_b` later succeeds, when failure logging runs for `cred_a`, then the failure payload includes `credential_id=cred_a`, `status=refresh_failed`, and a redacted reason.
2. Given a credential returns 401 after forced refresh, when it is disabled, then the audit/error payload includes `action=disable`, `status=failed_auth_after_refresh` or equivalent stable code, and no upstream sensitive body.
3. Given all configured credentials fail, when the request returns an aggregate error, then only credential IDs and reason codes are exposed.
4. Given OpenAI upstream error text contains token-shaped data, when any sink receives it, then the redacted marker replaces secret material.

### User Story 3 - Credential Lifecycle Actions Are Audited (Priority: P1)

As a security reviewer, I want OpenAI subscription credential lifecycle actions audited with actor, credential, organization, action, and status so credential operations are accountable.

**Independent Test**: Handler/service tests call connect, refresh success, refresh failure, delete, test, and disable paths and assert `InsertAuditLog` receives safe JSON with the expected action/status fields.

**Acceptance Scenarios**:

1. Given OAuth callback saves a new OpenAI subscription credential, when connect completes, then an audit row records `action=connect`, `status=success`, `credential_id`, `provider=openai`, and `organization_id`.
2. Given refresh succeeds, when credential metadata is updated, then audit records `action=refresh`, `status=success`, and safe metadata such as `last_refresh_at`.
3. Given refresh fails, when metadata records the failure, then audit records `action=refresh`, `status=failure`, and a redacted `reason_code`/`last_error`.
4. Given an operator deletes, tests, or disables a subscription credential, when the action completes or fails, then audit records the action and final status without `credential_value` or token fields.

### User Story 4 - API-key Spend and Audit Behavior Regressions Are Locked (Priority: P1)

As a maintainer, I need existing API-key traffic and management audit behavior to remain compatible while adding subscription attribution.

**Independent Test**: Existing API-key spend tests and credential audit tests are extended with negative assertions showing no subscription `credential_id` is fabricated for API-key traffic.

**Acceptance Scenarios**:

1. Given an API-key deployment, when a successful request is logged, then `SpendLogs.api_key` and token-hash based budget updates behave as before.
2. Given API-key traffic has no selected OpenAI subscription credential, when callback metadata is built, then no fake subscription `credential_id` is added.
3. Given existing credential CRUD audit uses `createAuditLog`, when non-subscription credentials are audited, then existing redacted payload shape remains compatible.
4. Given subscription attribution metadata is added, when tests inspect `UpdateVerificationTokenSpend`, then it is still called only for API-key traffic with `rec.APIKey`.

## Functional Requirements

- **FR-001**: System MUST carry the selected OpenAI subscription `credential_id` from the provider attempt loop into request context or callback data before success/failure logging.
- **FR-002**: System MUST attribute successful subscription requests with `credential_id`, `provider`, and `organization_id` where available.
- **FR-003**: System MUST attribute failed subscription attempts with `credential_id`, `provider`, `organization_id`, `action`, `status`, and stable reason code where available.
- **FR-004**: System MUST NOT store raw subscription bearer/access tokens in `SpendLogs.api_key`, `ErrorLogs.api_key_hash`, callback payloads, audit payloads, logs, or metadata.
- **FR-005**: System MUST preserve existing `api_key` spend attribution and budget update behavior when subscription IDs are omitted or empty.
- **FR-006**: System MUST NOT fabricate a subscription credential ID for API-key traffic, custom `api_base`, or OpenAI-compatible providers.
- **FR-007**: System MUST audit OpenAI subscription `connect`, `refresh`, `delete`, `test`, and `disable` lifecycle actions.
- **FR-008**: Audit payloads MUST include safe fields only: `credential_id`, `provider`, `organization_id`, `action`, `status`, `reason_code`, `request_id`, timestamps, and safe metadata.
- **FR-009**: Audit payloads MUST pass through `redact.JSONValue` or the HO-1183 shared equivalent before `InsertAuditLog` or management event dispatch.
- **FR-010**: Failure/error/callback messages MUST pass through `redact.String` or the HO-1183 shared equivalent before persistence or outbound dispatch.
- **FR-011**: Attribution MUST key subscription routing by stable credential/account IDs, never by raw token material.
- **FR-012**: Tests MUST cover successful subscription spend attribution, multi-credential failover attribution, failed-attempt attribution, lifecycle audit actions, redaction, and API-key regression.
- **FR-013**: Tests MUST use `httptest`, existing handler harnesses, and local mocks only; no test may call `api.openai.com`, `auth.openai.com`, or use real OpenAI credentials.

## Key Entities

- **Subscription Attempt Attribution**: Safe runtime facts for one official OpenAI subscription attempt: credential ID, provider, organization, status, and reason code.
- **Spend Attribution Metadata**: Safe `SpendLogs.metadata` and `callback.LogData` fields that identify the subscription credential used for a successful call.
- **Lifecycle Audit Event**: Safe audit payload for `connect`, `refresh`, `delete`, `test`, or `disable`.
- **API-key Regression Path**: Existing token-hash/API-key attribution path that must not receive subscription-only metadata.

## Edge Cases

- First configured credential fails but a later credential succeeds; success attribution must identify the serving credential.
- A 401 retry succeeds after forced refresh; spend attribution must still identify the same credential.
- A credential is disabled after post-refresh 401; audit must identify disable reason without storing the upstream body.
- Request context has no `organization_id`; attribution should omit or store empty org without failing the request.
- Callback logger runs asynchronously; selected credential attribution must be captured before goroutine dispatch.
- Cached responses may currently return before upstream dispatch; this issue must not fabricate new subscription spend for cache hits unless existing cache-hit spend behavior already logs it.
- API-key traffic with provider `openai` must remain distinct from subscription traffic.

## Success Criteria

- **SC-001**: Successful OpenAI subscription requests produce spend/callback attribution containing selected `credential_id`, `provider=openai`, and organization when available.
- **SC-002**: Multi-credential failover logs the final serving credential, not merely the first configured ID.
- **SC-003**: Connect/refresh/delete/test/disable actions produce redacted audit records with action/status.
- **SC-004**: API-key spend attribution and budget updates keep existing behavior.
- **SC-005**: Redaction tests prove no token, JWT, bearer value, encrypted credential value, raw account payload, or fallback API key leaks to spend/audit/error/callback sinks.

## Dependencies

- HO-1167: OpenAI subscription config and credential resolution.
- HO-1176: Encrypted credential persistence and safe metadata shape.
- HO-1177: Refresh manager and refresh failure metadata.
- HO-1178: Official endpoint subscription auth injection.
- HO-1179: Multi-credential routing and failover.
- HO-1180: 401 forced refresh/retry/disable behavior.
- HO-1181: OpenAI quota/rate-limit parser/store integration.
- HO-1183: Shared token/JWT redaction.
