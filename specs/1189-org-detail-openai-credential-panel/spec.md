# Feature Specification: Org detail OpenAI credential panel

**Feature Branch**: `HO-1189-org-detail-openai-credential-panel`
**Created**: 2026-05-08
**Input**: Linear HO-1189 - `[FE] Org detail OpenAI credential panel`

## 摘要

在既有 TianjiLLM `/ui/orgs/{org_id}` organization detail page 加入一個 OpenAI subscription credentials panel，讓 proxy admin 在查看 organization 時可以直接看到該 org 下面的 OpenAI credentials safe metadata、credential health、quota summary，並能點進既有 `/ui/credentials/{credential_id}` detail/actions。

此 issue 是 read-only org detail surface。它必須重用 HO-1186 已建立的 Credentials UI safe view model/status/quota 呈現邏輯，不新增 secret display path，不解密 `CredentialTable.credential_value`，也不新增 OpenAI live call。

## Scope

- 在 Org detail page 加入 `OpenAI credentials` section/panel。
- 從 `CredentialTable` 取得目前 org-scoped `credential_type = "openai_subscription"` rows。
- 顯示 safe metadata/status summary：credential name、safe account email、credential status、quota status/summary、last refresh、last error summary。
- 每筆 credential 提供連到既有 `/ui/credentials/{credential_id}` detail/actions 的 link。
- 顯示 org 無 credential 時的 empty state。
- 遵循現有 Org detail UI patterns：`AppLayout`、`card`、`table`、`badge`、`button`、HTMX/templ server-rendered pattern。
- UI E2E 覆蓋 org detail 有 credentials、empty state、detail link route。

## Out of Scope

- Todo 階段 production code。
- Credential create/edit/test/refresh/disable/delete action 直接嵌入 Org detail。
- 新增 schema migration 或新的 credential table。
- 新增 generic API-key credential UI。
- 解密 `credential_value`、讀取 raw token bundle、或呼叫 OpenAI API 取得 live account/quota。
- 改變既有 `/ui/credentials` list/detail/action behavior。
- 月費、subscription entitlement、remaining dollars、billing estimate。

## User Stories and Tests

### User Story 1 - Admin 在 Org detail 看到該 org 的 OpenAI credentials (P1)

Proxy admin 開啟 organization detail 時，可以在同頁看到該 org 所屬 OpenAI subscription credentials，快速判斷 account/status/quota。

**Independent Test**: UI E2E seed 一個 org、兩筆 `organization_id = orgID` 的 OpenAI subscription credentials、另一個 org 的 credential，開 `/ui/orgs/{orgID}` 後斷言只顯示該 org credentials。

**Acceptance Scenarios**:

1. Given org 有 OpenAI subscription credentials，when admin opens `/ui/orgs/{org_id}`，then `OpenAI credentials` panel shows matching rows。
2. Given credential belongs to a different org，when current org detail renders，then that credential is not visible。
3. Given generic `api_key` credential belongs to the org，when panel renders，then generic credential is excluded。

### User Story 2 - Admin 看到 safe metadata/status summary (P1)

Admin 可以在 Org detail panel 掃描 credential safe metadata，不需要先進 credentials list。

**Independent Test**: E2E seed credential info with safe fields and secret-looking fields，assert panel displays safe account email/status/quota summary and DOM does not contain secret material。

**Acceptance Scenarios**:

1. Given safe metadata includes `email`、`status`、`last_refresh_at`、`last_error`，then panel shows redacted/safe summary。
2. Given metadata missing or malformed，then panel uses `Unknown` / `Not recorded` style fallback without error。
3. Given `credential_info` contains token-looking values，then rendered Org detail HTML omits access token、refresh token、JWT、bearer string、encrypted blob。

### User Story 3 - Admin 點進 existing credential detail/actions (P1)

Admin 可以從 Org detail 的 credential row 進入既有 detail/actions page，避免新增第二套 lifecycle controls。

**Independent Test**: E2E click panel credential link and wait for `/ui/credentials/{credential_id}`，assert existing credential detail page renders。

**Acceptance Scenarios**:

1. Given panel row exists，when admin clicks credential name/action link，then route navigates to `/ui/credentials/{credential_id}`。
2. Given detail page exists from HO-1186/HO-1188，then Org detail does not duplicate Test/Refresh/Disable/Delete buttons。
3. Given credential is wrong type or missing，then it is not exposed from the org panel。

### User Story 4 - Empty state is explicit and non-blocking (P2)

