# Quickstart: Claude Code OAuth Token Usage & Limits

## Prerequisites

- TianjiLLM built and running (`make build && make run`)
- At least one Anthropic OAuth token configured in `proxy_config.yaml`
- PostgreSQL running (for org ID persistence)

## Verify the Feature

### 1. Visit the Claude Code Tab

```
http://localhost:4000/ui/usage?tab=claude-code
```

Expected: One card per configured OAuth token showing:
- "Current session" progress bar with % and reset countdown
- "Weekly — All models" progress bar with % and reset time
- Extra usage info (if available)
- Organization ID or hash prefix as card title

### 2. Verify On-Demand Fetch

If no proxy traffic has occurred, visiting the tab should trigger an on-demand fetch from `api.anthropic.com/api/oauth/usage`. Cards should populate within 5 seconds.

### 3. Verify Passive Update

Send a proxy request:
```bash
curl -X POST https://localhost:4000/v1/messages \
  -H "Authorization: Bearer sk-your-virtual-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
```

Refresh the Claude Code tab — data should be updated from the background fetch triggered by the proxy response.

### 4. Verify Org ID Capture

After at least one proxy request, the card title should change from the hash prefix (e.g. `a3bb64f70a1d`) to the organization ID (e.g. `40403251-c713-4890-91a7-03763c31003f`).

## Key Code Paths

| Component | File |
|-----------|------|
| Usage API client | `internal/callback/oauth_usage_fetch.go` |
| Usage store | `internal/callback/oauth_usage_store.go` |
| Org ID DB queries | `internal/db/queries/oauth_token_metadata.sql` |
| DB migration | `internal/db/schema/NNN_oauth_token_metadata.up.sql` |
| UI handler | `internal/ui/handler_claude_code.go` |
| Templ component | `internal/ui/pages/usage_claude_code.templ` |
| Proxy callback | `internal/proxy/handler/native_format.go` (modified) |
| Old widget removal | `internal/ui/pages/usage.templ` (RateLimitWidget deleted) |

## Run Tests

```bash
# Unit tests for usage fetch + store
go test ./internal/callback/... -run "TestOAuthUsage" -v

# UI handler tests
go test ./internal/ui/... -run "TestClaudeCode" -v

# Contract tests for org ID capture
go test ./test/contract/... -run "TestOrgID" -v

# Full suite
make test
```
