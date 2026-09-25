# Data Model: Standard OpenAI-Compatible API

No database tables or sqlc queries are added. The following are in-process
contract entities used after authentication and model resolution.

## Model/Backend Binding

Identifies the public model name and the transport selected for one request.

| Field | Type | Rule |
|---|---|---|
| `public_model` | string | Exact configured/listed model identifier exposed to clients |
| `backend` | enum/string | `direct_openai_http` or `chatgpt_codex_backend` initially |
| `upstream_model` | string | Provider-specific model name used by the transport |
| `provider` | `provider.Provider` | Existing transport implementation |
| `embedding_provider` | optional `provider.EmbeddingProvider` | Required only for embeddings |
| `access` | existing route/access result | Must be resolved by existing auth/model controls |

The binding is selected from the model catalog and request model. Caller
metadata never participates in selection.

## Capability Record

One immutable record for a `(backend, model)` pair.

| Field | Type | Meaning |
|---|---|---|
| `backend` | string | Transport identity |
| `model` | string | Public model identifier |
| `supports_stream` | bool | Upstream can produce a translated stream |
| `supports_non_stream` | bool | Upstream can produce one translated JSON response |
| `supports_response_format` | bool | Any structured response format is accepted |
| `supports_json_object` | bool | JSON object mode is verified |
| `supports_json_schema` | bool | JSON Schema mode is verified |
| `supports_tools` | bool | Tool definitions and call fragments are equivalent |
| `supports_tool_choice` | bool | Tool-choice semantics are equivalent |
| `supports_temperature` | bool | Sampling temperature is honored |
| `supports_top_p` | bool | Nucleus sampling is honored |
| `supports_max_tokens` | bool | `max_tokens` is representable |
| `supports_max_completion_tokens` | bool | `max_completion_tokens` is representable |
| `supports_stream_options_include_usage` | bool | Final usage chunk semantics are verified |
| `supports_embeddings` | bool | Backend/model can serve embeddings |
| `supports_dimensions` | bool | `dimensions` is explicitly accepted |
| `allowed_dimensions` | `[]int` | Optional model-specific allowed values |

The matrix lookup key is `(backend, model)`, never `graphiti`, `cognee`,
`openviking`, or a client library name.

## Capability Matrix

An in-memory map:

```text
CapabilityKey{Backend, Model} -> CapabilityRecord
```

Lookup is fail-closed for a required capability. A provider's
`GetSupportedParams()` can seed transport-level defaults, but it cannot turn on
structured-output modes, usage chunks, or dimensions without an explicit
record.

The runtime builder creates one record for each configured public model
binding. Direct OpenAI HTTP uses standard transport defaults and the resolved
`EmbeddingProvider` result; Codex uses the Phase 0 evidence result. A test may
inject records for a capable fixture, but production decisions never include a
caller/project key.

## Normalized Chat Request

The existing `model.ChatCompletionRequest` remains the input shape. Validation
adds these invariants before provider I/O:

- `model` resolves to a visible binding.
- `messages` is present and content is representable by the backend.
- omitted `stream` is treated as `false`.
- each declared parameter is either supported and translated or rejected.
- `response_format` is one of the supported standard modes.
- token and sampling values are valid.
- `max_tokens` plus `max_completion_tokens` is rejected when ambiguous.
- `stream_options.include_usage=true` is allowed only with the relevant
  streaming capability.

## Chat Stream Aggregate

Transient state used only for `stream=false` on a stream-only backend:

| Field | Type | Rule |
|---|---|---|
| `id` | string | First non-empty upstream response ID |
| `model` | string | Upstream model or requested public model |
| `content` | string/builder | Concatenate content deltas in order |
| `refusal` | optional string | Concatenate refusal deltas in order |
| `tool_calls` | ordered map by index | Merge IDs, function names, and argument fragments |
| `finish_reason` | optional string | Last verified terminal reason |
| `usage` | `model.Usage` | Latest complete usage event |
| `completed` | bool | True only after a valid completion event |

No choice is fabricated for a usage-only event. An incomplete/malformed stream
does not produce a successful JSON response.

## Embedding Contract

Transient request/response invariants:

- input is one string or a non-empty list of strings;
- `encoding_format=float` returns numeric arrays;
- `encoding_format=base64` returns Base64-encoded little-endian float32 bytes
  derived from validated numeric upstream vectors, with every converted value
  still finite;
- each output item has `object=embedding` and the zero-based input index;
- the public response has `object=list`, the public model, and explicitly present
  prompt/total usage fields;
- `dimensions` is checked against the matrix record and optional allowlist.

Base64 is an output representation owned by the gateway, not a backend
capability bit. The internal/provider response remains numeric so indexes,
finite values, float32 representability, dimensions, and usage are validated
before encoding. The raw standard OpenAI-compatible response requires both
usage fields. A first-class provider adapter may normalize a documented
embedding-only `total_tokens` value to `prompt_tokens=total_tokens`; missing
`usage` or `total_tokens` remains invalid.

## Standard Error

The client-visible error is:

```text
ErrorResponse{
  Error: ErrorDetail{
    Message: string,
    Type: "invalid_request_error" | safe upstream type,
    Param: optional string,
    Code: "unsupported_parameter" | "invalid_value" |
      "model_not_found" | "invalid_request" | safe upstream code
  }
}
```

Raw upstream details remain internal/log-redacted. Authentication and
organization failures continue to use the existing boundary.
