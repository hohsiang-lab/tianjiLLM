# Feature Specification: OpenAI OAuth Connect/Callback State Lifecycle

**Feature Branch**: `HO-1175-openai-oauth-connect-callback-state-lifecycle`
**Created**: 2026-05-07
**Status**: Draft
**Input**: Linear HO-1175 - `[BE] OpenAI OAuth connect/callback state lifecycle`

> Informed by memory: no HO-1175-specific memory was found in daily notes or memory search. Related repo artifacts checked: HO-1174 OpenAI OAuth PKCE/config primitives, HO-1176 OpenAI subscription credential persistence, HO-1190 OpenAI OAuth mock harness, and current UI/session/cache/server routing code.

## Scope Boundary

This issue owns the authenticated UI start endpoint and public OAuth callback lifecycle for OpenAI subscription credentials. It must bridge existing OpenAI PKCE/token helpers to existing encrypted credential persistence through a short-lived server-side state record.

**In scope**:

- Add `/ui/openai/connect` as an authenticated UI-session start handler.
- Add `/oauth/openai/callback` as a public callback handler outside API auth and outside the `/ui` cookie path dependency.
- Generate and store OpenAI OAuth `state`, PKCE `code_verifier`, and selected `org_id` server-side with TTL before redirecting to OpenAI.
- Build the authorize redirect using existing OpenAI OAuth config, public base URL redirect derivation, PKCE challenge, and state.
- Callback must load `org_id` only from server-side state, never from callback query parameters.
- Callback must exchange the authorization code using the stored `code_verifier`.
- Callback must persist the received token bundle through the OpenAI subscription credential persistence helpers.
- Callback must delete/consume state after terminal use.
- Callback failures must render a readable HTML error page without token material or raw upstream secrets.

**Out of scope**:

- PKCE primitive generation, OpenAI OAuth defaults, endpoint override plumbing, or redirect URI helper work already covered by HO-1174.
- Encrypted credential persistence helper internals already covered by HO-1176.
- OpenAI OAuth mock harness and no-real-network guard work already covered by HO-1190.
- Refresh scheduling, refresh retry, failover routing, model selection, subscription upstream injection, or UI management screens.
- Real OpenAI network calls or real OpenAI credentials in automated tests.
- Schema migration unless implementation proves existing cache/credential storage is insufficient.

## User Scenarios & Testing

### User Story 1 - Start OAuth From Authenticated UI Session (Priority: P1)

As an admin using the TianjiLLM UI, I want to start OpenAI OAuth only from an authenticated UI session, so subscription credentials cannot be connected by unauthenticated users.

**Independent Test**: Route/handler tests request `/ui/openai/connect` with and without a valid UI session cookie and assert unauthenticated requests are rejected while authenticated requests create state and redirect to the configured OpenAI authorize URL.

**Acceptance Scenarios**:

1. **Given** no valid UI session cookie, **When** `/ui/openai/connect?org_id=org_123` is requested, **Then** the request redirects to `/ui/login` or returns the existing UI unauthorized HTMX behavior.
2. **Given** a valid UI session and `org_id`, **When** `/ui/openai/connect` is requested, **Then** the server generates state and PKCE verifier, stores state server-side with TTL, and redirects to OpenAI authorization.
3. **Given** a missing or invalid `org_id`, **When** `/ui/openai/connect` is requested, **Then** no state is stored and a readable UI error is returned.
4. **Given** configured OpenAI OAuth endpoint overrides, **When** connect builds the authorization URL, **Then** the redirect uses the configured authorize endpoint and does not fall back to live OpenAI in tests.

---

### User Story 2 - Handle Callback With Server-Side State (Priority: P1)

As TianjiLLM, I want the OAuth callback to trust only server-side state, so callback query tampering cannot attach a token to the wrong organization.

**Independent Test**: Callback tests seed a state record with `org_id` and `code_verifier`, call `/oauth/openai/callback` with matching state and a conflicting query `org_id`, and assert persistence uses the stored `org_id`.

**Acceptance Scenarios**:

1. **Given** a valid stored state and callback `code`, **When** `/oauth/openai/callback?state=...&code=...&org_id=evil` is requested, **Then** token persistence uses the `org_id` from stored state and ignores the query `org_id`.
2. **Given** a valid stored state, **When** token exchange succeeds, **Then** the state is deleted/consumed and a readable success page is rendered.
3. **Given** the same callback is replayed after success, **When** the state is requested again, **Then** it fails safely as missing/expired state and no second token exchange occurs.
4. **Given** no UI cookie on `/oauth/openai/callback`, **When** a valid state and code are present, **Then** callback still succeeds because callback auth comes from server-side state, not the `/ui` session cookie.

---

### User Story 3 - Fail Safely for State and Provider Errors (Priority: P1)

As an admin, I want callback failures to be understandable and safe, so expired state, mismatches, denials, and token exchange errors do not leak secrets or leave reusable state.

**Independent Test**: Handler tests cover expired/missing state, state mismatch, OpenAI `error` callback, and token exchange failure; each returns readable HTML, no token material, and consumes any state record that was found.

**Acceptance Scenarios**:

