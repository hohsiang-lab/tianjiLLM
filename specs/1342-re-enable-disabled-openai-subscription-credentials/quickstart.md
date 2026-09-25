# Quickstart: HO-1342 Re-enable disabled OpenAI subscription credentials

## Implementation Entry

Start only after Linear HO-1342 moves to `In Progress`.

```bash
cd /Users/n0rmanc/.openclaw/workspace-sima/worktrees/tianjiLLM/HO-1342-re-enable-disabled-openai-subscription-credentials
git status --short --branch
```

## Expected RED Tests

```bash
go test ./internal/proxy/handler -run 'OpenAISubscriptionLifecycle.*Enable' -count=1
go test ./internal/auth -run 'CredentialsRemainProxyAdminOnly' -count=1
go test ./internal/ui -run 'Credential.*Enable|Credentials' -count=1
go test ./internal/ui/pages -run 'Credential.*Enable|Credentials' -count=1
```

These should fail before implementation because enable routes/handlers/actions do not exist.

## Implementation Order

1. Add proxy lifecycle tests and handler.
2. Register proxy route and RBAC test.
3. Add UI route/dispatcher/copy.
4. Update credentials templates to switch `Enable`/`Disable`.
5. Run `make ui`.
6. Run targeted tests and `git diff --check`.

## Manual Smoke After Implementation

Seed or use an existing OpenAI subscription credential with:

```json
{
  "status": "disabled",
  "disabled_reason": "operator_disabled"
}
```

Expected behavior:

- `/ui/credentials` row shows `Disabled` and `Enable`.
- `/ui/credentials/{credential_id}` detail shows `Enable`.
- Clicking `Enable` returns active status and clears disabled reason.
- Existing route resolution can use the credential again.

## Security Checks

Search output and audit fixtures for forbidden values:

- access token
- refresh token
- `Bearer`
- JWT-like `eyJ`
- encrypted credential blob

No forbidden value should appear in JSON responses, rendered HTML, or audit metadata.
