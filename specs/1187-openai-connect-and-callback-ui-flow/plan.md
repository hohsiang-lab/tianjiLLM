# Implementation Plan: OpenAI connect and callback UI flow

**Branch**: `HO-1187-openai-connect-and-callback-ui-flow`
**Spec**: `specs/1187-openai-connect-and-callback-ui-flow/spec.md`
**Linear**: HO-1187
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Language/runtime: Go, `templ`, HTMX, Tailwind CSS output embedded under `internal/ui/assets`。
- UI route tree: `internal/ui/routes.go` mounts `/ui/openai/connect` inside the protected `sessionAuth` group。
- Existing connect handler: `internal/ui/handler_openai.go` validates `org_id`, creates OpenAI OAuth state through `openaioauth.StateStore`, derives redirect URI, and redirects to OpenAI authorize URL。
- Existing callback route: `internal/proxy/server.go` mounts public `GET /oauth/openai/callback` outside API auth。
- Existing callback handler: `internal/proxy/handler/openai_oauth.go` consumes state, handles provider errors, exchanges code, saves OpenAI subscription credential, audits lifecycle, then writes minimal HTML through `writeOpenAIOAuthPage`。
- Existing Credentials UI: `internal/ui/pages/layout.templ` already has `Credentials` sidebar entry; `internal/ui/pages/credentials.templ` renders OpenAI subscription credential list/detail。
- Existing E2E helpers: `test/e2e/helpers_test.go` logs in UI sessions, seeds credentials, and has `NavigateToCredentials` / `NavigateToCredentialDetail` helpers。
- Existing OpenAI mock: `internal/testutil/openaitest` provides local authorize/token endpoints and request recording。

## Constitution Check

- **Spec-first**: This PR is docs-only in Todo. No production files outside `specs/1187-*` are modified。
- **Repo reality first**: Plan is based on current `origin/main` files for UI routes, callback handler, Credentials page, E2E fixtures, and OpenAI mock harness。
- **TDD**: Implementation tasks start with failing UI E2E for connect success, callback failure, and unauthenticated connect。
- **Security**: Result pages must preserve HO-1175 server-side state/PKCE/callback constraints and HO-1184/HO-1186 redaction boundaries。
- **No new framework**: Reuse Go `templ` and existing UI components; no SPA package。

## Architecture

### Connect Entry Point

Default placement: add the CTA on `/ui/credentials`, because HO-1186 established this as the OpenAI subscription credential management surface.

Preferred shape:

- Primary button/link label: `Connect OpenAI`
- URL: `/ui/openai/connect?org_id=<selected-or-default-org-id>`
- Component style: existing `button` component and `key-round` / `plus` style icon if available。
- Empty-state integration: when there are no OpenAI subscription credentials, show the CTA in the credentials page header and/or empty state。

Organization context:

- Use the same organization source the current UI already exposes to `Credentials`/session/admin context by implementation time。
- If only a master/default org is available in UI context, encode that explicit ID/source in the view model rather than leaving `org_id` empty。
- Do not accept callback query `org_id` as a fallback; HO-1175 requires stored state `org_id` only。

### Callback Result Page

Replace or wrap the existing minimal `writeOpenAIOAuthPage` output with a reusable UI result renderer.

Recommended implementation options:

1. Add `internal/proxy/handler/openai_oauth_page.go` with a small result view model and safe HTML renderer that links existing `/ui/static/css/output.css`。
2. If dependency direction allows, add a `templ` page under `internal/ui/pages/openai_oauth_result.templ` and render it from proxy callback without requiring UI session。

The result page should include:

- Title: success `OpenAI connected`; failure `OpenAI connection failed`。
- Concise safe message。
- Primary action link to `/ui/credentials`。
- Secondary action for retry by returning to `/ui/credentials` instead of embedding a direct retry URL without organization context。
- Optional safe metadata row for account email if already available from saved credential info。

### Error Mapping

Map backend failure cases to stable user-visible messages:

- invalid/missing state: `Connection session is invalid. Start again from TianjiLLM.`
- expired state: `Connection session expired. Start again from TianjiLLM.`
- provider `error=access_denied`: `OpenAI authorization was cancelled or denied.`
- missing code: `OpenAI did not return an authorization code.`
- token exchange failure: `TianjiLLM could not complete the OpenAI token exchange.`
- credential save failure: `TianjiLLM could not save the OpenAI credential.`

