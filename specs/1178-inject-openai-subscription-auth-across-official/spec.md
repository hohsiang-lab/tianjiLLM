# Feature Specification: Inject OpenAI Subscription Auth Across Official Endpoints

**Feature Branch**: `HO-1178-inject-openai-subscription-auth-across-official`
**Created**: 2026-05-07
**Input**: Linear HO-1178 - `[BE] Inject OpenAI subscription auth across official endpoints`

## Summary

Apply the already-resolved OpenAI subscription bearer material from HO-1167/HO-1177 to every official OpenAI endpoint TianjiLLM serves, while preserving existing API-key behavior and excluding custom `api_base` / OpenAI-compatible providers.

## Scope

- Direct OpenAI HTTP endpoints:
  - `/v1/chat/completions`
  - `/v1/completions`
  - `/v1/responses`
  - `/v1/embeddings`
  - `/v1/images/generations`
  - `/v1/images/edits`
  - `/v1/images/variations`
  - `/v1/audio/transcriptions`
  - `/v1/audio/speech`
  - official model/test upstream helper behavior; repo reality shows `/v1/models` itself is a local configured-model list and does not call upstream OpenAI.
- Use `openai_subscription_credential_ids` bearer material before `api_key` when configured.
- Keep `api_key` fallback unchanged when subscription IDs are omitted or explicitly empty.
- Keep custom `api_base` / OpenAI-compatible provider behavior API-key based.
- Add endpoint-level tests that assert upstream `Authorization: Bearer <subscription access token>` for each official endpoint in scope.

## Out of Scope

- Production implementation while Linear is Todo.
- Creating or storing OpenAI subscription credentials; HO-1176 owns persistence.
- OAuth connect/callback lifecycle; HO-1175 owns connect/callback state.
- Token refresh manager internals; HO-1177 owns refresh and usable-token selection.
- Redaction sink expansion; HO-1183 owns shared redaction.
- Applying subscription credentials to custom `api_base`, `openaicompat`, Azure, Groq, DeepInfra, or other OpenAI-compatible providers.
- Real OpenAI network calls in tests.
- Assistants, files, batches, fine-tuning, vector stores, containers, agents, skills, RAG, moderation, realtime, or vendor-native passthrough unless a later issue explicitly adds them.

## User Stories

### US1 - Official text endpoints use subscription bearer auth

As an operator with a configured OpenAI subscription credential, I want chat/completions, legacy completions, and responses requests to reach official OpenAI with the subscription access token so API-key configuration is not required for those official endpoints.

**Acceptance Scenarios**

1. Given a model config has `openai_subscription_credential_ids: ["cred_a"]`, when `/v1/chat/completions` calls an official OpenAI deployment, then the upstream request has `Authorization: Bearer <resolved access token>`.
2. Given the same config, when `/v1/completions` calls official OpenAI, then the upstream request uses the same resolved subscription bearer token.
3. Given the same config, when `/v1/responses` calls official OpenAI, then the reverse-proxy request uses the resolved subscription bearer token instead of `api_key`.

### US2 - Official embeddings, image, and audio endpoints use subscription bearer auth

As an operator, I want non-chat official OpenAI endpoints to use the same subscription credential path so auth behavior is consistent across request types.

**Acceptance Scenarios**

1. Given a subscription-configured embedding model, when `/v1/embeddings` is requested, then upstream receives the subscription bearer token.
2. Given a subscription-configured image model, when `/v1/images/generations`, `/v1/images/edits`, or `/v1/images/variations` is requested, then upstream receives the subscription bearer token.
3. Given a subscription-configured audio model, when `/v1/audio/transcriptions` or `/v1/audio/speech` is requested, then upstream receives the subscription bearer token.

### US3 - API key and custom base URL behavior remains unchanged

As a proxy operator, I need existing API-key deployments and custom OpenAI-compatible providers to keep working exactly as before.

**Acceptance Scenarios**

1. Given no subscription IDs are configured, when any endpoint in scope is called, then upstream auth is still `Authorization: Bearer <api_key>`.
2. Given `openai_subscription_credential_ids: []`, when any endpoint in scope is called, then behavior is identical to omitted IDs.
3. Given a deployment uses custom `api_base`, when a request is routed to that deployment, then subscription auth is not applied and existing API-key auth remains the only supported behavior.

