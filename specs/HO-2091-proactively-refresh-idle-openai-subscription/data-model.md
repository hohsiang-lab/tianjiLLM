# Data Model: Proactive OpenAI Subscription Credential Refresh

## OpenAI Subscription Credential

Existing `CredentialTable` row.

- `credential_id`: stable identifier referenced by model routes.
- `credential_type`: must equal `openai_subscription`.
- `credential_value`: encrypted `OpenAISubscriptionTokenBundle`.
- `credential_info`: safe JSONB metadata visible to UI/operators.
- `organization_id`: optional organization attribution.

## OpenAI Subscription Token Bundle

Existing encrypted JSON payload.

- `access_token`: secret, never logged or exposed.
- `refresh_token`: secret, preserved if refresh response omits a replacement.
- `expires_at`: access-token expiry used for 5-minute request freshness and 30-minute proactive candidate selection.
- `account_id`: OpenAI/ChatGPT account identifier used by Codex transport.

## OpenAI Subscription Credential Info

Existing safe metadata extended for proactive failure clarity.

- `email`: safe operator context.
- `scopes`: safe OAuth scopes.
- `status`: `active`, `disabled`, `refresh_failed`, or reconnect-required terminal value.
- `last_refresh_at`: latest successful refresh timestamp.
- `first_refresh_failed_at`: first failed refresh timestamp for the current failure streak.
- `last_refresh_failed_at`: latest failed refresh timestamp.
- `last_error`: redacted latest failure message.
- `disabled_reason`: stable reason code, including `refresh_failed`, `refresh_token_invalidated`, or `auth_failed_after_refresh`.
- `operator_message`: safe text such as `OpenAI session ended; reconnect required`.

## Proactive Refresh Candidate

Derived in the job, not a persisted entity.

- `credential_id`
- current safe metadata status
- decrypted `expires_at`
- skip reason when not processed

Candidate rule:

```text
credential_type == "openai_subscription"
AND metadata status is active/selectable
AND deleted/non-selectable metadata is absent
AND decrypted expires_at <= now + 30m
```

## Route Candidate Reference

Existing model route reference in `tianji_params.openai_subscription_credential_ids`.

- active reference: resolves to an active/selectable credential.
- stale reference: missing, deleted, disabled, `refresh_failed`, or reconnect-required.

Validation/result state should be surfaced to model UI/logging so operators can fix or prune references.
