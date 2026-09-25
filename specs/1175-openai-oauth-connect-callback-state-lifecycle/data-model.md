# Data Model: OpenAI OAuth Connect/Callback State Lifecycle

## Ephemeral Entity: OpenAIOAuthStateRecord

Stored as JSON bytes in `cache.Cache` under a prefixed key such as `openai_oauth_state:{state}`.

```json
{
  "state": "random-url-safe-state",
  "code_verifier": "random-url-safe-pkce-verifier",
  "org_id": "org_123",
  "created_at": "2026-05-07T00:00:00Z",
  "expires_at": "2026-05-07T00:10:00Z"
}
```

### Validation

- `state` required, non-empty, exact match with requested callback state.
- `code_verifier` required, non-empty.
- `org_id` required, non-empty.
- `created_at` required for diagnostics.
- `expires_at` required and must be in the future when consumed.
- Unknown fields may be ignored, but required field validation must fail closed.

### Lifecycle

```text
Create
  -> generate state + code_verifier
  -> compute expires_at = now + ttl
  -> cache.Set(key(state), recordJSON, ttl)

Consume
  -> cache.Get(key(state))
  -> parse and validate record
  -> cache.Delete(key(state)) after found terminal use
  -> return record or safe invalid/expired error
```

## Request Entity: OpenAIConnectRequest

Browser request to authenticated UI endpoint:

```text
GET /ui/openai/connect?org_id=org_123
```

### Fields

| Field | Source | Trust | Use |
|-------|--------|-------|-----|
| `org_id` | UI request query/form | trusted only after UI session and validation | copied into server-side state |
| UI session cookie | `/ui` cookie | trusted by existing `sessionAuth` | authorizes connect start |

## Request Entity: OpenAICallbackRequest

Browser redirect from OpenAI:

```text
GET /oauth/openai/callback?state=...&code=...
GET /oauth/openai/callback?state=...&error=access_denied&error_description=...
```

### Fields

| Field | Source | Trust | Use |
|-------|--------|-------|-----|
| `state` | callback query | lookup key only | exact lookup of server-side state record |
| `code` | callback query | accepted only after valid state | exchanged with stored PKCE verifier |
| `error` | callback query | untrusted display input | sanitized readable failure page |
| `error_description` | callback query | untrusted display input | sanitized readable failure page |
| `org_id` / `organization_id` | callback query/body | untrusted | ignored |
| UI session cookie | absent or ignored | not required | no callback authorization dependency |

## Durable Entity: OpenAISubscriptionCredential

Existing credential row created through HO-1176 helper after successful token exchange.

### Required Callback Inputs

- `org_id` from `OpenAIOAuthStateRecord`.
- `openai.TokenBundle` from `openai.ExchangeCode`.
- safe metadata extracted by existing/future credential helper.

### Trust Rule

The durable credential's organization scope must derive from `OpenAIOAuthStateRecord.org_id`. Callback query values must not override it.

## Cache Key Contract

```text
openai_oauth_state:{state}
```

Rules:

- Prefix is fixed and specific to OpenAI OAuth state.
- State value is the exact random state string from connect.
- No prefix search or list operation is required.
- `Delete` must be called for consumed/malformed/expired records when a cache entry is found.

## Error Page Model

```json
{
  "title": "OpenAI connection failed",
  "message": "The OpenAI connection expired. Please start again from the TianjiLLM UI.",
  "status_code": 400
}
```

Forbidden page content:

- `access_token`
- `refresh_token`
- `id_token`
- `code_verifier`
- raw authorization code
- raw token endpoint response
- encrypted credential value
