# Feature Specification: Credential Test/Refresh/Disable/Delete actions UI

**Feature Branch**: `HO-1188-credential-test-refresh-disable-delete-actions-ui`
**Created**: 2026-05-08
**Input**: Linear HO-1188 - `[FE] Credential Test/Refresh/Disable/Delete actions UI`
**State**: Todo planning only; production code starts only after Linear moves to `In Progress`.

## 摘要

在 TianjiLLM 既有 `/ui/credentials` OpenAI subscription credential list/detail UI 加上 lifecycle actions：`Test`、`Refresh`、`Disable`、`Delete`。操作必須沿用現有 HTMX confirm + toast/partial-update pattern，呼叫 HO-1185 已完成的安全 lifecycle API 與既有 credential delete API，成功或失敗後更新 UI 中 safe status metadata，不顯示任何 token、JWT、bearer、encrypted credential blob 或 raw upstream response。

> Informed by memory: `memory/2026-05-08.md:93` - HO-1185 lifecycle APIs already define test/refresh/disable semantics and redaction gates.
> Informed by wiki: `syntheses/code-review-patterns-learned-from-past-prs.md` Pattern #481 - credential lifecycle UI/review must treat responses, metadata, audit, toast, and DOM as secret egress boundaries.

## Scope

- 在既有 `internal/ui` Go `templ` + HTMX + Tailwind UI 中擴充 `/ui/credentials` list/detail actions。
- 在 credential list row 與 detail header/summary 區提供 `Test`、`Refresh`、`Disable`、`Delete` actions。
- `Test` 呼叫 backend `POST /credentials/openai-subscription/{credential_id}/test`，顯示 safe success/error toast，成功時顯示 models count。
- `Refresh` 呼叫 backend `POST /credentials/openai-subscription/{credential_id}/refresh`，成功後重新載入 row/detail safe metadata，顯示 `last_refresh_at`。
- `Disable` 呼叫 backend `POST /credentials/openai-subscription/{credential_id}/disable`，需 confirm，成功後 row/detail 顯示 disabled status 與 safe disabled reason。
- `Delete` 呼叫既有 backend `DELETE /credentials/delete/{credential_id}` 或 UI wrapper，需 destructive confirm；成功後 list 移除該 credential，detail 不再顯示已刪除 credential。
- 所有 action result 必須使用現有 `toast` component / HTMX out-of-band update pattern 或 repo 已有等價 pattern。
- UI E2E 必須覆蓋 Test/Refresh/Disable/Delete、confirm/toast behavior、安全 error rendering。
- 只處理 `credential_type = "openai_subscription"` rows；generic `api_key` credential UI 不在本 issue。

## Out of Scope

- Todo 階段 production code implementation。
- 新增 OpenAI OAuth connect/callback flow；HO-1187 已處理。
- 新增 credential CRUD backend、token refresh manager、OpenAI lifecycle backend semantics；HO-1184/HO-1185 已處理。
- Remote OpenAI revoke/logout/delete；`Disable` / `Delete` 只作用於 TianjiLLM local metadata/storage。
- 改動 DB schema、credential encryption format、OpenAI token bundle shape。
- 顯示或解密 `credential_value`、`access_token`、`refresh_token`、`id_token`、authorization code、raw OpenAI account payload。
- 新增 React/Vue/Svelte/client-side SPA framework 或新 frontend package。

## User Stories and Tests

### User Story 1 - Admin 在 Credentials list 操作 credential (P1)

Proxy admin 可以在 `/ui/credentials` list 直接對每個 OpenAI subscription credential 執行 `Test`、`Refresh`、`Disable`、`Delete`，並立即看到 toast 與 row 狀態更新。

**Independent Test**: UI E2E seed active OpenAI subscription credential，進 `/ui/credentials`，逐一點擊 `Test`、`Refresh`、`Disable`、`Delete`，使用 mock backend/upstream fixture，斷言 toast、row metadata、confirm 行為與列表移除。

**Acceptance Scenarios**:

1. Given active credential exists，when admin clicks `Test`，then UI sends lifecycle test request and displays success toast with safe result summary.
2. Given refresh succeeds，when admin clicks `Refresh`，then row/detail updates status metadata and `last_refresh_at`.
3. Given admin clicks `Disable` and accepts confirm，then UI marks credential disabled and shows safe disabled reason.
4. Given admin clicks `Delete` and accepts destructive confirm，then credential disappears from list and a success toast appears.

