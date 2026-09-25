# Research: HO-490 — allowed_warning bypass

## Python Reference Check

**Status**: N/A — Python TianjiLLM codebase not available locally.

This feature (Anthropic OAuth rate limit header parsing + upstream selection logic)
is a Go-only addition with no corresponding Python implementation. The behavior is
derived directly from Anthropic's API header specification.

**Documented deviation**: Go implementation introduces OAuth-specific upstream
selection logic (`selectUpstreamWithThrottle`, `lowestUtilizationSelect`) that
does not exist in Python TianjiLLM. Rationale: Python version pre-dates Anthropic's
OAuth/Claude Code plan; this is a new capability.

---

## Anthropic Header Values (Verified)

**Sources**:
- <https://github.com/anthropics/claude-code/issues/29604> — feature request, lists all internal fields
- <https://github.com/anthropics/claude-code/issues/29300> — statusline JSON proposal, header list
- <https://github.com/anthropics/claude-code/issues/41185> — `utilization` missing for `allowed` status bug
- <https://github.com/anthropics/claude-code/issues/12829> — production header dump (real values confirmed)
- <https://support.claude.com/en/articles/12429409-manage-extra-usage-for-paid-claude-plans> — extra usage / overage mechanics

### `anthropic-ratelimit-unified-status`

Confirmed values (as of 2026-04):

| Value | When | Requests accepted? |
|-------|------|--------------------|
| `allowed` | Normal quota available | ✅ |
| `allowed_warning` | Normal quota exhausted, extra usage (overage credit) active | ✅ (deducts from extra credit) |
| `rejected` | Quota exhausted, no extra usage | ❌ |

**Removed values** (not official, previously assumed):
- `rate_limited` — does not appear in Anthropic headers
- `overage` — does not appear as `unified-status` value; `overage-disabled-reason` is a separate header

### Related headers (full list, verified from #12829 production dump + #29604 + #29300)

| Header | Purpose | In scope? | Notes |
|--------|---------|----------|-------|
| `anthropic-ratelimit-unified-status` | Overall: `allowed`/`allowed_warning`/`rejected` | ✅ this PR | |
| `anthropic-ratelimit-unified-5h-utilization` | 5h window usage fraction [0,1] | existing | |
| `anthropic-ratelimit-unified-7d-utilization` | 7d window usage fraction [0,1] | existing | |
| `anthropic-ratelimit-unified-5h-reset` | 5h reset Unix timestamp | existing | |
| `anthropic-ratelimit-unified-7d-reset` | 7d reset Unix timestamp | existing | |
| `anthropic-ratelimit-unified-5h-status` | Per-window status for 5h | ⚠️ not parsed yet | seen in #12829 dump |
| `anthropic-ratelimit-unified-7d-status` | Per-window status for 7d | ⚠️ not parsed yet | seen in #12829 dump |
| `anthropic-ratelimit-unified-representative-claim` | Authoritative window (`five_hour`/`seven_day`) | existing | |
| `anthropic-ratelimit-unified-fallback-percentage` | Overage availability as float [0,1] | ⚠️ not parsed yet | **correction**: not `fallback` (bool), actual header is `fallback-percentage` (float), confirmed #12829 |
| `anthropic-ratelimit-unified-overage-disabled-reason` | Why overage disabled (e.g. `org_level_disabled`) | existing | |
| `anthropic-ratelimit-unified-overage-status` | Whether extra usage is active | ⚠️ not parsed yet | internal field: `overageStatus` |
| `anthropic-ratelimit-unified-reset` | Overall reset Unix timestamp | existing | |

**Production header dump (from issue #12829, real request)**:
```json
{
  "anthropic-ratelimit-unified-status": "allowed",
  "anthropic-ratelimit-unified-5h-status": "allowed",
  "anthropic-ratelimit-unified-5h-reset": "1764554400",
  "anthropic-ratelimit-unified-5h-utilization": "0.018416969696969696",
  "anthropic-ratelimit-unified-7d-status": "allowed",
  "anthropic-ratelimit-unified-7d-reset": "1764615600",
  "anthropic-ratelimit-unified-7d-utilization": "0.7370692663445869",
  "anthropic-ratelimit-unified-representative-claim": "five_hour",
  "anthropic-ratelimit-unified-fallback-percentage": "0.2",
  "anthropic-ratelimit-unified-reset": "1764554400",
  "anthropic-ratelimit-unified-overage-disabled-reason": "org_level_disabled"
}
```

### Overage disable conditions (from official docs + header evidence)

Extra usage (overage credit) is disabled when **any** of:
1. `overage-disabled-reason: org_level_disabled` — org admin has disabled extra usage
2. User manually disabled extra usage in Settings → Usage
3. Extra usage spending limit exhausted → subsequent requests → `status: rejected`

**No API header exposes remaining credit balance (USD).** Only `utilization` (0-1 fraction of quota) is available. Remaining credit amount is only visible in the Claude Console dashboard.

### surpassedThreshold

Internal field `surpassedThreshold` (from #29604 binary reverse) corresponds to threshold-crossing events (e.g. 75%, 80% utilization). No confirmed header name for this yet — not parsed in repo, out of scope for this PR.

**Note — official docs**:
`platform.claude.com/docs/api/rate-limits` covers only standard API key headers (`x-ratelimit-*`).
The `anthropic-ratelimit-unified-*` headers are **OAuth/Claude Code plan specific** and NOT in
standard docs. Source of truth: Anthropic GitHub issues + production header dumps.

---

## Decision

### D-001: Use `rejected` as the canonical skip status

- **Decision**: Remove `rate_limited`/`overage` consts; use only `rejected`
- **Rationale**: These are the only official values from Anthropic
- **Alternatives rejected**: Keep legacy aliases — adds confusion, tests fail against real headers

### D-002: Bypass utilization gate for `allowed_warning`

- **Decision**: `allowed_warning` tokens skip 5h/7d gate entirely in `selectUpstreamWithThrottle`
- **Rationale**: Anthropic accepts requests when status is `allowed_warning`; blocking them causes unnecessary failures
- **Alternatives rejected**:
  - Apply reduced threshold: more complex, still risks false throttling
  - Only bypass 5h gate: 7d gate could still block; semantics unclear

### D-003: Non-blocking alert for `allowed_warning`

- **Decision**: Alert fires for `allowed_warning` but does NOT early-return (utilization alerts can still fire)
- **Rationale**: Informational signal, not a hard stop — operator should know overage is consuming
- **Alternatives rejected**: Treat same as `rejected` (early-return) — too aggressive, suppresses util alerts

---

## No new dependencies

All changes are within existing packages (`internal/callback`, `internal/proxy/handler`).
No new Go libraries required.
