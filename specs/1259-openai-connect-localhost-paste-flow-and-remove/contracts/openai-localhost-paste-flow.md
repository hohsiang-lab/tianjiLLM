# Contract: OpenAI localhost paste flow

## GET `/ui/openai/connect`

Protected UI route. This route may still respond with an HTTP redirect, but the Credentials UI must open it in a new tab/window so the original Tianji tab remains on `/ui/credentials` with the paste modal available.

### Query

- `org_id` or `organization_id`: required existing organization ID.

### Behavior

- Resolve OpenAI OAuth redirect URI from `general_settings.openai_oauth.redirect_uri`, defaulting to `http://localhost:1455/auth/callback`.
- Create `openaioauth.StateRecord` with exact redirect URI.
- Redirect to configured OpenAI authorize URL.
- The triggering link/button in Tianji UI must use `target="_blank"` or equivalent, with `rel="noopener"` for tab isolation.

### Authorize query requirements

- `response_type=code`
- `client_id=<resolved OpenAI OAuth client id>`
- `redirect_uri=<record.RedirectURI>`
- `scope=<resolved scopes>`
- `code_challenge=<S256 challenge>`
- `code_challenge_method=S256`
- `state=<record.State>`
- existing configured OpenAI flags such as `id_token_add_organizations` and `codex_cli_simplified_flow`

## GET `/oauth/openai/callback`

Public direct callback route.

### Query

- `state`: required.
- `code`: required for success path.
- `error`: provider terminal error.
- `error_description`: optional provider error detail.

### Behavior

- Parse query values.
- Call shared callback completion helper.
- Use stored `record.RedirectURI` for token exchange.
- Render safe success/failure page.

## `/ui/credentials` Paste Callback Modal

Protected UI surface.

### Behavior

- Render a `Paste callback URL` button near `Connect OpenAI`.
- Opening the button shows a modal/dialog using the existing Tianji dialog component pattern.
- The modal contains one textarea/input for the full localhost callback URL and submits to the protected paste endpoint.
- The original `/ui/credentials` page remains available while the OpenAI authorize tab is open.

## GET `/ui/openai/callback-url`

Protected UI route or HTMX partial endpoint.

### Behavior

May render the paste modal body/partial or a fallback protected paste form. It is not the primary user navigation surface; `/ui/credentials` owns the primary paste UX.

## POST `/ui/openai/callback-url`

Protected UI route.

### Form fields

- `callback_url`: required full URL.

### Accepted input

```text
http://localhost:1455/auth/callback?code=<code>&state=<state>
http://localhost:1455/auth/callback?error=<error>&state=<state>
```

### Behavior

- Parse URL server-side.
- Validate scheme/host/port/path against configured redirect contract.
- Extract state/code/error fields.
- Call shared callback completion helper.
- Render safe success/failure page.

### Security

- Do not echo `callback_url`.
- Do not log raw `callback_url`.
- Do not audit raw `callback_url`, `code`, `state`, `code_verifier`, or tokens.
- Do not trust form/query org values.
