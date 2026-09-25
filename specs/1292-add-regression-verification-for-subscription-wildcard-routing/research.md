# Research: Subscription wildcard routing regression verification

## Repo Evidence

- `internal/proxy/handler/handler.go` `findModelConfig` performs exact match first, then wildcard match via `internal/wildcard`, and resolves captured `*` segments into `tianji_params.model`.
- `internal/proxy/handler/chat.go` `resolveProviderRoute` attaches `ChatGPTCodexBackend` when the selected model config has `openai_subscription_transport = "chatgpt_codex_backend"`.
- `internal/proxy/handler/chat.go` dispatches non-streaming Codex routes to `handleChatGPTCodexBackendCompletion`.
- `internal/proxy/handler/chatgpt_codex_backend_test.go` already proves exact Codex backend routing, required headers, unsupported streaming, redaction, and actionable 401/403/429 errors.
- `test/contract/wildcard_test.go` proves basic `openai/*` wildcard matching but does not combine wildcard routing with subscription credentials or Codex transport.
- `internal/provider/openai/openai_test.go` proves normal API-key OpenAI routes call `https://api.openai.com/v1/chat/completions` and keep chat-completions request shape.
- `internal/config/validate.go` requires known `openai_subscription_transport` when subscription credential IDs are configured and rejects custom `api_base` with subscription credentials.

## Official Documentation

- OpenAI API authentication docs describe Platform API bearer API-key authentication and response/debugging headers including `x-request-id` and `x-ratelimit-*`: <https://platform.openai.com/docs/api-reference/authentication?api-mode=responses>.
- OpenAI API error docs classify 401 invalid authentication and 429 rate-limit/quota failures separately, supporting distinct diagnostics instead of generic 502s: <https://platform.openai.com/docs/guides/error-codes/api-errors>.
- OpenAI Responses API docs describe `POST /v1/responses`, typed `input`, and `instructions` on the public Platform API surface: <https://platform.openai.com/docs/api-reference/responses/retrieve>.
- Official docs search did not find a public contract for private `chatgpt.com/backend-api/codex/responses`. Treat that path as repo-owned/private and verify through local mocks.

## GitHub Prior Art

`grep-app-cli --json 'ResponseInputItem::Message' --language Rust --repo openai/codex` found real Codex code using `ResponseInputItem::Message { role, content, phase }` and `ContentItem::InputText` in:

- `openai/codex` `codex-rs/protocol/src/models.rs`
- `openai/codex` `codex-rs/core/src/goals.rs`
- `openai/codex` `codex-rs/core/src/session/mod.rs`

This supports exact typed input assertions in TianjiLLM's Codex payload tests. A direct grep-app query for `/backend-api/codex/responses` failed with upstream grep.app HTML content-type response, so HO-1292 should not depend on that query.

## Decision: Handler-Level Regression Tests

Use handler package tests as the primary regression boundary.

Reasons:

- The bug class spans wildcard config lookup, subscription credential resolution, transport flag selection, Codex backend request building, and HTTP error writing.
- Provider unit tests alone cannot prove wildcard route selection.
- E2E UI tests are unnecessary for this non-UI regression because HO-1291 already covers selector persistence.

## Decision: Local Mocks Only

Use `httptest.Server` for Codex backend and Platform wrong-route sentinel. Do not call real OpenAI, ChatGPT, or OpenClaw services.

Reasons:

- Regression is about routing correctness, not provider availability.
- Real calls would require secrets and introduce flake.
- The issue explicitly requires verification instructions that do not print secrets.

## Decision: API-Key Wildcard Regression Is In Scope

Add a normal API-key `openai/*` wildcard test alongside subscription wildcard tests.

Reasons:

- The issue explicitly says normal API-key OpenAI routes are unaffected.
- This is the highest-value regression guard against over-broad `openai/*` Codex routing.
- Existing API-key tests cover exact provider behavior but not the wildcard alias shape used by OpenClaw rollout.

## Open Questions

None for Todo scope. Implementation can proceed after Linear `In Progress`.
