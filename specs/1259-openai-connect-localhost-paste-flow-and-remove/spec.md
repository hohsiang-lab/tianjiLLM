# Feature Specification: OpenAI Connect localhost paste flow and remove `public_base_url`

**Branch**: `HO-1259-openai-connect-localhost-paste-flow-and-remove`
**Linear**: HO-1259
**Created**: 2026-05-08
**Status**: Todo planning only; no production code in this phase

## User Stories & Tests

### User Story 1 - 使用 localhost redirect 開始 OpenAI Connect (Priority: P1)

TianjiLLM admin 在 `/ui/credentials` 點擊 `Connect OpenAI` 後，原本 Tianji 頁面必須保留 credentials UI，並提供可開啟的 paste modal；OpenAI authorize 應在新分頁/新視窗開啟，該分頁建立 OAuth state + PKCE 後導向 OpenAI authorize URL；authorize query 的 `redirect_uri` 預設必須是 `http://localhost:1455/auth/callback`。

**Independent Test**: handler/unit 或 E2E 觸發 `/ui/openai/connect?org_id=...`，assert authorize URL query 包含 `redirect_uri=http://localhost:1455/auth/callback`、`state`、`code_challenge`、`code_challenge_method=S256`，且不讀取 `general_settings.public_base_url`。

**Acceptance Scenarios**:

1. **Given** OpenAI OAuth config 未設定 redirect override，**When** admin 點擊 `Connect OpenAI`，**Then** Tianji 原頁保持在 `/ui/credentials` 並可開啟 paste modal，OpenAI authorize 在新分頁/新視窗開啟且 authorize URL 使用 localhost callback。
2. **Given** 測試或未來支援模式設定 explicit redirect URI，**When** connect flow 建立 state，**Then** state record 保存該 exact redirect URI。
3. **Given** `general_settings.public_base_url` 未設定，**When** connect flow 開始，**Then** OpenAI OAuth 不因缺少 `public_base_url` 失敗。

---

### User Story 2 - 貼上完整 localhost callback URL 完成連線 (Priority: P1)

使用者完成 OpenAI 授權後，瀏覽器可能停在 `http://localhost:1455/auth/callback?code=...&state=...` 且 localhost 沒 listener；TianjiLLM 必須在 `/ui/credentials` 提供 `Paste callback URL` 按鈕與 modal form，讓使用者貼上整段 URL，由 server-side parse `code` / `state` / `error` 並完成 token exchange。

**Independent Test**: E2E 或 handler test 先走 connect 產生 state，再 POST 完整 localhost URL 到 Tianji paste endpoint，assert server 使用 stored `code_verifier` 和 stored `redirect_uri` exchange token，最後保存 OpenAI subscription credential。

**Acceptance Scenarios**:

1. **Given** valid stored state 和 pasted URL 含 `code` + `state`，**When** 使用者提交 paste form，**Then** credential 被保存並顯示安全成功頁。
2. **Given** pasted URL 含 provider `error`，**When** 使用者提交，**Then** state 被安全處理並顯示可讀錯誤，不 exchange token。
3. **Given** pasted URL 缺少 `code` 或 `state`，**When** 使用者提交，**Then** 顯示可讀錯誤且不保存 credential。
4. **Given** pasted URL 的 path/host 不符合 configured redirect URI，**When** 使用者提交，**Then** 拒絕該 URL 並避免 consume 不相干 state。

---

### User Story 3 - 移除 `public_base_url` 的 OpenAI OAuth critical path (Priority: P1)

OpenAI OAuth 不再從 Tianji hosted public base URL 推 callback，也不再把 `public_base_url` 視為必要 config。

**Independent Test**: `rg "PublicBaseURL|public_base_url|DeriveOpenAIRedirectURI|ValidateOpenAIRedirectURI"` 對 live runtime path 無 OpenAI OAuth 依賴；config loader tests 不再要求 `public_base_url` 以啟用 OpenAI OAuth；`proxy.yml` / proxy YAML config artifact 若存在，也不得保留 `general_settings.public_base_url`。

**Acceptance Scenarios**:

1. **Given** runtime config 沒有 `public_base_url`，**When** OpenAI Connect flow 執行，**Then** authorize + exchange 仍可成功。
2. **Given** codebase cleanup 完成，**When** 搜尋 runtime code，**Then** 沒有 live OpenAI OAuth path 呼叫 `DeriveOpenAIRedirectURI` 或讀 `GeneralSettings.PublicBaseURL`。
3. **Given** deployment/sample `proxy.yml` 或 proxy YAML config artifact 存在，**When** implementation cleanup 完成，**Then** `general_settings.public_base_url` 已被移除且 OpenAI OAuth 仍由 `openai_oauth.redirect_uri` 或預設 localhost callback 決定。
4. **Given** historical specs/docs 保留舊 decision，**When** 搜尋舊 spec，**Then** 必須能清楚辨識那些是 historical，不是 current implementation requirement。

---

### User Story 4 - 防止 pasted URL / code / state 外洩 (Priority: P2)

Pasted callback URL、authorization code、state、code verifier、token material 不得進入 HTML、log、audit metadata、error text 或 PR/test fixture output 的 user-facing surface。

