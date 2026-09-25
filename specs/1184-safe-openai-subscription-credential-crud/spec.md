# Feature Specification: Safe OpenAI Subscription Credential CRUD

**Feature Branch**: `HO-1184-safe-openai-subscription-credential-crud`
**Created**: 2026-05-08
**Input**: Linear HO-1184 - `[API] Safe OpenAI subscription credential CRUD`

## 摘要

在既有受保護的 `/credentials/*` 管理 API 上，定義 `credential_type = "openai_subscription"` 的安全 CRUD contract。API 必須能 create/list/info/update/delete 本地 OpenAI subscription credential，但管理回應不得暴露 encrypted `credential_value`、raw token、JWT、bearer string 或 raw account payload；delete 只刪 Tianji 本地 credential row，不做 remote OpenAI revoke/delete；權限沿用既有 session/auth/RBAC 慣例。

## Scope

- 支援建立 OpenAI subscription credential，寫入前驗證 token bundle 並加密 `credential_value`。
- 支援 list/info，僅回傳 safe metadata：ID、name、type、organization、timestamps、sanitized `credential_info`。
- 支援 update token bundle 與 safe metadata，但不得允許 `credential_type` 變更或 cross-type mutation。
- 支援 idempotent local delete，missing row 也視為成功 no-op，除非 DB operation 失敗。
- 保留既有 generic `api_key` credential 行為，除非需要補強 shared redaction response。
- 沿用 `/credentials` 既有 `AuthMiddleware` 與 `RoleProxyAdmin` 權限。

## Out of Scope

- Linear Todo 階段的 production code implementation。
- Credential management UI。
- Remote OpenAI OAuth revoke/logout/delete/API key delete。
- OAuth authorize/callback、token refresh scheduler、routing/failover、endpoint injection、quota parsing、spend attribution、redaction engine，這些由 HO-1175 到 HO-1183 sibling issues 負責。
- Schema migration，除非 In Progress 實作證明既有 `CredentialTable` 無法安全支援 contract。
- 任何管理 API 回傳 decrypted `credential_value`、access token、refresh token、`id_token`、JWT、bearer string、encrypted blob 或 raw account payload。

## User Stories and Tests

### User Story 1 - 安全建立 subscription credential (P1)

Proxy admin 可以提交 OpenAI subscription token bundle 與 safe metadata，讓後續 subscription features 能解析本地 credential，但不會在 HTTP response 洩漏 token。

**Independent Test**: Handler test 呼叫 create endpoint，輸入 `credential_type=openai_subscription`，斷言 DB 收到 encrypted canonical token bundle，HTTP response 不含任何 secret field。

**Acceptance Scenarios**:

1. Given valid token bundle，when create is called，then row has `credential_type=openai_subscription`、encrypted `credential_value`、safe `credential_info`。
2. Given metadata contains `access_token`、`refresh_token`、`id_token`、`credential_value`、JWT、bearer text 或 secret account payload，when create is called，then request is rejected or sanitized before persistence。
3. Given response body is inspected，then it includes only safe fields and never includes encrypted/raw credential material。

### User Story 2 - List/info 不暴露 secret (P1)

Proxy admin 可以用 safe metadata 辨識 credential 狀態與 ownership，但不能讀取 secret payload。

**Independent Test**: Seed subscription/API-key credentials with encrypted values and nested secret metadata，assert list/info responses only return safe fields。

**Acceptance Scenarios**:

1. Given subscription credentials exist，when list is requested，then every result omits `credential_value` and token fields。
2. Given credential ID exists，when info is requested，then detail includes safe metadata and omits encrypted/raw secrets。
3. Given `organization_id` filter is used，when list is requested，then filtering follows existing DB query path and redaction remains identical。

### User Story 3 - 安全 update credential material 與 metadata (P1)

Proxy admin 可以更新本地 OpenAI subscription credential 的 token bundle 與 safe metadata，不需要直接改 DB。

**Independent Test**: Handler test update an existing `openai_subscription` row，assert secret payload is validated/encrypted，safe metadata persisted，type changes and cross-type updates rejected。

**Acceptance Scenarios**:

1. Given existing subscription credential，when valid replacement token bundle is submitted，then stored `credential_value` is replaced with encrypted canonical bundle JSON。
2. Given update includes safe metadata such as `email`、`scopes`、`status`、`last_refresh_at`、`last_error`、`disabled_reason`，then metadata is sanitized and persisted。
3. Given update attempts to change `credential_type`、update an `api_key` row through subscription semantics、or store secret metadata，then request fails with sanitized client error。

### User Story 4 - Delete 是 idempotent 且 local-only (P1)

Proxy admin 可以刪除本地 OpenAI subscription credential row；這個動作不代表 OpenAI remote revoke。

**Independent Test**: Handler tests delete existing and missing credential IDs；both return successful idempotent local-delete response，and mocks assert no remote OpenAI client is invoked。

**Acceptance Scenarios**:

1. Given subscription credential exists，when delete is called，then row is deleted locally and response reports local deletion。
2. Given same credential is deleted again，when delete is called，then response remains successful and does not reveal whether secret material existed。
3. Given delete succeeds or no-ops，then no OpenAI revoke/logout/delete endpoint is called and audit/event payloads remain redacted。

### User Story 5 - 權限沿用既有 conventions (P1)

Maintainer 需要 subscription credential CRUD 維持既有 management auth boundary，避免低權限角色讀取或修改 credential metadata。

**Independent Test**: Middleware/RBAC tests verify `/credentials` remains `RoleProxyAdmin` and handler integration tests use existing authenticated route wiring。

**Acceptance Scenarios**:

1. Given unauthenticated request，when it reaches `/credentials/*`，then existing auth middleware rejects before handler behavior。
2. Given non-admin role，when it calls `/credentials/*`，then RBAC denies according to existing route permissions。
3. Given proxy admin calls the API，then CRUD behavior proceeds。

## Functional Requirements

- **FR-001**: System MUST support create/list/info/update/delete for credentials with `credential_type = "openai_subscription"`。
- **FR-002**: Create MUST validate and encrypt OpenAI subscription token bundle before DB write。
- **FR-003**: Create/update MUST reject malformed token bundles before persistence。
- **FR-004**: List/info responses MUST NOT include `credential_value`。
- **FR-005**: Responses MUST NOT include raw `access_token`、`refresh_token`、`id_token`、bearer strings、JWTs、encrypted credential blobs、or raw account payloads。
- **FR-006**: API responses MUST include safe metadata only: credential ID、name、type、organization ID、timestamps、sanitized `credential_info`。
- **FR-007**: Update MUST NOT allow changing an existing row's `credential_type`。
- **FR-008**: Update through subscription semantics MUST reject non-`openai_subscription` rows。
- **FR-009**: Update MUST support safe metadata updates plus optional secret token bundle replacement。
- **FR-010**: Delete MUST be idempotent and return success for missing rows unless DB operation itself fails。
- **FR-011**: Delete MUST remove only the local credential row and MUST NOT call remote OpenAI revoke/logout/delete APIs。
- **FR-012**: CRUD errors returned to clients MUST be sanitized and MUST NOT include secret-bearing input。
- **FR-013**: CRUD operations MUST keep existing `/credentials` `AuthMiddleware` and `RoleProxyAdmin` route permission behavior。
- **FR-014**: Implementation MUST use existing sqlc-backed DB interfaces for credential reads/writes; no ad hoc SQL in handlers。
- **FR-015**: Tests MUST run offline with synthetic token fixtures only and no real OpenAI credentials or network calls。

## Key Entities

- **OpenAI Subscription Credential**: Existing `CredentialTable` row with `credential_type=openai_subscription`。
- **Subscription Token Bundle**: Secret JSON payload encrypted into `credential_value`; contains `access_token`、`refresh_token`、`expires_at`、`account_id`。
- **Safe Credential Metadata**: Sanitized `credential_info` JSON containing non-secret fields such as `email`、`scopes`、`status`、`last_refresh_at`、`last_error`、`disabled_reason`。
- **Redacted Credential Response**: Management API response shape that excludes `credential_value` and secret-bearing fields。
- **Local Delete Result**: Idempotent local DB deletion outcome that does not imply remote OpenAI revocation。

## Edge Cases

- Create request omits `credential_type`; existing generic API-key default must remain compatible, but subscription tests must pass explicit `openai_subscription`。
- Token bundle has empty `access_token`、`refresh_token`、`expires_at`、or `account_id`。
- Metadata contains secret keys at nested map/array levels or secret-looking strings in free text。
- Update request targets a missing credential ID。
- Update request targets an existing `api_key` credential。
- Delete request targets a missing credential ID。
- DB delete returns an error after lookup succeeds。
- `organization_id` filter is present but no rows match。
- Invalid `credential_info` JSON should fail before DB write。

## Success Criteria

- **SC-001**: Targeted handler tests prove create/update encrypt canonical OpenAI subscription token bundle payloads and never return secrets。
- **SC-002**: List/info tests prove responses omit `credential_value`、token fields、JWTs、bearer text、and encrypted blobs。
- **SC-003**: Update tests prove metadata can be changed safely and credential type cannot be changed。
- **SC-004**: Delete tests prove local delete is idempotent and no remote OpenAI revoke path is used。
- **SC-005**: RBAC/middleware tests prove `/credentials` remains proxy-admin-only。
- **SC-006**: Affected Go tests pass offline without real OpenAI credentials or OpenAI network access。

## Dependencies

- HO-1176: Encrypted OpenAI subscription credential persistence and redacted list/info response shape。
- HO-1183: Shared token/JWT redaction boundary。
- HO-1182: Redacted audit lifecycle patterns for subscription credential delete/connect/refresh events。
- Existing `/credentials` route and RBAC behavior in `internal/proxy/server.go` and `internal/auth/rbac.go`。
