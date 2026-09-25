# Feature Specification: Normalize Codex Responses output and errors for OpenAI-compatible callers

**Branch**: `HO-1290-normalize-codex-responses-output-and-errors`
**Linear**: HO-1290
**Created**: 2026-05-10
**Status**: Todo planning only; no production code in this phase

## Summary

HO-1288 scopes the ChatGPT Codex backend transport. HO-1290 owns the caller-facing adapter after that transport receives a backend response: successful Codex Responses-style output must be returned as Tianji's existing OpenAI-compatible non-streaming chat completion shape, and actionable upstream failures must keep useful HTTP status, error type, code, and message instead of being hidden behind a generic `502 transform response`.

## Scope

In scope:

- Normalize successful non-streaming Codex backend Responses output into `model.ModelResponse`.
- Return `object: "chat.completion"` with one assistant message choice for successful Codex text output.
- Map Codex backend usage into existing Tianji `prompt_tokens`, `completion_tokens`, and `total_tokens`.
- Preserve upstream 401, 403, and 429 status codes when backend errors are already actionable.
- Preserve safe upstream `error.message`, `error.type`, and `error.code` for auth, missing-scope, quota, and backend failures.
- Avoid converting actionable upstream failures into generic `502` proxy/transform errors.
- Keep existing Platform OpenAI provider error behavior unchanged for API-key routes.
- Add mocked/offline tests for success mapping, 401/403 missing-scope diagnostics, 429 quota diagnostics, and OpenAI provider regression.

Out of scope:

- Building the initial ChatGPT Codex backend transport URL/header/body path from HO-1288.
- Streaming response normalization.
- OpenAI OAuth login, refresh, credential persistence, or credential UI changes.
- Real OpenAI or ChatGPT network calls.
- Changing normal API-key-backed OpenAI `/v1/chat/completions` behavior.

## User Stories and Tests

### User Story 1 - Successful Codex output looks like chat completions (P1)

As an OpenAI-compatible caller, I want a non-streaming Codex backend success to look like a normal chat completion, so my existing Chat Completions client can read `choices[0].message.content` without knowing about Responses internals.

**Independent Test**: A provider/adapter unit test feeds a mocked Codex Responses JSON body with `output[].content[].type = "output_text"` and asserts the returned `model.ModelResponse` has `object: "chat.completion"`, assistant role, text content, finish reason, model, ID, created timestamp, and mapped usage.

**Acceptance Scenarios**:

1. Given a completed Codex Responses body with one assistant output text item, when the adapter transforms it, then the caller receives one `choices[0].message.role = "assistant"` and matching text content.
2. Given usage has `input_tokens`, `output_tokens`, and `total_tokens`, then Tianji maps them to `prompt_tokens`, `completion_tokens`, and `total_tokens`.
3. Given the response has no text output, then Tianji returns a tested safe error instead of an empty successful chat completion.

### User Story 2 - Auth and missing-scope failures stay actionable (P1)

As an operator debugging subscription credentials, I need 401/403 backend failures to expose safe upstream diagnostics, so I can distinguish expired auth from missing permission scopes.

**Independent Test**: Mocked backend error tests return 401 and 403 JSON error bodies with missing-scope messages/codes and assert the external HTTP status and error payload are not rewritten to 502.

**Acceptance Scenarios**:

1. Given backend returns HTTP 401 with `invalid_token` or auth message, then Tianji returns HTTP 401 and an OpenAI-compatible `{"error": ...}` body.
2. Given backend returns HTTP 403 with missing-scope diagnostics, then Tianji returns HTTP 403 and preserves safe `message`, `type`, and `code`.
3. Given the upstream body contains bearer token-like material, then response/log output redacts it.

### User Story 3 - Quota failures stay quota failures (P1)

As a caller, I need a quota failure to remain a 429 rate-limit/quota error, so retry and billing handling can work correctly.

