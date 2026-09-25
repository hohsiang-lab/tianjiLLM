# Feature Specification: Runtime routes consume UI DB-managed models

**Feature Branch**: `HO-1285-runtime-routes-db-models`
**Created**: 2026-05-10
**Input**: Linear HO-1285 - `Runtime routes ignore UI DB-managed models`
**Phase**: Todo planning only; production implementation starts only after Linear moves to `In Progress`.

## Summary

TianjiLLM Models UI and `/model/*` APIs already persist model rows in `ProxyModelTable`, but runtime routing, listing, provider resolution, and discovery still use only YAML `cfg.ModelList`. A model created from `/ui/models` is therefore visible to operators and key/team restriction UIs, but unavailable to `/v1` runtime traffic. This issue makes YAML config models and DB-managed models flow through one merged runtime model source.

## Root Cause

`cmd/tianji/main.go` builds `router.New(cfg.ModelList, ...)` once from YAML config. `internal/proxy/handler/handler.go` then lists and resolves models by iterating `h.Config.ModelList`, while `internal/proxy/handler/discovery.go` explicitly reports `h.Router.ListModelGroups()` and notes DB-managed models are outside the router. The UI writes `ProxyModelTable` rows, but no runtime component loads those rows into the router/resolver source, so runtime routes cannot see them.

## Scope

- Add a shared runtime model source that merges YAML `config.ModelList` and DB rows from `ProxyModelTable`.
- Make `/v1/models` and bare `/models` list the merged source.
- Make chat routing resolve DB-managed model rows through the same provider selection path as config models.
- Cover at least one non-chat route that currently resolves through `resolveProviderFromConfigRouteWithContext` or `resolveProviderBaseURL`, and audit all issue-listed runtime/discovery paths for config-only model lookup.
- Make `/model_group/info` report merged model groups, not only the startup YAML router.
- Define duplicate and wildcard merge rules between YAML and DB-managed models.
- Define refresh behavior after `/ui/models/*` or `/model/new|update|delete` changes.
- Preserve current YAML-only behavior when DB is unavailable or empty.

## Affected Route Inventory

The implementation audit must explicitly classify each issue-listed path as fixed by the merged runtime model source, unaffected because it does not use model config, or follow-up with owner-backed reason:

- Model listing: `/v1/models`, bare `/models`.
- OpenAI-compatible model routes: `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/transcriptions`, `/v1/audio/speech`, `/v1/moderations`, `/v1/rerank`, and bare equivalents.
- Azure-compatible model routes: `/engines/{model}/chat/completions`, `/engines/{model}/completions`, `/engines/{model}/embeddings`, `/openai/deployments/{model}/chat/completions`, `/openai/deployments/{model}/completions`, `/openai/deployments/{model}/embeddings`.
- OpenAI resource/pass-through routes using `resolveProviderBaseURL`, `resolveAssistantsUpstream`, or `resolveOpenAIEndpointRouteForRequest`: files, batches, fine-tuning, assistants, threads, vector stores, responses, image edits, image variations, OCR, videos, containers, RAG, and Anthropic batch pass-through.
- Native provider routes using configured upstreams: `/v1/messages`, `/v1/messages/count_tokens`, `/v1beta/models/{model}:generateContent`, `/v1beta/models/{model}:streamGenerateContent`, `/v1beta/models/{model}:countTokens`.
- Discovery: `/model_group/info`.
- DB management surfaces: `/ui/models/*` and `/model/new|info|update|delete`.
- Restriction selector surfaces: key/team UI model-name lists can expose DB model names but do not make runtime routing work by themselves.

## Out of Scope

- Todo state production implementation.
- Replacing `ProxyModelTable` schema.
- Changing provider implementations.
- Changing OpenAI subscription credential resolution semantics from HO-1167 / HO-1172.
- Adding a distributed cross-process hot reload mechanism.
- Real upstream OpenAI/Anthropic/Gemini network calls in tests.

## User Stories and Tests

### User Story 1 - Runtime clients can list DB-managed models (P1)

Operator creates a model in the Models UI or `/model/new`; API clients then see the same model in `/v1/models` without editing YAML.

**Independent Test**: Integration or E2E seed a `ProxyModelTable` row, call `/v1/models`, and assert the DB model appears with the existing OpenAI-compatible list response shape.

**Acceptance Scenarios**:

1. Given YAML has no model named `db-chat-model`, when DB contains `ProxyModelTable.model_name = db-chat-model`, then `/v1/models` returns `db-chat-model`.
2. Given the same name exists in YAML and DB, when listing models, then the response contains one entry following the documented precedence rule.
3. Given DB is unavailable, when `/v1/models` is called, then existing YAML-only listing still works and does not panic.

### User Story 2 - Chat completion routes can use DB-managed models (P1)

Client sends `/v1/chat/completions` with a model created from the UI; TianjiLLM resolves it to provider/model/upstream config and forwards to the mocked upstream.

**Independent Test**: Seed DB row with `tianji_params.model = openai/gpt-4o-mini` and mocked `api_base`; call `/v1/chat/completions`; assert upstream receives the request and model resolution uses the DB row.

**Acceptance Scenarios**:

1. Given DB-managed model has default OpenAI params, when chat completion is requested, then routing resolves and forwards successfully.
2. Given DB-managed model has `api_base`, when chat completion is requested, then the provider uses that base URL.
3. Given DB-managed model has access control, when unauthorized caller requests it, then access is denied consistently with YAML router access control.

### User Story 3 - Non-chat runtime routes use the merged source (P1)

Routes that bypass `Router.Route` and use config lookup helpers must not keep ignoring DB-managed models.

**Independent Test**: Cover one non-chat route such as `/v1/embeddings`, `/v1/rerank`, or a pass-through route using `resolveProviderBaseURL`, with a DB-only model row and mocked upstream.

