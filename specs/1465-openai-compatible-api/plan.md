# Implementation Plan: Standard OpenAI-Compatible API

**Branch**: `1465-openai-compatible-api` | **Date**: 2026-08-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/1465-openai-compatible-api/spec.md`

## Summary

Tianji already exposes most of the standard `/v1` routes, authentication
middleware, provider transformations, streaming handlers, and model catalog
logic. This feature completes the common OpenAI contract without adding
Graphiti- or Cognee-specific routes. The implementation will:

1. add exact model retrieval at `GET /v1/models/{model}`;
2. introduce a conservative capability matrix keyed by
   `(backend, model)`;
3. validate declared chat and embedding parameters before upstream execution,
   including standard float/Base64 embedding representation;
4. preserve standard JSON, SSE, tool-call, refusal, finish-reason, and usage
   semantics;
5. aggregate a stream-only backend when the client requests `stream=false`;
6. map provider failures to sanitized standard errors; and
7. retain the existing Responses, direct OpenAI HTTP, Codex, embedding, and
   model-catalog behavior through regression tests.

The Codex structured-output path is deliberately gated. Phase 0 records
authenticated upstream evidence for `text.format`, JSON object/schema behavior,
stream aggregation, and sampling/token semantics. Until that evidence exists,
the Codex capability record remains false and requests fail closed with
`unsupported_parameter`.

## Technical Context

**Language/Version**: Go 1.26.0 from `go.mod`

**Primary Dependencies**: Existing `chi/v5` router, provider interfaces in
`internal/provider`, `testify`, `httptest`, the current runtime model catalog,
and Go standard-library Base64/binary/float conversion. No new dependency.

**Storage**: None for this feature. Reuse the existing configuration/runtime
model list, authentication, organization/model access, callbacks, cache, and
provider credential paths. No schema or sqlc change.

**Testing**: Focused Go unit/contract/integration tests, existing provider and
handler tests, `go test -race -cover ./...`, and authenticated client smoke
tests documented in [quickstart.md](./quickstart.md).

**Target Platform**: Linux HTTP service; local `httptest` and authenticated
deployed endpoint validation.

**Project Type**: Go HTTP gateway/web service.

**Performance Goals**:

- `stream=true` must forward translated chunks as they arrive; it must not
  buffer the complete answer.
- `stream=false` on a stream-only backend may buffer only the upstream stream
  required to construct one JSON response.
- Capability validation must happen before provider I/O.
- No new persistence, retry loop, or background worker is introduced.

**Constraints**:

- Use only standard `/v1` routes; do not add `/v1/graphiti/*` or
  `/v1/cognee/*`, `graphiti_params`, or `cognee_params`.
- Do not branch on caller, project, or client-library names.
- Never silently drop a declared request parameter.
- Fail closed when structured-output or another capability is not proven.
- Preserve existing authentication, organization/model access, redaction, and
  cancellation behavior.
- Keep `direct_openai_http` and `chatgpt_codex_backend` as the initial backend
  identities; do not create a generic transport abstraction with no second
  implementation.

**Scale/Scope**: Existing Tianji model catalog and the two named backend
transports. This design does not add a database-backed capability service or
client-owned graph/vector storage.

## Constitution Check

*GATE: Must pass before Phase 0 research and be re-checked after Phase 1
design.*

- **I. Python-First Reference**: This is a new standards-first surface, not a
  Python parity exception. Existing Python-compatible behavior is preserved
  unless this spec explicitly requires standard error/capability behavior.
- **II. Feature Parity**: Existing `/v1/responses`, direct OpenAI HTTP, Codex,
  embedding, and model-catalog routes remain covered by regression tests.
- **III. Evidence-Based Research Before Build**: Current OpenAI/LiteLLM
  contract documentation, repository evidence, and the mandatory authenticated
  Codex Phase 0 probe are recorded in `research.md`.
- **IV. Failing-Tests-First Development**: The plan includes concrete failing
  tests below. The first implementation task in every user-story phase writes
  and runs those tests before the behavior change.
- **V. Idiomatic Go & Simplicity**: Reuse existing route registration,
  `Provider`/`EmbeddingProvider`, model types, stream scanners, and error
  writers. No new dependency, database table, or speculative interface.
- **VI. Verified, Current Decisions**: Repository facts, documentation facts,
  authenticated upstream evidence, and unverified assumptions are separated in
  `research.md`; Phase 0 evidence is required before claiming Codex support.
- **VII. SQL-First Database Access**: No database behavior changes.
- **VIII. Standards-First Compatibility**: The public contract is one
  authenticated OpenAI-compatible `/v1` surface and uses capability-based
  validation rather than project-specific routes.
- **IX. Secure-by-Default Operations**: Existing auth/model access middleware
  remains on all standard routes; upstream errors are sanitized and secrets
  are not returned.

**Gate result**: PASS. No constitution exception is required.

## Failing Tests

These tests are the required red-green starting point. They describe the
expected failures against the current repository (notably, exact model
retrieval is absent, Codex non-stream aggregation is incomplete, capability
validation is provider-level, and several response fields are not retained).

### User Story 1 — standard route and client neutrality

- `test/contract/models_test.go:TestListModelsAndRetrieveModel_UseSameVisibleCatalog`
  — expected to fail because `GET /v1/models/{model}` is not registered; after
  implementation both responses must expose the same model ID and availability.
- `test/contract/models_test.go:TestRetrieveModel_UnknownModelReturnsStandardNotFound`
  — expected to fail until exact lookup returns HTTP 404 with
  `type=invalid_request_error` and `code=model_not_found`.
- `test/contract/openai_compatibility_test.go:TestStandardClientRoutes_DoNotDependOnCallerIdentity`
  — expected to fail until the fixture can send identical requests with
  Graphiti/Cognee/OpenAI/LiteLLM metadata and observe identical routing and
  validation.
- `test/contract/routes_test.go:TestGraphitiAndCogneeSpecificRoutesAreAbsent`
  — expected to fail if a project-specific route is added; it must pass with
  no 2xx project-specific handler or fallback (the existing generic
  not-implemented catch-all may remain).

### User Story 2 — chat streaming and aggregation

- `test/contract/chat_completion_test.go:TestChatCompletion_NonStreamingJSON`
  — expected to fail until the fixture asserts the complete standard
  `chat.completion` object rather than accepting only 200/502.
- `test/contract/chat_completion_test.go:TestChatCompletion_StreamingSSE`
  — expected to fail until chunks, content, finish reason, and `[DONE]` are
  validated as standard SSE.
- `test/contract/chat_completion_test.go:TestChatCompletion_StreamOnlyBackendAggregatesToJSON`
  — expected to fail because a stream-only Codex route currently chooses the
  non-stream transport path without a common aggregation result.
- `test/contract/chat_completion_test.go:TestChatCompletion_StreamOnlyBackendForwardsChunks`
  — expected to fail until `stream=true` preserves incremental output and
  terminal usage without buffering.
- `internal/proxy/handler/chat_stream_test.go:TestAggregateStream_PreservesToolCallsRefusalFinishReasonAndUsage`
  — expected to fail because the current response model/aggregator does not
  preserve all listed fields.
- `internal/provider/chatgptcodex/stream_test.go:TestTransformStreamChunk_PreservesUsageOnlyTerminalEvent`
  — expected to fail until a usage-only terminal event does not fabricate a
  choice or content.

### User Story 3 — capabilities and structured output

- `test/contract/chat_capabilities_test.go:TestChatCapabilityMatrix_CoversDeclaredParameters`
  — expected to fail until every declared parameter is checked against the
  `(backend, model)` record before upstream execution.
- `test/contract/chat_capabilities_test.go:TestUnsupportedChatParameter_ReturnsStandardError`
  — expected to fail because unsupported fields currently reach provider
  validation or are forwarded without `param` and `code`.
- `test/contract/chat_capabilities_test.go:TestBothTokenLimitFieldsRejectAmbiguousBackend`
  — expected to fail until both token fields are rejected when the backend
  cannot represent both unambiguously.
- `test/contract/structured_output_test.go:TestChatJSONSchema_TranslatesToResponsesTextFormat`
  — expected to remain blocked by the Phase 0 gate until an upstream probe
  proves the conversion; once enabled it must preserve name, schema, and
  strictness exactly.
- `test/contract/structured_output_test.go:TestCodexStructuredOutputFailsClosedWithoutEvidence`
  — expected to fail until Codex returns HTTP 400 with
  `param=response_format` and `code=unsupported_parameter`.
- `internal/provider/chatgptcodex/payload_test.go:TestBuildPayload_MapsVerifiedJSONSchema`
  — expected to fail until the verified `text.format` payload is emitted;
  it must not be enabled by serialization alone.

### User Story 4 — embeddings and model consistency

- `test/contract/embedding_test.go:TestEmbedding_StringInputReturnsNumericVectorAndUsage`
  — expected to fail until the fixture asserts numeric values, index 0, and
  prompt/total usage.
- `test/contract/embedding_test.go:TestEmbedding_ListInputPreservesZeroBasedIndexes`
  — expected to fail until one response item is asserted per input.
- `test/contract/embedding_test.go:TestEmbedding_Base64RequestUsesFloatUpstreamAndEncodesResponse`
  — expected to fail until the public Base64 representation is produced from a
  validated numeric upstream vector as little-endian float32 bytes.
- `test/contract/embedding_test.go:TestEmbedding_Base64RejectsFloat32Overflow`
  — expected to fail until values that become non-finite during float32
  conversion are rejected before Base64 output.
- `internal/provider/openai/embedding_test.go:TestTransformEmbeddingResponse_RejectsInvalidShape`
  — expected to fail for standard OpenAI-compatible upstream responses that
  omit `usage`, `usage.prompt_tokens`, or `usage.total_tokens`.
- `internal/provider/jina/` and `internal/provider/voyage/` response tests
  — expected to fail until each first-class adapter normalizes its documented
  total-only embedding usage to the standard public fields.
- `test/contract/embedding_test.go:TestEmbedding_DimensionsRequiresExplicitCapability`
  — expected to fail until unsupported dimensions return a standard 400 rather
  than being forwarded or ignored.
- `test/contract/embedding_test.go:TestEmbedding_DoesNotAssumeGlobal1024Dimension`
  — expected to fail until fixtures use model-specific capability records and
  accept different vector lengths.
- `test/contract/models_test.go:TestModelRetrieveMatchesEmbeddingModelIdentifier`
  — expected to fail until model discovery and embedding responses use the same
  public identifier.

### User Story 5 — real client smoke

- `test/e2e/openai_compatible_clients_test.go:TestGraphitiOpenAIGenericClientSmoke`
  — expected to fail until entity/relationship extraction and embeddings run
  only through the standard base URL.
- `test/e2e/openai_compatible_clients_test.go:TestCogneeCustomEndpointSmoke`
  — expected to fail until Instructor/schema extraction, embedding ingestion,
  and graph construction use only standard routes.
- `test/e2e/openai_compatible_clients_test.go:TestOpenAISDKAndLiteLLMSmoke`
  — expected to fail until both general clients exercise the same fixtures and
  standard error behavior.

## Project Structure

### Documentation (this feature)

```text
specs/1465-openai-compatible-api/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── openai-v1.md
│   └── capability-matrix.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/
├── model/
│   ├── request.go                 # chat/embedding request validation inputs
│   ├── response.go                # refusal/tool-call/usage response fields
│   ├── errors.go                  # standard error classification
│   └── capability.go              # backend+model capability record/matrix
├── provider/
│   ├── provider.go                # reuse existing transport boundary
│   ├── openai/
│   │   ├── openai.go              # direct chat translation
│   │   └── embedding.go           # direct embedding translation
│   └── chatgptcodex/
│       ├── payload.go             # verified Responses payload translation
│       ├── response.go            # response/event/refusal/tool/usage mapping
│       └── transport.go           # upstream request mode
└── proxy/
    ├── server.go                  # standard route registration
    └── handler/
        ├── handler.go              # exact model lookup and route binding
        ├── capability.go          # validation and matrix resolution
        ├── chat.go                # common chat dispatch/aggregation
        ├── chatgpt_codex_backend.go # Codex stream/non-stream execution
        ├── embedding.go           # embedding validation and response mapping
        └── errors.go               # safe standard error writer/mapping

test/
├── contract/
│   ├── chat_completion_test.go
│   ├── chat_capabilities_test.go
│   ├── structured_output_test.go
│   ├── embedding_test.go
│   ├── models_test.go
│   ├── routes_test.go
│   ├── helpers_test.go
│   └── assertions_test.go
├── integration/
│   └── error_format_test.go
├── fixtures/
│   └── openai/
│       └── codex_capabilities/
│           └── responses_events.json
└── e2e/
    ├── codex_capability_probe_test.go
    └── openai_compatible_clients_test.go

testutil/openaitest/
└── upstream_server_test.go
```

**Structure Decision**: Keep the existing single Go service and existing
provider/handler boundaries. Put the generic capability record in
`internal/model` so both provider and handler packages can use it without a
package cycle. Keep request validation and route lookup in the handler layer,
where authentication and the resolved backend/model are already available.
Do not add a database, queue, new transport interface, or client-specific
package.

## Phase 0 — Upstream Capability Evidence

Phase 0 is a hard gate and makes no claim of Codex feature support by
serialization alone.

1. Use an authenticated, redacted probe against the configured Codex upstream
   to test `text.format` with `json_object` and `json_schema`.
2. Record request acceptance, response status, output validity, schema
   adherence, and refusal behavior in `research.md`; never commit tokens,
   cookies, raw authorization headers, or secret-bearing response bodies.
3. Probe both streamed and non-streamed Responses events, including content,
   tool-call fragments, finish/completion status, usage-only terminal events,
   and refusal/error events.
4. Probe `max_output_tokens`/the Chat Completions token aliases,
   `temperature`, and `top_p` separately; distinguish accepted-but-ignored
   behavior from equivalent semantics.
5. If any required behavior is unavailable or unverifiable, set the
   corresponding Codex capability record to false and keep the standard
   unsupported-parameter response. Do not add a Graphiti/Cognee fallback.

The probe may reuse `internal/provider/chatgptcodex` payload/stream helpers and
the existing `testutil/openaitest` server for deterministic fixtures, but live
upstream evidence must be labeled separately from local mock evidence.

## Phase 1 — Common API Contract

### Route and model discovery

- Register `GET /v1/models/{model}` beside the existing `/v1/models` route.
- Reuse the existing visible runtime model list and access filtering.
- Exact lookup must not invoke general fallback or wildcard resolution.
- Return the same public model identifier as the list endpoint, or a sanitized
  `model_not_found` error.

### Capability matrix and request validation

- Add a typed `CapabilityRecord` and `CapabilityMatrix` keyed by backend plus
  model. Required booleans are the fields in FR-008; add explicit
  `supports_dimensions` and an optional allowed-dimensions list for embedding
  validation.
- Build one matrix entry for every visible runtime model binding. Direct
  OpenAI HTTP infers only verified first-class provider semantics and exact
  preserved parameters, plus whether the provider implements
  `EmbeddingProvider`; Codex starts from the conservative Phase 0 evidence.
  Unverified or model-specific bits remain false unless an explicit
  backend/model record enables them. Tests inject capable and incapable
  records without adding a caller/project dimension.
- Resolve backend identity from the existing
  `direct_openai_http`/`chatgpt_codex_backend` transport selection, never from
  caller metadata.
- Use `Provider.GetSupportedParams()` as transport evidence only. Structured
  output modes, `stream_options.include_usage`, and embedding dimensions need
  explicit capability bits.
- Validate the resolved request after model/backend selection and before
  guardrail/upstream side effects. Return one standard 400 error with
  `param` and `code=unsupported_parameter` for a known but unsupported field.
- Validate malformed values (`temperature`, `top_p`, token limits,
  `response_format`, `stream_options`, input type/encoding) with
  `code=invalid_value` or `invalid_request`.
- Reject ambiguous simultaneous `max_tokens` and `max_completion_tokens` when
  the selected backend cannot represent both.

### Chat translation and stream semantics

- Preserve the existing direct OpenAI request/response path for capabilities
  that are directly represented.
- Keep the Codex payload conservative. Map `max_completion_tokens` to
  `max_output_tokens` only where the capability record says it is equivalent.
- Add one common aggregation result for stream-only backends. It must merge
  content, ordered tool-call fragments, refusal, finish reason, model/id, and
  the latest complete usage record.
- For client `stream=true`, emit standard `chat.completion.chunk` SSE events
  and `[DONE]`; for `stream=false` on a stream-only backend, buffer only the
  translated stream and emit one `chat.completion` JSON object.
- A usage-only terminal event must update usage without creating a fake choice.
- Malformed/incomplete streams must produce a safe error and must not be
  reported as a successful completed response.

### Structured output

- Parse the Chat Completions `response_format` object without losing its raw
  schema.
- Direct OpenAI HTTP forwards the standard request shape only when the
  capability record says the selected model supports that mode.
- Codex maps Chat Completions JSON Schema to Responses `text.format` only after
  Phase 0 evidence. Until then, `json_object` and `json_schema` are both
  rejected with `param=response_format`.
- Never replace a rejected structured-output request with a prompt
  instruction or a client-specific fallback.

### Embeddings and errors

- Validate string/list-of-string input, non-empty list, the supported
  `encoding_format=float|base64` values, and optional dimensions before the
  provider call.
- Reuse `EmbeddingProvider` and existing provider-specific transformations.
- Keep validated numeric vectors as the internal form. Return them directly
  for float output; for Base64 output, encode little-endian float32 bytes at
  the Tianji boundary and reject values that overflow to non-finite float32.
  Preserve zero-based indexes and require both prompt/total fields in the
  public response. Keep raw standard OpenAI-compatible responses strict;
  first-class adapters for documented total-only embedding protocols may set
  `prompt_tokens=total_tokens` before shared validation.
- Do not turn a runtime model dimension (for example 1024) into a global
  constant.
- Centralize safe standard error writing for `unsupported_parameter`,
  `invalid_value`, `invalid_request`, and `model_not_found`. Do not expose raw
  upstream URLs, credentials, or secret-bearing bodies.

## Phase 2 — Contract and Regression Tests

Add the failing tests listed above, then implement the smallest fixtures needed
to run them against:

- a direct OpenAI-compatible `httptest` upstream;
- a stream-only Codex-like SSE upstream;
- a capable and an incapable capability record;
- string/list float and Base64 embedding requests;
- exact model list/retrieve paths; and
- sanitized upstream error bodies.

Keep all existing tests in:

- `internal/provider/openai/*_test.go`;
- `internal/provider/chatgptcodex/*_test.go`;
- `internal/proxy/handler/chat_stream_test.go`;
- `internal/proxy/handler/list_models_codex_test.go`;
- `test/contract/responses_create_test.go`;
- `test/contract/embedding_test.go`;
- `test/integration/error_format_test.go`; and
- `test/integration/full_flow_test.go`.

Run focused contract tests first, then `rtk go test -race -cover ./...`.

## Phase 3 — Authenticated Client Smoke

Run the same local or deployed authenticated Tianji base URL through:

- Graphiti `OpenAIGenericClient` for entity extraction, relationship
  extraction, structured output, and embeddings;
- Cognee custom LLM/embedding endpoint for Instructor extraction, JSON Schema,
  embedding ingestion, and graph construction;
- the general OpenAI SDK for models, chat, structured output, and embeddings;
  and
- LiteLLM/OpenAI-compatible client for chat and embeddings.

Capture the route, model, status, response shape, and error code for each
operation. Use synthetic data and client-owned persistence only. A smoke test
does not enable a capability that Phase 0 did not verify.

## Constitution Check — Post-Design

- Standards-first route and payload contract is defined in `contracts/`.
- Every user-facing story has failing tests and a corresponding task phase.
- The Phase 0 evidence gate is explicit and prevents unsupported Codex claims.
- The matrix is keyed only by backend/model; no project-specific behavior is
  represented.
- No SQL, schema, dependency, or secret-handling exception is introduced.

**Post-design gate result**: PASS.

## Complexity Tracking

No constitution violations or complexity exceptions are required.

| Decision | Why it is necessary | Simpler alternative rejected because |
|---|---|---|
| Typed backend/model capability record | One validation point must distinguish transport support from structured-output and dimension support | Provider-wide `GetSupportedParams()` cannot express model-specific or mode-specific support |
| Stream aggregation helper | Codex can be stream-only while clients can request JSON | Returning SSE for `stream=false` violates the standard contract |