**Independent Test**: Mocked backend returns HTTP 429 with an `insufficient_quota` style error and tests assert Tianji returns HTTP 429, not 502, with preserved safe code/message.

**Acceptance Scenarios**:

1. Given backend returns HTTP 429 quota diagnostics, then Tianji returns HTTP 429.
2. Given upstream includes rate-limit headers, then implementation preserves relevant response headers if current handler abstractions allow it, or records the limitation in implementation evidence.
3. Given the normal OpenAI Platform provider returns 429 through existing API-key path, then existing behavior remains unchanged unless a separate regression proves the same bug is in scope.

### User Story 4 - Platform OpenAI behavior does not regress (P1)

As an existing Tianji API-key user, I need the normal OpenAI provider surface to remain stable while Codex backend normalization is added.

**Independent Test**: Existing and new OpenAI provider tests assert `internal/provider/openai` still parses normal chat completion success/error bodies as before.

**Acceptance Scenarios**:

1. Given an API-key-backed OpenAI request succeeds, then response shape remains the existing `model.ModelResponse`.
2. Given an API-key-backed OpenAI request errors, then existing provider error tests still pass.
3. Given the new Codex adapter handles private backend response shapes, then it is not registered as the default OpenAI provider behavior.

## Functional Requirements

- **FR-001**: System MUST map completed Codex Responses-style non-streaming output into Tianji `model.ModelResponse`.
- **FR-002**: System MUST return `object: "chat.completion"` for successful OpenAI-compatible caller responses.
- **FR-003**: System MUST aggregate assistant `output_text` content into `choices[0].message.content`.
- **FR-004**: System MUST map Codex `usage.input_tokens` to `prompt_tokens`.
- **FR-005**: System MUST map Codex `usage.output_tokens` to `completion_tokens`.
- **FR-006**: System MUST preserve or compute `total_tokens`.
- **FR-007**: System MUST return a safe transform error when a completed backend response has no assistant text output.
- **FR-008**: System MUST preserve HTTP 401 for upstream auth failures.
- **FR-009**: System MUST preserve HTTP 403 for upstream missing-scope/permission failures.
- **FR-010**: System MUST preserve HTTP 429 for upstream quota/rate-limit failures.
- **FR-011**: System MUST preserve safe upstream error `message`, `type`, and `code` fields when present.
- **FR-012**: System MUST NOT wrap actionable upstream 401/403/429 errors into generic `502` or `internal_error`.
- **FR-013**: System MUST redact access tokens, refresh tokens, bearer headers, and raw credential JSON from returned errors and logs.
- **FR-014**: System MUST keep existing API-key-backed OpenAI provider behavior unchanged.
- **FR-015**: Tests MUST use mocked upstream responses and no real OpenAI/ChatGPT network calls.

## Key Entities

- **Codex Responses Adapter**: Mapping layer from backend Responses JSON to Tianji `model.ModelResponse`.
- **Actionable Upstream Error**: Backend error with HTTP status and safe `error` fields that callers can use directly.
- **OpenAI-Compatible Error Surface**: Tianji `model.ErrorResponse` with `error.message`, `error.type`, optional `error.code`, provider, and model.

## Success Criteria

- **SC-001**: Success mapping test covers non-streaming Codex Responses JSON to chat completion JSON.
- **SC-002**: Error mapping tests cover 401 auth, 403 missing scope, and 429 quota diagnostics.
- **SC-003**: Tests prove actionable 401/403/429 responses are not rewritten to 502.
- **SC-004**: OpenAI Platform provider regression tests remain green.
- **SC-005**: Token leakage sentinel tests pass.
- **SC-006**: Targeted Go tests pass offline with mocked upstream bodies.

## Dependencies

- HO-1288 ChatGPT Codex backend transport planning.
- Existing `model.ModelResponse` and `model.ErrorResponse` contracts.
- Existing `model.TianjiError` status/type/code propagation boundary.
- Existing `handleNonStreamingCompletion` error-writing path in `internal/proxy/handler/chat.go`.
