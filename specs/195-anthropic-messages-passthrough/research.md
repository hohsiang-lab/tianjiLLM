# Research: Anthropic /v1/messages Passthrough

**Date**: 2026-03-25

## R1: LiteLLM Reference Implementation

**Decision**: Follow LiteLLM's `/v1/messages` endpoint pattern with Go adaptation.

**Rationale**: LiteLLM handles this exact use case — Claude Code CLI sends `POST /v1/messages` with `anthropic-beta` headers, and LiteLLM extracts those headers from the incoming request and merges them into the upstream Anthropic API call. Key code path:

1. `litellm/proxy/anthropic_endpoints/endpoints.py:22-71` — Route handler reads body, calls `base_process_llm_request(route_type="anthropic_messages")`
2. `litellm/llms/anthropic/common_utils.py:520-522` — Extracts `anthropic-beta` from incoming request headers via `_get_user_anthropic_beta_headers(headers.get("anthropic-beta"))`
3. `litellm/llms/anthropic/common_utils.py:458-459` — Merges user beta headers with system betas: `betas.update(user_anthropic_beta_headers)`
4. `litellm/llms/anthropic/common_utils.py:443-452` — OAuth detection: if `sk-ant-oat` prefix → `Authorization: Bearer` + `anthropic-dangerous-direct-browser-access: true` + OAuth beta header

**Go adaptation**: Instead of LiteLLM's full translation pipeline, use `httputil.ReverseProxy` for zero-copy forwarding since the body is already Anthropic-native. Only the headers need manipulation (auth replacement + beta merge).

**Alternatives considered**:
- Full translation approach (like `/v1/chat/completions`) — rejected because body is already Anthropic format, translation would be wasted work
- Pure passthrough via existing `internal/proxy/passthrough/router.go` — rejected because it doesn't extract model from body to resolve upstream API key

## R2: Go Reverse Proxy Pattern

**Decision**: Use `net/http/httputil.ReverseProxy` with custom `Director`.

**Rationale**: This is Go's stdlib solution for HTTP proxying. The `Director` function modifies the request before forwarding (URL rewrite, header manipulation). Streaming SSE works automatically because `ReverseProxy` copies the response byte-by-byte using `io.Copy` with `http.Flusher` detection.

**Key implementation detail**: `ReverseProxy` handles:
- Connection management (keep-alive, close)
- Hop-by-hop header removal
- `X-Forwarded-For` header
- Response streaming via `FlushInterval` or automatic flushing

TianjiLLM already uses this pattern in `internal/proxy/passthrough/router.go:97`.

**Alternatives considered**:
- Manual `http.Client.Do()` + `io.Copy` — more code, same result, misses hop-by-hop handling
- Third-party proxy lib (oxy, traefik) — overkill for single-endpoint forwarding

## R3: Model Resolution from Body

**Decision**: Peek at request body to extract `model` field, then use existing `findModelConfig()` to resolve upstream API key.

**Rationale**: The handler needs to know which upstream API key to use. The model name is in the JSON body (`"model": "claude-sonnet-4-6"`). Read body, extract model, resolve config, then restore body for forwarding.

**Pattern**: `io.ReadAll(r.Body)` → `json.Unmarshal` (partial, only `model` field) → `r.Body = io.NopCloser(bytes.NewReader(body))` to restore for `ReverseProxy`.

This pattern is already used in `internal/proxy/handler/chat.go:33` and `internal/proxy/passthrough/router.go:86`.

## R4: Beta Header Merging

**Decision**: New `MergeBetaHeaders(existing, additional string) string` in `internal/provider/anthropic/oauth.go`.

**Rationale**: Client sends `anthropic-beta: claude-code-20250219,oauth-2025-04-20,...`. When upstream API key is OAuth, we need to ensure `oauth-2025-04-20` is present without duplicating it. Split both strings by comma, deduplicate into a set, rejoin.

LiteLLM does this via `_merge_beta_headers()` in `common_utils.py:36-41` and the `betas` set in `get_anthropic_headers()`.
