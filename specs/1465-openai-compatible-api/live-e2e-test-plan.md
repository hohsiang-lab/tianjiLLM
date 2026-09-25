# Local Live E2E Test Plan and Evidence

**Feature**: `1465-openai-compatible-api`  
**Date**: 2026-08-08  
**Status**: Executed locally; no canary deployment

This plan verifies the standard OpenAI-compatible contract with the real
Graphiti, Cognee, OpenAI SDK, and LiteLLM clients. It is intentionally
local-only. It does not deploy a canary, expose Tianji through Tailscale, or
connect the local process to production PostgreSQL.

## 1. Scope and safety boundary

### In scope

- `GET /v1/models`
- `GET /v1/models/{model}`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- `POST /v1/responses`
- Existing authentication, model access, capability validation, error mapping,
  stream translation, and embedding response validation.
- Real-client smoke tests using synthetic data.

### Explicitly out of scope

- `/v1/graphiti/*`
- `/v1/cognee/*`
- `graphiti_params`, `cognee_params`, or any caller-specific field
- Caller/project-name routing or fallback
- FalkorDB, Qdrant, Neo4j, or other Graphiti/Cognee persistence inside Tianji
- A global or client-specific embedding dimension
- Canary deployment, Tailscale exposure, or production database access

### Secret and data handling

- Test credentials are read by the local runtime or test environment but are
  never written to this document, test output, Git, or the final report.
- Do not record API keys, cookies, bearer values, account IDs, database URLs,
  authorization headers, key hashes, or raw secret-bearing upstream bodies.
- Use synthetic prompts and temporary client-owned Graphiti/Cognee storage.
- A local request can create telemetry rows; this test is not read-only from
  the local database's perspective.

## 2. Local topology

```text
Graphiti / Cognee / OpenAI SDK / LiteLLM
                  |
                  | standard OpenAI API
                  v
        Tianji http://127.0.0.1:18083/v1
                  |
          +-------+----------------------+
          |                              |
          v                              v
  ephemeral PostgreSQL             backend transports
  127.0.0.1:15433                  direct_openai_http
  database: tianji_e2e              chatgpt_codex_backend
          |                              |
          | local telemetry               +-- local/direct provider fixtures
          +-- no production connection   +-- authenticated Codex upstream
```

The executed run used:

| Public model | Resolved provider/upstream | Backend |
|---|---|---|
| `e2e-openrouter-gpt-4.1-nano` | `openrouter/openai/gpt-4.1-nano` | `direct_openai_http` |
| `e2e-openrouter-claude-haiku-4.5` | `openrouter/anthropic/claude-haiku-4.5` | `direct_openai_http` |
| `qwen3-embedding` | `ollama/qwen3-embedding:0.6b` | local embedding transport |
| `gpt-5.6-terra` | authenticated Codex Responses upstream | `chatgpt_codex_backend` |

The public capability key remains `(backend, model)`. It never contains
`graphiti`, `cognee`, `openviking`, or a client-library name.

## 3. Preconditions and setup

Before every rerun:

1. Confirm the branch and worktree. Preserve unrelated `graphify-out/`,
   `internal/graphify-out/`, and `test/graphify-out/` directories.
2. Start an ephemeral PostgreSQL instance on a dedicated local port and run
   Tianji migrations. Do not reuse a production connection string.
3. Start Tianji on a dedicated local port with a test-only configuration.
4. Confirm the Tianji process has PostgreSQL connections only to the local
   ephemeral port.
5. Confirm the client smoke environment contains the pinned dependencies:
   `graphiti-core 0.29.3`, `cognee 1.4.1`, `litellm 1.95.0`,
   `instructor 1.15.1`, and `openai 2.53.0`.
6. Use a route recorder or sanitized access log that records method and path
   only. Do not record authorization headers or request bodies.

Useful checks:

```bash
rtk ps
rtk lsof -nP -iTCP:18083 -sTCP:LISTEN
rtk lsof -nP -iTCP:15433
```

The local client smoke command is:

