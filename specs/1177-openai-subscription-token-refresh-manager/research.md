# Research: OpenAI Subscription Token Refresh Manager

## Repo Findings

- `internal/proxy/handler/openai_subscription_resolution.go` currently loads exactly the configured credential ID, validates `credential_type=openai_subscription`, decrypts the bundle, rejects disabled metadata, and returns `credential expired` when `ExpiresAt` is not after `time.Now()`.
- `resolveOpenAIAPIKeyForParams` is already used by chat/forward/direct OpenAI paths; this is the right integration point for refreshed bearer material.
- Codex app-server resolution already builds `chatgptAuthTokens` from the stored bundle through `internal/codexapp/auth.go`; it must receive refreshed access tokens.
- `internal/proxy/handler/credentials.go` already implements encrypted create/update helpers and preserves old refresh token when a refresh response omits `refresh_token`.
- `internal/provider/openai/oauth.go` supports authorization-code exchange but has no refresh-token helper.
- `internal/testutil/openaitest/oauth_server.go` already supports refresh fixtures keyed by `grant_type=refresh_token` and `refresh_token`.
- `internal/security/redact` already redacts token field names, bearer tokens, JWTs, API keys, and secret key/value pairs.

## Memory and Wiki Recall

- `memory_search("HO-1177 openai-subscription-token-refresh-manager...")`: no hits.
- `memory_search("OpenAI subscription credential refresh token rotate lock...")`: no hits.
- `wiki_search("OpenAI subscription credential refresh token rotation...")`: no hits.
- Decision: use repo-adjacent specs and current code as the source of continuity rather than long-term memory.

## External Evidence

### OpenAI / Codex

- OpenAI Codex app-server README documents `account/read` with a `refreshToken` flag and says managed ChatGPT auth persists and refreshes tokens automatically.
- OpenAI Codex app-server protocol source defines `chatgptAuthTokens` as an experimental login type and states that in external auth mode `refresh_token` is ignored by `account/read`; clients should refresh tokens themselves and call `account/login/start` with `chatgptAuthTokens`.
- OpenAI Codex login source persists `access_token`, `refresh_token`, optional `account_id`, and `last_refresh` after the OAuth exchange.
- OpenAI API key safety guidance reinforces server-side credential handling and rotation; this supports keeping refresh logic backend-only and never exposing tokens.

### Context7 / Go Docs

- Context7 `/websites/pkg_go_dev_golang_org_x_sync`: `singleflight.Group.Do` allows one in-flight function per key; duplicate callers wait and share the result.
- Context7 `/golang/oauth2`: `ReuseTokenSourceWithExpiry` uses a configurable early-expiry buffer before token expiry; this plan adopts the buffer behavior while keeping Tianji's custom OpenAI request shape.
- pkg.go.dev search confirmed current docs for `golang.org/x/sync/singleflight` and `golang.org/x/oauth2`.

### grep-app

- `/Users/n0rmanc/.cargo/bin/grep-app-cli --json 'singleflight.Group' --language Go` failed with transport `Unexpected content type: text/html`.
- `/Users/n0rmanc/.cargo/bin/grep-app-cli --json 'grant_type", "refresh_token"' --language Go` failed with the same transport error.
- No public GitHub pattern was adopted.

## Decisions

### R-001: Keep Refresh Manager in `internal/proxy/handler`

**Decision**: Implement the manager near `openai_subscription_resolution.go`.

**Rationale**: The manager needs handler DB access, master key, OpenAI OAuth config, existing update helpers, redaction, and resolver integration. A separate package would force broad interface extraction before implementation proves it is useful.

**Alternatives**:

- Provider package owns persistence: rejected because provider code should not know DB/encryption.
- DB package owns refresh: rejected because token endpoint and redaction behavior belong outside sqlc.

### R-002: Add Provider-Level Refresh Helper

**Decision**: Add `openai.RefreshToken(...)` beside `ExchangeCode`.

**Rationale**: Existing code already owns OpenAI token URL/client ID/request shape in `internal/provider/openai`. The test harness can verify public-client refresh requests with no `client_secret`.

**Alternatives**:

- Use `oauth2.Config.TokenSource`: rejected for first pass because Tianji's existing helper preserves raw response metadata, endpoint overrides, and public-client no-secret assertions.

### R-003: Default Refresh Buffer

**Decision**: Use a default `5 * time.Minute` refresh buffer.

**Rationale**: The Linear issue says "normal expiry buffer"; Context7 oauth2 docs confirm early-expiry buffering is standard. Five minutes is conservative for one-hour token lifetimes and simple to test.

**Alternatives**:

- No buffer: rejected because requests can race expiry.
- Configurable buffer now: deferred until operators need it; a constant keeps this issue smaller.

### R-004: Per-Credential `singleflight`

**Decision**: Use `singleflight.Group` keyed by `credential_id`.

**Rationale**: Context7/pkg.go.dev document the exact duplicate-suppression behavior needed. `golang.org/x/sync` is already in `go.mod` indirectly.

**Alternatives**:

- Custom map of mutexes: rejected because it is more code and easy to leak locks.
- Global mutex: rejected because one credential would block all other credentials.
- DB advisory/distributed lock: deferred because current scope can be proven with in-process tests and no deployment evidence requires cross-process suppression.

### R-005: Failure Metadata Does Not Auto-Disable

**Decision**: Refresh failures set `status=refresh_failed` and redacted failure fields; they do not change status to `disabled` by default.

**Rationale**: Network/5xx/rate-limit failures can be transient. Existing `disabled` should remain an operator/explicit terminal state unless a later policy ticket defines terminal refresh classification.

**Alternatives**:

- Disable on any refresh failure: rejected because it would create unnecessary manual recovery after transient errors.

### R-006: No Schema Migration

**Decision**: Do not plan a schema migration.

**Rationale**: HO-1176 already added/uses `UpdateCredentialValueAndInfo` and `UpdateCredentialInfo` over the existing `CredentialTable` columns needed for this issue.

**Alternatives**:

- Add refresh lock/status columns: rejected for Todo scope; not needed for in-process on-demand refresh.
