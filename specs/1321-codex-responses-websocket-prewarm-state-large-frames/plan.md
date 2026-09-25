# Implementation Plan: HO-1321 Codex Responses WebSocket adapter

## Goal

Upgrade Tianji `/v1/responses` WebSocket support from a simple frame-to-backend bridge into a Codex-compatible Responses WebSocket adapter that supports:

- prewarm `response.create` with `generate:false`
- same-connection `previous_response_id` chaining
- realistic Codex frames larger than `github.com/coder/websocket` default read limit
- true Codex CLI primary transport with no reconnect fallback noise

## Constraints

- Todo phase is docs-only. No production code in this PR.
- All behavior changes must be RED-test-first in implementation phase.
- Existing generic OpenAI-compatible `/v1/responses` and Codex chat-completions paths must not regress.
- Secrets and live credential values must stay redacted.

## Repo Reality

- `internal/proxy/handler/responses.go`
  - `handleResponsesWebSocket` calls `websocket.Accept` and immediately reads frames in a loop.
  - It does not call `conn.SetReadLimit`.
  - `handleResponsesWebSocketFrame` routes each decoded `response.create` independently to `transport.BuildResponsesRequest`.
  - It does not keep connection-local state or recognize `generate:false`.
- `internal/provider/chatgptcodex/transport.go`
  - `normalizeResponsesPayload` copies all raw fields into the backend payload.
  - It normalizes `model`, defaults `store:false`, and sets `stream:true` when requested.
  - It does not strip or consume `generate`.
- `internal/proxy/handler/responses_codex_test.go`
  - Covers small HTTP `/v1/responses` and small sequential WebSocket frames.
  - It does not cover Codex prewarm, `input: []` follow-up after warmup, or >32KiB frame reads.

## External Protocol Evidence

- Official OpenAI WebSocket mode docs: WebSocket mode starts each turn with client `response.create`; the payload mirrors Responses create body, but transport-specific fields like `stream` and `background` are not used.
- Official OpenAI docs: clients may warm up request state with `response.create` + `generate:false`; the warmup returns a response id that can be chained with `previous_response_id`.
- Official OpenAI docs: WebSocket mode keeps the most recent previous-response state in connection-local memory; with `store:false`, no persisted fallback exists if an id is absent.
- `openai/codex` upstream tests confirm Codex sends `OpenAI-Beta: responses_websockets=2026-02-06`, prewarm `generate:false`, `tools: []`, and follow-up `previous_response_id` with `input: []`.

Sources:

- <https://developers.openai.com/api/docs/guides/websocket-mode>
- <https://openai.com/index/speeding-up-agentic-workflows-with-websockets/>
- <https://github.com/openai/codex/blob/main/codex-rs/core/tests/suite/client_websockets.rs>
- <https://github.com/openai/codex/blob/main/codex-rs/core/tests/suite/websocket_fallback.rs>

## Proposed Design

### 1. Introduce Responses WebSocket connection state

Add a small connection-local adapter state inside `handleResponsesWebSocket` or a helper object:

- latest response id
- latest full backend-capable request payload or safe reconstructed context
- latest successful warmup/generated response metadata

State is per WebSocket connection only. Do not store it globally or in DB for this issue.

### 2. Raise WebSocket read limit

After `websocket.Accept`, set a documented high enough read limit for Codex frames. The implementation should choose a bounded value based on real Codex payload size and repo safety, not unlimited reads.

Planning target: at least several MiB, with test coverage using >32KiB and a comment explaining the bound.

### 3. Consume `generate:false` at Tianji boundary

When a frame contains `generate:false`:

- Validate model/route as usual.
- Normalize model and store settings as usual.
- Do not forward `generate` to ChatGPT Codex backend.
- Either:
  - send a safe backend request if backend can accept a no-output warmup equivalent, then capture returned response id; or
  - synthesize a local warmup response id and cache the complete request state without calling backend.

Preferred conservative design: synthesize/cache locally if ChatGPT Codex backend has no proven no-output warmup equivalent. This avoids inventing backend semantics and prevents `generate` leakage.

### 4. Expand follow-up frames using cached state

When a later frame has `previous_response_id` matching cached connection-local state:

- If `input` is empty or incremental only, rebuild a backend-valid payload from cached warmup/full request plus incremental fields.
- Preserve allowed updated fields such as metadata/client_metadata only when safe and covered by tests.
- Do not forward stale `previous_response_id` blindly if the ChatGPT Codex backend cannot hydrate it.

When the id is missing:

- return redacted `previous_response_not_found`-style WebSocket error for `store:false`/local-only state;
- do not fallback to static API key or unrelated route.

### 5. Keep sequential frame semantics

Continue to process one frame at a time in the existing read loop. Do not multiplex concurrent responses on one connection.

### 6. Guard existing routes

Keep the existing DB-managed `openai/*` + `chatgpt_codex_backend` route behavior. Preserve direct HTTP/OpenAI API-key behavior outside this Codex-specific adapter path.

## Test Plan

RED tests to add before production changes:

- `TestCreateResponseWebSocket_CodexPrewarmConsumesGenerateFalse`
- `TestCreateResponseWebSocket_CodexPrewarmFollowupExpandsEmptyInputFromState`
- `TestCreateResponseWebSocket_AcceptsLargeCodexFrameOverDefaultReadLimit`
- No-regression for existing small sequential frame test.

Expected commands:

```bash
go test ./internal/proxy/handler -run 'CreateResponseWebSocket|Responses|Codex' -count=1
GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e
git diff --check
```

Live verification after implementation:

```bash
codex exec --json 'say OK'
```

Configured against `https://tianji.hohsiang.com.tw/v1` and `openai/gpt-5.5`; acceptance requires no `Reconnecting...` WebSocket fallback noise.

## Plan Review

- Official OpenAI docs checked: prewarm, `generate:false`, `previous_response_id`, connection-local memory, `store:false`, and sequential `response.create` semantics are part of WebSocket mode.
- `openai/codex` upstream tests checked: current Codex sends the exact beta header and prewarm/follow-up pattern named in the issue.
- Repo reality checked first: current code lacks read limit/state/prewarm handling and currently forwards raw `generate`.
- No owner input needed for Todo scope.
