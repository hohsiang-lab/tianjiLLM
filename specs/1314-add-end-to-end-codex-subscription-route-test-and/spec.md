# Feature Specification: Codex subscription route E2E and credential lookup recovery

**Branch**: `HO-1314-codex-subscription-route-credential-lookup`
**Linear**: HO-1314
**Created**: 2026-05-11
**Status**: Todo planning only; no production or test code in this phase

## Summary

HO-1306 proved the `chatgpt_codex_backend` streaming handler can forward Codex SSE chunks, but the live `openai/*` subscription path still fails intermittently at credential lookup even though the configured `CredentialTable` row exists and is active. HO-1314 owns one continuous regression and fix for the chained runtime path:

`openai/* wildcard route -> subscription credential lookup/refresh -> Codex payload -> Codex SSE normalization`

The implementation must start with a failing route-level test after Linear moves to `In Progress`, then repair the credential lookup/read integration without weakening API-key-backed `openai/*`.

## Scope

In scope:

- Add one route-level regression that exercises subscription-backed `openai/*` through wildcard resolution, DB-backed credential lookup, optional refresh boundary, Codex payload build, and streaming Codex SSE normalization.
- Add deterministic route coverage for the current subscription-backed non-streaming behavior, so the old `store:false` / `unsupported_streaming` blockers do not reappear as an ambiguous runtime failure.
- Reproduce the current live failure class where `DB.GetCredential(...)` returns a non-`pgx.ErrNoRows` lookup failure even though the credential row exists.
- Fix credential lookup/read integration so existing active subscription credentials can be loaded deterministically on the Codex backend path.
- Preserve distinct diagnostics for missing credential, lookup/read failure, malformed credential, disabled credential, refresh failure, and upstream Codex errors.
- Keep all bearer/access/refresh token material redacted in responses, logs, tests, and live verification notes.
- Keep API-key-backed `openai/*` routing on normal OpenAI-compatible `/v1/chat/completions`.

Out of scope:

- Changing OpenAI OAuth connect UI or credential management UI.
- Replacing the Codex payload mapper, except for defects directly exposed by the full-route RED test.
- Reworking sticky/failover policy beyond preserving candidate order and failure reasons.
- Calling real OpenAI or ChatGPT services from automated tests.
- Printing live credential token values during diagnostics.

## User Stories and Tests

### User Story 1 - Subscription wildcard completes streaming Codex route (P1)

As a Tianji operator, I want `openai/gpt-5.5` on a subscription-backed `openai/*` wildcard to complete as an SSE chat stream, so OpenClaw can use the ChatGPT Codex subscription route end to end.

**Independent Test**: Seed a DB-backed active `openai_subscription` credential, configure `ProxyModelTable` or equivalent runtime model data for `openai/*` with `openai_subscription_transport = "chatgpt_codex_backend"`, send `/v1/chat/completions` with `stream:true`, and assert the mocked Codex backend receives `/backend-api/codex/responses` and the client receives `data: ...` chunks plus `[DONE]`.

**Acceptance Scenarios**:

1. Given an active subscription credential row and configured `openai/*` wildcard, when `openai/gpt-5.5` is requested with `stream:true`, then the route resolves the wildcard model and selected credential.
2. Given the Codex backend mock emits `response.output_text.delta` and `response.completed`, then Tianji returns OpenAI-compatible chat completion SSE chunks and terminates with `[DONE]`.
3. Given a Platform mock exists as a wrong-route sentinel, then it receives zero calls and never receives the subscription bearer token.
4. Given the same subscription wildcard is requested without `stream:true`, then the route returns the current deterministic Codex contract result instead of falling back to Platform OpenAI or hiding the failure as credential lookup.

### User Story 2 - Existing active credential row does not become generic lookup failure (P1)

As an operator debugging live rollout, I need an existing active credential row to load or fail with a precise cause, so `credential lookup failed` does not hide schema/scan/pool/runtime integration defects.

**Independent Test**: Use the real generated `db.Queries.GetCredential` path against a test DB or pgx-backed fixture that contains the active credential shape, and assert resolution returns the decrypted bundle rather than `credential_lookup_failed`.

**Acceptance Scenarios**:

1. Given a row exists in `CredentialTable` with all columns selected by generated `GetCredential`, then `loadOpenAISubscriptionCredential` returns the active bundle.
2. Given `GetCredential` returns `pgx.ErrNoRows`, then the caller sees `credential_missing`, not `credential_lookup_failed`.
3. Given `GetCredential` returns a scan/query/runtime error, then the caller sees `credential_lookup_failed` with a redacted but actionable internal cause available to diagnostics.

### User Story 3 - Lookup/refresh failure handling remains safe and actionable (P1)

As a Tianji operator, I need lookup and refresh failures to keep enough reason-code fidelity to decide whether to reconnect credentials, fix DB/runtime schema, or inspect upstream auth.

