# Implementation Plan: HO-1319 Codex CLI `/v1/responses` gateway

## Goal

Make Codex CLI work through Tianji's OpenAI-compatible `/v1/responses` gateway for `openai/gpt-5.5`, without relying on repeated websocket failure followed by HTTP fallback.

## Constraints

- Todo phase is docs-only. No production code in this PR.
- Fallback-based solution requires explicit owner confirmation before acceptance or shipping.
- Existing `/v1/chat/completions` Codex streaming route must not regress.
- Token material must stay redacted in tests, docs, logs, and PR content.

## Current Architecture

- Router: `internal/proxy/server.go`
  - Registers `POST /responses`, but not `GET /responses`.
  - Existing websocket relay is separate Realtime code.
- Responses handler: `internal/proxy/handler/responses.go`
  - `CreateResponse` delegates to `openAIEndpointProxy`.
- OpenAI endpoint route: `internal/proxy/handler/assistants.go`
  - Model route with subscription credentials uses `openAISubscriptionTransportDirectHTTP`.
  - Does not use the model's `openai_subscription_transport` for `/v1/responses`.
- Subscription resolver: `internal/proxy/handler/openai_subscription_resolution.go`
  - Supports `direct_openai_http`, `codex_app_server`, and `chatgpt_codex_backend`.
- Existing E2E: `test/e2e/codex_subscription_route_test.go`
  - Covers DB-managed wildcard for chat-completions only.

## Proposed Design

### 1. Add RED tests for Codex CLI transport shape

Add handler or E2E coverage that sends a Codex-like `GET /v1/responses` websocket upgrade request and then sends the first Responses WebSocket request frame.

Required handshake shape:

- `Connection: Upgrade`
- `Upgrade: websocket`
- `Authorization: Bearer <tianji-api-key>`
- `OpenAI-Beta: responses_websockets=2026-02-06`
- `x-client-request-id: <thread-or-request-id>`
- `session_id: <codex-session-id>`
- `thread_id: <codex-thread-id>`

Required first frame shape:

```json
{
  "type": "response.create",
  "model": "openai/gpt-5.5",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "say OK" }]
    }
  ]
}
```

Expected RED today:

- Router returns `405 Method Not Allowed` because only `POST /responses` is registered.

Expected after fix:

- WebSocket handshake succeeds with the Codex headers.
- The first `response.create` frame is parsed as Responses WebSocket protocol, not Realtime protocol.
- WebSocket handling does not require the client frame to contain HTTP `stream`; Tianji bridges the frame to backend SSE internally.
- The same WebSocket connection remains usable for the next sequential `response.create`.
- The frame routes `model=openai/gpt-5.5` through DB-managed `openai/*` and returns Responses-compatible streaming events such as `response.created`, output delta/message events, and `response.completed`.
- A route-only implementation that only makes `GET /responses` return non-405 is still failing.

### 2. Add RED tests for DB-managed `/v1/responses`

Add issue-owned coverage for:

- DB-managed `openai/*` wildcard.
- Request model `openai/gpt-5.5`.
- `/v1/responses` body similar to Codex CLI: `{"model":"openai/gpt-5.5","input":"say OK"}`.
- Intended subscription credential route.
- Healthy and stale-refreshable credential cases.

Expected RED today:

- Route uses `direct_openai_http` and does not prove Codex-compatible backend behavior for `openai/gpt-5.5`.

### 3. Implement a primary transport path

Preferred direction:

- Implement explicit `/v1/responses` websocket/upgrade support for Codex CLI's primary transport path.
- Treat the websocket protocol as Responses WebSocket: authenticate during handshake, preserve/inspect Codex headers, then resolve model from the first `response.create` frame.
- Reuse credential resolution primitives but choose the transport intended by model config when the model row is DB-managed `openai/*` and configured for Codex subscription backend.
- Do not reuse the Realtime `/v1/realtime` relay as proof unless tests demonstrate it handles Responses WebSocket `response.create` frames and event ordering correctly.

Rejected default direction:

- Do not merely let `GET /v1/responses` fail and depend on HTTP `POST /v1/responses` fallback success.
- Do not merely register `GET /responses`, return `426`, or return any non-405 response and call the primary transport fixed.

### 4. Align `/v1/responses` subscription transport selection

Change `/v1/responses` model route resolution so it does not force `direct_openai_http` for every subscription credential route when the model config explicitly selects Codex-compatible subscription transport.

Guardrails:

- Preserve generic official OpenAI Responses API behavior for API-key-backed or direct OpenAI subscription routes.
- Keep transport decisions model-scoped, not global.
- Add no-regression tests for generic `gpt-4o` `/v1/responses` subscription routing.

### 5. Credential health/refresh behavior

Add tests for:

- Healthy credential succeeds.
- Expired refreshable credential refreshes then succeeds.
- Disabled/unrefreshable credential returns redacted actionable failure.

The implementation must make live `openai_subscription_reauthorization_required` or disabled-credential states diagnosable at the intended boundary.

## Test Plan

- `go test ./internal/proxy/handler -run 'Responses|OpenAISubscription|Codex' -count=1`
- `GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e`
- WebSocket RED must assert Codex handshake headers, `response.create` behavior without client-provided `stream`, sequential frame handling, and backend SSE bridging, not just route registration.
- Existing chat-completions Codex E2E must stay green.
- `git diff --check`

## Plan Review

- Official OpenAI docs checked: Codex config reference documents `base_url`, `supports_websockets`, and `wire_api = responses` for custom providers, and reserves built-in provider IDs.
- Official Responses WebSocket docs checked: `/v1/responses` WebSocket mode starts each turn with a client-sent `response.create` payload, does not use HTTP transport-specific fields such as `stream`, and allows multiple sequential `response.create` messages on one connection.
- `openai/codex` source checked: Codex keeps a turn-scoped Responses WebSocket connection, performs best-effort prewarm with `response.create`, and expects the next request to reuse the same connection/`previous_response_id`.
- GitHub prior-art search checked: current Codex CLI issue reports custom `openai_base_url` fallback/authorization behavior, which reinforces owner direction not to treat fallback as the default completion path.
- Repo reality checked first and controls this plan.

## Rollout Notes

- After implementation, live verification must test both `GET /v1/responses` websocket/upgrade shape and actual Codex CLI prompt completion.
- If any fallback behavior remains part of the proposed fix, stop before acceptance and get explicit owner confirmation.