```bash
rtk env \
  TIANJI_CLIENT_SMOKE=1 \
  TIANJI_CLIENT_SMOKE_REQUIRED=1 \
  TIANJI_CLIENT_SMOKE_PYTHON=/tmp/tianji-client-smoke-venv/bin/python \
  go test ./test/e2e \
  -run 'Test(GraphitiOpenAIGenericClientSmoke|CogneeCustomEndpointSmoke|OpenAISDKAndLiteLLMSmoke)$' \
  -count=1 -v
```

Run the same command with `go test -race` before declaring the local smoke
complete.

## 4. User Story 1 — Standard routes and client neutrality

### Requests

Use one valid test key and a model returned by `/v1/models`.

| Case | Request | Expected result |
|---|---|---|
| Model list | `GET /v1/models` | HTTP 200, standard list shape |
| Exact model | `GET /v1/models/{model}` | HTTP 200, same public `id` as the list |
| Unknown model | `GET /v1/models/not-configured` | HTTP 404, `model_not_found` |
| Missing auth | Any required route without the test key | HTTP 401, standard auth error |
| Graphiti route probe | `GET /v1/graphiti/test` | No 2xx project-specific handler |
| Cognee route probe | `GET /v1/cognee/test` | No 2xx project-specific handler |

Send equivalent requests with only harmless caller metadata changed. The
status, selected public model, backend, capability decision, and route must
remain the same. The request must not require a Graphiti/Cognee header or
parameter.

### Assertions

- The model list and exact retrieval expose the same visible catalog.
- The route recorder sees only standard `/v1` routes.
- Project-specific route probes do not enter a special fallback.
- Invalid authentication is rejected before provider execution.
- No error body contains upstream credentials or internal provider details.

### Live result

PASS. Four models were listed and exact retrieval returned matching IDs.
Missing-auth and missing-model behavior used standard errors. Both
project-specific route probes returned 404, and no client smoke used a
Graphiti/Cognee route.

## 5. User Story 2 — Chat streaming and stream-only aggregation

### Direct transport cases

Run equivalent chat requests against a direct model:

```json
{
  "model": "e2e-openrouter-gpt-4.1-nano",
  "messages": [{"role": "user", "content": "Reply with OK."}],
  "stream": false
}
```

Repeat with `"stream": true`.

Assert:

- non-stream response has `object=chat.completion`, choices, a finish reason,
  and usage;
- stream response has `chat.completion.chunk` events, ordered content, a
  terminal finish reason, and `[DONE]`;
- `stream=true` forwards the first translated chunk before upstream completion;
- usage-only terminal chunks have `choices=[]` and do not invent content.

### Codex stream-only cases

Use `gpt-5.6-terra`:

1. Send `stream=false`. Tianji must request upstream SSE, aggregate content,
   tool calls, refusal, finish reason, and usage, then return one standard Chat
   Completion JSON body.
2. Send `stream=true`. Tianji must request upstream SSE and translate events
   into standard Chat Completion chunks without waiting for the complete
   answer.
3. Use deterministic fixtures for malformed, incomplete, tool-call, refusal,
   and usage-only terminal events. Incomplete or malformed streams must not be
   reported as successful completions.

The `supports_non_stream=false` matrix bit describes the upstream transport.
The public `stream=false` behavior is still valid through Tianji aggregation
when `supports_stream=true`.

### Live result

PASS. Direct streaming and non-streaming responses passed. The Codex path
aggregated stream-only upstream data for `stream=false` and emitted standard
SSE for `stream=true`. Content, tool-call arguments, finish information, and
terminal usage were preserved. Refusal preservation is covered by
deterministic fixtures; a deterministic live private-Codex refusal was not
elicited.

## 6. User Story 3 — Capability matrix and structured output

### Matrix checks

For every declared request field, assert one of:

1. equivalent translation and preserved semantics; or
2. HTTP 400 with:

```json
{
  "error": {
    "type": "invalid_request_error",
    "param": "<field>",
    "code": "unsupported_parameter"
  }
}
```

Cover:

- `response_format.type=json_object`
- `response_format.type=json_schema`
- `max_tokens`
- `max_completion_tokens`
- `temperature`
- `top_p`
- `tools`
- `tool_choice`
- `stream_options.include_usage`

Also test malformed values with `invalid_value`, and invalid combinations such
as both token-limit fields when the selected backend cannot represent them
unambiguously with `invalid_request`.

### Structured-output translation

For a verified backend, submit:

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

