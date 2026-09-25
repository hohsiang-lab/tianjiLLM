# Research: Composite Score V2

**Date**: 2026-04-10  
**Feature**: 491-composite-score-v2

## No External Research Required

This feature modifies an existing internal formula with no new library dependencies, no schema changes, and no new external integrations.

### Decision 1: Formula — max-normalized vs quadratic

- **Decision**: Replace `util5h² + util7d² × 2.0` with `max(u5h/θ_5h, u7d/θ_7d, u7ds/θ_7d)`
- **Rationale**: Mathematical derivation from token usage maximization formula shows that the system bottleneck is determined by the single most-stressed window (min of remaining capacities ↔ max of normalized utilizations). A sum-based score can mask a nearly-full window when other windows are low. The max function correctly identifies the bottleneck.
- **Alternatives considered**:
  - Weighted sum with 3 terms: Still masks individual window pressure. Requires tuning 3 weights with no principled basis.
  - Geometric mean: Better than arithmetic but still averages out extreme values.
  - Keep quadratic, add 7d_s term: `u5h² + u7d² × w7d + u7ds² × w7ds` — requires choosing two weight parameters with no clear justification.
- **Source**: Mathematical analysis documented in Obsidian (`200-Areas/Engineering/tianji-token-usage-maximization.md`).

### Decision 2: Sentinel handling for u7ds

- **Decision**: Treat sentinel (-1) as 0 before normalization
- **Rationale**: Consistent with existing handling of u7d sentinel in `lowestUtilizationSelect`. A missing window should not penalize or benefit the token beyond "no constraint known."
- **Alternatives considered**:
  - Exclude from max: Equivalent to treating as 0 since max ignores lower values.
  - Treat as -inf: Would bias selection toward tokens with missing data — undesirable.

### Decision 3: θ_5h in CompositeScore — parameter vs hardcode

- **Decision**: Pass θ_5h as a parameter to CompositeScore (from `config.RatelimitAlertThreshold`)
- **Rationale**: θ_5h is configurable. Hardcoding 0.80 in the formula would cause score normalization to diverge from the actual gate threshold if the user changes it. θ_7d (0.90) is hardcoded in both the gate and the formula because it's intentionally fixed.
- **Alternatives considered**:
  - Hardcode both: Simpler but breaks if user customizes θ_5h.
  - Pass both as params: Over-engineering — θ_7d is a constant (`gate7d`).

### Decision 4: sentinelMaxComposite value

- **Decision**: Change from 4.0 to 2.0
- **Rationale**: New score range is [0, ~1.11]. Any value > 1.11 works as sentinel. 2.0 provides clear margin while being simple.
- **Alternatives considered**: 1.5 (too close to boundary), math.MaxFloat64 (overkill).
