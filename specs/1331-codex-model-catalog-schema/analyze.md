# Analyze: HO-1331 Codex model catalog schema

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
| --- | --- | --- | --- | --- |
| Codex refresh can decode `/models` | US1, FR-001, SC-001, SC-004 | Response Shape, Tests | T002, T007-T014, T019 | Covered |
| OpenAI-compatible clients keep `object/data` | US2, FR-003, SC-002, SC-005 | Response Shape, Tests | T003, T009, T019-T020 | Covered |
| Same runtime source for both shapes | US3, FR-004, FR-005, FR-007, SC-003 | Source-of-Truth Invariant | T004, T008, T017 | Covered |
| Hidden aliases stay hidden | US4, FR-006 | Hidden Alias Filter | T005, T018 | Covered |
| Explicit Codex defaults | US5, FR-008, FR-009, FR-010 | Codex Catalog Converter | T006, T010-T014 | Covered |
| No secret leakage | FR-011 | Safety and No-Drift Guardrails | T015-T016, T025 | Covered |
| Live model refresh no longer logs missing field | FR-013, SC-006 | Live Verification | T024-T025 | Covered |

## Repo Reality Alignment

- `internal/proxy/handler/handler.go` `ListModels` currently emits only `object` and `data`.
- `internal/proxy/server.go` maps both `/models` and `/v1/models` to `ListModels`.
- `internal/proxy/handler/runtime_model_source.go` already merges YAML and DB-managed model rows into `[]config.ModelConfig`.
- `internal/config/config.go` `ModelInfo` lacks Codex full `ModelInfo`, so a Tianji-owned converter is necessary.
- Existing `runtime_model_source_test.go` already contains the right handler test harness for DB-managed model list behavior.

## External Contract Alignment

- Codex source fetches provider-owned `/models`.
- Codex source decodes top-level `ModelsResponse.models`.
- Codex `ModelInfo` requires many fields beyond `slug`.
- OpenAI-compatible model listing shape must remain available for existing clients.

## Consistency Checks

- Todo phase remains docs-only.
- Tasks are unchecked because implementation has not started.
- Plan rejects a second static catalog.
- Plan rejects thin `slug`-only payload.
- Contract keeps both response shapes from one source.
- Live verification waits until implementation/deploy.

## Fatal Issues

0

## Critical Issues

0

## Owner Input Required

0 for Todo scope.
