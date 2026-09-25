# Feature Specification: Anthropic /v1/messages Passthrough

**Feature Branch**: `195-anthropic-messages-passthrough`
**Created**: 2026-03-25
**Status**: Draft
**Input**: User description: "Add Anthropic /v1/messages passthrough endpoint to support Claude Code OAuth token forwarding with proper header passthrough"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Core Passthrough: Streaming + Non-streaming (Priority: P1)

A developer configures Claude Code CLI with `ANTHROPIC_BASE_URL=https://tianji.hohsiang.com.tw` and a TianjiLLM virtual key. When the CLI sends `POST /v1/messages?beta=true` with Anthropic-native request body and OAuth-specific headers (`anthropic-beta: claude-code-20250219,oauth-2025-04-20,...`), TianjiLLM authenticates the virtual key, replaces the auth with the configured upstream OAuth token, preserves the client's `anthropic-beta` and query params, and forwards the request to `api.anthropic.com`. The response (streaming SSE or non-streaming JSON) is returned to the CLI unmodified.

**Why this priority**: This is the core use case — without it, Claude Code CLI cannot use sonnet/opus models through TianjiLLM because OAuth tokens require `claude-code-20250219` beta header that the current `/v1/chat/completions` translation path drops. Streaming and non-streaming are handled by the same `httputil.ReverseProxy` mechanism, so they are a single story.

**Independent Test**: Send a curl request to `/v1/messages?beta=true` with a valid virtual key and Anthropic-native body including `anthropic-beta: claude-code-20250219,oauth-2025-04-20`. Verify upstream receives the OAuth token, all beta headers, and `?beta=true` query param. Verify a successful response is returned in Anthropic native format.

**Acceptance Scenarios**:

1. **Given** TianjiLLM is configured with an Anthropic OAuth token (`sk-ant-oat01-*`) and a virtual key exists, **When** a client sends `POST /v1/messages?beta=true` with valid virtual key and `anthropic-beta: claude-code-20250219,oauth-2025-04-20`, **Then** the request is forwarded to `api.anthropic.com/v1/messages?beta=true` with the upstream OAuth token as `Authorization: Bearer`, client's `anthropic-beta` headers preserved, and `anthropic-dangerous-direct-browser-access: true` set.
2. **Given** the upstream response is successful (200), **When** the response arrives, **Then** the Anthropic-native JSON response is returned to the client unmodified.
3. **Given** the upstream response is an error (400/401/429/500), **When** the error arrives, **Then** the error response is returned to the client with the upstream status code and Anthropic error format preserved.
4. **Given** a valid streaming request, **When** `"stream": true` is in the body, **Then** the response has `Content-Type: text/event-stream` and SSE events are flushed to the client as they arrive from upstream.
5. **Given** a streaming response is in progress, **When** the client disconnects, **Then** the upstream connection is also closed (handled by stdlib `httputil.ReverseProxy` context cancellation).
6. **Given** a valid non-streaming request, **When** `"stream"` is absent or false, **Then** the complete Anthropic JSON response is returned with the original status code.

---

### User Story 2 - Count Tokens Passthrough (Priority: P2)

`POST /v1/messages/count_tokens` forwards token counting requests to Anthropic for token estimation before sending large requests.

**Why this priority**: Claude Code CLI may use this endpoint to estimate token counts before sending large requests.

**Independent Test**: `curl -X POST "http://localhost:4000/v1/messages/count_tokens" -H "Authorization: Bearer <virtual-key>" -d '{"model":"claude-sonnet-4-6","messages":[...]}'` returns `{"input_tokens": N}`.

**Acceptance Scenarios**:

1. **Given** a valid count_tokens request, **When** sent to `/v1/messages/count_tokens`, **Then** the request is forwarded to Anthropic's `/v1/messages/count_tokens` and the response is returned unmodified.

---

### User Story 3 - Event Logging Stub (Priority: P2)

`POST /api/event_logging/batch` returns `{"status":"ok"}` to prevent Claude Code CLI telemetry 404 errors.

**Why this priority**: Claude Code CLI sends telemetry to this endpoint. Without it, 404 errors pollute logs.

