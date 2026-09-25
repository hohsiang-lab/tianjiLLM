# Data Model: Composite Score V2

**Date**: 2026-04-10  
**Feature**: 491-composite-score-v2

## No New Entities

This feature does not introduce new data entities, database tables, or schema changes.

## Modified Interfaces

### `CompositeScore` function signature

```
Before: CompositeScore(util5h, util7d float64) float64
After:  CompositeScore(util5h, util7d, util7dSonnet, threshold5h float64) float64
```

- `util7dSonnet`: 7d Sonnet utilization [0,1], sentinel -1 → caller normalizes to 0
- `threshold5h`: configurable 5h gate threshold (default 0.80)

### Score semantics

| Property | Before | After |
|----------|--------|-------|
| Formula | `u5h² + u7d² × 2.0` | `max(u5h/θ_5h, u7d/0.9, u7ds/0.9)` |
| Range | [0, 3.0] | [0, ~1.11] |
| Meaning | Weighted quadratic distance | Fraction of most-stressed gate |
| Sentinel | 4.0 | 2.0 |

## Existing Data Used (no changes)

- `AnthropicOAuthRateLimitState.Unified7dSonnetUtilization` — already parsed and stored
- `AnthropicOAuthRateLimitState.Unified7dSonnetReset` — already stored, used for reset tracking
- `config.ProxyConfig.RatelimitAlertThreshold` — existing configurable θ_5h