**Independent Test**: 使用 sentinel `code` / `state` / token strings 走 success、provider error、invalid state、token exchange failure，assert DOM、handler error message、audit metadata 不含 raw pasted URL、`code`、`state`、token-like keys。

## Requirements

### Functional Requirements

- **FR-001**: System MUST remove `GeneralSettings.PublicBaseURL` from the OpenAI OAuth runtime dependency.
- **FR-002**: System MUST remove or replace `DeriveOpenAIRedirectURI` / `ValidateOpenAIRedirectURI` for OpenAI OAuth runtime behavior.
- **FR-003**: System MUST define explicit OpenAI OAuth authorize redirect ownership under `openai_oauth` or an equivalent typed config.
- **FR-004**: System MUST default authorize redirect URI to `http://localhost:1455/auth/callback`.
- **FR-005**: System MUST launch OpenAI authorize from the protected Tianji UI in a new tab/window (`target="_blank"` or equivalent) so the original Tianji page remains available for paste completion.
- **FR-006**: System MAY support an explicit redirect URI override for tests or future supported modes, but the override must be part of OpenAI OAuth config, not `public_base_url`.
- **FR-007**: System MUST persist the exact authorize `redirect_uri` in `openaioauth.StateRecord`.
- **FR-008**: System MUST send the same stored `redirect_uri` to `openai.ExchangeCode` that was sent to `openai.BuildAuthorizeURL`.
- **FR-009**: System MUST keep state short-lived and one-time-use.
- **FR-010**: System MUST provide a `/ui/credentials` paste button that opens a modal where the user can paste the full localhost callback URL.
- **FR-011**: System MUST parse pasted URL server-side and extract only expected OAuth fields.
- **FR-012**: System MUST reject malformed pasted URLs, unexpected callback host/path, missing state, invalid/expired state, missing code, and provider error cases with readable safe messages.
- **FR-013**: System MUST preserve the existing direct callback handler behavior where possible by routing direct callback and pasted URL through the same exchange/save service.
- **FR-014**: System MUST persist successful pasted callback credentials through the existing OpenAI subscription credential save path.
- **FR-015**: System MUST update E2E/unit fixtures and any `proxy.yml` / proxy YAML config artifacts so OpenAI OAuth no longer requires or documents `public_base_url`.
- **FR-016**: System MUST redact pasted URL, `code`, `state`, `code_verifier`, access token, refresh token, and ID token from logs, errors, audit metadata, and HTML.
- **FR-017**: Regression tests MUST cover authorize redirect, new-tab launch contract, credentials-page paste modal, pasted URL parsing, redirect URI identity, one-time state consume, and no secret leakage.

### Non-Goals

- Do not implement hosted redirect whitelist support for OpenAI public Codex OAuth client in this issue.
- Do not create a separate OpenAI OAuth client registration flow.
- Do not import or paste raw `auth.json` / token cache content.
- Do not change OpenAI subscription credential CRUD, quota, refresh, disable, delete, or spend attribution behavior beyond the connect lifecycle.

## Key Entities

- **OpenAIOAuthConfig**: typed OpenAI OAuth settings including endpoints, client ID, scopes, originator, flags, and explicit authorize redirect URI.
- **OpenAIOAuthStateRecord**: cache-backed one-time state containing state, code verifier, org ID, created/expiry timestamps, and exact redirect URI.
- **Pasted Callback URL**: user-submitted full URL from the browser address bar after OpenAI redirects to localhost.
- **OpenAI Subscription Credential**: existing durable credential saved after successful token exchange.

## Success Criteria

- **SC-001**: `Connect OpenAI` authorize query uses `http://localhost:1455/auth/callback` by default.
- **SC-002**: Token exchange request form uses the stored redirect URI exactly.
- **SC-003**: Runtime config and proxy YAML config artifacts no longer require, read, or include `public_base_url` for OpenAI OAuth.
- **SC-004**: User can complete connect by opening a paste modal from `/ui/credentials` and submitting the full localhost callback URL.
- **SC-005**: Replay of the same pasted URL fails after first successful or terminal state consume.
- **SC-006**: DOM/log/audit test sentinels show no raw pasted URL, code, state, verifier, or token leakage.
- **SC-007**: Starting OpenAI authorization does not navigate away from the Tianji paste UI in the original tab.

## Evidence

- Linear HO-1259 description and comments, 2026-05-08.
- Repo reality: `internal/ui/handler_openai.go` and `internal/proxy/handler/openai_oauth.go` currently derive redirect URI from `GeneralSettings.PublicBaseURL`.
- Repo reality: `internal/openaioauth.StateRecord` currently stores state, code verifier, org ID, created/expiry timestamps, but not redirect URI.
- Repo reality: current tracked proxy YAML fixtures are `test/fixtures/proxy_config_full.yaml` and `test/fixtures/mcp/proxy_config_mcp.yaml`; no tracked root `proxy.yml` exists in this worktree.
- Official Codex auth docs fetched 2026-05-08: browser login returns through a localhost callback and headless/blocked-localhost cases prefer device-code or localhost forwarding.
- Official `openai/codex` source fetched 2026-05-08: CLI login builds `http://localhost:{actual_port}/auth/callback` and notes port allow-list sync.