### User Story 2 - Admin 在 detail page 操作並看到 safe result (P1)

Proxy admin 可以在 `/ui/credentials/{credential_id}` detail page 執行同樣 lifecycle actions，完成後不需要手動 reload 就能看到 safe metadata 更新。

**Independent Test**: UI E2E 從 list 進 detail，點擊 each action，assert detail header/status cards/metadata cards 更新，toast 顯示成功或 safe error。

**Acceptance Scenarios**:

1. Given detail page open，when `Test` succeeds，then detail keeps current credential visible and shows safe success toast.
2. Given `Refresh` succeeds，when response includes `last_refresh_at`，then detail `Last refresh` changes to refreshed timestamp.
3. Given `Disable` succeeds，then detail status badge becomes `Disabled` and disabled reason is visible.
4. Given `Delete` succeeds from detail，then browser returns to `/ui/credentials` or renders a safe deleted state, never a stale secret-bearing page.

### User Story 3 - Confirm/toast/error behavior follows existing UI pattern (P1)

Maintainer 需要 lifecycle actions 看起來像現有 TianjiLLM management UI，不引入另一套 modal/toast/action framework。

**Independent Test**: E2E asserts destructive actions require browser confirm or existing confirm dialog, cancelled confirm does not mutate state, success/error toast uses existing component markup.

**Acceptance Scenarios**:

1. Given `Disable` or `Delete` is clicked，when admin cancels confirm，then no request mutates credential and no success toast appears.
2. Given lifecycle API returns error，when action completes，then UI shows safe error toast with stable reason code/copy.
3. Given action is in progress，then UI avoids duplicate submit where repo pattern supports it, or E2E proves duplicate click does not produce unsafe UI state.
4. Given implementation diff is reviewed，then it reuses existing `button`、`badge`、`toast`、HTMX partial patterns.

### User Story 4 - Secret material never leaks through action UI (P1)

Security reviewer 需要確定 lifecycle actions 不會把 backend response、metadata、upstream error 或 token-looking strings 透過 toast、DOM、hidden inputs、data attributes 洩漏。

**Independent Test**: E2E seed credential value/metadata/upstream error with token-looking strings and action failures; assert rendered DOM/toast/html never contains raw token material.

**Acceptance Scenarios**:

1. Given backend test/refresh/disable/delete responses are rendered，then DOM never contains `credential_value` or encrypted blobs.
2. Given upstream error includes bearer/JWT/API-key-looking text，then toast and metadata show redacted/safe reason only.
3. Given detail/list rows include form attributes，then no hidden input or HTMX attribute contains access token、refresh token、id token、bearer string、JWT、authorization code、raw account payload.
4. Given action error happens，then UI may show stable `reason_code` but not raw upstream body.

### User Story 5 - Existing read-only credentials UI remains compatible (P2)

HO-1186 list/detail quota UI must keep working after actions are added.

**Independent Test**: Existing credentials E2E continue passing; new action E2E covers quota/status interaction after lifecycle mutation.

**Acceptance Scenarios**:

1. Given generic `api_key` credentials exist，then they remain excluded from OpenAI subscription UI.
2. Given quota state exists，then action controls do not break quota cards/table rendering.
3. Given credential metadata is missing/malformed，then action buttons either disable safely or show safe error behavior.

## Functional Requirements

