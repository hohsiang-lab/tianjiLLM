# Implementation Plan: Composite Score V2

**Branch**: `491-composite-score-v2` | **Date**: 2026-04-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/491-composite-score-v2/spec.md`

## Summary

Replace the quadratic composite score formula (`util5h² + util7d² × 2.0`) with a max-normalized formula (`max(u5h/θ_5h, u7d/θ_7d, u7ds/θ_7d)`) that includes the 7d Sonnet window. Add a hard 7d_s gate to `selectUpstreamWithThrottle`. Update UI score display and color thresholds.

## Technical Context

**Language/Version**: Go 1.26  
**Primary Dependencies**: No new dependencies. Changes are internal to existing packages: `callback`, `proxy/handler`, `ui`.  
**Storage**: No schema changes. `Unified7dSonnetUtilization` is already stored in `AnthropicOAuthRateLimitState` and persisted to DB via `RateLimitFlusher`.  
**Testing**: `go test` + `testify` (assert/require)  
**Target Platform**: Linux server (Docker)  
**Project Type**: Single Go binary  
**Performance Goals**: No regression in token selection latency (hot path: 1 `max()` call replaces 2 multiplies + 1 add)  
**Constraints**: Zero new allocations on the selection hot path  
**Scale/Scope**: 6 files changed (3 source + 3 test), ~50 lines net

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | This is a Go-native optimization with no Python equivalent. CompositeScore was added in the Go port and has no counterpart in Python TianjiLLM. |
| II. Feature Parity | N/A | Same as above — no Python behavior to replicate. |
| III. Research Before Build | PASS | No new libraries needed. Formula derived from mathematical analysis of Anthropic's rate limit windows (documented in Obsidian: `tianji-token-usage-maximization.md`). |
| IV. Failing-Tests-First | PASS | Failing tests listed below in § Failing Tests. |
| V. Go Best Practices | PASS | No new abstractions. Function signature changes are minimal. max() is stdlib (`builtin`). |
| VI. No Stale Knowledge | PASS | No external library decisions. All changes use existing project types and patterns. |
| VII. sqlc-First DB Access | N/A | No DB schema or query changes. |

## Project Structure

### Documentation (this feature)

```text
specs/491-composite-score-v2/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 (minimal — no external research needed)
├── data-model.md        # Phase 1 (minimal — no new entities)
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output
```

### Source Code (files to modify)

```text
internal/
├── callback/
│   └── composite.go              # Formula change: 3-arg max-normalized
├── proxy/handler/
│   ├── native_upstream.go        # 7d_s gate + pass util7ds to CompositeScore
│   └── native_upstream_test.go   # New + updated tests
└── ui/
    ├── handler_claude_code.go        # Pass sonnetUsage to CompositeScore
    ├── handler_claude_code_test.go   # Updated expectations
    └── pages/
        ├── usage_claude_code_templ.go  # Color threshold update
        └── usage_claude_code_test.go   # Updated color boundary tests
```

**Structure Decision**: All changes are within existing files. No new files created.

## Failing Tests

### User Story 1 Tests — Sonnet-aware selection

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCompositeScore_MaxNormalized` | `native_upstream_test.go` | `CompositeScore(0.06, 0.69, 0.48, 0.8) == max(0.06/0.8, 0.69/0.9, 0.48/0.9) == 0.767` | FR-002 formula |
| `TestCompositeScore_MaxNormalized_AllZero` | `native_upstream_test.go` | `CompositeScore(0, 0, 0, 0.8) == 0` | FR-002 zero case |
| `TestCompositeScore_MaxNormalized_SonnetDominates` | `native_upstream_test.go` | `CompositeScore(0.1, 0.3, 0.85, 0.8) == 0.85/0.9 ≈ 0.944` | AS-1.1: Sonnet bottleneck detected |
| `TestLowestUtilSelect_PreferLowSonnet` | `native_upstream_test.go` | Token with u7ds=30% selected over token with u7ds=85% (when u7d similar) | AS-1.1 |
| `TestLowestUtilSelect_7dAllStillDominates` | `native_upstream_test.go` | When u7d > u7ds for both tokens, selection unchanged from old behavior | Regression: no change when 7d_all is bottleneck |

### User Story 2 Tests — 7d_s gate

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestSelectUpstream_7dSonnetGate_Skips` | `native_upstream_test.go` | Token with u7ds=0.92 is filtered out; other token selected | AS-2.1 |
| `TestSelectUpstream_7dSonnetGate_AllThrottled` | `native_upstream_test.go` | Both tokens u7ds >= 0.90 → `allTokensThrottledError` with 7d_s reset time | AS-2.1 + FR-006 |
| `TestSelectUpstream_7dSonnetGate_SentinelSkipped` | `native_upstream_test.go` | Token with u7ds=-1 (sentinel) is NOT filtered by 7d_s gate | Edge case: sentinel |
| `TestSelectUpstream_AllowedWarning_Bypasses7dSonnetGate` | `native_upstream_test.go` | Token with u7ds=0.95 + status=allowed_warning passes gate | Edge case: overage bypass |

### User Story 3 Tests — UI score display

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCcCompositeScoreStyle_Boundaries` (updated) | `usage_claude_code_test.go` | Green < 0.5, orange < 0.8, red >= 0.8 | FR-007 |
| `TestLoadClaudeCodeData_CompositeScore` (updated) | `handler_claude_code_test.go` | Score uses max-normalized with 3 windows | FR-009 |

### User Story 4 Tests — Missing data graceful degradation

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCompositeScore_SonnetSentinel` | `native_upstream_test.go` | `CompositeScore(0.3, 0.4, -1, 0.8)` → u7ds treated as 0, score = max(0.375, 0.444, 0) = 0.444 | AS-4.1 |
| `TestCompositeScore_AllSentinel` | `native_upstream_test.go` | All inputs <= 0 → score = 0 | AS-4.2 |
| `TestLowestUtilSelect_Util7dSonnetSentinel` | `native_upstream_test.go` | Token with u7ds=-1 beats token with u7ds=0.5 (all else equal) | AS-4.1 in context |
| `TestLoadClaudeCodeData_CompositeScore_OneNilField` (updated) | `handler_claude_code_test.go` | Nil sonnetUsage → treated as 0 in score | AS-4.1 UI path |

### Existing tests to update (expectations change)

| Test Function | File | Change |
|---------------|------|--------|
| `TestCompositeScore_Formula` | `native_upstream_test.go` | Replace quadratic assertions with max-normalized assertions |
| `TestLowestUtilizationSelect_Equal7dLower5hWins` | `native_upstream_test.go` | Update composite value comments + guard assertion |
| `TestSelectUpstreamThrottle_UsesLowestUtilization` | `native_upstream_test.go` | Add u7ds field to state setup |
| `TestLowestUtilizationSelect_Util7dSentinel` | `native_upstream_test.go` | Update expected composite values in comments |

### Verification Command

```bash
go test ./internal/callback/... ./internal/proxy/handler/... ./internal/ui/... -run "TestCompositeScore|TestLowestUtil|TestSelectUpstream|TestCcCompositeScoreStyle|TestLoadClaudeCodeData" -v
```

## Complexity Tracking

No constitution violations. All changes are within existing files with no new abstractions, packages, or dependencies.
