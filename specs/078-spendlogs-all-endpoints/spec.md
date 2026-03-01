# Feature Specification: SpendLogs Tracking for All Proxy Endpoints

**Feature Branch**: `078-spendlogs-all-endpoints`  
**Created**: 2026-03-01  
**Status**: Draft

## Background

tianjiLLM currently records SpendLogs only for chat completion (`/v1/chat/completions`). Three additional proxy endpoints are missing spend tracking:

- `POST /v1/embeddings` — handled in `internal/proxy/handler/embedding.go`
- `POST /v1/rerank` — handled in `internal/proxy/handler/rerank.go` (passthrough via `forwardToProvider`)
- `POST /v1/completions` (legacy) — handled in `internal/proxy/handler/completion.go` (passthrough via `proxyUpstream`)

This feature adds spend tracking to all three endpoints so administrators have a complete view of API usage and costs across all call types.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Embedding Requests Are Logged (Priority: P1)

An administrator wants to see how much is spent on embedding calls in SpendLogs. Currently embedding requests are silently proxied without any record.

**Why this priority**: Embeddings are commonly used for RAG pipelines and may account for significant token usage. Without tracking, cost attribution is incomplete.

**Independent Test**: Send a `POST /v1/embeddings` request through the proxy. Verify a SpendLogs entry is created with `call_type = "embedding"`, the correct model, token counts, and cost.

**Acceptance Scenarios**:

1. **Given** a valid `/v1/embeddings` request is proxied successfully, **When** the provider returns a response with `usage.prompt_tokens` and `usage.total_tokens`, **Then** a SpendLog entry is created with `call_type = "embedding"`, prompt_tokens, total_tokens, model name, and calculated cost.
2. **Given** the provider returns a non-2xx error, **When** the request fails, **Then** no SpendLog entry is created.
3. **Given** the provider response `usage` fields are missing, **When** parsing fails gracefully, **Then** the response is still forwarded to the client and a SpendLog entry is created with zero token counts.

---

### User Story 2 - Rerank Requests Are Logged (Priority: P1)

An administrator wants to track spend on rerank calls. The rerank endpoint is currently a passthrough that pipes the response body directly to the client with no logging.

**Why this priority**: Rerank (e.g., Jina API) is a billed operation. Without spend tracking, costs are invisible.

**Independent Test**: Send a `POST /v1/rerank` request. Verify a SpendLog entry appears with `call_type = "rerank"` and token count from the provider's `usage.total_tokens`.

**Acceptance Scenarios**:

1. **Given** a valid `/v1/rerank` request is proxied, **When** the provider returns a response with `usage.total_tokens`, **Then** a SpendLog entry is created with `call_type = "rerank"`, total_tokens, model name, and cost.
2. **Given** the rerank response body is buffered to parse usage, **When** the log is recorded, **Then** the complete buffered response is still forwarded to the client unchanged.
3. **Given** the provider returns an error, **When** the request fails, **Then** no SpendLog entry is created.

---

### User Story 3 - Legacy Completion Requests Are Logged (Priority: P2)

An administrator wants spend tracking for legacy `POST /v1/completions` requests. Currently this endpoint is a passthrough without any logging.

**Why this priority**: Some clients still use the legacy completions API. Without tracking, usage is invisible.

**Independent Test**: Send a `POST /v1/completions` request. Verify a SpendLog entry appears with `call_type = "completion"`, prompt_tokens, completion_tokens, and total_tokens.

**Acceptance Scenarios**:

1. **Given** a valid `/v1/completions` request is proxied, **When** the provider returns a response with `usage.prompt_tokens`, `usage.completion_tokens`, and `usage.total_tokens`, **Then** a SpendLog entry is created with `call_type = "completion"` and all three token counts.
2. **Given** the legacy completion response body is buffered, **When** the log is recorded, **Then** the complete response is forwarded to the client unchanged.
3. **Given** the provider returns an error, **When** the request fails, **Then** no SpendLog entry is created.

---

### User Story 4 - CallType Is Dynamic Per Endpoint (Priority: P1)

The existing `SpendRecord.CallType` is hard-coded to `"completion"`. Admins viewing SpendLogs need to distinguish between embedding, rerank, and completion calls.

