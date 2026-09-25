# Feature Specification: OpenAI Credential Test/Refresh/Disable Lifecycle APIs

**Feature Branch**: `HO-1185-openai-credential-test-refresh-disable-lifecycle`
**Created**: 2026-05-08
**Input**: Linear HO-1185 - `[API] OpenAI credential test/refresh/disable lifecycle APIs`

## 摘要

在既有安全 OpenAI subscription credential CRUD、refresh manager、401 retry/disable、redaction、audit attribution 基礎上，新增受保護的 lifecycle management API：test、refresh、disable。這三個 API 必須 idempotent、可離線測試、只呼叫 mockable upstream，且回應只包含 safe status/error metadata，不得暴露 access token、refresh token、`id_token`、JWT、bearer string、encrypted blob 或 raw OpenAI response。

## Scope

- 新增 OpenAI subscription credential test API：必要時先 refresh，再用 credential bearer 呼叫輕量 OpenAI-compatible `/v1/models`。
- 新增 explicit refresh API：對指定 `openai_subscription` credential 執行 refresh-token grant，成功後更新 encrypted bundle 與 safe metadata。
- 新增 disable API：把指定 `openai_subscription` credential 標記為 disabled/unusable，不刪除 credential row。
- Explicitly preserve existing delete idempotency behavior for `openai_subscription` credentials as a regression gate because Linear Scope names `test/refresh/disable/delete behavior is idempotent`。
- test、refresh、disable 對 missing/already-disabled/refresh-failed 狀態需回傳 stable safe status，而不是 secret-bearing error。
- lifecycle actions 必須寫入既有 redacted audit payload，action 包含 `test`、`refresh`、`disable`。
- API 必須沿用 `/credentials` 既有 `AuthMiddleware` 與 `RoleProxyAdmin` 管理權限。

## Out of Scope

- Linear Todo 階段的 production code implementation。
- OpenAI OAuth authorize/callback、credential CRUD、routing/failover、quota parser、spend attribution、shared redaction implementation。
- Credential management UI。
- Remote OpenAI revoke/logout/delete/API key delete。
- 自動背景 refresh scheduler 或 cron。
- 修改 direct request routing 的 401 retry 行為；HO-1180 已覆蓋。
- Real OpenAI network calls or real subscription credentials in automated tests。
- Schema migration，除非 In Progress 實作證明既有 `credential_info` 無法保存必要 safe metadata。

## User Stories and Tests

### User Story 1 - Test credential 可安全驗證 upstream access (P1)

Proxy admin 可以要求 TianjiLLM 測試指定 OpenAI subscription credential 是否可用；系統必要時先 refresh，再呼叫 mockable `/v1/models`，回應只顯示 safe status。

**Independent Test**: Handler test seed fresh credential，呼叫 test endpoint，mock upstream 收到 `GET /v1/models` 與 subscription bearer，HTTP response 包含 `status=ok`、`credential_id`、safe model count 或 request metadata，不含任何 token。

**Acceptance Scenarios**:

1. Given credential token fresh，when test API is called，then it calls configured OpenAI upstream `/v1/models` once with the credential bearer。
2. Given credential token is stale，when test API is called，then it refreshes first and calls `/v1/models` with refreshed bearer。
3. Given `/v1/models` succeeds，then response includes safe status metadata only。
4. Given upstream returns 401/403/429/5xx，then response includes stable `reason_code` and redacted message only。

### User Story 2 - Explicit refresh 可重複安全執行 (P1)

Proxy admin 可以明確 refresh 指定 credential；重複 refresh 不應造成 secret 洩漏或 unsafe response。

**Independent Test**: Handler test calls refresh endpoint twice with mock token server fixtures，assert encrypted bundle and `last_refresh_at` update safely，response never includes raw token bundle。

**Acceptance Scenarios**:

