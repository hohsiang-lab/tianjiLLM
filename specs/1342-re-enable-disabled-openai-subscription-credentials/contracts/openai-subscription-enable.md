# Contract: OpenAI subscription credential enable lifecycle

## Proxy API

```http
POST /credentials/openai-subscription/{credential_id}/enable
```

Authorization:

- Minimum role: `proxy_admin`
- Route must remain under `/credentials` RBAC protection.

Success:

```json
{
  "credential_id": "cred-123",
  "action": "enable",
  "status": "ok"
}
```

Failure:

```json
{
  "credential_id": "cred-123",
  "action": "enable",
  "status": "error",
  "reason_code": "credential_missing"
}
```

## UI API

```http
POST /ui/credentials/{credential_id}/enable
```

Behavior:

- Calls proxy lifecycle enable.
- On success, re-renders the credentials table or credential detail content based on HTMX target/referrer.
- Shows success toast `Credential enabled`.
- On failure, shows redacted error toast `Credential enable failed: <reason_code>`.

## Metadata Mutation

For `operator_disabled`:

- Before: `status=disabled`, `disabled_reason=operator_disabled`
- After: `status=active`, `disabled_reason=""`

Enable must not update `credential_value`.

## UI Rendering Contract

Credential list:

- Disabled credential row renders `Enable`.
- Active credential row renders `Disable`.

Credential detail:

- Disabled credential detail renders `Enable`.
- Active credential detail renders `Disable`.

## Out-of-Scope Contract

The enable action must not:

- start OAuth reconnect
- call OpenAI upstream
- call ChatGPT Codex usage endpoint
- change delete semantics
- auto-enable credentials in background
