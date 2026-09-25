# Implementation Plan: Anthropic /v1/messages Passthrough

**Branch**: `195-anthropic-messages-passthrough` | **Date**: 2026-03-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/195-anthropic-messages-passthrough/spec.md`

## Summary

Add Anthropic-native passthrough endpoints to tianjiLLM:
- `POST /v1/messages` — core message passthrough (streaming + non-streaming)
- `POST /v1/messages/count_tokens` — token counting passthrough
- `POST /api/event_logging/batch` — telemetry stub (prevents Claude Code CLI 404s)

This unblocks Claude Code CLI usage through the proxy with OAuth tokens that require `claude-code-20250219` beta header.

**Approach**: Follow LiteLLM's `/anthropic/{endpoint}` passthrough pattern — forward **all** client headers (replacing only auth), preserve query params, forward body unchanged. Use `httputil.ReverseProxy` for zero-copy SSE forwarding. Extract `model` from body to resolve upstream API key from config.

**Key design decision**: Forward all client headers (except `Authorization`, `Host`, `Content-Length`) rather than whitelisting specific Anthropic headers. This matches LiteLLM's passthrough behavior and ensures future Anthropic header additions work without code changes.

## Technical Context

**Language/Version**: Go 1.24.4
**Primary Dependencies**: chi/v5 (router), net/http/httputil (reverse proxy), encoding/json (body parsing)
**Storage**: N/A (stateless passthrough)
**Testing**: `go test` + `testify` + `httptest.NewServer` (mock upstream)
**Target Platform**: Linux server (Kubernetes)
**Project Type**: Single Go module
**Performance Goals**: <100ms added latency over upstream
**Constraints**: Must not break existing `/v1/chat/completions` path
**Scale/Scope**: Handles all Claude Code CLI traffic (~100 req/min per user)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | PASS | LiteLLM's `/anthropic/{endpoint}` passthrough and `/v1/messages` endpoint both studied. We follow the passthrough pattern (forward all headers, replace auth, preserve query params). |
| II. Feature Parity | PASS | Replicates LiteLLM's passthrough behavior: all-header forwarding, auth replacement, body passthrough, streaming SSE, event_logging stub. |
| III. Research Before Build | PASS | See research.md — cross-checked against LiteLLM source. |
| IV. Failing-Tests-First | PASS | Tests defined below, will be implemented before feature code. |
| V. Simplicity-First | PASS | Single handler + ReverseProxy Director. No new abstractions. |
| VI. Go Idioms | PASS | Uses stdlib `httputil.ReverseProxy`, `http.Flusher`, `io.Copy`. |
| VII. sqlc Pipeline | N/A | No database queries in this feature. |

## Project Structure

### Documentation (this feature)

```text
specs/195-anthropic-messages-passthrough/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal — stateless feature)
├── quickstart.md        # Phase 1 output
└── contracts/
    └── anthropic-messages.md  # API contract
```

### Source Code (repository root)

```text
internal/proxy/handler/
├── anthropic_messages.go          # NEW: /v1/messages + /v1/messages/count_tokens handler
├── anthropic_messages_test.go     # NEW: tests
└── anthropic_event_logging.go     # NEW: /api/event_logging/batch stub

internal/provider/anthropic/
├── oauth.go                       # MODIFY: add MergeBetaHeaders()
└── oauth_test.go                  # MODIFY: add tests for MergeBetaHeaders()

internal/proxy/server.go           # MODIFY: register new routes
```

**Structure Decision**: Follows existing handler pattern (`internal/proxy/handler/`) with provider-specific helpers in `internal/provider/anthropic/`.

## Header Forwarding Strategy

Matches LiteLLM's `/anthropic/{endpoint}` passthrough (`forward_headers=True`):

```
Client headers → Forward ALL (copy from incoming request)
  EXCEPT: Remove Authorization, Host, Content-Length (hop-by-hop)
  THEN: Set auth based on upstream API key type:
    - OAuth token (sk-ant-oat*): Authorization: Bearer + anthropic-dangerous-direct-browser-access: true
    - API key (sk-ant-api*): x-api-key: <key>
  THEN: Merge anthropic-beta (client betas + OAuth beta, deduped)
  THEN: Set anthropic-version from client or default "2023-06-01"
