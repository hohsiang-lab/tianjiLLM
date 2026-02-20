# Requirements Checklist: Phase 5 Migration Gap Closure

**Purpose**: Quality gate for the Phase 5 spec — ensures completeness, accuracy, and actionability.
**Created**: 2026-02-18
**Feature**: [spec.md](../spec.md)

## Spec Completeness

- [x] CHK001 All 8 user stories (US-1 through US-8) are documented with priority assignment
- [x] CHK002 Each user story has "Why this priority" justification
- [x] CHK003 Each user story has "Independent Test" description
- [x] CHK004 All P0 user stories (US-1, US-2, US-3) have 2+ acceptance scenarios with Given/When/Then
- [x] CHK005 All P1 user stories (US-4, US-5, US-6) have 2+ acceptance scenarios with Given/When/Then
- [x] CHK006 All P2 user stories (US-7, US-8) have 2+ acceptance scenarios with Given/When/Then
- [x] CHK007 Edge cases section lists 5+ cross-feature interaction scenarios
- [x] CHK008 Out of Scope section explicitly lists excluded features with rationale

## Accuracy Verification

- [x] CHK009 Verify US-1: No MCP support exists in Go codebase (no `mcp` package)
- [x] CHK010 Verify US-2: No search provider exists in Go codebase (no brave/tavily/searxng packages)
- [x] CHK011 Verify US-3: `images/edits` is wired (server.go:154) but `images/variations` is missing from routes
- [x] CHK012 Verify US-4: Prompt routes ARE wired (server.go:361-370) but chat flow has no template resolution
- [x] CHK013 Verify US-5: Go has 36 providers in `internal/provider/`; listed ~20 are genuinely missing
- [x] CHK014 Verify US-5: `ollama`, `vllm`, `lm_studio` are covered via `openaicompat` — correctly noted in spec
- [x] CHK015 Verify US-6: No discovery endpoints exist in Go codebase
- [x] CHK016 Verify US-7: Phase 4 tasks T082-T115 are all unchecked (34 pending plugins)

## Requirements Quality

- [x] CHK017 FR-001 through FR-010 are all testable (no vague language)
- [x] CHK018 Success criteria SC-001 through SC-009 are measurable
- [x] CHK019 No implementation details leak into user stories (no framework/language/API mentions in acceptance scenarios)
- [x] CHK020 Key Entities section defines all new domain concepts introduced

## Actionability

- [x] CHK021 Each user story can be independently implemented and tested
- [x] CHK022 Priority ordering enables incremental delivery (P0 → P1 → P2)
- [x] CHK023 Spec is sufficient for a developer to write a plan.md without additional research
- [x] CHK024 No circular dependencies between user stories
- [x] CHK025 Provider list in US-5 is concrete (specific names, not "various providers")
- [x] CHK026 Plugin list in US-7 is concrete (all 34 named, matching Phase 4 T082-T115)

## Consistency with Phase 4

- [x] CHK027 US-4 correctly reflects that prompts routes are already wired (not "missing")
- [x] CHK028 US-3 correctly reflects that images/edits exists (only variations missing)
- [x] CHK029 US-7 plugin list matches Phase 4 spec Category C (C1-C22) and Category E (E1-E12) exactly
- [x] CHK030 Out of Scope items match Phase 4 spec items D3 (A2A), D5 (OCR), D6 (Video), D7 (RAG), D9 (Container)

## Notes

- Check items off as completed: `[x]`
- CHK009-CHK016 require reading Go source code to verify claims
- Items are numbered sequentially for easy reference in reviews
