# Research: OpenAI OAuth Mock Harness and Endpoint Override Coverage

## Repo Reconnaissance

- `internal/provider/openai/oauth.go` already exposes `ExchangeCode(ctx, client, cfg, ...)`, which accepts an injected `*http.Client` and reads `config.OpenAIOAuthConfig.TokenURL`.
- `internal/config/openai_oauth.go` already defines default OpenAI OAuth issuer/authorize/token URLs and override fields.
- Existing OpenAI provider code exposes `NewWithBaseURL(baseURL string)` and `GetRequestURL(modelName)` returns `<baseURL>/chat/completions`.
- Existing tests already use `httptest.NewServer` in `internal/provider/openai/oauth_test.go`, `test/contract/*`, and `test/e2e/setup_test.go`, but each test currently hand-rolls mocks.
- `test/e2e` is a Go package inside the repo, so it can import `internal/testutil/openaitest` under Go `internal` visibility rules.

## External Checks

- Official Go docs: `net/http/httptest` provides `NewServer(handler http.Handler) *Server`; `Server` exposes `URL`, `Client()`, and `Close()`. This matches the required local-loopback mock server pattern.
- OpenAI docs search/fetch confirms `/v1/models` is an OpenAI API surface, so the upstream mock should include model-list fixtures.
- grep-app public GitHub search for `httptest.NewServer(http.HandlerFunc` failed due grep.app transport returning HTML instead of JSON (`Unexpected content type: text/html; charset=utf-8`); repo-local patterns and official Go docs are sufficient for the v1 plan.
- Context7 CLI lookup for generic Go/httptest did not return the Go standard library package; official `pkg.go.dev` was used as authoritative source instead.

## Decisions

### Decision 1: Put reusable helpers in `internal/testutil/openaitest`

**Rationale**: Backend tests and Go E2E tests can both import this path. Keeping it under `internal/testutil` prevents accidental production API exposure while making helpers reusable across packages.

**Alternatives considered**:
- `test/openaitest`: importable but less aligned with repo's internal package organization.
- Per-package helpers: repeats request recording/fixtures and does not satisfy reusable harness requirement.

### Decision 2: One package, two server types

**Rationale**: Keep OAuth and upstream concerns explicit while sharing request-recording helpers:

- `OAuthServer` for authorize/token/refresh fixtures.
- `UpstreamServer` for `/v1/models` and representative `/v1/*` OpenAI-compatible endpoints.

### Decision 3: Guard through injected HTTP clients/transports first

**Rationale**: Current `ExchangeCode` accepts an injected client, and provider tests can send transformed requests with a custom client. This catches real OpenAI attempts without global network monkey-patching.

**Implementation note**: If a future code path constructs its own default client internally, tests should expose that as a gap and add dependency injection before enabling the guard path.

### Decision 4: Keep v1 mock strictly in-process

**Rationale**: Linear scope explicitly says no WireMock in v1. `httptest.NewServer` is simpler, faster in CI, and already common in this repo.
