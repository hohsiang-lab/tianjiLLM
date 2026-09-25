# Data Model: HO-1259 OpenAI localhost paste flow

## `config.OpenAIOAuthConfig`

Existing typed config for OpenAI OAuth endpoints/client metadata.

### New / changed field

```go
RedirectURI string `yaml:"redirect_uri,omitempty"`
```

### Rules

- Empty value resolves to `http://localhost:1455/auth/callback`.
- Non-empty value must be absolute URL.
- Runtime OpenAI OAuth must not read `GeneralSettings.PublicBaseURL`.
- Tests may override redirect URI for mock or future modes.

## `openaioauth.StateRecord`

Cache-backed one-time OAuth state record.

### Existing fields

- `State string`
- `CodeVerifier string`
- `OrgID string`
- `CreatedAt time.Time`
- `ExpiresAt time.Time`

### New field

```go
RedirectURI string `json:"redirect_uri"`
```

### Validation

- `State` exactly matches cache key suffix.
- `CodeVerifier` is non-empty.
- `OrgID` is non-empty.
- `RedirectURI` is non-empty and absolute.
- `CreatedAt` / `ExpiresAt` are non-zero.
- `now < ExpiresAt`.

### Lifecycle

1. `Create(ctx, orgID, redirectURI, ttl)` generates state + PKCE and stores record.
2. `Consume(ctx, state)` loads, validates, deletes, and returns record.
3. Malformed/expired records are deleted and reported as invalid/expired state.

## `OpenAIOAuthCallbackInput`

Internal parsed callback value object for shared completion logic.

```go
type OpenAIOAuthCallbackInput struct {
    State string
    Code string
    ProviderError string
    ProviderErrorDescription string
    Source string
}
```

### Sources

- `direct_callback`: values parsed from `GET /oauth/openai/callback`.
- `pasted_url`: values parsed from the protected `/ui/credentials` paste modal/form.

### Rules

- `State` is required for all terminal paths.
- `Code` is required for success path.
- `ProviderError` is terminal and must not trigger token exchange.
- `ProviderErrorDescription` may influence safe display only after redaction and escaping.

## `PastedCallbackURL`

User-submitted full URL copied from browser address bar.

### Accepted shape

```text
http://localhost:1455/auth/callback?code=...&state=...
http://localhost:1455/auth/callback?error=...&state=...
```

### Rejected shape

- malformed URL
- unsupported scheme
- unexpected host or port
- unexpected path
- missing state
- success URL missing code
- URL fragment carrying OAuth fields
- raw hosted Tianji callback URL when localhost mode is configured

### Security

The raw pasted URL is sensitive. It must not be logged, audited, rendered back, or included in error messages.
