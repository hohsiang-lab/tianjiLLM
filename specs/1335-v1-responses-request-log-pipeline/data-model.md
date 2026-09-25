# Data Model: HO-1335 `/v1/responses` request-log pipeline

## Request Log Row

Represents one real upstream model-work attempt visible through `/ui/logs`.

Fields already expected by the UI pipeline:

- `request_id`: Tianji request id / trace id when available.
- `ts`: request start timestamp.
- `endtime`: request end timestamp for successful spend rows.
- `model`: user-facing model such as `openai/gpt-5.5`.
- `key_hash`: API key hash or equivalent auth key identifier.
- `key_alias`: display alias from `VerificationToken` when available.
- `upstream_token_key`: upstream subscription token key when available.
- `team_id`: caller team id when available.
- `end_user`: caller end user when available.
- `status`: success if no joined error row, failed if an error row exists.
- `spend`: computed cost, zero when unknown/unavailable.
- `prompt_tokens`: prompt/input tokens when derivable.
- `completion_tokens`: completion/output tokens when derivable.
- `total_tokens`: total tokens when derivable.
- `cache_hit`: existing cache status field.
- `error_status_code`: HTTP/upstream status for failures.
- `error_type`: redacted failure category.

## Responses Logging Event

Transient implementation object or helper input used to map `/v1/responses` model work into callback log data.

Suggested fields:

- `model`
- `requestID`
- `startTime`
- `endTime`
- `llmLatency`
- `statusCode`
- `error`
- `usage`
- `openAISubscriptionAttribution`
- `source`: `responses_http` or `responses_websocket`
- `realUpstreamWork`: boolean; false for prewarm/synthetic frames

Rules:

- `realUpstreamWork=false` MUST NOT create a request-log row.
- One Responses logging event with `realUpstreamWork=true` MUST create at most one success or failure row.
- Success with missing usage MUST still create a visible row with zero token counts.

## Responses Usage

Usage values extracted from non-streaming JSON or streaming lifecycle events.

Fields:

- `input_tokens` or prompt-equivalent
- `output_tokens` or completion-equivalent
- `total_tokens`
- cache token details if present and safely mappable

Rules:

- Unknown fields are ignored.
- Missing usage is not a failure.
- Cost calculation follows existing pricing/model normalization; unknown pricing yields visible zero-cost row.

## Error Log

Existing failed-request representation included by UI request-log queries.

Rules:

- Failures must be redacted.
- Raw access tokens, refresh tokens, account ids beyond safe identifiers, bearer headers, encrypted credential payloads, and raw upstream secret-bearing messages must not be persisted.
- Failed Responses requests should be queryable by model, request id, upstream token, status, and time range through existing UI filters.
