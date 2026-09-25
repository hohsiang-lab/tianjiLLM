# Implementation Plan: HO-1342 Re-enable disabled OpenAI subscription credentials

## Goal

Add a reversible lifecycle path for operator-disabled OpenAI subscription credentials without changing token storage, OAuth reconnect, or routing selection logic.

The implementation should add one lifecycle action:

- proxy API: `POST /credentials/openai-subscription/{credential_id}/enable`
- UI action: `POST /ui/credentials/{credential_id}/enable`
- UI behavior: disabled credentials show `Enable`; active credentials show `Disable`
- regression: enabled credentials become resolvable by the existing OpenAI subscription resolver

## Constraints

- Todo phase is docs-only. No production code in this PR.
- Do not decrypt or rewrite token bundles for enable; lifecycle metadata is sufficient.
- Preserve existing redaction and audit behavior.
- Do not broaden OAuth reconnect or refresh-failure recovery semantics.
- Keep implementation close to existing disable lifecycle patterns.

## Repo Reality

- `internal/proxy/handler/openai_subscription_lifecycle.go`
  - Defines `openAISubscriptionLifecycleActionTest`, `Refresh`, and `Disable`.
  - `OpenAISubscriptionCredentialDisable` loads credential metadata, sets `Status="disabled"` and `DisabledReason="operator_disabled"`, updates `CredentialInfo`, and writes lifecycle audit action `disable`.
  - There is no `enable` action constant or handler.
- `internal/proxy/handler/openai_subscription_refresh.go`
  - `loadOpenAISubscriptionCredential` rejects credentials when `info.Status == "disabled"` before decrypting and resolving the bundle.
  - Therefore metadata enable is enough for a fresh token bundle to become resolvable again.
- `internal/proxy/server.go`
  - Credential routes register `test`, `refresh`, and `disable`, but no `enable`.
- `internal/auth/rbac.go` / `internal/auth/rbac_test.go`
  - `/credentials` is proxy-admin only.
  - Tests explicitly cover test/refresh/disable but not enable.
- `internal/ui/routes.go`
  - UI credentials routes include test/refresh/disable/delete and Codex usage refresh, but no enable.
- `internal/ui/handler_credentials.go`
  - `handleCredentialDisable` and `invokeCredentialLifecycle` support `disable`.
  - Success/error copy switches include disable only.
  - `credentialStatusLabel` treats `status=disabled` or any `disabled_reason` as disabled.
- `internal/ui/pages/credentials.templ`
  - `CredentialActions` always renders `Disable`; it does not receive credential status or decide action by state.
  - Templ generated file mirrors this behavior and must be regenerated with `make ui` after implementation.
- Existing lifecycle tests in `internal/proxy/handler/openai_subscription_lifecycle_test.go`
  - Prove disable is idempotent, local metadata-only, excludes resolution, and redacts audit failures.
  - These provide the nearest pattern for enable tests.

## Proposed Design

### 1. Lifecycle handler

Add `openAISubscriptionLifecycleActionEnable = "enable"` and `OpenAISubscriptionCredentialEnable`.

Suggested conservative semantics:

- Load metadata with `loadOpenAISubscriptionCredentialMetadata`.
- If already active/allowed and no disabled reason, return success idempotently.
- If `disabled_reason == operator_disabled` or `status == disabled` with operator-disabled metadata:
  - set `Status = "active"`
  - clear `DisabledReason`
  - preserve email/scopes/last refresh time
  - redact/preserve or clear `LastError` according to implementation's UI consistency choice; default preserve redacted value unless it would keep UI looking failed.
- If disabled by `auth_failed_after_refresh` / `refresh_failed`, reject with a safe reason and leave metadata unchanged.
- Update `CredentialInfo` only; do not update `CredentialValue`.
- Write audit action `enable` success/failure.

### 2. Routes and RBAC

Register proxy route:

```text
POST /credentials/openai-subscription/{credential_id}/enable
```

Add UI route:

```text
POST /ui/credentials/{credential_id}/enable
```

RBAC remains inherited from `/credentials`, but tests must explicitly prove the enable path is admin-only.

### 3. UI action switching

Extend credential row/detail data with enough state to choose lifecycle action.

Render:

- active/allowed/unknown non-disabled: `Disable`
- disabled or disabled_reason non-empty: `Enable`

Do this for both list rows and detail actions. Keep delete/test/refresh unchanged.

### 4. Tests first

Add RED tests before implementation:

- lifecycle enable metadata test
- lifecycle enable makes resolver usable again
- enable idempotency for active credential
- enable rejection/safe handling for non-operator disabled reason
- enable audit redaction
- RBAC route coverage
- UI list/detail action rendering
- UI dispatcher success/error copy and target re-render

### 5. Verification

Expected local gates:

```bash
go test ./internal/proxy/handler -run 'OpenAISubscriptionLifecycle.*Enable|OpenAISubscriptionLifecycle.*Disable|ResolveOpenAISubscription' -count=1
go test ./internal/auth -run 'CredentialsRemainProxyAdminOnly' -count=1
go test ./internal/ui -run 'Credential.*Enable|Credentials' -count=1
go test ./internal/ui/pages -run 'Credential.*Enable|Credentials' -count=1
make ui
git diff --check
```

## Plan Review

- Repo reality checked first: current disable lifecycle persists metadata only, and resolver excludes disabled credentials before token decrypt/refresh.
- Existing code has a direct disable lifecycle pattern; enable should mirror that shape with opposite metadata mutation and audit action.
- No owner input is needed for Todo scope: implementation should allow only operator-disabled recovery and reject auth/refresh-failure disabled reasons safely.
