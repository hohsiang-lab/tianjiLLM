# Analysis: HO-1342 Re-enable disabled OpenAI subscription credentials

## Summary

- Fatal: 0
- Critical: 0
- Owner input: 0

## Findings

### Scope Confirmation - Operator-disabled only

The Linear description explicitly says to restore operator-disabled credentials. Current code distinguishes operator disable from auth/refresh failure metadata:

- operator disable writes `disabled_reason=operator_disabled`
- auth failure after forced refresh writes `disabled_reason=auth_failed_after_refresh`
- refresh failure writes `disabled_reason=refresh_failed`

Todo scope is safe and complete if implementation only enables `operator_disabled` and already-active idempotent credentials. Reactivating auth/refresh-failure disabled credentials remains out of scope because it can hide a real reauthorization-needed state until the next proxy request fails.

## Scope Confirmation

Implementation scope is locked: enable only `operator_disabled`; reject auth/refresh failure disabled reasons safely.

## Repo Reality Evidence

- `internal/proxy/handler/openai_subscription_lifecycle.go` implements disable only.
- `internal/proxy/handler/openai_subscription_refresh.go` rejects disabled credentials during resolution.
- `internal/proxy/server.go` registers test/refresh/disable only.
- `internal/ui/routes.go` registers test/refresh/disable/delete only.
- `internal/ui/handler_credentials.go` dispatches test/refresh/disable/delete only.
- `internal/ui/pages/credentials.templ` always renders `Disable`.
- `internal/auth/rbac_test.go` lacks enable route coverage.

## Gate Result

SpecKit package is suitable for Todo handoff as docs-only. Production implementation must wait for Linear `In Progress`.
