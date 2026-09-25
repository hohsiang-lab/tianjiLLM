# Tasks: OpenAI OAuth Mock Harness and Endpoint Override Coverage

**Input**: SpecKit artifacts under `specs/1190-openai-oauth-mock-harness-endpoint-override/`
**Prerequisites**: HO-1174 OpenAI OAuth PKCE/config primitives on `origin/main`

## Phase 1: Tests First

- [X] T001 Add `internal/testutil/openaitest/oauth_server_test.go` with failing tests for authorization-code exchange request recording and public-client PKCE assertions.
- [X] T002 Add refresh-token rotation fixture test for the OAuth mock server.
- [X] T003 Add `internal/testutil/openaitest/upstream_server_test.go` with failing tests for `/v1/models` and representative `/v1/chat/completions` fixtures.
- [X] T004 Add guard transport tests proving `auth.openai.com` and `api.openai.com` are rejected and local mock hosts are allowed.
- [X] T005 Add provider/config integration tests proving OpenAI OAuth authorize/token overrides and OpenAI upstream base URL overrides use mock URLs only.

## Phase 2: OAuth Mock Harness

- [X] T006 Implement `internal/testutil/openaitest.OAuthServer` using `httptest.NewServer` with endpoint accessors and `t.Cleanup`.
- [X] T007 Implement token exchange and refresh fixture matching with deterministic JSON responses and deterministic 400 for missing fixtures.
- [X] T008 Implement recorded request snapshots for method/path/host/header/body/form with mutex protection.
- [X] T009 Implement public-client assertion helper that fails on `client_secret` form fields or Basic authorization.

## Phase 3: Upstream Mock Harness

- [X] T010 Implement `internal/testutil/openaitest.UpstreamServer` with `/v1/models` fixture support.
- [X] T011 Implement representative OpenAI-compatible endpoint fixtures for chat completions and one existing provider endpoint shape such as embeddings/images.
- [X] T012 Ensure unexpected upstream paths return deterministic failure responses and are recorded for diagnostics.

## Phase 4: No-Real-OpenAI Guard and Integration

- [X] T013 Implement `NewNoRealOpenAIGuard` / `NewGuardedClient` with default forbidden hosts `auth.openai.com` and `api.openai.com`.
- [X] T014 Update/add OpenAI OAuth endpoint override tests to use the shared OAuth harness and guard client.
- [X] T015 Update/add OpenAI provider upstream override tests to use the shared upstream harness and guard client.
- [X] T016 Confirm existing API-key OpenAI provider behavior remains unchanged when no test override is supplied.

## Phase 5: Verification

- [X] T017 Run `go test ./internal/testutil/openaitest/... -v`.
- [X] T018 Run targeted OpenAI provider/OAuth tests covering exchange, endpoint overrides, upstream mock, and guard behavior.
- [X] T019 Run `git diff --check` and inspect changed files to confirm only HO-1190 scope changed.
- [X] T020 Record verification evidence in PR body before moving to review/CI gates.
