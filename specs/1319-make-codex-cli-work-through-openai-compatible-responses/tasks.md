# Tasks: HO-1319 Codex CLI `/v1/responses`

## Phase 1 - RED Coverage

- [X] T001 Re-read Linear HO-1319 description/comments and this SpecKit package before implementation.
- [X] T002 Add a failing test for Codex CLI-style `GET /v1/responses` websocket upgrade request with `OpenAI-Beta: responses_websockets=2026-02-06`, `x-client-request-id`, `session_id`, `thread_id`, and `Authorization`.
- [X] T003 Add a failing test that sends websocket `response.create` frames with `model=openai/gpt-5.5` and user `input` without relying on client-provided HTTP `stream`.
- [X] T004 Add a failing assertion that route registration / non-405 response alone is not enough without `response.create` frame handling.
- [X] T005 Add a failing test for `/v1/responses` with DB-managed `openai/*` wildcard and model `openai/gpt-5.5`.
- [X] T006 Add a failing test proving `/v1/responses` does not silently use the wrong subscription transport for Codex-configured wildcard models.
- [ ] T007 Add stale-refreshable credential coverage for `/v1/responses`.
- [ ] T008 Add disabled/unrefreshable credential coverage with redaction assertions.
- [ ] T009 Add no-regression coverage for existing `/v1/chat/completions` streaming Codex route.

## Phase 2 - Primary Transport

- [X] T010 Register or route `GET /responses` under both bare and `/v1` route groups for the Codex CLI websocket/upgrade shape.
- [X] T011 Implement explicit Codex-compatible `/v1/responses` websocket handling for Responses WebSocket, not Realtime WebSocket.
- [X] T012 Parse and route `response.create` websocket frames through the same model/credential selection used by the owned `/v1/responses` path.
- [X] T012a Keep the WebSocket connection open for sequential `response.create` frames until client close or error.
- [X] T013 Ensure unsupported non-websocket `GET /v1/responses` returns a clear redacted error without hiding websocket support failures.
- [ ] T014 Add logging/diagnostics that identify selected route/transport without leaking credentials.

## Phase 3 - Route Resolution

- [X] T015 Update `/v1/responses` model route resolution to honor DB-managed wildcard `openai/*` for `openai/gpt-5.5`.
- [X] T016 Use the model's intended subscription transport where required for Codex-compatible `/v1/responses`.
- [X] T017 Preserve generic direct OpenAI `/v1/responses` behavior for non-Codex models and API-key-backed models.
- [X] T018 Preserve `direct_openai_http` behavior where the model config intentionally selects direct HTTP.

## Phase 4 - Credential Health

- [X] T019 Ensure healthy subscription credential succeeds through `/v1/responses`.
- [ ] T020 Ensure expired refreshable credential refreshes and retries through the same intended transport.
- [ ] T021 Ensure disabled/unrefreshable credential returns redacted actionable failure.
- [ ] T022 Ensure refresh failure does not downgrade to static API key or unrelated fallback path.

## Phase 5 - Verification

- [X] T023 Run targeted handler tests: `go test ./internal/proxy/handler -run 'Responses|OpenAISubscription|Codex' -count=1`.
- [ ] T024 Run issue E2E: `GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e`.
- [X] T025 Run existing package/service tests required by touched files.
- [X] T026 Run `git diff --check`.
- [ ] T027 Live verify Codex CLI can complete `say OK` through Tianji `/v1` with `openai/gpt-5.5`.
- [ ] T028 Record verification evidence in PR and issue thread.

## Phase 6 - Review Guardrails

- [ ] T029 Confirm no production docs/tests contain API keys, subscription tokens, refresh tokens, or account secrets.
- [ ] T030 Confirm no fallback-based solution is marked accepted without explicit owner confirmation.
- [X] T031 Confirm existing HO-1314 chat-completions contract remains green.
