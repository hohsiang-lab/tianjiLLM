# Tasks: Passthrough Spend Tracking

**Input**: `specs/203-passthrough-spend-tracking/plan.md`, `research.md`
**Branch**: `203-passthrough-spend-tracking`

**Tests**: Mandatory (Constitution Principle IV). Task 1 of every user story writes failing tests. No implementation begins until tests compile and fail.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup

**Purpose**: No project structure changes needed — all work goes into existing packages.

- [ ] T001 Verify `internal/proxy/passthrough/` compiles cleanly and all existing tests pass: `go test ./internal/proxy/passthrough/... -v`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Confirm exact signatures before expanding the interface. Any discrepancy found here must be resolved before US1/US2 begin.

- [ ] T002 Audit `internal/proxy/passthrough/handlers.go` — confirm every `LoggingHandler` implementation and note which currently lose cache tokens or model name (Anthropic drops both; Vertex/Gemini drop model name)
- [ ] T003 [P] Audit `internal/proxy/passthrough/router.go` — confirm `NewRouter` signature and all places `logger.ParseUsage` is called (non-streaming ModifyResponse + streamingReader.Close)
- [ ] T004 [P] Confirm `callback.Registry` + `pricing.Default().Cost()` are importable from `passthrough` package without circular dependency: `go build ./internal/proxy/passthrough/...` after adding a blank import

**Checkpoint**: Audit complete — proceed to US1 and US2 in parallel

---

## Phase 3: User Story 1 — Expand LoggingHandler interface (P1) 🎯 MVP

**Goal**: `LoggingHandler` returns full usage data (cache tokens + model name + SSE variant) so the router has everything needed to build a correct `LogData`.

**Independent Test**: `go test ./internal/proxy/passthrough/... -run "TestAnthropicLoggingHandler|TestGemini|TestVertexAI"` — all handler tests pass with cache tokens and model name verified.

### Failing Tests for User Story 1 🔴

> Write these tests FIRST. Confirm they FAIL (`--- FAIL`) before any implementation.

- [ ] T005 [P] [US1] Write failing tests for `AnthropicLoggingHandler` in `internal/proxy/passthrough/handlers_test.go`:
  - `TestAnthropicLoggingHandler_ParseUsage_CacheTokens` — body with `cache_read_input_tokens` + `cache_creation_input_tokens` → correct cacheRead, cacheCreation, modelName returned
  - `TestAnthropicLoggingHandler_ParseSSEUsage_Streaming` — SSE bytes with `message_start` + `message_delta` → model extracted from message_start, output from message_delta
  - `TestAnthropicLoggingHandler_ParseSSEUsage_CacheTokens` — SSE with cache fields in `message_start` → cacheRead + cacheCreation populated
- [ ] T006 [P] [US1] Write failing tests for Gemini/Vertex in `internal/proxy/passthrough/handlers_test.go`:
  - `TestGeminiLoggingHandler_ParseSSEUsage` — multi-chunk SSE → last usageMetadata chunk wins
  - `TestVertexAILoggingHandler_ParseSSEUsage` — Vertex AI SSE format → prompt + completion extracted

### Implementation for User Story 1

- [ ] T007 [US1] Expand `LoggingHandler` interface in `internal/proxy/passthrough/handlers.go`:
  - `ParseUsage(body []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)`
  - `ParseSSEUsage(raw []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)`
  - Keep `ProviderName() string`
- [ ] T008 [P] [US1] Update `AnthropicLoggingHandler` in `internal/proxy/passthrough/handlers.go`:
  - `ParseUsage`: add `cache_read_input_tokens`, `cache_creation_input_tokens`, `model` to parsed struct; return all five values (prompt = input + cacheRead + cacheCreation)
  - `ParseSSEUsage`: port logic from `native_format.go:parseSSEUsage` case `"anthropic"` — model from `message_start`, output from `message_delta`, cache from `message_start`
- [ ] T009 [P] [US1] Update `GeminiLoggingHandler` + `VertexAILoggingHandler` in `internal/proxy/passthrough/handlers.go`:
  - `ParseUsage`: add modelVersion/model field to parsed struct; return modelName
  - `ParseSSEUsage`: port logic from `native_format.go:parseSSEUsage` cases `"gemini"` — take last usageMetadata chunk
