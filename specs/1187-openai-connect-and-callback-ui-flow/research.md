# Research: OpenAI connect and callback UI flow

## Decision 1 - Put Connect CTA on `/ui/credentials`

**Decision**: Default the OpenAI Connect entry point to the existing Credentials UI.

**Rationale**: `internal/ui/pages/layout.templ` already has a `Credentials` sidebar entry and `internal/ui/pages/credentials.templ` is scoped to OpenAI subscription credentials. HO-1186 made this the read-only credential management surface. Adding the connect CTA there avoids creating a parallel OpenAI page.

**Alternatives considered**:

- Add a standalone `/ui/openai` page: rejected because no existing OpenAI UI section exists and it would duplicate Credentials navigation.
- Put connect on Dashboard: rejected because the action manages credentials, not dashboard metrics.

## Decision 2 - Keep backend OAuth lifecycle owned by HO-1175

**Decision**: HO-1187 must not rewrite state, PKCE, code exchange, or credential save internals.

**Rationale**: Current `origin/main` already has `/ui/openai/connect`, `openaioauth.StateStore`, `openai.BuildAuthorizeURL`, public `/oauth/openai/callback`, token exchange, credential save, and backend tests from HO-1175. HO-1187 owns the visible UI flow and E2E integration around that backend.

**Alternatives considered**:

- Rebuild OAuth lifecycle from UI page code: rejected because it duplicates tested backend behavior and risks weakening state security.

## Decision 3 - Upgrade callback terminal pages, not callback auth

**Decision**: Keep `/oauth/openai/callback` public and state-authenticated, but replace its minimal HTML result with Tianji-styled readable success/failure pages.

**Rationale**: `internal/ui/session.go` scopes UI cookies to `/ui`, so callback cannot require UI session cookies. HO-1175 already relies on server-side state for callback auth. The visible gap is the result page quality and user route back to Credentials.

## Decision 4 - E2E must use local OpenAI mock upstream

**Decision**: UI E2E must configure local authorize/token endpoints and assert the mock server receives the browser/token flow.

**Rationale**: `internal/testutil/openaitest` already provides local OAuth endpoints. Real OpenAI credentials/network calls are out of scope and unsafe for CI.

## Evidence

- Linear HO-1187 requires: OpenAI Connect entry point, authenticated connect start, callback success state, callback readable failure page, toast/notification conventions where applicable, UI E2E success/failure/unauthenticated coverage.
- Repo `internal/ui/handler_openai.go`: connect handler exists and requires `org_id` before creating state.
- Repo `internal/ui/routes.go`: `/ui/openai/connect` is inside `sessionAuth`.
- Repo `internal/proxy/server.go`: `/oauth/openai/callback` is public and outside API auth.
- Repo `internal/proxy/handler/openai_oauth.go`: success/failure pages currently use minimal `writeOpenAIOAuthPage`, confirming the UI page gap.
- Repo `internal/ui/pages/credentials.templ`: Credentials list/detail is the natural UI host for the connect CTA.
- Repo `internal/testutil/openaitest`: local OAuth mock supports authorize URL, token URL, token fixtures, and request inspection.
- OpenAI official API authentication docs: OpenAI treats bearer/API credentials as secrets and says not to expose them client-side; this supports DOM redaction requirements for result pages.
- OpenAI docs search: the indexed OpenAI OAuth Actions page mentioning state was not available as a stable current page during this run, so state behavior is sourced from HO-1175 repo contract and standard OAuth/PKCE practice.
- grep.app: public Go search for `code_challenge_method` returned OAuth server implementations with stored PKCE challenge metadata; no repo should override Tianji's local HO-1175 contract.
- Context7 lookup identified `/go-chi/docs` for router groups and `/a-h/templ` for Go templ rendering; Tianji already uses both patterns locally.

## Open Questions

None for Todo scope. Implementation must stop if current UI/session code cannot provide an explicit organization identifier for the CTA.
