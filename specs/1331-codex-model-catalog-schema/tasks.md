# Tasks: HO-1331 Codex model catalog schema

## Phase 1 - RED Coverage

- [X] T001 Re-read Linear HO-1331, this SpecKit package, and current `ListModels` / runtime model source before implementation.
- [X] T002 Add failing handler test proving `GET /models` currently cannot decode as Codex `ModelsResponse` because `models` is missing.
- [X] T003 Add regression test proving existing OpenAI-compatible `object:"list"` + `data[]` behavior remains required.
- [X] T004 Add DB-managed runtime model test asserting the same DB model appears in both `data[]` and `models[]`.
- [X] T005 Add hidden alias test asserting hidden aliases are excluded from both response shapes.
- [X] T006 Add converter defaults test for required Codex `ModelInfo` fields.

## Phase 2 - Response Shape Refactor

- [X] T007 Introduce explicit Tianji response structs for OpenAI model entries, Codex model entries, and combined model-list response.
- [X] T008 Extract filtered runtime model list helper shared by OpenAI-compatible and Codex-compatible builders.
- [X] T009 Keep `ListModels` route behavior identical for `/models` and `/v1/models`.

## Phase 3 - Codex Catalog Converter

- [X] T010 Implement `config.ModelConfig` to Codex model-info converter.
- [X] T011 Populate required Codex fields with explicit Tianji defaults.
- [X] T012 Derive context/truncation values from `ModelInfo.MaxInputTokens`, `MaxTokens`, or safe fallback.
- [X] T013 Use stable priority ordering from filtered runtime model list.
- [X] T014 Keep unsupported Codex-only fields explicit as empty/nil/default values.

## Phase 4 - Safety and No-Drift Guardrails

- [X] T015 Ensure converter never emits API keys, subscription credential IDs, bearer tokens, refresh tokens, raw `tianji_params`, or raw `model_info`.
- [X] T016 Ensure DB-managed `openai/*` catalog entry uses runtime alias as `slug`, not upstream-resolved `openai/gpt-5.5`.
- [X] T017 Ensure OpenAI-compatible `data[]` and Codex `models[]` are built from the same filtered model slice.
- [X] T018 Preserve existing hidden alias filtering behavior.

## Phase 5 - Verification

- [X] T019 Run `go test ./internal/proxy/handler -run 'ListModels|CodexModel' -count=1`.
- [X] T020 Run `go test ./internal/proxy/handler -run 'RuntimeModelSource|ListModels' -count=1`.
- [X] T021 Run broader package tests if touched helpers are shared beyond handler list models.
- [X] T022 Run `git diff --check`.
- [X] T023 Verify PR diff contains only HO-1331 docs during Todo phase.
- [ ] T024 After implementation/deploy, live verify true Codex model refresh no longer logs `missing field models`.
- [ ] T025 Record verification evidence in PR and Linear/thread without secrets.
