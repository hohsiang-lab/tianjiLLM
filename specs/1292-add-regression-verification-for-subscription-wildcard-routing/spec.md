# Feature Specification: Subscription wildcard routing regression verification

**Branch**: `HO-1292-add-regression-verification-for-subscription`
**Linear**: HO-1292
**Created**: 2026-05-10
**Phase**: Todo planning only; production/test implementation starts only after Linear moves to `In Progress`.

## Summary

TianjiLLM now has explicit OpenAI subscription transport selection, ChatGPT Codex backend request mapping, response/error normalization, and Models UI persistence. HO-1292 adds the missing regression verification that ties those slices together for wildcard model routing: a subscription-backed `openai/*` wildcard configured with `openai_subscription_transport = "chatgpt_codex_backend"` must route through the ChatGPT Codex backend, while ordinary API-key-backed `openai/*` traffic must keep using the normal OpenAI-compatible `/v1/chat/completions` path.

## Scope

In scope:

- Add integration/contract regression tests for `openai/*` wildcard routing with `openai_subscription_credential_ids` and `openai_subscription_transport = "chatgpt_codex_backend"`.
- Prove the selected subscription wildcard route calls the mocked ChatGPT Codex backend `/backend-api/codex/responses`.
- Prove the same route does not call mocked Platform `/v1/chat/completions` and never sends ChatGPT OAuth bearer material to `api.openai.com`.
- Cover Codex backend 401/403 missing-scope diagnostics and 429 quota/rate-limit diagnostics through the full wildcard route, not only provider adapter unit tests.
- Add an API-key-backed `openai/*` wildcard regression proving normal OpenAI-compatible routes still call `/v1/chat/completions` with API-key auth.
- Add deployment verification notes for OpenClaw `openai/*` rollout without printing tokens or secrets.

Out of scope:

- Changing the ChatGPT Codex backend transport implementation unless the RED tests prove an implementation gap after Linear moves to `In Progress`.
- Changing Models UI selector behavior from HO-1291.
- Changing OpenAI OAuth connect/refresh/credential lifecycle.
- Calling real `api.openai.com`, `chatgpt.com`, or OpenClaw production services in tests.
- Printing access tokens, refresh tokens, API keys, bearer headers, encrypted credential blobs, or raw credential JSON in docs, logs, PR comments, or issue threads.

## User Stories and Tests

### User Story 1 - Subscription wildcard uses ChatGPT Codex backend (P1)

As a Tianji operator, I want a subscription-backed wildcard such as `openai/*` to use the configured ChatGPT Codex backend transport, so OpenClaw can route `openai/gpt-*` through subscription credentials without sending ChatGPT OAuth tokens to the Platform API.

**Independent Test**: Configure a wildcard model with `model_name = "openai/*"`, `model = "openai/*"`, one active `openai_subscription` credential, and `openai_subscription_transport = "chatgpt_codex_backend"`. Send `/v1/chat/completions` with `model = "openai/gpt-5.5"` and assert the mocked Codex backend receives `/backend-api/codex/responses`.

**Acceptance Scenarios**:

1. Given the subscription wildcard route exists, when `openai/gpt-5.5` is requested, then wildcard matching resolves the configured model and selects `ChatGPTCodexBackend`.
2. Given the backend request is sent, then the path is `/backend-api/codex/responses`.
3. Given a Platform mock is present, then it receives zero calls for the subscription Codex wildcard request.

### User Story 2 - ChatGPT OAuth token is not used against Platform `/v1/chat/completions` (P1)

As an operator, I need a missing-scope regression test to fail loudly if a ChatGPT subscription bearer token is routed to `api.openai.com/v1/chat/completions`.

**Independent Test**: Install a guarded HTTP transport or pair of local mocks that fails if a request with subscription token material targets `/v1/chat/completions`; the same test must pass only when the request goes to the Codex backend mock.

**Acceptance Scenarios**:

1. Given the subscription access token is `access-secret`, when the Codex wildcard request runs, then no Platform request carries `Authorization: Bearer access-secret`.
2. Given direct Platform transport would return a missing-scope style diagnostic, then the regression test exposes that wrong route as a failure.
3. Given the test fails, its output must identify the wrong host/path without printing token material.

### User Story 3 - Codex auth/quota diagnostics survive wildcard routing (P1)

As an operator debugging OpenClaw rollout, I need auth, missing-scope, and quota failures from the Codex transport to remain actionable through wildcard resolution.

**Independent Test**: Mock the Codex backend to return 401, 403, and 429 bodies; send requests through the wildcard route and assert returned status, `error.type`, `error.code`, and safe message are preserved.

**Acceptance Scenarios**:

1. Given Codex backend returns 401 auth failure, then Tianji returns HTTP 401 with safe auth diagnostics.
2. Given Codex backend returns 403 missing-scope failure, then Tianji returns HTTP 403 with safe missing-scope diagnostics.
3. Given Codex backend returns 429 quota/rate-limit failure, then Tianji returns HTTP 429 with safe quota diagnostics.
4. Given rate-limit headers are present, then quota state recording is covered or the limitation is documented in implementation evidence.

