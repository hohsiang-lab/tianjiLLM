# Contract: Discord Webhook Credential Usage Report

## Scope

This is an outbound-only contract from Tianji to the existing configured
Discord webhook destination. It does not add a Tianji public API, incoming
Discord interaction, or new configuration surface.

## Request

For every ordered report part, Tianji sends an HTTP `POST` to the configured
webhook with the `wait=true` query parameter.

```json
{
  "content": "Tianji OpenAI credential usage ...",
  "allowed_mentions": {
    "parse": []
  }
}
```

### Contract rules

- `content` is plain text and at most 2,000 characters after the period and
  part prefix is added.
- A multi-message report uses `part n/m` in each part and preserves row order.
- `allowed_mentions.parse` is always empty; report content must not trigger
  user, role, or broadcast mentions.
- The request body contains only safe report fields.
- The report identifies a credential only by its credential ID; it excludes
  email, credential name, and organization data.
- A configured empty credential inventory sends one clearly labeled
  zero-credential report part rather than skipping delivery as no work.
- The configured URL is never included in report content, logs, errors,
  metrics, or test output.

## Response handling

| Response / outcome | Behavior |
|--------------------|----------|
| 2xx | Mark the part delivered. |
| 429 with usable `retry_after` | Wait only within the remaining 60-second job budget, then retry once. |
| Transport failure, 5xx, other non-2xx, or exhausted retry | Mark delivery failed with a sanitized status/reason; do not affect proxy traffic. |
| Missing webhook configuration | Do not send; return a safe skipped result. |

The sender makes at most two attempts per report part, with the second attempt
reserved for a confirmed rate-limit response. It is best-effort; an unknown
transport outcome is not retried and must not be described as exactly-once
delivery.

## Internal job boundary

The scheduler job has the stable name:

```text
openai_subscription_discord_usage_report
```

It invokes one report run every 30 minutes after scheduler start. A
multi-replica deployment uses Tianji's existing distributed scheduler lock so
only the lock holder starts that run.
