# Specification: HO-1343 shared sticky routing strategy and Codex usage snapshots

## Scope

Refactor Tianji sticky routing so Claude/Anthropic native routing and OpenAI subscription/Codex routing share one core sticky state machine, while keeping provider-specific quota policy adapters separate.

This issue owns backend routing strategy structure and tests only. It does not change UI pages, credential CRUD, token refresh lifecycle, Codex CLI behavior, OpenAI OAuth connect/callback, or the Codex usage snapshot fetch lifecycle introduced by HO-1340.

## Problem

Sticky routing currently exists in two handler-local shapes:

- Claude/Anthropic native upstream routing in `internal/proxy/handler/native_upstream.go` stores `stickyEntry{APIKey, Reset5hAt}` and re-evaluates sticky selection when the 5h reset timestamp changes.
- OpenAI subscription routing in `internal/proxy/handler/openai_subscription_routing.go` stores only `credentialID` per org/model route key and reuses it while it remains in the available candidate set.
- Codex usage snapshots from HO-1340 are cached in `internal/proxy/handler/openai_subscription_codex_usage.go`, but proxy request routing does not consume that cached snapshot data.

Root cause chain: Claude sticky and OpenAI subscription sticky each implement their own selection/reuse loop inside handler code; Codex now has cached 5h/weekly usage windows, but OpenAI subscription sticky has no policy hook to re-evaluate based on those windows without duplicating more provider-specific logic or fetching `wham/usage` in the proxy hot path.

## User Stories

### US1 - Shared sticky state machine preserves reuse and reselect semantics (P1)

As a Tianji maintainer, I want sticky selection reuse/reselect behavior implemented once, so Claude and Codex do not drift when we add policy rules.

**Independent Test**: focused unit tests run the shared strategy with fake candidates/policies and prove sticky reuse, unavailable candidate reselect, deterministic fallback, and policy-triggered re-evaluation.

### US2 - Claude sticky behavior remains unchanged (P1)

As an Anthropic OAuth operator, I want the existing 5h reset re-evaluation, 7d reset scoring, sonnet/all track split, and throttle gates to keep working after the refactor.

**Independent Test**: existing `sticky_upstream_test.go`, `model_aware_gate_test.go`, and targeted native upstream tests remain green, with added adapter tests proving the Claude policy returns the same selections as current behavior for known 5h/7d/sonnet scenarios.

### US3 - Codex routing can use fresh cached usage snapshots (P1)

As a Codex/OpenAI subscription operator, I want routing to prefer lower-usage credentials when fresh cached Codex usage snapshots exist, so quota-aware selection does not depend only on prior OpenAI response headers or credential affinity.

**Independent Test**: OpenAI subscription routing tests seed `OpenAISubscriptionCodexUsageCache` with fresh primary 5h and weekly window snapshots for multiple credentials and assert candidate selection/re-evaluation uses the cached snapshot data.

### US4 - Codex fallback remains deterministic without fresh snapshots (P1)

As a client using Codex through Tianji, I want routing to remain deterministic when snapshots are missing, stale, unavailable, or in backoff, so the request path does not become flaky or perform hidden network calls.

**Independent Test**: routing tests cover missing, expired, unavailable, and backoff cache entries, proving the route falls back to current sticky/round-robin behavior and does not call the usage fetcher.

### US5 - Proxy hot path never fetches Codex usage (P1)

As a Tianji operator, I want `/v1/chat/completions`, `/v1/responses`, and Responses WebSocket routing to avoid synchronous `GET /backend-api/wham/usage`, so request latency and upstream rate-limit exposure are not tied to status refresh.

**Independent Test**: endpoint tests inject a fail-on-call Codex usage fetcher and assert chat completions, Responses HTTP, and Responses WebSocket routing succeed or fail based on cached state only, with zero fetcher calls.

## Functional Requirements

