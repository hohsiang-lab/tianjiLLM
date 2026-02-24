# Tasks: Models Multi-Select for Create API Key

**Input**: Design documents from `specs/001-models-multiselect/`
**Prerequisites**: plan.md ✅, spec.md ✅, data-model.md ✅, contracts/README.md ✅, research.md ✅, quickstart.md ✅

**Scope**: Pure UI-layer change — `internal/ui/handler_keys.go`, `internal/ui/pages/keys.templ`, `internal/ui/pages/key_detail.templ`. No new packages, no DB schema change.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[US1/US2/US3]**: Which user story this task belongs to
- Exact file paths included in every task description

---

## Phase 1: Setup (Baseline Verification)

**Purpose**: Confirm the build is clean before making any changes.

- [ ] T001 Checkout branch `001-models-multiselect` and verify `go build ./...` passes from `/Users/n0rmanc/src/hohsiang-lab/tianjiLLM`
- [ ] T002 Run `templ generate` and confirm no errors before any `.templ` edits

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data-struct and helper changes that MUST be complete before any user story UI work can begin.

**⚠️ CRITICAL**: US1, US2, and US3 all depend on this phase.

- [ ] T003 Add `AvailableModels []string` field to `KeysPageData` struct in `internal/ui/pages/keys.templ`
- [ ] T004 [P] Add `AvailableModels []string` field to `KeyDetailData` struct in `internal/ui/pages/key_detail.templ`
- [ ] T005 Add private `loadAvailableModelNames(ctx context.Context) []string` helper to `internal/ui/handler_keys.go` — merges DB `ListProxyModels` + `h.Config.ModelList`, deduplicates by name
- [ ] T006 Call `h.loadAvailableModelNames(r.Context())` inside `loadKeysPageData` in `internal/ui/handler_keys.go` to populate `data.AvailableModels`
- [ ] T007 [P] Call `h.loadAvailableModelNames(r.Context())` in `handleKeyDetail` and `handleKeyEdit` handlers in `internal/ui/handler_keys.go` to populate `data.AvailableModels`

**Checkpoint**: `AvailableModels` is now populated on all relevant pages. User story UI work can begin.

---

## Phase 3: User Story 1 — Select Specific Models When Creating a Key (Priority: P1) 🎯 MVP

**Goal**: Replace the free-text comma-separated Models input in the Create API Key form with a structured multi-select checkbox list showing all configured proxy models. Selecting specific models creates a key restricted to exactly those models.

**Independent Test**: Open the Create API Key form → expand the Models dropdown → uncheck "All Models" → check `gpt-4` and `claude-3` → submit → verify the created key row shows `gpt-4, claude-3` and `VerificationToken.Models == ["gpt-4", "claude-3"]`.

### Tests for User Story 1

- [ ] T008 [P] [US1] Write unit test `TestLoadAvailableModelNames` in `internal/ui/handler_keys_test.go` covering: DB + config merge, deduplication, DB nil fallback to config, both empty returns `[]string{}`
- [ ] T009 [P] [US1] Extend `test/contract/key_management_test.go` with `TestCreateKeyWithSpecificModels`: POST `/ui/keys/create` with `all_models=0&models=gpt-4&models=claude-3`, verify DB `VerificationToken.Models == ["gpt-4", "claude-3"]`

### Implementation for User Story 1

- [ ] T010 [US1] Add `modelsMultiSelect(formPrefix string, available []string, selected []string)` templ component + `toggleAllModels` script + `updateModelsSummary` script to `internal/ui/pages/keys.templ` (same package as `key_detail.templ`, so callable from both)
- [ ] T011 [US1] Replace the free-text `<input name="models">` block in `createKeyForm` with `@modelsMultiSelect("create", data.AvailableModels, []string{})` in `internal/ui/pages/keys.templ`
- [ ] T012 [US1] Fix `handleKeyCreate` form parsing in `internal/ui/handler_keys.go`: replace `parseCSV(r.FormValue("models"))` with `r.Form["models"]` slice read, guarded by `all_models` sentinel value
- [ ] T013 [US1] Run `templ generate` from repo root to regenerate `internal/ui/pages/keys_templ.go`; run `go build ./...` to confirm compilation

**Checkpoint**: US1 fully functional and independently testable. Create API Key form shows model checkboxes; selected models are stored in DB.