1. **Given** a state value that is not in server-side storage, **When** callback is requested, **Then** the server renders a readable error page and performs no token exchange.
2. **Given** a stored state whose TTL has expired, **When** callback is requested, **Then** the server renders a readable expired-state error and performs no token exchange.
3. **Given** OpenAI redirects back with `error=access_denied` and a valid state, **When** callback is requested, **Then** the server consumes the state and renders a readable error page.
4. **Given** token exchange fails after a valid state is loaded, **When** callback returns an error page, **Then** the loaded state is deleted and the page does not include access token, refresh token, ID token, code verifier, or raw upstream response.

## Edge Cases

- State key lookup must require exact state match and must not support prefix/partial matching.
- State records must include expiry/TTL and should be rejected if expired even if the cache backend returns stale bytes.
- State record JSON must validate required fields before token exchange: `state`, `code_verifier`, `org_id`, and creation/expiry timestamps.
- State must be consumed only once. Replaying a callback after success or terminal provider error must fail safely.
- Callback must not require the UI session cookie because existing UI cookies are scoped to `/ui`.
- Callback must not trust `org_id`, `organization_id`, `credential_id`, or return path query parameters from OpenAI.
- OpenAI callback `error` and `error_description` values must be rendered in sanitized readable text only.
- Token exchange must use the stored PKCE verifier and must not send `client_secret`.
- Tests must run offline through endpoint overrides and the HO-1190 mock harness/guard.
- Implementation must not create a credential row until token exchange succeeds.

## Requirements

### Functional Requirements

- **FR-001**: System MUST expose `/ui/openai/connect` under the existing UI route tree.
- **FR-002**: `/ui/openai/connect` MUST require a valid authenticated UI session.
- **FR-003**: Connect MUST require an `org_id` selected from the UI request and MUST reject missing/empty values before state creation.
- **FR-004**: Connect MUST generate a fresh random OAuth `state` and PKCE verifier for each request.
- **FR-005**: Connect MUST store state server-side with TTL before redirecting the browser.
- **FR-006**: The stored state record MUST include `state`, `code_verifier`, `org_id`, and expiry metadata.
- **FR-007**: Connect MUST build the authorize URL with the existing OpenAI OAuth config, derived redirect URI, PKCE S256 challenge, and state.
- **FR-008**: System MUST expose `/oauth/openai/callback` as a public callback route that does not require API auth or UI cookie auth.
- **FR-009**: Callback MUST load the state record by exact `state` query value before token exchange.
- **FR-010**: Callback MUST reject missing, mismatched, malformed, or expired state safely without token exchange.
- **FR-011**: Callback MUST use `org_id` only from the stored state record.
- **FR-012**: Callback MUST ignore callback query/body `org_id`, `organization_id`, and `credential_id` for persistence decisions.
- **FR-013**: Callback MUST exchange the authorization code using the stored PKCE verifier and existing OpenAI exchange helper.
- **FR-014**: Callback MUST persist successful token exchange results through OpenAI subscription credential persistence helpers.
- **FR-015**: Callback MUST delete/consume loaded state after terminal success or terminal failure.
- **FR-016**: Callback MUST render readable HTML success/error pages.
- **FR-017**: Error pages MUST NOT expose access tokens, refresh tokens, ID tokens, authorization codes, PKCE verifiers, encrypted credential values, or raw upstream responses.
- **FR-018**: Tests MUST use local mock endpoints and guard against live `auth.openai.com` calls.

### Key Entities

- **OpenAI OAuth State Record**: Short-lived server-side record keyed by OAuth state; contains `state`, `code_verifier`, `org_id`, `created_at`, and `expires_at`.
- **OpenAI Connect Request**: Authenticated UI request to `/ui/openai/connect` that selects an organization and starts OAuth.
- **OpenAI Callback Request**: Public OAuth redirect request to `/oauth/openai/callback` containing `state`, either `code` or `error`, and untrusted provider query fields.
- **OpenAI Subscription Credential**: Existing encrypted credential type persisted after successful callback.

## Success Criteria

- **SC-001**: Tests prove unauthenticated `/ui/openai/connect` requests cannot create OAuth state.
- **SC-002**: Tests prove authenticated connect creates server-side state with `org_id` and PKCE verifier, then redirects to the configured authorize endpoint.
- **SC-003**: Tests prove callback success consumes state and persists the credential for the stored `org_id`.
- **SC-004**: Tests prove callback ignores query `org_id`/`organization_id` and uses only server-side state.
- **SC-005**: Tests prove expired/missing/mismatched state returns readable HTML and performs no token exchange.
- **SC-006**: Tests prove provider error and token exchange failure pages do not leak token material or code verifier values.
- **SC-007**: Tests prove the OAuth lifecycle runs offline with mock endpoints and no real OpenAI network access.

## Assumptions

- HO-1174 has already added OpenAI OAuth PKCE/config/authorize/token helpers and `general_settings.public_base_url`.
- HO-1176 has already added OpenAI subscription credential persistence helpers.
- HO-1190 has already added reusable OpenAI OAuth mock harness and no-real-network guard tests.
- Existing cache backends support TTL and delete operations suitable for short-lived OAuth state.
