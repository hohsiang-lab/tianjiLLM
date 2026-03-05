# Research: OAuth Token 智能限流

**Branch**: `079-oauth-token-throttle` | **Date**: 2026-03-05

## No External Research Required

This feature operates entirely within existing internal packages. No new
libraries, external APIs, or unfamiliar patterns are introduced.

### Decision Log

| Decision | Rationale | Alternatives Rejected |
|----------|-----------|----------------------|
| Modify `selectUpstream` rather than middleware | Throttle logic is token-selection, not request-level. Middleware would add latency to all requests, not just Anthropic. | Middleware approach: adds unnecessary overhead to non-Anthropic requests |
| Method on `*Handlers` (not package function) | Needs access to `h.RateLimitStore` and `h.Config.RatelimitAlertThreshold`. Making it a method avoids passing these as params. | Package function with extra params: uglier signature, no benefit |
| Deduplicate by API key in selection | `resolveAllNativeUpstreams` returns one entry per model config. Multiple configs can share the same OAuth token. Without dedup, same token checked N times. | No dedup: wasted iterations, confusing round-robin distribution |
| Restore reverted code (commit eec5ca2) | `CheckAndAlertOAuth` and `sendOAuthAlertIfNotCooling` were well-designed, tested, and only reverted because the feature wasn't ready. Now it is. | Rewrite from scratch: unnecessary, original code is correct |
| Default threshold 0.8 when config is 0 | Aligns with user requirement (80%). The existing `NewDiscordRateLimitAlerter` uses 0.2 as default for legacy alerts (20% remaining), but for utilization-based checks, 0.8 (80% used) is the correct default. | Use same 0.2 default: wrong semantics (0.2 means 20% utilization, not 80%) |
| `allTokensThrottledError` as custom error type | Carries `resetAt` time needed for `Retry-After` header. Using sentinel error would lose this data. | `fmt.Errorf`: loses structured reset time data |
