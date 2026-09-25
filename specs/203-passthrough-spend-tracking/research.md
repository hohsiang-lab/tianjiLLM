# Research: Passthrough Spend Tracking

## Current State Analysis

### Two separate passthrough paths

| Path pattern | Handler | Auth middleware | Spend tracking |
|---|---|---|---|
| `/v1/messages`, `/v1beta/models/*` | `handler.nativeProxy()` | ✅ chi llmMiddleware | ✅ full (non-stream + stream) |
| `/v1/{provider}/*` (catch-all) | `passthrough.Router` | ✅ chi llmMiddleware | ❌ stdout only |

Both paths walk through `llmMiddleware` which runs auth, so `context.Context` always carries `ContextKeyTokenHash`, `ContextKeyTeamID`, `ContextKeyOrgID`. After PR #102 merges, it also carries `ContextKeyRequesterIP`.

**`native_format.go` is correct and complete — no changes needed.**
**`passthrough/router.go` needs spend tracking wired in.**

### What `passthrough/router.go` currently does

`streamingReader.Close()` and `ModifyResponse` (non-streaming):
- calls `logger.ParseUsage(body)` → gets `(promptTokens, completionTokens int)` only
- logs to stdout: `log.Printf("pass-through %s: prompt=%d completion=%d", ...)`
- **never calls `spend.Tracker` or `callback.Registry`**

### What `passthrough/handlers.go` currently provides

`LoggingHandler` interface:
```go
type LoggingHandler interface {
    ParseUsage(body []byte) (promptTokens, completionTokens int)
    ProviderName() string
}
```

Implementations: `AnthropicLoggingHandler`, `GeminiLoggingHandler`, `VertexAILoggingHandler`, etc.

**Problems:**
1. `ParseUsage` only returns `(int, int)` — missing `cacheRead`, `cacheCreation`, `modelName`
2. No `ParseSSEUsage` for streaming
3. `AnthropicLoggingHandler.ParseUsage` ignores cache tokens (unlike `native_format.go` which correctly handles them)

### `native_format.go` reference implementations

`parseUsage(providerName, body)` → `(prompt, completion, cacheRead, cacheCreation int, modelName string)` — correct, handles cache tokens
`parseSSEUsage(providerName, raw)` → same signature — correct for streaming
`buildNativeLogData(ctx, providerName, modelName, startTime, ...)` → `callback.LogData` — uses `buildBaseLogData(ctx)` to extract context values

### Circular import analysis

`passthrough` package currently imports only `internal/provider/anthropic`. To add spend tracking it needs to import `internal/callback` and `internal/pricing` — both are leaf packages with no dependency on `handler` or `passthrough`. **No circular import risk.**

The `parseUsage`/`parseSSEUsage`/`buildNativeLogData` logic from `native_format.go` can be duplicated into `passthrough/` package directly — these are small, focused functions. A new shared package (`nativeusage/`) would be an unnecessary abstraction for ~60 lines of code.

### main.go wiring

`callbackRegistry` is built at line 185, `passthroughHandler` at line 314. `callbackRegistry` is in scope when `passthrough.NewRouter` is called. Injection is a one-line change: `passthrough.NewRouter(endpoints, nil, callbackRegistry)`.

---

## Decision

**No new packages.** The entire fix is self-contained in `internal/proxy/passthrough/`:

1. Expand `LoggingHandler` interface → add `ParseSSEUsage` + richer `ParseUsage` return values
2. Update handler implementations (`handlers.go`) to match — fix Anthropic cache token accounting
3. Add `callbacks *callback.Registry` to `Router`; thread `ctx`/`startTime`/`requestModel` through `streamingReader`
4. After parse, call `buildPassthroughLogData(ctx, ...) → callbacks.LogSuccess()`
5. Wire in `main.go`: `NewRouter(endpoints, guardrail, callbackRegistry)`

`native_format.go` — untouched.

---

## LiteLLM reference pattern

Studied `/Users/norman/src/github.com/BerriAI/litellm/litellm/proxy/pass_through_endpoints/`:

- `streaming_handler.py`: collect `raw_bytes` → after stream ends, call `_route_streaming_logging_to_handler()` → `async_success_handler()`
- `anthropic_passthrough_logging_handler.py`: `_build_complete_streaming_response()` reassembles SSE chunks → `_create_anthropic_response_logging_payload()` → cost calculation → callback
- IP: captured in `litellm_pre_call_utils.py` before proxy fires, stored in `metadata["requester_ip_address"]` — same pattern as our context injection

**Key insight**: LiteLLM uses the same callback system for passthrough as for normal calls. We replicate this.
