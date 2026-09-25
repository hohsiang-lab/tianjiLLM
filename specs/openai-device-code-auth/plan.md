# Implementation Plan: OpenAI ChatGPT Device-Code Credential Login

**Branch**: `feat/openai-device-auth`
**Spec**: [spec.md](spec.md)

## Design summary

Add one provider-specific device client, one cache-backed device-flow store, and one lifecycle service on the existing proxy handler. Keep the UI handler responsible for session/CSRF/scope validation and templ rendering. Reuse the existing OpenAI code exchange, credential encryption, persistence, and audit helpers.

```text
POST /ui/openai/device/start
  -> UI session + permission + CSRF + scope validation
  -> provider.RequestDeviceCode
  -> DeviceStore.Create(shared cache, session binding, org scope)
  -> render verification URL/user code/expiry panel

POST /ui/openai/device/status
  -> UI session + permission + CSRF + flow binding validation
  -> shared DeviceStore.Get
  -> cache lock + re-read
  -> if eligible: provider.PollDeviceCode once
  -> pending/slow-down: update next_poll_at and render status
  -> success: mark exchanging, ExchangeCode(device callback URI, provider verifier)
  -> SaveOpenAISubscriptionCredential + audit, then mark success

POST /ui/openai/device/cancel
  -> UI session + permission + CSRF + flow binding validation
  -> delete pending state and render idle/cancelled panel
```

## Files and steps

### 1. RED provider contract tests

Modify `internal/provider/openai/oauth_test.go` or add `device_auth_test.go` with an `httptest.Server` that asserts:

- user-code POST path, JSON body, content type, and client ID;
- `user_code`/`usercode` aliases and string/numeric/missing interval parsing;
- verification and device callback URLs derived from the configured issuer;
- one-poll JSON body and success response parsing;
- 403/404 and RFC error-code mapping;
- unknown/5xx errors are categorized without exposing response bodies;
- final exchange receives authorization code, exact device callback redirect URI, and server-issued verifier.

Expected initial failure: missing provider API/types.

### 2. RED state-store and concurrency tests

Add `internal/openaioauth/device_store_test.go` covering:

- fresh opaque flow ID and required record fields;
- TTL and expired records;
- shared-cache reads rather than stale local reads;
- session-binding constant-time ownership checks;
- terminal state cannot be reused;
- cache lock acquisition/release and concurrent poll serialization;
- record JSON contains no token fields.

Expected initial failure: missing device store.

### 3. Implement provider client

Add `internal/provider/openai/device_auth.go`:

- resolve issuer-relative endpoints from `config.ResolveOpenAIOAuthConfig`;
- use bounded JSON request/response bodies and `http.NewRequestWithContext`;
- normalize interval to a positive default of five seconds;
- return typed/sentinel errors for pending, slow-down, expired, denied, transient, and unavailable cases;
- redact response bodies in errors;
- expose only the small request/poll result needed by the lifecycle service.

Also redact non-success bodies in the existing `ExchangeCode` path, which currently formats its raw body into an error.

### 4. Implement state store

Add `internal/openaioauth/device_store.go`:

- define `DeviceAuthRecord`, status constants, cache key/lock key helpers;
- create a flow with 15-minute TTL and `next_poll_at`;
- use `cache.SharedCache.GetShared` when available;
- require `cache.LockCache` for poll/cancel serialization and return a safe unavailable error otherwise;
- write only non-secret flow metadata and safe error codes;
- fail stale `exchanging` leases instead of replaying consumed authorization codes.

### 5. Implement lifecycle service

Add `internal/proxy/handler/openai_device.go`:

- `StartOpenAIDeviceAuth` requests the device code and stores it;
- `PollOpenAIDeviceAuth` re-reads under lock, respects `next_poll_at`, maps provider outcomes, and performs one exchange/save on success;
- preserve organization ID from the flow record only;
- reuse `openAISubscriptionTokenBundle`, `openAISubscriptionCredentialInfo`, `SaveOpenAISubscriptionCredential`, and `auditOpenAISubscriptionLifecycle`;
- return a UI-safe status DTO with only URL, user code, times, status, credential ID, and enumerated error code;
- `CancelOpenAIDeviceAuth` deletes only an owned pending flow.

### 6. RED UI/handler tests, then routes and CSRF

Modify `internal/ui/handler_openai_test.go` with tests for:

- start requires session, permission, and CSRF;
- start stores only bound flow state and renders no device ID;
- status rejects missing/wrong CSRF and wrong session;
- status uses stored organization and completes through the existing save path;
- cancel removes only the current session's flow;
- unavailable device endpoint renders callback fallback.

Add a small session CSRF helper in `internal/ui/session.go` (random token in SCS session, constant-time validation). Register:

```text
POST /ui/openai/device/start
POST /ui/openai/device/status
POST /ui/openai/device/cancel
```

Keep all under `PermissionCredentialsManage`. Add the CSRF token to the credentials page and the existing paste form so the newly protected callback-url POST is not a CSRF bypass.

### 7. UI contract tests and template

Modify `internal/ui/pages/credentials.templ` and generated template output through the repository's `templ` target:

- add `CSRFToken` to page data;
- add device start form beside the existing browser callback/paste controls;
- add escaped, accessible device status panel with `aria-live`, verification link, code, expiry, warning, cancel, and HTMX `POST` polling;
- stop polling on success/error/expired/denied;
- keep callback launch `target="_blank" rel="noopener"` and paste modal behavior.

Extend `internal/ui/pages/credentials_openai_test.go` for the rendered form, hidden CSRF field, no `device_auth_id`, no token-like sentinel, and terminal polling markup.

No visual mockup was supplied; review against existing card/button/dialog spacing and keyboard/focus conventions.

### 8. Mock harness and integration/E2E coverage

Extend `internal/testutil/openaitest/oauth_server.go` and tests with device user-code/token endpoints and request recording. Add handler-level integration coverage that uses the mock DB/store already used by credential lifecycle tests. If the existing browser E2E harness can run with its PostgreSQL/browser prerequisites, add or extend `test/e2e/openai_connect_flow_test.go`; otherwise keep a deterministic handler E2E contract and record the environment blocker rather than faking a browser result.

### 9. Verification and review loop

Run, in order:

```bash
gofmt -w <changed-go-files>
templ generate
go test ./internal/provider/openai/... ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/handler/... ./internal/testutil/openaitest/... -count=1
go test -race ./internal/provider/openai/... ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/handler/... -count=1
go vet ./internal/provider/openai/... ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/handler/...
make lint
make build
git diff --check origin/main...HEAD
```

Then run the available E2E command with its real prerequisites, inspect GitHub Actions, and perform five review passes:

1. correctness/security code review;
2. ponytail/subtractive review;
3. requirement/contract review;
4. E2E coverage assessment;
5. UI/mockup alignment review for the credentials page.

Every finding is fixed on the exact branch head, tests rerun, and the review is repeated until all five dimensions report clean (maximum five iterations; if a sixth review is needed, open a follow-up issue instead of silently widening scope). Only then create/update the PR. After PR creation, use exact-head CI/review reconciliation; fix CI failures, rerun review, and do not merge until CI is green and all required review gates are clean.

## Known baseline

`go test ./...` was run on the clean worktree before implementation. Non-integration packages passed; `test/integration` failed because local PostgreSQL was not listening on `localhost:5433`. This is baseline evidence, not a claim that the final full suite passes.
