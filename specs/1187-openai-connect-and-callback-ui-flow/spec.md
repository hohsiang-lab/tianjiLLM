# Feature Specification: OpenAI connect and callback UI flow

**Feature Branch**: `HO-1187-openai-connect-and-callback-ui-flow`
**Created**: 2026-05-08
**Input**: Linear HO-1187 - `[FE] OpenAI connect and callback UI flow`
**State**: Todo planning only; production code starts only after Linear moves to `In Progress`.

## 摘要

在 TianjiLLM 管理 UI 補上完整的 OpenAI subscription connect 使用者流程：proxy admin 可以從現有 UI 入口啟動 OpenAI OAuth、看到可讀的 callback success/failure result page，並在成功後回到 `Credentials` 管理面確認新連線。此 issue owns user-visible UI flow，不重寫 HO-1175 已完成的 backend OAuth state/callback lifecycle，也不新增 raw token 顯示。

## Scope

- 在既有受 `sessionAuth` 保護的 UI surface 新增 OpenAI Connect entry point。
- Connect 必須從 authenticated UI session 啟動，未登入 request 沿用現有 `/ui/login` redirect 行為。
- Connect entry point 必須帶入明確 organization context，使用既有 `/ui/openai/connect?org_id=...` backend start endpoint。
- Callback success 必須 render 可讀 UI result page，告知 OpenAI subscription credential 已連線成功，並提供回到 `Credentials` 的路徑。
- Callback failure 必須 render 可讀、安全的 UI error page，涵蓋 provider denial、missing/expired/invalid state、missing code、token exchange failure、credential save failure。
- Callback result pages 必須與現有 `templ` + Tailwind + card/button/badge style 對齊，不引入 SPA framework。
- 成功 connect 後，Credentials list/detail 應能顯示新 credential 的 safe metadata；本 issue 只負責 flow 驗收，不擴大 credential list/detail scope。
- UI E2E 必須覆蓋 connect success、callback failure/error state、unauthenticated connect。

## Out of Scope

- Todo 階段 production code implementation。
- PKCE/state primitive、state consume-once lifecycle、public `/oauth/openai/callback` backend route、安全 credential persistence；這些由 HO-1175/HO-1176/HO-1184 sibling issues 負責。
- OpenAI subscription credential list/detail quota UI 的 read-only management surface；由 HO-1186 負責。
- Token refresh/test/disable/delete lifecycle；由 HO-1185 或後續 lifecycle issues 負責。
- Routing/failover/quota/spend attribution。
- Real OpenAI network call、real OpenAI account、real token 或 production OAuth credentials in tests。
- Remote OpenAI revoke/logout/delete。

## User Stories and Tests

### User Story 1 - 從 authenticated UI 啟動 OpenAI Connect (P1)

Proxy admin 可以在 `Credentials` 管理 UI 看到 OpenAI Connect CTA，點擊後由既有 `/ui/openai/connect` 建 state 並 redirect 到 configured OpenAI authorize URL。

**Independent Test**: UI E2E 登入後進 `/ui/credentials`，點擊 `Connect OpenAI`，使用 mock OpenAI authorize/token upstream，assert browser 被導到 mock authorize URL 且 query 包含 `state`、`code_challenge`、`code_challenge_method=S256`、`redirect_uri`。

**Acceptance Scenarios**:

1. Given proxy admin 已登入，when `/ui/credentials` renders，then page shows `Connect OpenAI` action。
2. Given admin 點擊 `Connect OpenAI`，when organization context is available，then request goes to `/ui/openai/connect?org_id=<org>` and redirects to configured OpenAI authorize endpoint。
3. Given OpenAI OAuth is misconfigured，when admin clicks connect，then UI returns a readable error and no raw config/secret is exposed。
4. Given no active UI session，when `/ui/openai/connect?org_id=...` is requested，then existing UI auth redirects or blocks before state creation。

### User Story 2 - Callback success 呈現可讀 result page (P1)

Admin 完成 OpenAI OAuth 後，callback page 不應只是極簡 HTML；它應清楚說明連線成功，讓使用者回到 Credentials 管理面。

**Independent Test**: UI E2E seeds/starts valid state through the connect flow, mock OpenAI redirects to `/oauth/openai/callback?state=...&code=...`，assert success page displays `OpenAI connected`、safe account metadata when available、and a link/button to `/ui/credentials`。

**Acceptance Scenarios**:

1. Given valid state and successful token exchange，when callback returns，then page shows success state with clear title。
2. Given token response includes safe email/account metadata，when success page renders，then it may show safe account identifier but never raw token material。
3. Given credential is saved，when user clicks the result page action，then browser can navigate back to `/ui/credentials`。
4. Given callback path has no `/ui` cookie because cookie scope is `/ui`，when state is valid，then callback still succeeds and renders the public result page。

### User Story 3 - Callback failure 呈現可讀、安全 error page (P1)

Admin 遇到 denied/expired/error callback 時，需要看到可讀錯誤與 retry direction，而不是 raw `http.Error` 或 secret-bearing upstream text。

**Independent Test**: UI E2E directly visits callback failure URLs for provider `error=access_denied`、missing/expired/invalid state、token exchange failure，assert readable error UI and no token/code/verifier/raw upstream payload in DOM。

**Acceptance Scenarios**:

