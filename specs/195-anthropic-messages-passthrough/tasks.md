# Tasks: Anthropic /v1/messages Passthrough

**Input**: Design documents from `/specs/195-anthropic-messages-passthrough/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Shared helper needed by all user stories

- [x] T001 Write `MergeBetaHeaders` failing tests in `internal/provider/anthropic/oauth_test.go` — `TestMergeBetaHeaders_NoDuplicates`, `TestMergeBetaHeaders_Empty`. Confirm they fail.
- [x] T002 Implement `MergeBetaHeaders(existing, additional string) string` in `internal/provider/anthropic/oauth.go` — split by comma, deduplicate via set, rejoin. Confirm T001 tests pass.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Route registration that MUST be complete before any handler works

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T003 Register `POST /v1/messages` and `POST /messages` routes in `internal/proxy/server.go` — already existed in `registerLLMRoutes`
- [x] T004 Register `POST /v1/messages/count_tokens` and `POST /messages/count_tokens` routes in `internal/proxy/server.go` — already existed in `registerLLMRoutes`
- [x] T005 Register `POST /api/event_logging/batch` route in `internal/proxy/server.go` — pointing to `h.AnthropicEventLoggingBatch`

**Checkpoint**: Routes registered — handler implementation can begin

---

## Phase 3: User Story 1 — Core Passthrough + Streaming (Priority: P1) 🎯 MVP

**Goal**: Claude Code CLI can send `POST /v1/messages?beta=true` through TianjiLLM and get responses (streaming + non-streaming) from Anthropic with correct OAuth token + beta header forwarding.

**Independent Test**: `curl -X POST "http://localhost:4000/v1/messages?beta=true" -H "Authorization: Bearer <virtual-key>" -H "anthropic-beta: claude-code-20250219,oauth-2025-04-20" -H "anthropic-version: 2023-06-01" -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}],"max_tokens":10}'` returns Anthropic-native response.

### Tests for User Story 1 (MANDATORY - Principle IV) 🔴

> **MANDATORY: Write these tests FIRST from plan's ## Failing Tests section. Confirm they FAIL before any implementation task.**

- [x] T006 [P] [US1] Write `TestAnthropicMessages_ForwardsBodyUnmodified` in `internal/proxy/handler/anthropic_messages_test.go` — mock upstream verifies exact body received
- [x] T007 [P] [US1] Write `TestAnthropicMessages_ReplacesAuthWithUpstreamOAuth` in `internal/proxy/handler/anthropic_messages_test.go` — upstream receives `Authorization: Bearer sk-ant-oat01-...`, not virtual key
- [x] T008 [P] [US1] Write `TestAnthropicMessages_PreservesClientBetaHeaders` in `internal/proxy/handler/anthropic_messages_test.go` — upstream `anthropic-beta` contains all client betas + `oauth-2025-04-20`
- [x] T009 [P] [US1] Write `TestAnthropicMessages_ForwardsAllClientHeaders` in `internal/proxy/handler/anthropic_messages_test.go` — upstream receives `User-Agent`, `x-app`, `X-Stainless-*` from client
- [x] T010 [P] [US1] Write `TestAnthropicMessages_ForwardsQueryParams` in `internal/proxy/handler/anthropic_messages_test.go` — upstream URL includes `?beta=true`
- [x] T011 [P] [US1] Write `TestAnthropicMessages_ForwardsAnthropicVersion` in `internal/proxy/handler/anthropic_messages_test.go` — upstream gets client's `anthropic-version` or default `2023-06-01`
- [x] T012 [P] [US1] Write `TestAnthropicMessages_SuccessResponse` in `internal/proxy/handler/anthropic_messages_test.go` — client receives 200 + Anthropic JSON unmodified
- [x] T013 [P] [US1] Write `TestAnthropicMessages_ErrorResponse` in `internal/proxy/handler/anthropic_messages_test.go` — client receives upstream error status + body
- [x] T014 [P] [US1] Write `TestAnthropicMessages_StreamingSSE` in `internal/proxy/handler/anthropic_messages_test.go` — response has `Content-Type: text/event-stream`, SSE events forwarded
- [x] T015 [P] [US1] Write `TestAnthropicMessages_NonStreaming` in `internal/proxy/handler/anthropic_messages_test.go` — complete JSON returned when stream is false/absent
- [x] T016 [P] [US1] Write `TestAnthropicMessages_RegularAPIKey` in `internal/proxy/handler/anthropic_messages_test.go` — non-OAuth key uses `x-api-key` header
- [x] T017 [P] [US1] Write `TestAnthropicMessages_InvalidJSON` in `internal/proxy/handler/anthropic_messages_test.go` — returns 400 for malformed body
- [x] T018 [P] [US1] Write `TestAnthropicMessages_NoModelConfig` in `internal/proxy/handler/anthropic_messages_test.go` — returns 404 when no anthropic config found

