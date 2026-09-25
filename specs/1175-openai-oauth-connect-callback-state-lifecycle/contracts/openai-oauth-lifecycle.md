# Contract: OpenAI OAuth Connect/Callback State Lifecycle

## UI Connect Endpoint

```text
GET /ui/openai/connect?org_id={org_id}
```

### Authentication

- Requires existing UI session auth.
- Unauthenticated browser requests follow existing UI behavior:
  - normal browser request: redirect to `/ui/login`
  - HTMX request: unauthorized response with `HX-Redirect: /ui/login`

### Success Behavior

- Validate `org_id`.
- Generate `state` and PKCE verifier.
- Store `OpenAIOAuthStateRecord` server-side with TTL.
- Redirect with HTTP 303 to the configured OpenAI authorize URL.

### Failure Behavior

- Missing/invalid `org_id`, invalid public base URL, cache write failure, or authorize URL build failure returns a readable UI error.
- No state may be stored when validation fails before state creation.

## Public Callback Endpoint

```text
GET /oauth/openai/callback?state={state}&code={code}
GET /oauth/openai/callback?state={state}&error={error}
```

### Authentication

- Does not require API-key auth.
- Does not require UI cookie auth.
- Requires valid server-side OAuth state before code exchange.

### Success Behavior

- Load and consume the state record by exact `state`.
- Reject missing, expired, malformed, or mismatched state before token exchange.
- Exchange `code` using the stored PKCE verifier.
- Persist the token bundle through OpenAI subscription credential persistence using stored `org_id`.
- Render readable success HTML.

### Provider Error Behavior

- If `error` is present and state is valid, consume the state.
- Render readable error HTML with sanitized provider error text.
- Do not call token exchange.

### Token Exchange Error Behavior

- If state was loaded and token exchange fails, consume/delete the loaded state.
- Render readable error HTML.
- Do not include raw upstream response or secret values in the page.

## State Store API Contract

Future implementation API shape may vary, but it must satisfy:

```go
type StateRecord struct {
    State        string
    CodeVerifier string
    OrgID        string
    CreatedAt    time.Time
    ExpiresAt    time.Time
}

type StateStore interface {
    Create(ctx context.Context, orgID string, ttl time.Duration) (StateRecord, error)
    Consume(ctx context.Context, state string) (StateRecord, error)
}
```

Required behavior:

- `Create` generates fresh random state and PKCE verifier for each call.
- `Create` writes the record to `cache.Cache` with TTL before authorize redirect.
- `Consume` requires exact state key match.
- `Consume` deletes records after successful load.
- `Consume` deletes found expired/malformed records and returns safe invalid-state errors.
- Errors must not include code verifier or token values.

## Persistence Contract

Callback must pass stored organization scope to existing credential persistence:

```text
stored_state.org_id -> credential organization_id
```

Forbidden persistence inputs:

- callback query `org_id`
- callback query `organization_id`
- callback query `credential_id`
- request body organization fields

## Redaction Contract

Callback success/error pages and logs in this lifecycle must not expose:

- `access_token`
- `refresh_token`
- `id_token`
- `code_verifier`
- raw authorization code
- encrypted credential value
- raw token endpoint response body
