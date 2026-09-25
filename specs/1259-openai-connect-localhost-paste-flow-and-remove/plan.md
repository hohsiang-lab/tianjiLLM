# Implementation Plan: OpenAI Connect localhost paste flow and remove `public_base_url`

**Branch**: `HO-1259-openai-connect-localhost-paste-flow-and-remove`
**Linear**: HO-1259
**Spec**: [spec.md](spec.md)
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`.

## Summary

Replace Tianji-hosted OpenAI OAuth redirect derivation with an explicit localhost redirect contract. Connect creates state + PKCE and stores the exact authorize `redirect_uri`; callback exchange uses that stored URI. Add a hosted paste flow so users can copy the full localhost callback URL back into Tianji when no localhost listener exists. Remove OpenAI OAuth runtime dependency on `general_settings.public_base_url`, including deployment/sample `proxy.yml` or proxy YAML config artifacts.

## Technical Context

- Language/runtime: Go module in TianjiLLM.
- Router/UI: `go-chi/chi`, existing `/ui` protected route group, existing public `/oauth/openai/callback`.
- Existing OAuth primitives: `internal/provider/openai` has PKCE/state generation, `BuildAuthorizeURL`, `ExchangeCode`.
- Existing state: `internal/openaioauth.StateStore` cache-backed, consume-once, TTL-protected.
- Existing credential persistence: `internal/proxy/handler.OpenAIOAuthCallback` saves OpenAI subscription credential after token exchange.
- Existing tests: Go unit/handler tests and `test/e2e/openai_connect_flow_test.go` with `internal/testutil/openaitest`.
- Existing proxy YAML fixtures: `test/fixtures/proxy_config_full.yaml` and `test/fixtures/mcp/proxy_config_mcp.yaml`; no tracked root `proxy.yml` exists in the planning worktree.
- External constraint: OpenAI public Codex client flow is localhost-oriented; hosted Tianji callback is not assumed allowlisted.

## Constitution Check

| Principle | Status | Notes |
|---|---:|---|
| Spec-first | PASS | This Todo phase modifies only `specs/1259-*`. |
| Repo reality first | PASS | Plan is based on current `internal/config`, `internal/ui`, `internal/proxy/handler`, `internal/openaioauth`, `test/e2e`, and sibling specs. |
| Failing-tests-first | PASS | Tasks start with RED tests for redirect URI, state record redirect persistence, pasted URL parsing, replay, and leakage. |
| Security | PASS | Pasted URL and OAuth secrets are treated as sensitive inputs. |
| No new framework | PASS | Reuse existing Go handlers, templ/Tailwind surfaces, cache, and OpenAI mock harness. |

## Current Data Flow

```text
GET /ui/openai/connect?org_id=...
  -> config.DeriveOpenAIRedirectURI(general_settings.public_base_url)
  -> StateStore.Create(orgID, ttl)
  -> BuildAuthorizeURL(..., redirectURI, challenge, state)

GET /oauth/openai/callback?state=...&code=...
  -> StateStore.Consume(state)
  -> config.DeriveOpenAIRedirectURI(general_settings.public_base_url)
  -> ExchangeCode(..., redirectURI, record.CodeVerifier)
  -> SaveOpenAISubscriptionCredential(...)
```

Problem: authorize and exchange both recompute hosted redirect URI from `public_base_url`, which is invalid for the public Codex OAuth client and makes `public_base_url` required for OpenAI OAuth.

## Target Data Flow

```text
GET /ui/openai/connect?org_id=...
  -> ResolveOpenAIOAuthRedirectURI(config.OpenAIOAuth)
       default: http://localhost:1455/auth/callback
  -> StateStore.Create(orgID, redirectURI, ttl)
  -> BuildAuthorizeURL(..., record.RedirectURI, challenge, record.State)
  -> redirect to OpenAI authorize

Tianji original tab remains on /ui/credentials
  -> user can open Paste callback URL modal from the credentials page
  -> Connect OpenAI opens /ui/openai/connect?... in a new tab/window
  -> new tab redirects to OpenAI authorize
  -> browser lands on http://localhost:1455/auth/callback?code=...&state=...
  -> user copies full URL
  -> user returns to the credentials-page paste modal

