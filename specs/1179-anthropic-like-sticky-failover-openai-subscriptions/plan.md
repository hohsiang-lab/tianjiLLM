# Implementation Plan: Anthropic-like Sticky/Failover for OpenAI Subscriptions

**Branch**: `HO-1179-anthropic-like-sticky-failover-openai-subscriptions` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1179-anthropic-like-sticky-failover-openai-subscriptions/spec.md`

## Summary

Extend OpenAI subscription credential resolution from single-ID selection to multi-credential routing. The implementation should introduce an OpenAI-specific routing layer that reuses the existing handler strategy concepts, filters disabled/unusable credentials, tracks provider-appropriate OpenAI rate-limit state from official headers, scopes sticky selection by org, and retries another configured credential before returning an all-unusable error.

## Technical Context

**Language/Version**: Go
**Primary Dependencies**: Existing `net/http`, `sync`, `time`, `golang.org/x/sync/singleflight`, existing config/router/handler packages
**Storage**: Existing credential DB only; new OpenAI routing state is in-memory for this slice
**Testing**: Go unit/integration tests under `internal/proxy/handler` and `internal/config`; `httptest` for official OpenAI endpoint behavior
**Target Platform**: TianjiLLM server / Docker / Kubernetes
**Performance Goals**: Candidate selection must stay O(number of configured credential IDs) per request and cap failover attempts at the number of candidates
**Constraints**: No production code in Todo state; no raw secret logging; no real OpenAI network in tests; no `api_key` fallback when subscription IDs are configured
**Scale/Scope**: Small configured credential lists per model config; multiple orgs concurrently sharing the same Tianji deployment

## Current Repo Findings

- `internal/proxy/handler/openai_subscription_resolution.go` resolves configured subscription IDs by taking `ids[0]`; HO-1179 must replace that single-ID behavior with candidate routing.
- `internal/proxy/handler/openai_subscription_refresh.go` already has typed sanitized credential errors for missing, wrong type, disabled, malformed, expired, lookup failed, and refresh failed.
- `internal/config/validate.go` already validates `openai_subscription_credential_ids`, rejects empty/duplicates, rejects custom `api_base`, and limits the feature to official OpenAI models.
- `internal/proxy/handler/native_upstream.go` already implements native strategy concepts: per-provider round-robin, disabled-token exclusion, throttle gating, sticky selection, and lowest-utilization selection for Anthropic OAuth tokens.
- `internal/proxy/handler/native_format.go` records Anthropic rate-limit headers on both success and non-success native proxy responses; OpenAI needs a separate parser/state model because its headers are request/token reset based.
- `internal/proxy/middleware/auth.go` exposes `ContextKeyOrgID`; `internal/proxy/handler/handler.go` already reads org ID for request metadata. This is the correct sticky scope source.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Repo Reality First | PASS | Plan is anchored in current resolver, native upstream strategy, config validation, and org context code. |
| II. Feature Parity | PASS | Mirrors Anthropic strategy intent without copying Anthropic-only quota math. |
| III. Research Before Build | PASS | research.md records repo, Context7/OpenAI docs, web, and grep-app evidence. |
| IV. Failing-Tests-First | PASS | Tasks begin with failing tests for resolver, sticky, gating, failover, and no fallback. |
| V. Go Best Practices | PASS | Small handler-local types, bounded attempts, context-aware credential refresh, mutex-protected in-memory state. |
| VI. No Stale Knowledge | PASS | OpenAI rate-limit headers verified against current official docs on 2026-05-07. |
| VII. sqlc-First DB Access | N/A | No new DB queries or migrations are planned; use existing credential DB methods. |

## Project Structure

### Documentation (this planning PR)

```text
specs/1179-anthropic-like-sticky-failover-openai-subscriptions/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── openai-subscription-routing.md
├── checklists/
│   └── requirements.md
├── tasks.md
└── analyze.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/
├── proxy/handler/
│   ├── openai_subscription_resolution.go       # MODIFY: route multiple IDs, not ids[0]
│   ├── openai_subscription_routing.go          # NEW: candidates, strategy, sticky, failover attempts
│   ├── openai_subscription_ratelimit.go        # NEW: parse OpenAI x-ratelimit headers into in-memory state
│   ├── openai_subscription_resolution_test.go  # MODIFY: multi-ID/no-fallback resolver cases
│   ├── openai_subscription_routing_test.go     # NEW: sticky/failover/all-unusable tests
│   └── openai_subscription_endpoints_test.go   # MODIFY: official endpoint failover/header tests
└── config/
    └── validate_test.go                        # MODIFY if new strategy/config edge coverage is needed
