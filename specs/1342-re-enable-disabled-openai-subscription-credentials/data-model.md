# Data Model: HO-1342 Re-enable disabled OpenAI subscription credentials

## Existing Entity: `CredentialTable`

No schema change planned.

Relevant fields:

- `credential_id`: stable credential identifier used by model routes and UI actions.
- `credential_type`: must be `openai_subscription` for this lifecycle.
- `credential_value`: encrypted token bundle; enable must not read, decrypt, rewrite, or expose it.
- `credential_info`: JSON metadata used for OpenAI subscription lifecycle status.

## Existing Metadata: `OpenAISubscriptionCredentialInfo`

Fields:

- `email`
- `scopes`
- `status`
- `last_refresh_at`
- `last_error`
- `disabled_reason`

### Current Disabled State

```json
{
  "status": "disabled",
  "disabled_reason": "operator_disabled"
}
```

### Enabled State

Conservative expected metadata after enabling an operator-disabled credential:

```json
{
  "status": "active"
}
```

Rules:

- Clear `disabled_reason`.
- Preserve `email`, `scopes`, and `last_refresh_at`.
- Do not include token fields.
- `last_error` must remain redacted if preserved.

## Lifecycle Response

Enable should use the existing lifecycle response shape:

```json
{
  "credential_id": "cred-123",
  "action": "enable",
  "status": "ok"
}
```

Failure response:

```json
{
  "credential_id": "cred-123",
  "action": "enable",
  "status": "error",
  "reason_code": "credential_disabled"
}
```

Reason code can be adjusted to the existing error taxonomy during implementation, but it must be redaction-safe.

## UI View Model Additions

Credential row/detail data should expose one derived action state:

- `LifecycleAction`: `disable` or `enable`
- `LifecycleLabel`: `Disable` or `Enable`
- `LifecycleConfirm`: disable keeps confirmation; enable can use empty confirm or a short confirmation if implementation follows local UI convention.

Derived disabled predicate:

- true when `status == "disabled"` or `disabled_reason != ""`
- false for active/allowed and empty disabled reason

## Audit Attribution

Enable audit metadata:

- `action`: `enable`
- `status`: `success` or `failure`
- `reason_code`: safe code on failure
- optional safe metadata: previous disabled reason, new status

Forbidden audit data:

- access token
- refresh token
- authorization header
- encrypted credential value
- raw credential value
- JWT-like upstream strings
