# Quickstart: HO-2368 verification

## Prerequisites

- Use the issue worktree: `/root/.openclaw/workspace-sima/worktrees/tianjiLLM/HO-2368-tianji-codex-session-sticky-should-not-reselect`
- Stay on branch: `HO-2368-tianji-codex-session-sticky-should-not-reselect`
- Do not implement from Todo; these commands are for In Progress verification after failing tests and implementation.

## Failing Tests First

1. Add the failing tests listed in [plan.md](./plan.md#failing-tests) to `internal/proxy/handler/openai_subscription_routing_test.go`.
2. Run the focused test command and confirm the new tests compile and fail before production code changes:

```bash
go test ./internal/proxy/handler -run 'Codex.*Sticky|OpenAISubscriptionRouting_StickyCodex|CodexSession' -count=1
```

## Implementation Verification

After implementing the routing-policy change, run:

```bash
go test ./internal/proxy/handler -run 'Codex.*Sticky|OpenAISubscriptionRouting_StickyCodex|CodexSession' -count=1
go test ./internal/proxy/handler -count=1
git diff --check origin/main...HEAD
```

## Expected Results

- Session-aware primary-reset-only test passes: the same session keeps the same credential when known/selectable.
- Primary and secondary gate tests pass: non-selectable selected credentials still reselect.
- Existing fallback primary-reset test passes or equivalent fallback coverage proves unchanged behavior.
- Log redaction test passes and raw session ids are not present.
- No UI, DB, config, route-key shape, or rendezvous hashing changes appear in the diff.