---

## Phase 4: User Story 2 — Create a Key with No Model Restriction ("All Models") (Priority: P1)

**Goal**: The "All Models" checkbox in the multi-select must be visible and pre-checked by default. Selecting it (or submitting without any specific model) creates an unrestricted key (`Models = []`). Same multi-select must appear in the Edit Key Settings form with the current model selection pre-filled.

**Independent Test**: Open Create API Key form → leave "All Models" checked (default) → submit → verify `VerificationToken.Models == []`. Also: open an existing key's Edit Settings → change model selection → save → verify DB update.

### Tests for User Story 2

- [ ] T014 [P] [US2] Extend `test/contract/key_management_test.go` with `TestCreateKeyWithAllModels`: POST with `all_models=1`, verify `VerificationToken.Models == []`
- [ ] T015 [P] [US2] Extend `test/contract/key_management_test.go` with `TestCreateKeyNoModelCheckbox`: POST with `all_models=0` and no `models=` values, verify treated as unrestricted (`Models == []`)

### Implementation for User Story 2

- [ ] T016 [US2] Fix `handleKeyCreate` all_models branch in `internal/ui/handler_keys.go`: when `all_models == "1"`, set `models = nil` (unrestricted); ensure `r.ParseForm()` is called before reading `r.Form`
- [ ] T017 [US2] Replace the free-text models input in the edit settings form with `@modelsMultiSelect("edit", data.AvailableModels, data.Models)` in `internal/ui/pages/key_detail.templ`
- [ ] T018 [US2] Fix `handleKeyUpdate` form parsing in `internal/ui/handler_keys.go`: replace `parseCSV(r.FormValue("models"))` with `all_models` sentinel + `r.Form["models"]` slice; populate `data.AvailableModels` on the error-path re-render
- [ ] T019 [US2] Run `templ generate` from repo root to regenerate `internal/ui/pages/key_detail_templ.go`; run `go build ./...` to confirm compilation

**Checkpoint**: US1 and US2 both functional. Create and Edit forms show multi-select; "All Models" default works; edit form pre-fills current selection.

---

## Phase 5: User Story 3 — Empty Model List in Proxy Config (Priority: P2)

**Goal**: When the proxy has no configured models (`AvailableModels` is empty), the Create API Key form must still render without errors. Only the "All Models" option is shown; an unrestricted key can be created.

**Independent Test**: Access Create API Key form with `loadAvailableModelNames` returning `[]` → verify form renders with only "All Models" visible → submit → verify key created successfully with `Models == []`.

### Tests for User Story 3

- [ ] T020 [P] [US3] Add `TestLoadAvailableModelNamesEmpty` case in `internal/ui/handler_keys_test.go`: both DB and config empty → returns `[]string{}`
- [ ] T021 [P] [US3] Extend `test/contract/ui_test.go` with `TestCreateKeyFormEmptyModelList`: render Keys page with empty `AvailableModels` → assert only "All Models" option present, no individual model checkboxes

### Implementation for User Story 3

- [ ] T022 [US3] Verify `modelsMultiSelect` in `internal/ui/pages/keys.templ` handles `len(available) == 0` gracefully: hides the individual model list section entirely, shows only "All Models" checkbox (already gated by `if len(available) > 0` in plan Step 5 — confirm this guard is in place)

**Checkpoint**: All three user stories independently functional. Edge case of empty model list handled.

---

## Final Phase: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, accessibility attributes, and multi-select rendering assertions.

- [ ] T023 Add `aria-label="Select models for this API key"` to `<summary>` and `aria-live="polite"` to the summary `<span>` in `modelsMultiSelect` in `internal/ui/pages/keys.templ`
- [ ] T024 [P] Add `aria-label="Model: {model_name}"` to each individual model checkbox and `aria-label="All Models (unrestricted)"` to the All Models checkbox in `modelsMultiSelect` in `internal/ui/pages/keys.templ`
- [ ] T025 [P] Extend `test/contract/ui_test.go` with multi-select rendering assertions: Keys page includes `<details>` element, "All Models" checkbox, and model name checkboxes for each configured model
- [ ] T026 Run complete test suite (`go test ./...`) and confirm all tests pass; run `go vet ./...` for static analysis
- [ ] T027 Manual verification against `specs/001-models-multiselect/quickstart.md` checklist

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS all user story phases**
- **US1 (Phase 3)**: Depends on Phase 2 — can begin after Foundational completes
- **US2 (Phase 4)**: Depends on Phase 2 + T010/T011 (reuses `modelsMultiSelect` component from US1)
- **US3 (Phase 5)**: Depends on Phase 2 + T010 (verifies empty-list guard in component)
- **Polish (Final)**: Depends on all user story phases completing

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational — no dependencies on US2 or US3
- **US2 (P1)**: Can start after T010 (modelsMultiSelect component) — depends on the component built in US1 but is otherwise independent; the edit-form change (T017–T019) can proceed in parallel with US1
- **US3 (P2)**: Can start after T010 — only verifies the empty-list guard already in the component