**Acceptance Scenarios**:

1. Given DB-managed embedding model exists, when `/v1/embeddings` is called, then resolution uses the DB row.
2. Given DB-managed OpenAI-compatible model exists, when a route uses `resolveProviderBaseURL(model)`, then it can resolve the DB model.
3. Given no model parameter is supplied to a resource route, fallback selection must document whether YAML or DB OpenAI-compatible models are considered first.

### User Story 4 - Discovery reports merged model groups (P2)

`/model_group/info` should not contradict `/v1/models`; DB-managed model groups should appear in discovery with provider/deployment info where enough metadata exists.

**Independent Test**: Seed DB model row and call `/model_group/info`; assert model group appears with provider and deployment count.

**Acceptance Scenarios**:

1. Given DB model has model info/pricing fields, discovery includes the group with matching metadata where supported.
2. Given DB model lacks optional pricing fields, discovery still returns the group with safe defaults.
3. Given DB unavailable, existing router-based discovery behavior remains unchanged.

### User Story 5 - Model edits and deletes update runtime behavior (P1)

Operators should not create a model that remains unusable until an undocumented restart.

**Independent Test**: Create/update/delete through `/model/*` or handler-level equivalent; assert the runtime source refreshes or returns an explicit restart-required behavior documented in the API/UI response.

**Acceptance Scenarios**:

1. Given a DB model is created, when the change completes, then subsequent runtime lookups see it or the response explicitly says restart required.
2. Given a DB model is updated, when a runtime request arrives, then it uses the updated params or the documented restart behavior.
3. Given a DB model is deleted, when a runtime request uses it, then it is not routable after refresh or the documented restart behavior.

## Functional Requirements

- **FR-001**: Runtime model listing MUST read a merged source of YAML config models plus DB-managed `ProxyModelTable` rows.
- **FR-002**: Provider resolution helpers MUST resolve DB-managed models for exact model names.
- **FR-003**: Chat completion routing MUST support DB-managed models through the same access-control, fallback, and provider resolution semantics as YAML models where applicable.
- **FR-004**: At least one non-chat route using `resolveProviderFromConfigRouteWithContext` or `resolveProviderBaseURL` MUST be covered by a RED test and fixed.
- **FR-005**: `/model_group/info` MUST include DB-managed model groups when DB is available.
- **FR-006**: The merge rule MUST define duplicate-name precedence between YAML and DB rows.
- **FR-007**: The merge rule MUST define wildcard behavior. DB-managed wildcard model names MUST either be supported with the existing wildcard specificity rules or explicitly rejected/ignored by validation.
- **FR-008**: The merged source MUST preserve existing YAML-only behavior when DB is nil, down, or has no rows.
- **FR-009**: Runtime refresh after `/ui/models/*` and `/model/new|update|delete` MUST be implemented or the UI/API MUST expose explicit restart-required behavior.
- **FR-010**: No route may keep a private ad-hoc config-only lookup for model names after this fix if it participates in runtime routing/discovery.
- **FR-011**: Tests MUST be written RED first before production changes.
- **FR-012**: Tests MUST use mocked upstreams and seeded DB rows; no real provider network calls.
- **FR-013**: Error responses for missing/invalid DB model params MUST not leak API keys, credential IDs beyond configured model IDs, or raw JSON blobs.
- **FR-014**: OpenAI subscription credential behavior from existing `tianji_params.openai_subscription_credential_ids` MUST remain compatible.
- **FR-015**: The implementation MUST record an affected-route audit result for every path family in the affected route inventory.

## Key Entities

- **Runtime Model Source**: The shared provider of merged model configs used by listing, routing, provider resolution, and discovery.
- **Config Model**: A YAML `config.ModelConfig` entry from `cfg.ModelList`.
- **DB-Managed Model**: A `ProxyModelTable` row with `model_name`, `tianji_params`, and `model_info`.
- **Merged Model Config**: A `config.ModelConfig`-compatible runtime representation created from a config model or DB-managed model.
- **Refresh Boundary**: The code path that reloads DB-managed rows into the runtime source after model create/update/delete.

## Merge Rule

- Exact model names are unique by `model_name`.
- DB-managed model rows take precedence over YAML config rows for the same exact name, matching the Models UI existing display behavior where DB rows are considered authoritative when DB is available.
- YAML config rows remain available for names not present in DB.
- Wildcard support must be explicit:
  - Preferred: support DB wildcard `model_name` with the same exact-first, then wildcard specificity order already used by `findModelConfig`.
  - Minimum acceptable fallback: reject wildcard names in DB create/update validation and document that only YAML supports wildcard models.
- The implementation must choose one rule and encode it in tests.

## Success Criteria

- **SC-001**: RED test proves DB-only model appears in `/v1/models` only after the implementation.
- **SC-002**: RED test proves DB-only model routes `/v1/chat/completions` to mocked upstream only after the implementation.
- **SC-003**: RED test proves at least one non-chat route resolves DB-only model only after the implementation.
- **SC-004**: Discovery test proves `/model_group/info` includes DB-managed model groups.
- **SC-005**: Duplicate-name test proves DB-vs-YAML precedence is deterministic.
- **SC-006**: Refresh or restart-required behavior is covered for create/update/delete.
- **SC-007**: Existing YAML model tests still pass.
- **SC-008**: Route audit evidence maps every issue-listed path family to merged-source coverage, unaffected status, or explicit follow-up rationale.

## Dependencies

- HO-1167 typed `tianji_params.openai_subscription_credential_ids` and resolver behavior.
- HO-1172 Models UI DB persistence and safe credential selection.
- Existing `ProxyModelTable` sqlc queries.
- Existing router access-control and wildcard matching code.
