# Data Model: HO-1288 ChatGPT Codex backend transport

## `config.TianjiParams`

Existing per-model route parameters.

### Proposed new field

```go
OpenAISubscriptionTransport string `yaml:"openai_subscription_transport,omitempty"`
```

### Values

- Empty: current behavior; existing direct OpenAI subscription/API-key paths remain unchanged.
- `direct_openai_http`: explicit alias for current direct Platform `/v1` behavior if implementation wants a named value.
- `chatgpt_codex_backend`: select the new ChatGPT backend transport.

### Rules

- `chatgpt_codex_backend` requires non-empty `openai_subscription_credential_ids`.
- `chatgpt_codex_backend` must not silently fallback to `api_key`.
- `chatgpt_codex_backend` should reject custom OpenAI-compatible `api_base` unless implementation maps it explicitly to Codex backend base URL config.

## `config.OpenAIOAuthConfig` or equivalent backend config

Existing deployment-level OpenAI OAuth config.

### Proposed backend-specific fields

```go
CodexBackendBaseURL string `yaml:"codex_backend_base_url,omitempty"`
CodexBackendOriginator string `yaml:"codex_backend_originator,omitempty"`
```

### Defaults

- `CodexBackendBaseURL`: `https://chatgpt.com/backend-api`
- `CodexBackendOriginator`: `codex_cli_rs`

### Rules

- The final request URL normalizes to `/codex/responses`.
- These defaults must not change OAuth authorize `Originator` behavior unless explicitly intended and tested.
- Tests may override base URL to `httptest.Server`.

## `resolvedOpenAISubscriptionCredential`

Existing resolved credential struct.

### Existing relevant fields

- `CredentialID`
- `BearerToken`
- `AccountID`
- `CodexLogin`

### HO-1288 usage

For `chatgpt_codex_backend`, use:

- `BearerToken`: access token for `Authorization`.
- `AccountID`: value for `ChatGPT-Account-Id` when present.

Do not use `CodexLogin`; that is app-server login scope.

## `CodexBackendRequest`

Internal request shape for the new transport.

### Fields

- `Model`: normalized backend model ID.
- `Input` or `Messages`: mapped from incoming chat/responses request.
- `Stream`: caller stream setting.
- `Store`: default false unless Tianji explicitly supports store semantics.
- `Instructions`: optional mapped instructions/system content where compatible.
- `ExtraParams`: safe pass-through params supported by current backend contract.

### Rules

- Do not forward raw Tianji credential IDs.
- Do not include access token or refresh token in JSON body.
- Normalize `openai/...` / `chatgpt/...` model prefixes according to tested behavior.

## `CodexBackendResponse`

Internal adapter representation for backend response or SSE events.

### Rules

- Non-streaming response must map to Tianji's existing OpenAI-compatible JSON response.
- Streaming response must map to existing SSE stream chunk behavior, or return a tested safe unsupported error if implementation cannot support streaming in this slice.
- Error responses must be redacted before returning/logging.