```

This ensures `User-Agent: claude-cli`, `x-app: cli`, `X-Stainless-*` headers all reach Anthropic unchanged.

## Failing Tests

### User Story 1 Tests — Core Passthrough (Streaming + Non-streaming)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAnthropicMessages_ForwardsBodyUnmodified` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream receives exact request body from client | AS-1.1 |
| `TestAnthropicMessages_ReplacesAuthWithUpstreamOAuth` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream receives `Authorization: Bearer sk-ant-oat01-...`, not client virtual key | AS-1.1 |
| `TestAnthropicMessages_PreservesClientBetaHeaders` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream `anthropic-beta` contains all client-provided betas + `oauth-2025-04-20` | AS-1.1 / FR-005 |
| `TestAnthropicMessages_ForwardsAllClientHeaders` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream receives `User-Agent`, `x-app`, `X-Stainless-*` from client | FR-012 |
| `TestAnthropicMessages_ForwardsQueryParams` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream URL includes `?beta=true` from client | AS-1.1 / FR-006 |
| `TestAnthropicMessages_ForwardsAnthropicVersion` | `internal/proxy/handler/anthropic_messages_test.go` | Upstream gets client's `anthropic-version` or default `2023-06-01` | FR-007 |
| `TestAnthropicMessages_SuccessResponse` | `internal/proxy/handler/anthropic_messages_test.go` | Client receives 200 + Anthropic JSON body unmodified | AS-1.2 |
| `TestAnthropicMessages_ErrorResponse` | `internal/proxy/handler/anthropic_messages_test.go` | Client receives upstream error status + Anthropic error body | AS-1.3 |
| `TestAnthropicMessages_StreamingSSE` | `internal/proxy/handler/anthropic_messages_test.go` | Response has `Content-Type: text/event-stream` and SSE events forwarded | AS-1.4 |
| `TestAnthropicMessages_NonStreaming` | `internal/proxy/handler/anthropic_messages_test.go` | Complete JSON response returned when stream is false/absent | AS-1.6 |

### User Story 2 Tests — Count Tokens

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAnthropicMessages_CountTokensPassthrough` | `internal/proxy/handler/anthropic_messages_test.go` | `/v1/messages/count_tokens` forwards to upstream and returns response | AS-2.1 |

### User Story 3 Tests — Event Logging Stub

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAnthropicEventLogging_ReturnsOK` | `internal/proxy/handler/anthropic_messages_test.go` | `/api/event_logging/batch` returns `{"status":"ok"}` without hitting upstream | AS-3.1 |

### Edge Case Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestAnthropicMessages_InvalidAuth` | `internal/proxy/handler/anthropic_messages_test.go` | Returns 401 before reaching upstream | Edge: invalid virtual key |
| `TestAnthropicMessages_NoModelConfig` | `internal/proxy/handler/anthropic_messages_test.go` | Returns 404 when no anthropic config found | Edge: no config |
| `TestAnthropicMessages_InvalidJSON` | `internal/proxy/handler/anthropic_messages_test.go` | Returns 400 for malformed body | Edge: invalid JSON |
| `TestAnthropicMessages_RegularAPIKey` | `internal/proxy/handler/anthropic_messages_test.go` | Non-OAuth key uses `x-api-key` header instead of `Authorization: Bearer` | Edge: non-OAuth auth |
| `TestMergeBetaHeaders_NoDuplicates` | `internal/provider/anthropic/oauth_test.go` | Merging `"oauth-2025-04-20,foo"` with `"oauth-2025-04-20"` produces no duplicate | FR-005 |
| `TestMergeBetaHeaders_Empty` | `internal/provider/anthropic/oauth_test.go` | Empty client header + OAuth header → just OAuth header | FR-005 |

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./internal/proxy/handler/... -run "TestAnthropicMessages|TestAnthropicEventLogging" -v
go test ./internal/provider/anthropic/... -run "TestMergeBetaHeaders" -v
```

## Future TODO (not in scope)

- **Spend logging**: Extract `usage.input_tokens` / `usage.output_tokens` from upstream response for cost tracking. Requires response body interception in `ModifyResponse` (non-streaming) or SSE event parsing (streaming). Defer to follow-up PR.
- **Model routing via Router**: Current plan resolves API key from config directly. If multi-deployment load balancing is needed for passthrough, integrate with `internal/router/`.

## Complexity Tracking

No constitution violations. All design follows existing patterns with stdlib tools.