POST /ui/openai/callback-url
  -> parse pasted URL server-side
  -> validate host/path against stored/configured redirect contract
  -> run shared callback completion service with parsed code/state/error
  -> ExchangeCode(..., record.RedirectURI, record.CodeVerifier)
  -> SaveOpenAISubscriptionCredential(...)
  -> safe success/failure page

GET /oauth/openai/callback?state=...&code=...
  -> same shared callback completion service
```

## Architecture Decisions

### Decision 1 - Explicit OpenAI OAuth redirect setting

Add redirect ownership to `OpenAIOAuthConfig`, likely:

```go
RedirectURI string `yaml:"redirect_uri,omitempty"`
```

Default resolution returns `http://localhost:1455/auth/callback`. Endpoint overrides for tests remain in `OpenAIOAuthConfig`; `public_base_url` is not involved.

**Rejected**: keep `public_base_url` as fallback. That preserves the broken mental model and leaves production config implying hosted callback support that this issue explicitly removes.

### Decision 2 - Store exact redirect URI in state

Extend `openaioauth.StateRecord`:

```go
RedirectURI string `json:"redirect_uri"`
```

Change `StateStore.Create` to accept redirect URI, validate non-empty absolute URL, and include it in the cache record. `validFor` must require it.

**Rationale**: OAuth token exchange must use the same redirect URI as authorize. Storing it avoids recomputing from mutable config and supports future mode changes safely.

### Decision 3 - Shared callback completion service

Extract callback completion logic out of the raw HTTP GET handler into a helper/service that accepts parsed OAuth callback values:

```go
type OpenAIOAuthCallbackInput struct {
    State string
    Code string
    ProviderError string
    ProviderErrorDescription string
    Source string // direct_callback | pasted_url
}
```

Both direct callback and paste POST call the same helper. This keeps state consume, provider-error handling, token exchange, credential save, audit, and safe page rendering aligned.

### Decision 4 - Paste form belongs in a Credentials modal

Add the paste entry point to `/ui/credentials` using the existing Tianji dialog component pattern found in sibling UI pages. The protected endpoint can still be routed separately:

- `/ui/credentials` renders the `Paste callback URL` button and modal shell.
- `GET /ui/openai/callback-url` may render the modal body/partial or fallback form.
- `POST /ui/openai/callback-url` accepts `callback_url`.

The modal trigger should sit near `Connect OpenAI`. The `Connect OpenAI` launch control should use `target="_blank"` or equivalent with `rel="noopener"` so the original Tianji tab remains on `/ui/credentials` while the new tab completes OpenAI authorization and lands on the localhost callback URL.

**Security boundary**: paste endpoint requires UI session, but final callback completion still authenticates via server-side state. It must not trust form-provided org ID.

## UI Scope

Minimal owner-facing UI:

- Credentials page `Connect OpenAI` action remains primary and opens the OpenAI authorize flow in a new tab/window while the original Tianji tab stays on `/ui/credentials`.
- Add a `Paste callback URL` button on `/ui/credentials` that opens a modal with:
  - one textarea/input for full localhost callback URL
  - submit button
  - safe error/success state
  - no display of raw code/state after submit
- The helper text may mention copying the full browser address from `localhost:1455`, but must not include implementation/debug prose in permanent UI beyond what a user needs to complete the flow.

## Test Plan

### RED tests first

| Test | File | Expected pre-fix failure | Covers |
|---|---|---|---|
| `TestResolveOpenAIOAuthRedirectURI_DefaultsToLocalhostCallback` | `internal/config/openai_oauth_test.go` | helper/config missing | FR-003, FR-004, FR-006 |
| `TestLoadWithOpenAIOAuthConfig_DoesNotRequirePublicBaseURL` | `internal/config/loader_test.go` | loader/test still expects public base | FR-001, FR-015 |
| `TestStateStore_CreateStoresRedirectURI` | `internal/openaioauth/state_store_test.go` | field/API missing | FR-007 |
| `TestHandleOpenAIConnect_UsesConfiguredRedirectURIAndStoresIt` | `internal/ui/handler_openai_test.go` | derives from public base | FR-004, FR-007 |
| `TestCredentialsOpenAIConnectLaunch_OpensAuthorizeInNewTab` | UI render test | launch contract missing | FR-005, SC-007 |
| `TestCredentialsOpenAIPasteCallbackModal_RendersProtectedSubmitForm` | UI render test | credentials modal missing | FR-010, SC-004 |
| `TestOpenAIOAuthCallback_UsesStoredRedirectURIForExchange` | `internal/proxy/handler/openai_oauth_test.go` | recomputes from public base | FR-008 |
| `TestParseOpenAIPastedCallbackURL_ValidatesHostPathAndFields` | new parser test | parser missing | FR-011, FR-012 |
| `TestOpenAIOAuthPastedCallback_SuccessConsumesStateAndPersistsCredential` | handler/e2e | endpoint missing | FR-009, FR-010, FR-014 |
| `TestOpenAIOAuthPastedCallback_RejectsUnexpectedURLWithoutLeakingSecrets` | handler/e2e | endpoint missing | FR-012, FR-016 |
| `TestOpenAIConnectFlow_LocalhostPasteSuccessWithMockOAuth` | `test/e2e/openai_connect_flow_test.go` | E2E still uses hosted callback | SC-001..007 |

