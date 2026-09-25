# Contract: OpenAI Subscription Credential Routing

## Applicability

This contract applies when a Tianji model config contains one or more `openai_subscription_credential_ids`.

When the list is empty, existing OpenAI API-key behavior applies and this contract does not alter it.

## Candidate Construction

Given a configured list:

```yaml
openai_subscription_credential_ids:
  - cred_a
  - cred_b
  - cred_c
```

The router must evaluate all IDs in config order.

For each ID:

- Load credential from DB.
- Verify credential type is OpenAI subscription.
- Exclude disabled credentials.
- Resolve/refresh usable token material for the requested transport.
- Preserve sanitized typed failure reason if unusable.

The candidate set for selection contains only usable and not-currently-gated credentials.

## Strategy Contract

### `round_robin`

- Rotate across available candidates.
- Counter is scoped to OpenAI subscription routing key, not shared with Anthropic upstream counters unless implementation intentionally generalizes the primitive.

### `sticky`

- Reuse the selected credential while it remains available.
- Sticky state is scoped by org and model/route class.
- Different orgs must not share sticky selections.
- If sticky credential leaves the available set, select a replacement from available candidates.

### `lowest_utilization`

- Use OpenAI request/token remaining and reset state only when parsed from OpenAI headers.
- Do not apply Anthropic 5h/7d/sonnet utilization fields to OpenAI.
- When all available candidates have unknown OpenAI rate-limit state, use deterministic fallback behavior.

## Rate-limit Gate Contract

Parse these response headers when present:

- `x-ratelimit-limit-requests`
- `x-ratelimit-limit-tokens`
- `x-ratelimit-remaining-requests`
- `x-ratelimit-remaining-tokens`
- `x-ratelimit-reset-requests`
- `x-ratelimit-reset-tokens`

Gate a credential when:

- remaining requests is `0` and request reset is in the future, or
- remaining tokens is `0` and token reset is in the future.

Do not gate when:

- headers are absent,
- remaining values are positive,
- reset time has already passed,
- only malformed header values are present.

## Failover Contract

One client request may attempt at most one request per configured usable credential.

Failover is allowed for:

- credential load/refresh failure before upstream dispatch,
- direct OpenAI HTTP 429 before response body relay,
- direct OpenAI HTTP retryable 5xx before response body relay,
- streaming non-2xx response before any bytes are relayed.

Failover is not allowed for:

- streaming errors after bytes have been relayed,
- client request body read failures,
- non-retryable 4xx responses other than rate-limit responses,
- all candidates already attempted or unavailable.

## Error Contract

When no credential can serve the request, return an explicit OpenAI subscription routing error.

The error must include:

- model or route context,
- configured credential IDs,
- sanitized typed reason code for each unusable credential,
- retry-after/reset hint when available.

The error must not include:

- access tokens,
- refresh tokens,
- bearer tokens,
- encrypted credential values,
- raw upstream sensitive payloads.

## No-fallback Contract

If `openai_subscription_credential_ids` is non-empty, `api_key` must not be used for that request even if:

- all subscription credentials are disabled,
- all subscription credentials fail refresh,
- all subscription credentials are gated,
- the database is unavailable,
- upstream returns 429/5xx for every credential.
