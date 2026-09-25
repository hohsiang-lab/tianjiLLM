# Feature Specification: Standard OpenAI-Compatible API

**Feature Branch**: `1465-openai-compatible-api`

**Created**: 2026-08-07

**Status**: Draft

**Input**: User description: "Expose one standard OpenAI-compatible `/v1` contract for Graphiti, Cognee, the OpenAI SDK, and LiteLLM. Use model/backend capabilities for validation and translation, support chat completions, embeddings, models, and responses, and do not add project-specific Graphiti or Cognee routes."

## Scope and Boundaries

This feature makes Tianji a general OpenAI-compatible gateway for any client
that can use a base URL and API key:

```text
base_url=https://<tianji-host>/v1
api_key=<key>
```

The public contract covers:

- `GET /v1/models`
- `GET /v1/models/{model}`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- `POST /v1/responses`

The primary integration paths are `/v1/chat/completions` and
`/v1/embeddings`. Request behavior is selected from the requested model, its
configured backend, and the backend/model capability record. Tianji MUST NOT
select behavior from the caller name, project name, or client library.

The feature does not add `/v1/graphiti/*` or `/v1/cognee/*`, client-specific
request fields, graph or vector database behavior, or hard-coded embedding
dimensions.

## Validation Order

The work MUST proceed in this order:

1. **Phase 0 - Upstream capability evidence**: without changing application
   behavior, verify Codex upstream acceptance and semantics for `text.format`,
   stable JSON-object output, JSON Schema adherence, stream aggregation of
   content/tool calls/finish reason/usage/refusal, and the actual support for
   token limits, temperature, and top-p.
2. **Phase 1 - Common API contract**: implement only the general `/v1`
   validation, translation, capability decisions, standard errors, model
   consistency, and embedding behavior described here.
3. **Phase 2 - Contract tests**: add the shared route, request, response,
   streaming, structured-output, embedding, model, and error fixtures while
   retaining existing direct HTTP, Codex backend, Responses, embedding, and
   model-catalog tests.
4. **Phase 3 - Client smoke tests**: run authenticated Graphiti,
   Cognee, OpenAI SDK, and LiteLLM workflows against the same standard base
   URL. A failed or unverifiable Phase 0 capability remains unsupported; it is
   not replaced with a client-specific fallback.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use Tianji as a Standard OpenAI Endpoint (Priority: P1)

Graphiti, Cognee, an OpenAI SDK client, or LiteLLM configures only the Tianji
`base_url` and API key, then uses the standard `/v1` routes without knowing
which backend serves the selected model.

**Why this priority**: A single protocol boundary prevents integration-specific
fallbacks and allows future OpenAI-compatible clients to use Tianji without
new routes.

**Independent Test**: Configure a standard client with the Tianji base URL and
API key, call `/v1/models`, then make one chat and one embedding request
without any Graphiti- or Cognee-specific setting.

**Acceptance Scenarios**:

1. **Given** a valid API key and a configured model, **When** a standard client
   calls `GET /v1/models`, **Then** the response contains the model in the
   standard model-list shape.
2. **Given** a valid API key and a listed model, **When** a client calls
   `GET /v1/models/{model}`, **Then** the response identifies the same model
   and has the same availability result as the model list.
3. **Given** Graphiti or Cognee configured with `base_url=<tianji>/v1`,
   **When** it performs extraction and embedding operations, **Then** requests
   use only the standard chat-completion and embedding routes.
4. **Given** an OpenAI SDK or LiteLLM client with the same base URL, **When** it
   sends a supported request, **Then** Tianji does not require caller-specific
   headers, parameters, or route names.
5. **Given** any caller name or project label in metadata, **When** the same
   model and backend receive the request, **Then** routing and capability
   validation are unchanged.

### User Story 2 - Complete Chat Requests with Explicit Streaming Semantics (Priority: P1)

A client sends a standard chat-completion request and receives either one
OpenAI-compatible JSON response or OpenAI-compatible SSE chunks according to
`stream`.

