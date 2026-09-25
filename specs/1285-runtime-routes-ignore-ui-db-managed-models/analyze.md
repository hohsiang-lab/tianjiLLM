# Analyze: HO-1285 Runtime routes ignore UI DB-managed models

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0
- Governance blocker: production implementation cannot start until Linear moves to `In Progress`.

## Linear Scope Coverage

| Linear explicit behavior | Spec coverage | Plan/tasks coverage |
| --- | --- | --- |
| DB model row appears in `/v1/models` | FR-001, US1, SC-001 | T005, T023 |
| DB model routes `/v1/chat/completions` through mocked upstream | FR-002, FR-003, US2, SC-002 | T006, T025, T028 |
| Cover at least one non-chat route | FR-004, US3, SC-003 | T007, T026-T027, T031-T036 |
| Merge rule between YAML and DB | FR-006, FR-007, Merge Rule | T009, T019-T020 |
| Runtime router/listing/discovery/provider resolution share one source | FR-001-FR-005, contract | T014-T030 |
| UI/API edits/deletes hot-refresh or expose restart-required behavior | FR-009, US5, SC-006 | T011, T037-T042 |
| `/model_group/info` reports DB-managed model groups | FR-005, US4, SC-004 | T008, T030 |
| All issue-listed affected route families audited | Affected Route Inventory, FR-015, SC-008 | T031-T036 |

## Issue Explicit Behavior Map

| Issue noun / verb | Artifact mapping | Alignment result |
| --- | --- | --- |
| Models UI and `/model/*` persist `ProxyModelTable` rows | `spec.md` Summary/Root Cause/US5, `data-model.md` `ProxyModelTable`, `tasks.md` T037-T042 | Covered |
| Runtime initialized/resolved from YAML `cfg.ModelList` | `spec.md` Root Cause, `research.md` startup/router evidence, `plan.md` Plan Review Evidence | Covered |
| `/v1/models` and bare `/models` | `spec.md` Affected Route Inventory/US1/FR-001/SC-001, `tasks.md` T005/T023 | Covered |
| `/v1/chat/completions` and bare chat route | `spec.md` US2/FR-003/SC-002, `tasks.md` T006/T025/T028 | Covered |
| `/v1/completions`, `/v1/embeddings`, images generation, audio, moderations, rerank | `spec.md` Affected Route Inventory/US3/FR-004/FR-015, `tasks.md` T007/T031 | Covered |
| Azure `/engines/{model}` and `/openai/deployments/{model}` route variants | `spec.md` Affected Route Inventory, `contracts/runtime-model-source.md` Required Consumers, `tasks.md` T029 | Covered |
| Files, batches, fine-tuning | `spec.md` Affected Route Inventory, `contracts/runtime-model-source.md` Required Consumers, `tasks.md` T032-T033 | Covered after artifact-review revision |
| Assistants, threads, vector stores, responses | `spec.md` Affected Route Inventory, `plan.md` consumers include `resolveAssistantsUpstream` / `resolveOpenAIEndpointRouteForRequest`, `tasks.md` T032-T033 | Covered after artifact-review revision |
| Image edits/variations, OCR, videos, containers, RAG | `spec.md` Affected Route Inventory, `tasks.md` T032/T035-T036 | Covered after artifact-review revision |
| Anthropic batch pass-through | `spec.md` Affected Route Inventory, `contracts/runtime-model-source.md` Required Consumers, `tasks.md` T032 | Covered after artifact-review revision |
| Native `/v1/messages`, `/v1/messages/count_tokens`, Gemini `generateContent` / `streamGenerateContent` / `countTokens` | `spec.md` Affected Route Inventory, `contracts/runtime-model-source.md`, `tasks.md` T034 | Covered |
| `/model_group/info` reports router only | `spec.md` US4/FR-005/SC-004, `tasks.md` T008/T030 | Covered |
| Key/team UI can list DB names but does not make runtime work | `spec.md` Summary and Affected Route Inventory, `research.md` key/team UI evidence | Covered |
| Failing E2E/integration first | `spec.md` FR-011/SC-001..SC-003, `plan.md` Failing Tests, `tasks.md` Phase 1 | Covered |
| Source-of-truth merge rule, wildcard, duplicate names | `spec.md` Merge Rule/FR-006/FR-007, `data-model.md` Duplicate Names/Wildcards, `tasks.md` T009/T012/T019-T020 | Covered |
| Runtime router/listing/discovery/provider resolution consume same source | `spec.md` FR-010, `contracts/runtime-model-source.md`, `tasks.md` T014-T030 | Covered |
| Edits/deletes hot-reload or explicit restart-required UX/API | `spec.md` US5/FR-009/SC-006, `data-model.md` Refresh Semantics, `tasks.md` T011/T037-T042 | Covered |