Verify the upstream request is equivalent to:

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

Do not enable `json_schema` merely because the gateway can serialize this
object. The upstream must accept it and enforce the schema.

### Codex capability result

The authenticated Phase 0 result for `gpt-5.6-terra` is:

| Capability | Result |
|---|---|
| stream | supported |
| non-stream upstream transport | unsupported; public `stream=false` uses aggregation |
| `json_object` | supported |
| `json_schema` | supported |
| tools | supported |
| tool choice | supported |
| `stream_options.include_usage` | supported |
| temperature | unsupported |
| top-p | unsupported |
| max tokens | unsupported |
| max completion tokens | unsupported |
| embeddings/dimensions | unsupported |

The unsupported Codex fields must fail before provider I/O. They must not be
silently dropped or replaced with a prompt-based JSON fallback.

The direct `e2e-openrouter-gpt-4.1-nano` model is the positive direct
structured/advanced-parameter fixture. The direct Claude Haiku probe returned
fenced JSON for a JSON-object request and is not acceptance evidence for a
strict JSON-object capability. Other OpenRouter models, including
provider-routed DeepSeek variants, are not stable acceptance fixtures because
provider routing can vary.

### Live result

PASS. Capability tests preserved supported fields and returned standard
unsupported-parameter errors for unsupported fields. The Codex
`response_format` to `text.format` translation was verified for JSON object and
strict JSON Schema. No parameter was silently removed.

## 7. User Story 4 — Embeddings and model consistency

### Requests

Use `qwen3-embedding` and synthetic input:

```json
{
  "model": "qwen3-embedding",
  "input": "one",
  "encoding_format": "float"
}
```

Repeat with:

- `input` as `["one", "two"]`;
- omitted `encoding_format`;
- `encoding_format=base64`;
- unsupported `dimensions`;
- an invalid empty or mixed-type input.

### Assertions

- String input returns one numeric vector with `index=0`.
- List input returns one vector per input with indexes `0..n-1`.
- Float responses contain finite numeric arrays.
- Base64 responses decode to little-endian float32 bytes with the expected
  vector width.
- Usage includes both `prompt_tokens` and `total_tokens`.
- `dimensions` is accepted only for an explicit `(backend, model)` capability.
- The public model in the response matches model discovery.
- The observed `qwen3-embedding` width of 1024 is runtime model evidence only;
  it is not a global Tianji rule.

### Live result

PASS. String/list inputs, float/Base64 representations, indexes, usage, model
identity, and unsupported-dimension errors passed. The verified runtime output
was 1024-dimensional for `qwen3-embedding`; no global dimension was added.

## 8. User Story 5 — Real standard-client smoke

All client tests use the same local base URL and test key. They use no
Tianji-specific route, parameter, or persistence hook.

### Graphiti

Configure the stock `OpenAIGenericClient` and `OpenAIEmbedder` with the local
`/v1` base URL, test key, chat model, and embedding model.

Verify:

- entity extraction;
- relationship extraction;
- JSON Schema structured requests;
- embedding ingestion;
- only `/v1/chat/completions` and `/v1/embeddings` are recorded.

Live result: PASS. Entity `Alice`, relationship extraction, and embeddings
completed through the standard routes. The embedding client used the standard
Base64 representation without a Graphiti adapter.

### Cognee

Use:

```text
LLM_PROVIDER=custom
LLM_ENDPOINT=<local-tianji>/v1
LLM_MODEL=<chat-model>
LLM_INSTRUCTOR_MODE=json_schema_mode
EMBEDDING_PROVIDER=openai_compatible
EMBEDDING_ENDPOINT=<local-tianji>/v1
```

Run the real `cognee.add()` then `cognee.cognify()` pipeline. Verify
Instructor/JSON Schema requests, embedding ingestion, graph construction, and
vector readback.

Fresh live result: PASS.

```text
graph_node_count: 7
graph_edge_count: 6
graph_relationship_types: contains, is_a, is_part_of, made_from
embedding_dimensions: 1024
vector_collection_count: 6
```

The graph and vector data remained in Cognee's temporary client-owned storage.
Semantic edge labels are model-output-dependent; an earlier
`alice -> works_at -> acme` observation is not a deterministic acceptance
condition. The run emitted an `Unclosed client session` warning after a
successful process exit; this is recorded as a client cleanup limitation, not
as a Tianji route failure.

