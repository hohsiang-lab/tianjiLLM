# Implementation Plan: Passthrough Spend Tracking

**Branch**: `203-passthrough-spend-tracking` | **Date**: 2026-03-28 | **Spec**: `specs/203-passthrough-spend-tracking/spec.md`

## Summary

`passthrough/router.go` parses token usage but only prints to stdout — it never calls `spend.Tracker`. `native_format.go` is already correct and needs no changes.

Fix is entirely within `internal/proxy/passthrough/`:
1. Expand `LoggingHandler` interface to return full usage (cache tokens + model name + SSE variant)
2. Inject `*callback.Registry` into `Router`; thread `ctx`/`startTime`/`requestModel` into `streamingReader`
3. After parse → `callbacks.LogSuccess(buildPassthroughLogData(ctx, ...))`
4. Wire `callbackRegistry` in `main.go`

No new packages. No schema changes. `native_format.go` untouched.

## Passthrough Path Inventory

There are three distinct passthrough paths in the codebase. This PR targets **Path 2 only**.

| # | Path pattern | Handler | Spend tracking | This PR |
|---|---|---|---|---|
| 1 | `/v1/messages`, `/v1beta/models/*` | `handler.nativeProxy()` | ✅ complete | untouched |
| 2 | `/v1/{provider}/*` (catch-all) | `passthrough.Router` | ❌ stdout only | ✅ **in scope** |
| 3 | Batches, OCR, Video, Containers, RAG, Assistants, Threads, Runs | `proxyPassthrough()` → `forwardToProvider()` | ❌ none | ❌ out of scope |

### Why Path 3 is out of scope

`forwardToProvider()` handles two categories of endpoints:

- **Management APIs** (Assistants, Threads, Messages, Runs, Files) — these are CRUD operations, not LLM completion calls. They produce no `prompt_tokens`/`completion_tokens`. Spend tracking is not applicable.
- **Async/non-standard APIs** (Batches, OCR, Video, RAG) — token usage is either returned asynchronously in batch results, provider-specific with no unified format, or not token-based. Each would require a dedicated tracking strategy outside the scope of this PR.

These should be tracked separately per endpoint type in follow-up issues.

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: `internal/callback`, `internal/pricing` (both leaf packages, no circular import risk)
**Storage**: PostgreSQL via existing `spend.Tracker` — no schema changes, no new sqlc queries
**Testing**: `go test` + `testify`
**Target Platform**: Linux pod (OrbStack k8s)
**Performance Goals**: Zero overhead on hot path — spend logging is always `go callbacks.LogSuccess(...)`
**Constraints**: `native_format.go` must remain untouched (already correct)
**Scope**: `internal/proxy/passthrough/` only + one-line wiring in `cmd/tianji/main.go`

## Constitution Check

| Gate | Status | Notes |
|---|---|---|
| I. Python-First Reference | ✅ | LiteLLM `streaming_handler.py` + `anthropic_passthrough_logging_handler.py` studied. Same pattern: collect bytes → parse after stream → same callback system as normal calls. |
| II. Feature Parity | ✅ | LiteLLM logs spend for all passthrough paths. We match this. |
| III. Research Before Build | ✅ | See `research.md`. No new libraries. Circular import analysis complete. |
| IV. Failing-Tests-First | ✅ | Tests listed below. |
| V. Go Best Practices | ✅ | No new abstractions. Changes contained within existing package. |
| VI. No Stale Knowledge | ✅ | All decisions based on reading actual source files. |
| VII. sqlc-First | ✅ | No new DB queries. `CreateSpendLog` called via existing `spend.Tracker`. |

## Project Structure

### Documentation

```text
specs/203-passthrough-spend-tracking/
├── plan.md       ← this file
├── research.md   ← done
└── tasks.md      ← /speckit.tasks output
```

### Source Code Changes

```text
internal/proxy/passthrough/
├── handlers.go       MODIFY — expand LoggingHandler interface + fix implementations
├── handlers_test.go  MODIFY — update tests for new interface + cache token cases
├── router.go         MODIFY — inject callbacks, ctx, startTime, requestModel
└── router_test.go    MODIFY — add spend tracking tests

cmd/tianji/main.go    MODIFY — pass callbackRegistry to passthrough.NewRouter
```

**Structure Decision**: All changes within `internal/proxy/passthrough/` and `main.go`. No new packages.

## Interface Design

### Before (handlers.go)

```go
type LoggingHandler interface {
    ParseUsage(body []byte) (promptTokens, completionTokens int)
    ProviderName() string
}
```

### After (handlers.go)

```go
type LoggingHandler interface {
    // ParseUsage extracts token usage from a non-streaming response body.
    ParseUsage(body []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)
    // ParseSSEUsage extracts token usage from collected SSE bytes.
    ParseSSEUsage(raw []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)
    ProviderName() string
}
```

`AnthropicLoggingHandler.ParseUsage` currently **silently drops cache tokens** — this is a bug fixed as part of this PR.

