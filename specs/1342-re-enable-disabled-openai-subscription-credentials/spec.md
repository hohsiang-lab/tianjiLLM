# Specification: HO-1342 Re-enable disabled OpenAI subscription credentials

## Scope

讓 Tianji 管理員可以從 UI/API 將 operator-disabled 的 OpenAI subscription credential 重新啟用。

本 issue owns OpenAI subscription credential 的 enable lifecycle endpoint、RBAC/route wiring、UI list/detail action 切換，以及 enable 後重新回到 existing credential resolution/routing 的 regression coverage。它不處理 OAuth reconnect、不改 refresh/auth failure 的判斷語意，也不改 delete 或 Codex usage snapshot 行為。

## Problem

目前 `/ui/credentials` 可以 disable OpenAI subscription credential，但 disabled 狀態不可從 UI 反轉。disabled row/detail 仍然顯示 `Disable`，沒有 `Enable` action。

Root cause chain: `OpenAISubscriptionCredentialDisable` 只把 `credential_info.status` 寫成 `disabled` 並設定 `disabled_reason=operator_disabled`；`loadOpenAISubscriptionCredential` 在 refresh/test/routing 前會拒絕 `status=disabled`；proxy routes、RBAC tests、UI action dispatcher、UI pages 只註冊 `disable`，沒有反向 `enable` lifecycle；所以 operator-disabled credential 永遠留在不可 routable metadata。

## User Stories

### US1 - Disabled credential shows Enable action (P1)

As a Tianji admin, I want disabled OpenAI subscription credentials to render `Enable` instead of `Disable`, so I can discover the recovery action directly from list/detail views.

**Independent Test**: UI tests seed an OpenAI subscription credential with `status=disabled` and `disabled_reason=operator_disabled`, then assert `/ui/credentials` and `/ui/credentials/{credential_id}` render an `Enable` action targeting the enable route and do not render the destructive disable action for that credential.

### US2 - Enable clears operator-disabled metadata (P1)

As a Tianji admin, I want enabling an operator-disabled credential to clear disable metadata and mark it active, so the same stored token bundle becomes resolvable again.

**Independent Test**: handler tests disable a fresh credential, call enable, assert metadata becomes active/allowed with empty `disabled_reason`, and assert `resolveUsableOpenAISubscriptionBundle` can load the credential again without refresh when the token is fresh.

### US3 - Enable is admin-only and redaction-safe (P1)

As a Tianji maintainer, I want enable to use the same admin-only lifecycle guard and redaction behavior as disable, so lower roles cannot reactivate credentials and no token material leaks in responses/audits.

**Independent Test**: RBAC/route tests assert proxy admin can call `/credentials/openai-subscription/{id}/enable`, team/internal/end users cannot, and handler audit/error tests prove access token, refresh token, bearer string, JWT-like values, and encrypted credential values are absent from response and audit logs.

### US4 - Non-operator disabled states remain safe (P2)

As a Tianji maintainer, I want enable semantics to be explicit for disabled credentials produced by auth/refresh failure, so operators do not accidentally hide a reauthorization-required failure as healthy.

**Independent Test**: lifecycle tests cover credentials with `disabled_reason=auth_failed_after_refresh` or `refresh_failed`; implementation rejects enable with a safe reason and leaves metadata unchanged.

## Functional Requirements

- **FR-001**: Tianji MUST add an admin-only proxy lifecycle endpoint for `POST /credentials/openai-subscription/{credential_id}/enable`.
- **FR-002**: The enable endpoint MUST only operate on `CredentialTypeOpenAISubscription` credentials.
- **FR-003**: Enabling an operator-disabled credential MUST set metadata to active/allowed semantics and clear `disabled_reason`.
- **FR-004**: Enable MUST preserve existing safe metadata such as email, scopes, last refresh time, and redacted last error unless implementation determines last error must be cleared for UI correctness.
- **FR-005**: Enable MUST NOT decrypt, replace, or expose `credential_value` when the action only changes lifecycle metadata.
- **FR-006**: Enable MUST be idempotent for already active credentials.
- **FR-007**: Enable MUST return safe JSON lifecycle output parallel to test/refresh/disable, with `action="enable"` and no secret fields.
- **FR-008**: Enable failures MUST use existing redacted `OpenAISubscriptionCredentialError` reason codes where possible.
- **FR-009**: Enable MUST write lifecycle audit entries with action `enable`, success/failure status, safe reason code, and no token/raw credential material.
- **FR-010**: Proxy router MUST register the enable route beside test/refresh/disable.
- **FR-011**: RBAC MUST keep the enable route proxy-admin only.
- **FR-012**: UI route registration MUST add `/ui/credentials/{credential_id}/enable` under the protected credentials routes.
- **FR-013**: UI action dispatcher MUST invoke the proxy enable lifecycle handler for action `enable`.
- **FR-014**: Credential list rows MUST render `Enable` instead of `Disable` when status is disabled or `disabled_reason` is non-empty.
- **FR-015**: Credential detail actions MUST render `Enable` instead of `Disable` for disabled credentials.
- **FR-016**: After successful enable from list/detail, UI MUST re-render the affected table/detail target with an active status and no disabled reason.
- **FR-017**: Existing test, refresh, disable, delete, Codex usage refresh, and usage tab behavior MUST remain unchanged.
- **FR-018**: Enable MUST reject non-operator disabled reasons such as `auth_failed_after_refresh` and `refresh_failed` with safe metadata unchanged.
- **FR-019**: Regression coverage MUST prove an enabled credential becomes resolvable/routable again through the existing OpenAI subscription resolution path.
- **FR-020**: Regression coverage MUST prove disabled credentials expose and execute enable from both UI list and detail contexts.

## Non-Goals

- Do not reconnect OAuth accounts or launch a new OAuth flow.
- Do not treat refresh/auth failure recovery as billing or routing source of truth.
- Do not alter credential deletion semantics.
- Do not change Codex usage snapshot fetching/cache/backoff.
- Do not change model route selection semantics beyond enabled credentials becoming resolvable by the existing resolver.
- Do not add background auto-enable behavior.

## Success Criteria

- **SC-001**: Disabled OpenAI subscription credentials show `Enable` in list and detail UI, while active credentials show `Disable`.
- **SC-002**: Calling enable on an operator-disabled credential clears `disabled_reason`, sets active/allowed status, and returns safe lifecycle JSON.
- **SC-003**: The same credential resolves through `resolveUsableOpenAISubscriptionBundle` after enable.
- **SC-004**: RBAC and route tests prove enable is proxy-admin only.
- **SC-005**: Audit/redaction tests prove enable never leaks token, bearer, JWT-like, encrypted credential, or raw upstream material.
- **SC-006**: Existing OpenAI subscription lifecycle/routing/UI tests remain green.

## Open Questions

None for Todo scope. Linear explicitly scopes restoration to operator-disabled credentials; refresh/auth failure recovery remains out of scope.
