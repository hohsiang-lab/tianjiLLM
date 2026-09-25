# Feature Specification: ChatGPT Codex backend transport for subscription credentials

**Branch**: `HO-1288-chatgpt-codex-backend-transport`
**Linear**: HO-1288
**Created**: 2026-05-10
**Status**: Todo planning only; no production code in this phase

## Summary

TianjiLLM already stores OpenAI subscription OAuth credentials and can resolve them for direct OpenAI HTTP routes. That direct path sends the subscription access token to `https://api.openai.com/v1/*`, which is not the ChatGPT subscription Codex backend contract needed here. HO-1288 adds a dedicated transport/provider path for subscription credentials that calls `https://chatgpt.com/backend-api/codex/responses` with ChatGPT account headers, while leaving normal OpenAI API-key provider behavior unchanged.

## Scope

In scope:

- Add a dedicated ChatGPT Codex backend transport for models configured to use OpenAI subscription credentials.
- Resolve stored `openai_subscription` credential access token and `account_id` through the existing credential resolver/refresh boundary.
- Send `Authorization: Bearer <access_token>`.
- Send `ChatGPT-Account-Id` when the stored credential has an account ID.
- Send `originator: codex_cli_rs` by default unless explicit config overrides it.
- Call `https://chatgpt.com/backend-api/codex/responses` by default.
- Transform Tianji OpenAI-compatible chat/responses input into the Codex backend Responses-style payload needed by this transport.
- Parse non-streaming and streaming Codex backend responses back into existing Tianji/OpenAI-compatible response surfaces.
- Preserve direct OpenAI API-key and existing direct subscription `/v1` behavior unless a route explicitly selects the ChatGPT Codex backend transport.

Out of scope:

- Changing OpenAI API-key provider behavior.
- Moving all OpenAI subscription routes away from `https://api.openai.com/v1`.
- Reworking OAuth login/connect/refresh persistence.
- Adding UI for credential management.
- Real ChatGPT/OpenAI network calls in tests.
- Codex app-server `account/login/start` integration; this issue is the HTTP backend transport path, not app-server auth.

## User Stories and Tests

### User Story 1 - Subscription-backed Codex model calls ChatGPT backend (P1)

As a Tianji operator, I want a model backed by stored OpenAI subscription credentials to call the ChatGPT Codex backend endpoint, so ChatGPT subscription tokens are not incorrectly sent to the Platform `/v1/chat/completions` endpoint.

**Independent Test**: Unit test seeds an active `openai_subscription` credential, configures a model for the Codex backend transport, sends a chat or responses request, and asserts the mocked upstream receives exactly `/backend-api/codex/responses`.

**Acceptance Scenarios**:

1. Given an active subscription credential with access token and account ID, when the Codex backend transport is selected, then the upstream request goes to `/backend-api/codex/responses`.
2. Given the same request, then no request is made to `/v1/chat/completions`.
3. Given the upstream responds successfully, then Tianji returns an OpenAI-compatible response to the caller.

### User Story 2 - Codex backend request carries required auth headers (P1)

As the ChatGPT backend transport, I need the subscription access token and account identity sent in the expected headers, so the backend can authorize the request against the selected ChatGPT account.

**Independent Test**: Unit test captures headers from a mocked ChatGPT backend and asserts `Authorization`, `ChatGPT-Account-Id`, `originator`, `OpenAI-Beta`, `Accept`, and `Content-Type` values.

**Acceptance Scenarios**:

1. Given a resolved subscription credential, when sending a backend request, then `Authorization` is `Bearer <access_token>`.
2. Given `account_id` is available, when sending a backend request, then `ChatGPT-Account-Id` is present.
3. Given no config override, when sending a backend request, then `originator` is `codex_cli_rs`.
4. Given a config override, when sending a backend request, then `originator` uses the configured value.

### User Story 3 - API-key OpenAI provider remains unchanged (P1)

As an existing Tianji API-key user, I need normal OpenAI provider traffic to keep using the Platform API path and headers, so the new subscription backend transport does not regress established deployments.

**Independent Test**: Existing and new regression tests send API-key-backed OpenAI requests and assert they still use `https://api.openai.com/v1/chat/completions` or configured `api_base`.

**Acceptance Scenarios**:

1. Given a model with only `api_key`, when `/v1/chat/completions` is called, then the OpenAI provider still uses `/v1/chat/completions`.
2. Given a model with custom `api_base`, when `/v1/chat/completions` is called, then the custom OpenAI-compatible behavior remains unchanged.
3. Given no Codex backend transport selection, then existing direct subscription `/v1` route coverage remains compatible.

