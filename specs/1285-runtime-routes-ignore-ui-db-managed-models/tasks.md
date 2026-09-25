# Tasks: Runtime routes consume UI DB-managed models

**Input**: SpecKit artifacts in `specs/1285-runtime-routes-ignore-ui-db-managed-models/`
**Prerequisites**: Linear HO-1285 must move to `In Progress` before production implementation.

## Phase 0: Governance

- [x] T001 Confirm Linear HO-1285 is `In Progress` before editing production files.
- [x] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1285-runtime-routes-db-models`.
- [x] T003 Re-read this spec, Linear description, and current `ProxyModelTable` / router code before implementation.
- [x] T004 Confirm branch diff before implementation contains only HO-1285 SpecKit docs.

## Phase 1: RED tests first

- [x] T005 Add failing test proving `/v1/models` includes a DB-managed `ProxyModelTable` model.
- [x] T006 Add failing test proving `/v1/chat/completions` routes a DB-managed model to mocked upstream.
- [x] T007 Add failing test for one non-chat route such as `/v1/embeddings`, `/v1/rerank`, or `resolveProviderBaseURL(model)`.
- [x] T008 Add failing test proving `/model_group/info` includes DB-managed model group.
- [x] T009 Add failing unit test for DB-over-YAML duplicate-name precedence.
- [x] T010 Add failing unit test for YAML-only fallback when DB is nil or unavailable.
- [x] T011 Add failing test for create/update/delete refresh or explicit restart-required behavior.
- [x] T012 Add failing test for the chosen wildcard rule.
- [x] T013 Run targeted tests and confirm failures are missing implementation, not fixture/environment failures.

## Phase 2: Runtime model source

- [x] T014 Add runtime model source type near handler/server boundary.
- [x] T015 Add converter from `db.ProxyModelTable` to `config.ModelConfig`.
- [x] T016 Decode `tianji_params` with structured JSON into `config.TianjiParams`.
- [x] T017 Decode `model_info` into optional `config.ModelInfo` and access-control data where supported.
- [x] T018 Validate non-empty DB `model_name` and `tianji_params.model`.
- [x] T019 Implement DB-over-YAML exact-name merge.
- [x] T020 Implement selected wildcard rule.
- [x] T021 Documented alternative: use per-request merged read plus `Router.WithModels` instead of a long-lived snapshot, so UI/API DB mutations are visible without a separate refresh hook.
- [x] T022 Preserve previous/YAML snapshot when DB refresh fails.

## Phase 3: Runtime consumers

- [x] T023 Update `ListModels` to use merged source.
- [x] T024 Update `findModelConfig` or replace it with merged-source lookup.
- [x] T025 Update `resolveProviderFromConfigRouteWithContext` to consume merged lookup.
- [x] T026 Update `resolveProviderBaseURLWithContext` explicit model lookup to consume merged lookup.
- [x] T027 Document and test no-model OpenAI-compatible fallback order.
- [x] T028 Ensure chat `Router.Route` uses router built from merged snapshot.
- [x] T029 Ensure Azure-compatible `{model}` routes preserve model extraction behavior.

## Phase 4: Discovery and route audit

- [x] T030 Update `/model_group/info` to use merged router/source.
- [x] T031 Audit `/v1/completions`, `/v1/embeddings`, images, audio, moderations, rerank for config-only lookup.
- [x] T032 Audit files, batches, fine-tuning, assistants, threads, vector stores, responses, image edits, image variations, OCR, videos, containers, RAG, and Anthropic batch pass-through routes.
- [x] T033 Audit OpenAI endpoint helpers `resolveProviderBaseURL`, `resolveAssistantsUpstream`, and `resolveOpenAIEndpointRouteForRequest` for YAML-only fallback scans.
- [x] T034 Audit native routes `/v1/messages`, `/v1/messages/count_tokens`, and Gemini `/v1beta/models/{model}:...`.
- [x] T035 Produce affected-route audit evidence that classifies every issue-listed route family as merged-source covered, unaffected, or follow-up.
- [x] T036 Add route-level tests for any additional config-only path found in the audit.

## Phase 5: Model management refresh

- [x] T037 Covered by per-request merged runtime read after `/model/new`; no separate hook required.
- [x] T038 Covered by per-request merged runtime read after `/model/update`; no separate hook required.
- [x] T039 Covered by per-request merged runtime read after `/model/delete`; no separate hook required.
- [x] T040 UI create/update/delete writes become visible to runtime through the same per-request DB-backed source.
- [x] T041 DB load failure falls back to YAML behavior without leaking DB row payloads or API keys.
- [x] T042 Ensure successful refresh updates runtime listing/routing without process restart.

## Phase 6: Regression and security

- [x] T043 Run existing YAML route/list/discovery tests.
- [x] T044 Run existing OpenAI subscription credential resolver tests.
- [x] T045 Assert errors/logs do not leak API keys, bearer tokens, encrypted credential values, or raw credential JSON.
- [x] T046 Confirm DB-unavailable startup keeps YAML-only runtime alive.
- [x] T047 Confirm duplicate model listing returns one model entry.

## Phase 7: Verification and state progression

- [x] T048 Run targeted E2E/integration tests for HO-1285.
- [x] T049 Run affected handler/router/config/ui package tests.
- [x] T050 Run `go tool golangci-lint run`.
- [x] T051 Run `git diff --check origin/main...HEAD`.
- [x] T052 Verify PR diff contains only HO-1285 implementation files and this specs directory.
- [ ] T053 Move Linear to `In Review` and run review gate after implementation tasks complete.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Waiting Merge is blocked until Linear moves through `In Progress`, `In Review`, `Waiting CI`, and CI/result gates.
