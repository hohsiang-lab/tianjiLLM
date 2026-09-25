# Research: HO-1343 shared sticky routing strategy

## Repo Reality

### Claude sticky

- `internal/proxy/handler/native_upstream.go` contains `stickyEntry`, `stickyTrackKey`, `stickySelect`, `sticky5hReset`, and `lowestUtilizationSelect`.
- Claude sticky currently stores raw API key in `stickyEntry.APIKey`; the refactor should move shared identity to stable non-secret IDs while keeping provider logic able to map back to candidates.
- Claude route classes are `anthropic:sonnet` and `anthropic:all`.
- Claude re-evaluates sticky selection when stored 5h reset timestamp differs from live `RateLimitStore` state.
- Claude scoring prefers earliest known 7d reset; unknown reset is treated as farthest.

### OpenAI subscription sticky

- `internal/proxy/handler/openai_subscription_routing.go` contains `roundRobinOpenAISubscriptionSelect`, `stickyOpenAISubscriptionSelect`, `lowestUtilizationOpenAISubscriptionSelect`, and `openAISubscriptionRouteKey`.
- OpenAI subscription sticky stores credential ID per org/model route key.
- Candidate filtering already excludes rate-limited credentials via `openAISubscriptionRateLimitState(...).Gated(now)`.
- `openAISubscriptionRouteKey` scopes sticky by org ID from `middleware.ContextKeyOrgID`, with deterministic master/no-org fallback.

### OpenAI response-header quota

- `internal/callback/openai_quota_store.go` defines `OpenAIQuotaState`, normalization, gate checks, and utilization.
- `internal/proxy/handler/openai_subscription_ratelimit.go` parses `x-ratelimit-*` headers and mock quota utilization.
- This state is response-header based and should remain separate from Codex usage snapshots.

### Codex usage snapshots

- `internal/proxy/handler/openai_subscription_codex_usage.go` defines `OpenAISubscriptionCodexUsageCache`, `getFresh`, `getBackoff`, `putSuccess`, and failure/backoff behavior.
- Fresh cache entries require `Status == "fresh"` and `ExpiresAt.After(now)`.
- Fetching happens through `OpenAISubscriptionCodexUsageSnapshot`, which calls `chatgptcodex.UsageFetcher.Fetch` after resolving a credential.
- Routing should read cache but not call the snapshot fetch method.

## External Contract Notes

- OpenAI-compatible response-header rate-limit state already exists in repo and is covered by HO-1179-era tests.
- Codex `GET /backend-api/wham/usage` is treated as an OpenAI subscription/Codex backend status source, not as an OpenAI-compatible proxy request dependency.
- Because `wham/usage` can return auth/rate-limit/backoff errors and requires subscription bearer token material, it must stay outside normal request routing.

## Design Decisions

### Decision 1: Shared core handles mechanics, policies handle semantics

Status: Accepted.

Reasoning: Claude and Codex share the mechanics of track state, reuse, unavailable reselect, and deterministic fallback. They do not share quota semantics. A provider-neutral core reduces duplication without forcing Anthropic 5h/7d/sonnet logic onto Codex.

### Decision 2: Codex snapshots are cache-only in routing

Status: Accepted.

Reasoning: HO-1340 intentionally added usage snapshots as UI/status-oriented cache data. Request routing should use fresh cached snapshots opportunistically and keep missing/stale/backoff snapshots as deterministic fallback, not a blocking network dependency.

### Decision 3: Candidate IDs must be non-secret

Status: Accepted.

Reasoning: The current Claude sticky entry stores API key material in memory. The shared core should not normalize that pattern. It should store stable IDs only and leave provider adapters to resolve candidates inside the request scope.

### Decision 4: Keep implementation under handler first

Status: Accepted for initial implementation.

Reasoning: Both current sticky implementations and their tests are handler-local. A handler-local core minimizes cross-package API churn. A later move to a lower-level package can happen only if the first refactor proves clean.

## Alternatives Rejected

- **One unified quota model for Claude and Codex**: rejected because Anthropic unified 5h/7d/7d_sonnet headers and Codex usage snapshots do not share source, freshness, or gate semantics.
- **Fetch Codex usage before every Codex request**: rejected because it adds latency and upstream rate-limit/auth failure exposure to the proxy hot path.
- **Leave duplicate sticky loops in place and only add Codex snapshot logic**: rejected because it deepens the duplicated handler logic that this issue is explicitly meant to remove.
- **Persist sticky state in DB**: rejected as out of scope; current behavior is in-memory and this issue is a refactor plus cached snapshot scoring.