**Run and confirm all tests fail:**
```bash
go test ./internal/proxy/handler/... -run "TestAnthropicMessages" -v
```

### Implementation for User Story 1

- [x] T019 [US1] Implement `AnthropicMessages` handler — already existed in `internal/proxy/handler/native_format.go` via `nativeProxy()`. Fixed: beta header dedup via `MergeBetaHeaders`, added `anthropic-dangerous-direct-browser-access: true` for OAuth:
  - Read body, extract `model` field (partial JSON unmarshal)
  - Restore body via `io.NopCloser(bytes.NewReader(body))`
  - Call `findModelConfig(model)` → resolve upstream API key
  - Create `httputil.ReverseProxy` with `Director` that:
    - Sets `URL.Scheme`, `URL.Host`, `URL.Path` to `api.anthropic.com/v1/messages`
    - Preserves `URL.RawQuery` from client (carries `?beta=true`)
    - Copies all client headers except `Authorization`, `Host`, `Content-Length`
    - If OAuth token: set `Authorization: Bearer`, `anthropic-dangerous-direct-browser-access: true`, merge beta headers via `MergeBetaHeaders`
    - If regular API key: set `x-api-key`
    - Set `anthropic-version` from client or default `2023-06-01`
  - Set `FlushInterval: -1` for immediate SSE flush
  - Serve via `proxy.ServeHTTP(w, r)`

- [x] T020 [US1] Run all US1 tests and confirm they pass:
  ```bash
  go test ./internal/proxy/handler/... -run "TestAnthropicMessages" -v
  ```

**Checkpoint**: Core passthrough working — Claude Code CLI can use sonnet/opus through TianjiLLM

---

## Phase 4: User Story 2 — Count Tokens Passthrough (Priority: P2)

**Goal**: `/v1/messages/count_tokens` forwards to Anthropic for token estimation.

**Independent Test**: `curl -X POST "http://localhost:4000/v1/messages/count_tokens" -H "Authorization: Bearer <virtual-key>" -d '{"model":"claude-sonnet-4-6","messages":[...]}'` returns `{"input_tokens": N}`.

### Tests for User Story 2 (MANDATORY - Principle IV) 🔴

- [x] T021 [P] [US2] Write `TestAnthropicMessages_CountTokensPassthrough` in `internal/proxy/handler/anthropic_messages_test.go` — upstream receives request at `/v1/messages/count_tokens`, response forwarded

### Implementation for User Story 2

- [x] T022 [US2] Implement `AnthropicMessagesCountTokens` handler — already existed in `internal/proxy/handler/native_format.go` via `nativeProxy()`

- [x] T023 [US2] Run US2 test and confirm it passes:
  ```bash
  go test ./internal/proxy/handler/... -run "TestAnthropicMessages_CountTokens" -v
  ```

**Checkpoint**: Token counting works through proxy

---

## Phase 5: User Story 3 — Event Logging Stub (Priority: P2)

