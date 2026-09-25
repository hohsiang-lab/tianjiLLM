# Research: Codex subscription route E2E and credential lookup recovery

## Repo Evidence

- `internal/proxy/handler/chatgpt_codex_backend.go` has separate non-streaming and streaming Codex handlers; streaming sends `text/event-stream`, transforms `response.output_text.delta`/`response.completed`, and writes `[DONE]`.
- `internal/provider/chatgptcodex/transport.go` builds requests for `/backend-api/codex/responses`, sends bearer auth, `ChatGPT-Account-Id`, `originator`, `OpenAI-Beta`, and `Accept: text/event-stream` for streaming.
- `internal/provider/chatgptcodex/payload.go` already includes `store:false`, optional `stream`, normalized model, `instructions`, typed `input`, and strict unsupported-field validation.
- `internal/proxy/handler/openai_subscription_refresh.go` maps `pgx.ErrNoRows` to `credential_missing` and non-`ErrNoRows` `GetCredential` errors to `credential_lookup_failed`.
- `internal/db/queries/credential.sql` defines `GetCredential` as `SELECT * FROM "CredentialTable" WHERE credential_id = $1;`; generated code expands this into all current columns and scans into `db.CredentialTable`.
- `internal/db/schema/004_credentials.up.sql` defines `credential_id TEXT PRIMARY KEY`, so UUID-shaped credential IDs are normal text values.
- `internal/proxy/handler/chatgpt_codex_wildcard_test.go` already covers mocked `openai/*` subscription wildcard routing to Codex backend, but does not use a generated-query/DB-backed credential lookup.
- `internal/proxy/handler/openai_subscription_refresh_test.go` covers fresh/stale/refresh behavior at resolver level, but not the full route with DB-managed wildcard config and Codex streaming.

## Live Evidence From Linear

- Production `ProxyModelTable.model_name = 'openai/*'` is configured with `openai_subscription_transport = "chatgpt_codex_backend"` and one credential ID.
- Production `CredentialTable` contains that credential ID with `credential_info.status = "active"`.
- Live `POST /v1/chat/completions` with `model=openai/gpt-5.5` and `stream:true` can reach Codex SSE after HO-1306, but a later retest fails with `OpenAI subscription credential resolution failed ... credential lookup failed`.
- Because the row exists, the remaining defect is not the wildcard model route or streaming adapter alone; the route is reaching credential lookup/read integration.

## External Documentation And Prior Art

- Context7 `/jackc/pgx` documents that `QueryRow` defers query errors until `Scan` and returns `pgx.ErrNoRows` when no row matches. This validates separate test cases for no-row vs scan/query/runtime errors.
- grep-app against `jackc/pgx` found the same `QueryRow` implementation comment in `conn.go`: query errors are deferred until `Scan`, and no rows produce `ErrNoRows`.
- grep-app against `openai/codex` found `ResponseInputItem::Message` with role/content/phase and `ContentItem::InputText` in `codex-rs/protocol/src/models.rs`, `codex-rs/core/src/goals.rs`, and `codex-rs/core/src/session/mod.rs`. This supports existing Tianji typed Codex payload assertions.
- The private `chatgpt.com/backend-api/codex/responses` route has no public official contract in prior HO-1292 research, so tests must keep using local mocks and repo-owned fixtures.

## Decision: DB-Backed Route Test Is Required

Use a test boundary that includes generated DB lookup, not only `mockStore.GetCredential`.

Reasons:

- The live symptom is specifically `GetCredential` returning a non-`ErrNoRows` lookup failure while the row exists.
- Existing mock-store tests can prove routing and redaction, but cannot catch schema/query/scan mismatches.
- pgx `QueryRow` can defer query/scan errors, so the failure may be invisible unless the test actually scans rows through generated code.

## Decision: Handler Route Remains Primary E2E Boundary

Use handler-level route tests with local `httptest.Server` Codex backend.

Reasons:

- The behavior crosses model wildcard resolution, subscription credential lookup/refresh, Codex transport, payload, and streaming response normalization.
- Provider unit tests alone cannot prove the `openai/*` runtime route selects `chatgpt_codex_backend`.
- Browser/UI E2E is not relevant; this is a backend route issue.

## Decision: Preserve API-Key Wildcard Guard

Keep an API-key-backed `openai/*` regression in the same verification set.

Reasons:

- The issue explicitly requires API-key-backed `openai/*` behavior to remain unchanged.
- The riskiest over-fix would force all `openai/*` routes through subscription Codex lookup.
- A wrong-route sentinel catches bearer leakage to Platform chat completions.

## Open Questions

None for Todo scope. Implementation should answer the exact RED failure after Linear moves to `In Progress`.
