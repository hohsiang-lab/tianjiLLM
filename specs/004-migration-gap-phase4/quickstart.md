# Quickstart: Phase 4 Implementation

**Branch**: `004-migration-gap-phase4` | **Date**: 2026-02-17

## Prerequisites

- Go 1.22+
- PostgreSQL running (for existing DB-backed features)
- Redis running (for cache integration testing)
- `make check` passes on current `main` branch

## Wave 1: Wire Existing Code (Start Here)

Wave 1 has zero new dependencies and touches only existing files. Each task is independent — can be done in any order.

### A5 — Model Group Alias (Smallest, start here)

1. Add `ModelGroupAlias map[string]string` to `internal/router/router.go:RouterSettings`
2. Fix type in `internal/config/config.go:217` from `map[string]any` to `map[string]string`
3. Wire in `cmd/tianji/` config → router settings
4. Add 2 lines in `router.Route()` before `r.deployments[modelName]` lookup:
   ```go
   if alias, ok := r.settings.ModelGroupAlias[modelName]; ok {
       modelName = alias
   }
   ```
5. Test: `go test ./internal/router/... -run TestModelGroupAlias -v`

### A2 — Responses API CreateResponse

1. In `internal/proxy/handler/responses.go`, replace 501 block with `h.assistantsProxy(w, r)`
2. Test: `go test ./test/contract/... -run TestCreateResponse -v`

### B6 — Tag match_any

1. Add `hasAnyTag()` function in `internal/router/strategy/tag.go`
2. In `TagBased.PickWithTags()`, use `hasAnyTag` when `matchAny` is true
3. In `router.Route()`, call `PickWithTags()` when `EnableTagFiltering` is set
4. Test: `go test ./internal/router/strategy/... -run TestTagMatchAny -v`

### A1 — Pass-through Endpoints

1. In `internal/proxy/server.go:setupRoutes()`, add loop after line 154:
   ```go
   for _, pt := range s.config.PassThroughEndpoints {
       r.HandleFunc(pt.Path+"/*", s.Handlers.PassThrough(pt))
   }
   ```
2. Implement `PassThrough(pt config.PassThroughEndpoint)` handler using existing `forwardToProvider()`
3. Test: `go test ./test/contract/... -run TestPassThrough -v`

### A3 — SSO Config Wiring

1. Add SSO fields to `internal/config/config.go:GeneralSettings`
2. In `cmd/tianji/main.go`, construct `auth.NewSSOHandler()` from config
3. Inject into `Handlers.SSOHandler`
4. Test: `go test ./test/contract/... -run TestSSO -v`

## Wave 2: Add Logic (After Wave 1 passes)

### A4 — General Fallback

1. Add `Fallbacks`, `DefaultFallbacks` to `router.RouterSettings`
2. Add `GeneralFallback(modelName string, req *model.ChatCompletionRequest)` method
3. In chat handler, after `Route()` returns error, call `GeneralFallback()`
4. Test with config: `fallbacks: [{"gpt-4": ["claude-3"]}]`

### F2 — Response Caching

1. In `internal/proxy/handler/chat.go:handleNonStreamingCompletion`:
   - Before provider call: compute cache key, check `h.Cache.Get(key)`
   - After provider response: `h.Cache.Set(key, response, ttl)`
2. Cache key: `SHA256(model + json.Marshal(messages))`
3. Test: send same request twice, verify second is cache hit

### F1 — Token Counting (Wave 3 dependency: tiktoken-go)

1. `go get github.com/pkoukk/tiktoken-go`
2. Create `internal/token/counter.go` with `CountMessages(model, messages)`
3. Integrate into budget pre-check and TPM rate limiter

### A6 — WebSocket Realtime (Wave 3 dependency: coder/websocket)

1. `go get github.com/coder/websocket`
2. Create `internal/proxy/handler/realtime.go`
3. Mount at `/v1/realtime` in `server.go`
4. Test: WebSocket client → proxy → mock upstream WebSocket server

## Verification

After each wave:

```bash
make check          # lint + test + build (must pass)
go test ./... -race # race detector (must pass)
```

After all waves:
```bash
# Verify config compatibility
make run            # with existing proxy_config.yaml
# Verify no regressions
go test ./test/contract/... -v
go test ./test/integration/... -v
```
