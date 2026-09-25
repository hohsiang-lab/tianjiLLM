# Data Model: HO-1340 Codex usage snapshot

## CodexUsageSnapshot

Normalized safe snapshot for one OpenAI subscription credential.

Fields:

- `credential_id`: Tianji credential id.
- `email`: account email from upstream when present.
- `plan_type`: upstream plan type when present.
- `status`: `fresh`, `stale`, `backoff`, `auth_error`, `rate_limited`, `upstream_error`, `unavailable`, or `no_data`.
- `primary_window`: `CodexUsageWindow`.
- `weekly_window`: `CodexUsageWindow`.
- `additional_buckets`: list of `CodexUsageBucket`.
- `credits_status`: optional normalized credit/spend-control status.
- `fetched_at`: timestamp of last successful fetch.
- `expires_at`: cache expiry.
- `last_success_at`: timestamp of last successful fetch.
- `backoff_until`: optional timestamp when refresh is deferred.
- `last_error_reason`: safe reason code only.

Forbidden fields:

- access token
- refresh token
- id token
- cookie
- authorization header
- raw upstream request headers
- raw upstream response body
- encrypted credential value

## CodexUsageWindow

Normalized usage window.

Fields:

- `name`: stable label such as `primary_5h` or `weekly`.
- `used_percent`: optional float in `[0, 1]`.
- `reset_at`: optional timestamp or normalized upstream reset string.
- `status`: safe status string when present.
- `limit_label`: optional upstream limit label if non-secret.
- `remaining_label`: optional derived or upstream remaining label if non-secret.

## CodexUsageBucket

Additional usage bucket such as `GPT-5.3-Codex-Spark`.

Fields:

- `name`: upstream bucket/model label, redacted/sanitized.
- `used_percent`: optional float in `[0, 1]`.
- `reset_at`: optional timestamp or normalized reset string.
- `status`: safe status string when present.

## CodexUsageCacheEntry

Runtime cache/backoff entry by credential id.

Fields:

- `credential_id`
- `snapshot`: optional `CodexUsageSnapshot`.
- `fetched_at`
- `expires_at`
- `backoff_attempt`
- `backoff_until`
- `last_error_reason`
- `last_success_at`

Rules:

- TTL must be at least 60 seconds.
- 429/5xx increments backoff attempt and preserves last successful snapshot.
- A successful fetch resets backoff attempt.
- Manual refresh must not bypass active backoff unless future owner scope explicitly approves forced override.

## CodexUsageViewData

UI-safe view model.

Fields:

- `CredentialID`
- `CredentialName`
- `Email`
- `OrganizationID`
- `PlanType`
- `StatusLabel`
- `StatusVariant`
- `PrimaryWindow`
- `WeeklyWindow`
- `AdditionalBuckets`
- `CreditsStatus`
- `LastUpdated`
- `CacheState`
- `SafeError`
- `RefreshDisabledReason`

Rules:

- Render only normalized safe fields.
- Missing data must render as no-data/stale state, not panic.
- Token-like substrings in safe errors must be redacted before rendering.
