# Quickstart: Proactive OpenAI Subscription Credential Refresh

## Todo Validation

```bash
pnpm dlx prettier@3.7.1 --check specs/HO-2091-proactively-refresh-idle-openai-subscription --no-config
git diff --check origin/main...HEAD
git diff --name-only origin/main...HEAD
```

Expected Todo diff:

```text
specs/HO-2091-proactively-refresh-idle-openai-subscription/**
```

## Implementation Validation After Linear Moves To In Progress

Write failing tests first, confirm they fail, then implement.

```bash
go test ./internal/proxy/handler/... ./internal/scheduler/... -run 'TestOpenAISubscriptionProactive|TestResolveOpenAISubscriptionCandidates|TestOpenAISubscriptionLifecycle' -count=1 -v
go test ./internal/proxy/handler/... ./internal/ui/... ./internal/scheduler/... -count=1
make generate
git diff --check origin/main...HEAD
```

No test may call real `auth.openai.com` or `api.openai.com`; use existing `internal/testutil/openaitest` guarded clients.
