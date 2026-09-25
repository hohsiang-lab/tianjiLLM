# Contract: Proactive OpenAI Subscription Refresh

## Scheduler Job

Name:

```text
openai_subscription_proactive_refresh
```

Cadence:

```text
5 minutes
```

Candidate buffer:

```text
30 minutes
```

Request-time freshness buffer remains:

```text
5 minutes
```

## Job Result

Internal result for logs/tests:

```json
{
  "scanned": 3,
  "eligible": 2,
  "refreshed": 1,
  "skipped": 1,
  "locked": 0,
  "failed": 0,
  "reconnect_required": 1
}
```

All values are counts. No token material may appear.

## Reconnect-Required Metadata

Safe `credential_info` shape after `refresh_token_invalidated`:

```json
{
  "email": "operator@example.com",
  "scopes": ["openid", "offline_access"],
  "status": "refresh_failed",
  "first_refresh_failed_at": "2026-06-18T00:00:00Z",
  "last_refresh_failed_at": "2026-06-18T00:00:00Z",
  "last_error": "OpenAI session ended; reconnect required",
  "disabled_reason": "refresh_token_invalidated",
  "operator_message": "OpenAI session ended; reconnect required"
}
```

Notes:

- Exact status enum may use a stricter terminal value if implementation updates all routing/UI consumers consistently.
- `first_refresh_failed_at` must not be overwritten while the same failure streak continues.
- `last_error` must be redacted before persistence.

## Route Eligibility

Selectable credential:

```text
status == "" OR status == "active"
AND disabled_reason is empty
AND credential row exists
AND credential is not deleted
AND encrypted bundle validates
```

Non-selectable credential:

```text
status IN ("disabled", "refresh_failed")
OR disabled_reason IN ("refresh_failed", "refresh_token_invalidated", "auth_failed_after_refresh", "operator_disabled")
OR row missing/deleted
```

## Operator Message

Reconnect-required message must be exactly:

```text
OpenAI session ended; reconnect required
```

This text is safe for UI, logs, PR evidence, and Linear comments.
