# Data Model: OpenAI connect and callback UI flow

## OpenAI Connect Entry Point

User-visible action rendered from the protected UI.

Fields:

- `label`: `Connect OpenAI`
- `href`: `/ui/openai/connect?org_id=<organization_id>`
- `organization_id`: explicit current/default org identifier selected before connect start
- `disabled_reason`: optional safe UI message if OAuth config or org context is unavailable

Rules:

- Must render only for authenticated UI pages.
- Must not embed token material, client secret, state, code verifier, or raw OAuth config.
- Must not construct callback URL client-side.

## OAuth Callback Result View

Terminal page rendered by `/oauth/openai/callback`.

Fields:

- `status`: `success` or `failure`
- `title`: safe user-facing title
- `message`: safe user-facing message
- `detail`: optional redacted/escaped detail
- `primary_action_label`: e.g. `Back to Credentials`
- `primary_action_href`: `/ui/credentials`
- `safe_email`: optional safe account email if already available

Rules:

- Must be renderable without a UI session cookie.
- Must link existing UI CSS/assets without requiring authenticated `/ui` data.
- Must HTML-escape all provider-supplied strings.
- Must pass existing redaction before rendering any error details.
- Must not include raw `access_token`, `refresh_token`, `id_token`, authorization code, PKCE verifier, bearer string, JWT, encrypted credential blob, or raw upstream response.

## Mock OAuth Test Flow

E2E-only model using local mock server.

Fields:

- `authorize_url`: local mock `/oauth/authorize`
- `token_url`: local mock `/oauth/token`
- `code`: fixture authorization code
- `token_response`: fixture JSON with safe account metadata and synthetic tokens
- `recorded_authorize_query`: query captured from browser redirect
- `recorded_token_form`: form captured from server-side exchange

Rules:

- Must never call live OpenAI hosts.
- Must assert PKCE query shape on authorize.
- Must assert no client secret/basic auth in token exchange through existing mock helpers where applicable.

## Relationship to Existing Models

- `openaioauth.StateRecord`: existing backend state record; HO-1187 reads behavior through tests but should not change shape unless a real UI-flow bug requires it.
- `CredentialTable`: successful callback persists an OpenAI subscription credential through existing backend helper; HO-1187 may verify list visibility but must not broaden CRUD semantics.
- `CredentialsPageData`: should gain a connect action view field if implementation needs clean page rendering.
