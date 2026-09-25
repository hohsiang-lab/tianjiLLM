# Quickstart: Validate the Standard `/v1` Contract

This is a validation guide, not an implementation recipe. It assumes the
existing Tianji authentication and model configuration are available.

## 1. Phase 0 Codex evidence

Use a test-only authenticated upstream configuration. Do not put access tokens,
cookies, account IDs, or raw secret-bearing responses in the repository.

The evidence record must answer:

- `text.format` acceptance for JSON object and JSON Schema;
- valid JSON and schema adherence over repeated requests;
- streamed content, tool-call fragments, refusal, completion, and usage;
- usage-only terminal events;
- token limits, temperature, and top-p semantics.

If any answer is unavailable, leave the corresponding capability false and
verify the standard 400 error instead of enabling a fallback.

## 2. Local focused tests

Run the new contract tests after the Phase 0 decision:

```bash
rtk go test ./test/contract/... ./test/integration/... \
  ./internal/provider/openai/... ./internal/provider/chatgptcodex/... \
  ./internal/proxy/handler/...
```

Then run the repository quality gate:

```bash
rtk go test -race -cover ./...
```

Existing direct OpenAI, Codex, Responses, embedding, model-catalog, and error
tests must remain green.

### User Story 1 red baseline — 2026-08-07

Before implementation, the focused contract run reported `5 passed, 12
failed`: `/v1/models/{model}` reached the generic 501 catch-all, bare
`/models/{model}` returned an unstructured 404, and all five caller-neutral
subtests failed at exact model retrieval. The restricted-key middleware test
also failed both model paths with HTTP 403 because the model could not be
extracted from the URL. The negative Graphiti/Cognee route checks and the five
missing-auth checks already passed.

### User Story 2 red baseline — 2026-08-07

Before implementation, the focused chat run reported `6 passed, 8 failed`.
Direct non-streaming requests dropped `max_completion_tokens`; a stream-only
route returned HTTP 502 instead of aggregating SSE; malformed/incomplete
streams used the wrong error shape; the streaming handler logged false success
instead of failure; and Codex completion/tool/refusal events did not preserve
the required usage-only and delta shapes.

### User Story 2 green evidence — 2026-08-07

The focused model/provider/handler/contract suite reported `1551 passed` in 15
packages, and the targeted callback/middleware/RBAC integration slice reported
`47 passed`. The same two runs under `-race` reported the same counts. The
incremental forwarding contract read the first SSE chunk before releasing the
upstream fixture, while the non-stream contract aggregated content, tool calls,
refusal, finish reason, and terminal usage into one Chat Completion JSON body.

The complete repository run reported `3176 passed, 21 failed, 8 skipped` in 119
packages. All 21 failures were existing PostgreSQL-backed integration tests
unable to connect to `localhost:5433`; no feature-scope test failed. `go vet`,
`golangci-lint`, `git diff --check`, and the uncommitted correctness review
were clean.

### User Story 3 green evidence — 2026-08-07

The capability/structured-output focused suite reported `1473 passed` in five
packages, plus `11 passed` standard-error integration tests. Both runs produced
the same counts under `-race`. The table-driven contract proves every declared
chat capability is either preserved exactly or rejected with
`invalid_request_error` and the expected `param`/`code` before mock-upstream
I/O.

Direct OpenAI JSON object and JSON Schema requests preserve the complete
`response_format` object. Verified Codex structured output maps to Responses
`text.format`; the unverified Codex model fixture rejects structured output,
sampling, token limits, tools, and tool choice before backend or usage-fetch
calls. `go vet`, `golangci-lint`, `git diff --check`, and the uncommitted
correctness review were clean.

### User Story 4 red baseline — 2026-08-07

Before implementation, the deterministic embedding/provider run reported `306
passed, 27 failed` in three packages. The public handler accepted missing,
empty, mixed, and non-string input; silently allowed `base64` and unsupported
dimensions; returned the upstream model instead of the public model; accepted
incomplete response lists; and exposed the wrong model/request/upstream error
classifications. The OpenAI provider also accepted invalid list/item/index,
empty-vector, and usage shapes.

The later wildcard regression test separately proved that an omitted model
could match `model_name: "*"`, reach upstream, and return HTTP 200.

### User Story 4 green evidence — 2026-08-07

The final model/provider/handler/contract suite reported `1436 passed` in six
packages, with the same count under `-race`. The uncommitted correctness review
also ran `39` race-enabled embedding cases and `1811` provider/handler/contract
tests across 60 packages with no actionable finding.

The public route returns numeric arrays for an omitted `encoding_format` or
`float`. For `base64`, Tianji requests numeric vectors upstream, validates
them, and returns the standard Base64 representation of little-endian float32
bytes. Dimensions require exact backend/model evidence, never provider-wide
parameter inference or a global `1024` default, and a requested width is
checked against the returned vectors. Blank models are rejected before
wildcard resolution.

