# Implementation Plan: OpenAI Quota/Rate-limit Header Parser and Store Integration

**Branch**: `HO-1181-openai-quota-rate-limit-header-parser-and-store`
**Date**: 2026-05-07
**Spec**: [spec.md](spec.md)
**Linear**: HO-1181

## Summary

HO-1179 introduced OpenAI subscription routing and a handler-local `openAIRateLimitState`. HO-1181 should turn that behavior into a formal parser/store integration: parse official OpenAI `x-ratelimit-*` headers plus available mock quota fixtures, normalize state through one OpenAI quota store contract, and have gate, sticky, and lowest-utilization routing all read that same stored state.

## Technical Context

**Language/Version**: Go
**Relevant packages**: `internal/proxy/handler`, `internal/callback`, `internal/testutil/openaitest`, `internal/security/redact`
**Existing parser**: `internal/proxy/handler/openai_subscription_ratelimit.go`
**Existing routing consumers**: `internal/proxy/handler/openai_subscription_routing.go`
**Existing Anthropic store**: `internal/callback/ratelimit_store.go`
**Testing**: Go tests with `httptest`, `openaitest`, and no-real-OpenAI guards
**Constraints**: Todo state is docs-only; implementation starts with failing tests; no real OpenAI calls; no token leakage

## Current Repo Findings

- `internal/proxy/handler/openai_subscription_ratelimit.go` already parses official OpenAI request/token limit, remaining, and reset headers into `openAIRateLimitState`.
- The current OpenAI state is handler-local and stored in `Handlers.openAISubscriptionRateLimit`; it is not yet a shared store-facing model comparable to the existing Anthropic `RateLimitStore`.
- `internal/proxy/handler/openai_subscription_routing.go` gates subscription candidates with `state.Gated(now)` and uses the same handler-local state for lowest-utilization.
- Sticky routing for OpenAI stores only credential ID by org/model key; it re-evaluates when the selected credential leaves the available candidate set after gating.
- `internal/callback/ratelimit_store.go` contains the mature Anthropic model: explicit unknown sentinels, reset normalization, in-memory dirty tracking, DB preload/flush support, alert integration, and no raw token keys.
- `internal/testutil/openaitest/upstream_server.go` currently mocks official endpoint responses but does not expose rich quota fixture helpers; HO-1181 should add mock quota fixture support during implementation if the available mocks are insufficient.

## External Evidence

- OpenAI official rate-limit docs list `x-ratelimit-limit-requests`, `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-requests`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-requests`, and `x-ratelimit-reset-tokens`, with reset examples `1s` and `6m0s`.
- OpenAI official docs state rate limits are organization/project/model scoped and that usage limits/spend caps are separate from request/token rate limits; Tianji must not derive monthly quota locally from response headers.
- OpenAI API reference debugging docs list the same rate-limit headers and `x-request-id` as response troubleshooting metadata.
- Context7 could not be used in this run because the local account returned `Monthly quota exceeded`; official OpenAI docs were used as the primary source, and the failed Context7 check is recorded rather than silently skipped.
- grep-app public GitHub search found Go examples in `sashabaranov/go-openai`, `weaviate/weaviate`, `PullRequestInc/go-gpt3`, and `newrelic/go-agent` parsing or recording the same OpenAI rate-limit headers as durations/integers.

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| Repo reality first | PASS | Plan is based on current OpenAI parser, routing, sticky, and Anthropic store code. |
| Research before build | PASS | Official OpenAI docs and grep-app evidence checked; Context7 failure recorded. |
| Failing tests first | PASS | tasks.md starts with parser/store/routing failing tests. |
| Provider-specific semantics | PASS | OpenAI does not inherit Anthropic 5h/7d/sonnet math. |
| No production code in Todo | PASS | This branch contains SpecKit artifacts only. |
| Redaction discipline | PASS | State keys and diagnostics exclude raw token material. |

## Project Structure

### Documentation

```text
specs/1181-openai-quota-rate-limit-header-parser-and-store/
+-- spec.md
+-- plan.md
+-- research.md
+-- data-model.md
+-- quickstart.md
+-- analyze.md
+-- contracts/
|   +-- openai-quota-rate-limit-store.md
+-- checklists/
|   +-- requirements.md
+-- tasks.md
```

### Source Code, Future Implementation Scope

```text
internal/
+-- callback/
|   +-- openai_quota_store.go                # NEW or equivalent: normalized OpenAI quota state/store
|   +-- openai_quota_store_test.go           # NEW: unknown/reset/clamp/store tests
+-- proxy/handler/
|   +-- openai_subscription_ratelimit.go      # MODIFY: parser emits store-facing state
|   +-- openai_subscription_routing.go        # MODIFY: gate/sticky/lowest-util read normalized store
|   +-- openai_subscription_provider_retry.go # MODIFY: update store on all relevant attempts
|   +-- openai_subscription_ratelimit_test.go # MODIFY: official + mock quota parser tests
|   +-- openai_subscription_routing_test.go   # MODIFY: store-driven gate/sticky/lowest-util tests
+-- testutil/openaitest/
    +-- upstream_server.go                    # MODIFY if needed: quota header fixtures
