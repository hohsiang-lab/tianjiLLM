# Research: HO-1259 OpenAI Connect localhost paste flow

## Repo Reality

### Current runtime references

- `internal/ui/handler_openai.go` derives redirect URI from `h.Config.GeneralSettings.PublicBaseURL`, then calls `openai.BuildAuthorizeURL`.
- `internal/proxy/handler/openai_oauth.go` consumes state, then derives redirect URI from `h.Config.GeneralSettings.PublicBaseURL` again before `openai.ExchangeCode`.
- `internal/openaioauth/state_store.go` stores `State`, `CodeVerifier`, `OrgID`, `CreatedAt`, and `ExpiresAt`; it does not store redirect URI.
- `test/e2e/openai_connect_flow_test.go` currently expects hosted test callback and sets `cfg.GeneralSettings.PublicBaseURL = testServer.URL`.
- `internal/config/loader_test.go` currently includes `public_base_url` in OpenAI OAuth config fixture.

## Official OpenAI Evidence

### Codex auth docs

Fetched from `https://developers.openai.com/codex/auth.md` on 2026-05-08.

Relevant facts:

- Codex browser login returns an access token to the CLI or IDE extension after browser sign-in.
- Remote/headless or blocked-localhost cases are explicitly called out.
- Preferred workaround for blocked localhost callback is device-code auth; fallback is copying local auth cache or forwarding localhost callback over SSH.
- The SSH forwarding fallback names default `localhost:1455`.

Impact on HO-1259: official Codex auth docs confirm localhost callback is a first-class Codex login assumption; they do not document a hosted third-party redirect allow-list path for Tianji's current public client.

### Official `openai/codex` source

Fetched from `https://raw.githubusercontent.com/openai/codex/main/codex-rs/login/src/server.rs` on 2026-05-08.

Relevant facts:

- The login server is explicitly documented as a local OAuth callback server.
- It builds redirect URI as `http://localhost:{actual_port}/auth/callback`.
- Default port is `1455`; fallback port is `1457`.
- A source comment says callback ports must stay in sync with the Codex CLI Hydra redirect URI allow-list.
- Token exchange sends the same redirect URI used in authorize.
- Sensitive URL query fields such as `code`, `state`, tokens, and `code_verifier` are redacted in logging helpers.

Impact on HO-1259: Tianji's hosted `https://tianji.../oauth/openai/callback` is incompatible with the Codex-style public client assumption captured in the current official source; Tianji should persist exact redirect URI and avoid logging pasted URL secrets.

### Apps SDK auth docs

Fetched from `https://developers.openai.com/apps-sdk/build/auth.md` on 2026-05-08.

Relevant facts:

- Apps SDK auth describes ChatGPT as OAuth client connecting to our MCP/resource server.
- Redirect URI is `https://chatgpt.com/connector/oauth/{callback_id}` and must be allowlisted on our authorization server.
- This is the opposite direction from Tianji connecting to OpenAI/Codex OAuth.

Impact on HO-1259: Apps SDK docs do not validate a Tianji hosted redirect URI for OpenAI's public Codex OAuth client. They should not be used as evidence to keep `public_base_url`.

## Decisions

### D1 - Treat hosted OpenAI redirect whitelist as unavailable

**Decision**: Do not wait for or design around a hosted Tianji allow-list for `app_EMoamEEZ73f0CkXaXp7hrann` in this issue.

**Rationale**: Linear comments record Norman's product decision; official docs/source support localhost callback as the current workable path.

### D2 - Use explicit redirect URI under OpenAI OAuth config

**Decision**: Add explicit redirect ownership to `openai_oauth`, defaulting to `http://localhost:1455/auth/callback`.

**Rationale**: Redirect URI belongs to the OAuth client/authorize contract, not Tianji server public base URL.

### D3 - Persist redirect URI in state

**Decision**: Store exact authorize redirect URI in `StateRecord` and use it for token exchange.

**Rationale**: OAuth token exchange must match authorize redirect URI. Persisting it avoids recomputing from mutable config.

### D4 - Add hosted paste URL form instead of raw code entry

**Decision**: User pastes the full localhost callback URL; Tianji parses code/state/error server-side.

**Rationale**: It is less error-prone than asking users to copy a raw `code` or rewrite callback URLs manually, and it allows strict server-side validation/redaction.

### D5 - Shared callback completion helper

**Decision**: Direct callback and pasted callback must call the same helper after parsing inputs.

**Rationale**: Keeps one-time state consume, token exchange, credential persistence, audit, and redaction behavior consistent.

## Open Questions

None requiring owner input for Todo planning.
