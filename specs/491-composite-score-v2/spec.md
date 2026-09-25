# Feature Specification: Composite Score V2 — Max-Normalized Token Selection

**Feature Branch**: `491-composite-score-v2`  
**Created**: 2026-04-10  
**Status**: Draft  
**Input**: User description: "Improve OAuth token selection strategy: replace quadratic composite score with max-normalized formula, add 7d Sonnet window support, add 7d_s gate"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sonnet-heavy workloads no longer exhaust a single token (Priority: P1)

A team runs Claude Code heavily using Sonnet models. Under the current system, the token selector ignores the 7d Sonnet utilization window. One token's 7d Sonnet usage climbs to 89% while its 7d All Models usage is only 40%. The selector keeps picking this token because its composite score (based only on 5h + 7d all) looks low. The next Sonnet request pushes it past 90%, and Anthropic rejects the request.

With the new formula, the selector sees that this token's 7d Sonnet window is the bottleneck (`0.89/0.90 = 0.99`) and routes to a healthier token instead.

**Why this priority**: This is the primary bug the change fixes — invisible Sonnet exhaustion causing unexpected 429 errors.

**Independent Test**: Configure two tokens where one has high 7d Sonnet utilization but low 7d All utilization. Send a request and verify the selector picks the other token.

**Acceptance Scenarios**:

