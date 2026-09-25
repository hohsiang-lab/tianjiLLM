# Data Model: OpenAI Subscription Routing

## OpenAISubscriptionRouteCandidate

Represents one configured OpenAI subscription credential after loading, refresh, and availability classification.

Fields:

- `CredentialID string`: Stable configured credential ID. Primary routing key.
- `AccountID string`: OpenAI/ChatGPT account ID when available; diagnostic only.
- `Transport openAISubscriptionTransport`: Direct OpenAI HTTP or Codex app-server transport.
- `BearerToken string`: Direct HTTP access token. Must never be logged.
- `CodexLogin *codexapp.LoginStartParams`: Codex app-server login material when transport requires it.
- `UnavailableCode OpenAISubscriptionCredentialErrorCode`: Empty when usable; typed reason when unavailable.
- `UnavailableMessage string`: Sanitized message for aggregate errors.
- `RateLimitState *OpenAISubscriptionRateLimitState`: Last parsed OpenAI rate-limit state, when available.

Rules:

- Disabled credentials are unavailable and excluded before selection.
- Missing/malformed/expired/refresh-failed credentials are retained only for all-unusable error reporting, not for selection.
- Raw access tokens, refresh tokens, encrypted values, and upstream sensitive payloads must not appear in candidate keys, logs, or errors.

## OpenAISubscriptionRateLimitState

Provider-specific in-memory state derived from official OpenAI HTTP response headers.

Fields:

- `CredentialID string`
- `LimitRequests *int64`
- `LimitTokens *int64`
- `RemainingRequests *int64`
- `RemainingTokens *int64`
- `ResetRequestsAt *time.Time`
- `ResetTokensAt *time.Time`
- `UpdatedAt time.Time`
- `LastStatusCode int`

Rules:

- A credential is gated when `RemainingRequests == 0` and `ResetRequestsAt` is in the future.
- A credential is gated when `RemainingTokens == 0` and `ResetTokensAt` is in the future.
- Expired reset times clear the gate.
- Missing headers do not update the corresponding fields and do not create a new gate.
- Malformed header values are ignored for gating and may be logged only without secrets.

## OpenAISubscriptionStickyEntry

Tracks sticky credential selection for one org/model route class.

Fields:

- `CredentialID string`
- `SelectedAt time.Time`
- `RateLimitResetAt *time.Time`: Optional reset deadline at selection time when known.

Rules:

- Sticky reuse is valid only while `CredentialID` remains in the available candidate set.
- Sticky selection re-evaluates when the selected credential becomes disabled, unusable, or gated.
- Sticky state is in-memory and handler-local, matching current native upstream state durability.

## OpenAISubscriptionRoutingKey

Stable key used for round-robin counters and sticky tracks.

Components:

- Provider namespace: `openai-subscription`.
- Org scope: `org:<orgID>` or deterministic master/no-org fallback.
- Model/route class: resolved model or model class chosen by implementation.
- Strategy namespace when needed to avoid counter/sticky collisions.

Example:

```text
openai-subscription:org:org_acme:model:gpt-5.1
openai-subscription:org:__master__:model:gpt-5.1
```

## OpenAISubscriptionRoutingError

Sanitized aggregate error returned when no configured credential can serve the request.

Fields:

- `Model string`
- `Transport openAISubscriptionTransport`
- `Reasons []OpenAISubscriptionCredentialFailure`
- `RetryAfter *time.Time`

`OpenAISubscriptionCredentialFailure` fields:

- `CredentialID string`
- `Code OpenAISubscriptionCredentialErrorCode`
- `Message string`

Rules:

- Reasons must preserve typed codes useful for operators.
- Messages must be redacted and must not include raw token material.
- `api_key` fallback must not be attempted after this error is produced.

## State Ownership

- Credential truth remains in the existing credential DB.
- Token refresh remains owned by existing OpenAI subscription refresh manager.
- OpenAI rate-limit and sticky routing state are in-memory fields on `Handlers`.
- No schema migration is required for HO-1179.
