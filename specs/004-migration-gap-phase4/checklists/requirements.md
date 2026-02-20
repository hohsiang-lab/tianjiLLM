# Requirements Checklist: Full Migration Gap Analysis (Phase 4)

**Purpose**: Quality gate for the migration gap analysis spec — ensures completeness, accuracy, and actionability.
**Created**: 2026-02-17
**Feature**: [spec.md](../spec.md)

## Spec Completeness

- [x] CHK001 All 6 categories (A-F) are documented with Python location and Go status
- [x] CHK002 Every gap item has a severity/priority assignment (P0-P3)
- [x] CHK003 All Category A (blocking) gaps have acceptance scenarios with Given/When/Then
- [x] CHK004 All Category B (router) gaps have acceptance scenarios
- [x] CHK005 Category C lists all missing guardrails with Python file locations
- [x] CHK006 Category D lists all missing proxy features with Python file locations
- [x] CHK007 Category E lists all missing callbacks with Python file locations
- [x] CHK008 Category F gaps have acceptance scenarios for P1/P2 items

## Accuracy Verification

- [x] CHK009 Verify A1: `internal/proxy/passthrough/` exists and `server.go` returns 501
- [x] CHK010 Verify A2: `handler/responses.go` returns 501
- [x] CHK011 Verify A3: `handler/sso.go` returns 501 on both endpoints
- [x] CHK012 Verify A4: Router only has ContextWindowFallbacks, no general fallback
- [x] CHK013 Verify A5: Config has `ModelGroupAlias` but `router.go` Route() ignores it
- [x] CHK014 Verify A6: No WebSocket support in Go codebase
- [x] CHK015 Verify B6: `strategy/tag.go` only has `hasAllTags`, missing match_any
- [x] CHK016 Verify F2: Cache package exists but chat handler has no cache integration

## Requirements Quality

- [x] CHK017 FR-001 through FR-008 are all testable (no vague language)
- [x] CHK018 Success criteria SC-001 through SC-005 are measurable
- [x] CHK019 No implementation details leak into the spec (technology-agnostic where possible)
- [x] CHK020 Edge cases section covers cross-feature interactions

## Actionability

- [x] CHK021 Each gap can be independently implemented and tested
- [x] CHK022 Priority ordering enables incremental migration (P0 first, then P1, etc.)
- [x] CHK023 Spec is sufficient for a developer to write a plan.md without additional research
- [x] CHK024 No circular dependencies between gap items

## Notes

- Check items off as completed: `[x]`
- CHK009-CHK016 require reading Go source code to verify claims
- Items are numbered sequentially for easy reference in reviews
