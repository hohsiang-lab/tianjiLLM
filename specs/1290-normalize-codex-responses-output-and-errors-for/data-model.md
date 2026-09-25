# Data Model: HO-1290 Codex response/error normalization

## CodexBackendResponse

Represents the private/backend Responses-style success body consumed by the adapter.

Fields:

- `id` string: Backend response ID. Maps to `ModelResponse.ID`.
- `object` string: Expected `response`; not returned as-is to chat callers.
- `created_at` number: Unix timestamp. Maps to `ModelResponse.Created`.
- `model` string: Backend model ID. Maps to `ModelResponse.Model`.
- `status` string: Expected `completed` for success.
- `output` array: Generated output items.
- `usage` object: Token usage details.
- `error` object/null: Backend response-level error if generation failed.

Validation:

- Completed success must have at least one assistant text output.
- Non-completed/error states must become structured errors, not empty chat completions.

## CodexOutputMessage

Represents one assistant message item inside `output`.

Fields:

- `type` string: Expected `message`.
- `role` string: Expected `assistant` for generated output.
- `content` array: Content parts.

## CodexOutputText

Represents generated text content.

Fields:

- `type` string: Expected `output_text`.
- `text` string: Text to aggregate into `choices[0].message.content`.

Rules:

- Aggregate text parts in order.
- Ignore non-text parts unless implementation adds a tested mapping.
- Empty aggregate is an adapter error.

## CodexUsage

Maps backend usage to Tianji usage.

Fields:

- `input_tokens` -> `Usage.PromptTokens`
- `output_tokens` -> `Usage.CompletionTokens`
- `total_tokens` -> `Usage.TotalTokens`
- `output_tokens_details.reasoning_tokens` may remain unmapped in this issue unless an existing Tianji field is available.

## UpstreamErrorPayload

Represents safe upstream error bodies from Codex backend.

Fields:

- `error.message` string: Safe caller-facing diagnostic after redaction.
- `error.type` string: Error category.
- `error.code` string: Machine-readable diagnostic, e.g. missing scope or quota code.
- HTTP status: Preserved for actionable statuses.

Rules:

- 401 maps to HTTP 401.
- 403 maps to HTTP 403.
- 429 maps to HTTP 429.
- 5xx may remain 502/503 depending on existing upstream error policy, but must preserve safe message/type/code where useful.
- Token-like strings must be redacted before response/log emission.

## OpenAICompatibleError

Uses existing `model.ErrorResponse`.

Fields:

- `error.message`
- `error.type`
- `error.code`
- `error.llm_provider`
- `error.model`

No schema migration is required for this Todo plan.