**Why this priority**: Chat completions are the main integration path for
Graphiti, Cognee, and general OpenAI-compatible clients.

**Independent Test**: Send equivalent requests with `stream=false` and
`stream=true` to a direct HTTP backend and a stream-only Codex backend, then
validate the returned objects, content, finish reason, and usage.

**Acceptance Scenarios**:

1. **Given** a backend that supports non-streaming chat, **When** the client
   sends `stream=false`, **Then** Tianji returns one valid Chat Completion JSON
   object.
2. **Given** a backend that supports streaming chat, **When** the client sends
   `stream=true`, **Then** Tianji returns OpenAI-compatible SSE chunks followed
   by the standard completion terminator.
3. **Given** a stream-only backend, **When** the client sends `stream=false`,
   **Then** Tianji requests a stream upstream, aggregates it, and returns one
   valid Chat Completion JSON object.
4. **Given** a stream-only backend, **When** the client sends `stream=true`,
   **Then** Tianji translates upstream events into standard Chat Completion SSE
   chunks without buffering the complete answer before sending it.
5. **Given** a stream containing content, tool-call fragments, finish reason,
   usage, or refusal information, **When** Tianji translates or aggregates it,
   **Then** each supported piece is preserved in the corresponding standard
   response field.

### User Story 3 - Request Structured Output and Advanced Chat Capabilities (Priority: P1)

A client can request JSON output, JSON Schema output, tools, tool choice, token
limits, sampling controls, and streaming usage only when the selected
backend/model has explicitly declared equivalent capabilities.

**Why this priority**: Structured extraction is required by common Graphiti and
Cognee workflows; silently dropping a constraint can produce invalid graph
data while appearing successful.

**Independent Test**: Run one contract test per request capability against a
capable backend and one unsupported-capability test against a Codex model that
does not advertise the capability.

**Acceptance Scenarios**:

1. **Given** a backend/model that supports `response_format.type=json_object`,
   **When** the client sends that request, **Then** Tianji translates it and
   returns valid JSON content or a standard structured-output response.
2. **Given** a backend/model that supports `response_format.type=json_schema`,
   **When** the client sends a schema with name, schema, and strictness,
   **Then** Tianji preserves the schema semantics and the result conforms to
   the requested schema.
   The canonical conversion under test is:

   ```json
   {
     "response_format": {
       "type": "json_schema",
       "json_schema": {
         "name": "entity",
         "schema": {},
         "strict": true
       }
     }
   }
   ```

   to the equivalent upstream Responses shape:

   ```json
   {
     "text": {
       "format": {
         "type": "json_schema",
         "name": "entity",
         "schema": {},
         "strict": true
       }
     }
   }
   ```
3. **Given** the Codex upstream has not been proven to accept the equivalent
   Responses API `text.format` request, **When** a client requests structured
   output through that backend, **Then** the capability is reported as
   unsupported and the request fails before upstream execution.
4. **Given** a request contains `tools` or `tool_choice`, **When** the selected
   capability record does not support the requested feature, **Then** Tianji
   returns a standard unsupported-parameter error and does not remove the
   field silently.
5. **Given** a request contains `temperature`, `top_p`, `max_tokens`,
   `max_completion_tokens`, or `stream_options.include_usage`, **When** the
   selected backend/model does not support that field, **Then** Tianji returns
   a standard unsupported-parameter error naming the parameter.
6. **Given** both `max_tokens` and `max_completion_tokens` are supplied and
   the backend cannot represent both unambiguously, **When** validation runs,
   **Then** Tianji rejects the request as invalid rather than choosing one
   silently.

### User Story 4 - Use Embeddings and Model Discovery Consistently (Priority: P1)

A client sends one string or a list of strings to `/v1/embeddings`, receives
the requested standard float or Base64 embedding representation with correct
indexes and usage, and can discover the same model through either model
endpoint.

**Why this priority**: Graph construction and vector ingestion depend on
embeddings independently of chat extraction.

