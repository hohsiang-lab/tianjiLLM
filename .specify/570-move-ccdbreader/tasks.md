# Tasks: HO-570 Move ccDBReader to function-level injection

## Tasks

- [ ] T001: Add `dbReader()` helper method to `UIHandler` in `handler_claude_code.go`
- [ ] T002: Change `loadClaudeCodeData` signature to accept `rateLimitStateReader` parameter; update internal logic to use parameter instead of `h.ccDBReader`
- [ ] T003: Update 3 production callers in `handler_claude_code.go` to pass `h.dbReader()`
- [ ] T004: Update 2 production callers in `handler_usage.go` to pass `h.dbReader()`
- [ ] T005: Remove `ccDBReader` field from `UIHandler` struct in `handler.go`
- [ ] T006: Update test helper and 6 test cases in `handler_claude_code_test.go` to pass stub via parameter
- [ ] T007: Run `go build ./...` and `go test ./internal/ui/...` — confirm all green
