# Implementation Plan: Org detail OpenAI credential panel

**Branch**: `HO-1189-org-detail-openai-credential-panel`
**Spec**: `/specs/1189-org-detail-openai-credential-panel/spec.md`
**Linear**: HO-1189
**Phase**: Todo planning only；production implementation starts only after Linear moves to `In Progress`。

## Technical Context

- Repo: `hohsiang-lab/tianjiLLM`
- Language/runtime: Go, `templ`, HTMX, Tailwind utility classes。
- UI routing: `internal/ui/routes.go` mounts `/ui/orgs/{org_id}` and `/ui/credentials/{credential_id}` inside existing `sessionAuth` group。
- Org detail handler: `internal/ui/handler_orgs_detail.go` builds `pages.OrgDetailData` with org row, members, teams, metadata。
- Org detail page: `internal/ui/pages/orgs.templ` renders overview cards, members table, teams card, metadata card。
- Credential UI: `internal/ui/handler_credentials.go` builds safe `pages.CredentialRow` / `pages.CredentialDetailData` from `CredentialTable`, redacted metadata, and `RateLimitStore`。
- Credential page: `internal/ui/pages/credentials.templ` already has list/detail row structures and `/ui/credentials/{credential_id}` links。
- DB query: `internal/db/queries/credential.sql` already has `ListCredentialsByOrg(ctx, organizationID *string)`。

## Constitution Check

- **Spec-first**: Todo branch contains only SpecKit artifacts and mockscreen evidence。
- **Repo reality first**: Plan reuses the current Org detail and Credentials UI code paths instead of inventing a new frontend stack。
- **TDD**: First implementation task is a failing UI E2E for org-scoped credential panel。
- **Security boundary**: Org panel must reuse/narrow existing credential safe view model logic; it must never render `credential_value` or raw metadata maps。
- **No new framework**: Keep Go `templ` server-rendered UI; no JS framework/package changes。

## Architecture

### Data Flow

Implementation should extend `loadOrgDetailData(r, orgID)`:

1. Load organization with `h.DB.GetOrganization(ctx, orgID)` as today。
2. Load members and teams as today。
3. Load org-scoped credentials via `h.DB.ListCredentialsByOrg(ctx, &orgID)`。
4. Filter rows to `CredentialType == "openai_subscription"`。
5. Convert rows through shared credentials safe row builder logic。
6. Pass rows into `pages.OrgDetailData.OpenAICredentials`。

Do not call `ListCredentials` and filter all credentials in memory unless `ListCredentialsByOrg` is unavailable at implementation time; current repo reality shows it exists.

### Shared Credential View Model

Current HO-1186 code has credential-safe logic in `internal/ui/handler_credentials.go`:

- `buildCredentialRow(cred, now) pages.CredentialRow`
- `parseOpenAISubscriptionInfo(raw)`
- `openAIQuotaState(credentialID, now)`
- credential/quota status helpers

Implementation should keep one source of truth. Prefer reusing `pages.CredentialRow` directly for the org panel or extracting a small private helper from credentials handler code. Do not copy a second divergent parser into `handler_orgs_detail.go`.

### Page Composition

Add one new section in `internal/ui/pages/orgs.templ` after overview cards and before Members:

- Card title: `OpenAI credentials`
- Description/count line: e.g. `{n} credential(s) connected to this organization`
- Empty state: `No OpenAI subscription credentials connected to this organization`
- Table columns:
  - Name: link to `/ui/credentials/{id}` + monospace credential ID
  - Email
  - Credential status badge
  - Quota status + summary
  - Last refresh
  - Last error
  - Actions: link/button to detail

Keep lifecycle actions on the detail page only. The org panel is read-only navigation.

### Security

The panel must not include:

- `CredentialTable.CredentialValue`
- decrypted token bundle
- raw `access_token`, `refresh_token`, `id_token`
- bearer/JWT-looking strings
- encrypted blob
- arbitrary raw `credential_info` map values
- raw OpenAI account payload

Long `last_error` / `disabled_reason` text must use the existing redaction path and wrap without overlapping layout.

### UI Mockscreen

The mockscreen shows:

- Existing Org detail header and overview cards。
- New `OpenAI credentials` panel between overview and Members。
- Two credential rows with status/quota badges。
- Empty-state variant note in the same visual direction。
- Link affordance goes to existing credential detail/actions page。

Generated mockscreen files live under `~/.openclaw/media/` and are not committed。