**Independent Test**: `curl -X POST "http://localhost:4000/api/event_logging/batch"` returns 200 + `{"status":"ok"}`.

**Acceptance Scenarios**:

1. **Given** any POST request to `/api/event_logging/batch`, **When** received, **Then** return 200 with `{"status":"ok"}` without forwarding to upstream.

---

### Edge Cases

- What happens when the virtual key is invalid or expired? → Return 401 with TianjiLLM error format before reaching upstream.
- What happens when no Anthropic model config exists in the config file? → Return 404 with a clear error message.
- What happens when the upstream OAuth token is expired? → Forward the upstream 401 error to the client.
- What happens when the request body is not valid JSON? → Return 400 with an error message.
- What happens when the client sends non-Anthropic headers (e.g., `x-custom-header`)? → Forward all headers to upstream (except auth/hop-by-hop), matching LiteLLM passthrough behavior.
- What happens when the request has no `model` field in the body? → Forward as-is and let Anthropic return the error.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept `POST /v1/messages` requests with Anthropic-native request body format.
- **FR-002**: System MUST authenticate the request using TianjiLLM's existing auth middleware (virtual key, master key, or JWT).
- **FR-003**: System MUST extract the model name from the request body and resolve the upstream API key from the config (`model_list` with `anthropic/*` wildcard matching).
- **FR-004**: System MUST replace the client's auth header with the configured upstream API key/OAuth token.
- **FR-005**: System MUST preserve the client's `anthropic-beta` header values and merge them with any OAuth-required beta headers (e.g., `oauth-2025-04-20`).
- **FR-006**: System MUST forward the original query parameters (e.g., `?beta=true`) to the upstream URL.
- **FR-007**: System MUST forward the `anthropic-version` header from the client, or default to `2023-06-01` if absent.
- **FR-008**: System MUST support streaming (SSE) responses by flushing upstream events to the client in real-time.
- **FR-009**: System MUST forward non-streaming responses as-is without body transformation.
- **FR-010**: System MUST return upstream error responses to the client with the original status code and body preserved.
- **FR-011**: System MUST also register the endpoint at the bare path `POST /messages` (without `/v1` prefix) to match SDK behavior.
- **FR-012**: System MUST forward all client headers to upstream (except `Authorization`, `Host`, `Content-Length`), then override auth based on upstream API key type. This ensures `User-Agent`, `x-app`, `X-Stainless-*` and any future Anthropic headers reach upstream unchanged.
- **FR-013**: System MUST provide a `POST /v1/messages/count_tokens` passthrough endpoint that forwards token counting requests to Anthropic.
- **FR-014**: System MUST provide a `POST /api/event_logging/batch` stub endpoint that returns `{"status":"ok"}` to prevent Claude Code CLI telemetry 404 errors.

### Key Entities

- **Upstream OAuth Token**: The `sk-ant-oat01-*` token configured in `model_list` under `api_key` for `anthropic/*` models. Used to authenticate with Anthropic API.
- **Virtual Key**: TianjiLLM's proxy authentication token. Validated by auth middleware, then stripped before forwarding to upstream.
- **anthropic-beta Header**: Comma-separated list of beta feature flags. Client-provided values must be preserved and merged with OAuth-required values.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Claude Code CLI configured with `ANTHROPIC_BASE_URL` pointing to TianjiLLM can successfully send and receive messages using sonnet and opus models via OAuth token.
- **SC-002**: All existing `/v1/chat/completions` functionality remains unaffected (zero regression).
- **SC-003**: Streaming responses arrive at the client within 100ms of upstream emission (no buffering delay).
- **SC-004**: The endpoint correctly handles all Anthropic error codes (400, 401, 403, 429, 500) by forwarding them to the client.

## Assumptions

- The Anthropic OAuth token is configured in `model_list` under `api_key` for anthropic models. The system resolves the correct API key by matching the `model` field in the request body against configured model patterns.
- The existing auth middleware handles virtual key validation before the handler is invoked.
- All client headers are forwarded to upstream (except `Authorization`, `Host`, `Content-Length`), matching LiteLLM's passthrough behavior. Auth is replaced with the upstream API key/token.
- The `?beta=true` query parameter and any other query parameters are forwarded verbatim to upstream.
