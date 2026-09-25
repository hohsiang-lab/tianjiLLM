# Implementation Plan: Inject OpenAI Subscription Auth Across Official Endpoints

**Branch**: `HO-1178-inject-openai-subscription-auth-across-official`
**Spec**: `specs/1178-inject-openai-subscription-auth-across-official/spec.md`
**Linear**: HO-1178

## Technical Context

- Language: Go
- Relevant packages:
  - `internal/provider/openai`
  - `internal/proxy/handler`
  - `internal/proxy`
  - `internal/router`
  - `internal/testutil/openaitest`
- Existing auth boundary:
  - `resolveOpenAIAPIKeyForParams(ctx, config.TianjiParams)` returns either API key or subscription access token.
  - `openai.Provider.SetupHeaders(req, apiKey)` sets bearer auth.
- Existing risk:
  - Some official endpoints go through `resolveProviderFromConfig` and likely inherit subscription material.
  - `/v1/responses` currently calls `assistantsProxy`, whose `resolveAssistantsUpstream` only returns static API-key material.
  - Native image edit/variation paths need explicit verification because they are not implemented in `images.go`.

## Constitution Check

- No production code in Todo state.
- Implementation must start with failing tests when Linear moves to In Progress.
- No real OpenAI network calls.
- Secrets must not be logged, returned, or persisted by new tests.
- Scope stays on official OpenAI endpoint auth only; no UI, no new credential schema, no broad provider interface rewrite.

## Endpoint Matrix

| Endpoint family | Current handler | Expected implementation direction |
| --- | --- | --- |
| Chat completions | `ChatCompletion` + `resolveProvider` | Add/extend test proving router and direct config subscription bearer reach upstream for streaming/non-streaming. |
| Legacy completions | `Completion` | Use same official OpenAI auth resolution path as chat; add test for bearer and API-key fallback. |
| Responses | `CreateResponse` -> `assistantsProxy` | Replace/extend proxy resolution for `/v1/responses` so it resolves subscription bearer from model config and official OpenAI base only. |
| Embeddings | `Embedding` + provider `TransformEmbeddingRequest` | Add endpoint test; implementation probably inherited through `resolveProviderFromConfig`. |
| Images generation | `ImageGeneration` | Add endpoint test; implementation probably inherited through `resolveProviderFromConfig`. |
| Images edits/variations | native-format handlers | Inspect and adapt auth resolution in native-format path; add tests for both endpoints. |
| Audio transcription/speech | `AudioTranscription`, `AudioSpeech` | Add multipart and JSON endpoint tests proving bearer material. |
| Models/test | `resolveProviderBaseURLWithContext` helper | Repo reality shows `/v1/models` is local-only. Cover the upstream helper used by model-test style pass-through endpoints with subscription/API-key/custom-base tests. |

## Data Flow

1. Client sends request with virtual key to TianjiLLM.
2. Middleware authenticates the virtual key and attaches request context.
3. Handler selects the model config and provider.
4. For official OpenAI deployments:
   - if `openai_subscription_credential_ids` is non-empty, resolve and refresh usable subscription access token;
   - set upstream `Authorization: Bearer <access token>`.
5. For omitted/empty subscription IDs:
   - preserve current API-key value and header.
6. For custom `api_base` / OpenAI-compatible providers:
   - do not apply subscription auth; existing config validation should reject subscription IDs with custom base, and endpoint logic must not bypass that rule.

## Test Plan

- Add endpoint-level tests in `internal/proxy/handler/*_test.go` using `httptest.NewServer`.
- Use encrypted test credential helpers from existing HO-1176/1177 tests where possible.
- For each endpoint family:
  - subscription configured -> upstream sees `Authorization: Bearer subscription-access`;
  - no subscription IDs -> upstream sees `Authorization: Bearer sk-existing`;
  - custom `api_base` / compatible provider -> no subscription resolution and API-key behavior remains.
- Add at least one negative assertion that client virtual key and fallback API key do not reach upstream when subscription IDs are configured.
- Run:
  - `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/... -run 'OpenAI|Subscription|Chat|Completion|Response|Embedding|Image|Audio|Model'`
  - `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/...`

## Implementation Steps

1. Write failing endpoint matrix tests for all in-scope official endpoints.
2. Add helper(s) that resolve deployment-aware OpenAI auth material with request context.
3. Route chat/completions/embeddings/images/audio through the shared helper where existing direct resolution is insufficient.
4. Split `/v1/responses` from generic Assistants proxy behavior or pass an explicit proxy mode so responses uses subscription-aware auth without changing Assistants scope.
5. Patch native image edit/variation auth path if tests show it bypasses subscription resolution.
6. Cover model/test upstream helper behavior after confirming `/v1/models` is local-only.
7. Run targeted and package tests; update SpecKit tasks checkboxes only after implementation.

## Risk Controls

- Do not alter provider registration for non-OpenAI providers.
- Do not change `openaicompat` auth header behavior.
- Do not add subscription ID scanning or fallback selection.
- Do not catch subscription resolution errors and continue with API key.
- Avoid broad `provider.Provider` interface changes unless tests prove no smaller path exists.

## Plan Review

- Context7 attempted for Go `net/http`; no relevant header-auth implementation guidance returned.
- grep-app attempted for public Go `Authorization: Bearer` examples; tool returned HTML content-type transport error.
- Official OpenAI docs confirm bearer auth and in-scope `api.openai.com/v1` endpoints.
- No plan revision required from external evidence; repo-specific endpoint handlers are the source of truth.