```

**Structure Decision**: Keep OpenAI subscription routing under `internal/proxy/handler` because current subscription resolution, refresh, endpoint injection, and native proxy strategy all live there. Do not generalize the Anthropic `callback.RateLimitStore` in this slice; OpenAI headers have a different shape and should use a provider-specific state type keyed by credential ID/account ID.

## Data Flow

```text
Request resolves model config
  -> TianjiParams.OpenAISubscriptionCredentialIDs present
  -> build candidates from every configured credential ID
       -> load credential metadata
       -> exclude disabled/wrong type/missing/malformed/expired/refresh-failed
       -> attach resolved bearer/Codex login material by transport
       -> attach in-memory OpenAI rate-limit state if known
  -> filter gated candidates
  -> select candidate using native_upstream_strategy semantics
       -> round_robin: per OpenAI subscription routing key
       -> sticky: org-scoped sticky credential ID while available
       -> lowest_utilization: use OpenAI remaining/reset state when present; deterministic fallback when absent
  -> send official OpenAI HTTP request with selected bearer
  -> parse response headers
       -> update candidate rate-limit state
       -> if retryable/pre-body failure and more candidates exist, try next
  -> return first successful/final response or all-unusable sanitized error
```

## Routing Semantics

### Candidate identity

Use credential ID as the primary routing key. Account ID may be stored for diagnostics, but raw bearer/access/refresh tokens must never be used as map keys, logs, or error text.

### Sticky key

OpenAI sticky state should use:

```text
openai-subscription:<org-scope>:<model-or-route-class>
```

- `org-scope` is `org:<ContextKeyOrgID>` when present.
- `org-scope` is a deterministic `org:__master__` or `org:__none__` value for master/no-org requests.
- Model/route class should be stable enough to avoid unrelated model groups fighting over the same sticky state. It must not include user ID.

### Rate-limit gate

OpenAI response headers to parse:

- `x-ratelimit-limit-requests`
- `x-ratelimit-limit-tokens`
- `x-ratelimit-remaining-requests`
- `x-ratelimit-remaining-tokens`
- `x-ratelimit-reset-requests`
- `x-ratelimit-reset-tokens`

Gate a candidate when either remaining requests or remaining tokens is parsed as zero and the corresponding reset time is in the future. If both reset headers exist, use the later relevant reset for the exhausted dimension. If headers are absent or malformed, do not gate solely from missing data; keep the candidate available and record no state update.

### Failover

Failover is allowed only before Tianji commits a response body to the client:

- Credential load/refresh failure: try the next candidate.
- Direct HTTP non-streaming 429/5xx before returning to client: update state and try next candidate.
- Direct HTTP streaming non-2xx before body relay: update state and try next candidate.
- Streaming mid-body failure: do not retry because the client has already observed partial output.

Attempts must be capped at the number of configured candidates after disabled exclusion.

## Failing Tests

### Resolver and candidate tests

| Test Function | File | Assertion |
|---------------|------|-----------|
| `TestResolveOpenAISubscriptionCandidates_IncludesAllConfiguredIDs` | `internal/proxy/handler/openai_subscription_routing_test.go` | Multiple configured IDs produce multiple candidates in config order. |
| `TestResolveOpenAISubscriptionCandidates_DisabledExcluded` | `internal/proxy/handler/openai_subscription_routing_test.go` | Disabled credential is not returned as a candidate. |
| `TestResolveOpenAISubscriptionCredential_AllUnusableNoAPIKeyFallback` | `internal/proxy/handler/openai_subscription_resolution_test.go` | With subscription IDs configured, all unusable returns explicit error and never returns fallback API key. |
| `TestResolveOpenAISubscriptionCandidates_PreservesReasonCodes` | `internal/proxy/handler/openai_subscription_routing_test.go` | Missing/disabled/malformed/refresh failed reasons are preserved and sanitized. |

### Sticky and strategy tests

| Test Function | File | Assertion |
|---------------|------|-----------|
| `TestOpenAISubscriptionRouting_StickyStablePerOrg` | `internal/proxy/handler/openai_subscription_routing_test.go` | Repeated requests for one org reuse the same credential while available. |
| `TestOpenAISubscriptionRouting_StickySeparateOrgs` | `internal/proxy/handler/openai_subscription_routing_test.go` | Different org IDs do not share sticky state. |
| `TestOpenAISubscriptionRouting_StickyReevaluatesUnavailableCredential` | `internal/proxy/handler/openai_subscription_routing_test.go` | Disabled/gated sticky candidate is replaced. |
| `TestOpenAISubscriptionRouting_RoundRobinDefault` | `internal/proxy/handler/openai_subscription_routing_test.go` | Default strategy rotates through available candidates. |

### OpenAI rate-limit tests

| Test Function | File | Assertion |
|---------------|------|-----------|
| `TestParseOpenAIRateLimitHeaders_RequestsExhausted` | `internal/proxy/handler/openai_subscription_ratelimit_test.go` | Remaining requests zero gates until reset. |
| `TestParseOpenAIRateLimitHeaders_TokensExhausted` | `internal/proxy/handler/openai_subscription_ratelimit_test.go` | Remaining tokens zero gates until reset. |
| `TestParseOpenAIRateLimitHeaders_MissingHeadersNoGate` | `internal/proxy/handler/openai_subscription_ratelimit_test.go` | Missing headers do not fabricate a gate. |
| `TestOpenAISubscriptionEndpoints_UpdateRateLimitStateOn200And429` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | Official endpoint responses update state on both success and rate-limit responses. |

### Failover tests

| Test Function | File | Assertion |
|---------------|------|-----------|
| `TestOpenAISubscriptionRouting_FailoverAfterRefreshFailure` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | First credential refresh failure sends request with second credential. |
| `TestOpenAISubscriptionRouting_FailoverOnOpenAI429BeforeReturn` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | First credential 429 is gated and request succeeds with second credential. |
| `TestOpenAISubscriptionRouting_FailoverOnOpenAI5xxBeforeReturn` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | Retryable 5xx tries next credential before returning. |
| `TestOpenAISubscriptionRouting_NoRetryAfterStreamingBodyStarted` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | Mid-stream failure is not retried after bytes are relayed. |

### Regression tests

| Test Function | File | Assertion |
|---------------|------|-----------|
| `TestResolveOpenAIAPIKeyForParams_APIKeyPathUnchanged` | `internal/proxy/handler/openai_subscription_resolution_test.go` | No subscription IDs keeps existing API-key behavior. |
| `TestValidateOpenAISubscriptionCredentialIDs_CustomAPIBaseStillRejected` | `internal/config/validate_test.go` | Existing validation still rejects subscription IDs with custom `api_base`. |

## Verification Commands

```bash
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscription|TestOpenAISubscriptionRouting|TestParseOpenAIRateLimit|TestOpenAISubscriptionEndpoints' -count=1 -v
go test ./internal/config/... -run 'TestValidateOpenAISubscription' -count=1 -v
go test ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Phase 0: Research Complete

