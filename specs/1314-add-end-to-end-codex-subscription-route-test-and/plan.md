# Implementation Plan: Codex subscription route E2E and credential lookup recovery

**Branch**: `HO-1314-codex-subscription-route-credential-lookup`
**Spec**: `specs/1314-add-end-to-end-codex-subscription-route-test-and/spec.md`
**Linear**: HO-1314
**Phase**: Todo planning only; production/test implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Language/runtime: Go module `github.com/praxisllmlab/tianjiLLM`.
- Route entry: `internal/proxy/handler.ChatCompletion`.
- Codex backend route: `internal/proxy/handler/chatgpt_codex_backend.go`.
- Credential resolution: `internal/proxy/handler/openai_subscription_resolution.go`, `openai_subscription_routing.go`, and `openai_subscription_refresh.go`.
- Generated DB lookup: `internal/db/queries/credential.sql` and `internal/db/credential.sql.go`.
- Credential schema: `internal/db/schema/004_credentials.up.sql`.
- Codex transport/payload/SSE mapping: `internal/provider/chatgptcodex`.
- Existing tests: `internal/proxy/handler/chatgpt_codex_backend_test.go`, `chatgpt_codex_wildcard_test.go`, `openai_subscription_*_test.go`, and `test/e2e/models_openai_subscription_test.go`.

## Governance

- Linear HO-1314 is currently `Todo`; only SpecKit artifacts, spec-only branch, draft PR, and scope confirmation are allowed.
- Do not edit `internal/`, `test/`, generated SQL, schema, or runtime config until Linear moves to `In Progress`.
- Implementation must begin with RED route-level tests. The first implementation commit after `In Progress` must not be a fix-only commit.
- Draft PR is for scope review, not code review.

## Recovery Checkpoint

- Linear issue: `Todo`, project `TianjiLLM`, label `Bug`, comments empty.
- Discord thread contains start/resume messages only.
- Existing HO-1314 worktree is clean at `302a98f`, tracking `origin/main`.
- No HO-1314 PR exists yet.
- Pipeline ledger shows this orchestration `016c5654-ffe2-434a-bb60-3be3365949a0` executing.
- Required dirty/unpushed Git checkpoint was a no-op because no dirty files or unpushed commits were present before these artifacts.

## Repo Reality

- `loadOpenAISubscriptionCredential` calls `h.DB.GetCredential(ctx, credentialID)` and maps `pgx.ErrNoRows` to `credential_missing`; every other DB error becomes `credential_lookup_failed`.
- Generated `GetCredential` selects every `CredentialTable` column and scans into `db.CredentialTable`.
- `CredentialTable` stores `credential_id` as `TEXT PRIMARY KEY`; live failure uses UUID-like text, so ID type mismatch is not the expected missing-row explanation.
- Existing `chatgpt_codex_wildcard_test.go` proves mocked subscription wildcard routing reaches `/backend-api/codex/responses`, but uses handler mock stores rather than a generated-query/DB-backed credential lookup.
- Existing `chatgpt_codex_backend_test.go` proves Codex streaming transforms backend events into chat chunks.
- Existing refresh tests prove refresh success/failure at resolver level, but not the full `openai/*` wildcard + DB lookup + Codex streaming route.
- API-key wildcard coverage exists and must remain a no-regression guard.

## Plan Review Evidence

- Context7 `/jackc/pgx` documents that `QueryRow` defers query errors until `Scan`, and no matching row returns `pgx.ErrNoRows`; this supports testing scan/query failures separately from missing rows.
- grep-app against `jackc/pgx` found the same `QueryRow` implementation comment in `conn.go`, confirming that lookup failures can surface at `Scan`, not only at call construction.
- grep-app against `openai/codex` found `ResponseInputItem::Message` and `ContentItem::InputText` usages in `codex-rs/protocol/src/models.rs`, `codex-rs/core/src/goals.rs`, and `codex-rs/core/src/session/mod.rs`, supporting typed message/content assertions already used by Tianji's Codex payload tests.
- Prior Tianji specs HO-1292, HO-1300, and HO-1306 establish that private `chatgpt.com/backend-api/codex/responses` behavior must be validated through repo-owned local mocks, not real network calls.

## Architecture

### Primary RED route test

Create a handler-level integration test that uses a real or pgx-compatible DB fixture for credentials and proxy model config:

1. Apply minimal schema needed for `CredentialTable` and `ProxyModelTable`, or use the repo's E2E DB helper if it can run fast and deterministically.
2. Insert an encrypted active `openai_subscription` credential row using the same `master_key` as the handler config.
3. Insert DB-managed `openai/*` runtime model config through `ProxyModelTable`, then force/trigger runtime model refresh so the route proves the same DB config source used in production:
   - `model = "openai/*"`
   - `openai_subscription_credential_ids = ["<fixture-id>"]`
   - `openai_subscription_transport = "chatgpt_codex_backend"`
4. Point `CodexBackendBaseURL` at an `httptest.Server` that emits deterministic SSE:
   - `data: {"type":"response.output_text.delta",...}`
   - `data: {"type":"response.completed",...}`
   - `data: [DONE]`
5. Send `/v1/chat/completions` with `model:"openai/gpt-5.5"` and `stream:true`.
6. Assert:
   - mocked Codex backend receives `/backend-api/codex/responses`
   - request body includes normalized model, `store:false`, `stream:true`, and typed `input`
   - client receives OpenAI-compatible chunk(s) and `[DONE]`
   - response/log/test output does not contain fake token sentinels

### Credential lookup defect isolation

Add targeted tests around the generated query boundary:

- Existing row loads and decrypts successfully through `loadOpenAISubscriptionCredential`.
- Missing row yields `credential_missing`.
- Non-row DB/scan/runtime error yields `credential_lookup_failed` and preserves redacted internal cause in a test-observable diagnostic field or log hook.

If the RED route test exposes a schema/query mismatch, fix the generated query/schema/runtime scan boundary rather than masking the error in handler code.

If the RED route test only exposes overly generic error reporting, add a precise diagnostic path without exposing secrets, then confirm live-equivalent behavior no longer flips between success and generic lookup failure.

### Candidate/failover behavior

Keep `resolveOpenAISubscriptionAttemptOrder` as the candidate ordering boundary. Tests should prove:

- unusable first credential does not prevent a later active credential from being tried
- all-candidate failure preserves per-candidate reason codes
- explicit subscription credentials never fallback to configured API key

### Refresh and non-streaming route behavior

Add one stale-token route case using the same DB-backed fixture and mocked token endpoint. It must prove the full route uses the refreshed bearer/account ID before calling Codex, reusing the existing refresh helper rather than inventing a parallel path.

Add one subscription-backed non-streaming route case. It should assert the current deterministic contract for a request without `stream:true` and verify that this path does not:

- call Platform OpenAI
- leak subscription token material
- collapse into the live `credential_lookup_failed` class

### API-key no-regression

Keep or extend the current API-key wildcard test:

- `ModelName: "openai/*"`
- `TianjiParams.Model: "openai/*"`
- `APIKey` and optional `APIBase`
- no subscription credential IDs
- no subscription transport

Assert Platform mock receives `/v1/chat/completions`, Codex mock receives zero calls, request body keeps `messages`, and no credential lookup hook fires.

## Failing Tests

| Test | File | Initial failure to prove | Covers |
| --- | --- | --- | --- |
| `TestCodexSubscriptionRouteE2E_DBBackedCredentialStreams` | `internal/proxy/handler/chatgpt_codex_subscription_e2e_test.go` or `test/e2e/codex_subscription_route_test.go` | Full route fails with `credential_lookup_failed` or misses streaming Codex path | FR-001, FR-002, FR-006..FR-008 |
| `TestLoadOpenAISubscriptionCredential_DBBackedExistingRow` | `internal/proxy/handler/openai_subscription_lookup_db_test.go` | Existing active row cannot be loaded through generated query | FR-002 |
| `TestLoadOpenAISubscriptionCredential_DistinguishesMissingFromLookupFailure` | same | Missing row and scan/query errors collapse incorrectly | FR-003, FR-004 |
| `TestCodexSubscriptionRoute_AllCandidateFailuresPreserveReasonCodes` | handler route or resolver test | All failures become generic lookup failure | FR-005, FR-010 |
| `TestOpenAIAPIKeyWildcard_StillUsesChatCompletionsPath` | existing handler test | API-key wildcard is hijacked by Codex subscription route | FR-011 |
| `TestCodexSubscriptionRouteE2E_StaleCredentialRefreshesBeforeStreaming` | route E2E test | Full route bypasses refresh or uses stale bearer/account ID | FR-002, FR-005, FR-006 |
| `TestCodexSubscriptionRoute_NonStreamingContractIsDeterministic` | handler route test | Non-streaming request falls back to Platform or hides as credential lookup | FR-010, FR-013 |

## Implementation Phases

### Phase 1 - RED tests

Add only tests. Confirm at least one issue-owned test fails against current code for the live-equivalent credential lookup route, and confirm failures are not caused by bad fixtures.

### Phase 2 - Minimal lookup/runtime fix

Patch the smallest boundary proven by RED:

- generated query/schema/scan compatibility if `CredentialTable` reads fail despite existing rows
- resolver error classification if failure reason is hidden
- handler/runtime model source integration if route selects stale/malformed credential IDs

Do not refactor OAuth lifecycle, UI, or Codex payload/streaming beyond what the RED test requires.

### Phase 3 - Secret-safe diagnostics

Ensure returned errors, logs, and tests include credential ID and reason code but not token material. Use fake sentinels and negative assertions.

### Phase 4 - Verification

Run targeted Go tests, generated-code checks if SQL changes, `gofmt`, `git diff --check`, and API-key no-regression tests.

## Verification Commands

```bash
go test ./internal/proxy/handler -run 'TestCodexSubscriptionRoute|TestLoadOpenAISubscriptionCredential|TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard' -count=1
go test ./internal/provider/chatgptcodex -run 'Test.*Payload|Test.*Stream' -count=1
go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/provider/openai -count=1
git diff --check origin/main...HEAD
```

If SQL queries or generated DB code change:

```bash
sqlc generate
go test ./internal/db ./internal/proxy/handler -count=1
git diff --check origin/main...HEAD
```

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Route test passes with mock store and misses generated-query bug | Use real/generated DB query path for primary credential lookup coverage. |
| Fix hides DB errors behind generic message | Preserve reason code plus redacted internal cause in diagnostics. |
| API-key wildcard is accidentally routed to Codex | Keep wrong-route sentinel and API-key wildcard regression. |
| Test leaks fake token values in failure output | Use fake sentinel values and negative assertions on response/log bodies. |
| Real ChatGPT/OpenAI call sneaks into tests | Use `httptest.Server` and guarded clients only. |
| Over-fix changes OAuth lifecycle | Scope fix to lookup/read integration unless RED proves lifecycle involvement. |

## Todo Gate Status

Spec/plan/tasks/analyze are ready for docs-only draft PR review. Production/test implementation remains blocked until Linear `In Progress`.
