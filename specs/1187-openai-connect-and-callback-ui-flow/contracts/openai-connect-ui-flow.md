# Contract: OpenAI connect and callback UI flow

## GET `/ui/credentials`

Authenticated UI page.

Expected UI additions:

- Renders `Connect OpenAI` action.
- Action target is `/ui/openai/connect?org_id=<organization_id>`.
- Existing credentials list/detail behavior remains compatible.

Security:

- No token material in DOM.
- No OAuth state/code verifier generated client-side.

## GET `/ui/openai/connect?org_id=<organization_id>`

Existing authenticated UI start endpoint.

Expected behavior:

- Requires UI session.
- Requires non-empty `org_id`.
- Creates server-side state and PKCE verifier through existing helpers.
- Redirects to configured OpenAI authorize URL.

E2E assertions:

- Unauthenticated request redirects/blocks through existing UI auth.
- Authenticated request reaches mock authorize URL.
- Authorize URL includes `state`, `code_challenge`, `code_challenge_method=S256`, `redirect_uri`, `client_id`, and scope.

## GET `/oauth/openai/callback`

Public callback route, authenticated by server-side OAuth state.

Success:

- Consumes state.
- Exchanges code with stored PKCE verifier.
- Saves OpenAI subscription credential through existing helper.
- Renders readable success UI.
- Provides action to `/ui/credentials`.

Failure:

- Renders readable safe failure UI for provider error, invalid/expired state, missing code, exchange failure, and save failure.
- Does not expose raw upstream body or secret-bearing values.

Forbidden output:

- `access_token`
- `refresh_token`
- `id_token`
- authorization code
- PKCE verifier
- bearer string
- JWT
- encrypted credential value
- raw upstream response body