### Router changes (router.go)

```go
type Router struct {
    endpoints []Endpoint
    loggers   map[string]LoggingHandler
    guardrail GuardrailHook
    callbacks *callback.Registry   // NEW
}

func NewRouter(endpoints []Endpoint, guardrail GuardrailHook, callbacks *callback.Registry) *Router

type streamingReader struct {
    src          io.ReadCloser
    buf          bytes.Buffer
    logger       LoggingHandler
    guardrail    GuardrailHook
    provider     string
    resp         *http.Response
    // NEW fields:
    ctx          context.Context
    startTime    time.Time
    requestModel string
    callbacks    *callback.Registry
}
```

### New helper (router.go)

```go
// buildPassthroughLogData constructs a LogData from passthrough usage.
// Mirrors buildNativeLogData in native_format.go.
func buildPassthroughLogData(ctx context.Context, providerName, modelName string,
    startTime time.Time, prompt, completion, cacheRead, cacheCreation int) callback.LogData
```

## Failing Tests

### User Story 1: LoggingHandler interface returns full usage

| Test Function | File | Assertion | Covers |
|---|---|---|---|
| `TestAnthropicLoggingHandler_ParseUsage_CacheTokens` | `internal/proxy/passthrough/handlers_test.go` | `ParseUsage` returns correct `cacheRead` + `cacheCreation` + `modelName` for Anthropic response with cache fields | Cache token accounting bug fix |
| `TestAnthropicLoggingHandler_ParseSSEUsage_Streaming` | `internal/proxy/passthrough/handlers_test.go` | `ParseSSEUsage` extracts model from `message_start`, output from `message_delta` | Streaming Anthropic SSE |
| `TestAnthropicLoggingHandler_ParseSSEUsage_CacheTokens` | `internal/proxy/passthrough/handlers_test.go` | `ParseSSEUsage` returns cacheRead + cacheCreation from `message_start` | Cache tokens in streaming |
| `TestGeminiLoggingHandler_ParseSSEUsage` | `internal/proxy/passthrough/handlers_test.go` | `ParseSSEUsage` takes last usageMetadata chunk | Gemini streaming |
| `TestVertexAILoggingHandler_ParseSSEUsage` | `internal/proxy/passthrough/handlers_test.go` | `ParseSSEUsage` handles Vertex AI SSE format | Vertex AI streaming |

### User Story 2: Router calls LogSuccess with correct data

| Test Function | File | Assertion | Covers |
|---|---|---|---|
| `TestRouter_NonStreaming_CallsLogSuccess` | `internal/proxy/passthrough/router_test.go` | Non-streaming response → `mockCallbacks.LogSuccess` called once with correct model + tokens + provider | Non-streaming spend log |
| `TestRouter_Streaming_CallsLogSuccessAfterStreamEnd` | `internal/proxy/passthrough/router_test.go` | Streaming response fully consumed → `LogSuccess` called with correct tokens | Streaming spend log |
| `TestRouter_NonStreaming_CacheTokensLogged` | `internal/proxy/passthrough/router_test.go` | Anthropic non-streaming response with cache fields → `LogData.CacheReadInputTokens` + `CacheCreationInputTokens` populated | Cache tokens reach LogData |
| `TestRouter_NoCallbacks_DoesNotPanic` | `internal/proxy/passthrough/router_test.go` | `Router` with `nil` callbacks handles request without panic | Nil-safe |
| `TestRouter_NonOK_DoesNotCallLogSuccess` | `internal/proxy/passthrough/router_test.go` | 429 upstream → `LogSuccess` NOT called | Error path |
| `TestRouter_LogData_HasContextValues` | `internal/proxy/passthrough/router_test.go` | `LogData.APIKey` + `TeamID` extracted from request context | Context threading |
| `TestRouter_LogData_HasDuration` | `internal/proxy/passthrough/router_test.go` | `LogData.EndTime.After(LogData.StartTime)` | Duration tracking |

### Verification Command

```bash
go test ./internal/proxy/passthrough/... -v -run "TestAnthropicLoggingHandler|TestGemini|TestVertexAI|TestRouter"
go test ./internal/... 2>&1 | grep -E "FAIL|ok"
```

## Implementation Order

1. Expand `LoggingHandler` interface in `handlers.go`
2. Write failing tests in `handlers_test.go` → `go test` confirms they fail
3. Update handler implementations (`AnthropicLoggingHandler`, etc.) → tests pass
4. Write failing tests in `router_test.go` → `go test` confirms they fail
5. Update `router.go`: add `callbacks` field, thread `ctx`/`startTime`/`requestModel`, add `buildPassthroughLogData`, call `LogSuccess`
6. Update `main.go`: pass `callbackRegistry` to `NewRouter`
7. Run full suite: `go test ./internal/...`

## Complexity Tracking

No violations. The previous plan proposed a `nativeusage/` package to avoid a circular import that does not actually exist — eliminated. All changes stay within the existing `passthrough` package boundary.
