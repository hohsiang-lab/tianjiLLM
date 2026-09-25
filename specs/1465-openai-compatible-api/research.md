# Research: Standard OpenAI-Compatible API

**Feature**: `1465-openai-compatible-api`
**Date**: 2026-08-08

This document separates repository evidence, current protocol references, and
unverified upstream behavior. A capability is not marked supported merely
because a JSON field can be serialized.

## Decision 1 — Reuse one authenticated `/v1` surface

**Decision**: Keep `POST /v1/chat/completions`, `POST /v1/embeddings`,
`GET /v1/models`, and `POST /v1/responses` as the public integration surface,
and add only the missing `GET /v1/models/{model}` route. Do not add
`/v1/graphiti/*` or `/v1/cognee/*`.

**Rationale**:

- `internal/proxy/server.go` already registers the standard routes under the
  existing authentication, budget, rate, cache, and model-access middleware.
- `internal/proxy/handler/chat.go` and `embedding.go` already resolve a model
  before provider I/O.
- A caller-neutral route boundary lets Graphiti, Cognee, OpenAI SDK, LiteLLM,
  and future clients share the same contract.

**Alternatives considered**:

- Project-specific Graphiti/Cognee routes: rejected because they violate the
  standards-first constitution principle and create behavior selected by caller.
- A second authentication or client adapter layer: rejected because the
  existing middleware is already the trust boundary.

**Repository evidence**:

- `internal/proxy/server.go:122-153, 280-297`
- `internal/proxy/handler/chat.go:29-137`
- `internal/proxy/handler/embedding.go:16-55`
- `internal/proxy/middleware/auth.go`

## Decision 2 — Capability matrix keyed by backend plus model

**Decision**: Add a typed, in-process `CapabilityMatrix` whose key is
`(backend, model)`. Resolve the backend from the selected transport identity,
not from caller metadata. Start with `direct_openai_http` and
`chatgpt_codex_backend`.

**Rationale**:

- `Provider.GetSupportedParams()` is currently provider-wide and cannot
  distinguish `json_object` from `json_schema`, verified usage chunks, or
  model-specific embedding dimensions.
- `config.TianjiParams.OpenAISubscriptionTransport` already names the two
  transport choices.
- An in-memory record avoids a new database/config service for a contract
  change; the matrix remains explicit and test-injectable.

**Population**:

- At startup/route resolution, emit one record for each configured public model
  binding, so the runtime key still contains both backend and model.
- `direct_openai_http` infers capabilities conservatively from verified
  first-class provider semantics and exact preserved parameters;
  `supports_embeddings` is true only when the resolved provider implements
  `EmbeddingProvider`, and dimensions remain false without explicit
  model evidence.
- `chatgpt_codex_backend` begins with the conservative Phase 0 result. A
  missing or unverified bit remains false.
- Contract tests inject a matrix with capable and incapable records; this is
  test setup, not a caller-specific production behavior.

**Required record fields**:

```text
supports_stream
supports_non_stream
supports_response_format
supports_json_object
supports_json_schema
supports_tools
supports_tool_choice
supports_temperature
supports_top_p
supports_max_tokens
supports_max_completion_tokens
supports_stream_options_include_usage
supports_embeddings
supports_dimensions
allowed_dimensions (optional)
```

`supports_dimensions` and `allowed_dimensions` are included because the
embedding contract must not forward or ignore `dimensions` without explicit
backend/model support.

**Alternatives considered**:

- Provider-only supported parameter lists: rejected because they are too broad
  for model/mode-specific validation.
- Caller/project keys such as `graphiti` or `cognee`: rejected by scope.
- A database-backed matrix: rejected as unnecessary storage and migration
  complexity for the initial two transports.

## Decision 3 — Phase 0 is a hard gate for Codex structured output

**Decision**: Keep Codex `response_format`, tools, and any other unverified
feature disabled until an authenticated upstream probe proves equivalent
semantics. Record redacted evidence in this feature directory.

**Repository evidence**:

- `internal/provider/chatgptcodex/payload.go` currently rejects `tools`,
  `tool_choice`, and `response_format`.
