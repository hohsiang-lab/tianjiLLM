---

description: "Executable task list for the standard OpenAI-compatible API"

---

# Tasks: Standard OpenAI-Compatible API

**Input**: Design documents from
`/specs/1465-openai-compatible-api/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`,
`contracts/openai-v1.md`, and `contracts/capability-matrix.md`.

**Tests**: Tests are required by the feature specification and by the
constitution's failing-tests-first rule. Each user-story phase starts with
tests that must fail against the current implementation.

## Phase 0: Upstream Evidence Gate

**Purpose**: Establish what the authenticated Codex upstream actually
supports before enabling any structured-output or sampling capability.

- [X] T001 [P] Add and run the environment-gated redacted authenticated Codex capability probe, recording acceptance, semantics, and unknowns in `test/e2e/codex_capability_probe_test.go` and `specs/1465-openai-compatible-api/research.md`
- [X] T002 [P] Add deterministic Responses/SSE capability fixtures for `text.format`, content, tool-call fragments, refusal, completion, and usage-only terminal events in `test/fixtures/openai/codex_capabilities/responses_events.json`
- [X] T003 Update the Phase 0 evidence table and set each Codex capability bit explicitly in `specs/1465-openai-compatible-api/research.md`

**Checkpoint**: Unverified Codex capabilities remain false; no implementation
or smoke test may claim them as supported.

## Phase 1: Setup (Shared Contract Fixtures)

**Purpose**: Prepare reusable local upstreams and contract helpers without
changing production behavior.

- [X] T004 [P] Extend the OpenAI-compatible mock upstream builder with injectable base URLs, JSON responses, and SSE streams in `testutil/openaitest/upstream_server_test.go`
- [X] T005 [P] Add shared authenticated contract-server builders for direct HTTP and stream-only backends in `test/contract/helpers_test.go`
- [X] T006 [P] Add standard error and SSE decoding assertions used by contract tests in `test/contract/assertions_test.go`

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the common types and validation seams required by every user
story. No story implementation starts before this phase is complete.

- [X] T007 [P] Add backend/model capability keys, records, matrix lookup, and conservative fail-closed defaults in `internal/model/capability.go`
- [X] T008 [P] Extend chat/stream response types with refusal, ordered tool-call fragments, and complete usage fields in `internal/model/response.go`
- [X] T009 [P] Add standard client-error constructors and unsupported-parameter classifications in `internal/model/errors.go`
- [X] T010 Add resolved-route capability lookup and declared-parameter validation in `internal/proxy/handler/capability.go`
- [X] T011 Add one sanitized standard error writer for invalid values, unsupported parameters, invalid requests, model-not-found, and safe upstream failures in `internal/proxy/handler/errors.go`
- [X] T012 Populate one capability record per resolved runtime model/backend binding and wire it into route resolution without changing authentication or organization access behavior in `internal/proxy/handler/handler.go` and `internal/proxy/handler/capability.go`
- [X] T013 Add the shared stream aggregation state machine for content, refusal, tool calls, finish reason, IDs, and usage in `internal/proxy/handler/stream_aggregate.go`

**Checkpoint**: The shared matrix, validator, error shape, and stream
aggregator compile with focused unit coverage before story-specific behavior
is changed.

## Phase 3: User Story 1 - Standard Route and Client Neutrality (Priority: P1) 🎯 MVP

**Goal**: Any authenticated OpenAI-compatible client can use one `/v1` base
URL, exact model discovery, chat, and embeddings without naming itself.

**Independent Test**: List models, retrieve one exact model, issue equivalent
requests with different caller metadata, and confirm identical route and
capability decisions; confirm Graphiti/Cognee-specific routes do not exist.

### Tests for User Story 1 (write and run first)

- [X] T014 [P] [US1] Add list/retrieve consistency and exact `model_not_found` contract tests in `test/contract/models_test.go`
- [X] T015 [P] [US1] Add caller-neutral model/chat/embedding routing tests for Graphiti, Cognee, OpenAI SDK, LiteLLM, and anonymous metadata in `test/contract/openai_compatibility_test.go`
- [X] T016 [P] [US1] Add negative route inspection tests proving `/v1/graphiti/*` and `/v1/cognee/*` have no 2xx project-specific handler or fallback in `test/contract/routes_test.go`
- [X] T017 [US1] Run the new User Story 1 tests and record the expected failures before implementation in `specs/1465-openai-compatible-api/quickstart.md`

