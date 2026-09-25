# Research: HO-1319 Codex CLI `/v1/responses`

## Inputs Checked

- Linear HO-1319 description and comments.
- Memory: `memory/2026-05-12.md` Tianji Codex live contract after HO-1314.
- Repo reality on `origin/main` at `73d5311`.
- Official OpenAI Codex config reference.
- GitHub search for current Codex CLI custom gateway behavior.

## Findings

### Codex config surface

Official OpenAI Codex config reference documents custom provider fields:

- `model_providers.<id>.base_url`: API base URL for the model provider.
- `model_providers.<id>.supports_websockets`: whether the provider supports Responses API WebSocket transport.
- `model_providers.<id>.wire_api`: `responses`, default when omitted.
- Built-in provider IDs such as `openai` are reserved and cannot be overridden.

The current official config reference page did not expose `openai_base_url` in searchable text during this Todo pass. Linear live verification remains the source that `openai_base_url` currently redirects Codex built-in OpenAI traffic to Tianji in Norman's environment.

Relevant source: <https://developers.openai.com/codex/config-reference>

### Responses WebSocket protocol shape

Official OpenAI Responses WebSocket docs state that WebSocket mode connects to `/v1/responses` and starts each turn by sending a client `response.create` event. The payload mirrors the Responses create body, while transport-specific HTTP fields such as `stream` and `background` are not used. A single WebSocket connection can receive multiple sequential `response.create` messages.

Current `openai/codex` source/test evidence further narrows the Codex CLI shape:

- Handshake includes `OpenAI-Beta: responses_websockets=2026-02-06`.
- Handshake includes request/session identity headers such as `x-client-request-id`, `session_id`, and `thread_id`.
- The websocket body contains `type=response.create`, `model`, and `input`; Tianji must not require client-provided HTTP `stream=true`.
- Codex keeps a turn-scoped WebSocket connection and may use a prewarm `response.create`, so closing after the first completed frame can trigger reconnect/fallback behavior.

This means HO-1319 tests must fail any implementation that only registers `GET /responses` or only proves a non-405 handshake. The RED test must prove `response.create` frames are accepted, routed, bridged to backend streaming, and reusable on the same connection.

Relevant sources:
- <https://developers.openai.com/api/docs/guides/websocket-mode>
- <https://github.com/openai/codex/blob/main/codex-rs/core/tests/suite/client_websockets.rs>
- <https://github.com/openai/codex/blob/main/codex-rs/core/src/client.rs>

### Current Tianji route registration

`internal/proxy/server.go` registers:

- `POST /responses` -> `CreateResponse`
- `GET /responses/{response_id}` -> `GetResponse`
- `POST /responses/{response_id}/cancel` -> `CancelResponse`
- `GET /responses/{response_id}/input_items` -> `ListResponseInputItems`

It does not register `GET /responses` for websocket upgrade. The existing websocket relay is in `internal/proxy/handler/realtime.go` and is scoped to Realtime behavior, not `/v1/responses`.

### Current `/v1/responses` implementation

`internal/proxy/handler/responses.go` calls `openAIEndpointProxy`.

`internal/proxy/handler/assistants.go` resolves a model-specific OpenAI endpoint route. If the model has `openai_subscription_credential_ids`, it calls:

`resolveOpenAISubscriptionAttemptOrder(ctx, cfg.TianjiParams, openAISubscriptionTransportDirectHTTP)`

This means `/v1/responses` currently selects `direct_openai_http` for subscription credentials even when the DB-managed `openai/*` model row uses `openai_subscription_transport = "chatgpt_codex_backend"` for chat-completions.

### Existing coverage gap

`internal/proxy/handler/openai_subscription_endpoints_test.go` covers `/v1/responses` mainly with `gpt-4o` and generic subscription endpoint routing.

`test/e2e/codex_subscription_route_test.go` covers DB-managed `openai/*` + `openai/gpt-5.5`, but only for `/v1/chat/completions`.

There is no issue-owned test proving Codex CLI's actual `/v1/responses` path works with DB-managed `openai/*` wildcard and `openai/gpt-5.5`.

### Prior Tianji work

- HO-1300 fixed `store:false` for ChatGPT Codex backend chat payloads.
- HO-1306 added streaming support for Codex backend chat route.
- HO-1314 added E2E coverage for DB-managed Codex subscription chat-completions route.

Those were necessary but did not cover `/v1/responses`.

## Decision

Scope HO-1319 to the complete Codex CLI `/v1/responses` chain, with primary transport success as the acceptance target.

Do not accept a plan whose only milestone is "HTTP fallback works after websocket probe fails." Owner explicitly rejected that direction unless separately confirmed.

## Prior Art / External Signal

GitHub issue search found current Codex CLI reports around custom `openai_base_url` and WebSocket-to-HTTPS fallback losing authorization headers. This supports treating the fallback path as risky and not as the default acceptance path.

Search result: <https://github.com/openai/codex/issues/15492>

## Risks

- Implementing `/v1/responses` websocket by reusing Realtime relay may mismatch protocol semantics if Codex Responses WebSocket differs from Realtime.
- Registering `GET /responses` or returning `426` can silence the visible 405 without making primary transport usable; tests must include first-frame `response.create` and event semantics.
- Routing all `/v1/responses` subscription traffic through Codex backend could regress generic official OpenAI Responses API behavior.
- Live credential health can mask route fixes; tests need a stale-refreshable fixture and a disabled credential fixture.