**Why this priority**: Without correct call_type, all log entries would be mislabeled, making the data misleading.

**Independent Test**: Send one request of each type. Verify SpendLogs has distinct `call_type` values: `"embedding"`, `"rerank"`, and `"completion"`.

**Acceptance Scenarios**:

1. **Given** an embedding request is logged, **When** querying SpendLogs, **Then** the entry has `call_type = "embedding"`.
2. **Given** a rerank request is logged, **When** querying SpendLogs, **Then** the entry has `call_type = "rerank"`.
3. **Given** a legacy completion request is logged, **When** querying SpendLogs, **Then** the entry has `call_type = "completion"`.
4. **Given** a chat completion request is logged (existing behavior), **When** querying SpendLogs, **Then** the entry still has `call_type = "completion"` (no regression).

---

### Edge Cases

- What happens when the provider returns an empty response body for a buffered endpoint (rerank/completion)?
- What happens when `usage` is absent from the provider response?
- What happens when cost calculation returns zero (e.g., unknown model in pricing table)?
- How does the system behave under high concurrency where buffering may increase memory usage?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record a SpendLog entry for every successful `POST /v1/embeddings` request, with `call_type = "embedding"`.
- **FR-002**: System MUST record a SpendLog entry for every successful `POST /v1/rerank` request, with `call_type = "rerank"`.
- **FR-003**: System MUST record a SpendLog entry for every successful `POST /v1/completions` (legacy) request, with `call_type = "completion"`.
- **FR-004**: Each SpendLog entry MUST include: model name, call_type, prompt_tokens (where applicable), completion_tokens (where applicable), total_tokens, and calculated cost.
- **FR-005**: `SpendRecord.CallType` MUST be dynamically set per endpoint (not hard-coded).
- **FR-006**: For `/v1/rerank` and `/v1/completions`, the system MUST buffer the provider response body, parse usage data, call `h.Callbacks.LogSuccess`, and then forward the full buffered response to the client.
- **FR-007**: For `/v1/embeddings`, the system MUST call `h.Callbacks.LogSuccess` after the existing `TransformEmbeddingResponse` step, before returning to the client.
- **FR-008**: Spend tracking MUST NOT alter the response body or status code sent to the client.
- **FR-009**: If usage fields are missing or zero in the provider response, the system MUST still forward the response and MAY create a SpendLog entry with zero token counts (no panic or dropped response).
- **FR-010**: Existing chat completion SpendLog behavior MUST NOT regress.

### Key Entities

- **SpendLog**: A record of API usage and cost per request. Attributes: model, call_type, prompt_tokens, completion_tokens, total_tokens, cost, timestamp, user/team identifiers.
- **TokenUsage**: Parsed token counts from provider response body. Fields vary by endpoint:
  - embedding: prompt_tokens, total_tokens
  - rerank: total_tokens only
  - completion (legacy/chat): prompt_tokens, completion_tokens, total_tokens
- **CallType**: String identifier for endpoint type. Values: `"completion"` (chat + legacy), `"embedding"`, `"rerank"`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of successful embedding, rerank, and legacy completion requests produce a SpendLog entry (zero silent drops).
- **SC-002**: SpendLog entries for each endpoint type carry the correct `call_type` value with no mismatches.
- **SC-003**: Response latency increase due to response buffering is under 50ms for typical payloads (< 1MB response body).
- **SC-004**: Existing SpendLog entries for chat completion are unaffected — zero regression in count, call_type, or token values.
- **SC-005**: Administrators can filter SpendLogs by `call_type` and see distinct cost breakdowns for embedding, rerank, and completion traffic.

## Assumptions

- Provider response `usage` format follows: embedding (`prompt_tokens`, `total_tokens`), rerank (`total_tokens` only, Jina format), legacy completion (`prompt_tokens`, `completion_tokens`, `total_tokens`).
- Cost calculation uses existing `pricing.Default().Cost(modelName, pricing.TokenUsage{...})` — no new pricing logic required.
- `callback.LogData` struct can accommodate a dynamic `CallType` field with a minor extension if not already present.
- Buffering response bodies for rerank and legacy completion is acceptable; streaming for these two endpoints is not currently supported and is out of scope.
- No database schema changes are required beyond ensuring the `call_type` column accepts values beyond `"completion"`.