### Implementation for User Story 1

- [X] T018 [US1] Add `GET /v1/models/{model}` beside the existing list route in `internal/proxy/server.go`
- [X] T019 [US1] Implement exact visible-catalog lookup and standard `model_not_found` response in `internal/proxy/handler/handler.go`
- [X] T020 [US1] Reuse the same model-item builder for list and exact retrieval responses in `internal/proxy/handler/list_models_catalog.go`
- [X] T021 [US1] Ensure caller metadata never participates in route or capability selection, and never add or branch on `graphiti_params` or `cognee_params`, in `internal/proxy/handler/chat.go` and `internal/proxy/handler/embedding.go`
- [X] T022 [US1] Run User Story 1 contract tests and verify the five standard routes remain authenticated in `test/integration/error_format_test.go`

**Checkpoint**: Standard route/model discovery and caller neutrality work
independently, with no project-specific route or fallback.

## Phase 4: User Story 2 - Chat Streaming and Aggregation (Priority: P1)

**Goal**: `stream=false` returns one standard JSON completion and `stream=true`
returns standard SSE, including stream-only Codex aggregation.

**Independent Test**: Run equivalent requests against direct HTTP and
stream-only fixtures; validate content, tool calls, refusal, finish reason,
usage, chunk ordering, and `[DONE]`.

### Tests for User Story 2 (write and run first)

- [X] T023 [P] [US2] Add direct non-streaming Chat Completion JSON contract tests in `test/contract/chat_completion_test.go`
- [X] T024 [US2] Add direct streaming SSE contract tests for chunks, finish reason, usage, and `[DONE]` in `test/contract/chat_completion_test.go`
- [X] T025 [US2] Add stream-only backend aggregation tests for client `stream=false` and incremental forwarding for `stream=true` in `test/contract/chat_completion_test.go`
- [X] T026 [P] [US2] Add content/tool-call/refusal/usage aggregate tests in `internal/proxy/handler/chat_stream_test.go`
- [X] T027 [P] [US2] Add Codex usage-only terminal and incomplete/malformed stream tests in `internal/provider/chatgptcodex/stream_test.go`
- [X] T028 [US2] Run User Story 2 tests and confirm current non-stream Codex and incomplete-stream gaps in `specs/1465-openai-compatible-api/quickstart.md`

### Implementation for User Story 2

- [X] T029 [US2] Update chat dispatch to choose direct JSON, direct SSE, or stream aggregation from the resolved capability record in `internal/proxy/handler/chat.go`
- [X] T030 [US2] Use upstream `stream=true` plus the shared aggregator for client `stream=false` on stream-only transports in `internal/proxy/handler/chatgpt_codex_backend.go`
- [X] T031 [US2] Preserve incremental translated chunks and terminal usage for client `stream=true` in `internal/proxy/handler/chat.go` and `internal/proxy/handler/chatgpt_codex_backend.go`
- [X] T032 [US2] Extend Codex response/event translation for refusal, tool-call fragments, finish status, and usage without fabricating choices in `internal/provider/chatgptcodex/response.go`
- [X] T033 [US2] Preserve direct OpenAI stream request/response behavior and standard field forwarding in `internal/provider/openai/openai.go`
- [X] T034 [US2] Map malformed or incomplete upstream streams to safe client errors and failure callbacks without reporting a false successful completion in `internal/proxy/handler/chat.go`
- [X] T035 [US2] Run focused chat contract/provider tests and verify `stream=false` aggregation and `stream=true` latency behavior in `test/contract/chat_completion_test.go`

**Checkpoint**: Both stream modes work against both transport styles and
preserve supported completion semantics.

## Phase 5: User Story 3 - Capabilities and Structured Output (Priority: P1)

**Goal**: Declared chat parameters are translated only when their
backend/model capability is explicit; otherwise the client receives a
standard 400 error.

