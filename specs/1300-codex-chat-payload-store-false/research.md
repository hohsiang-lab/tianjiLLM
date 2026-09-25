# Research: HO-1300 Codex backend `store:false`

## Memory Recall

- `memory/2026-05-11.md` records the production failure chain: `openai/gpt-5.5` via `chatgpt_codex_backend` fails with `Store must be set to false`; adding client `store:false` then fails because it is captured as `ExtraParams` and rejected as unknown.
- `memory/2026-05-10.md` records HO-1288 scope: ChatGPT Codex backend transport must call `/backend-api/codex/responses`, preserve API-key OpenAI behavior, and use mocked tests.
- Wiki search found prior Tianji code review lessons from HO-1290: Codex backend error handling must be scoped to `chatgpt_codex_backend` and must not regress Platform OpenAI behavior.

## Repo Evidence

- `internal/provider/chatgptcodex/payload.go`: `Payload` does not define `Store`; `validateSupported` rejects any `ExtraParams`.
- `internal/model/request.go`: `knownFields` does not include `store`, so top-level `store` enters `ExtraParams`.
- `internal/proxy/handler/chat.go`: unknown params are logged, then Codex routes call `handleChatGPTCodexBackendCompletion`.
- `internal/provider/openai/openai.go`: normal OpenAI provider merges `ExtraParams` into the outgoing chat-completions body.
- `specs/1288-chatgpt-codex-backend-transport/contracts/chatgpt-codex-backend-transport.md`: existing contract already expected a minimum Codex backend body containing `"store": false`.

## Official Docs Evidence

- Official OpenAI Responses create reference is available at <https://developers.openai.com/api/reference/resources/responses/methods/create>.
- The fetched reference text says reasoning items can be used in stateless Responses API mode when `store` is set to `false`. This supports treating `store:false` as the stateless contract.
- No official public docs were found for private `chatgpt.com/backend-api/codex/responses`; use repo-owned contracts and local mocks for that backend.

## External Lookup Notes

- Context7 lookup for OpenAI was attempted and returned monthly quota exceeded.
- `grep-app-cli` search for public OpenAI SDK `store:false` examples returned no useful match and one grep.app transport content-type error.
- These misses are non-blocking because the issue's required behavior is explicit in Linear, current production repro, and repo-owned HO-1288 Codex contract.

## Decision

Implement `store:false` inside `internal/provider/chatgptcodex`, and keep request parsing/global OpenAI provider behavior unchanged. This directly fixes the production failure while preserving the existing API-key path.
