# Implementation Plan: Subscription wildcard routing regression verification

**Branch**: `HO-1292-add-regression-verification-for-subscription`
**Spec**: `specs/1292-add-regression-verification-for-subscription-wildcard-routing/spec.md`
**Linear**: HO-1292
**Phase**: Todo planning only; production/test implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Language/runtime: Go module `github.com/praxisllmlab/tianjiLLM`.
- Routing: `internal/proxy/handler.ChatCompletion`, `resolveProviderRoute`, `findModelConfig`, `internal/wildcard`.
- Codex transport: `internal/proxy/handler/chatgpt_codex_backend.go` and `internal/provider/chatgptcodex`.
- Existing tests: handler package tests, provider package tests, `test/contract/wildcard_test.go`, and OpenAI subscription resolver tests.
- Config field: `tianji_params.openai_subscription_transport`, with values `direct_openai_http` and `chatgpt_codex_backend`.

## Governance

- Todo state allows SpecKit artifacts, spec-only branch, and draft PR only.
- No production code or test code may be edited until Linear moves to `In Progress`.
- This issue is a regression-verification ticket. If RED tests pass without implementation changes, the implementation PR can remain test/docs-only after the `In Progress` gate.
- The draft PR is for scope review, not code review.

## Repo Reality

- `Handlers.findModelConfig` already supports exact match first, then wildcard match sorted by specificity, and resolves `tianji_params.model` with captured `*` segments.
- `resolveProviderRoute` sets `ChatGPTCodexBackend` from `isChatGPTCodexBackendTransport(d.Config.TianjiParams)`.
- `ChatCompletion` dispatches non-streaming Codex routes to `handleChatGPTCodexBackendCompletion`.
- Existing `TestChatGPTCodexBackendTransport_UsesBackendURLHeadersAndBody` covers exact `chatgpt/gpt-5.5` but not wildcard `openai/*`.
- Existing `test/contract/wildcard_test.go` covers wildcard match/no-match but not subscription credentials or Codex transport selection.
- Existing `internal/provider/openai/openai_test.go` proves API-key OpenAI provider sends `/v1/chat/completions` and keeps `messages`, not Codex `input`.
- Existing OpenAI subscription tests cover resolver behavior, candidate selection, and direct/Codex token resolution, but not full wildcard HTTP routing.

## Plan Review Evidence

- Official OpenAI API docs state Platform API authentication uses bearer API keys and that request/rate-limit diagnostics include request IDs plus `x-ratelimit-*` headers: <https://platform.openai.com/docs/api-reference/authentication?api-mode=responses>.
- Official OpenAI API error docs classify 401 authentication failures and 429 rate-limit/quota failures as distinct actionable errors: <https://platform.openai.com/docs/guides/error-codes/api-errors>.
- Official Responses API docs describe `POST /v1/responses`, typed `input`, and `instructions` as the public Responses surface: <https://platform.openai.com/docs/api-reference/responses/retrieve>.
- Official docs search found no public `chatgpt.com/backend-api/codex/responses` contract, so tests must keep this private backend path isolated behind local mocks and repo-owned fixtures.
- `grep-app-cli` against `openai/codex` found `ResponseInputItem::Message` with role/content/phase in `openai/codex` (`codex-rs/protocol/src/models.rs`) and Codex core usage of `ContentItem::InputText`, supporting exact typed payload assertions. A second grep-app query for `/backend-api/codex/responses` failed due upstream grep.app HTML content-type response; use the successful `ResponseInputItem::Message` evidence plus repo-owned HO-1288 transport fixtures.

## Architecture

### Test placement

Preferred placement: `internal/proxy/handler/chatgpt_codex_wildcard_test.go`.

Reason:

- The behavior crosses wildcard config lookup, subscription credential resolution, Codex transport selection, backend request building, and response/error writing.
- Handler package tests can reuse `newOpenAISubscriptionRoutingHarness`, fake encrypted credentials, and existing local `httptest.Server` mocks.
- Contract tests under `test/contract` are useful for API surface but do not currently expose the subscription credential harness as directly.

### Subscription wildcard harness

Create a narrow helper that:

1. Seeds an active `openai_subscription` credential with fake access token and account ID.
2. Configures model:
   - `ModelName: "openai/*"`
   - `TianjiParams.Model: "openai/*"`
   - `OpenAISubscriptionCredentialIDs: []string{"cred-a"}`
   - `OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend`
3. Sets `GeneralSettings.OpenAIOAuth.CodexBackendBaseURL` to a local Codex backend mock.
4. Optionally provides an APIBase/platform mock only as a wrong-route sentinel.

The test should call `h.ChatCompletion` directly or use `proxy.NewServer` when auth middleware behavior is part of the regression. Direct handler invocation is enough for routing/transport verification if request context is not auth-dependent.

