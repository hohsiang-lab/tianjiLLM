# Research: OpenAI OAuth Connect/Callback State Lifecycle

## R-001: Route Placement

**Decision**: Register `/ui/openai/connect` inside the existing UI `sessionAuth` route group and register `/oauth/openai/callback` as a separate public route in `internal/proxy/server.go`.

**Rationale**: Repo inspection shows UI pages live under `/ui` and are protected by `h.sessionAuth`, while the UI cookie is scoped to `/ui`. The callback path is `/oauth/openai/callback`, so it cannot rely on that cookie. The callback must be public and protected by server-side state instead.

**Alternatives considered**:

- Put callback under `/ui/openai/callback`: rejected because HO-1174 redirect helper and Linear HO-1175 specify `/oauth/openai/callback`.
- Put connect under API-key auth: rejected because Linear explicitly says UI-started authenticated UI session.

## R-002: State Storage

**Decision**: Use a small cache-backed state store with a fixed key prefix and JSON record containing `state`, `code_verifier`, `org_id`, `created_at`, and `expires_at`.

**Rationale**: Existing `cache.Cache` supports `Get`, `Set`, `Delete`, and TTL. `cmd/tianji/main.go` wires the same cache backend into both `UIHandler` and `Handlers`, so connect and callback can share state without new durable schema. Storing `expires_at` in the payload protects against stale bytes from any backend.

**Alternatives considered**:

- Store state in the UI session cookie: rejected because callback path is outside `/ui` cookie scope and must not require the UI cookie.
- Store state in the database: rejected for this slice because state is short-lived and cache TTL/delete semantics match the lifecycle.
- Encode `org_id` in state/JWT: rejected because Linear requires server-side state and callback must not trust query/client-carried organization data.

## R-003: PKCE and OAuth State

**Decision**: Reuse existing `internal/provider/openai` helpers for PKCE/state generation, authorize URL construction, and code exchange.

**Rationale**: HO-1174 already added `GeneratePKCE`, `ChallengeFromVerifier`, `GenerateState`, `BuildAuthorizeURL`, and `ExchangeCode`. Go OAuth2 docs and Context7 both show the standard pattern: generate a fresh verifier per flow, send an S256 challenge on authorize, and send the verifier during code exchange. `AuthCodeURL` treats state as opaque client state echoed back by the provider.

**Alternatives considered**:

- Use `golang.org/x/oauth2` directly in HO-1175: rejected because Tianji already has OpenAI-specific request parameters and public-client token exchange helper.
- Recompute or omit verifier during callback: rejected because PKCE requires the original verifier stored from connect.

## R-004: Organization Trust Boundary

**Decision**: Callback persistence must use only `org_id` loaded from server-side state and ignore all callback query/body organization fields.

**Rationale**: Linear HO-1175 explicitly requires callback to trust `org_id` only from server-side state. Query parameters on the callback URL are controlled by the browser/provider redirect path and can be tampered with.

**Alternatives considered**:

- Accept `org_id` query as fallback when state lacks org: rejected because that validates the bug this issue exists to prevent.
- Store `credential_id` in callback query: rejected because credential creation should occur only after token exchange succeeds and should be scoped by stored organization.

## R-005: Consume-Once Semantics

**Decision**: Consume/delete loaded state on terminal success and on terminal failures after the state record is found.

**Rationale**: Linear requires callback deletes state after use. A provider denial or token exchange failure after state lookup is terminal for that OAuth attempt and must not leave a reusable state/code_verifier pair. Missing or expired states have no valid record to consume, but expired/malformed records returned by cache should be deleted.

**Alternatives considered**:

- Delete state only after successful credential persistence: rejected because provider/token errors would leave replayable state.
- Delete state before reading fields: rejected because callback still needs the stored verifier and org ID for valid processing.

## R-006: Failure Rendering

**Decision**: Render minimal HTML success/error pages from the callback handler and sanitize all provider-visible text.

**Rationale**: Linear requires readable callback failures. Returning JSON-only errors from a browser redirect would be poor UX, while raw provider errors may include request details. Pages must not include token material, code verifier, authorization code, encrypted credential value, or raw upstream response.

**Alternatives considered**:

- Redirect every failure to `/ui` with query parameters: rejected because it would serialize error details into URLs and would again depend on `/ui` session context.
- Include raw upstream response for debugging: rejected because token endpoints may include sensitive fields.

## R-007: External Verification Notes

- `go doc golang.org/x/oauth2 Config.AuthCodeURL` says state is an opaque client value maintained between request and callback and echoed by the authorization server; it also recommends PKCE/state protection for CSRF.
- `go doc golang.org/x/oauth2 GenerateVerifier`, `S256ChallengeOption`, and `VerifierOption` confirm the verifier/challenge/exchange pattern.
- Context7 `/golang/oauth2` confirms fresh verifier per authorization, S256 challenge on authorization URL, and verifier on token exchange.
- `go doc github.com/go-chi/chi/v5 Router.Use` confirms middleware stacking for route groups, matching existing `h.sessionAuth` usage.
- grep-app public code search found server-side state/code-verifier patterns in Ory Hydra, Dex, Authorizer, and Memoh; CodeQL examples flag constant OAuth state as unsafe.
- Official OpenAI developer docs were searched for OAuth callback/state lifecycle guidance. The previously indexed Apps SDK auth reference URL returned a current docs 404, so no OpenAI-specific state quote is used in this plan.

## R-008: Repo Evidence Notes

- `internal/provider/openai/oauth.go` already builds OpenAI authorize URLs with `state`, `code_challenge`, `code_challenge_method=S256`, `id_token_add_organizations=true`, and `codex_cli_simplified_flow=true`.
- `internal/config/openai_oauth.go` already derives the callback path `/oauth/openai/callback` from `general_settings.public_base_url`.
- `internal/config/config.go` already has `GeneralSettings.PublicBaseURL`.
- `internal/ui/routes.go` has a protected UI group using `h.sessionAuth`.
- `internal/ui/session.go` uses UI session cookies scoped to `/ui`.
- `internal/cache/cache.go` defines the required cache operations.
- `internal/testutil/openaitest` exists from HO-1190 and can provide offline OAuth mocks and no-real-OpenAI guard clients.
