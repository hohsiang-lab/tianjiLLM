# Data Model: Subscription wildcard routing regression verification

## SubscriptionWildcardRouteFixture

Represents the in-test model configuration that should route through ChatGPT Codex backend.

Fields:

- `model_name`: `openai/*`
- `tianji_params.model`: `openai/*`
- `tianji_params.openai_subscription_credential_ids`: non-empty list, e.g. `["cred-a"]`
- `tianji_params.openai_subscription_transport`: `chatgpt_codex_backend`
- `general_settings.openai_oauth.codex_backend_base_url`: local `httptest.Server` URL

Rules:

- Must not include custom `api_base` because subscription credentials reject custom API bases.
- Must use fake token material only.
- Must assert wildcard captured model resolves to backend model without losing the requested suffix.

## APIKeyWildcardRouteFixture

Represents normal OpenAI-compatible wildcard behavior that must not select Codex backend.

Fields:

- `model_name`: `openai/*`
- `tianji_params.model`: `openai/*`
- `tianji_params.api_key`: fake API key such as `sk-api-key-sentinel`
- `tianji_params.api_base`: local Platform mock base URL
- `tianji_params.openai_subscription_credential_ids`: empty
- `tianji_params.openai_subscription_transport`: empty

Rules:

- Must route to the OpenAI provider's normal chat-completions path.
- Must not call Codex backend.
- Must preserve existing OpenAI-compatible response shape.

## CodexBackendMock

Local test server standing in for `chatgpt.com/backend-api/codex/responses`.

Captured fields:

- request path
- method
- redacted header names and safe selected values
- request body model/input shape
- call count

Responses:

- success response with `output_text` and usage
- 401 auth error
- 403 missing-scope error
- 429 quota/rate-limit error plus optional `x-ratelimit-*` headers

Rules:

- May assert `Authorization` exists but must not print the bearer value on failure.
- Should expose helper methods that return redacted diagnostics.

## PlatformWrongRouteMock

Local server or guarded transport standing in for the wrong `api.openai.com/v1/chat/completions` path.

Captured fields:

- request path
- method
- whether subscription token sentinel appeared
- call count

Rules:

- Any subscription wildcard call to `/v1/chat/completions` is a test failure.
- Failure message must include host/path only, not token value.

## SecretSentinel

Fake values used to prove no leakage.

Examples:

- subscription access token: `access-secret-sentinel`
- refresh token: `refresh-secret-sentinel`
- API key: `sk-api-key-sentinel`

Rules:

- These strings must not appear in external response bodies, failure diagnostics, deployment docs, or PR summary text.
- Tests may compare internal captured values directly but should redact them in messages.
