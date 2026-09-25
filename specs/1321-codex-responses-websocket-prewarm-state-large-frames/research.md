# Research: HO-1321 Codex Responses WebSocket adapter

## Inputs Checked

- Linear HO-1321 description and comments.
- Memory: `memory/2026-05-12.md` HO-1319 live follow-up and HO-1321 creation notes.
- Repo reality at `origin/main` commit `af5eb69`.
- Official OpenAI WebSocket mode docs.
- OpenAI WebSocket announcement article.
- `openai/codex` upstream WebSocket tests.

## Findings

### Official Responses WebSocket semantics

OpenAI WebSocket mode connects to `/v1/responses`. Each turn starts with client `response.create`; the payload mirrors Responses create body, but transport-specific HTTP fields like `stream` and `background` are not used.

The docs explicitly allow prewarm with `response.create` and `generate:false`. That warmup prepares request state and returns a response id. The next generated turn can chain from it using `previous_response_id`.

Continuation uses connection-local memory for the most recent previous-response state. This is important for `store:false` / ZDR-compatible use because no persisted fallback exists if the id is missing.

Source: <https://developers.openai.com/api/docs/guides/websocket-mode>

### Codex client behavior

Current `openai/codex` upstream tests show the Codex Responses WebSocket path sends:

- handshake header `OpenAI-Beta: responses_websockets=2026-02-06`
- warmup `response.create` with `generate:false`
- warmup `tools: []`
- follow-up `response.create` with `previous_response_id` from warmup
- follow-up `input: []` for the actual generation turn in the prewarm reuse test

Source: <https://github.com/openai/codex/blob/main/codex-rs/core/tests/suite/client_websockets.rs>

### Tianji current WebSocket handling

`internal/proxy/handler/responses.go` currently accepts WebSocket upgrades and reads frames in a loop. It does not call `SetReadLimit`, so it inherits coder/websocket's default read behavior.

Each frame is decoded as a standalone `response.create`, model-routed, and sent through `transport.BuildResponsesRequest`. There is no per-connection previous-response cache.

### Tianji current backend payload normalization

`internal/provider/chatgptcodex/transport.go` has `normalizeResponsesPayload(payload, stream)`:

- copies every raw field
- normalizes `model`
- defaults `store:false`
- sets `stream:true` when needed

It does not remove `generate`. Therefore Codex prewarm `generate:false` is currently forwarded to ChatGPT Codex backend.

### Existing test gap

`internal/proxy/handler/responses_codex_test.go` already proves small WebSocket `response.create` frames can be routed and that sequential frames work when the second frame contains non-empty user input.

It does not prove:

- prewarm `generate:false`
- backend does not receive `generate`
- follow-up `previous_response_id + input: []`
- large frame >32KiB
- true Codex CLI primary transport without reconnect noise

## Decision

HO-1321 implementation must treat Tianji as a protocol adapter, not a transparent field-forwarding proxy, for Codex Responses WebSocket frames.

The minimum safe strategy is:

1. accept large enough Codex frames with a bounded read limit;
2. consume `generate:false` locally;
3. cache warmup request state and response id on the WebSocket connection;
4. expand later `previous_response_id` frames into backend-valid payloads;
5. reject missing state explicitly instead of silently forwarding incomplete requests.

## Risks

- A pure read-limit fix will leave `generate:false` unsupported.
- A pure `generate` strip will still fail follow-up `input: []`.
- Global persisted state would exceed the issue scope and could create privacy/ZDR concerns.
- Overly high or unlimited WebSocket read limits could create memory pressure; the implementation should choose a bounded value.
