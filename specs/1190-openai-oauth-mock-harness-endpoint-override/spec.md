# Feature Specification: OpenAI OAuth Mock Harness and Endpoint Override Coverage

**Feature Branch**: `HO-1190-openai-oauth-mock-harness-and-endpoint-override`
**Created**: 2026-05-06
**Status**: Draft
**Input**: Linear HO-1190 — `[QA] OpenAI OAuth mock harness and endpoint override coverage`

## Scope Boundary

This issue provides reusable offline test infrastructure for the OpenAI subscription OAuth flow. It is QA/test-infrastructure scope: implementation may add test helper packages and test-only coverage, but it must not add production OAuth behavior beyond endpoint override plumbing required to make tests deterministic.

**In scope**:

- A reusable Go `httptest.NewServer`-based mock harness for OpenAI OAuth authorize/token/refresh endpoints and OpenAI upstream API endpoints.
- Endpoint override coverage proving OAuth authorize/token/upstream base URLs can point at mocks in tests.
- Token exchange and refresh mock helpers reusable by backend tests and UI E2E setup.
- Mock upstream coverage for `/v1/models` plus representative OpenAI-compatible endpoints already supported by TianjiLLM.
- A CI guard test proving the OpenAI OAuth/subscription test path does not call real OpenAI hosts.

**Out of scope**:

- Real OpenAI network calls, real OpenAI accounts, OpenAI scraping, browser credential import, or shared-machine credentials.
- WireMock or external mock services in v1.
- Production callback UI, credential persistence, refresh/failover implementation, or model routing behavior owned by other HO-1166/HO-1173 children.
- Contract generation or new OpenAPI generator work.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reusable OAuth Mock Server (Priority: P1)

As a backend test author, I want a reusable local OpenAI OAuth mock server, so token exchange and refresh tests can run in CI without OpenAI network access.

**Independent Test**: A Go unit test starts the mock harness, points OpenAI OAuth config at its authorize/token URLs, exchanges a code, refreshes a token, and asserts the mock recorded the expected requests.

**Acceptance Scenarios**:

1. **Given** a test starts the OpenAI OAuth mock harness, **When** it asks for authorize/token/refresh endpoint URLs, **Then** all URLs use the local `httptest.Server` base URL.
2. **Given** a callback-code fixture is registered, **When** production token exchange code posts to the mock token endpoint, **Then** the mock returns configured access/refresh/id token JSON and records `grant_type=authorization_code`, `client_id`, `code`, `redirect_uri`, and `code_verifier`.
3. **Given** a refresh-token fixture is registered, **When** refresh code posts to the mock token endpoint, **Then** the mock returns configured rotated token JSON and records `grant_type=refresh_token`.
4. **Given** a token request sends `client_secret` or Basic auth, **When** the mock receives the request, **Then** the test fails with a clear assertion because OpenAI OAuth is public-client PKCE.

---

### User Story 2 - Reusable OpenAI Upstream Mock Server (Priority: P1)

As a provider/integration test author, I want a reusable local OpenAI upstream mock, so `/v1/models` and representative provider calls can be tested without hitting `api.openai.com`.

**Independent Test**: A Go unit/integration test points OpenAI provider base URL at the mock upstream and verifies `/v1/models` plus representative endpoints return deterministic fixture responses.

**Acceptance Scenarios**:

1. **Given** the upstream mock has model fixtures, **When** code calls `/v1/models`, **Then** it receives a deterministic OpenAI-compatible models list.
2. **Given** the upstream mock has a chat completion fixture, **When** code calls `/v1/chat/completions`, **Then** it receives a deterministic OpenAI-compatible response and the mock records method, path, Authorization header, and body.
3. **Given** the upstream mock has an embeddings or images fixture matching existing TianjiLLM OpenAI provider coverage, **When** code calls the endpoint, **Then** it receives deterministic OpenAI-compatible JSON without real network.
4. **Given** a test calls an unregistered OpenAI endpoint, **When** the mock receives the request, **Then** it returns a clear test failure response and records the unexpected path.

---

### User Story 3 - Endpoint Override Coverage (Priority: P1)

As a QA owner, I want endpoint override tests for OAuth and upstream base URLs, so CI proves OpenAI OAuth tests are mockable and not tied to live endpoints.

**Independent Test**: Config/provider tests set authorize/token/upstream overrides to the harness URLs and assert the generated requests target only those URLs.

**Acceptance Scenarios**:

1. **Given** OpenAI OAuth config override values point at the harness, **When** authorize URL and token exchange requests are built, **Then** they use the harness base URL exactly.
2. **Given** OpenAI upstream provider base URL points at the harness, **When** representative provider requests are built/sent, **Then** request URLs use the harness base URL exactly.
3. **Given** override URLs include custom paths, **When** config resolves them, **Then** code does not append duplicate `/oauth/authorize`, `/oauth/token`, or `/v1` path segments.

---