1. Given OpenAI returns `error=access_denied`，when callback renders，then page shows a readable denial/failure message。
2. Given state is missing, invalid, consumed, or expired，when callback renders，then page tells user to restart from TianjiLLM UI。
3. Given token exchange fails，when callback renders，then page shows safe failure state without raw upstream body。
4. Given any failure page is inspected，then DOM contains no `access_token`、`refresh_token`、`id_token`、authorization code、PKCE verifier、bearer string、JWT、encrypted credential blob。

### User Story 4 - Follow current UI conventions (P2)

Maintainer 需要 connect/result UI 看起來像 TianjiLLM 現有 management UI，而不是另一套 static page。

**Independent Test**: UI render tests or E2E assert result pages include TianjiLLM app styling/assets and use existing `templ` components/classes; implementation review verifies no React/Vue/Svelte/client framework addition。

**Acceptance Scenarios**:

1. Given callback success/failure page renders，then it uses existing Tailwind output and component style conventions。
2. Given page is viewed on mobile and desktop，then title/message/action do not overlap or clip。
3. Given the diff is reviewed，then it reuses `templ` + existing UI components and does not introduce new frontend packages。

## Functional Requirements

- **FR-001**: UI MUST expose a visible OpenAI Connect entry point from an authenticated management UI surface, defaulting to `/ui/credentials` unless implementation finds an existing more appropriate OpenAI credentials surface。
- **FR-002**: Connect entry point MUST require the existing UI session boundary before state creation。
- **FR-003**: Connect entry point MUST pass an explicit organization identifier to existing `/ui/openai/connect` and MUST NOT infer organization from callback query parameters。
- **FR-004**: Connect start MUST use existing OpenAI OAuth backend endpoint/config/state/PKCE helpers; implementation MUST NOT duplicate OAuth primitive generation。
- **FR-005**: Callback success MUST render a readable HTML UI page, not only bare text or raw `http.Error`。
- **FR-006**: Callback success page MUST provide a clear action back to `/ui/credentials`。
- **FR-007**: Callback success page MAY display safe account metadata such as email/account ID when it is already redacted/safe。
- **FR-008**: Callback failure MUST render readable HTML UI for provider error, invalid/missing/expired state, missing code, token exchange failure, and credential save failure。
- **FR-009**: Callback result pages MUST NOT expose raw token material, authorization code, code verifier, encrypted credential value, bearer strings, JWTs, or raw upstream response bodies。
- **FR-010**: Callback result pages MUST escape provider-supplied `error` and `error_description` before rendering。
- **FR-011**: UI E2E MUST cover successful connect with mocked OAuth/token upstream。
- **FR-012**: UI E2E MUST cover callback failure/error state。
- **FR-013**: UI E2E MUST prove unauthenticated connect is blocked or redirected by existing UI behavior。
- **FR-014**: Tests MUST run offline using local mock endpoints and MUST NOT call live OpenAI。
- **FR-015**: Implementation MUST preserve HO-1175 backend security guarantees: server-side state, consume-once state, callback public but state-authenticated, and stored-state `org_id` only。
- **FR-016**: No new frontend framework/package may be added for this UI flow。

## Key Entities

- **OpenAI Connect Entry Point**: Authenticated UI action that starts `/ui/openai/connect?org_id=...`。
- **OpenAI OAuth Callback Result Page**: Public user-visible page rendered by `/oauth/openai/callback` after terminal success/failure。
- **Connect Result Action**: Link/button from callback result page back to `/ui/credentials`。
- **Safe Account Metadata**: Redacted metadata already allowed by OpenAI subscription credential contracts, such as email/status, never raw tokens。
- **Mock OpenAI OAuth Upstream**: Local test server used by E2E to simulate authorize redirect and token exchange without live OpenAI。

## Edge Cases

- User clicks connect with no UI session。
- UI session exists but organization context is missing。
- OpenAI OAuth config is absent or invalid。
- Browser lands on callback with `error=access_denied`。
- Browser lands on callback without `state`。
- Browser lands on callback with consumed/expired/malformed state。
- Callback has `state` but no `code` and no provider `error`。
- Token exchange mock returns non-2xx or secret-looking response。
- Credential save fails after successful token exchange。
- Provider `error_description` contains HTML/script-like text。
- User refreshes/replays success callback after state was consumed。

## Success Criteria

- **SC-001**: UI E2E proves authenticated admin can start OpenAI Connect from the UI and reaches mock OpenAI authorize URL with PKCE/state query。
- **SC-002**: UI E2E proves unauthenticated connect is blocked or redirected and does not create usable OAuth state。
- **SC-003**: UI E2E proves callback success renders readable success UI and has a route back to `/ui/credentials`。
- **SC-004**: UI E2E proves callback provider/state/token failure renders readable safe error UI。
- **SC-005**: Security assertions prove success/failure DOM never contains token material, code verifier, bearer/JWT strings, encrypted credential value, or raw upstream response。
- **SC-006**: Existing HO-1175 backend callback tests, HO-1186 credentials UI tests, and affected UI tests remain green。

## Dependencies

- HO-1175: OpenAI OAuth connect/callback state lifecycle。
- HO-1176/HO-1184: OpenAI subscription credential persistence and safe CRUD/redaction boundary。
- HO-1186: Credentials sidebar/list/detail UI surface where the connect entry point should live by default。
- HO-1190 or existing `internal/testutil/openaitest`: local OpenAI OAuth mock harness。
- Existing UI patterns in `internal/ui/pages/layout.templ`、`internal/ui/pages/credentials.templ`、`internal/ui/components/*`。