1. **Given** Token A has u7d=40%, u7ds=85%, Token B has u7d=60%, u7ds=30%, **When** the selector runs, **Then** Token B is selected (its max-normalized score 0.67 < Token A's 0.94)
2. **Given** Token A has u7d=85%, u7ds=20%, Token B has u7d=30%, u7ds=80%, **When** the selector runs, **Then** Token A is selected (max score 0.94 vs B's 0.89)
3. **Given** a single token with u7ds=91%, **When** the 7d_s gate is evaluated, **Then** the token is filtered out and `allTokensThrottledError` is returned

---

### User Story 2 - 7d Sonnet gate prevents overshoot (Priority: P1)

When only one token is available and its 7d Sonnet utilization exceeds the 90% gate, the system must not send the request — even though CompositeScore would still select it (there's no alternative). The gate layer is the hard safety net.

**Why this priority**: Without this gate, a single-token deployment has no protection against Sonnet window exhaustion. CompositeScore is a soft optimization; the gate is the hard stop.

**Independent Test**: Configure one token with u7ds >= 90%. Send a request and verify it returns HTTP 429 with `Retry-After` header.

**Acceptance Scenarios**:

1. **Given** one token with u7ds=92% and u7d=50%, **When** a request arrives, **Then** the token is filtered out by the 7d_s gate and HTTP 429 is returned
2. **Given** two tokens — A with u7ds=92% and B with u7ds=40%, **When** a request arrives, **Then** only Token B is available after gate filtering

---

### User Story 3 - Selection score reflects actual bottleneck window (Priority: P2)

An operator views the Claude Code usage dashboard and sees the "Selection score" for each token. Under the old system, the score was a unitless number from 0 to 3.0 with no clear meaning. Under the new system, the score represents "how close is the most stressed window to its throttle gate" on a 0-to-1 scale. A score of 0.85 means the tightest window is at 85% of its gate threshold.

**Why this priority**: Operator comprehension of token health. Not blocking, but improves the ability to diagnose and tune.

**Independent Test**: View the UI with known utilization values and verify the displayed score matches the expected max-normalized calculation.

**Acceptance Scenarios**:

1. **Given** a token with u5h=60%, u7d=50%, u7ds=70%, **When** the UI displays its score, **Then** the score is max(0.60/0.80, 0.50/0.90, 0.70/0.90) = max(0.75, 0.56, 0.78) = 0.78
2. **Given** a token with all windows below 50% of their gates, **When** the UI renders, **Then** the score is displayed in green
3. **Given** a token with one window above 80% of its gate, **When** the UI renders, **Then** the score is displayed in red

---

### User Story 4 - Missing Sonnet data does not degrade selection (Priority: P2)

Some tokens may not have 7d Sonnet utilization data (sentinel value -1) — for example, tokens that have only been used for non-Sonnet models, or freshly provisioned tokens. The selector must treat missing Sonnet data as "no constraint" (0%) rather than penalizing or crashing.

**Why this priority**: Graceful degradation. Without this, any token missing Sonnet data would be treated incorrectly.

**Independent Test**: Create a token with u5h=30%, u7d=40%, u7ds=-1 (sentinel). Verify the score is max(0.30/0.80, 0.40/0.90, 0) = 0.44, and the token is selectable.

**Acceptance Scenarios**:

1. **Given** a token with u7ds=-1 (missing), **When** the score is computed, **Then** u7ds is treated as 0 and does not influence the score
2. **Given** a token with u5h=-1 (missing), **When** the score is computed, **Then** the score is 0 (all-missing = assume idle), consistent with current behavior
3. **Given** a token with u7d=-1 and u7ds=0.50, **When** the score is computed, **Then** u7d is treated as 0, and score = max(u5h/0.80, 0, 0.50/0.90)

---

### Edge Cases

- What happens when all tokens have u7ds >= 90% but u7d < 90%? → All are filtered by 7d_s gate. `allTokensThrottledError` returned with earliest 7d_s reset time.
- What happens when θ_5h config is changed from default 0.80 to 0.60? → CompositeScore normalization for 5h uses the configured value, not hardcoded 0.80.
- What happens when a token is in `allowed_warning` (overage credit) status? → Bypasses all gates including 7d_s gate, consistent with current `allowed_warning` handling.
- What happens when only non-OAuth tokens are present? → No score computed, round-robin selection, no change from current behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST compute CompositeScore using all three utilization windows: 5h, 7d All Models, and 7d Sonnet
- **FR-002**: CompositeScore MUST be calculated as `max(u5h/θ_5h, u7d/θ_7d, u7ds/θ_7d)` where θ_5h is the configured 5h threshold and θ_7d is the hardcoded 7d gate (0.90)
- **FR-003**: The token with the lowest CompositeScore MUST be selected (lowest = furthest from any gate)
- **FR-004**: System MUST filter out tokens where 7d Sonnet utilization >= θ_7d (0.90) before score evaluation — same gate logic as existing 5h and 7d gates
- **FR-005**: When a token's 7d Sonnet utilization is missing (sentinel -1), the system MUST treat it as 0 for scoring purposes
- **FR-006**: The 7d_s gate MUST track its reset time for the `Retry-After` header when all tokens are throttled
- **FR-007**: The UI MUST display the new CompositeScore with updated color thresholds: green (< 0.5), orange (< 0.8), red (>= 0.8)
- **FR-008**: The sentinel value for "no valid composite score" MUST be updated to reflect the new score range [0, ~1.11]
- **FR-009**: The UI MUST pass 7d Sonnet utilization data to the CompositeScore calculation

### Key Entities

- **CompositeScore**: A normalized value in [0, ~1.11] representing how close the most stressed utilization window is to its throttle gate. Lower is better. Replaces the old quadratic score (range [0, 3.0]).
- **7d Sonnet Gate**: A hard filter (θ_7d = 0.90) applied before score evaluation. Tokens exceeding this are excluded from selection, same as the existing 5h and 7d gates.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When one token's 7d Sonnet utilization exceeds 85% and another's is below 50%, the system selects the lower-utilization token 100% of the time
- **SC-002**: When all tokens have 7d Sonnet utilization >= 90%, the system returns a throttle error rather than sending a request that will be rejected upstream
- **SC-003**: The displayed Selection Score accurately reflects the bottleneck window — operators can identify which window is constraining each token by comparing the score to the individual window percentages
- **SC-004**: No regression in token selection latency — the additional window comparison adds negligible overhead to the selection path
- **SC-005**: All existing tests pass with updated expectations, confirming backward-compatible behavior when 7d Sonnet data is absent

## Assumptions

- The `Unified7dSonnetUtilization` field is already parsed from Anthropic response headers and stored in `AnthropicOAuthRateLimitState`. No new data collection is needed.
- The `UnifiedStatus` field from Anthropic represents the aggregate status across all windows. If the 7d Sonnet window alone is rejected, `UnifiedStatus` will reflect this. Therefore, separate `Unified7dSonnetStatus` gate checks are not needed for the `rejected` status path.
- The `allowed_warning` bypass applies to all windows collectively — if Anthropic grants overage credit, it applies to the entire token, not per-window.
- The 7d Sonnet gate uses the same threshold (0.90) as the 7d All Models gate, since both are 7-day windows with the same upstream limit semantics.
