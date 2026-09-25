# Implementation Plan: Credentials sidebar list/detail quota UI

**Branch**: `HO-1186-credentials-sidebar-list-detail-quota-ui`
**Spec**: `/specs/1186-credentials-sidebar-list-detail-quota-ui/spec.md`
**Linear**: HO-1186
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`。

## Technical Context

- Language/runtime: Go, `templ`, HTMX, Tailwind CSS v4 output generated under `internal/ui/assets/css/output.css`。
- UI routing: `internal/ui/routes.go` mounts protected pages under `r.Group(... h.sessionAuth ...)`。
- Layout/sidebar: `internal/ui/pages/layout.templ` uses `AppLayout(title, activePath)` and `navItem(...)` for sidebar entries。
- Existing list/detail patterns: `internal/ui/pages/keys.templ` and `internal/ui/pages/key_detail.templ` use card/table/badge/button components。
- Credential data: `internal/db/queries/credential.sql` has `ListCredentials`, `ListCredentialsByOrg`, `GetCredential`。
- Safe metadata contract: `internal/proxy/handler/credentials.go` defines `OpenAISubscriptionCredentialInfo`, `redactCredential`, and secret redaction behavior。
- Quota state: `internal/callback/openai_quota_store.go` defines `OpenAIQuotaState` and known flags; `internal/proxy/handler/openai_subscription_ratelimit.go` reads/writes state through `RateLimitStore`。
- UI handler already has `RateLimitStore callback.RateLimitStore` on `internal/ui/handler.go`。

## Constitution Check

- **Spec-first**: This PR is docs-only during Todo. No production code until Linear state is `In Progress`。
- **Repo reality first**: Plan is based on existing Tianji UI route/layout/component patterns and existing credential/quota types。
- **TDD**: First implementation task is a failing UI E2E for sidebar/list/detail, followed by quota/status and secret DOM assertions。
- **No new framework**: Reuse Go `templ` components; no React/Vue/Svelte/package addition。
- **Secret boundary**: UI view models must be built from safe columns/metadata and `RateLimitStore`; never decrypt `credential_value`。

## Architecture

### Routes

Add protected UI routes in `internal/ui/routes.go`:

- `GET /ui/credentials` -> `h.handleCredentials`
- `GET /ui/credentials/{credential_id}` -> `h.handleCredentialDetail`

Both routes remain inside the existing `sessionAuth` group. This UI uses admin session auth, matching existing UI behavior.

### Sidebar

Add a `Credentials` nav item to `internal/ui/pages/layout.templ` near other Management entries:

- href: `/ui/credentials`
- label: `Credentials`
- icon: use an existing lucide icon from `internal/ui/components/icon`, preferably `key-round` if available, otherwise `key` to match API-key context。

### Data Adapter

Add `internal/ui/handler_credentials.go` to keep credential UI code separate from keys/users/org handlers.

Proposed helpers:

- `handleCredentials(w, r)`
- `handleCredentialDetail(w, r)`
- `buildCredentialRows(creds []db.CredentialTable, now time.Time) []pages.CredentialRow`
- `buildCredentialDetail(cred db.CredentialTable, quota callback.OpenAIQuotaState, hasQuota bool, now time.Time) pages.CredentialDetailData`
- `parseSafeOpenAISubscriptionInfo(raw []byte) pages.CredentialSafeInfo`
- `quotaDimensionView(name string, d callback.OpenAIQuotaDimension) pages.QuotaDimensionView`

Implementation must filter `CredentialTable.CredentialType == handler.CredentialTypeOpenAISubscription` after DB list, unless a generated query already exists by implementation time. Do not add ad hoc SQL in UI handlers; if a DB query is required, add sqlc query and generated code in the normal repo pattern.

### Pages

Add `internal/ui/pages/credentials.templ` with:

- `CredentialsPage(data CredentialsPageData)`
- `CredentialsTable(data CredentialsPageData)`
- `CredentialDetailPage(data CredentialDetailData)`
- `QuotaStatusBadge(status string)`
- `QuotaProgress(value float64, known bool)`
- `QuotaDimensionTable(rows []QuotaDimensionView)`

List page shape:

- Header row with title and count.
- Table columns: Name, Email, Status, Quota, Reset, Last refresh, Updated.
- Each row links to `/ui/credentials/{credential_id}`.
- Empty state matches existing table empty-state style.

Detail page shape:

- Header with credential name, status badge, back link.
- Cards for Account, Status, Quota Summary.
- Table for request/token quota dimensions.
- Metadata section for safe `last_error` and `disabled_reason`.
- Do not place UI cards inside other cards.

### Quota Semantics

Display rules:

- `OpenAIQuotaState.Status` drives quota badge only when known.
- `credential_info.status=disabled` or `disabled_reason` must remain visible even if quota state says allowed.
- Request/token dimensions display only known fields; unknown values render as `Unknown`/`Not recorded`。
- Progress width uses stored `Utilization` when `UtilizationKnown`; otherwise derive only when existing normalized state already has valid known limit/remaining in code. Clamp `0..100` for CSS.
- Reset time comes from known request/token reset or quota reset; absent means `Not recorded`。
- Do not compute billing quota, monthly allowance, tier, remaining dollars, or subscription entitlement.

### Security

View model builder must never include:

- `CredentialTable.CredentialValue`
- decrypted token bundle
- raw `access_token`, `refresh_token`, `id_token`
- bearer/JWT-looking values
- encrypted blob
- raw OpenAI account payload

For `credential_info`, implementation should decode into `OpenAISubscriptionCredentialInfo` or a narrow safe struct. It must not render arbitrary metadata map values unless they pass existing redaction rules.

## Tests

### RED first

1. Add failing E2E `test/e2e/credentials_list_test.go`:
   - Login.
   - Verify sidebar `Credentials` link.
   - Seed two `openai_subscription` credentials and one `api_key` credential.
   - Navigate list and assert only subscription credentials are visible.

2. Add failing E2E `test/e2e/credentials_detail_test.go`:
   - Seed one subscription credential with safe metadata.
   - Navigate detail from list.
   - Assert account email/status/last refresh/last error/disabled reason.

3. Add failing quota/status E2E:
   - Seed `RateLimitStore` state for request/token limit/remaining/reset.
   - Assert progress text/badge/reset/table rows.
   - Cover unknown/no quota state.

4. Add failing secret DOM assertion:
   - Seed credential value and metadata with token-looking fixtures.
   - Assert `f.Text("body")` and relevant HTML do not contain those strings.

### Verification commands

- `go test ./test/e2e -tags e2e -run 'TestCredentials' -count=1`
- `go test ./internal/ui/... -count=1`
- `go test ./internal/proxy/handler/... ./internal/callback/... -count=1`
- `go tool golangci-lint run`
- `git diff --check origin/main...HEAD`

If full `go test ./...` needs local Postgres/Redis and fails from missing services, report that as environment-gated and keep targeted tests mandatory.

## Plan Review Evidence

- Repo reality: existing UI route/layout/list/detail patterns verified in `internal/ui/routes.go`, `layout.templ`, `keys.templ`, `key_detail.templ`, and E2E helpers.
- Context7 `/a-h/templ`: `templ` supports rendering dynamic components directly from Go HTTP handlers using `Component.Render(r.Context(), w)`, matching existing `render(...)` helper.
- Context7 `/go-chi/docs`: `chi.Router` supports grouped routes and method-specific handlers, matching existing `r.Group` + `r.Get` route structure.
- GitHub grep.app: public search for `@sidebar.MenuButton`, `templ AppLayout`, and `x-ratelimit-remaining-requests` returned no useful direct prior art; use repo-local patterns as source of truth.
- Official OpenAI docs: Models list and bearer authentication evidence remains relevant only for sibling lifecycle test API; this UI must not call OpenAI directly. Rate-limit/quota display must therefore consume stored state from HO-1181, not external live OpenAI APIs.

## Risks and Mitigations

- **Risk**: UI accidentally renders arbitrary `credential_info` and leaks secret-looking values.
  **Mitigation**: Narrow typed safe metadata struct plus DOM secret assertions.

- **Risk**: Unknown quota is shown as `0%` and misleads operator.
  **Mitigation**: Known flags in view model; unknown renders explicit fallback.

- **Risk**: Disabled metadata and quota allowed state conflict.
  **Mitigation**: Status derivation prioritizes credential disabled/error metadata for credential health, while quota badge remains separate.

- **Risk**: New UI drifts from existing Tianji style.
  **Mitigation**: Reuse existing `AppLayout`, sidebar, card, table, badge, button, icon components and E2E viewport checks.

## Scope Confirmation

Owner input required: 0.

Default implementation scope is the Credentials UI read-only surface for OpenAI subscription credentials. Implementation must wait for Linear `In Progress`.