- The current payload contains `model`, `store`, `stream`, `instructions`,
  `input`, `temperature`, `top_p`, `max_output_tokens`, and `metadata`.
- `internal/provider/chatgptcodex/response.go` currently collects only
  `output_text`, maps completion usage, and maps completed/failed/incomplete
  stream events.
- `internal/proxy/handler/chatgpt_codex_backend.go` currently has a direct
  non-stream path and a direct SSE path; there is no common stream-to-JSON
  response path for `stream=false`.

**Probe questions**:

1. Does the authenticated upstream accept Responses `text.format`?
2. Does `json_object` produce valid JSON repeatedly?
3. Does `json_schema` preserve the submitted schema and strictness?
4. Do streamed events preserve content, tool calls, refusal, completion
   reason/status, and usage, including a usage-only terminal event?
5. Are token limits, temperature, and top-p accepted with equivalent
   semantics, rather than merely ignored?

**Decision if evidence is missing**: Capability stays false and Tianji returns
HTTP 400 with `type=invalid_request_error`, `param` naming the field, and
`code=unsupported_parameter`.

### Phase 0 live evidence — 2026-08-07

The authenticated probe used the local Codex credential store directly
against `https://chatgpt.com/backend-api/codex/responses`, with no token,
account ID, header value, or raw response body recorded. The model was
`gpt-5.6-terra`; the client compatibility version was `0.144.1`.

The same structured-output requests were also sent through the currently
deployed Tianji image
`sha256:603f6efa61b9bf43a9502a3a8a26749584935afe02a7a93cdbd9af4d58a7736d`
to confirm that the existing `/v1/responses` path preserved `text.format`.

| Capability | Result | Evidence |
|---|---|---|
| `supports_stream` | `true` | Direct upstream returned standard Responses SSE ending in `response.completed` |
| `supports_non_stream` | `false` | Direct upstream returned HTTP 400: `Stream must be set to true` |
| `supports_response_format` | `true` | `text.format` accepted for both verified modes |
| `supports_json_object` | `true` | Direct probe passed; deployed Tianji produced valid JSON in 3/3 runs |
| `supports_json_schema` | `true` | Direct probe passed; deployed Tianji matched the strict schema in 3/3 runs |
| `supports_tools` | `true` | Direct probe emitted function-call item and argument events |
| `supports_tool_choice` | `true` | `auto`, `required`, and `none` produced distinct expected call/no-call behavior |
| `supports_temperature` | `false` | Direct upstream returned HTTP 400 `Unsupported parameter: temperature` |
| `supports_top_p` | `false` | Direct upstream returned HTTP 400 `Unsupported parameter: top_p` |
| `supports_max_tokens` | `false` | No direct Codex equivalent was verified |
| `supports_max_completion_tokens` | `false` | Direct upstream returned HTTP 400 `Unsupported parameter: max_output_tokens` |
| `supports_stream_options_include_usage` | `true` | Terminal `response.completed` contained input/output/total usage |
| `supports_embeddings` | `false` | Codex Responses transport is not an embedding transport |
| `supports_dimensions` | `false` | Codex Responses transport is not an embedding transport |
| `allowed_dimensions` | `[]` | No embedding dimensions are accepted by this transport |

Content deltas, function-call argument deltas/final arguments, completion, and
usage were observed. A formal upstream refusal event was not reliably produced,
so refusal preservation is covered by deterministic fixtures but is not used
to enable any request capability.

The structured-output probe deliberately requested plain-text replies while
supplying `text.format`; it also checked the echoed terminal format, exact
JSON Schema keys, positive terminal usage, and both function-call argument
delta and done events. This prevents prompt compliance alone from being
mistaken for format enforcement.

## Decision 4 — Structured-output translation

**Decision**: Preserve the standard Chat Completions request shape:

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

When Phase 0 proves the Codex path, translate only the verified equivalent
shape:

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

**Rationale**: The OpenAI contract distinguishes Chat Completions
`response_format` from Responses `text.format`; the gateway must preserve
semantics, not simply rename a field and assume acceptance.

**Alternatives considered**:

- Prompting for JSON when structured output is unsupported: rejected because
  it is not equivalent and can corrupt Graphiti/Cognee data.
- Silently dropping the field: rejected by FR-010 and FR-016.

## Decision 5 — Stream-only backends use one aggregator

**Decision**:

- `stream=true`: request upstream streaming and translate each event to
  standard Chat Completions SSE.
- `stream=false` with only `supports_stream=true`: request upstream streaming,
  aggregate it, and return one standard Chat Completion JSON object.

The aggregation state preserves ordered content, refusal, tool-call IDs/indexes,
function names and argument fragments, final finish reason, model/id, and the
latest complete usage record. A usage-only terminal event updates usage without
creating a choice.

**Rationale**: This is the smallest way to reconcile a stream-only transport
with a standard client contract while retaining incremental behavior for
streaming clients.

**Repository evidence**:

- `internal/proxy/handler/chat.go:311-458`
- `internal/proxy/handler/chatgpt_codex_backend.go:73-184`
- `internal/provider/chatgptcodex/response.go`
- `internal/model/response.go`

## Decision 6 — Embedding validation is model-specific

**Decision**: Accept string and list-of-string input, support the standard
`float` and `base64` output representations, preserve indexes/usage, and accept
`dimensions` only when the `(backend, model)` record explicitly allows it.

The observed `qwen3-embedding` 1024-dimensional output is recorded as runtime
model evidence only; it is not a global Tianji rule.

The implementation uses these boundaries:

- the public input contract is one non-empty string or a non-empty list of
  non-empty strings; token arrays and mixed arrays are rejected;
- a blank model is rejected before exact or wildcard route resolution;
- omitted `encoding_format` and `encoding_format=float` return validated
  numeric arrays;
- `encoding_format=base64` is a Tianji-owned equivalent representation:
  Tianji requests numeric vectors upstream, validates them, and emits
  Base64-encoded little-endian float32 bytes rather than relying on a backend
  to return an unvalidated string;
- raw standard OpenAI-compatible responses must include
  `usage.prompt_tokens` and `usage.total_tokens`; missing fields are not
  accepted as implicit zeros;
- Jina and Voyage document embedding usage as `total_tokens` only, so their
  first-class provider adapters normalize `prompt_tokens=total_tokens` before
  shared validation and public serialization;
- Base64 conversion rejects numeric values that overflow to non-finite
  float32 rather than encoding infinity;
- `GetSupportedParams()` and parameter mappings are not proof that a model
  supports `dimensions`;
- first-party OpenAI `text-embedding-3-*` models using the default OpenAI
  endpoint have dimension evidence; custom endpoints, Jina, and other models
  remain fail-closed unless an exact backend/model capability record enables
  dimensions;
- `allowed_dimensions`, when present, is model-specific, and Tianji verifies
  that every returned vector has the requested width.

**Repository evidence**:

- `internal/model/response.go:69-137`
- `internal/proxy/handler/embedding.go`
- `internal/proxy/handler/capability.go`
- `internal/provider/openai/embedding.go`
- `test/contract/embedding_test.go`

**Alternatives considered**:

- Hard-code 1024: rejected because model runtime and backend can change.
- Infer dimensions from a provider-wide parameter list: rejected because it
  cannot prove exact model semantics.
- Forward all dimensions requests: rejected because unsupported parameters must
  fail before upstream execution.
- Require every embedding backend to implement Base64 itself: rejected because
  wire representation is gateway-owned and numeric vectors remain the
  validated internal form.

## Decision 7 — Standard sanitized errors

**Decision**: Use the existing `model.ErrorResponse`/`ErrorDetail` shape and
centralize the new classifications:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "param": "response_format",
    "code": "unsupported_parameter"
  }
}
```

Known client errors are HTTP 400. Exact model lookup uses HTTP 404 with
`code=model_not_found`. Provider credentials, raw upstream URLs, and secret
bodies never appear in client-visible errors.

**Repository evidence**:

- `internal/model/errors.go`
- `internal/proxy/handler/chat.go:269-305`
- `internal/proxy/handler/chatgpt_codex_backend_errors.go`
- `test/integration/error_format_test.go`

## Current protocol references

The implementation research used the current official documentation sources
through Context7:

- OpenAI API reference/guides:
  `https://developers.openai.com/api/docs/api-reference/chat/object`
  `https://developers.openai.com/api/docs/api-reference/embeddings/create`
  `https://developers.openai.com/api/docs/guides/responses-vs-chat-completions`
  `https://developers.openai.com/api/docs/guides/streaming-responses`
