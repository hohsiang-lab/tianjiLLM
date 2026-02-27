# Tasks: OpenRouter Pricing Sync

**Feature**: 005-openrouter-pricing-sync
**Branch**: `005-openrouter-pricing-sync`
**Total Tasks**: 8
**Phases**: 3

---

## Phase 1: Setup

- [ ] T001 Revert PR #23 manual pricing entries in `internal/pricing/model_prices.json` (remove `gemini-2.5-pro-preview` and `gemini/gemini-2.5-pro-preview` entries added at end of file)

## Phase 2: Core Implementation (US1 + US2)

- [ ] T002 [US1] Add OpenRouter response types (`openRouterResponse`, `openRouterModel`, `openRouterPricing`) in `internal/pricing/sync.go`
- [ ] T003 [US1] Add `fetchOpenRouter(ctx context.Context, url string) ([]openRouterModel, error)` function in `internal/pricing/sync.go` — HTTP GET, parse JSON, return `data` array. Use existing `syncHTTPClient`.
- [ ] T004 [US1] [US2] Update `SyncFromUpstream` signature to accept `openRouterURL string` parameter. After LiteLLM entries are parsed, call `fetchOpenRouter`. Build `litellmNames` set from LiteLLM entries. For each OpenRouter model: parse prompt/completion as float64, skip if both unparseable, extract provider (split on first `/`), generate bare name, skip keys already in `litellmNames`, append to entries. Log warning if OpenRouter fetch fails but continue (graceful degradation). In `internal/pricing/sync.go`.
- [ ] T005 [US1] Update `handleSyncPricing` in `internal/ui/handler_models.go` to read `OPENROUTER_PRICING_URL` env var (default: `https://openrouter.ai/api/v1/models`) and pass it to `SyncFromUpstream`.

## Phase 3: Tests

- [ ] T006 [P] [US1] Add integration test `TestIntegration_OpenRouterSupplementsLiteLLM` in `internal/pricing/sync_integration_test.go` — mock both LiteLLM and OpenRouter servers, verify OpenRouter-only models appear in DB with correct pricing, verify LiteLLM models not overwritten.
- [ ] T007 [P] [US2] Add integration test `TestIntegration_OpenRouterDualKey` in `internal/pricing/sync_integration_test.go` — verify both `provider/model` and `model` (bare) keys are created for OpenRouter entries.
- [ ] T008 [P] [US3] Add integration test `TestIntegration_OpenRouterFailureGraceful` in `internal/pricing/sync_integration_test.go` — mock OpenRouter returning 500, verify LiteLLM sync still succeeds, verify warning logged.

---

## Dependencies

```
T001 → T002 → T003 → T004 → T005
T005 → T006, T007, T008 (parallel)
```

## Parallel Execution

T006, T007, T008 can run in parallel (separate test functions, no shared state).

## Implementation Strategy

**MVP**: T001–T005 (core implementation). Tests (T006–T008) follow.
**Suggested chunks**: Chunk 1 = T001–T005, Chunk 2 = T006–T008.
