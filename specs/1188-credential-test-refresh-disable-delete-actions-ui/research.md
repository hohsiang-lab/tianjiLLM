# Research: Credential Test/Refresh/Disable/Delete actions UI

## Sources Checked

- Linear HO-1188 title/Goal/Scope/Tests.
- Repo reality in `internal/ui/handler_credentials.go`, `internal/ui/pages/credentials.templ`, `internal/ui/routes.go`, `test/e2e/credentials_test.go`.
- Backend lifecycle routes in `internal/proxy/server.go`.
- Backend lifecycle handlers in `internal/proxy/handler/openai_subscription_lifecycle.go`.
- Backend credential delete handler in `internal/proxy/handler/credentials.go`.
- Existing UI confirm/toast patterns in `internal/ui/pages/key_detail.templ`, `internal/ui/handler_keys.go`, `internal/ui/handler_guardrails.go`, `internal/ui/pages/orgs.templ`, `internal/ui/components/toast`.
- Memory `memory/2026-05-08.md:93` for HO-1185 lifecycle/redaction contract.
- Wiki Pattern #481 for credential lifecycle redaction across responses/metadata/audit/DOM.
- Context7 `/bigskysoftware/htmx` for `hx-confirm`.
- grep-app public GitHub search for `hx-confirm` real-world HTMX admin action examples.
- Official OpenAI API reference for bearer-authenticated `GET /v1/models`.

## Decisions

### Use existing backend lifecycle APIs

Decision: UI action handlers should delegate to existing backend behavior or shared helpers behind `OpenAISubscriptionCredentialTest` / `Refresh` / `Disable` and `CredentialDelete`.

Reason: HO-1185 already owns backend semantics, redaction, audit, refresh, and upstream test behavior. Reimplementing those in UI would create drift and a secret-egress risk.

### Use HTMX partial updates and existing toast component

Decision: Use existing `templ` + HTMX route style and OOB toast pattern, matching keys/orgs/guardrails.

Reason: Existing management UI already uses server-rendered partials and toast components. This keeps the feature small and avoids adding client framework state.

### Reload safe metadata after mutation

Decision: After success, rebuild the row/detail from DB-safe metadata, not from client-side lifecycle response alone.

Reason: Refresh/disable/delete are persisted mutations; DB-safe reload is the source of truth and ensures the UI shows exactly what later resolution/routing code will see.

### Treat toast/attributes as secret egress boundaries

Decision: E2E must inspect DOM after action success/failure and assert no token-looking strings appear in body, toast, hidden inputs, links, or HTMX attributes.

Reason: Lifecycle errors often originate from upstream/token boundaries. HO-1185/Pattern #481 explicitly require redaction across every egress sink.

## External Evidence

### HTMX

Context7 and htmx docs show `hx-confirm` is the built-in way to show a browser confirmation dialog before issuing an HTMX request. Public GitHub examples commonly combine `hx-confirm` with `hx-post`, `hx-target`, and partial swaps for admin destructive actions.

### OpenAI

Official OpenAI API reference documents `GET /v1/models` as a bearer-authenticated endpoint returning a model list. This supports keeping test behavior server-side and mockable, with UI displaying only a safe count/status summary.

## Open Questions

None requiring owner input. Linear scope is explicit enough:

- Actions: Test/Refresh/Disable/Delete.
- UI pattern: existing confirm + toast.
- Result: update safe status metadata.
- Tests: UI E2E for all actions, confirm/toast, safe error states.