- LiteLLM client/proxy examples:
  `https://docs.litellm.ai/docs/proxy/users`
  `https://docs.litellm.ai/docs/embedding/supported_embedding`
- Graphiti OpenAI-compatible client and embedder implementation:
  `https://github.com/getzep/graphiti/blob/main/README.md`
  `https://github.com/getzep/graphiti/blob/main/graphiti_core/embedder/openai.py`

The references establish the standard route/request/response shapes and client
base-URL pattern. They do **not** prove the private/authenticated Codex
upstream behavior; the separate authenticated Phase 0 probe above provides
that model-specific evidence.

## Phase 7 real-client evidence — 2026-08-08

An authenticated local Tianji fixture passed Graphiti, Cognee, OpenAI SDK, and
LiteLLM smoke tests with both normal and race-enabled Go test runs. The pinned
client set was `graphiti-core 0.29.3`, `cognee 1.4.1`, `litellm 1.95.0`,
`instructor 1.15.1`, and `openai 2.53.0`.

The route recorder saw only the standard model, chat-completion, and embedding
paths. Graphiti entity/relationship extraction, Cognee Instructor extraction,
stream and non-stream clients, numeric embeddings, model lookup, and
model-not-found behavior completed successfully. Graph/vector persistence
remained in the Python clients.

Two dependency behaviors require explicit interpretation rather than a
Tianji-specific fallback:

- `graphiti-core 0.29.3` does not expose an embedding `encoding_format`
  setting. `openai 2.53.0` serializes the omitted value as `base64`. Tianji now
  treats that standard public representation as a gateway-owned equivalent
  translation: it requests `float` from the embedding backend, validates the
  numeric vector, and returns Base64-encoded little-endian float32 bytes. The
  stock Graphiti embedder therefore passes with only its base URL, API key,
  model, and model-specific dimension; no Graphiti branch or adapter exists.
- `litellm 1.95.0` converts Tianji's standard 404 body to its public
  `NotFoundError` and does not retain the original `error.type` or
  `error.code` fields. The smoke verifies HTTP 404 and normalizes that public
  exception class to `invalid_request_error` / `model_not_found`; the raw
  Tianji error body remains covered by contract tests.

Cognee `1.4.1` was exercised through its actual local pipeline rather than a
manually assembled graph. The smoke ran `cognee.add` and `cognee.cognify`,
validated the `KnowledgeGraph` and `SummarizedContent` schema requests,
read back a persisted graph from Ladybug, and confirmed non-empty LanceDB
collections. The fresh rerun read back 7 graph nodes and 6 graph edges, with
relationship types `contains`, `is_a`, `is_part_of`, and `made_from`, plus 6
vector items/collection entries at 1024 dimensions. Its observed route counts
were three chat requests and ten embedding requests; all graph/vector files
stayed under Cognee's temporary system root.

The semantic edge labels are model-output-dependent and are not a deterministic
acceptance condition. An earlier `alice -> works_at -> acme` observation was
not stable across the fresh rerun and must not be used as the required result.
The process emitted an `Unclosed client session` warning after a successful
exit; this is a client cleanup limitation, not a Tianji route or persistence
failure.

All pinned client dependencies were available in the required smoke
environment. No passing fallback or skipped required client was used.

## Unverified assumptions carried forward

- A formal private-Codex refusal event can be elicited reliably; deterministic
  fixtures verify preservation, but the live probe did not establish this
  event.
- A particular direct OpenAI-compatible model supports every standard
  parameter; the matrix must be conservative per backend/model.
- Client-owned FalkorDB, Qdrant, Neo4j, and graph/vector workflows are healthy
  outside Tianji.