**Independent Test**: Table-driven tests cover missing, wrong type, disabled, malformed/decrypt, stale token refresh success, refresh failure, auth-failed-after-refresh, and lookup scan/query failure through the route-level Codex backend path.

**Acceptance Scenarios**:

1. Given a credential exists but is disabled, then the route reports `credential_disabled` and tries another configured candidate if available.
2. Given a stale token refresh succeeds, then the retry uses the refreshed bearer and account ID.
3. Given all candidates fail before upstream, then the response includes safe reason codes and no fallback API key is used.

### User Story 4 - API-key wildcard remains unchanged (P1)

As an existing Tianji API-key user, I need normal `openai/*` aliases to keep using the Platform/OpenAI-compatible chat route, so subscription Codex fixes do not hijack ordinary API-key traffic.

**Independent Test**: Configure `openai/*` with `api_key` and `api_base` only, send a streaming and non-streaming chat request, and assert the Platform mock receives `/v1/chat/completions` while the Codex mock receives zero calls.

**Acceptance Scenarios**:

1. Given no `openai_subscription_credential_ids` are configured, then no subscription credential lookup occurs.
2. Given no `openai_subscription_transport` is configured, then `chatgpt_codex_backend` is not selected.
3. Given API-key wildcard request succeeds, then request shape still contains `messages`, not Codex `input`.

## Functional Requirements

- **FR-001**: Tests MUST cover the full subscription-backed `openai/*` route with `stream:true`, not only provider or payload unit boundaries.
- **FR-002**: Tests MUST seed/use a DB-backed credential lookup path close enough to production to catch generated query scan/query failures.
- **FR-003**: System MUST classify `pgx.ErrNoRows` as `credential_missing`.
- **FR-004**: System MUST classify non-`ErrNoRows` DB lookup/read failures as `credential_lookup_failed` while preserving redacted internal diagnostics for logs/tests.
- **FR-005**: System MUST NOT flatten wrong-type, disabled, malformed, refresh, auth-after-refresh, and lookup errors into the same caller-facing cause when candidates are evaluated.
- **FR-006**: System MUST send selected subscription credentials to `chatgpt.com/backend-api/codex/responses` only when `openai_subscription_transport = "chatgpt_codex_backend"` is configured.
- **FR-007**: System MUST preserve Codex payload invariants already fixed by prior issues, including normalized model, `store:false`, compatible `instructions`/`input`, and `stream:true` on streaming requests.
- **FR-008**: System MUST transform Codex SSE delta/completed events into OpenAI-compatible chat completion chunks and `[DONE]`.
- **FR-009**: System MUST NOT send subscription bearer/access/refresh tokens to Platform `/v1/chat/completions`.
- **FR-010**: System MUST NOT silently fallback to API key when explicit subscription credentials are configured and fail.
- **FR-011**: Existing API-key-backed `openai/*` behavior MUST keep using OpenAI-compatible chat completions.
- **FR-012**: Tests and diagnostics MUST NOT print raw token material, decrypted credential bundles, Authorization headers, refresh tokens, or ID tokens.
- **FR-013**: Tests MUST deterministically validate subscription-backed non-streaming behavior according to the current Codex contract.

## Key Entities

- **Subscription Codex Route**: The runtime path combining model wildcard config, subscription credential IDs, `chatgpt_codex_backend`, and Codex streaming handler.
- **Credential Lookup Boundary**: `DB.GetCredential(ctx, credentialID)` plus generated `db.Queries.GetCredential`, row scan, `CredentialTable` schema, and error classification.
- **Resolved Subscription Credential**: In-memory candidate containing `CredentialID`, bearer access token, optional `AccountID`, and attribution metadata.
- **Codex Streaming Backend Mock**: Local `httptest.Server` emitting private Codex Responses SSE events for deterministic route tests.
- **API-key Wildcard Guard**: Regression setup proving ordinary API-key `openai/*` routes stay off the Codex subscription path.

## Success Criteria

- **SC-001**: A RED route-level test fails on the current credential lookup/runtime integration bug before implementation.
- **SC-002**: The same test passes after the lookup/read integration fix and verifies Codex SSE chunks plus `[DONE]`.
- **SC-003**: Lookup error tests distinguish missing row from non-row lookup/read failures and keep token output redacted.
- **SC-004**: API-key wildcard regression tests continue passing.
- **SC-005**: Targeted Go tests for handler/provider/DB boundaries pass locally with no real external network calls.
- **SC-006**: Subscription non-streaming requests are covered by an issue-owned test and cannot be mistaken for the credential lookup failure class.

## Dependencies

- HO-1288 ChatGPT Codex backend transport.
- HO-1289 chat request to Codex Responses payload mapping.
- HO-1291 `openai_subscription_transport` config/UI persistence.
- HO-1292 subscription wildcard Codex routing regression.
- HO-1300 `store:false` Codex payload invariant.
- HO-1306 Codex backend streaming response support.
