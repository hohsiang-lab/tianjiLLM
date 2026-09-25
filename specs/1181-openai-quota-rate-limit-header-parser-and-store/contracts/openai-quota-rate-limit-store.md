# Contract: OpenAI Quota/Rate-limit Parser and Store

## Parser Input

`ParseOpenAIQuotaHeaders(headers http.Header, subjectID string, now time.Time)` accepts response headers from direct official OpenAI HTTP attempts.

Official headers:

- `x-ratelimit-limit-requests`
- `x-ratelimit-limit-tokens`
- `x-ratelimit-remaining-requests`
- `x-ratelimit-remaining-tokens`
- `x-ratelimit-reset-requests`
- `x-ratelimit-reset-tokens`

Mock-only quota headers may be added under `internal/testutil/openaitest` for allowed/rejected/reset/utilization fixtures. They must be named and documented as test fixtures, not official OpenAI API contract.

## Parser Output

The parser returns `(OpenAIQuotaState, updated bool)`.

- `updated=false` when no recognized valid headers are present.
- Valid partial headers produce `updated=true` and update only known fields.
- Malformed values are ignored.
- Utilization is derived only with positive limit and known remaining.
- Reset duration strings are relative to `now`.

## Store Update Contract

All direct official OpenAI subscription attempts must call the parser before deciding whether to return, retry, or discard the response when headers are available.

Required attempt surfaces:

- Successful 2xx responses.
- 401 first attempts and refreshed retries.
- 429 responses before failover.
- Retryable 5xx responses before failover.

Streaming caveat: after response body bytes are committed to the client, no failover is allowed. Header parsing before commitment is still allowed.

## Routing Consumption Contract

OpenAI subscription routing must use the normalized store for:

- Candidate gating.
- Sticky re-evaluation.
- Lowest-utilization scoring.
- All-unusable retry/reset diagnostics.

No routing consumer may read a separate handler-private map if a normalized store exists for the same state.

## Safe Degradation Contract

- Missing headers do not gate.
- Unknown utilization is not zero utilization.
- Remaining zero without a future reset updates state but does not create an indefinite gate.
- Expired reset windows clear the gate.
- No local billing/quota estimate is generated.

## Security Contract

The parser, store, logs, errors, and diagnostics must not include:

- Access tokens.
- Refresh tokens.
- Bearer tokens.
- JWT-like token strings.
- Encrypted credential values.
- Full sensitive upstream payloads.
