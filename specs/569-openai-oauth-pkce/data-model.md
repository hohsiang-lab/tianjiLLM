# Data Model: OpenAI OAuth PKCE Primitives and Provider Config

## OpenAIOAuthConfig

Deployment-level OpenAI OAuth metadata and defaults.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `Enabled` | bool | no | Later issues may use this gate; HO-1174 only defines config. |
| `IssuerURL` | string | no | Default `https://auth.openai.com`. |
| `AuthorizeURL` | string | no | Default `https://auth.openai.com/oauth/authorize`; full URL override allowed for tests. |
| `TokenURL` | string | no | Default `https://auth.openai.com/oauth/token`; full URL override allowed for tests. |
| `ClientID` | string | no | Default public OpenClaw/PI Codex-style client ID per Linear issue, with config override. |
| `Scopes` | []string | no | Default `openid profile email offline_access api.connectors.read api.connectors.invoke`. |
| `Originator` | string | no | Sent as `originator` authorize query param; provider metadata detail, not a scope blocker. |
| `IDTokenAddOrganizations` | bool | no | Default true; sent as `id_token_add_organizations=true`. |
| `CodexCLISimplifiedFlow` | bool | no | Default true; sent as `codex_cli_simplified_flow=true`. |
| `PublicBaseURL` | string | yes for enabled OAuth | Used to derive redirect URI from the Tianji public base URL concept in the Linear issue. |

## PKCEPair

| Field | Type | Notes |
|-------|------|-------|
| `CodeVerifier` | string | URL-safe verifier generated per auth attempt; RFC 7636 length bounds. |
| `CodeChallenge` | string | S256 challenge: base64url-no-padding(SHA256(verifier)). |
| `CodeChallengeMethod` | string | Always `S256`. |

## OAuthState

| Field | Type | Notes |
|-------|------|-------|
| `Value` | string | Secure random URL-safe state generated per auth attempt. |

## OpenAITokenBundle

Typed successful token exchange response returned to later credential-persistence slices.

| Field | Type | Notes |
|-------|------|-------|
| `AccessToken` | string | Bearer token. Secret. |
| `RefreshToken` | string | Refresh token. Secret. |
| `IDToken` | string | Optional OIDC ID token. Secret-ish; store encrypted later. |
| `TokenType` | string | Usually `Bearer`. |
| `ExpiresIn` | int | Provider response seconds. |
| `Scope` | string | Scope string if returned. |
| `Raw` | map/string JSON | Optional raw payload for later metadata extraction without losing fields. |

## Validation Rules

- `client_secret` is not a field of `OpenAIOAuthConfig` for this feature.
- `AuthorizeURL` and `TokenURL` overrides must parse as absolute URLs.
- `PublicBaseURL` must parse as absolute URL when OAuth is enabled.
- Production redirect URI must be HTTPS.
- Dev/test may use HTTP only for localhost or loopback hostnames/IPs.
- Redirect URI path is always `/oauth/openai/callback`.