## 3. Local HTTP contract smoke

Set a local authenticated URL and key:

```bash
export TIANJI_BASE_URL="http://127.0.0.1:8080/v1"
export TIANJI_API_KEY="replace-with-test-key"
export TIANJI_MODEL="model-from-v1-models"
```

List and retrieve the same model:

```bash
rtk curl -fsS \
  -H "Authorization: Bearer $TIANJI_API_KEY" \
  "$TIANJI_BASE_URL/models"

rtk curl -fsS \
  -H "Authorization: Bearer $TIANJI_API_KEY" \
  "$TIANJI_BASE_URL/models/$TIANJI_MODEL"
```

Non-streaming chat:

```bash
rtk curl -fsS \
  -H "Authorization: Bearer $TIANJI_API_KEY" \
  -H "Content-Type: application/json" \
  "$TIANJI_BASE_URL/chat/completions" \
  -d "{\"model\":\"$TIANJI_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"reply with ok\"}],\"stream\":false}"
```

Streaming chat:

```bash
rtk curl -N \
  -H "Authorization: Bearer $TIANJI_API_KEY" \
  -H "Content-Type: application/json" \
  "$TIANJI_BASE_URL/chat/completions" \
  -d "{\"model\":\"$TIANJI_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"reply with ok\"}],\"stream\":true}"
```

Embedding:

```bash
rtk curl -fsS \
  -H "Authorization: Bearer $TIANJI_API_KEY" \
  -H "Content-Type: application/json" \
  "$TIANJI_BASE_URL/embeddings" \
  -d "{\"model\":\"$TIANJI_MODEL\",\"input\":[\"one\",\"two\"],\"encoding_format\":\"float\"}"
```

Expected checks:

- model list and exact model IDs agree;
- chat JSON has `object=chat.completion`, choices, finish reason, and usage;
- chat SSE emits `chat.completion.chunk` events followed by `[DONE]`;
- embeddings contain one numeric vector per input with indexes `0..n-1`;
- unsupported fields return HTTP 400 with `error.type`,
  `error.param`, and `error.code`;
- no response contains raw upstream credentials, URLs, or secret bodies.

## 4. Client smoke

Run against the same `$TIANJI_BASE_URL` and `$TIANJI_API_KEY`; use synthetic
inputs and keep Graphiti/Cognee persistence outside Tianji.

### Graphiti

Configure `OpenAIGenericClient` with the Tianji base URL and key. Verify entity
extraction, relationship extraction, structured output, and embeddings. Record
that requests use only `/v1/chat/completions` and `/v1/embeddings`.

### Cognee

Configure:

```text
LLM_PROVIDER=custom
LLM_ENDPOINT=<tianji>/v1
LLM_MODEL=<model>
EMBEDDING_ENDPOINT=<tianji>/v1
```

Verify Instructor structured extraction, JSON Schema mode, embedding
ingestion, and graph construction. A failed Codex Phase 0 capability must be
reported as an expected standard unsupported-parameter error, not bypassed.

### OpenAI SDK and LiteLLM

Configure both clients with:

```text
base_url=<tianji>/v1
api_key=<key>
```

Run model listing, non-streaming chat, streaming chat, and embeddings. Compare
their response/error shapes with the contract fixtures rather than granting
client-specific exceptions.

### User Story 5 green evidence — 2026-08-08

The authenticated local fixture ran repository Tianji code against real Python
clients installed in `/tmp/tianji-client-smoke-venv`:

- `graphiti-core 0.29.3`
- `cognee 1.4.1`
- `litellm 1.95.0`
- `instructor 1.15.1`
- `openai 2.53.0`

Run:

```bash
rtk env \
  TIANJI_CLIENT_SMOKE=1 \
  TIANJI_CLIENT_SMOKE_REQUIRED=1 \
  TIANJI_CLIENT_SMOKE_PYTHON=/tmp/tianji-client-smoke-venv/bin/python \
  go test ./test/e2e \
  -run 'Test(GraphitiOpenAIGenericClientSmoke|CogneeCustomEndpointSmoke|OpenAISDKAndLiteLLMSmoke)$' \
  -count=1 -v
```

The run passed, and the same command with `go test -race` passed. Recorded
routes and shapes were:

- Graphiti: the stock `OpenAIGenericClient` and `OpenAIEmbedder` sent two JSON
  Schema chat requests and one `encoding_format=base64` embedding request.
  Tianji requested floats from the backend and returned the vector as standard
  Base64-encoded little-endian float32 bytes; entity `Alice` and relationship
  `WORKS_AT` were extracted without a custom client adapter.
- Cognee: the real `add` then `cognify` pipeline sent three chat requests (one
  endpoint check plus `KnowledgeGraph` and `SummarizedContent` JSON Schemas)
  and ten float embedding requests. The fresh rerun read back seven graph
  nodes and six graph edges, with relationship types `contains`, `is_a`,
  `is_part_of`, and `made_from`; it also read six vector items/collection
  entries at 1024 dimensions. All graph/vector files remained under Cognee's
  temporary client-owned storage. Semantic edge labels are model-output
  dependent, so an earlier `alice -> works_at -> acme` observation is not a
  deterministic acceptance condition.