**Independent Test**: Send each declared capability to a capable fixture and
to an incapable Codex record; verify translation or an error with the correct
`param` and `code`.

### Tests for User Story 3 (write and run first)

- [X] T036 [P] [US3] Add table-driven capability tests for `response_format.type=json_object`, `response_format.type=json_schema`, `temperature`, `top_p`, `max_tokens`, `max_completion_tokens`, `tools`, `tool_choice`, and `stream_options.include_usage` in `test/contract/chat_capabilities_test.go`
- [X] T037 [US3] Add `unsupported_parameter`, `invalid_value`, `invalid_request`, and ambiguous-token-field error tests in `test/contract/chat_capabilities_test.go`
- [X] T038 [P] [US3] Add JSON object and JSON Schema contract tests with exact request preservation in `test/contract/structured_output_test.go`
- [X] T039 [P] [US3] Add verified Codex `text.format` payload mapping tests gated by the Phase 0 evidence result in `internal/provider/chatgptcodex/payload_test.go`
- [X] T040 [US3] Run User Story 3 tests and confirm unsupported fields do not reach the mock upstream in `specs/1465-openai-compatible-api/quickstart.md`

### Implementation for User Story 3

- [X] T041 [US3] Validate `response_format`, `max_tokens`, `max_completion_tokens`, `temperature`, `top_p`, `tools`, `tool_choice`, `stream_options.include_usage`, numeric values, and token-field combinations before provider I/O in `internal/proxy/handler/capability.go`
- [X] T042 [US3] Preserve standard direct OpenAI parameter names and map `max_completion_tokens` only when equivalent in `internal/provider/openai/openai.go` and `internal/provider/openai/params.go`
- [X] T043 [US3] Keep Codex tools, tool choice, structured output, and unverified sampling/token capabilities fail-closed with `param` and `code` in `internal/provider/chatgptcodex/payload.go`
- [X] T044 [US3] Add the Phase 0-approved Responses `text.format` translation without prompt fallback in `internal/provider/chatgptcodex/payload.go`
- [X] T045 [US3] Ensure capability decisions use only `(backend, model)` and remain independent of caller/project metadata in `internal/model/capability.go` and `internal/proxy/handler/capability.go`
- [X] T046 [US3] Run structured-output, capability, Codex payload, and standard error regressions in `test/contract/structured_output_test.go` and `test/integration/error_format_test.go`

**Checkpoint**: No declared field is silently removed; supported fields retain
semantics and unsupported fields fail before upstream execution.

## Phase 6: User Story 4 - Embeddings and Model Consistency (Priority: P1)

**Goal**: Standard embedding requests accept string/list input, return numeric
vectors with indexes and usage, and treat dimensions as model-specific.

**Independent Test**: Run string, list, float, invalid input, and dimensions
requests against capable and incapable embedding records.

### Tests for User Story 4 (write and run first)

- [X] T047 [P] [US4] Add string/list input, numeric/Base64 vector, float32-overflow rejection, index, usage, and `encoding_format=float|base64` contract tests in `test/contract/embedding_test.go`
- [X] T048 [US4] Add unsupported `dimensions`, empty input, mixed input, invalid `encoding_format`, and model-specific dimension tests in `test/contract/embedding_test.go`
- [X] T049 [P] [US4] Add provider response validation tests for numeric arrays, correct indexes, strict standard OpenAI usage presence, and documented total-only Jina/Voyage normalization in `internal/provider/openai/embedding_test.go`, `internal/provider/jina/jina_test.go`, and `internal/provider/voyage/voyage_test.go`
- [X] T050 [US4] Run User Story 4 tests and update the standard-route expectation for provider-specific encoding behavior in `specs/1465-openai-compatible-api/quickstart.md`

### Implementation for User Story 4

