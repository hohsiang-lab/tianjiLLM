# Feature Specification: OpenAI OAuth PKCE Primitives and Provider Config

**Feature Branch**: `HO-1174-openai-oauth-pkce`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: Linear HO-1174 — `[BE] OpenAI OAuth PKCE primitives and provider config`

## Scope Boundary

This issue is the first backend slice for OpenAI subscription OAuth support. It defines only the reusable OAuth primitives and OpenAI provider configuration needed by later issues.

**In scope**:

- PKCE verifier/challenge generation and random OAuth state generation.
- OpenAI OAuth authorize/token endpoint config with test override support.
- Public-client token exchange request shape for OpenAI OAuth: `client_id` + `code_verifier`, no `client_secret`.
- Redirect URI construction and validation for Tianji public base URL → `/oauth/openai/callback`.
- Config defaults/overrides for OpenAI OAuth client ID, issuer, authorize endpoint, token endpoint, scopes, originator, and feature flags used by the authorize URL.

**Out of scope**:

- Browser callback handler, credential persistence, refresh/retry/failover, UI screens, DB migrations, and model routing changes.
- Any cookie/browser/session/account-password scraping, OpenAI CLI login import, or shared machine credentials.
- Any production API call to OpenAI in tests; all token exchanges use local `httptest` mocks.

## Clarifications

### Session 2026-05-06

- Q: Should this issue create implementation code now? → A: No. Todo state requires SpecKit planning only: spec → plan → tasks. Production code starts only after Linear is moved to **In Progress**.
- Q: Should OpenAI OAuth use a confidential-client secret? → A: No. Use public-client PKCE only. Token exchange MUST NOT send `client_secret`.
- Q: Which client ID should be the default? → A: Align to the Linear issue: default public `client_id` follows OpenClaw/PI Codex-style public client, with config override.
- Q: How is redirect URI derived? → A: Align to the Linear issue: from Tianji public base URL plus `/oauth/openai/callback`; production requires HTTPS, while localhost is allowed only for dev/test.
- Q: Are endpoint URLs fixed? → A: Align to the Linear issue: OpenAI OAuth authorize/token endpoint config must have test overrides; production defaults target `https://auth.openai.com/oauth/authorize` and `https://auth.openai.com/oauth/token`.
- Q: Are any scope questions still open? → A: No. Norman confirmed to align this planning PR to the Linear issue scope; implementation details such as config field naming, originator, and scope defaults must not block the Todo planning gate.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Generate PKCE and State Primitives (Priority: P1)

As a TianjiLLM backend developer, I want a reusable OpenAI OAuth helper to generate RFC 7636 PKCE verifier/challenge pairs and random state values, so every OpenAI authorization flow starts with strong replay/CSRF protection.

**Why this priority**: Every later OpenAI OAuth flow depends on correct PKCE and state values. If this layer is wrong, the callback/token exchange slices become insecure or impossible to test.

**Independent Test**: Unit tests generate PKCE/state values without networking and assert format, entropy-sensitive uniqueness, and S256 challenge correctness.

**Acceptance Scenarios**:

1. **Given** the backend starts an OpenAI OAuth login, **When** it requests PKCE primitives, **Then** it receives a verifier within RFC 7636 length bounds and a S256 challenge equal to base64url-no-padding(SHA256(verifier)).
2. **Given** the backend starts two OAuth login attempts, **When** it generates state values, **Then** the states are non-empty, URL-safe, and different.
3. **Given** a test fixture supplies a deterministic verifier, **When** the challenge is derived, **Then** the output matches the known RFC-compatible S256 challenge.

---

### User Story 2 - Configure OpenAI OAuth Provider Metadata (Priority: P1)

As a TianjiLLM operator, I want OpenAI OAuth endpoints/client/scopes to have safe defaults and test overrides, so deployments can use the public OpenAI OAuth provider while CI can run against local mocks.

**Why this priority**: OAuth code must not bake untestable live OpenAI URLs into request builders. Provider metadata is the contract shared by authorize URL and token exchange code.

**Independent Test**: Config unit tests load default config and override config, then assert issuer, authorize endpoint, token endpoint, client ID, scopes, and originator values.

**Acceptance Scenarios**:

1. **Given** no OpenAI OAuth config overrides, **When** provider config is loaded, **Then** it uses `https://auth.openai.com/oauth/authorize`, `https://auth.openai.com/oauth/token`, public client ID default, and scopes `openid profile email offline_access api.connectors.read api.connectors.invoke`.
2. **Given** tests provide local OAuth endpoints, **When** provider config is loaded, **Then** authorize/token URLs use the test endpoints and no live OpenAI network calls are required.
3. **Given** a deployment provides a client ID override, **When** provider config is loaded, **Then** that client ID replaces the default without changing PKCE/token-exchange behavior.

---

### User Story 3 - Build Public-Client Authorize and Token Requests (Priority: P1)

As a TianjiLLM backend developer, I want OpenAI OAuth request builders to create the authorize URL and token exchange request with PKCE/public-client fields, so later callback logic can exchange codes safely without duplicating OAuth request logic.

**Why this priority**: This is the direct integration contract with OpenAI OAuth. It must be testable, deterministic where needed, and must never accidentally send a client secret.

**Independent Test**: Unit tests build an authorize URL and token request against a mock endpoint and assert exact query/form fields.

**Acceptance Scenarios**:

1. **Given** OpenAI OAuth config, redirect URI, PKCE challenge, and state, **When** the authorize URL is built, **Then** it includes `response_type=code`, `client_id`, `redirect_uri`, `scope`, `code_challenge`, `code_challenge_method=S256`, `state`, `id_token_add_organizations=true`, `codex_cli_simplified_flow=true`, and `originator`.
2. **Given** a callback code and PKCE verifier, **When** the token exchange request is built/sent, **Then** the form includes `grant_type=authorization_code`, `code`, `redirect_uri`, `client_id`, and `code_verifier`.
3. **Given** the token exchange request is inspected, **Then** it contains no `client_secret` field and no Authorization Basic credential.
4. **Given** the token endpoint returns JSON tokens, **When** exchange succeeds, **Then** the token response parser preserves access token, refresh token, ID token, token type, expiry, and any organization/account metadata needed by later credential-storage issues.

---

### User Story 4 - Validate Redirect URI Safety (Priority: P1)

As a TianjiLLM operator, I want redirect URI derivation and validation to reject unsafe production URLs, so OpenAI OAuth cannot be configured with an insecure callback in production.

**Why this priority**: Redirect URI mistakes are security-sensitive and hard to diagnose once users start authenticating.

**Independent Test**: Unit tests call redirect URI derivation/validation with production HTTPS, production HTTP, localhost HTTP, and malformed base URLs.

**Acceptance Scenarios**:

1. **Given** public base URL `https://tianji.example.com`, **When** redirect URI is derived, **Then** it becomes `https://tianji.example.com/oauth/openai/callback`.
2. **Given** production mode and `http://tianji.example.com`, **When** redirect URI is validated, **Then** validation fails because production requires HTTPS.
3. **Given** dev/test mode and `http://localhost:<port>`, **When** redirect URI is validated, **Then** validation succeeds.
4. **Given** an empty or malformed public base URL, **When** redirect URI is derived, **Then** the function returns a clear configuration error.

## Edge Cases

- Existing API-key OpenAI provider behavior remains unchanged when OpenAI subscription OAuth config is unused.
- OpenAI OAuth endpoint overrides may include paths; request builders must not double-append `/oauth/authorize` or `/oauth/token` when full endpoint URLs are provided.
- Redirect URI derivation must trim trailing slashes from public base URL exactly once.
- Token exchange error responses must preserve status code/body enough for callback code to report a useful failure, while not logging secrets.
- PKCE/state generation must use cryptographically secure randomness, not math/rand or timestamp-derived values.
- Tests must not depend on OpenAI network availability or a real OpenAI account.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose OpenAI OAuth PKCE generation that produces RFC 7636-compatible verifier/challenge pairs using S256.
- **FR-002**: System MUST expose secure random OAuth state generation suitable for CSRF protection.
- **FR-003**: System MUST define OpenAI OAuth provider config defaults for issuer, authorize endpoint, token endpoint, public client ID, scopes, and OpenAI/Codex flow flags.
- **FR-004**: System MUST allow tests/deployments to override OpenAI OAuth authorize and token endpoints without changing production defaults.
- **FR-005**: System MUST allow deployments to override the OpenAI OAuth public `client_id`.
- **FR-006**: System MUST derive redirect URI from a configured Tianji public base URL plus `/oauth/openai/callback`.
- **FR-007**: System MUST reject non-HTTPS redirect URI outside dev/test localhost mode.
- **FR-008**: System MUST build OpenAI authorize URLs with PKCE S256, state, client ID, redirect URI, scopes, `id_token_add_organizations=true`, `codex_cli_simplified_flow=true`, and `originator`.
- **FR-009**: System MUST build/send OpenAI token exchange requests as public-client authorization-code + PKCE form requests using `client_id` and `code_verifier`.
- **FR-010**: System MUST NOT support or send `client_secret` for OpenAI OAuth in this feature.
- **FR-011**: System MUST parse successful token responses into a typed token bundle for later credential-storage issues.
- **FR-012**: System MUST keep existing `api_key`/`OPENAI_API_KEY` OpenAI provider paths unmodified.

### Key Entities

- **OpenAI OAuth Provider Config**: Defaults/overrides for issuer, authorize endpoint, token endpoint, public client ID, scopes, originator, and boolean authorize-flow flags.
- **PKCE Pair**: Code verifier and S256 code challenge generated per authorization attempt.
- **OAuth State**: URL-safe random state value generated per authorization attempt and validated by later callback code.
- **Redirect URI**: Tianji public base URL plus `/oauth/openai/callback`, with production HTTPS validation.
- **Token Bundle**: Parsed OpenAI OAuth response containing access token, refresh token, ID token, token type, expiry, scopes, and raw/metadata fields needed by later persistence.

## Success Criteria *(mandatory)*

- **SC-001**: PKCE unit tests verify challenge math and verifier/state format/uniqueness without network calls.
- **SC-002**: Config tests verify production defaults and local `httptest` endpoint overrides.
- **SC-003**: Token exchange tests prove request body includes `client_id` + `code_verifier` and excludes `client_secret`.
- **SC-004**: Redirect URI tests reject production HTTP and allow localhost dev/test.
- **SC-005**: `go test` for the new OpenAI OAuth primitive/config package passes entirely offline.

## Assumptions

- TianjiLLM already depends on `golang.org/x/oauth2`, whose documented PKCE helpers include `GenerateVerifier`, `S256ChallengeOption`, and `VerifierOption`; implementation may use these helpers or equivalent internally as long as tests prove RFC 7636 behavior.
- OpenAI/Codex public OAuth examples use issuer `https://auth.openai.com`, localhost callback port 1455 for CLI flows, PKCE S256, public client ID, and no client secret. Tianji will use its own public base URL callback rather than a localhost CLI callback.
- Later HO-1166 split issues own callback handling, credential encryption/persistence, token refresh, model config validation, and routing/failover.
