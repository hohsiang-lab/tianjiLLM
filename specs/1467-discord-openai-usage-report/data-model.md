# Data Model: Discord OpenAI Credential Usage Report

## Persistence Impact

No new database table, SQL query, migration, cache key, or durable delivery
record is introduced. The feature reads the existing credential inventory and
uses the existing in-memory/persisted safe usage snapshot behavior.

## Report-period model

| Field | Meaning | Validation / Safety Rule |
|-------|---------|--------------------------|
| `period_start` | UTC half-hour period label for operators | Derived at job start; used only in report content and logs. |
| `period_end` | End of the displayed half-hour window | Must be later than `period_start`; does not imply wall-clock delivery alignment. |
| `part_index` / `part_count` | Ordered message-part identity | Starts at 1; displayed when more than one Discord message is required. |
| `row_count` | Number of credentials represented | Must equal the number of enumerated subscription credentials; zero is valid and produces a clearly labeled report. |

## Credential report row

| Field | Source | Validation / Safety Rule |
|-------|--------|--------------------------|
| `credential_id` | Existing `CredentialTable` | Stable safe identifier; never substitute a token or account identifier. |
| `plan_type` | Safe normalized usage snapshot | Optional; no billing or allowance claim. |
| `status` | Existing snapshot result / safe credential state | One of fresh, stale, backoff, unavailable, disabled, reconnect-required, or another redacted safe state. |
| `primary_window` | Existing normalized snapshot | Optional used percentage and reset information. |
| `weekly_window` | Existing normalized snapshot | Optional used percentage and reset information. |
| `additional_buckets` | Existing normalized snapshot | Zero or more safe name, percentage, and reset triples. |
| `safe_reason` | Existing safe error/status reason | Redact before rendering; do not include raw upstream errors, email, credential name, or organization data. |

## Delivery outcome

| Field | Meaning | Validation / Safety Rule |
|-------|---------|--------------------------|
| `attempted_parts` | Number of message parts attempted | Integer at least 0. |
| `delivered_parts` | Confirmed successful parts | Cannot exceed `attempted_parts`. |
| `skipped` | Whether missing configuration or database access caused no delivery | Never treated as a proxy failure. |
| `failure_reason` | Safe job-level reason | Status/reason code only; never webhook URL, response body, or token-shaped content. |

## State transitions

```text
enumerate credentials
  -> render safe row for each credential (or a zero-credential header)
  -> split report into ordered parts
  -> skipped (no configured destination / no database)
     or send part(s)
       -> delivered
       -> bounded retry
         -> delivered
         -> failed (safe result; next period remains eligible)
```

Credential lifecycle state is read-only in this feature. The report never
enables, disables, refreshes, deletes, reroutes, or otherwise mutates a
credential.