### US4 - Official model/test upstream helper has explicit auth behavior

As an operator, I want the official OpenAI model/test upstream helper to prove subscription bearer auth can be used for upstream capability checks without leaking or falling back to API keys.

**Acceptance Scenarios**

1. Given an official OpenAI deployment with subscription IDs, when the model/test endpoint performs an upstream OpenAI request, then it uses the subscription bearer token.
2. Given no subscription IDs, when the same model/test path runs, then it uses the configured API key as before.
3. Given a custom `api_base`, when the model/test path runs, then it remains API-key based.

## Functional Requirements

- **FR-001**: Endpoint auth resolution MUST use HO-1167/HO-1177 `resolveOpenAIAPIKeyForParams` or an equivalent context-aware wrapper so subscription credentials get refreshed usable bearer material before upstream dispatch.
- **FR-002**: If `openai_subscription_credential_ids` is non-empty, upstream official OpenAI requests MUST set `Authorization: Bearer <resolved access token>`.
- **FR-003**: If subscription IDs are omitted or empty, upstream official OpenAI requests MUST preserve current `api_key` behavior.
- **FR-004**: Subscription resolution failures MUST return safe request errors and MUST NOT fallback to `api_key`.
- **FR-005**: The implementation MUST NOT apply subscription auth to any non-empty custom `api_base` or OpenAI-compatible provider.
- **FR-006**: Chat completion streaming and non-streaming paths MUST share the same auth resolution result.
- **FR-007**: Legacy completions MUST share the official OpenAI auth path, not duplicate ad hoc API-key extraction.
- **FR-008**: Responses reverse proxy MUST resolve auth from the matched official OpenAI model config before setting upstream `Authorization`.
- **FR-009**: Embeddings MUST continue using provider `TransformEmbeddingRequest`, but the `apiKey` argument MUST be subscription bearer material when subscription IDs are configured.
- **FR-010**: Images generation/edit/variation endpoints MUST use subscription bearer auth for official OpenAI models.
- **FR-011**: Audio transcription/speech endpoints MUST use subscription bearer auth for official OpenAI models.
- **FR-012**: Model/test upstream helper behavior MUST have tests proving subscription bearer, API-key regression, and custom-base exclusion. `/v1/models` remains a local configured-model list because it has no upstream OpenAI request path in the current repo.
- **FR-013**: Tests MUST use `httptest` / existing `internal/testutil/openaitest` mocks and MUST NOT call `api.openai.com` or `auth.openai.com`.
- **FR-014**: Tests MUST assert upstream auth headers do not contain client virtual keys, fallback API keys, refresh tokens, ID tokens, JWTs, or code verifiers.
- **FR-015**: Official endpoint detection MUST be based on repo provider/config reality, not request path strings alone.

## Edge Cases

- Router path uses `Router.Route` rather than direct config lookup.
- Wildcard model config resolves a bare OpenAI model name.
- Streaming chat uses the same `apiKey` variable as non-streaming chat.
- Multipart audio/image edit requests must preserve their original body/content type while replacing auth.
- Responses proxy currently uses assistants proxy machinery; implementation must avoid accidentally applying Assistants-only `OpenAI-Beta` behavior to `/v1/responses`.
- Existing `resolveAssistantsUpstream` returns only `(base, apiKey)` and does not surface errors; this may need a narrow replacement for in-scope official endpoints.

## Success Criteria

- Endpoint matrix tests fail before implementation and pass after implementation for subscription bearer injection.
- API-key regression tests pass for every endpoint family in scope.
- Custom `api_base` tests prove subscription auth is not applied outside official OpenAI.
- `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/...` passes offline.
- PR diff for Todo planning contains only SpecKit artifacts.

## Dependencies

- HO-1167: `openai_subscription_credential_ids` config and resolver boundary.
- HO-1175: OAuth connect/callback lifecycle.
- HO-1176: encrypted subscription credential persistence.
- HO-1177: usable token refresh manager.
- HO-1183: secret redaction.
- HO-1190: reusable OpenAI OAuth/upstream mock harness.