- [X] T051 [US4] Validate string/list-of-string input, non-empty lists, `encoding_format=float|base64`, gateway-owned Base64 translation, and dimensions capability before provider I/O in `internal/proxy/handler/embedding.go`
- [X] T052 [US4] Reuse the backend/model capability record for embedding support and dimensions without a global 1024 default in `internal/proxy/handler/capability.go`
- [X] T053 [US4] Preserve numeric embedding arrays, finite standard Base64 float32 representation, zero-based indexes, public model identifiers, and explicit public prompt/total usage while normalizing documented total-only provider protocols in `internal/model/response.go`, `internal/provider/openai/embedding.go`, `internal/provider/jina/jina.go`, `internal/provider/voyage/voyage.go`, and `internal/proxy/handler/embedding.go`
- [X] T054 [US4] Map provider embedding validation/upstream failures to the standard sanitized `invalid_request`, `invalid_value`, and `unsupported_parameter` error shape in `internal/proxy/handler/embedding.go` and `internal/proxy/handler/errors.go`
- [X] T055 [US4] Keep provider-level special transformations covered while making the public `/v1/embeddings` contract reject unsupported encoding/dimension requests explicitly in `internal/provider/jina/jina_test.go` and `test/contract/embedding_test.go`
- [X] T056 [US4] Run embedding, model-catalog, and existing provider embedding regressions in `test/contract/embedding_test.go` and `internal/provider/openai/embedding_test.go`

**Checkpoint**: Embeddings are client-neutral and model-specific; no
Graphiti/Cognee dimension or vector-store logic exists in Tianji.

## Phase 7: User Story 5 - Authenticated Real-Client Smoke (Priority: P2)

**Goal**: Verify real client serialization and workflows against the same
standard base URL, not only local fixtures.

**Independent Test**: Run each smoke test with synthetic inputs and record
routes, model, status, response shape, and errors.

### Tests for User Story 5 (write and run first)

- [X] T057 [US5] Add an authenticated Graphiti `OpenAIGenericClient` smoke entry point for extraction and embeddings in `test/e2e/openai_compatible_clients_test.go`
- [X] T058 [US5] Add an authenticated Cognee custom-endpoint smoke entry point for Instructor/schema extraction and embeddings in `test/e2e/openai_compatible_clients_test.go`
- [X] T059 [US5] Add OpenAI SDK and LiteLLM standard-route smoke entry points in `test/e2e/openai_compatible_clients_test.go`
- [X] T060 [US5] Run the smoke-test skeletons against a deployed or local authenticated endpoint and record route/shape evidence in `specs/1465-openai-compatible-api/quickstart.md`

### Implementation for User Story 5

- [X] T061 [US5] Implement environment-gated client smoke orchestration and synthetic fixtures without adding runtime client-specific behavior in `test/e2e/openai_compatible_clients_test.go`
- [X] T062 [US5] Assert that Graphiti and Cognee persistence remains client-owned and that Tianji receives only standard chat/embedding routes in `test/e2e/openai_compatible_clients_test.go`
- [X] T063 [US5] Assert that OpenAI SDK and LiteLLM use the same model, success, streaming, embedding, and error contract fixtures in `test/e2e/openai_compatible_clients_test.go`
- [X] T064 [US5] Record any unavailable client dependency or unverified Codex capability as an explicit smoke limitation, not a passing fallback, in `specs/1465-openai-compatible-api/research.md`

**Checkpoint**: Real clients use only the standard base URL and no project
specific Tianji route or parameter.

## Phase 8: Polish and Cross-Cutting Verification

**Purpose**: Finish documentation, preserve regressions, and run the complete
quality gates.

- [X] T065 [P] Update endpoint, matrix, error, and Phase 0 evidence documentation after implementation in `specs/1465-openai-compatible-api/contracts/openai-v1.md`
- [X] T066 [P] Update the capability example and verified/unverified decisions after implementation in `specs/1465-openai-compatible-api/contracts/capability-matrix.md`
- [X] T067 [P] Keep existing Responses route behavior and regression coverage green in `test/contract/responses_create_test.go` and `internal/proxy/handler/responses_codex_test.go`
- [X] T068 Run focused route/chat/capability/embedding/model/error tests with `rtk go test` and fix only feature-scope failures in `test/contract/`, `test/integration/`, `internal/provider/`, and `internal/proxy/handler/`
- [X] T069 Run repository quality gates `rtk make lint`, `rtk make test`, `rtk make build`, and `rtk make check` from `Makefile`
- [X] T070 Run every scenario in `specs/1465-openai-compatible-api/quickstart.md` and attach final evidence without modifying `graphify-out/`, `internal/graphify-out/`, or `test/graphify-out/`