- **FR-001**: UI MUST render `Test`、`Refresh`、`Disable`、`Delete` actions for OpenAI subscription credential rows/detail.
- **FR-002**: UI action routes MUST remain protected by existing `sessionAuth` boundary.
- **FR-003**: `Test` action MUST call the existing lifecycle test backend for the selected credential and display only safe result summary.
- **FR-004**: `Refresh` action MUST call the existing lifecycle refresh backend and update safe metadata/status UI after success.
- **FR-005**: `Disable` action MUST call the existing lifecycle disable backend and require user confirmation before mutation.
- **FR-006**: `Delete` action MUST call existing credential delete contract through a UI-safe wrapper or route and require destructive confirmation.
- **FR-007**: Action success/failure MUST render toast via existing component/pattern; success copy must include action and credential name/id summary without secrets.
- **FR-008**: Action failure copy MUST use stable safe message/reason code and MUST NOT include raw upstream response body or token material.
- **FR-009**: After any successful action, the affected list row/detail view MUST refresh from DB-safe metadata instead of trusting stale client state.
- **FR-010**: `Disable` success MUST show disabled status/disabled reason in row/detail.
- **FR-011**: `Delete` success MUST remove the credential from list and route detail users back to list or safe deleted state.
- **FR-012**: Cancelled confirm MUST NOT send mutation or change UI state.
- **FR-013**: UI MUST NOT decrypt credential values or read raw token bundles to render action UI.
- **FR-014**: Rendered HTML/toast/attributes MUST NOT include raw access token、refresh token、`id_token`、JWT、bearer string、authorization code、encrypted credential value、fallback API key、or raw OpenAI account payload.
- **FR-015**: Implementation MUST reuse existing `templ` components、HTMX partial update pattern、`toast` component、and existing route style.
- **FR-016**: UI E2E MUST cover Test/Refresh/Disable/Delete happy paths.
- **FR-017**: UI E2E MUST cover lifecycle action error states and secret non-rendering.
- **FR-018**: UI E2E MUST cover confirm accept/cancel behavior for Disable/Delete.
- **FR-019**: Tests MUST run offline with seeded DB rows and mockable OpenAI upstream; no live OpenAI call is allowed.
- **FR-020**: Existing HO-1186 credentials list/detail/quota behavior MUST remain compatible.

## Key Entities

- **Credential Action Row**: List-row controls for one `openai_subscription` credential.
- **Credential Action Panel**: Detail-page action area for selected credential lifecycle operations.
- **Lifecycle Action Result**: Safe UI view model derived from backend status/reason code/models count/last refresh timestamp.
- **Action Toast**: Existing toast component instance rendered through HTMX response/OOB pattern.
- **Safe Credential Metadata**: Redacted `credential_info` fields such as `email`、`status`、`last_refresh_at`、`last_error`、`disabled_reason`.

## Edge Cases

- Credential is already disabled and admin clicks `Disable` again.
- Credential was deleted in another tab before action request.
- Lifecycle backend returns `credential_missing`、`credential_wrong_type`、`credential_malformed`、`refresh_failed`、`upstream_401`、`upstream_request_failed`.
- Test request succeeds but returns zero models.
- Refresh succeeds but `last_refresh_at` is absent.
- Disable succeeds while quota state still says allowed; disabled metadata must remain visible.
- Delete succeeds from detail page.
- Admin cancels confirm dialog.
- User double-clicks an action.
- `last_error` contains long or token-looking text.
- Credential name/email contains long text or HTML-like characters.
- Mobile viewport action controls wrap without overlapping table/detail content.

## Success Criteria

- **SC-001**: UI E2E proves list-page `Test` success shows safe toast and leaves row visible.
- **SC-002**: UI E2E proves `Refresh` updates `last_refresh_at` / status metadata from server-rendered safe data.
- **SC-003**: UI E2E proves `Disable` requires confirm, cancel does not mutate, accept marks disabled and shows toast.
- **SC-004**: UI E2E proves `Delete` requires confirm, cancel does not mutate, accept removes list row or returns detail to list.
- **SC-005**: UI E2E proves lifecycle error states render safe toast/copy.
- **SC-006**: Security assertions prove DOM/toast/attributes contain no token material、encrypted credential value、JWT、bearer strings、raw upstream response、or raw account payload.
- **SC-007**: Existing credentials list/detail/quota E2E remain green.

## Dependencies

- HO-1184: Safe OpenAI subscription credential CRUD and redacted response shape.
- HO-1185: Test/refresh/disable lifecycle APIs and delete idempotency/redaction regression gate.
- HO-1186: Credentials sidebar/list/detail quota UI.
- HO-1187: Connect UI entry point already places users on Credentials UI.
- Existing backend routes in `internal/proxy/server.go` for `/credentials/openai-subscription/{credential_id}/test|refresh|disable` and `/credentials/delete/{credential_id}`.
- Existing UI patterns in `internal/ui/pages/key_detail.templ`、`internal/ui/handler_keys.go`、`internal/ui/pages/orgs.templ`、`internal/ui/components/toast`.