**Independent Test**: Send string and list inputs with
`encoding_format=float`, validate every returned data item and usage field,
repeat with `encoding_format=base64` and decode the little-endian float32
bytes, then repeat with a model whose backend does not support requested
dimensions.

**Acceptance Scenarios**:

1. **Given** a model with embedding capability, **When** `input` is a string,
   **Then** `/v1/embeddings` returns a standard list response containing a
   numeric embedding array and `index: 0`.
2. **Given** a model with embedding capability, **When** `input` is a list of
   strings, **Then** the response contains one embedding per input with
   correct zero-based indexes.
3. **Given** `encoding_format=float`, **When** the backend returns a supported
   result, **Then** every `data[].embedding` value is a numeric array and
   the public response includes `usage.prompt_tokens` and
   `usage.total_tokens`.
4. **Given** `encoding_format=base64`, **When** the backend returns validated
   numeric vectors, **Then** Tianji returns each embedding as the Base64
   encoding of its finite little-endian float32 bytes without requiring
   native backend Base64 support.
5. **Given** a client requests `dimensions`, **When** the selected backend/model
   does not explicitly support dimensions, **Then** Tianji returns a standard
   unsupported-parameter error instead of forwarding or ignoring it.
6. **Given** a model currently produces 1024-dimensional vectors, **When** the
   model configuration or backend changes, **Then** Tianji does not apply 1024
   as a global API rule or as a Graphiti/Cognee-specific default.

### User Story 5 - Verify Real Client Compatibility Before Declaring Completion (Priority: P2)

Operators can verify that the standard contract works for real client
workflows, not only isolated handler tests.

**Why this priority**: A proxy can pass unit tests while still failing client
serialization, structured extraction, or embedding ingestion.

**Independent Test**: Run client smoke tests against a deployed or local
authenticated Tianji endpoint using synthetic inputs and record the request
route, response shape, and completion result.

**Acceptance Scenarios**:

1. **Given** Graphiti's `OpenAIGenericClient`, **When** it performs entity
   extraction, relationship extraction, and embedding calls, **Then** all
   calls use standard Tianji routes and complete successfully.
2. **Given** Cognee configured with `LLM_PROVIDER=custom`,
   `LLM_ENDPOINT=<tianji>/v1`, and `EMBEDDING_ENDPOINT=<tianji>/v1`,
   **When** it performs Instructor structured extraction, JSON Schema mode,
   embedding ingestion, and graph construction, **Then** no Tianji-specific
   route or parameter is required.
3. **Given** a general OpenAI SDK client and a LiteLLM client, **When** they
   exercise the supported `/v1` operations, **Then** their behavior matches
   the same contract fixtures used for Graphiti and Cognee.

### Edge Cases

- A model is absent from the configured catalog or an exact model item is
  requested for an unavailable model; return `model_not_found` without
  attempting an unrelated fallback.
- The API key is missing or invalid; use the existing authenticated API
  boundary and do not expose backend credentials.
- A request uses a negative, malformed, or otherwise invalid value for a
  supported parameter; return `invalid_value` with the relevant `param`.
- A request contains a recognized parameter that the selected backend/model
  cannot support; return HTTP 400 with `type: invalid_request_error`,
  `code: unsupported_parameter`, and the parameter name.
- A stream ends after content but before a finish event, contains malformed
  data, or reports a refusal; do not turn incomplete upstream data into a
  false successful completion.
- A stream-only backend returns an empty choices list for a usage-only final
  event; aggregate usage without inventing content or a tool call.
- A backend returns tool calls in multiple stream fragments; preserve order,
  indexes, IDs, function names, and arguments.
- Structured output is requested for a backend where `json_object` or
  `json_schema` has not been verified; fail closed rather than using prompt
  instructions as a substitute.
- An embedding request contains an empty input list, mixed non-string values,
  an `encoding_format` other than the supported `float` or `base64`, or
  unsupported dimensions; return a standard invalid request without a
  provider-specific error body.
