# Contract: `/v1/responses` request-log pipeline

## HTTP Success Contract

Given:

- `POST /v1/responses`
- model routes to `chatgpt_codex_backend`
- upstream returns 2xx with a valid Responses payload or stream

Then:

- client receives the same status/body semantics as before
- exactly one success log event is emitted
- log event includes model, request id when available, duration, upstream subscription attribution when available, usage/cost when derivable
- no failure log event is emitted

## HTTP Failure Contract

Given:

- `POST /v1/responses`
- model routes to `chatgpt_codex_backend`
- upstream transport fails or backend returns non-2xx

Then:

- client receives the existing redacted failure response semantics
- exactly one failure/error log event is emitted
- no success log event is emitted
- error metadata is redacted

## WebSocket Generation Success Contract

Given:

- `GET /v1/responses` WebSocket handshake succeeds
- client sends a real generation `response.create`
- Tianji calls upstream model work
- upstream emits successful Responses lifecycle through completion

Then:

- client receives forwarded Responses WebSocket messages
- exactly one success log event is emitted for that generation frame
- connection-local response state behavior remains unchanged

## WebSocket Generation Failure Contract

Given:

- `GET /v1/responses` WebSocket handshake succeeds
- client sends a real generation `response.create`
- upstream transport or backend generation fails

Then:

- client receives existing redacted WebSocket error semantics
- exactly one failure/error log event is emitted
- no success log event is emitted

## WebSocket Prewarm Contract

Given:

- `GET /v1/responses` WebSocket handshake succeeds
- client sends `response.create` with `generate:false`
- Tianji handles the frame locally and does not call upstream model work

Then:

- client may receive synthetic lifecycle events
- no success log event is emitted
- no failure log event is emitted
- no backend model call occurs

## UI Visibility Contract

Given:

- a success or failure log event is emitted through the existing pipeline

Then:

- `/ui/logs?page=1&time_range=24h&live_tail=true` can display the row through existing `CountRequestLogs` / `ListRequestLogs` queries
- filters for model, request id, status, team, API key, and upstream token continue using existing query behavior