The owner-approved desktop/mobile mockscreen is an implementation acceptance artifact, not just a planning illustration. The production UI must keep visual equivalence with the mockscreen unless the owner approves a later revision:

- Panel placement: after overview cards, before Members。
- Desktop: compact table card with the same hierarchy, spacing density, status/quota badges, last-refresh/error columns, and detail-link affordance。
- Mobile: responsive stacked/scroll-safe layout with no clipped text, no overlapping badges, and the same empty-state treatment。
- Copy tone: title, count/description, empty state, and detail CTA should stay consistent with the approved mockscreen and existing Org detail UI。
- Waiting CI → Waiting Merge gate must include result-screen screenshots and a one-line comparison against the approved mockscreen。

## Tests

### RED first

1. Add `TestOrgDetail_OpenAICredentialsPanelShowsOrgScopedCredentials`:
   - seed org A and org B；
   - seed two `openai_subscription` credentials for org A；
   - seed one OpenAI subscription credential for org B；
   - seed one `api_key` credential for org A；
   - navigate `/ui/orgs/{orgA}`；
   - assert only org A subscription credentials are visible。

2. Add `TestOrgDetail_OpenAICredentialsEmptyState`:
   - seed org without OpenAI subscription credentials；
   - assert panel title and empty message。

3. Add `TestOrgDetail_OpenAICredentialLinkNavigatesToDetail`:
   - seed org + credential；
   - click panel credential link；
   - wait for `/ui/credentials/{credential_id}`；
   - assert existing detail content。

4. Add `TestOrgDetail_OpenAICredentialsSecretMaterialNotRendered`:
   - seed `credential_value` and metadata with token-looking fixtures；
   - assert body/HTML omit secret strings and raw field names。

### Verification commands

- `go test ./test/e2e -tags e2e -run 'TestOrgDetail_OpenAICredential' -count=1`
- `go test ./internal/ui/... -count=1`
- `go test ./internal/proxy/handler/... ./internal/callback/... -count=1`
- `go tool golangci-lint run`
- `git diff --check origin/main...HEAD`

If local E2E needs `E2E_DATABASE_URL`, report the environment gate and keep CI E2E mandatory.

## Plan Review Evidence

- Repo reality: `internal/ui/handler_orgs_detail.go` already centralizes org detail data loading; `internal/ui/pages/orgs.templ` already renders card/table sections; `internal/ui/handler_credentials.go` already has safe credential row/detail builders; `internal/db/queries/credential.sql` has `ListCredentialsByOrg`。
- Context7 `/a-h/templ`: templ supports dynamic data rendering by calling component `Render(r.Context(), w)` from Go HTTP handlers and dynamic attributes/content in components, matching repo `render(...)` pattern。
- Official `go-chi/chi` docs: chi supports route groups, middleware, named route params, and `net/http` compatibility; current `/ui` protected group is the correct place for this feature。
- GitHub/web prior art: public Go templ/HTMX admin examples reinforce server-rendered table/card dashboards, but no direct OpenAI credential org panel prior art was useful; repo-local Tianji patterns remain source of truth。
- OpenAI docs review not required for implementation behavior because this UI does not call OpenAI; OpenAI-specific facts are inherited from sibling credential persistence/lifecycle issues and stored metadata。

## Risks and Mitigations

- **Risk**: Duplicated credential status formatting drifts from `/ui/credentials`。
  **Mitigation**: reuse `pages.CredentialRow` and existing builder helpers or extract a shared private adapter。

- **Risk**: Panel leaks secret-looking metadata。
  **Mitigation**: narrow safe metadata parser + E2E DOM negative assertions。

- **Risk**: Org panel accidentally shows `Master` or other-org credentials。
  **Mitigation**: `ListCredentialsByOrg(ctx, &orgID)` plus E2E org-isolation fixture。

- **Risk**: Org detail becomes visually dense。
  **Mitigation**: place one compact table card before Members; use existing responsive `overflow-x-auto` table pattern。

- **Risk**: Implementation is functionally correct but drifts from the approved mockscreen。
  **Mitigation**: keep the mockscreen as acceptance source of truth in tasks/analyze and require desktop/mobile result-screen evidence before `Waiting Merge`。

## Scope Confirmation

Owner input required: 0.

Default implementation scope is a read-only org-scoped credentials panel on Org detail, with links to existing credential detail/actions. Implementation must wait for Linear `In Progress`.