Admin opening an org with no OpenAI subscription credentials should see a calm empty state, not a missing section or error.

**Independent Test**: E2E seed org without subscription credentials，open detail，assert panel title and empty message are visible。

**Acceptance Scenarios**:

1. Given org has no OpenAI subscription credentials，then panel says no OpenAI credentials connected for this organization。
2. Given `ListCredentialsByOrg` fails，then Org detail still follows existing safe failure behavior and does not leak internal values。

## Functional Requirements

- **FR-001**: Org detail route MUST include an OpenAI credentials panel/section on `/ui/orgs/{org_id}`。
- **FR-002**: Panel MUST show only credentials with `credential_type = "openai_subscription"` and `organization_id = current orgID`。
- **FR-003**: Panel MUST exclude credentials for other organizations and generic `api_key` credentials。
- **FR-004**: Panel MUST display credential name, safe account email, credential health status, quota status/summary, last refresh, and last error summary when available。
- **FR-005**: Panel MUST link each visible credential to existing `/ui/credentials/{credential_id}` detail/actions page。
- **FR-006**: Panel MUST show explicit empty state when no OpenAI subscription credentials exist for the org。
- **FR-007**: Panel MUST reuse existing Credentials UI safe metadata/status/quota formatting helpers or extract shared helpers rather than duplicating incompatible logic。
- **FR-008**: Panel MUST NOT decrypt or render `credential_value`。
- **FR-009**: Rendered HTML MUST NOT contain raw access token, refresh token, `id_token`, JWT, bearer string, encrypted credential blob, fallback API key, or raw OpenAI account payload。
- **FR-010**: Unknown quota/status fields MUST render as unknown/not recorded and MUST NOT be treated as active, healthy, or zero usage。
- **FR-011**: UI MUST reuse existing Go `templ` + HTMX + Tianji components and add no frontend framework/package。
- **FR-012**: Tests MUST include UI E2E for populated panel, empty state, link navigation, org isolation, and secret DOM omission。
- **FR-013**: Implemented UI/UX MUST align with the owner-approved mockscreen posted in the issue thread: placement after org overview cards and before Members, card/table hierarchy, title/copy tone, populated rows, empty state treatment, detail-link affordance, desktop table density, and mobile responsive layout must remain visually equivalent unless the owner approves a later mock revision。

## Key Entities

- **Organization**: Existing `OrganizationTable` row rendered by `/ui/orgs/{org_id}`。
- **OpenAI Subscription Credential**: Existing `CredentialTable` row with `credential_type = "openai_subscription"` and nullable `organization_id`。
- **Org Credential Panel Row**: Safe credential summary displayed on Org detail, derived from safe DB columns, parsed safe metadata, and optional `OpenAIQuotaState`。
- **Safe Credential Metadata**: Narrow `credential_info` fields such as `email`, `status`, `last_refresh_at`, `last_error`, `disabled_reason` after existing redaction。
- **Credential Detail Link**: Existing `/ui/credentials/{credential_id}` route from HO-1186/HO-1188。

## Edge Cases

- Org has no credentials。
- Org has only generic `api_key` credentials。
- Org has subscription credential with malformed `credential_info`。
- Credential has missing `organization_id` (`Master`) and must not appear in org-scoped panel。
- Credential has disabled metadata but stale allowed quota state。
- Quota state missing entirely。
- `last_error` is long or includes redacted token-looking text。
- Another org has similarly named credential。

## Success Criteria

- **SC-001**: E2E proves `/ui/orgs/{org_id}` displays only the current org's OpenAI subscription credentials。
- **SC-002**: E2E proves org with no OpenAI subscription credentials displays explicit empty state。
- **SC-003**: E2E proves credential link from Org detail navigates to existing `/ui/credentials/{credential_id}` detail page。
- **SC-004**: Security E2E proves Org detail DOM does not contain credential secret material。
- **SC-005**: Existing Org detail and Credentials UI tests remain green。
- **SC-006**: Result-screen review captures desktop and mobile screenshots after implementation and confirms they match the approved mockscreen before the issue can move to `Waiting Merge`。

## Dependencies

- HO-1176: OpenAI subscription credential persistence。
- HO-1181: OpenAI quota/rate-limit state store。
- HO-1184: Safe OpenAI subscription credential CRUD/redaction。
- HO-1185: Credential test/refresh/disable lifecycle metadata。
- HO-1186: Credentials sidebar list/detail quota UI。
- HO-1188: Credential lifecycle actions UI。