### Wrong-route sentinel

Use two layers:

- A Platform mock that fails the test if `/v1/chat/completions` receives the subscription token.
- A guarded transport or explicit mock request recorder that reports wrong host/path without printing bearer values.

Do not rely only on response status. The test must assert concrete request path and zero wrong-route calls.

### Diagnostics matrix

Table-driven tests should send the same wildcard request while the Codex backend returns:

| Status | Body code | Expected |
| --- | --- | --- |
| 401 | `invalid_token` | HTTP 401, `authentication_error`, safe message |
| 403 | `missing_scope` | HTTP 403, `permission_error`, safe message |
| 429 | `insufficient_quota` | HTTP 429, quota/rate-limit code, safe message |

If the implementation records quota headers only on provider retry path and not final Codex error, the test plan should either assert store update when headers are parsed or document that this slice verifies caller-facing diagnostics only.

### API-key wildcard regression

Add isolated test using:

- `ModelName: "openai/*"`
- `TianjiParams.Model: "openai/*"`
- `APIKey: "sk-api-key"`
- `APIBase: <local platform mock base URL>`
- no subscription credential IDs
- no `openai_subscription_transport`

Assert the mock receives `/chat/completions` or `/v1/chat/completions` depending on `provider/openai.NewWithBaseURL` behavior, and assert no Codex backend call occurs.

## Failing Tests

| Test | File | Initial failure to prove | Covers |
| --- | --- | --- | --- |
| `TestChatGPTCodexBackendWildcard_RoutesSubscriptionOpenAIWildcardToCodexBackend` | `internal/proxy/handler/chatgpt_codex_wildcard_test.go` | Wrong transport sends to Platform or misses wildcard transport flag | FR-001..FR-003 |
| `TestChatGPTCodexBackendWildcard_NeverSendsSubscriptionBearerToPlatformChatCompletions` | same | ChatGPT OAuth token can reach `/v1/chat/completions` | FR-004, FR-011 |
| `TestChatGPTCodexBackendWildcard_PreservesAuthScopeQuotaDiagnostics` | same | 401/403/429 rewritten or hidden at wildcard layer | FR-005..FR-008 |
| `TestOpenAIAPIKeyWildcard_StillUsesChatCompletionsPath` | same or `test/contract/wildcard_test.go` | API-key wildcard accidentally selects Codex backend | FR-009 |
| `TestOpenAIWildcardDeploymentVerificationNotesAreSecretSafe` | docs/checklist review or lightweight text test if repo has doc-test convention | Verification notes leak token-printing commands | FR-011, FR-013 |

## Implementation Phases

### Phase 1 - RED tests

Add the four runtime regression tests above and confirm any failing test reflects the intended missing verification/implementation behavior, not test fixture setup.

### Phase 2 - Minimal implementation fix only if RED exposes a real bug

If tests fail because runtime still routes subscription wildcard through direct OpenAI HTTP, fix only the routing/transport selection boundary. Do not refactor credential lifecycle, UI selector, or provider packages beyond what the tests require.

### Phase 3 - Deployment verification note

Add a secret-safe quickstart/deployment section describing how to verify OpenClaw `openai/*` after rollout:

- test request through Tianji/OpenClaw using non-secret model alias and redacted logs
- assert route/path marker indicates Codex backend
- classify 401/403/429 vs wrong Platform path
- forbid printing Authorization headers or token-bearing env vars

### Phase 4 - Verification

Run targeted Go tests for handler/provider/config/wildcard plus diff sanity. CI remains the full merge gate.

## Verification Commands

```bash
go test ./internal/proxy/handler -run 'TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard|TestChatGPTCodexBackendTransport' -count=1
go test ./internal/provider/chatgptcodex ./internal/provider/openai ./internal/config -count=1
go test ./test/contract -run 'TestWildcard|TestResponsesCreate' -count=1
git diff --check origin/main...HEAD
```

If implementation remains tests/docs-only, no local E2E is required unless the added regression is placed under `test/e2e`.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Test passes by checking provider unit behavior instead of route behavior | Put primary test at handler route boundary with wildcard config. |
| Token leak in failure output | Use fake secret sentinels and negative assertions on response/log text. |
| API-key wildcard is accidentally forced into subscription validation | Keep isolated API-key wildcard regression with no credential IDs and no transport. |
| Private Codex backend path changes | Mock path through config override and keep deployment note focused on route class and safe diagnostics. |
| Real OpenAI network accidentally called | Use local mocks and guarded transport where the code path allows it. |

## Todo Gate Status

Spec/plan/tasks/analyze are ready for docs-only draft PR review. Production/test implementation remains blocked until Linear `In Progress`.