### Verification Commands

```bash
go test ./internal/config/... ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/handler/... -run 'Test.*OpenAI|Test.*OAuth' -count=1
go test ./internal/provider/openai/... -run 'TestBuildAuthorizeURL|TestExchangeCode' -count=1
go test ./test/e2e -tags e2e -run 'TestOpenAIConnect|TestOpenAICallback' -count=1
rg "PublicBaseURL|public_base_url|DeriveOpenAIRedirectURI|ValidateOpenAIRedirectURI" internal test cmd config || true
find . -maxdepth 4 \( -name 'proxy.yml' -o -name 'proxy.yaml' -o -name 'proxy*.yml' -o -name 'proxy*.yaml' \) -print
git diff --check origin/main...HEAD
```

## Plan Review Evidence

- Repo evidence: `internal/ui/handler_openai.go` currently calls `config.DeriveOpenAIRedirectURI(h.Config.GeneralSettings.PublicBaseURL)` before `BuildAuthorizeURL`.
- Repo evidence: `internal/proxy/handler/openai_oauth.go` currently calls the same derivation before `ExchangeCode`.
- Repo evidence: `internal/openaioauth/state_store.go` currently stores state/code verifier/org/timestamps but no redirect URI.
- Repo evidence: `test/e2e/openai_connect_flow_test.go` currently asserts authorize `redirect_uri` equals `testServer.URL + "/oauth/openai/callback"` and sets `cfg.GeneralSettings.PublicBaseURL`.
- Repo evidence: `find . -maxdepth 4 ... proxy*.yaml` currently finds only `test/fixtures/proxy_config_full.yaml` and `test/fixtures/mcp/proxy_config_mcp.yaml`; implementation must also remove `public_base_url` from root/deployment `proxy.yml` if that artifact is present in the implementation branch or deploy config package.
- Official OpenAI Codex auth docs fetched 2026-05-08: browser login returns through localhost callback; blocked-localhost/headless cases prefer device-code or localhost forwarding.
- Official `openai/codex` source fetched 2026-05-08: CLI login uses `http://localhost:{actual_port}/auth/callback`; comments mention keeping callback ports in sync with the Codex CLI Hydra redirect URI allow-list.
- Apps SDK auth docs fetched 2026-05-08 describe ChatGPT connecting to our MCP server with ChatGPT redirect URI allowlisting, not Tianji connecting to OpenAI Codex OAuth; it is not evidence that Tianji hosted callback is valid for the public Codex OAuth client.

## Risk Register

| Risk | Mitigation |
|---|---|
| Pasted URL leaks through logs or HTML | Treat as secret input; never echo raw URL; add sentinel leakage tests. |
| State consumed before URL host/path validation | Parse and validate URL shape first; consume state only after expected callback shape and state are present. |
| Direct callback behavior diverges from paste behavior | Use one shared callback completion helper for both paths. |
| Future hosted allowlisted client becomes possible | Explicit redirect config can support it later, but default remains localhost and no `public_base_url` fallback. |
| UI form accidentally trusts org ID | Paste flow derives org only from stored state record. |

## Scope Confirmation

Owner input required: 0.

Default scope: remove `public_base_url` from OpenAI OAuth runtime behavior and proxy YAML config artifacts, default OpenAI authorize redirect to localhost callback, persist exact redirect URI in state, add protected Tianji paste URL flow, and cover the bug path with RED-first tests.
