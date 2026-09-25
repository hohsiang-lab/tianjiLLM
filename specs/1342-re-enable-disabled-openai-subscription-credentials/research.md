# Research: HO-1342 Re-enable disabled OpenAI subscription credentials

## Decision 1: Enable should be metadata-only

**Decision**: Implement enable by updating `credential_info` only, not by decrypting or rewriting `credential_value`.

**Rationale**: Current disable is metadata-only and current resolver rejects `status=disabled` before token decrypt. Clearing disabled metadata is enough for the existing resolver to load a fresh token bundle again.

**Alternatives considered**:

- Force refresh during enable: rejected for Todo scope because reconnect/refresh recovery is explicitly out of scope and would conflate lifecycle activation with upstream auth health.
- Delete/recreate credential: rejected because it changes credential identity and routing references.

## Decision 2: Mirror existing lifecycle/audit pattern

**Decision**: Add `enable` beside existing `test`, `refresh`, and `disable` lifecycle actions.

**Rationale**: `OpenAISubscriptionCredentialDisable` already handles metadata validation, safe marshal, DB update, lifecycle response, and audit attribution. Enable should reuse the same shape to minimize new surface area.

## Decision 3: UI action should switch by disabled state

**Decision**: Disabled credentials render `Enable`; non-disabled credentials render `Disable`.

**Rationale**: Current UI action list is static, which is the visible bug. Existing `credentialStatusLabel` already treats `status=disabled` or any `disabled_reason` as disabled, so the UI can reuse a derived action state rather than inventing a separate status parser.

## Decision 4: Auth/refresh failure disabled credentials need conservative handling

**Decision**: Scope allows re-enabling `operator_disabled`; auth/refresh-failure disabled reasons must be rejected with metadata unchanged.

**Rationale**: `operator_disabled` is an intentional local admin lifecycle state. `auth_failed_after_refresh` and `refresh_failed` encode upstream credential health failures; silently clearing them may make a bad token look routable until the next request fails.

## Repo Evidence

- `internal/proxy/handler/openai_subscription_lifecycle.go`: disable writes `Status="disabled"` and `DisabledReason="operator_disabled"`.
- `internal/proxy/handler/openai_subscription_refresh.go`: resolver rejects `info.Status == "disabled"`.
- `internal/proxy/server.go`: no enable proxy route exists.
- `internal/ui/routes.go`: no enable UI route exists.
- `internal/ui/handler_credentials.go`: action dispatcher handles test/refresh/disable/delete only.
- `internal/ui/pages/credentials.templ`: `CredentialActions` always renders `Disable`.
- `internal/auth/rbac_test.go`: credentials RBAC coverage names test/refresh/disable only.

## Risks

- If enable clears all disabled reasons, auth-failed credentials may appear healthy before reauthorization.
- If UI only switches detail but not list, the original bug remains partially visible.
- If enable mutates token value unnecessarily, it increases secret-handling blast radius.
