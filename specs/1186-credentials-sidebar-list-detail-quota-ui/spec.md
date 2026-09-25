# Feature Specification: Credentials sidebar list/detail quota UI

**Feature Branch**: `HO-1186-credentials-sidebar-list-detail-quota-ui`
**Created**: 2026-05-08
**Input**: Linear HO-1186 - `[FE] Credentials sidebar list/detail quota UI`

## 摘要

在 TianjiLLM 管理 UI 新增 sidebar `Credentials` 入口，讓 proxy admin 可以查看 OpenAI subscription credentials 的列表與 detail。UI 必須只呈現 safe metadata 與 quota/rate-limit state，包括 account email、status、utilization/quota usage、reset time、`last_refresh_at`、`last_error`、`disabled_reason`，不得顯示 access token、refresh token、JWT、bearer string、encrypted credential blob 或任何 secret-looking value。

## Scope

- 在既有 `internal/ui` Go `templ` + HTMX + Tailwind UI 中新增 `/ui/credentials` sidebar item。
- 新增 Credentials list page，列出 `credential_type = "openai_subscription"` rows，包含 credential name、account email、status、organization、quota summary、last refresh、last error summary。
- 新增 Credential detail page `/ui/credentials/{credential_id}`，顯示 safe metadata 與 quota/status cards。
- Detail quota UI 使用既有 card、table、progress bar、status badge、reset time pattern。
- Quota/rate-limit state 讀取既有 `callback.RateLimitStore` / `OpenAIQuotaState`，不可本地估算月費、subscription allowance、usage tier 或 remaining dollars。
- Safe metadata 來源沿用既有 redaction contract：`CredentialTable.credential_info`、`CredentialTable` safe columns、`RateLimitStore` known fields。
- UI E2E 覆蓋 list/detail navigation、quota/status rendering、safe metadata only。

## Out of Scope

- Todo 階段 production code implementation。
- Credential create/update/delete/test/refresh/disable action UI。
- 新增或修改 `/credentials/*` management API contract。
- 解密 `credential_value`、讀取 raw token bundle、呼叫 OpenAI API 補資料。
- Remote OpenAI revoke/logout/delete。
- Quota billing estimate、monthly spend estimate、subscription entitlement inference。
- Generic `api_key` credential management UI；本 issue 聚焦 OpenAI subscription credentials。
- Schema migration，除非 In Progress 實作證明 safe metadata 無法由既有 tables/state 取得。

## User Stories and Tests

### User Story 1 - Proxy admin 從 sidebar 進入 Credentials list (P1)

Proxy admin 可以在左側 sidebar 看到 `Credentials`，點擊後進入 OpenAI subscription credential list，快速掃描每個 credential 的 safe status 與 quota summary。

**Independent Test**: UI E2E 登入後檢查 sidebar 出現 `Credentials`，點擊進入 `/ui/credentials`，seed 多筆 OpenAI subscription credentials，斷言列表 rows、account email、status badge、quota summary 可見。

**Acceptance Scenarios**:

1. Given admin 已登入，when sidebar renders，then `Credentials` nav item appears and active state follows `/ui/credentials`。
2. Given OpenAI subscription credentials exist，when admin opens list，then rows include credential name、email、status、last refresh、quota summary。
3. Given no subscription credentials exist，when admin opens list，then page shows empty state without error。
4. Given generic `api_key` credentials exist，when admin opens list，then they do not appear in this OpenAI subscription-focused list。

### User Story 2 - Admin 查看 Credential detail safe metadata (P1)

Proxy admin 可以開啟某筆 credential detail，確認 account email、status、refresh/error/disabled metadata，但不會看到任何 token material。

**Independent Test**: UI E2E 從 list 點進 detail，seed metadata 包含 `email`、`status`、`last_refresh_at`、`last_error`、`disabled_reason`，斷言 detail 顯示 safe fields 且 body 不含 seeded token-looking values。