### User Story 4 - CI No-Real-OpenAI Guard (Priority: P1)

As a maintainer, I want a guard test that fails on real OpenAI network attempts, so CI cannot accidentally call OpenAI during OAuth/subscription tests.

**Independent Test**: A guard HTTP transport rejects requests to `auth.openai.com` and `api.openai.com`; the OpenAI OAuth/upstream test path must pass only when every request goes to the mock harness.

**Acceptance Scenarios**:

1. **Given** the guard transport is installed, **When** OAuth token exchange runs with mock endpoint overrides, **Then** the test passes and the guard observes no `auth.openai.com` calls.
2. **Given** the guard transport is installed, **When** upstream provider calls run with mock base URL override, **Then** the test passes and the guard observes no `api.openai.com` calls.
3. **Given** a test accidentally uses a production OpenAI URL, **When** the guarded client sees it, **Then** the test fails with a message naming the forbidden host.

## Edge Cases

- Mock endpoints may include custom path prefixes; helpers must compose URLs without duplicate segments.
- Token JSON fixtures must support access token, refresh token, id token, token type, expiry, scope, organization/account metadata, and refresh rotation fields.
- The harness must be safe for parallel tests by keeping recorded requests per server instance and avoiding global mutable state.
- The no-real-network guard must block both `auth.openai.com` and `api.openai.com`, including HTTPS default endpoints.
- Existing OpenAI API-key tests must continue to support custom `NewWithBaseURL` mocks.
- UI E2E setup must be able to consume mock endpoint URLs via config/env without importing browser-specific code into production packages.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a reusable Go test helper that starts a local OpenAI OAuth mock server with authorize, token-exchange, and refresh-token endpoints.
- **FR-002**: The OAuth mock helper MUST expose resolved authorize/token endpoint URLs suitable for `config.OpenAIOAuthConfig` overrides.
- **FR-003**: The OAuth mock helper MUST record token endpoint requests and expose assertions for public-client PKCE fields.
- **FR-004**: The OAuth mock helper MUST support configurable token exchange and refresh-token responses, including rotated refresh tokens.
- **FR-005**: System MUST provide a reusable OpenAI upstream mock server for `/v1/models` and representative OpenAI-compatible endpoints used by existing provider tests.
- **FR-006**: The upstream mock helper MUST expose a base URL suitable for OpenAI provider `NewWithBaseURL`/future subscription upstream overrides.
- **FR-007**: Tests MUST prove OAuth authorize/token endpoint overrides are honored exactly and do not fall back to `https://auth.openai.com`.
- **FR-008**: Tests MUST prove upstream base URL overrides are honored exactly and do not fall back to `https://api.openai.com`.
- **FR-009**: System MUST include a no-real-OpenAI guard test that fails any request to `auth.openai.com` or `api.openai.com` during the covered OAuth/upstream paths.
- **FR-010**: Implementation MUST NOT introduce WireMock, Dockerized mock services, or external mock infrastructure for v1.
- **FR-011**: Implementation MUST NOT require OpenAI credentials or live OpenAI network access in CI.
- **FR-012**: Existing OpenAI API-key provider behavior MUST remain unchanged except for tests using explicit mock base URLs.

### Key Entities

- **OpenAI OAuth Mock Harness**: Test helper wrapping `httptest.Server`, fixture configuration, endpoint URL accessors, and recorded token requests.
- **Token Fixture**: Configurable authorization-code or refresh-token response payload returned by the OAuth mock.
- **OpenAI Upstream Mock Harness**: Test helper wrapping OpenAI-compatible upstream endpoints such as `/v1/models`, `/v1/chat/completions`, `/v1/embeddings`, and representative existing provider endpoints.
- **No-Real-OpenAI Guard Transport**: Test-only HTTP transport/client wrapper that rejects forbidden OpenAI hosts.
- **Recorded Request**: Method/path/header/body/form snapshot exposed for assertions without leaking token secrets in logs.

## Success Criteria *(mandatory)*

- **SC-001**: `go test` can run the OpenAI OAuth mock harness tests offline and pass without OpenAI credentials.
- **SC-002**: Token exchange/refresh tests assert request shape and response parsing through `httptest.NewServer` mocks.
- **SC-003**: Upstream mock tests cover `/v1/models` and at least one representative generation/embedding endpoint already supported by TianjiLLM.
- **SC-004**: Guard tests fail when a covered path targets `auth.openai.com` or `api.openai.com` and pass when overrides target local mocks.
- **SC-005**: CI evidence shows no test in this scope requires live OpenAI network access.

## Assumptions

- HO-1174 has already added initial OpenAI OAuth config and PKCE/token exchange primitives, including test endpoint override fields.
- Future child issues may add refresh/failover/UI behavior; this harness should be reusable by those tests without deciding their production implementation.
- A Go helper package under `internal/testutil/openaitest` is acceptable because backend and `test/e2e` packages live inside the same module and can import internal packages.