## Cross-Artifact Consistency

- `spec.md` defines the owner-facing behavior and merge rule.
- `plan.md` maps the implementation to one runtime model source and RED tests first.
- `data-model.md` defines DB row conversion and snapshot semantics.
- `contracts/runtime-model-source.md` defines expected consumers and error contract.
- `tasks.md` keeps all implementation tasks unchecked and blocked by Linear `In Progress`.
- `quickstart.md` contains only future implementation verification commands.

## Out-of-Scope Challenge

- Schema replacement is excluded because existing `ProxyModelTable` already has model name, params, and info fields needed for runtime conversion.
- Provider implementation changes are excluded because the bug is source-of-truth/routing, not provider protocol behavior.
- Distributed hot reload is excluded because the current service process can solve local runtime state first; cross-process invalidation can be follow-up if deployment topology requires it.
- Real provider network calls are excluded because mocked upstreams prove routing correctness without external flake or secrets.

## Bug-Fix Pipeline Alignment

HO-1285 is Bug-labeled and has a concrete root cause. Because live Linear state is `Waiting`, this artifact-review revision can only update SpecKit docs; RED tests still wait for Linear `In Progress`. The planned implementation still follows `bug-fix-pipeline` discipline:

1. Root cause is documented from repo evidence.
2. First implementation phase is RED E2E/integration tests.
3. Fix phase is limited to making those tests pass.
4. Review/CI gates remain required before `Waiting Merge`.

## E2E Coverage Map

- PR layer: TianjiLLM runtime routing and model management.
- Runtime boundary: HTTP route -> auth/middleware -> runtime model source -> router/provider resolver -> mocked upstream.
- Test commands: targeted `go test` commands in `quickstart.md`.
- CI evidence: full PR workflow after implementation.
- Gap/follow-up: multi-process hot reload is not covered unless the current deployment runs multiple TianjiLLM processes sharing one DB; that can be a follow-up after single-process runtime source is fixed.

## Implementation Route Audit

- `/v1/models` and bare `/models`: covered by `ListModels` reading `runtimeModelList`.
- `/v1/chat/completions` and bare chat route: covered by `resolveProviderRoute` using `runtimeRouter`, which preserves existing router settings through `Router.WithModels`.
- `/v1/completions`, `/v1/embeddings`, `/v1/images/generations`, audio, moderations: covered by `resolveProviderFromConfigRouteWithContext` and merged `findModelConfig`.
- `/v1/rerank`, files, batches, fine-tuning, and shared pass-through helpers: covered by `resolveProviderBaseURLWithContext` using merged explicit lookup and merged OpenAI-compatible fallback.
- Assistants, threads, vector stores, responses, image edits/variations, OCR, videos, containers, RAG, and OpenAI endpoint fallbacks: covered by merged `resolveAssistantsUpstream`, `resolveOpenAIEndpointRouteForRequest`, and `resolveOpenAIEndpointFallbackRoute`.
- Anthropic native `/v1/messages`, `/v1/messages/count_tokens`, Anthropic batch pass-through, and Gemini `/v1beta/models/{model}:...`: covered by `resolveAllNativeUpstreams` using `runtimeModelList`.
- `/model_group/info`: covered by `ModelGroupInfo` using merged `runtimeRouter`.
- `/model/new|update|delete` and UI create/update/delete: runtime reads DB rows per request, so committed DB mutations become visible without process restart or a separate in-memory refresh hook.
- Config/admin reporting endpoints that intentionally return raw config (`/config`, `/routes`, dashboard counts) remain config surfaces, not runtime model resolution paths.

## Final Scope Confirmation

HO-1285 Todo planning is internally consistent and does not need owner input. Move to `Waiting` after docs-only diff passes, draft PR exists, and the thread is notified with Linear and PR links.