### User Story 4 - API-key wildcard remains normal OpenAI-compatible routing (P1)

As an existing Tianji user, I need `openai/*` API-key routes to remain unchanged when subscription Codex regressions are added.

**Independent Test**: Configure a second wildcard or isolated test server with `model_name = "openai/*"`, `model = "openai/*"`, only `api_key`, and a custom mocked `api_base`; send `/v1/chat/completions` and assert the mock receives `/v1/chat/completions`, not `/backend-api/codex/responses`.

**Acceptance Scenarios**:

1. Given no subscription credential IDs are configured, then no Codex transport is selected.
2. Given a custom `api_base` is configured for the API-key wildcard, then the request uses that base and `/chat/completions`.
3. Given API-key wildcard succeeds, then response shape remains the existing OpenAI-compatible chat completion shape.

### User Story 5 - Deployment verification is secret-safe (P2)

As an OpenClaw operator, I need rollout checks for `openai/*` that verify routing without exposing credentials.

**Independent Test**: Documentation-only verification checklist describes safe commands/log assertions and explicitly forbids printing bearer values.

**Acceptance Scenarios**:

1. Given OpenClaw is configured to use TianjiLLM for `openai/*`, then verification checks request path/transport markers, response shape, and status code.
2. Given logs are inspected, then commands redact or avoid credential-bearing headers.
3. Given a failure occurs, then the checklist distinguishes wrong Platform route, Codex auth/missing-scope, quota, and normal API-key fallback behavior.

## Functional Requirements

- **FR-001**: Tests MUST cover `openai/*` wildcard resolution with `openai_subscription_credential_ids` and `openai_subscription_transport = "chatgpt_codex_backend"`.
- **FR-002**: Tests MUST assert subscription wildcard traffic calls `/backend-api/codex/responses`.
- **FR-003**: Tests MUST assert subscription wildcard traffic does not call `/v1/chat/completions`.
- **FR-004**: Tests MUST fail if ChatGPT subscription bearer material is sent to a Platform `/v1/chat/completions` route.
- **FR-005**: Tests MUST cover 401 auth diagnostics through the wildcard route.
- **FR-006**: Tests MUST cover 403 missing-scope diagnostics through the wildcard route.
- **FR-007**: Tests MUST cover 429 quota/rate-limit diagnostics through the wildcard route.
- **FR-008**: Tests MUST preserve and assert safe `error.message`, `error.type`, and `error.code` fields where upstream provides them.
- **FR-009**: Tests MUST prove API-key-backed `openai/*` wildcard routing remains on normal OpenAI-compatible `/v1/chat/completions`.
- **FR-010**: Tests MUST use local mocks/guards only; no real OpenAI, ChatGPT, or OpenClaw production calls.
- **FR-011**: Test output and deployment verification notes MUST NOT print access tokens, refresh tokens, API keys, bearer headers, encrypted credential values, or raw credential JSON.
- **FR-012**: Implementation tasks MUST start with RED tests after Linear moves to `In Progress`.
- **FR-013**: Deployment verification notes MUST include OpenClaw `openai/*` rollout checks and failure classification.

## Key Entities

- **Subscription Wildcard Route**: A `config.ModelConfig` or DB-managed runtime model with wildcard `model_name`, wildcard `tianji_params.model`, subscription credential IDs, and `chatgpt_codex_backend` transport.
- **Codex Backend Mock**: Local HTTP server representing `https://chatgpt.com/backend-api/codex/responses`.
- **Platform OpenAI Mock/Guard**: Local HTTP server or guarded transport representing wrong direct Platform `/v1/chat/completions` behavior.
- **Secret Sentinel**: Deliberate fake token/API-key strings used only to assert redaction and wrong-route prevention.
- **Deployment Verification Note**: Secret-safe checklist for validating OpenClaw `openai/*` routing after rollout.

## Success Criteria

- **SC-001**: A RED test fails when subscription wildcard routing uses the wrong direct OpenAI transport.
- **SC-002**: The subscription wildcard test passes only when traffic reaches the mocked Codex backend path.
- **SC-003**: Missing-scope/auth/quota diagnostics are tested through full wildcard routing.
- **SC-004**: API-key wildcard regression remains green and continues using `/v1/chat/completions`.
- **SC-005**: Token/API-key leakage sentinel tests pass.
- **SC-006**: Deployment verification instructions are secret-safe and actionable for OpenClaw `openai/*`.

## Dependencies

- HO-1288 ChatGPT Codex backend transport and handler route.
- HO-1289 chat request to Codex Responses payload mapper.
- HO-1290 Codex response/error normalization.
- HO-1291 explicit `openai_subscription_transport` config/UI selector.
- Existing wildcard matching in `internal/wildcard` and `Handlers.findModelConfig`.
- Existing OpenAI subscription credential resolver and mocked credential harness.