Provider-supplied values may be shown only after HTML escaping and existing redaction; raw upstream response bodies must not be rendered.

## Test Plan

### RED first

1. Add failing E2E `test/e2e/openai_connect_flow_test.go` for authenticated connect:
   - login via existing `setup(t)`。
   - ensure `/ui/credentials` shows `Connect OpenAI`。
   - click CTA。
   - assert browser reaches mock authorize endpoint。
   - assert authorize query has `state`, `code_challenge`, `code_challenge_method=S256`, `redirect_uri`。

2. Add failing E2E for callback success:
   - start from UI connect against mock authorize。
   - extract state/code challenge from mock authorize request or redirect URL。
   - visit `/oauth/openai/callback?state=<state>&code=code-ok` with mock token fixture。
   - assert page includes success title/message and `/ui/credentials` action。
   - assert saved credential appears after returning to Credentials page。

3. Add failing E2E for callback failure/error state:
   - provider denial: `/oauth/openai/callback?state=<valid-state>&error=access_denied&error_description=...`。
   - invalid state: `/oauth/openai/callback?state=missing&code=code-ok`。
   - token exchange failure via mock token fixture。
   - assert readable failure UI and no secret material in DOM。

4. Add unauthenticated connect E2E or handler-level UI route test:
   - request `/ui/openai/connect?org_id=...` without login。
   - assert existing redirect/block behavior and no success state。

### Verification Commands

```bash
go test ./test/e2e -tags e2e -run 'TestOpenAIConnect|TestOpenAICallback' -count=1
go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -count=1
go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -count=1
go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -count=1
git diff --check origin/main...HEAD
```

If full E2E needs local DB/browser services, implementation must report the exact environment gate and still keep targeted handler/unit tests mandatory.

## Plan Review Evidence

- Repo evidence: `internal/ui/routes.go` already protects `/ui/openai/connect` with `sessionAuth`; unauthenticated behavior should be tested, not reimplemented。
- Repo evidence: `internal/proxy/handler/openai_oauth.go` currently writes minimal success/failure HTML, making HO-1187 a UI/result-page enhancement rather than OAuth lifecycle rewrite。
- Repo evidence: `internal/ui/pages/credentials.templ` and `layout.templ` already provide `Credentials` sidebar/list/detail surface; connect CTA belongs there by default。
- Repo evidence: `internal/testutil/openaitest` already provides local authorize/token endpoints, so E2E must not call live OpenAI。
- Context7 lookup: `/go-chi/docs` is the relevant router source for grouped protected routes; current repo already follows `r.Group` + `r.Get` patterns。
- Context7 lookup: `/a-h/templ` is the relevant templ source; current repo renders templ components from Go handlers through the existing `render(...)` helper。
- grep.app lookup: public Go searches for `code_challenge_method` show OAuth implementations storing/verifying PKCE challenge metadata server-side; repo-local HO-1175 remains the source of truth for exact implementation。
- Official OpenAI docs lookup: current API authentication docs state secrets must not be exposed client-side and bearer tokens are secret; callback result UI must therefore never display token material. The older indexed OpenAI OAuth Actions page was not available as a stable current page, so OpenAI-specific state behavior is inherited from HO-1175 repo contract and standard OAuth/PKCE pattern rather than quoted as current OpenAI docs。

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| CTA cannot choose org reliably | Use explicit UI/session org source; if implementation cannot find one, stop with a concrete scope blocker before coding. |
| Callback page accidentally depends on `/ui` cookie | Keep callback public and state-authenticated; E2E success must work without UI cookie on callback path. |
| Error page leaks upstream body or token-like strings | Narrow error messages plus DOM secret assertions. |
| Connect button starts live OpenAI in tests | Endpoint overrides + mock authorize/token server + assertion on mock host. |
| UI duplicates credential management scope from HO-1186 | Limit this issue to CTA/result flow; list/detail quota/status remains HO-1186. |

## Scope Confirmation

Owner input required: 0.

Default scope is: add `Connect OpenAI` CTA to the existing Credentials UI, upgrade `/oauth/openai/callback` terminal success/failure pages into safe readable Tianji-styled pages, and prove the flow with offline UI E2E. Production implementation waits for Linear `In Progress`.
