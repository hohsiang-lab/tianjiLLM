# Contract: OpenAI-Compatible `/v1`

## Client configuration

```text
base_url=https://<tianji-host>/v1
api_key=<key>
```

The API key is sent through the existing authenticated middleware. No caller
name, project name, Graphiti setting, Cognee setting, or special header is
required.

## Routes

| Method | Path | Contract |
|---|---|---|
| GET | `/v1/models` | Standard model list |
| GET | `/v1/models/{model}` | Exact model item from the same visible catalog |
| POST | `/v1/chat/completions` | Standard JSON or SSE Chat Completions |
| POST | `/v1/embeddings` | Standard embedding list in `float` or Base64 representation |
| POST | `/v1/responses` | Existing Responses contract, preserved |

`/v1/graphiti/*` and `/v1/cognee/*` are not routes in this contract.

## Models

List success:

```json
{
  "object": "list",
  "data": [
    {"id": "model-name", "object": "model", "owned_by": "tianji"}
  ]
}
```

Exact retrieval returns the matching model item with the same public `id`.
An unavailable exact model returns HTTP 404:

```json
{
  "error": {
    "message": "model \"missing\" not found",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

## Chat Completions request

Required fields:

```json
{
  "model": "model-name",
  "messages": [
    {"role": "user", "content": "hello"}
  ]
}
```

Supported declared fields are validated against the selected `(backend, model)`
capability record:

```text
stream
response_format.type=json_object
response_format.type=json_schema
max_tokens
max_completion_tokens
temperature
top_p
tools
tool_choice
stream_options.include_usage
```

Omitted `stream` is equivalent to `false`. A field is either translated with
equivalent semantics or rejected before upstream execution.

### Structured output shapes

Chat JSON Schema input:

```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "entity",
      "schema": {
        "type": "object",
        "properties": {"name": {"type": "string"}},
        "required": ["name"],
        "additionalProperties": false
      },
      "strict": true
    }
  }
}
```

Only a verified backend may translate it to:

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

If the selected record does not support the requested mode, return HTTP 400
with `type=invalid_request_error`, `param=response_format`, and
`code=unsupported_parameter`.

The authenticated Phase 0 probe verified that a Codex Responses binding whose
resolved upstream model is `gpt-5.6-terra` accepts the equivalent
`text.format` shapes for `json_object` and strict `json_schema`, and preserves
tools, tool choice, and terminal usage. That upstream remains stream-only and
does not support temperature, top-p, either Chat Completions token-limit
field, or embeddings. Those unsupported fields therefore remain fail-closed.

## Chat JSON response

For `stream=false`:

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "model-name",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "hello",
        "tool_calls": [],
        "refusal": null
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 5,
    "total_tokens": 15
  }
}
```

For a stream-only backend, Tianji obtains upstream SSE, aggregates the same
fields, and emits this JSON only after a valid completion event.

## Chat SSE response

For `stream=true`, the response is `text/event-stream`. Each event is:

```text
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1700000000,"model":"model-name","choices":[{"index":0,"delta":{"content":"hel"},"finish_reason":null}]}

```

The stream ends with:

```text
data: [DONE]

```

When `stream_options.include_usage=true` is supported, a final usage-only
chunk may contain `choices: []` and a populated `usage`; it must not invent
content or tool calls.

## Embeddings request and response

Accepted input forms:

```json
{"model":"embedding-model","input":"hello","encoding_format":"float"}
```

```json
{"model":"embedding-model","input":["hello","world"],"encoding_format":"float"}
```

Omitting `encoding_format` or setting it to `float` returns numeric arrays:

```json
{
  "object": "list",
  "data": [
    {"object":"embedding","index":0,"embedding":[0.1,0.2]}
  ],
  "model": "embedding-model",
  "usage": {"prompt_tokens": 2, "total_tokens": 2}
}
```

Setting `encoding_format=base64` requests the equivalent standard wire
representation. Tianji requests numeric vectors from the selected embedding
backend, validates indexes, vector contents, dimensions, and usage, then
verifies that every value remains finite as float32, encodes each vector as
little-endian float32 bytes, and Base64-encodes those bytes:

```json
{
  "object": "list",
  "data": [
    {"object":"embedding","index":0,"embedding":"zczMPc3MTD4="}
  ],
  "model": "embedding-model",
  "usage": {"prompt_tokens": 1, "total_tokens": 1}
}
```

This conversion is Tianji-owned response representation, not a backend
capability inference and not a silently dropped parameter.

The public response always contains both `usage.prompt_tokens` and
`usage.total_tokens`. Raw standard OpenAI-compatible responses must supply both
fields. First-class adapters for documented embedding-only protocols that
return only `total_tokens` (currently Jina and Voyage) normalize
`prompt_tokens=total_tokens` before shared validation; missing `usage` or
`total_tokens` remains invalid. This provider-protocol normalization is not a
client-specific fallback and does not treat an omitted field as zero.

A value that overflows to non-finite float32 is an invalid upstream result and
does not produce a successful Base64 embedding response.

`dimensions` is accepted only when the selected record explicitly supports it.
No client or global model dimension is implied.

## Standard client errors

Known request-validation failures use HTTP 400:

```json
{
  "error": {
    "message": "This model does not support response_format=json_schema",
    "type": "invalid_request_error",
    "param": "response_format",
    "code": "unsupported_parameter"
  }
}
```

Use `invalid_value` for malformed values and `invalid_request` for malformed
cross-field/input combinations. Exact missing models use HTTP 404 with
`type=invalid_request_error` and `code=model_not_found`. Provider failures are
mapped to a safe error body and do not expose upstream credentials, URLs, raw
secret-bearing bodies, or internal transport details.
