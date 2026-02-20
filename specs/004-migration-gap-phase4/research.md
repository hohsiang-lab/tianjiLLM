# Research: Phase 4 — Full Migration Gap Analysis

**Branch**: `004-migration-gap-phase4` | **Date**: 2026-02-17
**Status**: Complete — all NEEDS CLARIFICATION resolved

## Research Summary

3 parallel research agents + 1 technology-specific agent were dispatched to investigate:
1. Category A blocking gaps — current Go code state and insertion points
2. Category B router + Category F core gaps — infrastructure reuse potential
3. Category D proxy features + plugin system architecture
4. Key technology decisions — WebSocket library, tiktoken Go, Responses API

---

## Decision 1: WebSocket Library for Realtime API (A6)

**Decision**: Use `github.com/coder/websocket` (formerly `nhooyr.io/websocket`)

**Rationale**:
- `context.Context` is a first-class citizen in all APIs — critical for proxy scenarios where client disconnect must cancel upstream connection immediately
- Built-in concurrent write safety — proxy relays from two goroutines (client→upstream, upstream→client) simultaneously
- `websocket.NetConn()` wrapper enables standard `io.Copy` pipe patterns
- Actively maintained by Coder (used in `coder/coder`, `cloudflared`, `boundary`)
- Verified WebSocket proxy pattern from kernel/kernel-images project (DevTools proxy, same architecture as our use case)

**Alternatives Considered**:
- `gorilla/websocket` — works but no context support, requires manual mutex for concurrent writes, maintainer has stepped back
- `golang.org/x/net/websocket` — effectively deprecated

**Implementation Pattern**:
```
clientConn := websocket.Accept(w, r, opts)
upstreamConn := websocket.Dial(ctx, upstreamURL, opts)
// goroutine 1: relay client → upstream
// goroutine 2: relay upstream → client
// context cancellation propagates cleanup
```

---

## Decision 2: Token Counting Library (F1)

**Decision**: Use `github.com/pkoukk/tiktoken-go` with offline BPE loader

**Rationale**:
- Only serious Go tiktoken implementation (884 stars, 2700+ dependents)
- OpenAI Cookbook-recommended
- Supports all current encodings: `o200k_base` (GPT-4o, GPT-4.1, GPT-4.5, o1, o3), `cl100k_base` (GPT-4, GPT-3.5-turbo), `p50k_base`, `r50k_base`
- Model prefix mapping auto-resolves new model names
- `tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())` avoids runtime download

**Alternatives Considered**:
- `tiktoken-go/tokenizer` (pure Go, embedded vocab) — smaller community, fewer features
- `pandodao/tokenizer-go` (Goja JS runtime) — performance overhead, not suitable for per-request counting

**Limitations**:
- Only accurate for OpenAI models; Anthropic/Gemini/etc. use different tokenizers
- For non-OpenAI models, fall back to API-returned `usage` (acceptable for proxy)
- Unknown models default to `o200k_base` encoding

---

## Decision 3: Responses API Implementation Strategy (A2)

**Decision**: Two-phase approach — Phase 1: pass-through to OpenAI; Phase 2: cross-provider bridge

**Rationale**:
- Responses API is fundamentally different from Chat Completions (not just a wrapper):
  - Request: `input` (not `messages`), `instructions` (not `system` role), `previous_response_id` (server-side state)
  - Response: `output` array with typed items (not `choices`), semantic status field
  - Streaming: event-based protocol (`response.created`, `response.output_text.delta`, `response.completed`) vs simple delta SSE
  - Native tools: `web_search`, `file_search`, `computer_use`, `code_interpreter`
- Current Go code already has `GetResponse`, `CancelResponse`, `ListResponseInputItems` working via `assistantsProxy()` — only `CreateResponse` is 501
- Python TianjiLLM supports cross-provider bridging (any model via Responses API format)

**Phase 1 (Quick Win)**: Replace CreateResponse 501 stub with `assistantsProxy()` call — ~5 lines
**Phase 2 (Full Parity)**: Implement Responses↔ChatCompletions bidirectional format translation + event stream state machine

---

## Decision 4: Pass-through Endpoint Architecture (A1)

**Decision**: Dynamic route registration from config, reusing existing `forwardToProvider()`

