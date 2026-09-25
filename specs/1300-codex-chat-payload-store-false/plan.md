# Implementation Plan: Codex backend chat payload `store:false`

**Branch**: `HO-1300-codex-chat-store-false`
**Spec**: `specs/1300-codex-chat-payload-store-false/spec.md`
**Linear**: HO-1300
**Phase**: Todo planning only; production/test implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Language/runtime: Go module `github.com/praxisllmlab/tianjiLLM`.
- Request model: `internal/model/request.go`.
- Codex payload builder: `internal/provider/chatgptcodex/payload.go`.
- Codex transport: `internal/provider/chatgptcodex/transport.go`.
- Chat route: `internal/proxy/handler/chat.go` and `internal/proxy/handler/chatgpt_codex_backend.go`.
- Existing wildcard coverage: `internal/proxy/handler/chatgpt_codex_wildcard_test.go`.
- Existing Codex payload tests: `internal/provider/chatgptcodex/payload_test.go`.

## Governance

- Linear HO-1300 is currently `Todo`; only planning artifacts and a draft PR are allowed.
- Do not edit `internal/`, `test/`, config, or production code until Linear moves to `In Progress`.
- Implementation must start with RED tests after the state gate unlocks.

## Repo Reality

- `chatgptcodex.Payload` currently has `Model`, `Instructions`, `Input`, sampling fields, and `Metadata`, but no `Store` field.
- `BuildPayload` rejects any non-empty `req.ExtraParams` as `unsupported ChatGPT Codex backend payload field: unknown extra parameters`.
- `ChatCompletionRequest.knownFields` does not include `store`, so top-level client JSON `store:false` is captured in `ExtraParams`.
- `internal/provider/openai/openai.go` intentionally merges `ExtraParams` into normal OpenAI API-key chat-completions requests.
- HO-1288's contract already documented a minimum Codex backend body containing `"store": false`; HO-1300 owns closing the implementation gap.

## Plan Review Evidence

- Official OpenAI Responses reference is the closest public API surface and documents `store=false` as stateless Responses API behavior: <https://developers.openai.com/api/reference/resources/responses/methods/create>.
- The ChatGPT Codex backend path is private; no public official page documents `chatgpt.com/backend-api/codex/responses`, so repo-owned HO-1288 contracts and local mocks remain the source for private backend behavior.
- Context7 lookup for OpenAI docs was attempted but failed with monthly quota exceeded.
- GitHub grep-app lookup for `store:false` in OpenAI public SDK examples produced no useful implementation pattern and one grep.app HTML content-type failure. This does not change the repo-specific plan because the fix is a private backend contract.
- Current repo evidence is decisive: the payload omits `store`, client `store:false` becomes `ExtraParams`, and normal OpenAI provider still depends on `ExtraParams` pass-through.

## Architecture

### Payload builder

Add a `Store bool` field to `chatgptcodex.Payload` with JSON tag `json:"store"`. Because the field must be present even when false, do not use `omitempty`.

`BuildPayload` should initialize:

```go
payload := Payload{
    Model: normalizeModel(req.Model),
    Input: make([]InputMessage, 0, len(req.Messages)),
    Temperature: req.Temperature,
    TopP: req.TopP,
    Store: false,
}
```

### Codex-only `store` validation

Keep `ChatCompletionRequest` global parsing unchanged so normal OpenAI API-key routes preserve unknown-parameter pass-through.

Inside Codex payload validation:

- If `req.ExtraParams["store"]` is boolean `false`, ignore it for the unknown-parameter check.
- If `req.ExtraParams["store"]` is boolean `true`, return local `invalid_request_error` explaining Codex backend requires `store:false`.
- If `store` is present but not boolean, return local `invalid_request_error` explaining `store` must be `false`.
- After handling `store`, any remaining `ExtraParams` still return `unsupported ChatGPT Codex backend payload field: unknown extra parameters`.

This keeps the scope local to `internal/provider/chatgptcodex` and avoids changing OpenAI provider behavior.

### Handler coverage

Add handler tests using mocked Codex backend and existing subscription harness:

- No client `store`: captured backend JSON includes `"store": false`.
- Client `store:false`: request succeeds and captured backend JSON includes `"store": false`.
- Client `store:true`: HTTP 400 and backend receives zero calls.
- Client `store:false` plus another unknown field: HTTP 400 for unknown parameters.

### OpenAI API-key no-regression

Add or extend OpenAI provider tests to prove normal API-key transform still passes `ExtraParams["store"] = false` through to `/chat/completions`. Do not add `store` to `knownFields` unless implementation evidence proves this is safe for normal API-key behavior.

## Failing Tests

| Test | File | Initial failure to prove | Covers |
| --- | --- | --- | --- |
| `TestBuildPayload_IncludesStoreFalse` | `internal/provider/chatgptcodex/payload_test.go` | marshaled payload omits `store` | FR-001..FR-003 |
| `TestBuildPayload_AllowsClientStoreFalse` | same | `store:false` rejected as unknown `ExtraParams` | FR-004 |
| `TestBuildPayload_RejectsClientStoreTrue` | same | no clear local `store` validation | FR-005..FR-006 |
| `TestChatGPTCodexBackendTransport_AllowsClientStoreFalse` | `internal/proxy/handler/chatgpt_codex_backend_test.go` or wildcard test | handler returns 400 before backend | FR-004, FR-012 |
| `TestChatGPTCodexBackendTransport_RejectsClientStoreTrueBeforeUpstream` | same | backend is called or error is unclear | FR-005..FR-006 |
| `TestOpenAITransformRequest_StoreExtraParamPassThrough` | `internal/provider/openai/openai_test.go` | normal OpenAI pass-through changed | FR-008 |

## Implementation Phases

### Phase 1 - RED tests

Add the tests above and run targeted commands to confirm failures correspond to missing `store:false` and Codex-only `store` validation.

### Phase 2 - Minimal fix

Patch only `internal/provider/chatgptcodex/payload.go` unless RED tests prove a handler-level change is required. Avoid changing request parsing globally.

### Phase 3 - Regression verification

Run targeted provider/handler tests, OpenAI provider no-regression tests, `gofmt`, and `git diff --check`.

### Phase 4 - State progression

After implementation and review gates pass, move through `In Review`, `Waiting CI`, and finally `Waiting Merge` only when CI is green and worktree is clean.

## Verification Commands

```bash
go test ./internal/provider/chatgptcodex -run 'TestBuildPayload_.*Store' -count=1
go test ./internal/proxy/handler -run 'TestChatGPTCodexBackend.*Store|TestChatGPTCodexBackendWildcard' -count=1
go test ./internal/provider/openai -run 'TestTransformRequest_ExtraParams|TestOpenAI.*Store' -count=1
go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai -count=1
git diff --check origin/main...HEAD
```

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Fix changes normal OpenAI chat parameter pass-through | Keep `store` handling inside Codex payload validation; add OpenAI provider no-regression test. |
| `store:false` is accepted while other unknown fields accidentally pass | Filter only `store:false`; keep strict unknown-parameter rejection. |
| `store:true` reaches upstream | Handler test asserts zero mocked backend calls. |
| Tests leak fake bearer material | Use fake sentinels and negative assertions; never print raw auth headers in failure messages. |
| Real network call sneaks into tests | Use local `httptest.Server` and existing handler harnesses only. |

## Todo Gate Status

Spec/plan/tasks/analyze can proceed to docs-only draft PR review. Production/test implementation remains blocked until Linear `In Progress`.