### Within Each User Story

- Tests (T008, T009, T014, T015, T020, T021): Write and confirm they **FAIL** before implementation
- Data structs (T003, T004) before component (T010)
- Helper (T005, T006) before form parsing (T012, T016, T018)
- `templ generate` (T013, T019) after each `.templ` file edit batch

### Parallel Opportunities

- T003 and T004: Different files — run in parallel
- T005, T006, T007: T005 must complete before T006/T007; T006 and T007 can run in parallel
- T008 and T009: Both test files, no dependency — parallel
- T010 and T011: T010 must complete before T011
- T014 and T015: Different test cases — parallel
- T023 and T024: Same file but different attributes — sequential or one combined edit
- T025 and T026: Different activities — parallel

---

## Parallel Example: User Story 1

```bash
# After Phase 2 completes, launch US1 test stubs in parallel:
Task T008: "Write TestLoadAvailableModelNames in internal/ui/handler_keys_test.go"
Task T009: "Write TestCreateKeyWithSpecificModels in test/contract/key_management_test.go"

# Then implement:
Task T010: "Add modelsMultiSelect component to internal/ui/pages/keys.templ"
# T010 complete → unlock T011 and T017 in parallel:
Task T011: "Replace createKeyForm models input in internal/ui/pages/keys.templ"
Task T017: "Replace edit settings form models input in internal/ui/pages/key_detail.templ"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (verify baseline)
2. Complete Phase 2: Foundational (T003–T007 — CRITICAL)
3. Complete Phase 3: US1 (T008–T013)
4. **STOP and VALIDATE**: `go test ./...` passes; create a key with specific models via UI; confirm DB stores only those models
5. Demo / merge to main if ready

### Incremental Delivery

1. Phase 1 + Phase 2 → Foundation ready
2. Phase 3 (US1) → Create form works with specific model selection → **MVP deliverable**
3. Phase 4 (US2) → Edit form works; "All Models" default confirmed → **Full P1 scope**
4. Phase 5 (US3) → Empty model list edge case handled → **P2 scope complete**
5. Final Phase → Polish, accessibility, full test suite

### Parallel Team Strategy

With two developers after Phase 2 completes:

- **Developer A**: US1 (T008–T013) — `keys.templ` + `handleKeyCreate`
- **Developer B**: Edit form work from US2 (T017–T019) — `key_detail.templ` + `handleKeyUpdate`
- Both can work independently; merge after T010 is shared/committed

---

## Task Summary

| Phase | Tasks | Story |
|-------|-------|-------|
| Phase 1: Setup | T001–T002 | — |
| Phase 2: Foundational | T003–T007 | — |
| Phase 3: US1 (P1) | T008–T013 | US1 |
| Phase 4: US2 (P1) | T014–T019 | US2 |
| Phase 5: US3 (P2) | T020–T022 | US3 |
| Final: Polish | T023–T027 | — |
| **Total** | **27 tasks** | |

**Parallel opportunities**: 11 tasks marked `[P]`  
**MVP scope**: Phases 1–3 (13 tasks, US1 only)  
**Full scope**: All 27 tasks

---

## Notes

- `[P]` tasks modify different files with no dependency on incomplete tasks in their phase
- Each user story is independently completable and testable after Phase 2
- `templ generate` must run after **every batch of `.templ` edits** — do not skip
- `parseCSV` helper in `handler_keys.go` may be removed after all callers are updated (verify no other callers before deleting)
- No new packages, no DB migrations, no new SQL queries — scope is strictly `internal/ui/`