- **FR-001**: System MUST introduce a shared sticky strategy core that owns track-key state, selected ID reuse, reselect when selected candidate is unavailable, policy-triggered re-evaluation, lock handling, and deterministic fallback.
- **FR-002**: Shared sticky core MUST identify candidates by stable non-secret IDs, never raw API keys, bearer tokens, refresh tokens, or encrypted credential values.
- **FR-003**: Shared sticky core MUST let provider policies define candidate availability, reuse allowance, re-evaluation triggers, and scoring/tie-break behavior.
- **FR-004**: Claude adapter MUST preserve current provider/model track behavior: `anthropic:sonnet` and `anthropic:all`.
- **FR-005**: Claude adapter MUST preserve current 5h reset re-evaluation behavior.
- **FR-006**: Claude adapter MUST preserve current 7d reset selection scoring and current unknown-token fallback semantics.
- **FR-007**: Claude adapter MUST preserve current gate behavior for `rejected`, 5h utilization, 7d utilization, and 7d_sonnet utilization.
- **FR-008**: OpenAI subscription adapter MUST preserve existing org-scoped route key behavior for `openai-subscription:<org-scope>:<model-or-route-class>`.
- **FR-009**: OpenAI subscription adapter MUST preserve existing disabled/rate-limited candidate exclusion and all-unusable error behavior.
- **FR-010**: Codex policy MUST read only `OpenAISubscriptionCodexUsageCache` entries that are fresh at routing time.
- **FR-011**: Codex policy MUST use cached primary 5h and weekly windows for quota-aware scoring/re-evaluation when both candidate and snapshot data are fresh enough to compare.
- **FR-012**: Codex policy MUST treat stale, unavailable, backoff, or missing snapshot data as unknown and fall back to current deterministic sticky/round-robin behavior.
- **FR-013**: Proxy routing MUST NOT call `chatgptcodex.UsageFetcher.Fetch` or `GET /backend-api/wham/usage` during `/v1/chat/completions`, `/v1/responses`, or Responses WebSocket handling.
- **FR-014**: UI/manual/background refresh may continue to populate `OpenAISubscriptionCodexUsageCache`; this issue MUST NOT move that fetch into request routing.
- **FR-015**: Refactor MUST keep existing API-key OpenAI routing behavior unchanged when no OpenAI subscription credential IDs are configured.
- **FR-016**: Tests MUST prove shared core behavior independently from provider adapters, and provider-specific tests MUST prove Claude unchanged behavior plus Codex cached-snapshot behavior.

## Non-Goals

- No UI changes for Codex usage cards or credential pages.
- No new Codex usage background scheduler unless separately approved.
- No synchronous `wham/usage` fetch in proxy request routing.
- No Codex CLI source changes.
- No OpenAI OAuth connect/callback/token refresh lifecycle changes.
- No persistence change for sticky state in this issue; in-memory routing state remains acceptable.
- No replacement of OpenAI response-header `OpenAIQuotaState`; cached Codex snapshots augment selection when fresh.

## Key Entities

- **Sticky Core Candidate**: Stable candidate ID plus provider-owned metadata used by policy adapters.
- **Sticky Track**: Shared state map entry keyed by provider/route policy, storing the selected stable candidate ID and provider-owned selection metadata.
- **Sticky Policy Adapter**: Provider-specific rules for availability, reuse, re-evaluation, score, and fallback ordering.
- **Claude Sticky Policy**: Adapter over `RateLimitStore`, Anthropic OAuth headers, 5h reset, 7d reset, and sonnet/all route class.
- **Codex Sticky Policy**: Adapter over OpenAI subscription candidates, `OpenAIQuotaState`, org/model route class, and fresh cached `OpenAISubscriptionCodexUsageResult`.
- **Fresh Codex Usage Snapshot**: Cache result with `Status == "fresh"` and `ExpiresAt` after routing time.

## Success Criteria

- **SC-001**: Shared sticky core unit tests cover reuse, unavailable candidate reselect, policy-triggered re-evaluation, and deterministic tie-break fallback.
- **SC-002**: Existing Claude sticky/throttle tests pass without behavior changes.
- **SC-003**: Codex routing tests show fresh cached primary 5h and weekly snapshots influence selection/re-evaluation.
- **SC-004**: Codex routing tests show missing/stale/backoff snapshots fall back to current deterministic behavior.
- **SC-005**: Endpoint tests prove no Codex usage fetcher call occurs in chat completions, Responses HTTP, or Responses WebSocket request routing.
- **SC-006**: No production log, error, map key, or artifact exposes raw credential/token material.

## Open Questions

None for Todo scope. Implementation may choose the exact package boundary after test-first exploration, but the shared core must stay provider-neutral and provider adapters must keep quota semantics separate.
