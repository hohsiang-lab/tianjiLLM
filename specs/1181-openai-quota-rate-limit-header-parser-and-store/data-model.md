# Data Model: OpenAI Quota/Rate-limit State

## OpenAIQuotaState

Normalized per-credential/account state.

| Field | Type | Notes |
| --- | --- | --- |
| `SubjectID` | string | Credential ID or account ID. Never raw token material. |
| `Status` | enum/string | `unknown`, `allowed`, `rejected`, `exhausted`; `allowed/rejected` may come from mock quota fixtures unless production evidence exists. |
| `RequestLimit` | int | Official `x-ratelimit-limit-requests`; known only when parsed. |
| `RequestRemaining` | int | Official `x-ratelimit-remaining-requests`; known only when parsed. |
| `RequestResetAt` | time | Parse-time plus duration from `x-ratelimit-reset-requests`; zero means unknown. |
| `RequestUtilization` | float | Derived `(limit - remaining) / limit`, clamped `[0,1]`; unknown if limit/remaining absent. |
| `TokenLimit` | int | Official `x-ratelimit-limit-tokens`; known only when parsed. |
| `TokenRemaining` | int | Official `x-ratelimit-remaining-tokens`; known only when parsed. |
| `TokenResetAt` | time | Parse-time plus duration from `x-ratelimit-reset-tokens`; zero means unknown. |
| `TokenUtilization` | float | Derived `(limit - remaining) / limit`, clamped `[0,1]`; unknown if limit/remaining absent. |
| `QuotaResetAt` | time | Optional mock quota reset. |
| `QuotaUtilization` | float | Optional mock utilization, clamped `[0,1]`; unknown if absent. |
| `UpdatedAt` | time | Parse timestamp. |

Implementation may use explicit `Known` booleans instead of sentinel values. Unknown must be distinguishable from zero.

## OpenAIQuotaStore

Store-facing contract.

```text
SetOpenAIQuotaState(subjectID, state)
SetOpenAIQuotaStateClean(subjectID, state) optional
GetOpenAIQuotaState(subjectID, now) -> normalized state, ok
GetAllOpenAIQuotaStates(now) -> map
```

`Get` should normalize expired exhaustion before returning state. If a request or token reset deadline has passed, the exhausted gate for that dimension no longer applies.

## GateDecision

Computed from state at routing time.

| Field | Meaning |
| --- | --- |
| `Available` | True unless rejected/exhausted with future reset. |
| `Reason` | Stable reason code such as `request_exhausted`, `token_exhausted`, `quota_rejected`, or empty. |
| `ResetAt` | Earliest relevant reset for retry-after/all-unusable error. |

## Store Key Rules

- Preferred key: OpenAI subscription credential ID.
- Optional secondary metadata: account ID for diagnostics.
- Forbidden keys: raw bearer token, access token, refresh token, encrypted credential value, JWT, or token hash derived solely for log display.

## State Merge Rules

- New known fields replace old known fields for the same subject.
- Unknown fields in a partial update should not erase still-valid known fields unless the reset deadline has expired.
- Malformed fields are ignored and do not overwrite known state.
- Expired exhausted/rejected gates are cleared on read.
