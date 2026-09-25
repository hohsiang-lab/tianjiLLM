# Contract: Backend/Model Capability Matrix

## Key

```text
CapabilityKey{
  backend: "direct_openai_http" | "chatgpt_codex_backend",
  model:  "<public Tianji model identifier>"
}
```

The key contains no caller/project field. The same request receives the same
decision for the same backend/model regardless of whether the caller is
Graphiti, Cognee, OpenAI SDK, LiteLLM, OpenViking, or an unnamed client.

## Record

```text
{
  "supports_stream": true,
  "supports_non_stream": true,
  "supports_response_format": false,
  "supports_json_object": false,
  "supports_json_schema": false,
  "supports_tools": false,
  "supports_tool_choice": false,
  "supports_temperature": true,
  "supports_top_p": true,
  "supports_max_tokens": true,
  "supports_max_completion_tokens": false,
  "supports_stream_options_include_usage": false,
  "supports_embeddings": false,
  "supports_dimensions": false,
  "allowed_dimensions": []
}
```

The values above are an example conservative record, not a universal default.
Each backend/model entry must be populated from transport behavior and
verified evidence.

## Current runtime decisions

### `chatgpt_codex_backend` binding resolved to upstream `gpt-5.6-terra`

The lookup key still uses the configured public model identifier. Runtime
default population applies the following bits when that public binding
resolves to upstream `gpt-5.6-terra`; for example, an alias named
`terra-alias` is keyed as `(chatgpt_codex_backend, terra-alias)`. The
authenticated Phase 0 probe established this record:

```text
{
  "supports_stream": true,
  "supports_non_stream": false,
  "supports_response_format": true,
  "supports_json_object": true,
  "supports_json_schema": true,
  "supports_tools": true,
  "supports_tool_choice": true,
  "supports_temperature": false,
  "supports_top_p": false,
  "supports_max_tokens": false,
  "supports_max_completion_tokens": false,
  "supports_stream_options_include_usage": true,
  "supports_embeddings": false,
  "supports_dimensions": false,
  "allowed_dimensions": []
}
```

Capability records do not imply that Codex Chat Completions are supported.
After normal decode and route resolution, `/v1/chat/completions` returns HTTP
501 `not_supported`, without a Codex backend request, Platform Chat fallback,
usage fetch, or success callback. Embeddings remain unsupported for direct
official OpenAI bindings and Codex-resolved aliases or wildcards through the
existing route guards. Native Responses forwarding and generic Chat aggregation
remain unchanged.

### `direct_openai_http`

Direct transport inference is conservative. A provider's parameter list alone
does not prove structured-output modes, usage chunks, non-stream behavior, or
dimensions. First-class providers may declare verified standard semantics;
custom and remapped OpenAI-compatible endpoints remain fail-closed for
unverified bits. Dimensions are enabled only by explicit model evidence, such
as eligible first-party OpenAI `text-embedding-3-*` models, or by an injected
exact backend/model record.

`encoding_format=base64` is not a capability bit. It is gateway-owned
representation translation: Tianji obtains and validates numeric vectors, then
emits Base64-encoded little-endian float32 bytes. Backend support is still
represented by `supports_embeddings`; requested dimensions remain governed by
`supports_dimensions` and `allowed_dimensions`.

## Decision rules

1. Resolve the model/backend binding first.
2. Look up the exact `(backend, model)` record.
3. For every declared request parameter:
   - translate it with equivalent semantics when the relevant bit is true; or
   - return HTTP 400 `invalid_request_error` with
     `code=unsupported_parameter` and the parameter name.
4. Do not use `GetSupportedParams()` as proof for structured-output modes,
   usage chunks, or dimensions.
5. An absent or unverified record fails closed.
6. An allowed dimensions list, when present, is checked after
   `supports_dimensions=true`.

## Stream mode rules

| Client request | Matrix requirement | Upstream action |
|---|---|---|
| `stream=false` | `supports_non_stream` | direct JSON translation |
| `stream=false` | only `supports_stream` | request SSE, aggregate, return JSON |
| `stream=true` | `supports_stream` | request SSE, translate chunks |
| either | neither | standard unsupported capability error |

## Phase 0 rule

Codex capabilities are true only where the authenticated probe established
equivalent semantics for the exact model. For `gpt-5.6-terra`, JSON object,
JSON Schema, tools, tool choice, and terminal usage are verified; temperature,
top-p, token limits, embeddings, and dimensions remain false. A smoke test
cannot override this record.