- A standard OpenAI-compatible embedding backend omits `usage`,
  `usage.prompt_tokens`, or `usage.total_tokens`; reject the malformed result
  instead of treating an omitted field as zero. A first-class provider whose
  documented embedding protocol supplies only `total_tokens` may normalize
  `prompt_tokens=total_tokens` in its provider adapter before shared
  validation.
- An embedding backend returns a value that becomes non-finite when converted
  to float32 for Base64 output; reject the malformed upstream result instead
  of encoding infinity.
- An upstream service returns an internal error or secret-bearing detail;
  map it to a safe standard Tianji error and do not expose raw upstream
  credentials, URLs, or implementation details.
- A caller identifies itself as Graphiti, Cognee, OpenViking, or another
  project; the request follows the same model/backend capability path and
  never a project-specific fallback.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tianji MUST provide the standard public routes
  `GET /v1/models`, `GET /v1/models/{model}`, `POST /v1/chat/completions`,
  `POST /v1/embeddings`, and `POST /v1/responses` under one authenticated
  OpenAI-compatible contract.
- **FR-002**: Tianji MUST accept the client configuration
  `base_url=https://<tianji-host>/v1` and `api_key=<key>` without requiring
  the client to identify itself as Graphiti, Cognee, OpenAI SDK, or LiteLLM.
- **FR-003**: Tianji MUST NOT add `/v1/graphiti/*`, `/v1/cognee/*`,
  `graphiti_params`, `cognee_params`, or behavior selected by client/project
  name for this feature.
- **FR-004**: Route selection and request validation MUST use the requested
  model, selected backend, and supported capability record only.
- **FR-005**: `GET /v1/models` and `GET /v1/models/{model}` MUST expose
  consistent model identifiers and availability results; an unavailable exact
  model MUST use the standard model-not-found error behavior.
- **FR-006**: `/v1/chat/completions` MUST support `stream=false` as a standard
  Chat Completion JSON response and `stream=true` as standard OpenAI SSE
  chunks. Omitting `stream` MUST use the non-streaming behavior.
- **FR-007**: Chat request validation MUST explicitly handle
  `response_format`, `max_tokens`, `max_completion_tokens`, `temperature`,
  `top_p`, `tools`, `tool_choice`, and `stream_options.include_usage`.
- **FR-008**: The capability record for each backend/model pair MUST include,
  at minimum:
  `supports_stream`, `supports_non_stream`, `supports_response_format`,
  `supports_json_object`, `supports_json_schema`, `supports_tools`,
  `supports_tool_choice`, `supports_temperature`, `supports_top_p`,
  `supports_max_tokens`, `supports_max_completion_tokens`,
  `supports_stream_options_include_usage`, and `supports_embeddings`.
- **FR-009**: Capability records MUST be keyed by backend plus model, not by
  Graphiti, Cognee, OpenViking, or any other caller/project name.
- **FR-010**: A request parameter in the declared contract MUST either be
  translated with equivalent semantics or be rejected before upstream
  execution. Tianji MUST NOT silently drop an unsupported declared parameter.
- **FR-011**: For a stream-only backend receiving `stream=false`, Tianji MUST
  request the upstream stream, aggregate the events, and return a standard
  non-streaming Chat Completion JSON response.
- **FR-012**: Stream aggregation and translation MUST preserve supported
  content, tool calls, finish reason, usage, and refusal semantics. When
  `stream_options.include_usage=true` is supported, the standard final usage
  chunk MUST be emitted. A usage-only terminal event MUST NOT create a
  fabricated choice or content value.
- **FR-013**: `response_format.type=json_object` MUST be accepted only when the
  selected backend/model has verified equivalent structured-output behavior.
- **FR-014**: `response_format.type=json_schema` MUST preserve the client's
  schema, name, and strictness when translated to the upstream structured
  output format. The Chat Completions shape MUST map to the Responses API
  `text.format` shape only after Phase 0 verifies that the upstream accepts and
  honors that translation.