### OpenAI SDK

Using `openai 2.53.0`, verify:

- `models.list()` and `models.retrieve()`;
- non-stream and stream chat;
- float and Base64 embeddings;
- missing-model HTTP 404 with `model_not_found`;
- missing-auth HTTP 401.

Live result: PASS.

### LiteLLM

Using `litellm 1.95.0`, verify:

- non-stream chat;
- stream chat;
- `stream_options.include_usage`;
- embeddings;
- missing-model HTTP 404.

Live result: PASS. LiteLLM converted the standard 404 into its public
`NotFoundError`, while Tianji's raw standard error shape remains covered by
contract tests.

### Route recorder result

The real-client recorder observed only:

```text
GET  /v1/models
GET  /v1/models/{model}
POST /v1/chat/completions
POST /v1/embeddings
```

The `/v1/responses` route was verified separately by the Responses smoke.
No `/v1/graphiti/*` or `/v1/cognee/*` request occurred.

## 9. `/v1/responses` compatibility boundary

The Codex Responses upstream is stream-only:

- `POST /v1/responses` with `stream=true` passed and emitted
  `response.created`, `response.in_progress`, output text delta/done, and
  `response.completed` events with usage.
- The OpenAI SDK `stream=false` request returned HTTP 400 with
  `Stream must be set to true`.

Therefore this feature must not claim non-stream Responses support for this
Codex binding. This is an explicit upstream capability boundary, not a
Graphiti/Cognee fallback opportunity.

## 10. Database impact and isolation evidence

The local process was checked with `lsof`; all Tianji PostgreSQL connections
pointed to `127.0.0.1:15433`. No production PostgreSQL connection was present.

The test does more than write a request log. The ephemeral local database
received normal Tianji telemetry such as:

- `SpendLogs`;
- `ErrorLogs`;
- request payload and usage records;
- provider/error records.

This is acceptable for the local test database and is why the plan does not
describe the run as read-only. It does not imply any production database
mutation.

## 11. Evidence classification

| Evidence class | What it proves | What it does not prove |
|---|---|---|
| Repository/static | Routes, validators, matrix, translation, fixtures, and standard error code | Current provider availability or private upstream behavior |
| Local live | Real client serialization, local routing, local DB isolation, current model outputs | Production deployment behavior |
| Authenticated upstream | Codex `text.format`, stream-only behavior, tools, usage, and rejected sampling/token parameters | A deterministic private-Codex refusal event |
| Client-owned storage | Graphiti/Cognee extraction, graph construction, vector ingestion | Tianji owning or managing those stores |

## 12. Pass/fail and stop/go criteria

### GO

Declare the local E2E complete only when:

- all five user-story smoke groups pass;
- both normal and race-enabled client smoke pass;
- no unsupported parameter is silently dropped;
- standard error shapes are preserved and upstream internals are sanitized;
- only standard `/v1` routes are observed;
- local PostgreSQL isolation is verified;
- Graphiti/Cognee storage remains client-owned;
- the documented Responses stream-only limitation is retained;
- no secret appears in tracked files, logs selected for evidence, or the final
  report.

### STOP

Stop and investigate if any of these occur:

- a local process connects to a production database;
- an unsupported capability reaches upstream or returns success;
- a project-specific route or fallback is invoked;
- `stream=false` turns an incomplete stream into a successful completion;
- auth or model access is bypassed;
- a raw upstream credential, URL, or secret-bearing body is returned;
- a client requires a Tianji-specific parameter.

## 13. Cleanup

After evidence is recorded:

1. Recheck listeners and process ownership with `rtk ps` and `rtk lsof`.
2. Stop the local Tianji process.
3. Stop any old local Tianji instances on ports `18081` and `18082` only after
   confirming they belong to this test.
4. Stop the ephemeral PostgreSQL container
   `tianji-e2e-postgres-1465`.
5. Stop the test-only port forwards.
6. Remove the temporary probe directory and temporary client scripts/config
   files.
7. Preserve `graphify-out/`, `internal/graphify-out/`, and
   `test/graphify-out/`.
8. Re-run `rtk git status --short` and confirm no generated artifact or secret
   was staged.

