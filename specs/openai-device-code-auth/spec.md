# Feature Specification: OpenAI ChatGPT Device-Code Credential Login

**Branch**: `feat/openai-device-auth`
**Status**: Implementation approved by Norman; device code is the primary flow and browser callback remains the compatibility fallback.

## Goal

Let an authenticated TianjiLLM administrator add an OpenAI/ChatGPT subscription credential from a headless or remote browser environment by using OpenAI Codex's device-code login, without removing the existing localhost callback/paste flow.

## Scope

### In scope

- Add a protected device-login start action on `/ui/credentials`.
- Request the OpenAI Codex device code through the provider layer.
- Display only the verification URL and one-time user code to the authenticated administrator.
- Poll from a short browser-driven status request; never hold one HTTP request open for the whole authorization window.
- Store device authorization state in the existing cache abstraction, using shared reads and a cache lock when supported.
- Bind the pending flow to the creating UI session and selected organization scope.
- Handle pending, slow-down, expiry, denial, unavailable, transient, and malformed-provider responses safely.
- Complete the provider-issued authorization code with PKCE and persist through the existing encrypted OpenAI subscription credential path.
- Add cancel support and a safe status/error UI.
- Protect new state-changing UI actions with the existing server-side session plus a session CSRF token.
- Keep browser callback, pasted callback URL, refresh, credential CRUD, and existing schema unchanged.

### Out of scope

- Removing `/ui/openai/connect`, `/oauth/openai/callback`, or `/ui/openai/callback-url`.
- Calling OpenAI directly from browser JavaScript.
- Adding a credential-table/schema migration.
- Using `golang.org/x/oauth2.Config.DeviceAuth` or `DeviceAccessToken` for the provider-specific OpenAI JSON protocol.
- Registering a new OpenAI OAuth client or changing client/scope defaults.
- Real OpenAI credentials or real OpenAI network calls in automated tests.

## External contract

OpenAI Codex's current implementation uses these issuer-relative endpoints:

```text
POST {issuer}/api/accounts/deviceauth/usercode
POST {issuer}/api/accounts/deviceauth/token
GET  {issuer}/codex/device
POST {issuer}/oauth/token
```

The first request is JSON:

```json
{"client_id":"..."}
```

The response contains `device_auth_id`, `user_code` (or `usercode`), and an optional string interval. The poll request is JSON containing `device_auth_id` and `user_code`. A successful poll returns `authorization_code`, `code_challenge`, and `code_verifier`; the final OAuth token exchange uses redirect URI `{issuer}/deviceauth/callback` and the returned verifier.

Codex currently treats HTTP 403/404 from the device-token endpoint as authorization-pending. Standard RFC 8628 error codes are also recognized: `authorization_pending`, `slow_down`, `expired_token`, and `access_denied`. Unknown 4xx and 5xx responses are never rendered raw; transient failures are retried only within the pending flow TTL and bounded retry budget.

## State machine

```text
pending --(authorization pending)--> pending
pending --(slow_down)-------------> pending with interval + 5s
pending --(success)----------------> exchanging
pending --(expiry)-----------------> expired
pending --(access denied)----------> denied
pending --(bounded transient fail)-> pending or failed
exchanging --(token + save ok)------> success
exchanging --(exchange/save error)--> failed
```

A successful or terminal flow cannot be used again. A stale `exchanging` lease is failed rather than replaying a one-time authorization code.

## Security requirements

- `flow_id` is a fresh opaque random identifier; it is not the OpenAI `device_auth_id`.
- `device_auth_id`, user code, authorization code, PKCE verifier, and tokens stay server-side; none may appear in URLs, logs, audit metadata, or safe error text.
- The cache record contains no access token, refresh token, ID token, or credential value. After persistence it contains only the credential ID.
- The record includes selected `org_id` and a SHA-256 binding of the creating SCS session token. Status/cancel requests must match both the flow ownership and current permission middleware.
- Device status is a POST because polling can complete and persist a credential. It requires a session CSRF token and permission `credentials.manage`.
- Provider responses are bounded before decoding and error bodies are redacted.
- Final token exchange uses the stored/provider-issued PKCE verifier and must verify the returned challenge matches it.

## UI requirements

The credentials page keeps the existing browser callback launch and paste modal, and adds a primary `Sign in with Device Code` form. After start it shows:

- verification URL as a safe external link with `target="_blank"` and `rel="noopener"`;
- one-time user code in a copyable, escaped code block;
- expiry time and a clear phishing warning;
- pending/slow-down/exchanging/success/denied/expired/error status;
- a cancel action;
- a callback fallback when device login is unavailable.

Polling stops after a terminal result. No mockup was provided for this feature; the implementation must reuse the existing credentials-page, button, dialog, and Tailwind patterns rather than inventing a new visual system.

## Acceptance criteria

1. A permitted authenticated session can start device login and receives a verification URL, user code, expiry, and interval without exposing `device_auth_id`.
2. A different session, missing CSRF token, or insufficient permission cannot poll or cancel the device flow. Status/cancel requests explicitly specifying a different organization or scope are rejected with 4xx before polling, exchange, credential persistence, or cancellation; matching or absent scope uses the flow's stored organization. Browser and pasted callback semantics remain unchanged.
3. The provider client sends the exact JSON request shapes and issuer-relative URLs, and never sends a client secret.
4. Pending, slow-down, 403/404 pending, expiry, denial, malformed success, transient failure, and retry-budget behavior are deterministic and safe.
5. A successful device flow exchanges the provider-issued code/verifier and calls the existing encrypted `SaveOpenAISubscriptionCredential` path with the flow's stored organization.
6. Concurrent status requests cause at most one upstream poll/exchange/save operation; a second request observes the shared terminal state.
7. Browser callback and pasted callback regression tests remain green.
8. Automated tests use local mock endpoints and prove no request reaches `auth.openai.com`.
9. The final diff passes formatting, lint, targeted tests, build, and the repository's available CI checks; integration failures caused only by missing local PostgreSQL are reported separately.

## Evidence

- OpenAI Codex authentication docs: <https://developers.openai.com/codex/auth>
- Official Codex implementation: `codex-rs/login/src/device_code_auth.rs`
- Official Codex tests: `codex-rs/login/src/device_code_auth_tests.rs`
- RFC 8628: <https://www.rfc-editor.org/rfc/rfc8628>
- Go `x/oauth2` device implementation: `deviceauth.go` (Context7 and local `go doc` checked). Its helper uses form-encoded RFC 8628 `device_code` polling, so it is not wire-compatible with the OpenAI Codex JSON endpoint.
- Repo contracts: `specs/1175-openai-oauth-connect-callback-state-lifecycle`, `specs/1176-openai-subscription-credential-persistence`, and `specs/1259-openai-connect-localhost-paste-flow-and-remove`.
- ccc search was run from `/root/.hermes/profiles/coder/workspace/tianjiLLM`; grep.app was queried for public Go device-auth patterns; GitHub CLI inspected the official Codex source/PR; Google/web search located the current official documentation.
