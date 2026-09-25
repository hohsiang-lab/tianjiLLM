# Implementation Plan: Credential Test/Refresh/Disable/Delete actions UI

**Branch**: `HO-1188-credential-test-refresh-disable-delete-actions-ui`
**Spec**: `specs/1188-credential-test-refresh-disable-delete-actions-ui/spec.md`
**Linear**: HO-1188

## Technical Context

**Language/Version**: Go 1.24.x, `templ`, HTMX, Tailwind classes already bundled in `internal/ui`.
**UI Framework**: Server-rendered Go `templ` pages in `internal/ui/pages`, protected by `UIHandler.sessionAuth`.
**Storage**: Existing `CredentialTable`; no schema changes.
**Backend API**: Existing management routes:

- `POST /credentials/openai-subscription/{credential_id}/test`
- `POST /credentials/openai-subscription/{credential_id}/refresh`
- `POST /credentials/openai-subscription/{credential_id}/disable`
- `DELETE /credentials/delete/{credential_id}`

**Testing**: `go test -tags e2e ./test/e2e`, targeted handler tests, `go test ./internal/ui/...`, `git diff --check`.

## Repo Reality Findings

- `internal/ui/routes.go` currently registers only `GET /ui/credentials` and `GET /ui/credentials/{credential_id}`.
- `internal/ui/handler_credentials.go` already builds safe list/detail view models from redacted `credential_info` and `RateLimitStore`.
- `internal/ui/pages/credentials.templ` currently renders list/detail but no lifecycle action controls.
- `test/e2e/credentials_test.go` already covers sidebar/list/detail/quota/redaction and should be extended rather than replaced.
- `internal/proxy/server.go` already exposes lifecycle endpoints under auth middleware.
- `internal/proxy/handler/openai_subscription_lifecycle.go` returns safe status/reason/model count/last refresh response shapes.
- `internal/proxy/handler/credentials.go` already supports `DELETE /credentials/delete/{credential_id}` and idempotent missing delete behavior at API layer.
- Existing UI confirm/toast patterns live in `internal/ui/pages/key_detail.templ`, `internal/ui/handler_keys.go`, `internal/ui/handler_guardrails.go`, `internal/ui/pages/orgs.templ`, and `internal/ui/components/toast`.

## Architecture Decision

Implement a thin UI wrapper layer in `internal/ui`, not a new backend lifecycle implementation.

1. Add protected UI POST routes under `/ui/credentials/{credential_id}/test|refresh|disable|delete`.
2. UI handlers call existing lifecycle logic through shared handler/service boundary or local helper that uses the same DB-safe operations; implementation must avoid duplicating OpenAI refresh/test semantics.
3. Return server-rendered partials for list/detail action areas plus OOB toast.
4. After mutation, reload credential from DB and rebuild existing safe view model.
5. For delete success:
   - list context: return refreshed credentials table/list partial with row removed and success toast.
   - detail context: redirect or swap to safe "deleted" panel with link back to `/ui/credentials`; no stale credential metadata remains.

## Data Flow

```text
admin click action
  -> HTMX POST /ui/credentials/{id}/{action}
  -> sessionAuth
  -> UI handler validates selected credential is openai_subscription
  -> existing lifecycle/delete backend behavior runs
  -> DB-safe metadata is reloaded
  -> partial HTML + toast returns
  -> list/detail updates without raw token material
```

## Action Behavior Matrix

| Action | Confirm | Success UI | Error UI | State refresh |
| --- | --- | --- | --- | --- |
| Test | No | Success toast, optional models count | Safe reason toast | Reload row/detail to keep status current |
| Refresh | No | Success toast with last refresh timestamp | Safe reason toast | Reload row/detail safe metadata |
| Disable | Yes | Disabled badge/reason + success toast | Safe reason toast | Reload row/detail safe metadata |
| Delete | Yes | Row removed or detail returns to list/deleted state | Safe reason toast | Reload list or safe deleted state |

## Security Boundaries

- UI handlers must not decrypt `CredentialTable.credential_value`.
- Toast copy must be derived from stable action/reason codes, not raw upstream response body.
- HTMX forms must carry only credential ID/path and context, never tokens or credential value.
- `last_error` / `disabled_reason` display must continue through existing redaction helper.
- E2E must assert secret-looking strings are absent from rendered DOM after action success and failure.

## UI Layout Plan

- List page: add a compact "Actions" column with icon/text buttons; stable width, wraps on narrow viewport.
- Detail page: add action toolbar near status badges/header, before quota cards.
- Destructive actions use existing destructive button variant and `hx-confirm`.
- Non-destructive actions use outline/default variants.
- Toast appears via existing `toast.Script()` + OOB insertion pattern.
- No nested cards; action toolbar is unframed or a single detail action band.

## Plan Review Evidence

- Context7 `/bigskysoftware/htmx` confirms `hx-confirm` is the standard htmx confirmation attribute for destructive actions and can be combined with `hx-post` / `hx-target`.
- GitHub grep-app examples show production HTMX apps commonly pair `hx-confirm` with `hx-post`, `hx-target`, and partial swaps for admin destructive actions.
- Official OpenAI API docs confirm `GET /v1/models` is a bearer-authenticated list endpoint returning model list data; UI must keep that interaction in backend/mock boundary and only show safe summary.
- Current repo already owns the lifecycle backend API; FE should not call OpenAI directly.

## Alternatives Considered

### Alternative A - Direct browser `fetch()` to management API

Rejected. It would introduce custom client-side JS and duplicate HTMX/toast handling that existing Tianji UI already solves.

### Alternative B - Add modal framework for lifecycle actions

Rejected. Existing key delete flow and HTMX `hx-confirm` cover confirmation without adding new dependencies.

### Alternative C - Add only detail-page actions

Rejected. Linear scope says actions UI, and list page is the primary credential management surface from HO-1186/HO-1187.

### Alternative D - Recompute UI from lifecycle JSON response only

Rejected. Reloading DB-safe metadata after mutation is safer and avoids stale/partial client state.

## Implementation Phases

1. Add failing UI E2E for list/detail actions, confirm cancel/accept, toast, and redaction.
2. Add UI route handlers for credential actions under existing `sessionAuth`.
3. Add page partials/components for action toolbar, credentials table/detail swap targets, and action toast wrappers.
4. Wire lifecycle/delete calls to existing backend behavior without duplicating token logic.
5. Regenerate `templ` output.
6. Run targeted UI/E2E compile and affected test gates.

## Constitution Check

- Failing tests first: required for all four actions before implementation.
- No production code in Todo: this PR contains only SpecKit artifacts.
- Reuse existing repo patterns: `templ`, HTMX partials, `toast`, `button`, `badge`.
- No secret egress: explicit DOM/toast/attribute assertions.
- Offline tests: mock/seeded DB only; no live OpenAI network call.

> Plan reviewed via context7 / grep-app / official OpenAI docs — revisions captured above.
