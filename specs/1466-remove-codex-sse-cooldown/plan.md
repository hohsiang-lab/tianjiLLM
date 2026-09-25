# Implementation Plan: Remove Codex failed-SSE synthetic cooldown

**Branch**: `1466-remove-codex-sse-cooldown` | **Date**: 2026-08-13 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/1466-remove-codex-sse-cooldown/spec.md`

## Summary

Delete Tianji's post-commit, two-minute failed-SSE credential gate. Keep the existing `response.failed` parsing/logging and HTTP 200/SSE passthrough, while leaving official OpenAI response-header quota gating, pre-body 429/5xx failover, credential lifecycle safety, and the separate Codex usage snapshot backoff path unchanged.

The smallest implementation is limited to the failed-SSE handler boundary and its focused regression test in `internal/proxy/handler/responses.go` and `internal/proxy/handler/responses_codex_test.go`. No new runtime component, configuration, persistence, endpoint, or abstraction is needed.

## Technical Context

**Language/Version**: Go; use the version declared by `go.mod` and CI.

**Primary Dependencies**: Existing `net/http`, handler routing, callback logging, OpenAI quota helpers, and `testify`; no new dependency.

**Storage**: Existing in-memory OpenAI quota state is unchanged for official upstream header signals. No new or changed persistent storage.

**Testing**: Focused `go test` for `internal/proxy/handler`; existing adjacent routing, quota, credential lifecycle, and Codex usage tests; `git diff --check`.

**Target Platform**: TianjiLLM Go proxy service on its existing server platform.

**Project Type**: HTTP proxy/web service.

**Performance Goals**: Failed SSE handling adds no retry or cooldown work after response passthrough; request latency and upstream call count remain unchanged.

**Constraints**: Narrow removal only; preserve response bytes/status, safe failure logging, pre-commit failover, official header gates, credential disable/refresh behavior, and usage snapshot backoff. Never retry after response commitment. No schema, public contract, or dependency changes.

**Scale/Scope**: One handler path, one focused regression test, two production/test files; no cross-process state or new API surface.

## Current Repository Evidence

- `internal/proxy/handler/responses.go:32` defines `codexResponsesFailureBackoffDuration = 2 * time.Minute`.
- `internal/proxy/handler/responses.go:118-121` copies the upstream response before parsing a failed SSE and then calls the synthetic recorder.
- `internal/proxy/handler/responses.go:428-456` writes `OpenAIQuotaStatusRejected`, `QuotaResetAt = now + 2 minutes`, utilization `1`, and logs `responses.codex_sse_failure_backoff`.
- `internal/proxy/handler/openai_subscription_routing.go:116-123` excludes candidates whose existing quota state satisfies `state.Gated(now)` and reports `rate_limited`.
- `internal/callback/openai_quota_store.go:43-51` contains the general gate semantics used by official quota states and must remain unchanged.
- `internal/proxy/handler/openai_subscription_provider_retry.go` performs pre-body retry/failover for retryable HTTP statuses and refreshable 401s; it is not a failed-SSE retry path.
- `internal/proxy/handler/responses_codex_test.go:817-854` currently asserts the synthetic gate and must be rewritten to assert its absence while retaining passthrough and failure logging assertions.
- `specs/1179-anthropic-like-sticky-failover-openai-subscriptions/` defines the response-commit boundary and pre-body 429/5xx failover invariant.
- `specs/1181-openai-quota-rate-limit-header-parser-and-store/` defines conservative official header gating and missing-header behavior.
- `specs/1340-add-codex-usage-snapshot-for-openai-subscription/` and `specs/1343-refactor-sticky-routing-to-share-core-strategy/` keep usage snapshot refresh/backoff outside the proxy hot path.

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Python-First Reference | PASS / N/A | This is Go proxy behavior with no maintained Python parity surface identified. |
| II. Feature Parity | PASS | Existing client-visible HTTP 200/SSE passthrough and failure diagnostics remain stable; only an internal synthetic gate is removed. |
| III. Evidence-Based Research Before Build | PASS | Repository source, focused test, graph/CCC searches, and adjacent specs establish the behavior; no new library/API decision is introduced. |
| IV. Failing-Tests-First Development | PASS | Plan requires rewriting the focused regression to fail on the old gate and pass only after its removal. |
| V. Idiomatic Go & Simplicity | PASS | Delete the unused constant/recorder path rather than add a replacement abstraction. |
| VI. Verified, Current Decisions | PASS | Current HEAD `dad5042`, current source, graph index, and focused test were checked on 2026-08-13. |
| VII. SQL-First Database Access | PASS / N/A | No schema or SQL changes. |
| VIII. Standards-First Compatibility | PASS | `/v1/responses` status/body/error behavior is preserved; no new route or payload is introduced. |
| IX. Secure-by-Default Operations | PASS | Existing redacted failure logging and credential safety remain; no secret-bearing state is added. |

**Gate result**: PASS. No constitution exception or complexity tracking entry is required.

## Failing Tests

The first implementation task must update the existing focused test before production code changes:

- `internal/proxy/handler/responses_codex_test.go:817` — first rewrite `TestCreateResponse_LogsCodexSSEFailedAsFailureAndBacksOffCredential` to `TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff`, keeping the passthrough/failure-log assertions and replacing the gate assertions with `assert.False(t, ok)` for the rate-limit state. The pre-change test must fail because it currently finds state and expects it to be gated.
- The same test must assert the in-memory state lookup is absent; the existing routing gate already maps any independently present state to `rate_limited`, so a separate route-resolution test is unnecessary for this deletion.

No new test framework or fixture is needed; reuse `newCodexResponsesTestHandler`, `responsesLogCapture`, and the existing mock backend.

## Research Decisions

### Decision 1: Remove the root cause, not the downstream gate

- **Chosen**: Delete the failed-SSE-specific state writer and its fixed-duration constant/log event; retain `OpenAIQuotaState.Gated` and routing exclusion for independently recorded quota states.
- **Rationale**: The gate is generic and still required for official upstream header signals. Removing it globally would broaden scope and break unrelated routing.
- **Alternatives rejected**: Clear the state later, shorten the duration, make the duration configurable, or add a replacement retry/backoff. Each preserves or expands the proxy-owned policy the feature explicitly removes.

### Decision 2: Preserve the response-commit boundary

- **Chosen**: Keep `copyHTTPResponse` before failure inspection and do not retry after it.
- **Rationale**: The response status/body has already reached the client; retrying would duplicate or mix output and cannot repair the current request.
- **Evidence**: Existing `responses.go` ordering and the adjacent HO-1179 response-commit contract.

### Decision 3: Keep Codex usage snapshot backoff separate

- **Chosen**: Do not touch `internal/proxy/handler/openai_subscription_codex_usage.go` or its cache/backoff tests.
- **Rationale**: That path protects an unofficial usage-status endpoint and is not the failed-SSE request-routing behavior.
- **Evidence**: HO-1340/HO-1343 design artifacts and current routing code.

## Project Structure

### Documentation (this feature)

```text
specs/1466-remove-codex-sse-cooldown/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── tasks.md
```

No `contracts/` directory is needed: the public `/v1/responses` contract is intentionally unchanged and this feature introduces no new external interface.

### Source Code (repository root)

```text
internal/proxy/handler/
├── responses.go              # remove failed-SSE synthetic cooldown writer
└── responses_codex_test.go   # rewrite focused regression expectations
```

**Structure Decision**: Keep the implementation in the existing Responses handler and test file. No new package or shared helper is justified by a single deleted policy path.

## Implementation Phases

### Phase 0 — RED test and boundary review

1. Rewrite the focused failed-SSE regression so it expects passthrough/logging plus no synthetic quota gate; run it and confirm it fails against the current implementation.
2. Verify exact callers of the failed-SSE recorder and confirm no sibling path requires it.
3. Re-run targeted CCC search and existing graph query after the test boundary is precise.

### Phase 1 — Minimal deletion

1. Remove the failed-SSE recorder call from `handleChatGPTCodexResponse` while leaving failure parsing/logging and client passthrough intact.
2. Delete the now-unused fixed-duration constant and recorder function/log event.
3. Do not edit generic quota store, routing gate, provider retry, usage snapshot, credential lifecycle, or database code.

### Phase 2 — Verification

1. Run the focused Responses regression.
2. Run adjacent OpenAI subscription routing/quota/failover/credential lifecycle and Codex usage backoff tests.
3. Run `git diff --check`; implementation remains limited to the planned two-file boundary.

## Complexity Tracking

No violations. The feature deletes one proxy-owned policy rather than introducing a replacement mechanism.