**Rationale** (from codebase analysis):
- `config.PassThroughEndpoint` struct already fully defined (Path, Target, Headers)
- `handler.forwardToProvider()` already implements HTTP forwarding (copy body, set auth, relay response)
- `server.go:154` has explicit insertion point with 501 stub
- Only missing: loop in `setupRoutes()` to iterate `config.PassThroughEndpoints` and register handlers

**Estimated Effort**: ~40 lines

---

## Decision 5: SSO Implementation Completeness (A3)

**Decision**: Wire existing code — implementation is already complete

**Rationale** (from codebase analysis):
- `internal/auth/sso.go` has full OIDC authorization code flow (LoginURL, ExchangeCode, GetUserInfo, MapRole)
- `internal/proxy/handler/sso.go` has complete HTTP handler layer with CSRF protection
- `server.go:384-388` routes already mounted (`/sso/login`, `/sso/callback`)
- Handlers check `h.SSOHandler == nil` and return 501 — fail-safe design
- Only missing: SSO config fields in `GeneralSettings` + wire code in `cmd/tianji/main.go`

**Estimated Effort**: ~50 lines (config struct + wire)

---

## Decision 6: General Fallback Architecture (A4)

**Decision**: Add `Fallbacks` + `DefaultFallbacks` to `RouterSettings`, implement `GeneralFallback()` method

**Rationale** (from codebase analysis):
- Config already parses `fallbacks` and `default_fallbacks` in both `TianjiLLMSettings` and `RouterSettings`
- `ContextWindowFallback()` provides exact pattern to follow (iterate fallback list, call `Route()` for each)
- `Route()` currently returns error when all deployments of a model fail — natural hook point
- Handler layer (`chat.go`) calls `Route()` — can add fallback retry after error

**Estimated Effort**: ~60 lines

---

## Decision 7: Model Group Alias (A5)

**Decision**: Add field to `router.RouterSettings`, insert 2-line lookup before deployment selection

**Rationale** (from codebase analysis):
- `config.go:217` already has `ModelGroupAlias map[string]any`
- `router.Route()` line 80 (`allDeployments := r.deployments[modelName]`) is exact insertion point
- `router.RouterSettings` struct missing the field — needs to be added and wired

**Estimated Effort**: ~15 lines

---

## Decision 8: LLM Response Caching Integration (F2)

**Decision**: Integrate cache Get/Set into chat handler using request content hash as key

**Rationale** (from codebase analysis):
- Cache infrastructure is production-ready: memory, Redis, Redis Cluster, dual, S3, GCS, Azure Blob, disk, semantic (vector similarity via Redis Stack)
- `Handlers.Cache` field exists but `chat.go` has zero cache calls
- Cache key strategy: SHA256 of (model + sorted messages JSON) — matches Python TianjiLLM approach
- Streaming responses: cache assembled result after stream completes, not during

**Estimated Effort**: Medium — ~80 lines in chat handler (key gen + pre-check + post-store + streaming assembly)

---

## Codebase State Summary

### "Code Exists but Not Wired" (Quick Wins)

