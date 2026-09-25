# Data Model: Inject OpenAI Subscription Auth Across Official Endpoints

## Existing Entities

### `config.TianjiParams`

- `Model`: resolved provider/model target.
- `APIKey`: existing API-key material or env-resolved value.
- `APIBase`: custom base URL. Non-empty custom values are incompatible with subscription auth.
- `OpenAISubscriptionCredentialIDs`: explicit ordered credential IDs introduced by HO-1167.

### `CredentialTable`

- Existing durable credential row.
- `credential_type = "openai_subscription"` identifies OpenAI subscription credentials.
- `credential_value` stores encrypted token bundle.
- `credential_info` stores safe metadata.

### OpenAI Subscription Token Bundle

- `access_token`: bearer token used for official OpenAI upstream requests.
- `refresh_token`: secret used only by refresh manager, never sent to API endpoints.
- `expires_at`: used by HO-1177 to refresh before expiry.
- `account_id`: used by Codex app-server flow and metadata.

### Resolved OpenAI Subscription Credential

Existing runtime object in `internal/proxy/handler/openai_subscription_resolution.go`.

- `CredentialID`
- `BearerToken`
- `AccountID`
- `CodexLogin`

HO-1178 uses only direct HTTP `BearerToken` for official OpenAI endpoints.

## New Data

No new persistent data model is required.

Implementation may introduce internal helper structs for endpoint auth resolution, for example:

- `officialOpenAIUpstreamAuth`
  - `BaseURL`
  - `BearerToken`
  - `ModelName`
  - `ProviderName`
  - `UsesSubscription`

This helper must remain process-local and must not persist or expose token material.

## Invariants

- Subscription IDs configured -> no API-key fallback after any resolution failure.
- Subscription IDs omitted/empty -> API-key behavior unchanged.
- `APIBase` custom -> subscription auth never applied.
- Refresh token, ID token, client virtual key, and code verifier never become upstream official API auth.
- Only the resolved access token can become `Authorization: Bearer ...` for direct official OpenAI HTTP.
