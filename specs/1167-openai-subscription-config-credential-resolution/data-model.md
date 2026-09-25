# Data Model: OpenAI Subscription Config and Credential Resolution

## Config Field

### `TianjiParams.OpenAISubscriptionCredentialIDs`

| Field | Type | YAML | Required | Notes |
|-------|------|------|----------|-------|
| `OpenAISubscriptionCredentialIDs` | `[]string` | `openai_subscription_credential_ids` | no | Empty or omitted means existing API-key path. Non-empty enables subscription credential resolution. |

Validation:

- Omitted and `[]` are equivalent.
- Empty strings are invalid.
- Duplicate IDs are invalid.
- Non-empty IDs require OpenAI/default-base provider.
- Non-empty IDs are invalid with non-empty custom `api_base`.

Example:

```yaml
model_list:
  - model_name: gpt-5-codex-subscription
    tianji_params:
      model: openai/gpt-5.2-codex
      openai_subscription_credential_ids:
        - cred_openai_subscription_main
```

## Existing Credential Row

Owned by HO-1176 and reused here.

| Column | Requirement |
|--------|-------------|
| `credential_id` | Must match one configured ID exactly. |
| `credential_type` | Must be `openai_subscription`. |
| `credential_value` | Encrypted token/account bundle. Never returned raw. |
| `credential_info` | Safe metadata, status, and refresh/error fields. |

Expected safe metadata shape:

```json
{
  "email": "user@example.com",
  "scopes": ["openid", "profile"],
  "status": "active",
  "last_refresh_at": "2026-05-07T00:00:00Z",
  "last_error": "",
  "disabled_reason": ""
}
```

## Resolver Inputs

```go
type OpenAISubscriptionResolutionInput struct {
    ModelName string
    ProviderName string
    APIBase string
    APIKey string
    CredentialIDs []string
    OrganizationID string
    Transport CredentialTransport
}
```

`APIKey` remains present for the no-subscription path. When `CredentialIDs` is non-empty, resolver failures must not return `APIKey` fallback.

## Resolver Outputs

```go
type CredentialTransport string

const (
    CredentialTransportDirectOpenAIHTTP CredentialTransport = "direct_openai_http"
    CredentialTransportCodexAppServer   CredentialTransport = "codex_app_server"
)

type ResolvedOpenAICredential struct {
    CredentialID string
    AccountID string
    AccessToken string
    ExpiresAt time.Time
    Transport CredentialTransport
}
```

The `AccessToken` is secret material and must stay inside auth/injection boundaries.

## Direct HTTP Application

Direct OpenAI HTTP paths consume:

```go
type DirectOpenAIHTTPAuth struct {
    BaseURL string
    BearerToken string
    SourceCredentialID string
}
```

This issue creates the resolver contract. Later route/injection tickets can attach `BearerToken` to outbound OpenAI HTTP requests.

## Codex App-Server Application

Codex app-server paths consume:

```go
type CodexAppServerLogin struct {
    Method string // account/login/start
    Params CodexChatGPTAuthTokensParams
}

type CodexChatGPTAuthTokensParams struct {
    Type string // chatgptAuthTokens
    AccessToken string
    ChatGPTAccountID string
    ChatGPTPlanType string
}
```

Implementation must verify exact field names against the checked OpenAI Codex app-server protocol version.

## Local Stdio App-Server Env

Input environment:

```go
map[string]string{
    "CODEX_API_KEY": "...",
    "OPENAI_API_KEY": "...",
    "OTHER_VAR": "keep",
}
```

Subscription-style output:

```go
map[string]string{
    "OTHER_VAR": "keep"
}
```

API-key style output keeps existing behavior.

## WebSocket App-Server Auth Separation

```go
type CodexAppServerConnectionAuth struct {
    AuthToken string
    Headers map[string]string
}
```

This authenticates Tianji to the app-server transport only. It must not replace the account login RPC used for OpenAI subscription credentials.