| Gap | What Exists | What's Missing | Lines |
|-----|-------------|---------------|-------|
| A1 Pass-through | `passthrough/router.go` has full `httputil.ReverseProxy` + guardrail pre/post-call + usage logging + provider-specific auth | Wire `passthrough.Router.Handler()` to `server.go` routes + config→endpoint mapping | ~30 |
| A2 Responses API (CreateResponse) | GET/Cancel/List working via proxy | Replace 501 with proxy call | ~5 |
| A3 SSO/OIDC | Full auth flow + handlers + routes | Config fields + wire code | ~50 |
| A5 Model Group Alias | Config field parsed | RouterSettings field (use `ModelGroupAliasItem{Model, Hidden}` struct per Python's `RouterModelGroupAliasItem`) + Route() lookup + `/v1/models` hidden filter | ~25 |
| B6 Tag match_any | PickWithTags() exists | Router.Route() integration | ~15 |

### "Infrastructure Ready, Logic Missing" (Medium Effort)

| Gap | Infrastructure | Missing Logic |
|-----|---------------|--------------|
| A4 General Fallback | ContextWindowFallback pattern | GeneralFallback method + handler |
| F2 Response Cache | All cache backends | chat.go Get/Set integration |
| F5 Slack Advanced | Basic throttled sender | Multi-channel + alert types |
| F6 Dynamic Rate Limiter | RPM Lua script reusable | TPM tracking + proportional throttle |
| F8 Model Budget Limiter | Budget middleware exists | Per-model spend tracking |

### "From Scratch" (Large Effort)

| Gap | Nothing Exists | Complexity |
|-----|---------------|------------|
| A6 WebSocket/Realtime | Zero WS code | New dependency + bidirectional proxy |
| B3 Priority Queue | Wrong concept (cron vs queue) | Request priority queue system |
| B4 AutoRouter | Nothing | Semantic routing (optional, P3) |
| F1 Token Counting | Zero | New dependency + model mapping |
| D1 UI Dashboard | Zero | Full backend API surface |

### Plugin Systems Assessment

**Guardrails**: `Guardrail` interface (Name/SupportedHooks/Run). Adding new guardrail = 1 implementation file + 1 case in factory.go switch. 10 existing implementations.

**Callbacks**: `CustomLogger` interface (LogSuccess/LogFailure). Adding new callback = 1 implementation file + 1 case in factory.go switch. 33+ existing implementations. Synchronous broadcast (potential bottleneck).

### Middleware Pipeline Gap

Budget middleware, rate limit middleware, and IP whitelist middleware all **exist as code but are NOT mounted** in `server.go`. The actual runtime pipeline is: `chi.RequestID → chi.RealIP → chi.Recoverer → AuthMiddleware → handler`. This is a separate issue from the spec gaps but worth noting.

---

## Post-Review Corrections (Context7 + Claude Context Verification)

### Correction 1: A1 Pass-through is More Complete Than Initially Assessed

**Original claim**: "Only `forwardToProvider()` exists, need ~40 lines for route registration loop"

**Actual state** (verified via Claude Context):
- `passthrough/router.go` has a full `Router` struct with `Handler() http.HandlerFunc` — complete `httputil.ReverseProxy` implementation with:
  - Provider-specific auth (`setProviderAuth`) for Anthropic, default Bearer
  - Pre-call + post-call guardrail integration
  - Usage logging via `LoggingHandler` interface
  - `RegisterLogger()` for per-provider usage tracking
- `passthrough/handler.go` has a separate standalone `Handler(cfg Config)` (simpler version)
- The `Router` is production-ready — just needs mounting in `server.go`

**Impact**: A1 effort drops from ~40 lines to ~30 lines (config→endpoint mapping + mount call).

### Correction 2: A5 ModelGroupAlias Type is Not a Simple String Map

**Original claim**: "Fix type from `map[string]any` to `map[string]string`"

**Actual Python type** (verified via Claude Context on Python codebase):
```python
class RouterModelGroupAliasItem(TypedDict):
    model: str
    hidden: bool  # if 'True', don't return on `.get_model_list`
```

**Impact**: Go type should be `map[string]ModelGroupAliasItem` where `ModelGroupAliasItem{Model string, Hidden bool}`. The `Hidden` field controls whether the alias appears in `/v1/models` responses. Effort estimate increased from ~15 to ~25 lines.

### Correction 3: A2 Responses API Python Implementation is Richer Than Expected

**Original claim**: "Phase 1 is ~5 lines (proxy call)"

**Actual Python state** (verified via Claude Context on Python codebase):
- Full `ResponsePollingHandler` with Redis-backed state machine
- `background_streaming_task` that processes OpenAI event stream format
- `should_use_polling_for_request()` decision logic
- Initial state object construction per OpenAI Response API spec

**Impact**: Phase 1 (pass-through to OpenAI) is still ~5 lines and correct. But Phase 2 (cross-provider bridge + background polling) is significantly larger than implied. Added note: Phase 2 requires Redis polling infrastructure similar to Python's `ResponsePollingHandler`.

### Library Verification (Context7)

| Library | Context7 ID | Reputation | Benchmark | Verified |
|---------|-------------|------------|-----------|----------|
| `coder/websocket` | `/coder/websocket` | High | 84 | ✅ Accept/Dial/Read/Write all context-aware; wsjson helper for JSON; CloseNow/Close for graceful shutdown |
| `pkoukk/tiktoken-go` | `/pkoukk/tiktoken-go` | High | N/A | ✅ EncodingForModel + GetEncoding; offline loader; NumTokensFromMessages cookbook example in docs |
| `gorilla/websocket` | `/gorilla/websocket` | High | 73.7 | Confirmed lower benchmark score; no context support — decision to use coder/websocket validated |
