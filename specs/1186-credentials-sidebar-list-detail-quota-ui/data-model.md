# Data Model: Credentials quota UI

## CredentialListRow

UI row for `/ui/credentials`.

| Field | Type | Source | Notes |
| --- | --- | --- | --- |
| `CredentialID` | string | `CredentialTable.credential_id` | Used for detail link only. |
| `Name` | string | `CredentialTable.credential_name` | Display name. |
| `Email` | string | safe `credential_info.email` | Fallback `Unknown`. |
| `Status` | string | safe metadata + quota state | Credential health badge. |
| `QuotaSummary` | string | `OpenAIQuotaState` | Unknown-safe summary. |
| `NextResetAt` | time/optional | quota dimensions/reset | Earliest known future reset. |
| `LastRefreshAt` | time/optional | safe metadata | Fallback `Not recorded`. |
| `LastError` | string | safe metadata | Truncated/wrapped; never raw secret. |
| `OrganizationID` | string/optional | `CredentialTable.organization_id` | Safe ID only. |
| `UpdatedAt` | time | `CredentialTable.updated_at` | Table timestamp. |

## CredentialDetailData

View model for `/ui/credentials/{credential_id}`.

| Field | Type | Notes |
| --- | --- | --- |
| `CredentialID` | string | Safe identifier. |
| `Name` | string | Display heading. |
| `Email` | string | Safe account email if known. |
| `CredentialStatus` | enum/string | `active`, `disabled`, `refresh_failed`, `auth_failed_after_refresh`, `unknown`, etc. |
| `QuotaStatus` | enum/string | `allowed`, `rejected`, `exhausted`, `unknown`. |
| `LastRefreshAt` | time/optional | Safe metadata. |
| `LastError` | string | Redacted text only. |
| `DisabledReason` | string | Safe reason code/text. |
| `Dimensions` | `[]QuotaDimensionView` | Request/token quota rows. |
| `QuotaResetAt` | time/optional | Overall quota reset if known. |
| `CreatedAt` | time | Safe DB timestamp. |
| `UpdatedAt` | time | Safe DB timestamp. |

## CredentialSafeInfo

Narrow decode target for `credential_info`.

| Field | Type | Notes |
| --- | --- | --- |
| `Email` | string | Displayable account email. |
| `Scopes` | []string | Optional; display only if safe and useful. |
| `Status` | string | Credential lifecycle health. |
| `LastRefreshAt` | time/optional | From lifecycle refresh/test APIs. |
| `LastError` | string | Must be redacted before display. |
| `DisabledReason` | string | Safe reason code. |

Do not carry arbitrary extra metadata into templates.

## QuotaDimensionView

One quota dimension row.

| Field | Type | Notes |
| --- | --- | --- |
| `Name` | string | `Requests` or `Tokens`. |
| `Limit` | int | Display only when `LimitKnown`. |
| `LimitKnown` | bool | Unknown must stay distinct from zero. |
| `Remaining` | int | Display only when `RemainingKnown`. |
| `RemainingKnown` | bool | Unknown must stay distinct from zero. |
| `ResetAt` | time/optional | Display only when `ResetKnown`. |
| `ResetKnown` | bool | Unknown fallback. |
| `Utilization` | float | `0..1`, display only when `UtilizationKnown`. |
| `UtilizationKnown` | bool | Progress bar guard. |

## Status Derivation

Credential health priority:

1. `credential_info.status = disabled` or non-empty `disabled_reason`
2. `credential_info.status = refresh_failed` or `last_error` with refresh/auth failure reason
3. `credential_info.status = active`
4. fallback `unknown`

Quota status priority:

1. `OpenAIQuotaState.Status` when known (`allowed`, `rejected`, `exhausted`)
2. gated request/token dimension with future reset
3. fallback `unknown`

These statuses are displayed separately to avoid hiding disabled credentials behind stale quota data.

## Secret Exclusion

Forbidden in all view models and rendered HTML:

- `CredentialTable.credential_value`
- decrypted token bundle
- `access_token`
- `refresh_token`
- `id_token`
- bearer strings
- JWT-like strings
- encrypted credential blobs
- fallback API keys
- raw OpenAI account payloads

E2E must assert these fixtures are absent from DOM text and relevant HTML.