**Goal**: `/api/event_logging/batch` returns `{"status":"ok"}` to prevent Claude Code CLI telemetry 404s.

**Independent Test**: `curl -X POST "http://localhost:4000/api/event_logging/batch"` returns 200 + `{"status":"ok"}`.

### Tests for User Story 3 (MANDATORY - Principle IV) 🔴

- [x] T024 [P] [US3] Write `TestAnthropicEventLogging_ReturnsOK` in `internal/proxy/handler/anthropic_messages_test.go` — returns 200 + `{"status":"ok"}` without hitting upstream

### Implementation for User Story 3

- [x] T025 [US3] Implement `AnthropicEventLoggingBatch` handler in `internal/proxy/handler/anthropic_event_logging.go` — return `{"status":"ok"}` with 200 status

- [x] T026 [US3] Run US3 test and confirm it passes:
  ```bash
  go test ./internal/proxy/handler/... -run "TestAnthropicEventLogging" -v
  ```

**Checkpoint**: No more 404 errors from Claude Code CLI telemetry

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [x] T027 Run full test suite to verify no regressions: `go test ./... -race -count=1`
- [x] T028 Run linter: `golangci-lint run`
- [x] T029 Build binary: `go build ./cmd/tianji/...`
- [x] T030 Run quickstart.md end-to-end validation against live Anthropic API

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — `MergeBetaHeaders` helper + tests
- **Foundational (Phase 2)**: Depends on Setup — route registration in server.go
- **US1 (Phase 3)**: Depends on Phase 1 + 2 — core passthrough handler
- **US2 (Phase 4)**: Depends on Phase 3 (reuses handler logic)
- **US3 (Phase 5)**: Depends on Phase 2 only (independent stub)
- **Polish (Phase 6)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Core passthrough — no story dependencies, MUST complete first as MVP
- **US2 (P2)**: Count tokens — reuses US1's ReverseProxy logic, can share handler file
- **US3 (P2)**: Event logging stub — fully independent, can run in parallel with US2

### Parallel Opportunities

- T001 + T002 can run in parallel (different files)
- T003 + T004 + T005 can run in parallel (different route groups in same file, but sequential is safer)
- T006–T018 (all US1 tests) can run in parallel (same test file, different test functions)
- US2 + US3 can run in parallel after US1 completes

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task T006: "TestAnthropicMessages_ForwardsBodyUnmodified"
Task T007: "TestAnthropicMessages_ReplacesAuthWithUpstreamOAuth"
Task T008: "TestAnthropicMessages_PreservesClientBetaHeaders"
Task T009: "TestAnthropicMessages_ForwardsAllClientHeaders"
Task T010: "TestAnthropicMessages_ForwardsQueryParams"
Task T011: "TestAnthropicMessages_ForwardsAnthropicVersion"
Task T012: "TestAnthropicMessages_SuccessResponse"
Task T013: "TestAnthropicMessages_ErrorResponse"
Task T014: "TestAnthropicMessages_StreamingSSE"
Task T015: "TestAnthropicMessages_NonStreaming"
Task T016: "TestAnthropicMessages_RegularAPIKey"
Task T017: "TestAnthropicMessages_InvalidJSON"
Task T018: "TestAnthropicMessages_NoModelConfig"
# All 13 tests write to same file but are independent functions — write as single batch
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: `MergeBetaHeaders` helper
2. Complete Phase 2: Route registration
3. Complete Phase 3: US1 tests → handler implementation
4. **STOP and VALIDATE**: Test with real OAuth token against Anthropic API
5. Deploy if passing

### Incremental Delivery

1. Setup + Foundational → Routes ready
2. Add US1 → Core passthrough works → Deploy (MVP!)
3. Add US2 → Count tokens works → Deploy
4. Add US3 → Event logging stub → Deploy
5. Polish → Full test suite green → Final deploy

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each phase completion
- Stop at any checkpoint to validate story independently