1. Given active credential has valid `refresh_token`，when refresh API is called，then Tianji refreshes through configured OpenAI OAuth token endpoint and persists encrypted new bundle。
2. Given refresh response omits rotated refresh token，then existing refresh token is preserved by current persistence behavior。
3. Given refresh fails with `invalid_grant` or malformed token JSON，then response is safe, metadata records `refresh_failed`, and raw upstream body is not exposed。
4. Given refresh endpoint is called repeatedly，then each completed call returns deterministic safe status and does not leak token material。

### User Story 3 - Disable credential 是 idempotent 且 local-only (P1)

Proxy admin 可以停用 credential，讓後續 candidate resolution 不再使用它；重複 disable 不應失敗或洩漏 credential 曾否存在 secret。

**Independent Test**: Handler test calls disable endpoint for active credential twice，assert `credential_info.status=disabled` and `disabled_reason` are safe; second call returns same success/no-op status。

**Acceptance Scenarios**:

1. Given active credential，when disable API is called，then credential row remains, safe metadata marks `status=disabled` and `disabled_reason=operator_disabled`。
2. Given credential already disabled，when disable API is called again，then response is successful idempotent no-op。
3. Given disabled credential，when candidate resolution runs later，then existing disabled filtering excludes it。
4. Given disable succeeds，then no remote OpenAI revoke/logout/delete endpoint is called。

### User Story 4 - Lifecycle errors are redacted and audited (P1)

Security reviewer 需要 test/refresh/disable 的成功與失敗都可追蹤，但任何 response、audit、metadata 都不能包含 secret。

**Independent Test**: Tests inject upstream/token errors containing token-looking strings and assert HTTP response, `credential_info`, and audit payloads contain only redacted safe codes.

**Acceptance Scenarios**:

1. Given upstream error contains bearer/JWT/token-shaped text，when lifecycle API returns，then response and persisted metadata do not contain raw secret material。
2. Given lifecycle action succeeds，then audit includes `credential_id`、`provider=openai`、`action`、`status`、safe metadata。
3. Given lifecycle action fails，then audit includes stable `reason_code` and redacted metadata only。
4. Given target credential is wrong type or missing，then response is stable safe error and no decrypted secret is attempted。

### User Story 5 - Existing credential API compatibility remains intact (P1)

Maintainer 需要新增 lifecycle endpoints 不破壞既有 `/credentials` CRUD、API-key credentials、subscription routing。

**Independent Test**: Regression tests assert existing credential CRUD and non-subscription API-key credential behavior remain unchanged; lifecycle endpoints reject non-`openai_subscription` credentials.

**Acceptance Scenarios**:

1. Given `api_key` credential，when test/refresh/disable lifecycle API is called，then request is rejected with safe type-mismatch error。
2. Given existing CRUD list/info/update/delete calls，then response redaction remains unchanged。
3. Given no lifecycle route is called，then normal routing/failover behavior is unchanged。

## Functional Requirements

