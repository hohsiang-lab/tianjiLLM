# Research: OpenAI OAuth PKCE Primitives and Provider Config

**Feature**: HO-1174 OpenAI OAuth PKCE primitives and provider config  
**Date**: 2026-05-06

## Decisions

### Decision 1: Use authorization-code + PKCE public-client flow, no client secret

**Decision**: Implement OpenAI OAuth request builders as public-client OAuth authorization-code flow with PKCE S256. Token exchange sends `client_id` and `code_verifier`; it never sends `client_secret` or HTTP Basic client auth.

**Rationale**:

- RFC 7636 defines the S256 transform as `BASE64URL-ENCODE(SHA256(ASCII(code_verifier)))`, used to protect public clients from authorization-code interception.
- OpenAI/Codex login source uses `https://auth.openai.com`, PKCE S256, `response_type=code`, a public `client_id`, and token exchange driven by verifier/state rather than a client secret.
- OpenAI Apps SDK auth documentation expects OAuth flows to advertise/support PKCE S256.

**Alternatives considered**:

- Confidential client with `client_secret`: rejected because HO-1174 scope explicitly requires no `client_secret`, and subscription OAuth must be usable as a public-client flow.
- Import tokens from OpenAI CLI/browser session: rejected by epic safety boundary; no cookie/session/account scraping.

**Sources**:

- RFC 7636 §4.2/§4.3 — PKCE S256 challenge method.
- OpenAI Codex source `codex-rs/login/src/server.rs` — authorize URL fields and `https://auth.openai.com` defaults.
- OpenAI Codex source `codex-rs/login/src/pkce.rs` — verifier/challenge generation.
- OpenAI Apps SDK auth docs — PKCE S256 support is expected for OAuth flows.

### Decision 2: Prefer `golang.org/x/oauth2` PKCE helpers where they fit

**Decision**: Implementation should prefer `golang.org/x/oauth2` PKCE helpers (`GenerateVerifier`, `S256ChallengeOption`, `VerifierOption`) for standards-compliant verifier/challenge/exchange options, with thin Tianji wrappers for OpenAI-specific config and validation.

**Rationale**:

- Repo already has `golang.org/x/oauth2 v0.35.0` in `go.mod`, so no new dependency is needed.
- Context7 docs for `golang.org/x/oauth2` confirm `GenerateVerifier` follows RFC 7636 and should be passed to `AuthCodeURL` via `S256ChallengeOption` and to `Exchange` via `VerifierOption`.
- Thin wrappers make config/error handling Tianji-specific without reimplementing well-known PKCE mechanics.

**Alternatives considered**:

- Fully custom PKCE code: acceptable as fallback but less preferable because a maintained dependency already exists and is documented.
- Reusing existing `internal/auth/SSOHandler`: rejected for OpenAI subscription OAuth because it is an app login/SSO abstraction that currently sends `client_secret` and is not a public-client provider primitive.

**Sources**:

- Repo `go.mod` contains `golang.org/x/oauth2 v0.35.0`.
- Context7 `/websites/pkg_go_dev_golang_org_x_oauth2` docs for `GenerateVerifier`, `S256ChallengeOption`, `VerifierOption`, and `Config.Exchange`.
- Repo `internal/auth/sso.go` currently includes confidential-client `client_secret` behavior and is not safe to reuse as-is.

### Decision 3: Add OpenAI OAuth provider config under `general_settings`, preserving existing OpenAI API-key path

**Decision**: Add typed OpenAI OAuth provider config to `GeneralSettings` (or a nested struct under it) rather than overloading model-level `api_key`/`api_base`. Existing OpenAI provider request transforms remain untouched in this slice.

**Rationale**:

- OAuth authorization/token endpoint configuration is deployment-level, not per-model routing configuration.
- Existing `TianjiParams.APIKey` and `APIBase` are used by model deployments and must remain compatible with current OpenAI API-key behavior.
- Later issues can consume the config for callback/credential persistence without changing current model routing in HO-1174.

**Alternatives considered**:

- Model-level `tianji_params.openai_oauth_*`: rejected because it would duplicate provider metadata across model entries and blur credential enrollment versus model use.
- Environment variables only: rejected because CI needs deterministic YAML/config tests and deployments need explicit override visibility.

**Sources**:

- Repo `internal/config/config.go` — `GeneralSettings` already holds global auth/SSO fields; `TianjiParams` holds model-level API key/base settings.
- Repo `internal/provider/openai/openai.go` — current OpenAI provider uses `defaultBaseURL` + bearer API key and should remain unchanged in this planning slice.

### Decision 4: Redirect URI derives from configured public base URL and is HTTPS-gated in production

**Decision**: Derive redirect URI as `<public_base_url>/oauth/openai/callback`, trimming trailing slashes. Validate that production redirect URIs use HTTPS, while `http://localhost` and loopback hosts are allowed only in dev/test mode.

**Rationale**:

- OAuth redirect URI must match provider allowlist exactly; deriving from a single public base URL avoids ad hoc callback strings.
- HTTPS is required for real production OAuth safety. Localhost exception is necessary for `httptest`/local development and matches common OAuth public-client development patterns.
- This issue is backend-only primitives, so it should expose validation errors early before callback/server registration code exists.

**Alternatives considered**:

- Hardcode callback URI: rejected because Tianji deployments have different public URLs.
- Allow arbitrary HTTP: rejected as unsafe for production authorization codes.

**Sources**:

- OpenAI Apps SDK auth docs — production OAuth redirect URLs are HTTPS-based and must be allowlisted.
- OpenAI Codex source — localhost callback is used for CLI development flow; Tianji replaces this with server public base URL.

## Scope Alignment Notes

Norman confirmed on 2026-05-06 to align this planning PR to the Linear issue rather than treating the extra questions as blockers.

- Public base URL: implement the Linear issue concept "Tianji public base URL"; prefer `public_base_url` unless repo convention during implementation indicates a better existing key.
- Public client ID: use the Linear issue decision — default public `client_id` follows OpenClaw/PI Codex-style public client, with config override.
- Originator/scopes: treat as provider metadata details for the Codex-style authorize URL, not separate scope blockers. Defaults may follow observed OpenAI/Codex examples and remain overrideable.
- Implementation remains gated by Linear **In Progress**; Todo/Waiting state is planning only.