- [ ] T010 [P] [US1] Update `OpenAILoggingHandler` + `CohereLoggingHandler` + `BaseLoggingHandler` in `internal/proxy/passthrough/handlers.go`:
  - Add `ParseSSEUsage` stub (return zeros) for OpenAI-format SSE if no streaming support needed; add modelName to ParseUsage
- [ ] T011 [US1] Run tests: `go test ./internal/proxy/passthrough/... -run "TestAnthropicLoggingHandler|TestGemini|TestVertexAI" -v` — confirm T005/T006 now pass

**Checkpoint**: All handler tests pass. LoggingHandler returns full usage. US2 can now begin.

---

## Phase 4: User Story 2 — Router calls LogSuccess (P1)

**Goal**: After every successful request through `passthrough.Router`, `callbacks.LogSuccess` is called with correct model, tokens, cost, and context values.

**Independent Test**: `go test ./internal/proxy/passthrough/... -run "TestRouter"` — all router spend-tracking tests pass.

### Failing Tests for User Story 2 🔴

> Write these tests FIRST. Confirm they FAIL before any implementation.

- [ ] T012 [US2] Write failing router tests in `internal/proxy/passthrough/router_test.go`:
  - `TestRouter_NonStreaming_CallsLogSuccess` — mock upstream returns 200 JSON with usage; after `ServeHTTP`, mockCallbacks.LogSuccess called once with correct model + prompt + completion + provider
  - `TestRouter_Streaming_CallsLogSuccessAfterStreamEnd` — mock upstream returns SSE stream; after response body fully consumed, LogSuccess called with correct tokens
  - `TestRouter_NonStreaming_CacheTokensLogged` — Anthropic JSON with cache fields → `LogData.CacheReadInputTokens` + `CacheCreationInputTokens` populated
  - `TestRouter_NoCallbacks_DoesNotPanic` — `NewRouter(..., nil)` → request completes without panic
  - `TestRouter_NonOK_DoesNotCallLogSuccess` — upstream returns 429 → LogSuccess NOT called
  - `TestRouter_LogData_HasContextValues` — context carries `ContextKeyTokenHash` + `ContextKeyTeamID` → LogData.APIKey + TeamID populated
  - `TestRouter_LogData_HasDuration` — `LogData.EndTime.After(LogData.StartTime)`

### Implementation for User Story 2

- [ ] T013 [US2] Add `callbacks *callback.Registry` field to `Router` struct in `internal/proxy/passthrough/router.go`; update `NewRouter` signature: `func NewRouter(endpoints []Endpoint, guardrail GuardrailHook, callbacks *callback.Registry) *Router`
- [ ] T014 [US2] Add `buildPassthroughLogData` helper in `internal/proxy/passthrough/router.go` (mirrors `buildNativeLogData` in `native_format.go`):
  ```
  func buildPassthroughLogData(ctx, providerName, modelName, startTime, prompt, completion, cacheRead, cacheCreation) callback.LogData
  ```
  — uses `buildBaseLogData` equivalent: reads `ContextKeyTokenHash`, `ContextKeyTeamID`, `ContextKeyOrgID`, `ContextKeyRequesterIP` from ctx
- [ ] T015 [US2] Add `ctx`, `startTime`, `requestModel`, `callbacks` fields to `streamingReader` struct in `internal/proxy/passthrough/router.go`
- [ ] T016 [US2] Update `streamingReader.Close()` in `internal/proxy/passthrough/router.go`:
  - Call `logger.ParseSSEUsage(body)` instead of `logger.ParseUsage(body)`
  - If `callbacks != nil`: `go callbacks.LogSuccess(buildPassthroughLogData(...))`
- [ ] T017 [US2] Update non-streaming branch in `Router.Handler()` `ModifyResponse` in `internal/proxy/passthrough/router.go`:
  - Capture `startTime := time.Now()` before proxy call (move to closure start)
  - Capture `requestModel` from request body before proxy fires
  - Call `logger.ParseUsage(body)` with new signature
  - If `callbacks != nil`: `go callbacks.LogSuccess(buildPassthroughLogData(...))`
- [ ] T018 [US2] Update streaming branch in `Router.Handler()` `ModifyResponse` in `internal/proxy/passthrough/router.go`:
  - Pass `ctx`, `startTime`, `requestModel`, `callbacks` into `streamingReader` on construction
