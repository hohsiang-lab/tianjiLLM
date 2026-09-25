# Quickstart: Persist RateLimitStore to Database

**Feature**: 199-persist-ratelimit-store
**Date**: 2026-03-26

## What Changes

OAuth token 的 rate limit utilization 數據（5h/7d/7d_sonnet）現在會每 30 秒 flush 到 PostgreSQL。Pod 重啟時從 DB 載入，不再盲選。

## Config — No Changes Required

零配置變更。只要有 PostgreSQL 連線（`DATABASE_URL`），自動啟用。沒有 DB 時退化為現有的純 in-memory 模式。

## Verification

```bash
make generate  # sqlc codegen
make test      # 全部測試

# 驗證 DB 表
psql $DATABASE_URL -c "SELECT * FROM \"OAuthTokenRateLimitState\";"

# 驗證 flush log
kubectl logs -l app=tianji | grep "ratelimit-flush"
```

## Key Files

| File | Change |
|------|--------|
| `internal/db/schema/015_oauth_token_ratelimit_state.up.sql` | NEW — DB migration |
| `internal/db/queries/oauth_token_ratelimit_state.sql` | NEW — sqlc queries |
| `internal/callback/ratelimit_store.go` | MODIFY — add dirty tracking to InMemoryRateLimitStore |
| `internal/callback/ratelimit_flusher.go` | NEW — periodic flush goroutine |
| `cmd/tianji/main.go` | MODIFY — startup preload + wire flusher |
