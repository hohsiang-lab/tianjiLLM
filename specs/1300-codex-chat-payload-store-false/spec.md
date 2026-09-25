# Feature Specification: Codex backend chat payload must set `store:false`

**Branch**: `HO-1300-codex-chat-store-false`
**Linear**: HO-1300
**Created**: 2026-05-11
**Phase**: Todo planning only; production/test implementation starts only after Linear moves to `In Progress`.

> Informed by memory: `memory/2026-05-11.md` records the live Tianji repro where `chatgpt_codex_backend` first required instructions, then failed with `Store must be set to false`; client-provided `store:false` was captured as `ExtraParams` and rejected as `unknown extra parameters`.

## Summary

TianjiLLM's ChatGPT Codex backend route maps `/v1/chat/completions` into a Responses-style payload for `chatgpt.com/backend-api/codex/responses`. Production now reaches that backend, but the request body omits required `store:false`, so `openai/gpt-5.5` fails before model output. HO-1300 fixes the payload contract and request validation around `store` without changing normal OpenAI API-key chat-completions behavior.

## Scope

In scope:

- Codex backend payload builder always emits JSON field `"store": false`.
- Codex route accepts client-provided `store:false` without treating it as an unsupported unknown parameter.
- Codex route rejects client-provided `store:true` locally with a clear validation error before any upstream call.
- Regression tests cover payload builder, handler route, wildcard `openai/*`, and normal OpenAI API-key no-regression behavior.
- Error messages and logs remain secret-safe.

Out of scope:

- Changing credential OAuth connect, refresh, disable, delete, or credential resolution behavior.
- Changing Models UI transport selector behavior from HO-1291.
- Enabling Codex backend streaming.
- Calling real `chatgpt.com`, `api.openai.com`, or production Tianji in automated tests.
- Fixing stale model rows that reference deleted credential IDs; that is a separate model/credential lifecycle issue.

## User Stories and Tests

### User Story 1 - Codex backend sends stateless payload (P1)

As a Tianji operator, I need subscription-backed Codex chat requests to include `store:false`, so ChatGPT Codex backend accepts the request as stateless and returns a model response.

**Independent Test**: Build a Codex payload for a normal chat request and assert marshaled JSON contains `"store":false` while still mapping `model`, `instructions`, and `input`.

**Acceptance Scenarios**:

1. Given a chat request uses `chatgpt_codex_backend`, when Tianji builds the upstream payload, then the body includes `store:false`.
2. Given the mocked Codex backend receives the request, then it no longer returns `Store must be set to false`.

### User Story 2 - Client `store:false` does not break Codex route (P1)

As an OpenAI-compatible client, I may send `store:false`; Tianji should accept that no-op for the Codex route instead of rejecting it as an unknown `ExtraParams` field.

**Independent Test**: Send `/v1/chat/completions` through the mocked Codex route with top-level `store:false`; assert backend is called and the outgoing payload still has `store:false`.

**Acceptance Scenarios**:

1. Given client JSON includes `store:false`, when the route is `chatgpt_codex_backend`, then Tianji accepts the request.
2. Given other unknown fields remain present, then Codex validation still rejects them.

### User Story 3 - Client `store:true` is rejected locally (P1)

As an operator, I need unsafe persistence semantics to fail before upstream, because Codex backend requires `store:false`.

**Independent Test**: Send `store:true` through the Codex route and assert HTTP 400, provider `chatgpt_codex_backend`, clear message mentioning `store`, and zero mocked backend calls.

**Acceptance Scenarios**:

1. Given client JSON includes `store:true`, when the selected route is Codex backend, then Tianji returns a local validation error.
2. Given the error is returned, then no subscription bearer request is sent upstream.

### User Story 4 - OpenAI API-key chat behavior remains unchanged (P1)

As an existing OpenAI-compatible API-key user, I need normal OpenAI provider behavior to remain untouched by this Codex-specific fix.

**Independent Test**: Existing and new OpenAI provider transform tests prove `ExtraParams` pass-through still works for normal API-key `/chat/completions`, including a `store:false` extra parameter if the client sends one.

**Acceptance Scenarios**:

1. Given an API-key-backed OpenAI route receives unknown parameters, then existing pass-through behavior remains unchanged.
2. Given an API-key-backed `openai/*` wildcard exists, then it still calls `/v1/chat/completions`, not `/backend-api/codex/responses`.

## Functional Requirements

- **FR-001**: `chatgptcodex.Payload` MUST include a `store` JSON field.
- **FR-002**: `BuildPayload` MUST set `store` to `false` for every Codex backend chat request.
- **FR-003**: Codex backend request JSON MUST include `"store":false`, not omit the field.
- **FR-004**: Client-provided `store:false` MUST be allowed for Codex backend routes.
- **FR-005**: Client-provided `store:true` MUST be rejected locally before upstream.
- **FR-006**: Rejection for `store:true` MUST be a clear `invalid_request_error` tied to `chatgpt_codex_backend`.
- **FR-007**: Unknown non-`store` `ExtraParams` MUST still be rejected for Codex backend routes.
- **FR-008**: Normal OpenAI API-key provider `ExtraParams` pass-through MUST remain unchanged.
- **FR-009**: Subscription bearer tokens, refresh tokens, API keys, encrypted credential values, and raw credential JSON MUST NOT be printed in tests, logs, docs, or PR text.
- **FR-010**: Tests MUST use local mocks only.
- **FR-011**: Implementation MUST start with RED tests after Linear moves to `In Progress`.
- **FR-012**: Wildcard `openai/*` Codex route MUST be covered because production uses that route.

## Edge Cases

- `store` omitted by client: Tianji still sends `store:false`.
- `store:false` in `ExtraParams`: accepted only by Codex backend payload validation.
- `store:true` in `ExtraParams`: rejected before backend call.
- `store` present with non-boolean value: rejected locally as invalid `store`.
- `store:false` plus another unknown field: unknown field remains rejected.
- API-key OpenAI route with `store:false`: existing pass-through behavior remains the expected behavior.

## Success Criteria

- **SC-001**: Targeted Codex payload test fails before the fix because `"store":false` is omitted.
- **SC-002**: Handler test with client `store:false` succeeds through mocked Codex backend.
- **SC-003**: Handler test with client `store:true` returns local HTTP 400 and zero backend calls.
- **SC-004**: API-key OpenAI tests prove normal provider pass-through remains unchanged.
- **SC-005**: `go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai -count=1` passes after implementation.

## Alternatives Considered

- Add `Store *bool` as a first-class `ChatCompletionRequest` field: rejected for first implementation because it would remove `store` from `ExtraParams` globally and could change normal OpenAI API-key pass-through behavior.
- Strip all unknown parameters before Codex payload validation: rejected because current strict rejection is a safety boundary for unsupported Codex backend fields.
- Require clients to omit `store`: rejected because OpenAI-compatible clients may send `store:false`, and production already proved the backend itself needs the field.

## State Gate

Todo planning can create SpecKit artifacts and a docs-only draft PR. Production/test implementation remains blocked until Linear HO-1300 moves to `In Progress`.
