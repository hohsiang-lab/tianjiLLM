# Research: OpenAI Subscription Config and Credential Resolution

## Repo Findings

### Current config model

`internal/config/config.go` defines `TianjiParams` with `Model`, `APIKey`, `APIBase`, `APIVersion`, rate limits, region, auto-router fields, and `Overflow`. There is no typed `openai_subscription_credential_ids` field yet.

**Decision**: Add a typed `[]string` field instead of reading `Overflow`.

**Rationale**: The field affects auth behavior and must be validated. Leaving it in `Overflow` would only warn and then ignore it.

### Current loader behavior

`internal/config/loader.go` resolves env vars for `APIKey`, `APIBase`, and `APIVersion`, then validates the config. Existing env interpolation must remain unchanged for no-subscription configs.

**Decision**: Do not env-expand subscription credential IDs. Treat IDs as opaque internal credential IDs.

**Rationale**: IDs are selectors, not secrets. Expanding them from env would blur config identity with secret material and complicate explicitness.

### Current validation behavior

`internal/config/validate.go` logs warnings for overflow fields and does not currently return config validation errors. HO-1167 needs hard failures for unsafe combinations.

**Decision**: Add targeted validation errors for `openai_subscription_credential_ids` while preserving general LiteLLM overflow tolerance.

**Rationale**: Context7 `/yaml/go-yaml` docs show strict known-field validation is possible, but this repo intentionally accepts unknown LiteLLM-compatible fields. A focused validator avoids breaking config compatibility.

### Current provider resolution

`internal/proxy/handler/handler.go`, `forward.go`, `native_upstream.go`, and `internal/router/router.go` currently build providers and upstreams from `TianjiParams.Model`, `APIBase`, and `APIKey`.

**Decision**: Insert a resolver boundary after deployment/model selection and before provider credential application.

**Rationale**: Deployment selection should remain router-owned. Credential resolution depends on selected deployment plus transport, so it belongs at the handler/app-server boundary.

### Current Codex app-server state

Repo search found OpenAI OAuth/Codex docs/spec artifacts and pricing references, but no current Tianji app-server implementation using `chatgptAuthTokens`.

**Decision**: Plan a small app-server auth helper package or local helper when implementation starts.

**Rationale**: Direct HTTP bearer auth and Codex app-server account login are different credential application paths and should not share a header-injection function.

## External Findings

### OpenAI Codex app-server auth surface

OpenAI Codex app-server README documents JSON-RPC auth/account methods:

- `account/read`
- `account/login/start`
- `account/login/completed`
- `account/logout`
- `account/updated`

It documents API-key login and ChatGPT managed login through `account/login/start`.

Source: <https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md#auth-endpoints>

**Decision**: Use `account/login/start` as the app-server auth handoff boundary.

**Rationale**: App-server auth state is controlled through JSON-RPC account methods, not by injecting OpenAI bearer headers into the app-server transport.

### `chatgptAuthTokens` protocol shape

OpenAI Codex protocol source includes `ChatgptAuthTokens` under `LoginAccountParams` and marks it experimental/internal-use. The same source says external-auth clients should refresh tokens themselves and call `account/login/start` with `chatgptAuthTokens`.

Source: <https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/account.rs>

**Decision**: Treat `chatgptAuthTokens` as an implementation-time protocol contract that must be verified against the checked Codex app-server version.

**Rationale**: The Linear scope explicitly requires this path, but the public README does not present it as stable. Tests must pin the versioned payload shape.

### Environment API key risk

Public openai/codex issues report cases where `OPENAI_API_KEY` in the environment overrides or confuses ChatGPT/OAuth auth mode.

Source: <https://github.com/openai/codex/issues/15151>

**Decision**: Strip `CODEX_API_KEY` and `OPENAI_API_KEY` from local stdio app-server env when a subscription-style profile is selected.

**Rationale**: HO-1167 must ensure selected account credentials are authoritative and not shadowed by env API-key fallback.

## Alternatives Considered

### Alternative A - Auto-consume all org OpenAI subscription credentials

Rejected.

This violates the Linear scope. Credential IDs must be explicit per model config.

### Alternative B - Allow subscription credentials with custom `api_base`

Rejected.

OpenAI subscription credentials are for official OpenAI account auth. Custom OpenAI-compatible providers remain API-key based.

### Alternative C - Reuse direct HTTP bearer injection for Codex app-server

Rejected.

Codex app-server has its own account/auth JSON-RPC surface. Bearer injection against the app-server transport would mix app-server connection auth with OpenAI account auth.

### Alternative D - Enable strict YAML known-field validation globally

Rejected for this issue.

The repo intentionally accepts LiteLLM-style overflow fields. HO-1167 only needs targeted hard validation for this security-sensitive auth field.

## Open Questions

None blocking spec scope. Implementation must verify the exact `chatgptAuthTokens` payload for the vendored or invoked Codex app-server version before coding the app-server helper.