- OpenAI SDK: model list/retrieve, non-stream and stream text `OK`, two
  embeddings, and a standard HTTP 404 model-not-found response.
- LiteLLM: the same chat, stream, embedding, and model-not-found fixture;
  LiteLLM represented the 404 as its public `NotFoundError`.

The recorder observed only `GET /v1/models`, `GET /v1/models/{model}`,
`POST /v1/chat/completions`, and `POST /v1/embeddings`. No Graphiti/Cognee
route, parameter, or persistence behavior entered Tianji.

The full local-only runbook and its safety boundary are recorded in
[live-e2e-test-plan.md](./live-e2e-test-plan.md). It explicitly uses
ephemeral PostgreSQL, does not deploy a canary or use Tailscale, and records
that local E2E writes normal Tianji telemetry beyond a request log.

## 5. Phase 8 final verification — 2026-08-08

Focused regressions passed with these final counts:

- Responses contract: `2 passed`;
- Responses handler: `40 passed`;
- feature packages: `1530 passed` across `test/contract`,
  `internal/provider/openai`, `internal/provider/jina`,
  `internal/provider/ollama`, `internal/provider/voyage`,
  `internal/provider/chatgptcodex`, and `internal/proxy/handler`;
- the same feature packages under `-race`: `1530 passed`;
- focused integration HTTP/error/callback/middleware/RBAC slice: `61 passed`;
- the same integration slice under `-race`: `61 passed`.

The Phase 8 review loop exposed three embedding boundary gaps before
completion. First, omitted required usage fields and finite float64 values
that overflow during Base64 float32 conversion were accepted; the new tests
first failed `3` provider subtests and `1` contract test, then passed after
shared provider/handler validation was added. Second, the documented Jina and
Voyage protocols return valid total-only usage, so their first-class adapters
now normalize `prompt_tokens=total_tokens` while the generic OpenAI-compatible
parser remains strict. Third, a custom Voyage `api_base` previously replaced
the Voyage adapter with the generic provider and bypassed that normalization;
a red registry-path regression now proves the Voyage wrapper is preserved.

The required real-client smoke passed in `10.158s`; its race-enabled run passed
in `10.930s`. All four clients used only the standard routes described above.

A process-level authenticated HTTP smoke against repository Tianji code and a
local mock upstream produced:

```text
{"models":2,"model_retrieve":"smoke-chat","chat_non_stream":"OK","chat_stream_done":true,"embedding_float_items":2,"embedding_base64_bytes":12,"responses_status":"completed","unsupported_code":"unsupported_parameter","missing_code":"model_not_found"}
http_statuses unsupported=400 missing=404 unauth=401
```

This covered model list/retrieval, non-stream JSON, OpenAI SSE plus `[DONE]`,
float and Base64 embeddings, `/v1/responses`, unsupported JSON Schema,
model-not-found, and missing authentication. The custom direct endpoint
remained conservative: streaming was exercised without claiming an
unverified `stream_options.include_usage` capability.

`rtk make lint` and `rtk make build` passed. The build warned that the
installed templ generator `v0.3.1001` is newer than `go.mod`'s `v0.3.977` and
regenerated 36 unrelated UI/CSS files; those generated changes were restored
after the successful build.

Direct `rtk make test` and `rtk make check` reached the repository test suite.
Their only failures were `19` top-level tests plus `2` subtests requiring
PostgreSQL at `localhost:5433`; both IPv4 and IPv6 connections were refused.
All feature packages, including `test/contract` and `test/e2e`, passed in those
runs. OrbStack was healthy, but its Docker API was unavailable and no process
listened on port 5433, so no unrelated local workloads were restarted.

`golangci-lint` reported `0 issues`. The correctness review findings were
corrected, and the Ponytail review found no justified simplification. Apart
from the intended feature-artifact edits prepared for this commit, the only
worktree entries were the untracked `graphify-out/`,
`internal/graphify-out/`, and `test/graphify-out/` directories; they were
preserved and excluded from staging.

## 6. Final evidence

Record the following before declaring completion:

| Evidence | Required result |
|---|---|
| Phase 0 | Redacted Codex capability decision in `research.md` |
| Route tests | Five required routes pass; project-specific routes absent |
| Chat tests | Both stream modes, aggregation, usage, tools/refusal fields |
| Structured output | Verified translation or fail-closed errors |
| Embeddings | String/list input, numeric vectors, indexes, usage |
| Regressions | Existing provider/Responses/model/error tests green |
| Client smoke | Graphiti, Cognee, OpenAI SDK, LiteLLM use standard routes |
| Security | Auth required; no secret-bearing error/client output |