- Repo reality checked for current OpenAI subscription resolver, refresh errors, native upstream strategy, Anthropic sticky state, OpenAI endpoint auth injection, config validation, and org context.
- OpenAI official documentation checked through Context7 and official docs search for response rate-limit headers.
- GitHub public examples checked through grep-app for Go handling/documentation of OpenAI-style rate-limit headers.
- No relevant matching implementation was found in local reference repos; the plan follows TianjiLLM's current handler architecture.

## Phase 1: Tests First

Write resolver/candidate tests, sticky org-scope tests, rate-limit parser tests, endpoint failover tests, and no-fallback regression tests before production code. Initial expected failures should be missing multi-candidate routing functions or current `ids[0]` behavior.

## Phase 2: Candidate Routing

Introduce OpenAI subscription candidate construction around existing `resolveUsableOpenAISubscriptionBundle`. Preserve typed credential error codes and produce a sanitized all-unusable aggregate error. Do not change API-key path when no subscription IDs are configured.

## Phase 3: Strategy and Sticky State

Add OpenAI routing selection that honors `NativeUpstreamStrategy`. Use credential ID keys, per-handler mutex/atomic state like existing native upstream routing, and org-scoped sticky tracks.

## Phase 4: Rate-limit State

Add OpenAI-specific header parser and in-memory state store. Update state on official OpenAI HTTP responses before deciding whether to fail over or return. Avoid reusing Anthropic `RateLimitStore` because header shape and gating semantics differ.

## Phase 5: Endpoint Failover Integration

Wrap direct official OpenAI endpoint dispatch so it can retry with the next candidate before response commitment. Keep streaming failover limited to pre-body non-2xx responses.

## Phase 6: Regression Verification

Run targeted handler/config tests, package tests for touched packages, `git diff --check`, and PR diff sanity. Verify changed source files stay in the backend routing scope plus SpecKit artifacts.

## Risk Register

| Risk | Mitigation |
|------|------------|
| Copying Anthropic 5h/7d gates would be wrong for OpenAI | Keep a separate OpenAI rate-limit state based only on official OpenAI headers. |
| Sticky global state could route one org based on another org's selection | Include org scope in OpenAI sticky key and add separate-org tests. |
| Retrying streaming after partial response would corrupt client behavior | Allow failover only before body relay; test the mid-stream no-retry case. |
| All-unusable error could leak tokens or upstream payloads | Use existing typed credential codes and redaction helpers; assert sanitized error text. |
| `lowest_utilization` may have sparse OpenAI data | Use OpenAI state only when headers exist; otherwise deterministic fallback, no invented utilization. |

> Plan reviewed via Context7 / grep-app / official OpenAI docs search on 2026-05-07. Revisions applied: OpenAI rate-limit gating is header/reset based and explicitly excludes Anthropic 5h/7d/sonnet math.
