# Research: Inject OpenAI Subscription Auth Across Official Endpoints

## Repo Evidence

- `internal/provider/openai/openai.go` defines default official OpenAI base URL as `https://api.openai.com/v1` and currently sets `Authorization: Bearer ` plus the caller-provided API key in `SetupHeaders`.
- `internal/proxy/handler/openai_subscription_resolution.go` already resolves configured subscription credential IDs into direct HTTP bearer material through `resolveOpenAIAPIKeyForParams`.
- `internal/proxy/handler/openai_subscription_resolution_test.go` already proves:
  - explicit credential IDs only;
  - no API-key fallback after subscription resolution failure;
  - no subscription IDs keeps the API-key path;
  - direct HTTP returns bearer material;
  - Codex app-server uses `chatgptAuthTokens` instead of raw HTTP bearer injection.
- `internal/proxy/server.go` routes official OpenAI-compatible endpoints under `/v1`: chat, completions, embeddings, images, audio, responses, and models.
- `internal/proxy/handler/embedding.go`, `images.go`, and `audio.go` already call `resolveProviderFromConfig`, so they can inherit subscription bearer material if the selected deployment is official OpenAI.
- `internal/proxy/handler/chat.go` calls `resolveProvider` and already passes the resulting auth material into streaming and non-streaming paths.
- `internal/proxy/handler/assistants.go` currently resolves upstream auth through `resolveAssistantsUpstream`, which returns static API key material and is reused by `CreateResponse`; this is the highest-risk gap for `/v1/responses`.
- `internal/proxy/handler/images.go` implements only image generations directly; image edits and variations route to native-format handlers in `internal/proxy/handler/native_format.go`.

## Official OpenAI Docs Evidence

- OpenAI API reference authentication requires HTTP bearer auth: `Authorization: Bearer OPENAI_API_KEY`.
- Official docs list these API endpoints as `https://api.openai.com/v1/...`: chat completions, responses, embeddings, images, audio, and models.
- The issue scope treats the stored subscription access token as bearer material replacing the API key value on official OpenAI endpoints. This plan does not claim OpenAI public docs document Tianji's subscription credential field; that is Tianji-owned behavior from HO-1167/1176/1177.

Official sources checked:

- https://platform.openai.com/docs/api-reference/authentication
- https://platform.openai.com/docs/api-reference/chat/create
- https://platform.openai.com/docs/api-reference/responses/create
- https://platform.openai.com/docs/api-reference/embeddings
- https://platform.openai.com/docs/api-reference/images/generate
- https://platform.openai.com/docs/api-reference/audio/createTranscription
- https://platform.openai.com/docs/api-reference/models

## Context7 / GitHub / Web Review

- Context7 command attempted: `npx -y ctx7@latest library net/http "Go reverse proxy request header authorization bearer official OpenAI endpoints"`.
  - Result: only OpenTelemetry `otelhttp` package surfaced; no useful Go `net/http` header-setting guidance for this plan.
- grep-app command attempted: `/Users/n0rmanc/.cargo/bin/grep-app-cli --json 'Authorization: Bearer' --language Go`.
  - Result: grep-app transport failed with `Unexpected content type: Some("text/html; charset=utf-8")`.
- Official web search returned OpenAI docs for authentication and endpoint references. The plan relies on official OpenAI docs plus repo code rather than public GitHub examples for endpoint auth injection.

## Decisions

### Decision 1: Treat subscription bearer material as the existing `apiKey` string boundary for direct HTTP providers

Use `resolveOpenAIAPIKeyForParams(ctx, params)` and pass its result to provider request builders. Existing `openai.Provider.SetupHeaders` already emits `Authorization: Bearer <value>`.

Rejected alternative: add a separate `SubscriptionAuth` header mechanism to the provider interface. That would increase blast radius across all providers even though the OpenAI provider already has the exact header behavior needed.

### Decision 2: Fix reverse-proxy endpoints by resolving a deployment-aware official OpenAI upstream, not by reading global assistant settings

`/v1/responses` should not inherit Assistants-only static API-key behavior. Add or adapt a narrow helper that selects official OpenAI config, resolves subscription bearer material, and sets the upstream URL/path without applying custom-base subscription behavior.

Rejected alternative: inject subscription auth inside `assistantsProxy` globally. That could unintentionally alter Assistants/files/vector-store behavior outside HO-1178 scope.

### Decision 3: Use endpoint matrix tests instead of one broad smoke test

Each official endpoint family must assert the upstream auth header because these handlers build upstream requests in different ways: provider transform, manual raw proxy, reverse proxy, and native-format pass-through.

Rejected alternative: only test `resolveOpenAIAPIKeyForParams`. That was already covered by HO-1167/1177 and would miss endpoint-specific bypasses.

## Open Questions

- None requiring owner input for Todo planning. The scope boundary excludes Assistants/files/etc. even though those are also OpenAI official APIs in the repo routing table; the Linear issue explicitly names chat/completions, responses, embeddings, images, audio, and models/test endpoint.