**Acceptance Scenarios**:

1. Given credential safe metadata 完整，when detail renders，then account email、status、last refresh、last error、disabled reason are visible。
2. Given optional metadata missing，when detail renders，then UI shows `Unknown` / `Not recorded` style fallback，不把 unknown 當成 zero quota 或 active success。
3. Given credential ID 不存在，when detail URL is opened，then UI returns existing not-found pattern or safe error page。
4. Given credential belongs to wrong type，when detail URL is opened，then UI does not render token-related fields and uses safe error/empty behavior。

### User Story 3 - Quota/status UI 正確呈現 known/unknown state (P1)

Proxy admin 可以在 detail card/table 中看到 request/token limits、remaining、utilization、reset time 與 current quota status，並理解 unknown fields 是未知，不是 0。

**Independent Test**: E2E seed `RateLimitStore` with known request/token state、status `allowed`、`rejected`、`exhausted`、unknown cases，assert badges/progress bars/reset time text match state。

**Acceptance Scenarios**:

1. Given requests limit/remaining/reset are known，when detail renders，then request quota progress uses `(limit - remaining) / limit` from stored state and shows reset time。
2. Given token limit/remaining/reset are known，when detail renders，then token quota progress and table row are visible。
3. Given status is `rejected` or `exhausted` with future reset，when detail renders，then badge indicates blocked/exhausted and reset time is visible。
4. Given no quota state exists，when detail renders，then quota card says no upstream quota data yet and does not fabricate usage。

### User Story 4 - Safe metadata only, no secret display (P1)

Security reviewer 需要確定 UI 只呈現 redacted metadata；即使 DB seed 裡有 token-looking strings，HTML body 也不能包含 secrets。

**Independent Test**: E2E 或 handler-render test seed credential metadata/value with access token、refresh token、JWT-like text、bearer text、encrypted blob，assert rendered list/detail HTML omits them while preserving safe fields。

**Acceptance Scenarios**:

1. Given encrypted credential value exists，when list/detail renders，then body never includes `credential_value` or encrypted blob。
2. Given metadata accidentally contains secret-looking strings，when UI data adapter builds view model，then output is redacted or omitted before render。
3. Given lifecycle `last_error` contains redacted text，when detail renders，then it shows safe redacted text only。
4. Given browser source is inspected，then no token material appears in DOM、data attributes、links、HTMX attributes、or hidden inputs。

### User Story 5 - UI follows existing Tianji patterns (P2)

Maintainer 需要 Credentials page 看起來像現有 API Keys、Usage、Organizations pages，避免新增另一套 layout 或 client-side framework。

**Independent Test**: UI handler/page tests or E2E verify route registration、templ render、table/card/badge classes align with existing components and no new JS framework/package is introduced。

**Acceptance Scenarios**:

1. Given implementation diff is reviewed，then it reuses existing `AppLayout`、sidebar components、card/table/badge/button/icon components。
2. Given detail page renders，then layout uses un-nested cards for repeated info groups and stable progress bar dimensions。
3. Given list page renders on narrow viewport，then rows/details remain readable without overlapping text。
4. Given implementation runs，then no new frontend framework/package is required。

## Functional Requirements

