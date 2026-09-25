# Data Model: OpenAI Subscription Spend and Audit Attribution

## Existing Tables

### SpendLogs

No schema change is planned by default.

Relevant existing fields:

| Column | HO-1182 Use |
| --- | --- |
| `api_key` | Existing client virtual key hash / API-key attribution. Must not store subscription bearer or credential ID. |
| `metadata` | Store safe subscription attribution object. |
| `provider` | Existing provider column; should be `openai` for official OpenAI subscription traffic. |
| `organization_id` | Existing request organization attribution when available. |
| `upstream_token_key` | Existing upstream token attribution; do not overload with OpenAI credential ID unless implementation deliberately preserves semantics. |

Recommended metadata shape:

```json
{
  "openai_subscription": {
    "credential_id": "cred_a",
    "provider": "openai",
    "organization_id": "org_123",
    "account_id": "acct_redacted_or_safe_id",
    "status": "success"
  }
}
```

`account_id` is optional and must be included only if treated as non-secret by existing subscription persistence/redaction rules.

### ErrorLogs

No schema change is planned.

Relevant existing fields:

| Column | HO-1182 Use |
| --- | --- |
| `api_key_hash` | Existing client token hash. Must not store subscription bearer. |
| `provider` | Existing provider attribution. |
| `organization_id` | Existing org attribution. |
| `error_message` | Redacted safe error text only. |
| `upstream_token_key` | Existing upstream token attribution; do not use for raw bearer material. |

Failure attribution should be present in redacted message or callback/audit metadata as stable reason codes, not raw upstream body text.

### AuditLog

No schema change is planned.

Relevant existing fields:

| Column | HO-1182 Use |
| --- | --- |
| `action` | Store lifecycle action such as `connect`, `refresh`, `delete`, `test`, `disable`. |
| `table_name` | Recommended `CredentialTable` for credential lifecycle events. |
| `object_id` | Subscription `credential_id` when available. |
| `before_value` | Optional safe previous metadata. |
| `updated_values` | Safe action/status payload. |

Recommended `updated_values` shape:

```json
{
  "credential_id": "cred_a",
  "provider": "openai",
  "organization_id": "org_123",
  "action": "refresh",
  "status": "failure",
  "reason_code": "refresh_failed",
  "request_id": "req_123",
  "metadata": {
    "last_refresh_at": "2026-05-07T15:00:00Z",
    "last_error": "[REDACTED or safe reason]"
  }
}
```

## Runtime Types

### OpenAISubscriptionAttribution

Recommended internal shape:

```go
type OpenAISubscriptionAttribution struct {
    CredentialID   string
    Provider       string
    OrganizationID string
    AccountID      string
    Action         string
    Status         string
    ReasonCode     string
    RequestID      string
    Metadata       map[string]any
}
```

Allowed values:

- `Provider`: `openai`
- `Action`: `request`, `connect`, `refresh`, `delete`, `test`, `disable`
- `Status`: `success`, `failure`, `refresh_failed`, `auth_failed_after_refresh`, `rate_limited`, `disabled`
- `ReasonCode`: stable codes from subscription credential/routing errors

Secret fields are not allowed in this type.

### callback.LogData Extension

Recommended addition:

```go
type LogData struct {
    OpenAISubscription *OpenAISubscriptionAttribution
}
```

Equivalent field names are acceptable if tests prove the same behavior.

### SpendRecord Extension

Recommended addition:

```go
type SpendRecord struct {
    OpenAISubscription *OpenAISubscriptionAttribution
}
```

`Tracker.Record` should merge this into `metadata` without replacing caller-provided metadata.

## Redaction Rules

The following must never appear in `SpendLogs`, `ErrorLogs`, `AuditLog`, callback payloads, management events, or logs:

- `access_token`
- `refresh_token`
- `id_token`
- `Authorization: Bearer ...`
- JWT-looking strings
- fallback API keys
- encrypted `credential_value`
- raw OAuth/account response payloads

Allowed:

- `credential_id`
- `provider`
- `organization_id`
- `request_id`
- stable `action`, `status`, and `reason_code`
- safe status metadata after redaction

## Relationships

```text
resolvedOpenAISubscriptionCredential.CredentialID
  -> openAISubscriptionProviderAttempt.credentialID
  -> OpenAISubscriptionAttribution
  -> callback.LogData.OpenAISubscription
  -> spend.SpendRecord.OpenAISubscription
  -> SpendLogs.metadata.openai_subscription

credential lifecycle operation
  -> OpenAISubscriptionAttribution(action/status/reason)
  -> redact.JSONValue
  -> AuditLog.updated_values / management event payload
```