- **FR-015**: If Phase 0 cannot verify `json_object` or `json_schema` for a
  backend/model, its corresponding capability MUST remain false and Tianji
  MUST return HTTP 400 with:
  `error.type=invalid_request_error`,
  `error.param=response_format`, and
  `error.code=unsupported_parameter`.
- **FR-016**: Unsupported `tools`, `tool_choice`, `temperature`, `top_p`,
  `max_tokens`, `max_completion_tokens`, or
  `stream_options.include_usage` MUST return the same standard error shape,
  with the unsupported field named in `error.param`.
- **FR-017**: `/v1/embeddings` MUST accept a string input and a list of string
  inputs, return one embedding value per input, preserve the correct
  zero-based `data[].index`, and return explicit prompt and total usage fields.
  Tianji MUST NOT treat omitted upstream usage fields as valid zeros. A
  first-class provider adapter MAY map a documented embedding-only
  `total_tokens` field to `prompt_tokens=total_tokens`; the raw standard
  OpenAI-compatible path remains strict.
- **FR-018**: `/v1/embeddings` MUST support the standard
  `encoding_format=float` and `encoding_format=base64` representations. Float
  responses MUST contain numeric arrays. Base64 responses MUST encode
  validated numeric vectors as finite little-endian float32 bytes, reject
  values that overflow that representation, and MUST NOT depend on the backend
  returning an unvalidated Base64 string. The API MUST NOT require a
  project-specific encoding or vector shape.
- **FR-019**: `dimensions` MUST be accepted only when the selected
  backend/model explicitly supports it. Tianji MUST NOT hard-code a global
  embedding dimension or a Graphiti/Cognee-specific dimension.
- **FR-020**: The embedding response MUST use the standard list shape with
  `object`, `data`, `model`, and `usage`; provider-specific error payloads
  MUST be mapped to the standard Tianji error shape.
- **FR-021**: Standard client errors MUST use the shape
  `{"error":{"message":...,"type":...,"param":...,"code":...}}` where
  applicable, including `invalid_request_error` for HTTP 400,
  `unsupported_parameter`, `invalid_value`, `model_not_found`, and
  `invalid_request` classifications.
- **FR-022**: Tianji MUST map upstream failures to safe client-facing errors
  and MUST NOT expose upstream credentials, raw secret-bearing bodies, or
  internal transport details.
- **FR-023**: Existing `/v1/responses`, direct OpenAI HTTP behavior,
  ChatGPT Codex backend behavior, embedding behavior, model catalog behavior,
  and their existing regression tests MUST remain compatible except where
  this specification explicitly changes an unsupported-parameter result.
- **FR-024**: Phase 0 MUST be completed before implementation claims
  structured-output support for the Codex backend. It MUST verify:
  upstream `text.format`, stable `json_object` output, schema adherence,
  stream aggregation of content/tool calls/finish reason/usage/refusal, and
  the actual support semantics for tokens, temperature, and top-p.
- **FR-025**: Contract tests MUST cover non-streaming chat, streaming chat,
  stream-only aggregation, both structured-output modes, each listed chat
  parameter, unsupported-parameter errors, float and Base64 embeddings, models,
  and standard error mapping.
- **FR-026**: Verification MUST retain the existing direct OpenAI HTTP,
  ChatGPT Codex backend, `/v1/responses`, embedding, and model-catalog test
  coverage.
- **FR-027**: Completion MUST include authenticated smoke tests using
  Graphiti's generic OpenAI client, Cognee's custom LLM/embedding settings,
  a general OpenAI SDK client, and LiteLLM against the same Tianji base URL.
- **FR-028**: Graphiti's FalkorDB behavior, Cognee's vector/graph storage,
  Qdrant, Neo4j, and other client-owned persistence behavior MUST remain
  outside Tianji's API contract.

### Key Entities

- **Model/Backend Binding**: The selected public model identifier together
  with the backend transport that serves it, including
  `direct_openai_http` and `chatgpt_codex_backend`.