```

## Proposed Data Flow

```text
Official OpenAI HTTP response
  -> parse official x-ratelimit-* headers
  -> parse available mock quota status/utilization/reset fixtures in tests
  -> normalize to OpenAIQuotaState
       -> known request/token limits and remaining
       -> reset deadlines
       -> optional derived utilization
       -> optional allowed/rejected status from test/mocked subscription quota signal
       -> unknown fields preserved explicitly
  -> store.SetOpenAIQuotaState(credentialID/accountID, state)
  -> next route resolution reads store
       -> gate rejects future-exhausted credential
       -> sticky re-evaluates when current credential is gated
       -> lowest-utilization scores known utilization/capacity
       -> unknown data fails open without fabricated estimates
```

## Design Decisions

### Store model should be OpenAI-specific

Do not force OpenAI into `AnthropicOAuthRateLimitState`. The useful store patterns are shared, but the fields are different. Recommended approach: add an OpenAI-specific state/store type near `internal/callback` or a handler-owned interface with the same explicit unknown/reset-normalization principles.

### Official headers are request/token reset based

OpenAI official docs expose request/token limit, remaining, and reset duration. Gate only from exhausted remaining counts plus future reset. Derived utilization is `(limit - remaining) / limit` only when limit is positive and remaining is known.

### Mock quota status is test evidence, not public API contract

The Linear issue mentions allowed/rejected/reset/utilization cases available from mocks. Treat those as mock-harness/subscription test fixtures unless repo evidence proves a production header name. Do not invent public OpenAI header names in production docs.

### State updates must happen before failover decisions

429 and retryable failures need to update store before trying the next credential. 401 first attempts and refreshed retries should also update any visible headers before the auth/failover branch discards the response.

### Unknown data fails open

Missing headers, malformed reset values, absent utilization, and zero/missing limits must not mark a credential unavailable. Store unknowns explicitly so UI/diagnostics and routing do not conflate unknown with zero.

## Verification Commands

```bash
go test ./internal/proxy/handler/... -run 'TestParseOpenAIRateLimit|TestOpenAIQuota|TestOpenAISubscriptionRouting' -count=1 -v
go test ./internal/callback/... -run 'TestOpenAIQuota' -count=1 -v
go test ./internal/proxy/handler/... ./internal/callback/... ./internal/testutil/openaitest/... -count=1
git diff --check origin/main...HEAD
```

## Implementation Phases

### Phase 1: Tests First

Write failing parser tests, store tests, route-gate tests, sticky re-evaluation tests, lowest-utilization tests, response-attempt update tests, and redaction tests before changing production code.

### Phase 2: Normalized State and Store

Introduce `OpenAIQuotaState` and a store contract. Preserve explicit unknowns, reset normalization, clamping, and credential/account ID keys.

### Phase 3: Parser and Mock Fixture Coverage

Update the parser to emit normalized state from official headers. Add mock-harness quota fixtures only where needed to cover allowed/rejected/reset/utilization cases without real OpenAI calls.

### Phase 4: Routing Integration

Route `resolveOpenAISubscriptionAttemptOrder`, gate filtering, sticky re-evaluation, and lowest-utilization selection through the normalized store instead of a private map.

### Phase 5: Response Attempt Integration

Ensure direct official OpenAI endpoint attempts update store on 200, 401, 429, and retryable 5xx before returning, retrying, or discarding responses.

### Phase 6: Regression Verification

Run targeted parser/store/routing tests, broader affected package tests, redaction checks, and `git diff --check`.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Existing HO-1179 behavior already parses some headers, causing duplicate logic | Refactor toward one normalized store contract; keep tests proving routing behavior unchanged. |
| Mock quota status gets mistaken for official OpenAI API | Label mock-only signals clearly and keep official header list sourced from OpenAI docs. |
| Unknown headers accidentally gate credentials | Use explicit known flags and fail-open tests. |
| Derived utilization divides by zero or overflows | Derive only with positive limits and clamp `[0,1]`. |
| Store keys leak secrets | Key by credential/account ID and add negative redaction assertions. |
| Sticky and gate read different state | Make routing consumers read one normalized store and add integration tests covering both. |

> Plan reviewed via official OpenAI docs, grep-app GitHub code search, and repo reality on 2026-05-07. Context7 was attempted but unavailable due monthly quota exhaustion.
