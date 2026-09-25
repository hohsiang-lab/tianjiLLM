# Research: HO-1288 ChatGPT Codex backend transport

## Repo Reality

### Existing OpenAI provider path

- `internal/provider/openai/openai.go` uses `defaultBaseURL = "https://api.openai.com/v1"` and `GetRequestURL()` returns `baseURL + "/chat/completions"`.
- `SetupHeaders()` sets `Authorization: Bearer <apiKey>` and `Content-Type: application/json`.
- This provider must remain the normal API-key OpenAI path.

### Existing subscription credential resolution

- `internal/proxy/handler/openai_subscription_resolution.go` defines `openAISubscriptionTransportDirectHTTP` and `openAISubscriptionTransportCodexAppServer`.
- Direct HTTP resolution returns `BearerToken = bundle.AccessToken`.
- Codex app-server resolution returns `codexapp.NewChatGPTAuthTokensLogin(...)`.
- There is no third transport for ChatGPT backend HTTP `/codex/responses` today.

### Existing routing and failure behavior

- `internal/proxy/handler/openai_subscription_routing.go` centralizes configured credential ordering, disabled/malformed/missing failures, rate-limit gating, sticky strategy, lowest-utilization strategy, and round-robin fallback.
- HO-1288 should reuse this path so a new transport does not bypass current credential governance.

### Existing config conflict

- `internal/config/openai_oauth.go` default `Originator` is `tianjillm`.
- HO-1288 requires backend request `originator: codex_cli_rs` unless config overrides it.
- Therefore backend request originator should be separate from OAuth authorize `originator`, or implementation must deliberately change config semantics with regression tests.

### Existing app-server boundary

- `internal/codexapp/auth.go` and tests model `account/login/start` `chatgptAuthTokens` plus stdio env stripping.
- HO-1288 is not that path. It needs an HTTP backend transport; do not route it through app-server login helpers.

## Official OpenAI Evidence

Official docs lookup was restricted to OpenAI domains on 2026-05-10.

- `https://developers.openai.com/api/docs/models/gpt-5.1-codex` describes Codex-family model usage through OpenAI API endpoints and marks Responses API support.
- `https://developers.openai.com/api/docs/models` states current OpenAI models are available through the Responses API and client SDKs.
- Search did not find official OpenAI documentation for `https://chatgpt.com/backend-api/codex/responses`, `ChatGPT-Account-Id`, or `originator: codex_cli_rs`.

Conclusion: public OpenAI Platform API docs support keeping the normal OpenAI provider on `/v1/*`. The ChatGPT backend URL/header contract is a separate ChatGPT/Codex wire contract supplied by issue scope and local OpenClaw evidence, not a documented public Platform endpoint.

## Local OpenClaw/Codex Wire Evidence

### `memory-lancedb-pro` OAuth client

`/Users/n0rmanc/.openclaw/workspace/plugins/memory-lancedb-pro/src/llm-oauth.ts` records:

- provider backend base URL: `https://chatgpt.com/backend-api`
- endpoint normalization to `/codex/responses`
- OAuth authorize `originator: codex_cli_rs`

`/Users/n0rmanc/.openclaw/workspace/plugins/memory-lancedb-pro/src/llm-client.ts` sends:

- `Authorization: Bearer <session.accessToken>`
- `chatgpt-account-id: <session.accountId>`
- `originator: codex_cli_rs`
- `OpenAI-Beta: responses=experimental`
- `Accept: text/event-stream`
- `Content-Type: application/json`

`/Users/n0rmanc/.openclaw/workspace/plugins/memory-lancedb-pro/test/llm-oauth-client.test.mjs` asserts OAuth mode calls:

```text
https://chatgpt.com/backend-api/codex/responses
```

### OpenClaw provider bundle

`/Users/n0rmanc/.openclaw/tmp/jiti/providers-openai-codex-responses.2a998283.cjs` records endpoint normalization:

- default backend base URL plus `/codex/responses`
- `Authorization` bearer token
- `chatgpt-account-id`
- `originator`
- `OpenAI-Beta: responses=experimental`

This is not Tianji repo production source, but it is local working wire-contract evidence for the backend path HO-1288 targets.

## Decisions

### D1 - Treat backend transport as private/wire-contract scoped

Because official OpenAI Platform docs do not document this ChatGPT backend endpoint, implementation must be isolated and test-controlled. It should not be used as the default OpenAI provider.

### D2 - Default URL and originator

Use:

```text
https://chatgpt.com/backend-api/codex/responses
originator: codex_cli_rs
```

Allow config/test override for URL/originator.

### D3 - Header casing

Go headers are case-insensitive, but docs and tests should use `ChatGPT-Account-Id` as the canonical issue-facing spelling. Existing local evidence also uses lowercase in JS runtime; both resolve to the same HTTP header key semantically.

### D4 - Explicit transport selection

Do not infer all `openai_subscription_credential_ids` routes should use ChatGPT backend. Existing direct OpenAI endpoint coverage from HO-1179 must remain compatible. Add an explicit transport/config signal.

### D5 - RED tests own the contract

Because the backend endpoint is not publicly documented by official OpenAI API docs, RED tests against `httptest.Server` are the primary regression contract for URL, headers, body, and no-Platform-call behavior.