- **Capability Record**: The per-model/backend set of supported request,
  streaming, structured-output, tool, token, sampling, usage, and embedding
  capabilities used for validation and translation.
- **Chat Completion Contract**: The standard request, JSON response, SSE
  stream, tool-call, refusal, finish-reason, and usage behavior exposed at
  `/v1/chat/completions`.
- **Embedding Contract**: The standard input, float/Base64 representation,
  vector, index, model, and usage behavior exposed at `/v1/embeddings`,
  independent of any client-owned vector store.
- **Standard Error**: The sanitized client-visible error containing message,
  type, optional parameter, and code fields.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the required standard routes are reachable through one
  authenticated `/v1` base URL, including exact model lookup.
- **SC-002**: 100% of declared chat request capabilities either preserve their
  semantics through a verified backend translation or return a 400 standard
  unsupported-parameter error; none are silently removed.
- **SC-003**: For every stream-only aggregation fixture, the returned
  non-streaming response preserves all supported content, tool calls, finish
  reason, usage, and refusal fields.
- **SC-004**: Chat contract tests pass for both stream modes, JSON object,
  JSON Schema, token limits, temperature, top-p, tools, tool choice, and
  stream usage handling.
- **SC-005**: Embedding contract tests pass for string input and list input in
  both float and Base64 representations, with correct indexes, decodable
  finite vectors, explicit public usage fields, strict rejection of malformed
  standard upstream results, and provider tests for documented total-only
  usage normalization in 100% of fixture cases.
- **SC-006**: No embedding test or runtime rule assumes a global dimension;
  a model-specific dimension is accepted only when the capability record
  allows it.
- **SC-007**: Graphiti and Cognee smoke tests complete entity extraction,
  relationship extraction, structured extraction, embedding ingestion, and
  graph construction using only standard Tianji routes.
- **SC-008**: General OpenAI SDK and LiteLLM smoke tests use the same route and
  error contract as the Graphiti and Cognee tests.
- **SC-009**: Route inspection and negative tests confirm that
  `/v1/graphiti/*` and `/v1/cognee/*` are absent and no client-name fallback
  is required.
- **SC-010**: Existing direct OpenAI HTTP, ChatGPT Codex backend,
  `/v1/responses`, embedding, and model-catalog regression tests remain green.
- **SC-011**: Upstream capability evidence is recorded for the Codex backend
  before its structured-output capability is marked supported; otherwise the
  client receives the documented fail-closed error.

## Assumptions

- The public contract follows the current OpenAI-compatible shapes for chat
  completions, streaming chunks, embeddings, models, responses, and errors.
- `stream` omitted means non-streaming behavior, matching the normal client
  default.
- Existing Tianji authentication and organization/model access controls are
  reused; this feature does not create a second authentication system.
- The configured model catalog remains the source of available public model
  identifiers. This feature does not provision upstream credentials or models.
- The two named backend transports are the initial capability-matrix entries;
  adding another backend later requires its own capability evidence and
  contract tests.
- The current qwen3-embedding runtime observation of 1024 dimensions is a
  model fact only. It is not a Tianji-wide API invariant.
- Phase 0 may conclude that a capability is unavailable. A capability is not
  considered supported merely because a request can be serialized or because
  an upstream accepts an unknown field.
- Graphiti and Cognee client versions used for smoke tests can be configured
  with a standard OpenAI-compatible base URL and API key.
- Client-owned storage systems, including FalkorDB, Qdrant, and Neo4j, are
  provisioned and validated by their respective services.

## Non-Goals

- Adding `/v1/graphiti/*` or `/v1/cognee/*`.
- Adding Tianji-specific Graphiti or Cognee request parameters.
- Detecting or branching on a caller's project, library, or product name.
- Silently deleting unsupported request fields.
- Hard-coding an embedding dimension for all models.
- Moving Graphiti or Cognee graph/vector persistence into Tianji.
- Claiming Codex structured-output support before the Phase 0 evidence gate.
