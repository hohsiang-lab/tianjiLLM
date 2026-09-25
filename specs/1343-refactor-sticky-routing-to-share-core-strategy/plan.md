# Implementation Plan: HO-1343 shared sticky routing strategy and Codex usage snapshots

## Goal

Extract sticky routing into a shared provider-neutral strategy core and adapt Claude/Anthropic plus OpenAI subscription/Codex routing to it, while allowing Codex to use fresh cached usage snapshots without fetching usage in the proxy hot path.

## Constraints

- Todo phase is docs-only. No production code or test implementation in this PR.
- Preserve Claude sticky behavior under existing tests.
- Preserve OpenAI subscription fallback behavior when Codex usage snapshots are unavailable.
- Do not call `GET /backend-api/wham/usage` from request routing.
- Use stable non-secret candidate IDs for shared state.

## Current Repo Findings

- `internal/proxy/handler/native_upstream.go` owns Claude/Anthropic native upstream selection, including `stickyEntry`, `stickyTrackKey`, `stickySelect`, `sticky5hReset`, and `lowestUtilizationSelect`.
- `internal/proxy/handler/openai_subscription_routing.go` owns OpenAI subscription candidate selection, org-scoped sticky map, round-robin map, OpenAI quota state scoring, and route key generation.
- `internal/callback/ratelimit_store.go` stores both Anthropic OAuth rate-limit state and OpenAI quota state behind `RateLimitStore`.
- `internal/callback/openai_quota_store.go` defines `OpenAIQuotaState` gating and normalization for OpenAI response-header quotas.
- `internal/proxy/handler/openai_subscription_codex_usage.go` defines `OpenAISubscriptionCodexUsageCache`, fresh/backoff cache semantics, and the only current fetch path for Codex `wham/usage`.
- `internal/provider/chatgptcodex/usage.go` performs the actual `GET /backend-api/wham/usage` call; routing must not call this fetcher.

## Proposed Design

### 1. Shared sticky core

Add a provider-neutral sticky strategy helper, likely under `internal/proxy/handler` initially to minimize package churn, with:

- mutex-protected track map,
- stable selected ID storage,
- provider-supplied track key,
- candidate ID lookup,
- reuse check,
- policy-triggered re-evaluation,
- scoring and deterministic fallback,
- no provider imports and no token material.

The core should not know about Anthropic, OpenAI, Codex, OAuth, response headers, or DB fallback. It only coordinates reusable sticky mechanics.

### 2. Claude adapter

Move Claude-specific rules out of the core:

- candidate ID is `RateLimitCacheKey(apiKey)` or another stable non-secret wrapper;
- provider route track remains `anthropic:sonnet` and `anthropic:all`;
- availability and gates stay in the current throttle filter;
- reuse is denied when selected candidate is absent or its 5h reset differs from selection metadata;
- scoring still prefers the earliest known 7d reset, with unknown reset treated as farthest.

DB fallback for Anthropic rate-limit state may remain in the adapter/helper layer because it is provider-specific and intentionally bounded by the current 200ms timeout.

### 3. OpenAI subscription/Codex adapter

Keep OpenAI subscription routing keys and candidate filtering:

- candidate ID is credential ID;
- track key remains org/model scoped;
- disabled, lookup, refresh, and `OpenAIQuotaState.Gated(now)` filtering remains before sticky selection;
- current sticky/round-robin fallback remains deterministic.

When transport is `chatgpt_codex_backend` and `OpenAISubscriptionCodexUsageCache` contains fresh entries for candidates, the Codex policy may score using cached primary 5h and weekly windows:

- lower used percent wins when comparing same known window;
- primary 5h should trigger re-evaluation when the selected candidate's primary reset changes or status becomes exhausted/unavailable;
- weekly window can break ties or push traffic away from credentials near exhaustion;
- missing/stale/backoff snapshots are unknown, not errors.

OpenAI response-header `OpenAIQuotaState` remains valid for direct OpenAI HTTP routing and can still gate candidates before snapshot scoring.

### 4. No hot-path usage fetch

Request routing must only read `OpenAISubscriptionCodexUsageCache`. UI/manual/background refresh remains the source that populates the cache.

Tests should inject a fail-on-call or counting `CodexUsageFetcher` and prove routing through:

- `/v1/chat/completions`,
- `/v1/responses`,
- Responses WebSocket,

does not call the fetcher.

### 5. Test-first sequence

Start with shared core unit tests, then adapter parity tests, then endpoint no-fetch tests. The first failing tests should fail because no shared core/API exists or because Codex routing ignores cached usage snapshots.

## Project Structure

### Documentation in this PR

```text
specs/1343-refactor-sticky-routing-to-share-core-strategy/
├── spec.md
├── plan.md
├── data-model.md
├── research.md
├── tasks.md
├── quickstart.md
├── contracts/
│   └── shared-sticky-routing.md
├── checklists/
│   └── requirements.md
└── analyze.md
```

### Future implementation candidates

```text
internal/proxy/handler/
├── sticky_strategy.go                    # NEW: provider-neutral sticky core
├── sticky_strategy_test.go               # NEW: core unit tests
├── native_upstream.go                    # MODIFY: Claude adapter calls shared core
├── sticky_upstream_test.go               # MODIFY: parity/regression coverage if needed
├── openai_subscription_routing.go        # MODIFY: OpenAI/Codex adapter calls shared core
├── openai_subscription_routing_test.go   # MODIFY: cached snapshot selection coverage
├── openai_subscription_codex_usage.go    # READ ONLY or narrow helper only; fetch lifecycle stays outside routing
└── openai_subscription_endpoints_test.go # MODIFY: no hot-path usage fetch coverage
```

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Shared abstraction accidentally imports provider semantics | Keep provider policy callbacks outside the core and test the core with fake candidates. |
| Claude behavior changes during refactor | Run existing sticky/throttle tests and add adapter parity cases before modifying production code. |
| Codex routing starts fetching usage synchronously | Add fail-on-call fetcher tests on all proxy request paths. |
| Snapshot scoring becomes non-deterministic with partial data | Treat partial/stale/missing snapshots as unknown and fall back to existing deterministic order. |
| Token material leaks through shared candidate identity | Candidate ID contract forbids raw key/token values; tests assert selected IDs and errors are sanitized. |

## Verification Commands

```bash
go test ./internal/proxy/handler -run 'Sticky|OpenAISubscriptionRouting|CodexUsage|Responses|ChatGPTCodex' -count=1
go test ./internal/callback -run 'RateLimit|OpenAIQuota' -count=1
go test ./internal/proxy/handler -count=1
git diff --check origin/main...HEAD
```

## Rollout Notes

- Keep the Todo PR docs-only and draft.
- Implementation begins only after Linear moves out of Todo/Waiting into an implementation state.
- PR review must compare Claude selections before/after the refactor, not only check compile success.