### User Story 4 - Credential failure is explicit and redacted (P2)

As an operator, I need unusable subscription credentials to fail clearly without falling back to API keys or leaking token material.

**Independent Test**: Unit tests cover missing, disabled, malformed, expired/refresh-failed, and rate-limited credentials and assert safe errors plus no API-key fallback once Codex backend subscription credentials are explicitly configured.

**Acceptance Scenarios**:

1. Given all configured subscription credentials are unusable, when the Codex backend transport is selected, then Tianji returns a safe explicit error.
2. Given an API key is also present, when explicit subscription credentials fail, then Tianji does not silently fallback to the API key.
3. Given an error is returned, then access token, refresh token, credential value, and raw credential JSON are not exposed.

## Functional Requirements

- **FR-001**: System MUST add an explicit ChatGPT Codex backend transport for subscription-backed models.
- **FR-002**: System MUST call `https://chatgpt.com/backend-api/codex/responses` by default for that transport.
- **FR-003**: System MUST allow a test/config override for the Codex backend base URL without affecting normal OpenAI API base URL behavior.
- **FR-004**: System MUST resolve subscription credential access token and account ID through existing OpenAI subscription credential helpers.
- **FR-005**: System MUST send `Authorization: Bearer <access_token>`.
- **FR-006**: System MUST send `ChatGPT-Account-Id` when a credential account ID is available.
- **FR-007**: System MUST send `originator: codex_cli_rs` by default unless config overrides it.
- **FR-008**: System MUST send backend-compatible response headers such as `OpenAI-Beta: responses=experimental`, `Accept: text/event-stream` for streaming, and `Content-Type: application/json`.
- **FR-009**: System MUST map caller model IDs so `openai/...` or `chatgpt/...` prefixes do not leak incorrectly into the backend request model when a normalized backend model is required.
- **FR-010**: System MUST support non-streaming responses.
- **FR-011**: System MUST support streaming/SSE responses or explicitly fail with a tested safe unsupported error if implementation discovers current repo streaming abstractions cannot support it in this slice.
- **FR-012**: System MUST preserve normal API-key-backed OpenAI provider behavior.
- **FR-013**: System MUST preserve custom OpenAI-compatible `api_base` behavior for API-key routes.
- **FR-014**: System MUST NOT send ChatGPT subscription bearer tokens to Platform `/v1/chat/completions` when the Codex backend transport is selected.
- **FR-015**: System MUST NOT silently fallback to API key when explicit Codex backend subscription credentials fail.
- **FR-016**: System MUST redact access tokens, refresh tokens, credential values, and raw credential JSON from logs/errors.
- **FR-017**: Tests MUST use mocked upstream HTTP servers and no real OpenAI/ChatGPT network.

## Key Entities

- **Codex Backend Transport**: A dedicated HTTP transport that targets ChatGPT backend `/backend-api/codex/responses`.
- **OpenAI Subscription Credential**: Existing encrypted credential row with `credential_type = "openai_subscription"`, access token, refresh token, expiry, and account ID.
- **Codex Backend Request Config**: Runtime config values for backend base URL and originator header.
- **Codex Backend Response Adapter**: Mapping layer from backend Responses/SSE payloads into Tianji's existing OpenAI-compatible response shape.

## Success Criteria

- **SC-001**: Unit tests prove Codex backend transport sends requests to `/backend-api/codex/responses`.
- **SC-002**: Unit tests prove Codex backend transport does not call Platform `/v1/chat/completions`.
- **SC-003**: Unit tests prove required headers include subscription bearer token, account ID when present, and default/overridden originator.
- **SC-004**: Regression tests prove API-key OpenAI provider behavior is unchanged.
- **SC-005**: Credential failure tests prove no API-key fallback and no token leakage.
- **SC-006**: Targeted Go tests pass offline with mocked upstreams.

## Dependencies

- Existing HO-1167 `tianji_params.openai_subscription_credential_ids` config and resolver boundary.
- Existing HO-1176 encrypted `openai_subscription` credential persistence.
- Existing HO-1177 refresh helpers and disabled/failure metadata.
- Existing HO-1179 credential selection, sticky/failover, and rate-limit gate behavior.
- Existing HO-1285 runtime DB-managed model source so UI-created `chatgpt/*` or equivalent backend models route through current runtime config.
