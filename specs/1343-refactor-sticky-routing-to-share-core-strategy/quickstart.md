# Quickstart: HO-1343 verification

## Todo docs-only gate

```bash
git diff --check origin/main...HEAD
git diff --name-only origin/main...HEAD
```

Expected Todo result: only files under `specs/1343-refactor-sticky-routing-to-share-core-strategy/` change.

## Future implementation verification

Run targeted sticky and routing tests:

```bash
go test ./internal/proxy/handler -run 'Sticky|OpenAISubscriptionRouting|CodexUsage|Responses|ChatGPTCodex' -count=1
go test ./internal/callback -run 'RateLimit|OpenAIQuota' -count=1
```

Run broader touched-package tests:

```bash
go test ./internal/proxy/handler -count=1
git diff --check origin/main...HEAD
```

## Manual review checks

- Shared sticky core stores non-secret candidate IDs only.
- Claude adapter still splits sonnet/all tracks.
- Claude adapter still re-evaluates on 5h reset change.
- Codex adapter reads only fresh cache entries.
- Chat completions, Responses HTTP, and Responses WebSocket do not call Codex usage fetch in request routing.
