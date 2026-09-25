# Implementation Plan: HO-2368 Codex session sticky reuse across primary reset

**Branch**: `HO-2368-tianji-codex-session-sticky-should-not-reselect` | **Date**: 2026-07-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/HO-2368-tianji-codex-session-sticky-should-not-reselect/spec.md`

## Summary

Change Codex session-aware sticky reuse so a primary reset timestamp change alone does not force re-selection for `:codex-session:` route keys. Keep existing non-session org/model sticky behavior, candidate filtering, usage gates, and raw-session-id redaction unchanged.

## Implementation Approach

Implementation detail: clarified. Add a small session-aware route-key classification in the existing OpenAI subscription routing handler, then branch `codexStickyCanReuseWithTrack` so known/selectable `:codex-session:` candidates reuse without comparing `PrimaryResetAt`. Leave unknown metadata handling, `Selectable=false` handling, candidate filtering, fallback route primary-reset behavior, and log redaction on the existing paths.

## Repo Reality Notes

- Repo resolved from Linear project registry: `hohsiang-lab/tianjiLLM`.
- Code confirmation used `rg` against `internal/proxy/handler/openai_subscription_routing.go` and `internal/proxy/handler/openai_subscription_routing_test.go`.
- Owning code: `codexStickyCanReuseWithTrack`, `openAISubscriptionCodexSessionRouteKey`, and `safeOpenAISubscriptionTrackLogValue` in `internal/proxy/handler/openai_subscription_routing.go`.
- Existing tests already cover session sticky reuse, over-gate reselect, missing session fallback, log redaction, missing/partial snapshots, and fallback primary-reset re-evaluation in `internal/proxy/handler/openai_subscription_routing_test.go`.
- No UI route, DB schema, sqlc query, config surface, or route-key shape change is part of this issue.

## Technical Context

**Language/Version**: Go; repository CI uses `actions/setup-go@v5` with Go `1.26` in `.github/workflows/ci.yml`.
**Primary Dependencies**: Existing standard library packages and existing project code only; no new dependency. Route-key marker detection can use the existing `strings` package, whose official docs define `func Contains(s, substr string) bool`.
**Storage**: N/A; no schema, sqlc, database, or cache persistence change.
**Testing**: `go test` with existing `testify` assertions in `internal/proxy/handler/openai_subscription_routing_test.go`.
**Target Platform**: TianjiLLM Go proxy routing path.
**Project Type**: Single Go web/proxy application.
**Performance Goals**: Preserve hot-path routing complexity; session route detection is constant substring/key-shape logic and must not call Codex usage fetchers.
**Constraints**: No production work in Todo. Implementation must be failing-tests-first. Do not change rendezvous hashing, route-key construction, UI, config, DB schema, or non-Codex routing.
**Scale/Scope**: One backend routing policy helper plus focused handler tests.

## Constitution Check

- **Python-first reference**: No Python TianjiLLM equivalent for this Go-only Codex sticky-session follow-up was identified in issue scope. This is a Go routing policy correction observed from HO-2360 live verification, not a Python parity feature.
- **Feature parity**: No public API/config behavior changes; route/session behavior remains TianjiLLM Go proxy internal policy.
- **Research before build / no stale knowledge**: No new library is selected. Official Go docs for `strings.Contains` were checked at `https://pkg.go.dev/strings#Contains`; repo code already imports and uses `strings` in the owning file.
- **Failing-tests-first development**: Pass. This plan names concrete failing tests before implementation tasks.
- **Go idioms**: Pass. Scope is a small helper/branch in existing handler code, no new abstraction unless it clarifies route-key classification.
- **sqlc-first DB access**: N/A. No SQL/query/schema changes.

## Project Structure

### Documentation (this feature)

```text
specs/HO-2368-tianji-codex-session-sticky-should-not-reselect/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── codex-session-sticky-policy.md
├── checklists/
│   └── requirements.md
├── tasks.md
└── analyze.md
```

### Source Code (repository root)

```text
internal/proxy/handler/
├── openai_subscription_routing.go
└── openai_subscription_routing_test.go

.github/workflows/
└── ci.yml
```

**Structure Decision**: Implement in the existing OpenAI subscription routing handler and focused test file. No new packages, migrations, UI files, or generated code.

## Failing Tests

### User Story 1 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexSessionSticky_PrimaryResetChangedDoesNotReselectWhenStillSelectable` | `internal/proxy/handler/openai_subscription_routing_test.go` | After the selected session credential's `PrimaryResetAt` changes but `Selectable=true`, the second selection equals the first selected credential. | US1 AS-1 |
| `TestOpenAISubscriptionRouting_StickyCodexPrimaryWindowResetReevaluates` | `internal/proxy/handler/openai_subscription_routing_test.go` | Existing non-session/fallback behavior still reselects on primary reset change or equivalent fallback coverage remains. | US1 AS-2 |

### User Story 2 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexSessionSticky_StillReselectsWhenPrimaryGateExceeded` | `internal/proxy/handler/openai_subscription_routing_test.go` | A session sticky to `cred-a` reselects `cred-b` when `cred-a` crosses the primary gate and becomes non-selectable. | US2 AS-1 |
| `TestCodexSessionSticky_StillReselectsWhenSecondaryGateExceeded` | `internal/proxy/handler/openai_subscription_routing_test.go` | A session sticky to `cred-a` reselects `cred-b` when `cred-a` crosses the weekly/secondary gate and becomes non-selectable. | US2 AS-2 |
| `TestOpenAISubscriptionRouting_StickyCodexMissingCurrentSnapshotReevaluatesToKnownCandidate` | `internal/proxy/handler/openai_subscription_routing_test.go` | Existing unknown-current-snapshot behavior remains conservative. | US2 AS-3 |

### User Story 3 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexSessionSticky_PrimaryResetChangedDoesNotLogReselectReasonWhenReused` | `internal/proxy/handler/openai_subscription_routing_test.go` | Session-aware primary-reset-only reuse does not emit `reason=primary_reset_changed` for a reselect; if reuse is logged, it uses a reuse-specific reason. | US3 AS-1 |
| `TestCodexSessionSticky_LogsDoNotExposeRawSessionID` | `internal/proxy/handler/openai_subscription_routing_test.go` | Existing redaction test continues to pass and raw session id is absent. | US3 AS-2 |

### Verification Command

```bash
go test ./internal/proxy/handler -run 'Codex.*Sticky|OpenAISubscriptionRouting_StickyCodex|CodexSession' -count=1
```

## Complexity Tracking

No constitution violations or extra complexity are planned.

## Phase 0 Research

See [research.md](./research.md). Decision: keep this as a small policy split in existing routing code, not a shared-strategy refactor.

## Phase 1 Design

See [data-model.md](./data-model.md), [contracts/codex-session-sticky-policy.md](./contracts/codex-session-sticky-policy.md), and [quickstart.md](./quickstart.md).