## Dependencies and Execution Order

### Phase dependencies

- **Phase 0** has no code dependency and blocks any claim of Codex
  structured-output or unverified sampling support.
- **Phase 1** depends only on the existing repository and enables reusable
  test fixtures.
- **Phase 2** depends on Phase 1 and blocks all user-story implementation.
- **User Stories 1–4** depend on Phase 2. They are independently testable,
  although US2/US3 share the stream and capability helpers.
- **User Story 5** depends on the completed standard contract in US1–US4 and
  on an available authenticated client environment.
- **Phase 8** depends on all desired stories and the Phase 0 evidence result.

### User-story order

1. US1 — standard routes and model discovery (MVP)
2. US2 — chat JSON/SSE and stream-only aggregation
3. US3 — capability validation and structured output
4. US4 — embeddings and dimensions
5. US5 — real-client smoke

### Parallel opportunities

- T001–T002 can run in parallel; T003 waits for their evidence.
- T004–T006 can run in parallel.
- T007–T009 can run in parallel; T010–T013 depend on their types.
- Within each story, all initial test tasks marked `[P]` can be written in
  parallel because they use disjoint files.
- T014–T016, T023/T026/T027, T036/T038/T039, and T047/T049 are the main
  parallel test batches; same-file tasks remain sequential.
- T057–T059 are sequential edits to one smoke test file; T060 waits for the
  environment.
- T065–T067 can run in parallel after implementation.

## Requirement and Success-Criteria Coverage

| Requirement | Task IDs |
|---|---|
| FR-001 | T018, T022, T067 |
| FR-002 | T015, T022, T057, T059 |
| FR-003 | T015, T016, T021, T045, T062 |
| FR-004 | T012, T021, T045 |
| FR-005 | T014, T019, T020 |
| FR-006 | T023–T035 |
| FR-007 | T036, T041 |
| FR-008 | T007, T012, T036 |
| FR-009 | T045 |
| FR-010 | T041–T044 |
| FR-011 | T025, T030 |
| FR-012 | T026, T031, T032 |
| FR-013 | T036, T038, T043, T044 |
| FR-014 | T038, T039, T044 |
| FR-015 | T037, T039, T043 |
| FR-016 | T036, T037, T043, T046 |
| FR-017 | T047, T051, T053 |
| FR-018 | T047, T051 |
| FR-019 | T048, T052 |
| FR-020 | T053, T054 |
| FR-021 | T009, T011, T037, T054 |
| FR-022 | T011, T034, T054 |
| FR-023 | T033, T056, T067–T069 |
| FR-024 | T001–T003, T039, T043, T044, T064 |
| FR-025 | T014–T016, T023–T027, T036–T039, T047–T049 |
| FR-026 | T056, T067–T069 |
| FR-027 | T057–T064 |
| FR-028 | T062 |
| SC-001 | T014, T018–T022 |
| SC-002 | T036–T046 |
| SC-003 | T025–T035 |
| SC-004 | T023–T046 |
| SC-005 | T047–T056 |
| SC-006 | T048, T052 |
| SC-007 | T057, T058, T062 |
| SC-008 | T059, T063 |
| SC-009 | T016, T021 |
| SC-010 | T056, T067–T069 |
| SC-011 | T001–T003, T039, T064 |
| Constitution IV and VIII evidence | T017, T028, T040, T050, T060, T068–T070 |

## Implementation Strategy

### MVP first

Complete Phase 0, Phase 1, Phase 2, and US1. Stop and validate the standard
routes/model discovery before enabling any advanced capability.

### Incremental delivery

1. Add US2 with stream/non-stream contract parity.
2. Add US3 with conservative capability errors and only Phase 0-approved
   structured output.
3. Add US4 with model-specific embedding validation.
4. Run US5 real-client smoke.
5. Finish Phase 8 quality gates and evidence.

No task adds a Graphiti/Cognee route, project-specific request field, client
fallback, global embedding dimension, database schema, or new dependency.
