# Quickstart: Credential actions UI

## Local Planning Gate

```bash
cd /Users/n0rmanc/.openclaw/workspace-sima/worktrees/tianjiLLM/HO-1188-credential-test-refresh-disable-delete-actions-ui
git diff --check origin/main...HEAD
```

Expected during Todo: diff contains only `specs/1188-credential-test-refresh-disable-delete-actions-ui/**`.

## Implementation Gate Commands

Run after Linear moves to `In Progress` and implementation is complete.

```bash
go test ./internal/ui/...
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionCredential|TestCredentialDelete' -count=1
go test -c -tags e2e ./test/e2e
git diff --check origin/main...HEAD
```

Run targeted browser E2E when `E2E_DATABASE_URL` is available:

```bash
go test -tags e2e ./test/e2e -run 'TestCredential.*Action|TestCredentials.*Action' -count=1
```

## Manual/UI Review Checklist

- Open `/ui/credentials`.
- Verify list row action controls: `Test`、`Refresh`、`Disable`、`Delete`.
- Verify `Disable` and `Delete` confirm before mutation.
- Verify cancelling confirm leaves row/detail unchanged.
- Verify success toast appears for all four actions.
- Verify safe error toast appears for mocked failures.
- Verify detail page action toolbar works and updates status metadata.
- Verify delete-from-detail returns to list or safe deleted state.
- Inspect page source/body after action failures for token-looking strings.

## Result-Screen Evidence Required Before Waiting Merge

Capture screenshots or equivalent browser evidence for:

- list actions visible
- detail actions visible
- test success toast
- refresh updated timestamp/status
- disabled state
- delete result
- safe error toast

Do not move Waiting CI -> Waiting Merge without this FE result-screen evidence.
