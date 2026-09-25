# Implementation Plan: OpenAI OAuth Mock Harness and Endpoint Override Coverage

**Branch**: `HO-1190-openai-oauth-mock-harness-and-endpoint-override` | **Date**: 2026-05-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1190-openai-oauth-mock-harness-endpoint-override/spec.md`

## Summary

Add test-only OpenAI mock infrastructure: a reusable `httptest.NewServer` OAuth harness, a reusable OpenAI upstream harness, endpoint override tests, and a no-real-OpenAI guard. The implementation must keep CI offline and must not introduce production behavior, WireMock, or real OpenAI credentials.

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Primary Dependencies**: Go stdlib `net/http`, `net/http/httptest`, `net/url`, `sync`; existing `testify`
**Storage**: None
**Testing**: Go unit/contract tests; future UI E2E can import helper URLs from Go setup
**Target Platform**: CI Linux and local developer machines
**Constraints**: No real OpenAI network; no WireMock; no production code beyond endpoint override plumbing if a test exposes a missing injection seam
**Scale/Scope**: Test helper package plus targeted tests for OAuth token/refresh, `/v1/models`, representative upstream endpoints, endpoint override, and guard behavior

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ⚠️ EXCEPTION | TianjiLLM is Go; this ticket is Go test infrastructure for existing Go OAuth/provider code. |
| II. Feature Parity | ✅ PASS | Test infra protects current/future OpenAI OAuth feature parity without changing product behavior. |
| III. Research Before Build | ✅ PASS | Repo patterns, Linear parent scope, official `httptest` docs, and OpenAI models docs checked. |
| IV. Failing-Tests-First | ✅ PASS | Tasks start with tests that fail before reusable harness exists. |
| V. Go Best Practices | ✅ PASS | In-process `httptest.Server`, per-test instances, explicit cleanup, no global mutable endpoint state. |
| VI. No Stale Knowledge | ✅ PASS | Official docs used where Context7 could not resolve Go stdlib. |
| VII. sqlc-First DB Access | ✅ N/A | No DB changes. |

## Project Structure

### Documentation

```text
specs/1190-openai-oauth-mock-harness-endpoint-override/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── tasks.md
├── analyze.md
├── contracts/
│   └── openai-mock-harness.md
└── checklists/
    └── requirements.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/testutil/openaitest/
├── oauth_server.go          # NEW: OAuth authorize/token/refresh httptest harness
├── upstream_server.go       # NEW: /v1/models + representative OpenAI endpoint httptest harness
├── guard_transport.go       # NEW: no-real-OpenAI guard client/transport
└── recorder.go              # NEW: recorded request snapshots/assert helpers

internal/provider/openai/
├── oauth_test.go            # MODIFY/ADD: use reusable OAuth harness for exchange coverage
├── openai_test.go           # MODIFY/ADD: upstream override/guard regression
└── models_test.go           # NEW if /v1/models helper belongs near provider tests

test/e2e/
└── openai_oauth_mock_test.go # OPTIONAL/FUTURE: prove E2E setup can consume harness endpoints when UI flow exists
```

## Data Flow

```text
Test setup
  openaitest.NewOAuthServer(t, fixtures)
    → exposes AuthorizeURL(), TokenURL(), Client(), Requests()
    → config.OpenAIOAuthConfig{AuthorizeURL, TokenURL}
    → production OAuth request builder/exchange code
    → recorded request assertions

Upstream setup
  openaitest.NewUpstreamServer(t, fixtures)
    → exposes BaseURL(), Client(), Requests()
    → openai.NewWithBaseURL(mock.BaseURL()+"/v1" or repo-compatible base)
    → provider request/HTTP call
    → deterministic OpenAI-compatible JSON

Guard setup
  openaitest.NewGuardedClient(allowedHosts...)
    → wraps transport
    → rejects auth.openai.com/api.openai.com
    → tests pass only when endpoint overrides point at mocks
```

## Failing Tests

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestOAuthMockServer_ExchangeCodeRecordsPublicClientPKCE` | `internal/testutil/openaitest/oauth_server_test.go` | missing helper package/server | FR-001..004 |
| `TestOAuthMockServer_RefreshTokenRotationFixture` | `internal/testutil/openaitest/oauth_server_test.go` | missing refresh fixture support | FR-004 |
| `TestUpstreamMockServer_ModelsEndpoint` | `internal/testutil/openaitest/upstream_server_test.go` | missing upstream helper | FR-005..006 |
| `TestUpstreamMockServer_ChatCompletionsEndpoint` | `internal/testutil/openaitest/upstream_server_test.go` | missing representative endpoint fixture | FR-005 |
| `TestOpenAIOAuthEndpointOverrides_UseMockURLsOnly` | `internal/provider/openai/oauth_test.go` | no shared harness/guard integration | FR-007, FR-009 |
| `TestOpenAIProviderBaseURLOverride_UsesMockUpstreamOnly` | `internal/provider/openai/openai_test.go` | no shared upstream guard test | FR-008..009 |
| `TestNoRealOpenAIGuard_RejectsDefaultHosts` | `internal/testutil/openaitest/guard_transport_test.go` | missing guard transport | FR-009 |

## Plan Review

- **Official docs**: `pkg.go.dev/net/http/httptest` confirms `NewServer`, `Server.URL`, `Server.Client()`, and cleanup via `Close()` are standard for local HTTP tests.
- **OpenAI docs**: models docs confirm `/v1/models` is a first-class OpenAI API surface; include it in upstream mock fixtures.
- **Repo evidence**: Existing tests already use `httptest.NewServer`; OpenAI provider has `NewWithBaseURL`; OAuth exchange accepts injected `*http.Client` and endpoint config override.
- **Context7**: CLI lookup did not resolve Go stdlib; official Go docs used instead.
- **grep-app**: public code search failed with grep.app transport content-type error; no plan change.

## Phase 1: Tests First

Add failing tests for the helper package and endpoint override/guard behavior before implementing helper internals.

## Phase 2: Helper Implementation

Implement `internal/testutil/openaitest` with per-test server instances, fixture registration, request recording, and guard transport.

## Phase 3: Integration Coverage

Refactor/add OpenAI OAuth/provider tests to use the reusable harness and prove covered paths do not target real OpenAI hosts.

## Phase 4: Verification

Run targeted tests:

```bash
go test ./internal/testutil/openaitest/... -v
go test ./internal/provider/openai/... -run 'Test.*OpenAI.*Mock|Test.*EndpointOverrides|Test.*NoRealOpenAI|TestExchangeCode' -v
```

If UI E2E integration is added in this ticket after In Progress, also run the relevant e2e package with its normal DB/browser prerequisites.

## Risk Register

| Risk | Mitigation |
|------|------------|
| Guard only covers injected clients | Treat any hard-coded default client path as a test failure and add explicit injection seam before continuing. |
| Helpers become production dependency | Keep package under `internal/testutil/openaitest`; production packages must not import it. |
| Mock response shape drifts from OpenAI-compatible JSON | Keep fixtures minimal but representative; include `/v1/models` and existing provider endpoint shapes. |
| Parallel tests race recorded requests | Per-server request recorder with mutex and no package-level mutable fixtures. |