- **FR-001**: System MUST expose protected lifecycle APIs for test、refresh、disable of `credential_type=openai_subscription` credentials。
- **FR-002**: Lifecycle APIs MUST require existing `/credentials` auth boundary and `RoleProxyAdmin` permission。
- **FR-003**: Test API MUST refresh the credential first when current token is not fresh by existing refresh-buffer rules。
- **FR-004**: Test API MUST call configured/mockable OpenAI-compatible `GET /v1/models` using the selected subscription bearer。
- **FR-005**: Test API MUST NOT call real `api.openai.com` in tests; tests MUST use `internal/testutil/openaitest` or equivalent guarded transport。
- **FR-006**: Refresh API MUST reuse existing OpenAI refresh-token grant helper and configured endpoint override。
- **FR-007**: Refresh API MUST preserve existing refresh token when OpenAI token response omits a rotated refresh token。
- **FR-008**: Disable API MUST update local safe metadata only and MUST NOT call remote OpenAI revoke/logout/delete endpoints。
- **FR-009**: Disable API MUST be idempotent for already-disabled credentials。
- **FR-010**: Lifecycle APIs MUST reject wrong-type credentials with safe type-mismatch errors。
- **FR-011**: Lifecycle APIs MUST return safe response bodies containing stable `status`、`credential_id`、`action`、`reason_code` and safe metadata only。
- **FR-012**: Responses, persisted metadata, logs, and audit payloads MUST NOT include raw access tokens、refresh tokens、`id_token`、bearer strings、JWTs、encrypted credential blobs、fallback API keys、or raw upstream sensitive responses。
- **FR-013**: All lifecycle success/failure paths MUST emit redacted audit events through existing subscription attribution/audit helpers。
- **FR-014**: Implementation MUST use existing sqlc-backed credential store methods and shared OpenAI subscription helpers where possible。
- **FR-015**: Tests MUST prove CRUD and non-subscription API-key behavior remain compatible。
- **FR-016**: Existing delete behavior for `openai_subscription` credentials MUST remain idempotent and redacted; lifecycle implementation MUST NOT regress delete safe response/audit behavior。

## Key Entities

- **Lifecycle Credential Target**: A local `CredentialTable` row selected by `credential_id` and required to have `credential_type=openai_subscription`。
- **Credential Test Result**: Safe response shape for upstream `/v1/models` validation, including `status`、`reason_code`、optional safe model count/request ID。
- **Explicit Refresh Result**: Safe response shape for manual refresh result, including `status` and `last_refresh_at` without token payload。
- **Disabled Credential Metadata**: Sanitized `credential_info` state with `status=disabled`、`disabled_reason`、optional `last_error`。
- **Lifecycle Audit Event**: Redacted audit payload for `test`、`refresh`、`disable` actions。

## Edge Cases

- Credential ID missing or malformed。
- Credential row does not exist。
- Credential is `api_key` or another non-subscription type。
- Credential metadata is malformed JSON。
- Credential value decrypts but token bundle is malformed。
- Token is stale and refresh succeeds before test。
- Token is stale and refresh fails before test。
- `/v1/models` returns 401、403、429、5xx、malformed JSON、empty body、or token-shaped error text。
- Refresh endpoint returns `invalid_grant`、429、5xx、malformed JSON、missing `access_token`、missing `expires_in`、or no rotated refresh token。
- Disable called on already-disabled credential。
- Disable called after refresh/test failure metadata exists。
- Audit persistence fails after lifecycle action succeeds。

## Success Criteria

- **SC-001**: Test API proves fresh and stale credentials call mock `/v1/models` with the correct bearer and no real OpenAI network。
- **SC-002**: Refresh API proves successful and failed refresh paths persist safe metadata and never return token material。
- **SC-003**: Disable API proves active/already-disabled credentials return idempotent safe status and remain local-only。
- **SC-004**: Wrong-type/missing/malformed credentials return stable safe errors。
- **SC-005**: Audit tests prove `test`、`refresh`、`disable` success/failure payloads are redacted。
- **SC-006**: Existing credential CRUD, API-key credentials, and OpenAI subscription routing tests remain green。
- **SC-007**: Existing delete regression coverage remains green and proves delete idempotency/redaction is still explicit, not only covered by generic CRUD compatibility。

## Dependencies

- HO-1176: Encrypted OpenAI subscription credential persistence and helper methods。
- HO-1177: Refresh manager and refresh-token rotation semantics。
- HO-1180: 401 retry/refresh/disable metadata semantics。
- HO-1182: Redacted lifecycle audit attribution。
- HO-1183: Shared token/JWT redaction boundary。
- HO-1184: Safe OpenAI subscription credential CRUD response contract。
- HO-1190: OpenAI OAuth/upstream mock harness and no-real-OpenAI guard。
- OpenAI official docs: API auth uses HTTP Bearer authentication and `GET /v1/models` lists available models.