- **FR-001**: UI MUST add sidebar `Credentials` entry linking to `/ui/credentials`。
- **FR-002**: UI MUST expose a protected credentials list route under existing `sessionAuth` boundary。
- **FR-003**: List route MUST show only `credential_type = "openai_subscription"` credentials for this issue。
- **FR-004**: List route MUST display credential name、account email、status、organization ID、last refresh、last error summary、quota summary when known。
- **FR-005**: UI MUST expose protected detail route `/ui/credentials/{credential_id}`。
- **FR-006**: Detail route MUST display account email、status、quota utilization/usage、reset time、`last_refresh_at`、`last_error`、`disabled_reason`。
- **FR-007**: Detail quota UI MUST render request and token quota dimensions separately when known。
- **FR-008**: Unknown quota fields MUST render as unknown/not recorded and MUST NOT be treated as zero capacity, zero utilization, or healthy active quota。
- **FR-009**: Status badges MUST distinguish at least active/allowed、disabled、refresh_failed/auth_failed、rejected/exhausted、unknown。
- **FR-010**: UI MUST reuse existing `templ` components and `AppLayout`; no React/Vue/Svelte/client SPA framework may be added。
- **FR-011**: UI MUST NOT decrypt credential values or read raw token bundle for display。
- **FR-012**: Rendered HTML MUST NOT include raw access token、refresh token、`id_token`、JWT、bearer string、encrypted credential value、fallback API key、or raw OpenAI account payload。
- **FR-013**: Quota UI MUST consume existing `OpenAIQuotaState`/`RateLimitStore` semantics; it MUST NOT invent billing quota or subscription allowance locally。
- **FR-014**: E2E tests MUST cover list/detail and quota/status rendering。
- **FR-015**: Tests MUST include safe metadata/no-token assertions for rendered DOM。
- **FR-016**: Implementation MUST remain offline-testable with seeded DB rows and in-memory quota state; no real OpenAI network call is allowed。
- **FR-017**: Existing Credentials API lifecycle endpoints and API-key behavior MUST remain compatible。

## Key Entities

- **OpenAI Subscription Credential**: Existing `CredentialTable` row with `credential_type = "openai_subscription"`。
- **Safe Credential Metadata**: Redacted `credential_info` fields such as `email`、`status`、`last_refresh_at`、`last_error`、`disabled_reason`。
- **Credential List Row**: UI view model for list page, built from safe DB columns and optional quota summary。
- **Credential Detail View**: UI view model for one credential plus optional `OpenAIQuotaState`。
- **Quota Dimension View**: Request/token quota row with known flags, limit, remaining, utilization, reset time。
- **Quota Status Badge**: UI label derived from safe metadata and stored quota state, never from raw token inspection。

## Edge Cases

- No OpenAI subscription credentials exist。
- `credential_info` is empty or malformed JSON。
- `last_refresh_at` is absent or invalid。
- `last_error` is long; UI must wrap/truncate without overlap and without losing redaction。
- Quota state missing entirely。
- Quota state has only request dimension or only token dimension。
- Remaining is greater than limit; progress clamps to valid range and displays known raw numbers safely。
- Reset time is expired; UI should rely on normalized store state or show it as no active gate。
- Disabled credential has stale previous quota state; disabled metadata must be visible and not hidden by quota allowed state。
- Generic `api_key` credential exists in DB。

## Success Criteria

- **SC-001**: Sidebar E2E proves `Credentials` nav appears, links to `/ui/credentials`, and active state is correct。
- **SC-002**: List E2E proves seeded OpenAI subscription credentials render with email/status/quota summary and generic API-key credentials are excluded。
- **SC-003**: Detail E2E proves safe metadata fields and request/token quota dimensions render correctly。
- **SC-004**: Quota E2E proves allowed/rejected/exhausted/unknown states render distinct badges and reset time behavior。
- **SC-005**: Security test proves rendered DOM contains no token material、encrypted credential value、JWT、bearer strings、or raw account payload。
- **SC-006**: Existing UI, credential CRUD, lifecycle, and quota/routing tests remain green。

## Dependencies

- HO-1176: OpenAI subscription credential persistence。
- HO-1181: OpenAI quota/rate-limit header parser and `OpenAIQuotaState` store integration。
- HO-1184: Safe OpenAI subscription credential CRUD and redacted response shape。
- HO-1185: Test/refresh/disable lifecycle safe metadata。
- Existing UI patterns in `internal/ui/pages/layout.templ`、`keys.templ`、`key_detail.templ`、`usage.templ`。
- Existing E2E fixture patterns in `test/e2e/helpers_test.go` and keys tests。