- [ ] T019 [US2] Add required imports to `internal/proxy/passthrough/router.go`: `internal/callback`, `internal/pricing`, `internal/proxy/middleware`
- [ ] T020 [US2] Run tests: `go test ./internal/proxy/passthrough/... -run "TestRouter" -v` — confirm T012 tests now pass

**Checkpoint**: Router spend tracking works end-to-end in tests.

---

## Phase 5: User Story 3 — Wire into main.go (P1)

**Goal**: Production `passthrough.Router` instance receives `callbackRegistry` so spend is logged in the running server.

**Independent Test**: Deploy to local OrbStack, send a request to `/v1/anthropic/v1/messages`, verify a row appears in `SpendLogs` table.

### Failing Tests for User Story 3 🔴

> No automated test possible for main.go wiring (integration-only). Manual verification step instead.

- [ ] T021 [US3] Update `passthrough.NewRouter` call in `cmd/tianji/main.go` (line ~342): pass `callbackRegistry` as third argument

### Implementation for User Story 3

- [ ] T022 [US3] Verify compile: `go build ./cmd/tianji/...` — no errors
- [ ] T023 [US3] Run full test suite: `go test ./internal/... 2>&1 | grep -E "FAIL|ok"` — all packages pass

**Checkpoint**: Server builds cleanly. All tests pass.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T024 [P] Remove now-redundant `stdout-only` log.Printf in `streamingReader.Close()` and non-streaming branch of `router.go` — replaced by proper spend logging
- [ ] T025 [P] Update `AnthropicLoggingHandler` test file `internal/proxy/passthrough/handlers_test.go` — remove/update any existing tests that tested the old 2-return-value signature
- [ ] T026 Confirm `native_format.go` unchanged: `git diff HEAD internal/proxy/handler/native_format.go` — no diff
- [ ] T027 [P] Run linter: `make lint` — 0 issues

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational audit)
    ↓ ↓ (can start in parallel)
Phase 3 (US1: interface)    Phase 4 (US2: router) — blocked on Phase 3 (needs new interface)
    ↓
Phase 4 (US2: router)
    ↓
Phase 5 (US3: main.go wiring)
    ↓
Phase 6 (Polish)
```

### User Story Dependencies

- **US1** depends on: Phase 2 complete
- **US2** depends on: US1 complete (router calls `logger.ParseSSEUsage` — new interface method)
- **US3** depends on: US2 complete (`NewRouter` signature changes)

### Within Each User Story

1. Write failing tests → confirm `--- FAIL`
2. Implement interface/struct changes
3. Implement logic
4. Run tests → confirm `--- PASS`

### Parallel Opportunities

- T005 + T006 can run in parallel (different test functions, same file — coordinate)
- T008 + T009 + T010 can run in parallel (different struct implementations, same file)
- T024 + T025 + T026 + T027 can all run in parallel (final polish)

---

## Implementation Strategy

### MVP (All three user stories are P1 — deliver together)

1. Phase 1: Verify baseline (5 min)
2. Phase 2: Audit — understand exact current state (15 min)
3. Phase 3: Expand interface — write failing tests → implement (30 min)
4. Phase 4: Wire router — write failing tests → implement (45 min)
5. Phase 5: Wire main.go — one-line change + compile check (5 min)
6. Phase 6: Polish (10 min)

### Commit Strategy

- Commit after T011 (US1 complete): `feat: expand LoggingHandler interface with full usage + SSE parsing`
- Commit after T020 (US2 complete): `feat: inject spend tracking into passthrough.Router`
- Commit after T023 (US3 complete): `feat: wire callbackRegistry into passthrough router in main`
- Commit after T027 (polish): `chore: remove stdout-only logs, lint clean`

---

## Notes

- `native_format.go` must remain untouched throughout — any diff is a bug
- `NewRouter` signature change is breaking — `main.go` (T021) must be updated in the same branch
- Cache token fix in `AnthropicLoggingHandler` is a correctness bug fix bundled into US1 — not optional
- After PR #102 merges to main and this branch rebases, `ContextKeyRequesterIP` will be automatically available via `buildPassthroughLogData` context extraction — no additional changes needed
