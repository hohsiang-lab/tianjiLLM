# Quickstart: Upstream Token in SpendLogs + Key Alias

## Implementation Order

```
1. DB migration (019)           → schema change, no code dependency
2. sqlc query changes           → make generate → new Go structs
3. callback.LogData + SpendRecord → add UpstreamTokenKey field
4. spend/tracker.go Record()    → pass UpstreamTokenKey to CreateSpendLog
5. middleware/helpers.go         → add ContextKeyUpstreamToken constant
   handler.go buildBaseLogData() → extract UpstreamTokenKey from ctx (1 line)
   native_format.go              → ctx = context.WithValue(ctx, ContextKeyUpstreamToken, sha256[:12])
                                   (1 line after line 58, before ReverseProxy. No signature changes.)
6. handler_logs.go              → map KeyAlias + UpstreamTokenKey in toLogRow()
7. logs.templ                   → add columns + filter input
8. log_detail.templ             → add UpstreamTokenKey display
```

## Key Commands

```bash
# After modifying .sql files:
make generate          # sqlc codegen

# After modifying .templ files:
make ui                # templ generate + tailwind build

# Run tests:
go test ./internal/proxy/handler/... -run "TestNativeSpend|TestStandardProvider" -v
go test ./internal/ui/... -run "TestListRequestLogs_|TestLogDetail_" -v

# Full check:
make check             # lint + test + build
```

## Verification

1. Send request through native proxy (`POST /v1/messages`)
2. Check SpendLogs: `SELECT upstream_token_key FROM "SpendLogs" ORDER BY starttime DESC LIMIT 5`
3. Visit `/ui/logs` — confirm Key Alias column shows alias, not just hash
4. Filter by upstream token — confirm only matching entries shown
5. Click a log entry — confirm upstream token displayed in detail panel
