# Research: Remove Codex failed-SSE synthetic cooldown

## Decision 1: Delete the synthetic writer, not generic quota gating

**Decision**: Remove only the failed-SSE-specific call and fixed two-minute state writer in `internal/proxy/handler/responses.go`. Keep `OpenAIQuotaState.Gated` and the candidate filter in `openai_subscription_routing.go`.

**Rationale**: The generic gate is also used for official OpenAI `x-ratelimit-*` response-header state. Removing it would be a wide behavioral change. The failed-SSE state is the only locally synthesized policy in this scope.

**Repository evidence**:

- `responses.go` detects failed SSE only after `copyHTTPResponse` has relayed the response.
- `recordOpenAISubscriptionCodexResponseFailure` writes `OpenAIQuotaStatusRejected`, `QuotaResetAt`, and utilization `1`.
- `openai_subscription_routing.go` consumes that state as `rate_limited`.
- The focused regression currently locks this behavior.

**Alternatives rejected**:

- Keep the gate but reduce its duration: still makes Tianji own upstream recovery timing.
- Add exponential backoff/jitter: expands proxy policy and duplicates the separate usage snapshot path.
- Clear the state asynchronously: adds timing races and still creates a false local quota signal.
- Remove `OpenAIQuotaState.Gated`: breaks official header-based gating.

## Decision 2: Preserve the response commitment boundary

**Decision**: Keep upstream response copying before failed-event inspection and do not retry HTTP 200 failed SSE.

**Rationale**: Once the response status/body is written to the client, another upstream attempt cannot repair the already-started stream and could duplicate output or billing.

**Adjacent specification evidence**: `specs/1179-anthropic-like-sticky-failover-openai-subscriptions/` explicitly limits failover to before streaming bytes are relayed; `specs/1181-openai-quota-rate-limit-header-parser-and-store/` preserves no failover after a streaming body is committed.

## Decision 3: Keep official quota and usage snapshot paths separate

**Decision**: Do not modify `internal/callback/openai_quota_store.go`, `internal/proxy/handler/openai_subscription_routing.go`, `internal/proxy/handler/openai_subscription_provider_retry.go`, or `internal/proxy/handler/openai_subscription_codex_usage.go`.

**Rationale**: Official response-header gating, pre-body 429/5xx failover, credential lifecycle handling, and Codex usage snapshot backoff each have separate ownership and tests. The feature only removes the post-commit failed-SSE gate.

## Decision 4: No external documentation dependency is required

**Decision**: No new library, API, SDK, or cloud-service contract is introduced. Existing repository evidence and adjacent specs are sufficient for this dependency-free deletion.

**Rationale**: The change is an internal refactor/bug fix, which the constitution permits without external technology research. If implementation later changes a public provider contract, that would require a new plan/research pass.

## Verification notes

- The planning baseline was current HEAD `dad5042` on 2026-08-13. Implementation must recheck source and test locations before editing because the repository may advance.
- The existing focused test is the direct assertion of the synthetic failed-SSE gate; implementation should still run a final caller/reference scan before deleting the recorder.
