# Quickstart: Remove Codex failed-SSE synthetic cooldown

## Prerequisites

- Repository root: `/Users/norman/src/github.com/hohsiang-lab/tianjiLLM`
- Go toolchain and repository dependencies available.
- Worktree contains only the planned documentation changes before implementation.

## RED test before implementation

After rewriting the existing focused test to expect no synthetic gate, run:

```bash
rtk go test ./internal/proxy/handler \
  -run '^TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff$' \
  -count=1
```

Expected before the production deletion: the test fails because the current handler records `OpenAIQuotaStatusRejected` with a future reset.

## Focused verification after implementation

```bash
rtk go test ./internal/proxy/handler \
  -run '^TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff$' \
  -count=1
```

Expected: HTTP 200 and original `response.failed` SSE content are preserved, a failure callback is recorded, no success callback is recorded, and no new rate-limit state exists for `cred-a`.

## Adjacent regression verification

```bash
rtk go test ./internal/proxy/handler \
  -run 'OpenAISubscription|CodexUsage|RateLimit|Failover|Credential' \
  -count=1
rtk git diff --check
```

Expected: official quota-header gating, pre-body 429/5xx failover, refresh/disable behavior, and independent Codex usage snapshot backoff remain green.

## Scope check

```bash
rtk git status --short --branch
rtk git diff --stat
```

Implementation should touch only:

- `internal/proxy/handler/responses.go`
- `internal/proxy/handler/responses_codex_test.go`

No database, public API, generated artifact, usage-snapshot, or generic quota-store file should change.
